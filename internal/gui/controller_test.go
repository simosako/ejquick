package gui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/logging"
	"github.com/simosako/ejquick/internal/search"
)

// fakeSearcher is a deterministic Searcher for controller tests.
type fakeSearcher struct {
	mu      sync.Mutex
	queries []string
	results map[string][]search.Entry
	err     error
	// block, when non-nil, parks Search until the channel is closed or
	// the context is done.
	block chan struct{}
	// closed counts Close calls.
	closed int
}

func (f *fakeSearcher) Search(ctx context.Context, rawQuery string) ([]search.Entry, error) {
	f.mu.Lock()
	f.queries = append(f.queries, rawQuery)
	entries := f.results[rawQuery]
	err := f.err
	block := f.block
	f.mu.Unlock()
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (f *fakeSearcher) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed++
	return nil
}

func (f *fakeSearcher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.queries)
}

func entries(ids ...int64) []search.Entry {
	out := make([]search.Entry, len(ids))
	for i, id := range ids {
		out[i] = search.Entry{ID: id, Headword: strings.Repeat("h", int(id)), Body: "body"}
	}
	return out
}

func bothAvailable(eiwa, waei Searcher) (map[dictionary.Type]Searcher, map[dictionary.Type]DictStatus) {
	services := map[dictionary.Type]Searcher{}
	statuses := map[dictionary.Type]DictStatus{}
	if eiwa != nil {
		services[dictionary.Eiwa] = eiwa
		statuses[dictionary.Eiwa] = DictStatus{Available: true}
	} else {
		statuses[dictionary.Eiwa] = DictStatus{Available: false, Category: search.OpenMissing}
	}
	if waei != nil {
		services[dictionary.Waei] = waei
		statuses[dictionary.Waei] = DictStatus{Available: true}
	} else {
		statuses[dictionary.Waei] = DictStatus{Available: false, Category: search.OpenMissing}
	}
	return services, statuses
}

// newTestController wires synchronous worker execution and synchronous
// event delivery so tests are deterministic.
func newTestController(t *testing.T, c *Controller) *Controller {
	t.Helper()
	c.runWorker = func(f func()) { f() }
	c.post = func(ev UIEvent) { c.ApplyEvent(ev) }
	return c
}

func TestControllerInitialView(t *testing.T) {
	eiwa := &fakeSearcher{}
	waei := &fakeSearcher{}
	services, statuses := bothAvailable(eiwa, waei)
	c := newTestController(t, NewController(ControllerConfig{
		Services: services, Statuses: statuses,
		Initial: dictionary.Eiwa, MaxResults: 50, Logger: logging.Nop(),
	}))

	v := c.View()
	if v.Dictionary != dictionary.Eiwa {
		t.Errorf("dictionary = %v, want eiwa", v.Dictionary)
	}
	if !v.SearchEnabled || !v.DictionaryEnabled || !v.SwitchEnabled {
		t.Errorf("enabled flags: search=%v dict=%v switch=%v, want all true",
			v.SearchEnabled, v.DictionaryEnabled, v.SwitchEnabled)
	}
	if v.Page != pageMessage || len(v.Message.Lines) == 0 {
		t.Fatalf("initial page = %v, want message page with guidance", v.Page)
	}
	if v.Message.Lines[0] != "Enter a search term" {
		t.Errorf("guidance first line = %q", v.Message.Lines[0])
	}
	if v.SelectedRow != -1 || v.Entry != nil || v.CountText != "" || v.CopyEnabled {
		t.Errorf("initial view should be empty: %+v", v)
	}
	if len(v.DictionaryItems) != 2 || !v.DictionaryItems[0].Enabled || !v.DictionaryItems[1].Enabled {
		t.Errorf("dictionary items = %+v", v.DictionaryItems)
	}
	if v.DictionaryItems[0].Type != dictionary.Eiwa || v.DictionaryItems[1].Type != dictionary.Waei {
		t.Errorf("dictionary item order = %+v", v.DictionaryItems)
	}
}

func TestControllerEmptyQueryIssuesNoSearch(t *testing.T) {
	eiwa := &fakeSearcher{}
	services, statuses := bothAvailable(eiwa, nil)
	c := newTestController(t, NewController(ControllerConfig{
		Services: services, Statuses: statuses,
		Initial: dictionary.Eiwa, MaxResults: 50, Logger: logging.Nop(),
	}))
	for _, q := range []string{"", " ", "　"} { // ASCII space, ideographic space
		c.SetQuery(q)
	}
	if eiwa.callCount() != 0 {
		t.Errorf("searcher called %d times for empty queries", eiwa.callCount())
	}
	if v := c.View(); v.Page != pageMessage || len(v.Headwords) != 0 {
		t.Errorf("view after empty query = %+v", v)
	}
}

func TestControllerSearchResultsView(t *testing.T) {
	eiwa := &fakeSearcher{results: map[string][]search.Entry{
		"care": {
			{ID: 1, Headword: "care", Body: "attention"},
			{ID: 2, Headword: "career", Body: "profession"},
		},
	}}
	services, statuses := bothAvailable(eiwa, nil)
	c := newTestController(t, NewController(ControllerConfig{
		Services: services, Statuses: statuses,
		Initial: dictionary.Eiwa, MaxResults: 50, Logger: logging.Nop(),
	}))

	c.SetQuery("care")
	v := c.View()
	if v.Page != pageDetail {
		t.Fatalf("page = %v, want detail", v.Page)
	}
	if strings.Join(v.Headwords, ",") != "care,career" {
		t.Errorf("headwords = %v", v.Headwords)
	}
	if v.SelectedRow != 0 || v.Entry == nil || v.Entry.Headword != "care" {
		t.Errorf("selection = %+v", v.Entry)
	}
	if v.CountText != "2 shown" {
		t.Errorf("count = %q, want %q", v.CountText, "2 shown")
	}
	if !v.CopyEnabled {
		t.Error("copy should be enabled with a selection")
	}
	text, ok := c.CopySelection()
	if !ok || text != "care\nattention" {
		t.Errorf("copy text = %q ok=%v", text, ok)
	}
}

func TestControllerNoResultsAndErrorStates(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "ejquick.log")
	logger, err := logging.OpenFile(logPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	eiwa := &fakeSearcher{}
	services, statuses := bothAvailable(eiwa, nil)
	c := newTestController(t, NewController(ControllerConfig{
		Services: services, Statuses: statuses,
		Initial: dictionary.Eiwa, MaxResults: 50, Logger: logger,
	}))

	c.SetQuery("zzz")
	v := c.View()
	if v.Page != pageMessage || v.Message.Lines[0] != "No matching headwords" {
		t.Errorf("no-result view = %+v", v)
	}
	if v.CountText != "" || v.CopyEnabled {
		t.Errorf("no-result view should hide count and copy: %+v", v)
	}

	eiwa.mu.Lock()
	eiwa.err = context.DeadlineExceeded
	eiwa.mu.Unlock()
	c.SetQuery("boom")
	v = c.View()
	if v.Page != pageMessage {
		t.Fatalf("error page = %v", v.Page)
	}
	want := []string{"Search failed", "Edit the query to try again"}
	if strings.Join(v.Message.Lines, "|") != strings.Join(want, "|") {
		t.Errorf("error message = %v", v.Message.Lines)
	}
	if !v.Message.ShowOpenLog {
		t.Error("error message should offer Open Log when logging to a file")
	}
}

func TestControllerErrorLogOmitsQuery(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "ejquick.log")
	logger, err := logging.OpenFile(logPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	eiwa := &fakeSearcher{err: os.ErrDeadlineExceeded}
	services, statuses := bothAvailable(eiwa, nil)
	c := newTestController(t, NewController(ControllerConfig{
		Services: services, Statuses: statuses,
		Initial: dictionary.Eiwa, MaxResults: 50, Logger: logger,
	}))
	c.SetQuery("secret query")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret query") {
		t.Errorf("error log leaked the query: %s", data)
	}
	if !strings.Contains(string(data), "request=") {
		t.Errorf("error log missing request id: %s", data)
	}
}

func TestControllerSelectionPreservedByID(t *testing.T) {
	eiwa := &fakeSearcher{results: map[string][]search.Entry{
		"care": {
			{ID: 1, Headword: "care", Body: "b1"},
			{ID: 2, Headword: "career", Body: "b2"},
			{ID: 3, Headword: "careful", Body: "b3"},
		},
		"caref": {
			{ID: 3, Headword: "careful", Body: "b3"},
			{ID: 4, Headword: "carefree", Body: "b4"},
		},
	}}
	services, statuses := bothAvailable(eiwa, nil)
	c := newTestController(t, NewController(ControllerConfig{
		Services: services, Statuses: statuses,
		Initial: dictionary.Eiwa, MaxResults: 50, Logger: logging.Nop(),
	}))
	c.SetQuery("care")
	c.MoveSelection(1) // select career (ID 2)
	if got := c.View().SelectedRow; got != 1 {
		t.Fatalf("row = %d, want 1", got)
	}
	c.SetQuery("caref")
	v := c.View()
	if v.SelectedRow != -1 {
		// ID 2 is not in the new result; the first row is selected.
		if v.SelectedRow != 0 || v.Entry.ID != 3 {
			t.Errorf("selection = row %d entry %+v, want row 0 id 3", v.SelectedRow, v.Entry)
		}
	} else if v.Entry == nil || v.Entry.ID != 4 {
		t.Errorf("unexpected fallback selection: %+v", v.Entry)
	}
}

func TestControllerStaleResultsDiscarded(t *testing.T) {
	eiwa := &fakeSearcher{results: map[string][]search.Entry{
		"a":  {{ID: 1, Headword: "a1", Body: "x"}},
		"ab": {{ID: 2, Headword: "ab1", Body: "x"}},
	}}
	services, statuses := bothAvailable(eiwa, nil)
	c := NewController(ControllerConfig{
		Services: services, Statuses: statuses,
		Initial: dictionary.Eiwa, MaxResults: 50, Logger: logging.Nop(),
	})
	var pending []func()
	var posted []UIEvent
	c.runWorker = func(f func()) { pending = append(pending, f) }
	c.post = func(ev UIEvent) { posted = append(posted, ev) }

	c.SetQuery("a")  // request 1
	c.SetQuery("ab") // request 2 supersedes it
	if len(pending) != 2 {
		t.Fatalf("pending workers = %d, want 2", len(pending))
	}
	pending[0]() // stale result arrives first
	c.ApplyEvent(posted[0])
	if v := c.View(); v.Page == pageDetail || len(v.Headwords) != 0 {
		t.Errorf("stale result applied: %+v", v)
	}
	pending[1]()
	c.ApplyEvent(posted[1])
	v := c.View()
	if v.Page != pageDetail || len(v.Headwords) != 1 || v.Entry.ID != 2 {
		t.Errorf("current result not applied: %+v", v)
	}
}

func TestControllerCancelOnNewQuery(t *testing.T) {
	eiwa := &fakeSearcher{results: map[string][]search.Entry{
		"a":  {{ID: 1, Headword: "a1", Body: "x"}},
		"ab": {{ID: 2, Headword: "ab1", Body: "x"}},
	}}
	eiwa.mu.Lock()
	eiwa.block = make(chan struct{})
	eiwa.mu.Unlock()
	defer func() {
		eiwa.mu.Lock()
		eiwa.block = nil
		eiwa.mu.Unlock()
	}()

	services, statuses := bothAvailable(eiwa, nil)
	c := NewController(ControllerConfig{
		Services: services, Statuses: statuses,
		Initial: dictionary.Eiwa, MaxResults: 50, Logger: logging.Nop(),
	})
	events := make(chan UIEvent, 4)
	var wg sync.WaitGroup
	c.runWorker = func(f func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f()
		}()
	}
	c.post = func(ev UIEvent) { events <- ev }

	c.SetQuery("a")
	c.SetQuery("ab") // cancels the first request
	close(eiwa.block)
	wg.Wait()

	// The canceled request reports context.Canceled and is discarded;
	// the second request applies.
	for len(events) > 0 {
		c.ApplyEvent(<-events)
	}
	v := c.View()
	if v.Page != pageDetail || len(v.Headwords) != 1 || v.Entry.ID != 2 {
		t.Errorf("current result not applied after cancel: %+v", v)
	}
}

func TestControllerDictionarySwitchClearsState(t *testing.T) {
	eiwa := &fakeSearcher{results: map[string][]search.Entry{
		"a": {{ID: 1, Headword: "a1", Body: "x"}},
	}}
	waei := &fakeSearcher{results: map[string][]search.Entry{
		"a": {{ID: 9, Headword: "あ", Body: "y"}},
	}}
	services, statuses := bothAvailable(eiwa, waei)
	c := newTestController(t, NewController(ControllerConfig{
		Services: services, Statuses: statuses,
		Initial: dictionary.Eiwa, MaxResults: 50, Logger: logging.Nop(),
	}))

	c.SetQuery("a")
	if v := c.View(); v.Page != pageDetail {
		t.Fatal("setup: no result")
	}
	c.SwitchDictionary()
	v := c.View()
	if v.Dictionary != dictionary.Waei {
		t.Errorf("dictionary = %v, want waei", v.Dictionary)
	}
	if v.Query != "" || v.Page != pageMessage || len(v.Headwords) != 0 || v.SelectedRow != -1 {
		t.Errorf("state not cleared after switch: %+v", v)
	}
	if waei.callCount() != 0 {
		t.Errorf("switch triggered %d searches, want 0", waei.callCount())
	}
	// Switching back is a no-op when the target is the current dict.
	c.SwitchToDictionary(dictionary.Waei)
	if c.View().Dictionary != dictionary.Waei {
		t.Error("switch to current dictionary changed state")
	}
}

func TestControllerSwitchToUnavailableIgnored(t *testing.T) {
	eiwa := &fakeSearcher{}
	services, statuses := bothAvailable(eiwa, nil)
	c := newTestController(t, NewController(ControllerConfig{
		Services: services, Statuses: statuses,
		Initial: dictionary.Eiwa, MaxResults: 50, Logger: logging.Nop(),
	}))
	c.SwitchToDictionary(dictionary.Waei)
	if c.View().Dictionary != dictionary.Eiwa {
		t.Error("switched to an unavailable dictionary")
	}
	if c.View().SwitchEnabled {
		t.Error("SwitchEnabled with a single available dictionary")
	}
	// The combo is disabled with a single available dictionary but
	// search still works (design D9).
	v := c.View()
	if v.DictionaryEnabled {
		t.Error("DictionaryEnabled with a single available dictionary")
	}
	if !v.SearchEnabled {
		t.Error("search should stay enabled")
	}
	// The unavailable dictionary carries its canonical reason as the
	// item tooltip (design D39/D45).
	if !strings.Contains(v.DictionaryItems[1].Tooltip, "was not found") {
		t.Errorf("waei tooltip = %q", v.DictionaryItems[1].Tooltip)
	}
}

func TestControllerMoveSelectionBounds(t *testing.T) {
	eiwa := &fakeSearcher{results: map[string][]search.Entry{
		"a": entries(1, 2, 3),
	}}
	services, statuses := bothAvailable(eiwa, nil)
	c := newTestController(t, NewController(ControllerConfig{
		Services: services, Statuses: statuses,
		Initial: dictionary.Eiwa, MaxResults: 50, Logger: logging.Nop(),
	}))
	c.SetQuery("a")
	if c.MoveSelection(-1) {
		t.Error("MoveSelection(-1) from row 0 should report no movement")
	}
	if got := c.View().SelectedRow; got != 0 {
		t.Errorf("row after up at top = %d, want 0", got)
	}
	if !c.MoveSelection(1) {
		t.Error("MoveSelection(1) should move")
	}
	c.MoveSelection(5) // stops at the last row, no wrap
	if got := c.View().SelectedRow; got != 2 {
		t.Errorf("row after overshoot = %d, want 2", got)
	}
	if c.MoveSelection(1) {
		t.Error("MoveSelection(1) at the end should not move")
	}
	c.MoveSelection(-2)
	if got := c.View().SelectedRow; got != 0 {
		t.Errorf("row = %d, want 0", got)
	}
}

func TestControllerClearSearch(t *testing.T) {
	eiwa := &fakeSearcher{results: map[string][]search.Entry{
		"a": entries(1),
	}}
	services, statuses := bothAvailable(eiwa, nil)
	c := newTestController(t, NewController(ControllerConfig{
		Services: services, Statuses: statuses,
		Initial: dictionary.Eiwa, MaxResults: 50, Logger: logging.Nop(),
	}))
	c.SetQuery("a")
	c.ClearSearch()
	v := c.View()
	if v.Query != "" || v.Page != pageMessage || v.CountText != "" || v.SelectedRow != -1 {
		t.Errorf("clear search left state: %+v", v)
	}
}

func TestControllerCountAtLimit(t *testing.T) {
	many := make([]search.Entry, 50)
	for i := range many {
		many[i] = search.Entry{ID: int64(i + 1), Headword: "h", Body: "b"}
	}
	eiwa := &fakeSearcher{results: map[string][]search.Entry{"a": many}}
	services, statuses := bothAvailable(eiwa, nil)
	c := newTestController(t, NewController(ControllerConfig{
		Services: services, Statuses: statuses,
		Initial: dictionary.Eiwa, MaxResults: 50, Logger: logging.Nop(),
	}))
	c.SetQuery("a")
	if got := c.View().CountText; got != "Showing first 50" {
		t.Errorf("count at limit = %q", got)
	}
}

func TestControllerBothUnavailable(t *testing.T) {
	statuses := map[dictionary.Type]DictStatus{
		dictionary.Eiwa: {Available: false, Category: search.OpenMissing},
		dictionary.Waei: {Available: false, Category: search.OpenCorrupt},
	}
	c := newTestController(t, NewController(ControllerConfig{
		Services: map[dictionary.Type]Searcher{}, Statuses: statuses,
		Initial: dictionary.Eiwa, MaxResults: 50, Logger: logging.Nop(),
	}))
	v := c.View()
	if v.SearchEnabled || v.DictionaryEnabled || v.SwitchEnabled {
		t.Errorf("controls should be disabled: %+v", v)
	}
	if len(v.Message.Lines) != 2 {
		t.Fatalf("message lines = %q, want both dictionaries", v.Message.Lines)
	}
	if !strings.Contains(v.Message.Lines[0], "English–Japanese") ||
		!strings.Contains(v.Message.Lines[0], "was not found") {
		t.Errorf("first line = %q, want current (default) dictionary first", v.Message.Lines[0])
	}
	if !strings.Contains(v.Message.Lines[1], "Japanese–English") ||
		!strings.Contains(v.Message.Lines[1], "damaged") {
		t.Errorf("second line = %q", v.Message.Lines[1])
	}
	// The per-item tooltips carry the same canonical text.
	for _, item := range v.DictionaryItems {
		if item.Enabled || item.Tooltip == "" {
			t.Errorf("item %+v should be disabled with a reason", item)
		}
	}
}

func TestControllerShutdownAndFinalize(t *testing.T) {
	eiwa := &fakeSearcher{}
	waei := &fakeSearcher{}
	services, statuses := bothAvailable(eiwa, waei)
	c := newTestController(t, NewController(ControllerConfig{
		Services: services, Statuses: statuses,
		Initial: dictionary.Eiwa, MaxResults: 50, Logger: logging.Nop(),
	}))
	c.SetQuery("a") // fills some state
	c.BeginShutdown()
	if c.View().SearchEnabled == false && c.View().Page != pageMessage {
		// BeginShutdown does not change the view; queries are just
		// rejected from now on.
		t.Log("state after shutdown request is unchanged")
	}
	before := eiwa.callCount()
	c.SetQuery("b")
	if eiwa.callCount() != before {
		t.Error("SetQuery after shutdown started a search")
	}
	c.ApplyEvent(SearchDoneEvent{RequestID: 1, Dictionary: dictionary.Eiwa, Entries: entries(1)})
	c.Finalize()
	if eiwa.closed != 1 || waei.closed != 1 {
		t.Errorf("close calls: eiwa=%d waei=%d, want 1 each", eiwa.closed, waei.closed)
	}
	c.Finalize() // idempotent
	if eiwa.closed != 1 {
		t.Errorf("second Finalize closed again: %d", eiwa.closed)
	}
}
