// Search controller: the widget-independent state machine of the search
// GUI (design D35 layer 1). All methods run on the Qt main thread; the
// only code that runs elsewhere is the worker closure started by
// SetQuery, which reports back through a Dispatcher as a typed UIEvent
// (design D37). Tests drive the controller with fake searchers and a
// synchronous dispatcher, without Qt.
package gui

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/logging"
	"github.com/simosako/ejquick/internal/normalize"
	"github.com/simosako/ejquick/internal/search"
)

// Searcher is the dictionary search backend the controller talks to. The
// production implementation is *search.Service; tests substitute fakes
// (design D35).
type Searcher interface {
	Search(ctx context.Context, rawQuery string) ([]search.Entry, error)
}

// Closer closes a searcher's resources. *search.Service satisfies it.
type Closer interface {
	Close() error
}

// UIEvent is a typed worker result delivered on the Qt main thread
// (design D37: a closed set of values, never closures or Qt pointers).
// M2 defines the search completion event; builder events join later.
type UIEvent interface{ isUIEvent() }

// SearchDoneEvent reports the outcome of one asynchronous search.
type SearchDoneEvent struct {
	// RequestID identifies the request; stale IDs are discarded.
	RequestID uint64
	// Dictionary is the dictionary the request ran against.
	Dictionary dictionary.Type
	// Entries is the result set on success.
	Entries []search.Entry
	// Err is the failure cause, if any.
	Err error
}

func (SearchDoneEvent) isUIEvent() {}

// Dispatcher posts a UIEvent to the Qt main thread. Post returns false
// when the application is closing and the event was dropped; callers
// must not block (design D37).
type Dispatcher interface {
	Post(UIEvent) bool
}

// DictStatus is the startup availability of one dictionary (design
// D39/D40). Category is only meaningful when Available is false.
type DictStatus struct {
	Available bool
	Category  search.OpenCategory
}

// ControllerConfig wires the controller to its backends.
type ControllerConfig struct {
	// Services holds one Searcher per available dictionary. Entries
	// implementing Closer are closed by Finalize.
	Services map[dictionary.Type]Searcher
	// Statuses holds the startup status of every dictionary, available
	// or not.
	Statuses map[dictionary.Type]DictStatus
	// Initial is the current dictionary at startup: the configured
	// default when available, else the first available dictionary.
	Initial dictionary.Type
	// MaxResults is the result limit used for count display (D17).
	MaxResults int
	// Logger receives search errors and debug timing.
	Logger *logging.Logger
}

// searchState is what the message/detail page shows for the current
// request state (design D14/D15).
type searchState int

const (
	// stateEmpty: no query; guidance message.
	stateEmpty searchState = iota
	// stateResults: a result set is displayed (possibly stale while the
	// next request is in flight, design D14).
	stateResults
	// stateNoResults: the current request returned no entries.
	stateNoResults
	// stateError: the current request failed.
	stateError
)

// Controller owns all search state of the main window.
type Controller struct {
	services   map[dictionary.Type]Searcher
	statuses   map[dictionary.Type]DictStatus
	dict       dictionary.Type
	maxResults int
	logger     *logging.Logger

	query       string
	requestID   uint64
	cancel      context.CancelFunc
	state       searchState
	entries     []search.Entry
	selectedRow int
	selectedID  int64
	closing     atomic.Bool
	wg          sync.WaitGroup

	// runWorker executes f. Production runs it on a new goroutine; tests
	// may run it inline for determinism.
	runWorker func(f func())
	// post delivers a UIEvent. Production uses the Dispatcher; tests may
	// apply events synchronously.
	post func(UIEvent)

	// finalized guards Finalize against double-closing services.
	finalized bool

	// mu guards finalized; controller state itself is confined to the
	// Qt main thread.
	mu sync.Mutex
}

// NewController builds the controller and returns the initial view.
func NewController(cfg ControllerConfig) *Controller {
	if cfg.Logger == nil {
		cfg.Logger = logging.Nop()
	}
	if cfg.MaxResults <= 0 {
		cfg.MaxResults = search.DefaultMaxResults
	}
	c := &Controller{
		services:    cfg.Services,
		statuses:    cfg.Statuses,
		dict:        cfg.Initial,
		maxResults:  cfg.MaxResults,
		logger:      cfg.Logger,
		state:       stateEmpty,
		selectedRow: -1,
		selectedID:  -1,
	}
	if c.services == nil {
		c.services = map[dictionary.Type]Searcher{}
	}
	if c.statuses == nil {
		c.statuses = map[dictionary.Type]DictStatus{}
	}
	c.runWorker = func(f func()) {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			f()
		}()
	}
	c.post = func(ev UIEvent) {}
	return c
}

// SetDispatcher connects the worker results to the Qt main thread. It is
// separate from NewController so tests can inject synchronous delivery.
func (c *Controller) SetDispatcher(d Dispatcher) {
	c.post = func(ev UIEvent) { d.Post(ev) }
}

// SetQuery starts a search for raw (design D10/D14): the request ID
// advances, an in-flight request is canceled, a normalization-empty
// query clears all search state without touching the database, and any
// other query keeps the previous result visible until the new one
// arrives.
func (c *Controller) SetQuery(raw string) {
	if c.closing.Load() {
		return
	}
	c.query = raw
	c.requestID++
	c.cancelInFlight()

	norm, err := normalize.Normalize(c.dict, raw)
	if err != nil {
		// Without a normalizable query there is nothing to search for;
		// report it through the generic search error state (D47) without
		// issuing a request.
		c.logger.Error("search dict=%s request=%d: %v", c.dict, c.requestID, err)
		c.clearResults()
		c.state = stateError
		return
	}
	if norm == "" {
		c.clearResults()
		c.state = stateEmpty
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	requestID := c.requestID
	dict := c.dict
	svc := c.services[dict]
	logger := c.logger
	start := time.Now()
	c.runWorker(func() {
		entries, err := svc.Search(ctx, raw)
		if err == nil {
			// Debug lines may carry the normalized query (design D24);
			// error lines never do (design D47).
			logger.Debug("search dict=%s request=%d query=%q results=%d elapsed=%s",
				dict, requestID, norm, len(entries), time.Since(start).Round(time.Microsecond))
		}
		c.post(SearchDoneEvent{RequestID: requestID, Dictionary: dict, Entries: entries, Err: err})
	})
}

// ClearSearch implements Escape (design D13): the query and every piece
// of derived state are cleared without a database query.
func (c *Controller) ClearSearch() {
	if c.closing.Load() {
		return
	}
	c.query = ""
	c.requestID++
	c.cancelInFlight()
	c.clearResults()
	c.state = stateEmpty
}

// SwitchDictionary implements Ctrl+Tab (design D10/D13): switching to
// the other available dictionary. Switching to an unavailable dictionary
// is ignored.
func (c *Controller) SwitchDictionary() {
	c.SwitchToDictionary(c.dict.Other())
}

// SwitchToDictionary switches to a specific dictionary (combo box and
// Dictionary menu, design D9/D10): the in-flight request is canceled and
// the query, results, selection, and scrolls are cleared for the new
// dictionary. Switching to the current or an unavailable dictionary is
// ignored.
func (c *Controller) SwitchToDictionary(dt dictionary.Type) {
	if c.closing.Load() || dt == c.dict {
		return
	}
	if !c.status(dt).Available {
		return
	}
	c.requestID++
	c.cancelInFlight()
	c.dict = dt
	c.query = ""
	c.clearResults()
	c.state = stateEmpty
}

// SelectRow sets the selection to an absolute result row (mouse clicks,
// list navigation). Out-of-range rows are ignored.
func (c *Controller) SelectRow(row int) {
	if c.closing.Load() || row < 0 || row >= len(c.entries) {
		return
	}
	c.selectedRow = row
	c.selectedID = c.entries[row].ID
}

// MoveSelection moves the selection by delta rows and reports whether it
// moved. The selection stops at the first and last row; it never wraps
// (design D13).
func (c *Controller) MoveSelection(delta int) bool {
	if c.closing.Load() || len(c.entries) == 0 {
		return false
	}
	row := c.selectedRow + delta
	if row < 0 {
		row = 0
	}
	if row > len(c.entries)-1 {
		row = len(c.entries) - 1
	}
	if row == c.selectedRow {
		return false
	}
	c.selectedRow = row
	c.selectedID = c.entries[row].ID
	return true
}

// CopySelection returns the clipboard text for Copy Entire Entry
// (design D18): headword, one newline, body, and no trailing newline.
// ok is false when nothing is selected.
func (c *Controller) CopySelection() (string, bool) {
	if c.selectedRow < 0 || c.selectedRow >= len(c.entries) {
		return "", false
	}
	e := c.entries[c.selectedRow]
	return e.Headword + "\n" + e.Body, true
}

// ApplyEvent processes one dispatched worker event on the Qt main
// thread (design D37). Stale and canceled outcomes are discarded
// (design D14/D44).
func (c *Controller) ApplyEvent(ev UIEvent) {
	if c.closing.Load() {
		return
	}
	e, ok := ev.(SearchDoneEvent)
	if !ok {
		return
	}
	if e.RequestID != c.requestID || e.Dictionary != c.dict {
		return // superseded request or dictionary switch
	}
	if e.Err != nil {
		if errors.Is(e.Err, context.Canceled) {
			return // expected outcome, not a failure
		}
		// Error logs carry the dictionary and request ID, never the
		// query (design D47).
		c.logger.Error("search dict=%s request=%d: %v", e.Dictionary, e.RequestID, e.Err)
		c.clearResults()
		c.state = stateError
		return
	}
	c.applyResults(e.Entries)
}

// BeginShutdown starts application shutdown (design D38): new queries
// and events are rejected and the in-flight request is canceled. It is
// idempotent and safe to call from the Qt main thread.
func (c *Controller) BeginShutdown() {
	if c.closing.Swap(true) {
		return
	}
	c.cancelInFlight()
}

// Finalize waits for workers and closes every closer-backed service. It
// runs after the Qt event loop has exited (design D38) and completes
// shutdown. It is idempotent.
func (c *Controller) Finalize() {
	c.BeginShutdown()
	c.wg.Wait()
	c.mu.Lock()
	if c.finalized {
		c.mu.Unlock()
		return
	}
	c.finalized = true
	services := c.services
	c.mu.Unlock()
	for _, s := range services {
		if closer, ok := s.(Closer); ok {
			closer.Close()
		}
	}
}

// Query returns the current query text.
func (c *Controller) Query() string { return c.query }

// Dictionary returns the current dictionary.
func (c *Controller) Dictionary() dictionary.Type { return c.dict }

// cancelInFlight cancels the in-flight request, if any.
func (c *Controller) cancelInFlight() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}

// clearResults resets everything derived from a search result.
func (c *Controller) clearResults() {
	c.entries = nil
	c.selectedRow = -1
	c.selectedID = -1
}

// applyResults installs a result set, preserving the selection by entry
// id when possible (design D11).
func (c *Controller) applyResults(entries []search.Entry) {
	prevID := c.selectedID
	c.entries = entries
	if len(entries) == 0 {
		c.selectedRow = -1
		c.selectedID = -1
		c.state = stateNoResults
		return
	}
	c.state = stateResults
	for i, e := range entries {
		if e.ID == prevID {
			c.selectedRow = i
			c.selectedID = prevID
			return
		}
	}
	c.selectedRow = 0
	c.selectedID = entries[0].ID
}

func (c *Controller) status(dt dictionary.Type) DictStatus {
	if s, ok := c.statuses[dt]; ok {
		return s
	}
	return DictStatus{}
}

// pageKind selects which stacked page the detail pane shows (design
// D15).
type pageKind int

const (
	// pageDetail shows headword and body of the selected entry.
	pageDetail pageKind = iota
	// pageMessage shows the contextual message.
	pageMessage
)

// View is a complete snapshot of what the Qt layer renders. Building it
// is pure computation, so the rendering code stays thin and everything
// about states and messages is unit-testable without Qt.
type View struct {
	// Dictionary is the current dictionary type.
	Dictionary dictionary.Type
	// DictionaryItems lists both dictionaries in stable order for the
	// combo box and the Dictionary menu.
	DictionaryItems []DictItemView
	// DictionaryEnabled is false when switching is impossible: with no
	// dictionary at all, or with exactly one available dictionary the
	// selector is disabled while still showing the current name
	// (design D9/D39).
	DictionaryEnabled bool
	// SearchEnabled is false when no dictionary is available.
	SearchEnabled bool
	// Query is the query field content.
	Query string
	// Headwords are the result rows in display order (design D11).
	Headwords []string
	// SelectedRow is the selected row index, -1 when nothing is
	// selected.
	SelectedRow int
	// Entry is the selected entry for the detail page, nil otherwise.
	Entry *search.Entry
	// Page selects the stacked page of the right pane.
	Page pageKind
	// Message is the message page content when Page is pageMessage.
	Message MessageView
	// CountText is the header count label text, "" when hidden (D17).
	CountText string
	// CopyEnabled reports whether Copy Entire Entry can run (D18).
	CopyEnabled bool
	// SwitchEnabled reports whether dictionary switching is possible:
	// both dictionaries must be available (D13/D39).
	SwitchEnabled bool
}

// DictItemView is one dictionary in the dictionary controls.
type DictItemView struct {
	// Type identifies the dictionary.
	Type dictionary.Type
	// Label is the display name (design D41).
	Label string
	// Enabled is false when the dictionary is unavailable.
	Enabled bool
	// Tooltip is the canonical unavailable reason; empty when enabled
	// (design D39).
	Tooltip string
}

// MessageView is the content of the contextual message page (design
// D15/D45/D47).
type MessageView struct {
	// Lines are the message text, top to bottom.
	Lines []string
	// ShowOpenLog offers the Open Log action (search errors and
	// unavailable dictionaries, when the logger has a file).
	ShowOpenLog bool
}

// View returns the current rendering snapshot.
func (c *Controller) View() View {
	items := make([]DictItemView, 0, len(dictionary.All))
	anyAvailable := false
	availableCount := 0
	for _, dt := range dictionary.All {
		st := c.status(dt)
		item := DictItemView{Type: dt, Label: DictionaryLabel(dt), Enabled: st.Available}
		if !st.Available {
			item.Tooltip = dictionaryUnavailable(dt, st.Category)
		} else {
			anyAvailable = true
			availableCount++
		}
		items = append(items, item)
	}

	v := View{
		Dictionary:      c.dict,
		DictionaryItems: items,
		SearchEnabled:   anyAvailable,
		SwitchEnabled:   availableCount == len(dictionary.All),
		Query:           c.query,
		SelectedRow:     -1,
		Page:            pageMessage,
	}
	// With only one available dictionary the selector is disabled while
	// still showing the current name (design D9); switching needs both
	// dictionaries available (design D13).
	v.DictionaryEnabled = v.SwitchEnabled

	switch {
	case !anyAvailable:
		// Both dictionaries are unavailable: the search controls are
		// disabled and the message page explains both, the configured
		// default first (design D39/D45). The Build Dictionary Database
		// action joins with the builder milestone (design D34).
		v.Message.Lines = c.unavailableLines()
		v.Message.ShowOpenLog = c.logAvailable()
	case c.state == stateEmpty:
		v.Message.Lines = EmptyQueryGuidance()
	case c.state == stateNoResults:
		v.Message.Lines = NoResultsText()
	case c.state == stateError:
		v.Message.Lines = SearchErrorText()
		v.Message.ShowOpenLog = c.logAvailable()
	default: // stateResults
		v.Page = pageDetail
		v.Headwords = make([]string, len(c.entries))
		for i, e := range c.entries {
			v.Headwords[i] = e.Headword
		}
		v.SelectedRow = c.selectedRow
		if c.selectedRow >= 0 && c.selectedRow < len(c.entries) {
			e := c.entries[c.selectedRow]
			v.Entry = &e
			v.CopyEnabled = true
		}
		v.CountText = CountText(len(c.entries), c.maxResults)
	}
	return v
}

// unavailableLines lists the reasons both dictionaries are unavailable,
// the current (default-first) dictionary first (design D45).
func (c *Controller) unavailableLines() []string {
	lines := make([]string, 0, 2*len(dictionary.All))
	order := append([]dictionary.Type{c.dict}, c.dict.Other())
	seen := make(map[dictionary.Type]bool, len(order))
	for _, dt := range order {
		if seen[dt] {
			continue
		}
		seen[dt] = true
		st := c.status(dt)
		if st.Available {
			continue
		}
		lines = append(lines, dictionaryUnavailable(dt, st.Category))
	}
	return lines
}

func (c *Controller) logAvailable() bool {
	_, ok := c.logger.Path()
	return ok
}
