// Package controller implements the GUI search state machine without any Qt
// dependency. Qt callbacks call its methods on the main thread; search work is
// performed by cancellable goroutines and returned through a Dispatcher.
package controller

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/normalize"
	"github.com/simosako/ejquick/internal/search"
)

// Searcher is the subset of search.Service used by the GUI.
type Searcher interface {
	Search(context.Context, string) ([]search.Entry, error)
}

// Dispatcher transfers an immutable worker event to the GUI main thread.
type Dispatcher interface {
	Post(Event) bool
}

// DispatchFunc adapts a function to Dispatcher.
type DispatchFunc func(Event) bool

func (f DispatchFunc) Post(event Event) bool { return f(event) }

// Event is a completed search operation. Entries belong to the event after
// Post and must not be modified by the worker.
type Event struct {
	RequestID  uint64
	Dictionary dictionary.Type
	Entries    []search.Entry
	Err        error
	Elapsed    time.Duration
}

// Content identifies what the detail pane should display.
type Content int

const (
	ContentEmpty Content = iota
	ContentPending
	ContentResults
	ContentNoResults
	ContentSearchError
)

// Snapshot is a read-only view of controller state. Entries must not be
// modified by the caller.
type Snapshot struct {
	Query      string
	Dictionary dictionary.Type
	Entries    []search.Entry
	Selected   int
	Content    Content
	RequestID  uint64
	Closing    bool
}

// Controller owns one GUI window's query and result state.
type Controller struct {
	services   map[dictionary.Type]Searcher
	dispatcher Dispatcher
	maxResults int

	query      string
	dictionary dictionary.Type
	entries    []search.Entry
	selected   int
	content    Content
	requestID  uint64
	cancel     context.CancelFunc
	closing    bool
	workers    sync.WaitGroup
}

// New creates a controller. The initial dictionary must have a service.
func New(initial dictionary.Type, services map[dictionary.Type]Searcher, maxResults int, dispatcher Dispatcher) *Controller {
	return &Controller{
		services: services, dispatcher: dispatcher, maxResults: maxResults,
		dictionary: initial, selected: -1, content: ContentEmpty,
	}
}

// Snapshot returns the current state without copying entry strings.
func (c *Controller) Snapshot() Snapshot {
	return Snapshot{
		Query: c.query, Dictionary: c.dictionary, Entries: c.entries,
		Selected: c.selected, Content: c.content, RequestID: c.requestID,
		Closing: c.closing,
	}
}

// MaxResults returns the configured display limit.
func (c *Controller) MaxResults() int { return c.maxResults }

// SetQuery starts an immediate asynchronous search when query changed. A
// normalization-empty query clears state without calling the service.
func (c *Controller) SetQuery(query string) bool {
	if c.closing || query == c.query {
		return false
	}
	c.query = query
	c.invalidateRequest()

	normalized, err := normalize.Normalize(c.dictionary, query)
	if err != nil {
		c.clearEntries(ContentSearchError)
		return true
	}
	if normalized == "" {
		c.clearEntries(ContentEmpty)
		return true
	}

	if c.content != ContentResults {
		c.clearEntries(ContentPending)
	}
	service := c.services[c.dictionary]
	if service == nil {
		c.clearEntries(ContentSearchError)
		return true
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	requestID := c.requestID
	dict := c.dictionary
	dispatcher := c.dispatcher
	c.workers.Add(1)
	go func() {
		defer c.workers.Done()
		started := time.Now()
		entries, err := service.Search(ctx, query)
		if dispatcher != nil {
			dispatcher.Post(Event{
				RequestID: requestID, Dictionary: dict, Entries: entries, Err: err,
				Elapsed: time.Since(started),
			})
		}
	}()
	return true
}

// Apply installs a worker event if it still belongs to the current request.
func (c *Controller) Apply(event Event) bool {
	if c.closing || event.RequestID != c.requestID || event.Dictionary != c.dictionary {
		return false
	}
	if errors.Is(event.Err, context.Canceled) {
		return false
	}
	c.cancel = nil
	if event.Err != nil {
		c.clearEntries(ContentSearchError)
		return true
	}

	previousID := int64(-1)
	if c.selected >= 0 && c.selected < len(c.entries) {
		previousID = c.entries[c.selected].ID
	}
	c.entries = event.Entries
	if len(c.entries) == 0 {
		c.selected = -1
		c.content = ContentNoResults
		return true
	}
	c.selected = 0
	for i := range c.entries {
		if c.entries[i].ID == previousID {
			c.selected = i
			break
		}
	}
	c.content = ContentResults
	return true
}

// SwitchDictionary clears all search state and selects an available service.
func (c *Controller) SwitchDictionary(dict dictionary.Type) bool {
	if c.closing || dict == c.dictionary || c.services[dict] == nil {
		return false
	}
	c.invalidateRequest()
	c.dictionary = dict
	c.query = ""
	c.clearEntries(ContentEmpty)
	return true
}

// Clear resets the query and all result state.
func (c *Controller) Clear() bool {
	if c.closing || (c.query == "" && c.content == ContentEmpty) {
		return false
	}
	c.query = ""
	c.invalidateRequest()
	c.clearEntries(ContentEmpty)
	return true
}

// SelectRow changes the selected result without starting a database query.
func (c *Controller) SelectRow(row int) bool {
	if row < 0 || row >= len(c.entries) || row == c.selected {
		return false
	}
	c.selected = row
	return true
}

// MoveSelection moves by delta and stops at the first or last result.
func (c *Controller) MoveSelection(delta int) bool {
	if len(c.entries) == 0 || delta == 0 {
		return false
	}
	next := c.selected + delta
	if next < 0 {
		next = 0
	}
	if next >= len(c.entries) {
		next = len(c.entries) - 1
	}
	return c.SelectRow(next)
}

// SelectedEntry returns a copy of the current selected entry.
func (c *Controller) SelectedEntry() (search.Entry, bool) {
	if c.selected < 0 || c.selected >= len(c.entries) {
		return search.Entry{}, false
	}
	return c.entries[c.selected], true
}

// BeginClose rejects future work and cancels the current search. Wait may be
// called by a shutdown coordinator goroutine while the GUI event loop runs.
func (c *Controller) BeginClose() {
	if c.closing {
		return
	}
	c.closing = true
	c.invalidateRequest()
}

// Wait blocks until every search worker has returned.
func (c *Controller) Wait() { c.workers.Wait() }

func (c *Controller) invalidateRequest() {
	c.requestID++
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}

func (c *Controller) clearEntries(content Content) {
	c.entries = nil
	c.selected = -1
	c.content = content
}
