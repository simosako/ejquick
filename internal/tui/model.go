// Package tui implements the EJQuick interactive terminal interface: a
// two-pane incremental dictionary search on Bubble Tea v2.
package tui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/logging"
	"github.com/simosako/ejquick/internal/search"
)

// searchResultMsg carries the outcome of one asynchronous search. The
// request ID lets Update drop results of superseded requests.
type searchResultMsg struct {
	requestID       uint64
	normalizedQuery string
	entries         []search.Entry
	err             error
}

// unavailableMsg reports a dictionary that cannot be used at startup.
type unavailableMsg struct {
	dict dictionary.Type
	err  error
}

// Model is the Bubble Tea model for the whole application.
type Model struct {
	// Query is the raw query string; QueryCursor is the grapheme-cluster
	// index of the insertion point.
	Query       string
	QueryCursor int

	Results  []search.Entry
	Selected int // index into Results; -1 when nothing is selected
	// selectedID preserves the selection identity across result updates.
	selectedID int64

	ListOffset   int
	DetailOffset int

	Width  int
	Height int

	Dictionary dictionary.Type
	Available  map[dictionary.Type]bool
	// UnavailableReasons describes why a dictionary is unusable, for the
	// initial status warning.
	UnavailableReasons map[dictionary.Type]string

	RequestID    uint64
	CancelSearch context.CancelFunc

	SearchError   string
	StatusWarning string

	services map[dictionary.Type]*search.Service
	logger   *logging.Logger
}

// New builds the model for the given available services. The first
// argument selects the initially active dictionary.
func New(initial dictionary.Type, services map[dictionary.Type]*search.Service, unavailable map[dictionary.Type]string, logger *logging.Logger) *Model {
	if logger == nil {
		logger = logging.Nop()
	}
	available := make(map[dictionary.Type]bool, len(services))
	for dt := range services {
		available[dt] = true
	}
	return &Model{
		Results:            nil,
		Selected:           -1,
		selectedID:         -1,
		Dictionary:         initial,
		Available:          available,
		UnavailableReasons: unavailable,
		services:           services,
		logger:             logger,
	}
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	// Startup fallback warning when the configured default dictionary is
	// unavailable but the other one works.
	if !m.Available[m.Dictionary] {
		for _, dt := range dictionary.All {
			if m.Available[dt] {
				m.Dictionary = dt
				break
			}
		}
	}
	if reason, ok := m.UnavailableReasons[m.Dictionary.Other()]; ok && m.Available[m.Dictionary] {
		m.StatusWarning = fmt.Sprintf("%s unavailable: %s", m.Dictionary.Other().Label(), reason)
	}
	return nil
}

// searchCmd starts one asynchronous search. The returned command carries
// the request ID so stale results can be identified, and logs the
// successful outcome at debug level with the elapsed time.
func (m *Model) searchCmd(ctx context.Context, requestID uint64, rawQuery, normalizedQuery string) tea.Cmd {
	svc := m.services[m.Dictionary]
	dict := m.Dictionary
	logger := m.logger
	return func() tea.Msg {
		start := time.Now()
		entries, err := svc.Search(ctx, rawQuery)
		if err == nil {
			logger.Debug("search dict=%s request=%d query=%q results=%d elapsed=%s",
				dict, requestID, normalizedQuery, len(entries), time.Since(start).Round(time.Microsecond))
		}
		return searchResultMsg{
			requestID:       requestID,
			normalizedQuery: normalizedQuery,
			entries:         entries,
			err:             err,
		}
	}
}

// startSearch increments the request ID, cancels any in-flight search,
// and returns the command for the new search following the design's
// ordering: 1) new ID becomes current, 2) cancel previous, 3) new
// context stored, 4) command issued. An empty normalized query clears
// the result state without running a database query.
func (m *Model) startSearch(rawQuery, normalizedQuery string) tea.Cmd {
	m.RequestID++
	if m.CancelSearch != nil {
		m.CancelSearch()
		m.CancelSearch = nil
	}
	m.SearchError = ""
	m.StatusWarning = ""

	if normalizedQuery == "" {
		m.clearResults()
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.CancelSearch = cancel
	return m.searchCmd(ctx, m.RequestID, rawQuery, normalizedQuery)
}

// clearResults resets everything derived from a search result.
func (m *Model) clearResults() {
	m.Results = nil
	m.Selected = -1
	m.selectedID = -1
	m.ListOffset = 0
	m.DetailOffset = 0
}

// applyResults installs a current-request result set, preserving the
// selection by entry id when possible.
func (m *Model) applyResults(entries []search.Entry) {
	prevID := m.selectedID
	m.Results = entries
	m.DetailOffset = 0
	if len(entries) == 0 {
		m.Selected = -1
		m.selectedID = -1
		return
	}
	for i, e := range entries {
		if e.ID == prevID {
			m.Selected = i
			m.selectedID = prevID
			m.ensureSelectionVisible()
			return
		}
	}
	m.Selected = 0
	m.selectedID = entries[0].ID
	m.ensureSelectionVisible()
}

// ensureSelectionVisible scrolls the list the minimum distance needed to
// bring the selected row into the visible window.
func (m *Model) ensureSelectionVisible() {
	if m.Selected < 0 || len(m.Results) == 0 {
		m.ListOffset = 0
		return
	}
	visible := m.listHeight()
	if visible <= 0 {
		return
	}
	if m.Selected < m.ListOffset {
		m.ListOffset = m.Selected
	} else if m.Selected >= m.ListOffset+visible {
		m.ListOffset = m.Selected - visible + 1
	}
	maxOffset := len(m.Results) - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.ListOffset > maxOffset {
		m.ListOffset = maxOffset
	}
}

// switchDictionary implements Tab: clear all search state, cancel the
// in-flight request, and switch to the other available dictionary.
func (m *Model) switchDictionary() tea.Cmd {
	if len(m.Available) < 2 {
		reason := m.UnavailableReasons[m.Dictionary.Other()]
		if reason == "" {
			reason = "not configured"
		}
		m.StatusWarning = fmt.Sprintf("%s unavailable: %s", m.Dictionary.Other().Label(), reason)
		return nil
	}
	m.RequestID++
	if m.CancelSearch != nil {
		m.CancelSearch()
		m.CancelSearch = nil
	}
	m.Dictionary = m.Dictionary.Other()
	m.Query = ""
	m.QueryCursor = 0
	m.SearchError = ""
	m.StatusWarning = ""
	m.clearResults()
	return nil
}
