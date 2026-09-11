package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/simosako/ejquick/internal/builder"
	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/search"
)

const fixtureDict = `care : attention
careful : cautious
take care : be careful
scare : frighten
daycare : childcare
caress : touch gently
organic : natural
card : rectangular
scar : mark
`

// newTestModel builds a real fixture database and returns a model wired
// to its search service.
func newTestModel(t *testing.T) *Model {
	t.Helper()
	dir := t.TempDir()
	input := filepath.Join(dir, "in.TXT")
	if err := os.WriteFile(input, []byte(fixtureDict), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "eiji.sqlite3")
	if _, err := builder.Run(builder.Options{
		Type: dictionary.Eiji, Input: input, Output: output, Progress: os.Stderr,
	}); err != nil {
		t.Fatal(err)
	}
	repo, err := search.OpenRepository(output, dictionary.Eiji)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })
	svc, err := search.NewService(repo, 50)
	if err != nil {
		t.Fatal(err)
	}
	return New(dictionary.Eiji,
		map[dictionary.Type]*search.Service{dictionary.Eiji: svc},
		nil, nil)
}

// typeQuery sends printable runes one by one, executing each returned
// search command and feeding its result message back.
func typeQuery(t *testing.T, m *Model, s string) {
	t.Helper()
	for _, r := range s {
		next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: string(r)}))
		m = modelOf(t, next)
		drainCmd(t, m, cmd)
	}
}

// drainCmd executes a command and feeds search result messages back.
func drainCmd(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	if _, isResult := msg.(searchResultMsg); isResult {
		m.Update(msg)
	}
}

func modelOf(t *testing.T, tm tea.Model) *Model {
	t.Helper()
	m, ok := tm.(*Model)
	if !ok {
		t.Fatalf("model type %T", tm)
	}
	return m
}

// key builds a KeyPressMsg from a key name.
func key(s string) tea.KeyPressMsg {
	codes := map[string]rune{
		"down": tea.KeyDown, "up": tea.KeyUp,
		"pgdown": tea.KeyPgDown, "pgup": tea.KeyPgUp,
		"tab":  tea.KeyTab,
		"left": tea.KeyLeft, "right": tea.KeyRight,
		"backspace": tea.KeyBackspace, "delete": tea.KeyDelete,
		"home": tea.KeyHome, "end": tea.KeyEnd,
	}
	ctrls := map[string]rune{
		"ctrl+c": 'c', "ctrl+u": 'u', "ctrl+w": 'w',
		"ctrl+a": 'a', "ctrl+e": 'e',
	}
	if code, ok := codes[s]; ok {
		return tea.KeyPressMsg(tea.Key{Code: code})
	}
	if base, ok := ctrls[s]; ok {
		return tea.KeyPressMsg(tea.Key{Code: base, Mod: tea.ModCtrl})
	}
	panic("unknown key " + s)
}

// press sends a non-printable key and returns the model.
func press(t *testing.T, m *Model, k string) *Model {
	t.Helper()
	next, _ := m.Update(key(k))
	return modelOf(t, next)
}

func TestTypeToSearch(t *testing.T) {
	m := newTestModel(t)
	typeQuery(t, m, "care")
	if m.Query != "care" || m.QueryCursor != 4 {
		t.Fatalf("query = %q cursor = %d", m.Query, m.QueryCursor)
	}
	if len(m.Results) == 0 {
		t.Fatal("no results after typing")
	}
	if m.Results[0].Headword != "care" {
		t.Errorf("first = %q, want care", m.Results[0].Headword)
	}
	if m.Selected != 0 {
		t.Errorf("selected = %d", m.Selected)
	}
}

func TestCursorMovementNoSearch(t *testing.T) {
	m := newTestModel(t)
	typeQuery(t, m, "ab")
	before := m.RequestID
	for _, k := range []string{"left", "left", "right", "ctrl+a", "ctrl+e", "home", "end"} {
		next, cmd := m.Update(key(k))
		m = modelOf(t, next)
		if cmd != nil {
			t.Errorf("key %s started a search", k)
		}
	}
	if m.RequestID != before {
		t.Error("request ID changed on cursor movement")
	}
	if m.QueryCursor != 2 {
		t.Errorf("cursor = %d", m.QueryCursor)
	}
}

func TestNoOpDeleteNoSearch(t *testing.T) {
	m := newTestModel(t)
	next, _ := m.Update(key("ctrl+a"))
	m = modelOf(t, next)
	before := m.RequestID
	next, cmd := m.Update(key("backspace"))
	m = modelOf(t, next)
	if cmd != nil {
		t.Error("backspace at start started a search")
	}
	next, cmd = m.Update(key("ctrl+e"))
	m = modelOf(t, next)
	if cmd != nil {
		t.Error("ctrl+e started a search")
	}
	next, cmd = m.Update(key("delete"))
	m = modelOf(t, next)
	if cmd != nil {
		t.Error("delete at end started a search")
	}
	if m.RequestID != before {
		t.Error("request ID changed on no-op deletes")
	}
}

func TestCtrlUAndCtrlW(t *testing.T) {
	m := newTestModel(t)
	typeQuery(t, m, "take care")
	if m.Query != "take care" {
		t.Fatalf("query = %q", m.Query)
	}
	m = press(t, m, "ctrl+w")
	if m.Query != "take " && m.Query != "take" {
		t.Errorf("after ctrl+w query = %q", m.Query)
	}
	if m.QueryCursor != graphemeCount(m.Query) {
		t.Errorf("cursor = %d after word delete", m.QueryCursor)
	}
	m = press(t, m, "ctrl+u")
	if m.Query != "" || m.QueryCursor != 0 {
		t.Errorf("after ctrl+u query = %q cursor = %d", m.Query, m.QueryCursor)
	}
	if len(m.Results) != 0 || m.Selected != -1 {
		t.Error("results not cleared on ctrl+u")
	}
	if m.RequestID == 0 {
		t.Error("empty query did not update request ID")
	}
}

func TestEmojiGraphemeEditing(t *testing.T) {
	m := newTestModel(t)
	typeQuery(t, m, "ab👍")
	if m.Query != "ab👍" {
		t.Fatalf("query = %q", m.Query)
	}
	if m.QueryCursor != 3 {
		t.Fatalf("cursor = %d, want 3 graphemes", m.QueryCursor)
	}
	m = press(t, m, "backspace")
	if m.Query != "ab" {
		t.Errorf("after backspace query = %q", m.Query)
	}
}

func TestSelectionPreservedByID(t *testing.T) {
	m := newTestModel(t)
	typeQuery(t, m, "care")
	m = press(t, m, "down")
	m = press(t, m, "down")
	if m.Selected != 2 {
		t.Fatalf("selected = %d", m.Selected)
	}
	wantID := m.Results[2].ID // careful stays in the "caref" results too

	// Extend then shrink the query back: fresh results arrive, and the
	// selection must stay with the same entry id.
	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "f"}))
	m = modelOf(t, next)
	drainCmd(t, m, cmd)
	if m.Selected < 0 || m.Results[m.Selected].ID != wantID {
		t.Fatalf("after extend: selected %d id %d, want id %d",
			m.Selected, m.Results[m.Selected].ID, wantID)
	}
	next, cmd = m.Update(key("backspace"))
	m = modelOf(t, next)
	drainCmd(t, m, cmd)

	if len(m.Results) == 0 {
		t.Fatal("no results after retyping")
	}
	if m.Selected < 0 || m.Selected >= len(m.Results) {
		t.Fatalf("selection out of range: %d", m.Selected)
	}
	if m.Results[m.Selected].ID != wantID {
		t.Errorf("selection id changed: got %d want %d",
			m.Results[m.Selected].ID, wantID)
	}
}

func TestStaleResultDropped(t *testing.T) {
	m := newTestModel(t)
	// Type "a": request 1 pending.
	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "a"}))
	m = modelOf(t, next)
	staleCmd := cmd
	// Type "ab": request 2 becomes current; request 1 is canceled.
	next, cmd = m.Update(tea.KeyPressMsg(tea.Key{Text: "b"}))
	m = modelOf(t, next)
	drainCmd(t, m, cmd)
	resultsNow := len(m.Results)

	// The stale request 1 result arrives last; it must be ignored.
	if staleCmd != nil {
		m.Update(staleCmd())
	}
	if len(m.Results) != resultsNow {
		t.Error("stale result changed the visible results")
	}
}

func TestSearchErrorClearsResults(t *testing.T) {
	m := newTestModel(t)
	typeQuery(t, m, "care")
	if len(m.Results) == 0 {
		t.Fatal("setup: no results")
	}
	m.Update(searchResultMsg{requestID: m.RequestID, err: errors.New("boom")})
	if m.SearchError == "" {
		t.Fatal("search error not set")
	}
	if len(m.Results) != 0 || m.Selected != -1 {
		t.Error("results not cleared on search error")
	}
	if m.Query != "care" {
		t.Error("query lost on search error")
	}
}

func TestSearchCanceledIgnored(t *testing.T) {
	m := newTestModel(t)
	typeQuery(t, m, "care")
	m.Update(searchResultMsg{requestID: m.RequestID, err: context.Canceled})
	if m.SearchError != "" {
		t.Error("context.Canceled surfaced as an error")
	}
}

func TestTabWithSingleDictionary(t *testing.T) {
	m := newTestModel(t)
	typeQuery(t, m, "care")
	results := len(m.Results)
	next, cmd := m.Update(key("tab"))
	m = modelOf(t, next)
	if cmd != nil {
		t.Error("tab started a search")
	}
	if m.Dictionary != dictionary.Eiji {
		t.Error("dictionary switched with only one available")
	}
	if len(m.Results) != results {
		t.Error("tab cleared results with one dictionary")
	}
	if m.StatusWarning == "" {
		t.Error("missing warning for unavailable dictionary")
	}
}

func TestGuideHidesTabWithOneDictionary(t *testing.T) {
	m := newTestModel(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = modelOf(t, next)
	v := m.View().Content
	if strings.Contains(v, "Tab") {
		t.Error("tab hint shown with a single dictionary")
	}
	if !strings.Contains(v, "Type to search") {
		t.Error("guide missing")
	}
}

func TestViewContainsPanesAndStatus(t *testing.T) {
	m := newTestModel(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = modelOf(t, next)
	v := m.View().Content
	for _, want := range []string{"Type to search", "EIJI", ">"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q", want)
		}
	}
	if !strings.Contains(v, "|") {
		t.Error("view missing pane separator")
	}
}

func TestNarrowTerminalWarning(t *testing.T) {
	m := newTestModel(t)
	typeQuery(t, m, "care")
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 24})
	m = modelOf(t, next)
	v := m.View().Content
	if !strings.Contains(v, "too narrow") {
		t.Errorf("narrow view missing warning: %q", v)
	}
	if m.Query != "care" {
		t.Error("query lost during narrow mode")
	}
	next, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = modelOf(t, next)
	if v := m.View().Content; !strings.Contains(v, "care") {
		t.Errorf("results not restored after widening: %q", v)
	}
}

func TestEmptyQuerySkipsDatabase(t *testing.T) {
	m := newTestModel(t)
	// A space normalizes to empty: no search may run.
	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: " "}))
	m = modelOf(t, next)
	if cmd != nil {
		t.Error("whitespace query started a search")
	}
}
