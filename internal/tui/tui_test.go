package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/rivo/uniseg"

	"github.com/simosako/ejquick/internal/builder"
	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/logging"
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
	if m.Query != "take " {
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

func TestTextInputMaintainsGraphemeCursor(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		cursor         int
		input          []string
		wantQuery      string
		wantCursor     int
		wantBackspace  string
		wantBackCursor int
	}{
		{
			name:          "combining mark in separate event",
			input:         []string{"e", "\u0301"},
			wantQuery:     "e\u0301",
			wantCursor:    1,
			wantBackspace: "",
		},
		{
			name:          "ZWJ joins surrounding emoji",
			query:         "\U0001F469\U0001F4BB",
			cursor:        1,
			input:         []string{"\u200d"},
			wantQuery:     "\U0001F469\u200d\U0001F4BB",
			wantCursor:    1,
			wantBackspace: "",
		},
		{
			name:          "regional indicators in separate events",
			input:         []string{"\U0001F1EF", "\U0001F1F5"},
			wantQuery:     "\U0001F1EF\U0001F1F5",
			wantCursor:    1,
			wantBackspace: "",
		},
		{
			name:           "multiple grapheme IME commit",
			query:          "ac",
			cursor:         1,
			input:          []string{"日本語"},
			wantQuery:      "a日本語c",
			wantCursor:     4,
			wantBackspace:  "a日本c",
			wantBackCursor: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(dictionary.Eiji, nil, nil, nil)
			m.Query = tt.query
			m.QueryCursor = tt.cursor
			for _, text := range tt.input {
				next, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: text}))
				m = modelOf(t, next)
				if m.QueryCursor < 0 || m.QueryCursor > graphemeCount(m.Query) {
					t.Fatalf("query = %q cursor = %d is out of range", m.Query, m.QueryCursor)
				}
			}
			if m.Query != tt.wantQuery || m.QueryCursor != tt.wantCursor {
				t.Fatalf("query = %q cursor = %d, want %q cursor %d",
					m.Query, m.QueryCursor, tt.wantQuery, tt.wantCursor)
			}

			m = press(t, m, "backspace")
			if m.Query != tt.wantBackspace {
				t.Errorf("after backspace query = %q, want %q", m.Query, tt.wantBackspace)
			}
			if m.QueryCursor != tt.wantBackCursor {
				t.Errorf("after backspace cursor = %d, want %d",
					m.QueryCursor, tt.wantBackCursor)
			}
		})
	}
}

func TestDeleteWordBefore(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		cursor     int
		wantQuery  string
		wantCursor int
	}{
		{
			name:       "whitespace only",
			query:      " \t\u3000",
			cursor:     3,
			wantQuery:  "",
			wantCursor: 0,
		},
		{
			name:       "leading whitespace before suffix",
			query:      "  suffix",
			cursor:     2,
			wantQuery:  "suffix",
			wantCursor: 0,
		},
		{
			name:       "word before cursor with suffix",
			query:      "take care later",
			cursor:     9,
			wantQuery:  "take  later",
			wantCursor: 5,
		},
		{
			name:       "whitespace and word before cursor",
			query:      "take \tcare",
			cursor:     6,
			wantQuery:  "care",
			wantCursor: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQuery, gotCursor, ok := deleteWordBefore(tt.query, tt.cursor)
			if !ok {
				t.Fatal("deleteWordBefore reported no change")
			}
			if gotQuery != tt.wantQuery || gotCursor != tt.wantCursor {
				t.Errorf("deleteWordBefore(%q, %d) = %q, %d; want %q, %d",
					tt.query, tt.cursor, gotQuery, gotCursor, tt.wantQuery, tt.wantCursor)
			}
		})
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

func TestDetailPagingUsesOneRowOverlapAndClamps(t *testing.T) {
	m := New(dictionary.Eiji, nil, nil, nil)
	m.Width = 80
	m.Height = 10
	m.Query = "entry"
	rightWidth := m.Width - 1 - m.leftWidth()
	m.Results = []search.Entry{{ID: 1, Headword: "entry", Body: strings.Repeat("x", rightWidth*12)}}
	m.Selected = 0

	total := m.detailBodyLineCount()
	visible := m.detailVisibleBodyRows(total)
	if total != 12 || visible != m.detailHeight()-1 {
		t.Fatalf("total = %d visible = %d detail height = %d", total, visible, m.detailHeight())
	}
	if got, want := m.detailPageStep(), visible-1; got != want {
		t.Fatalf("page step = %d, want %d", got, want)
	}

	next, cmd := m.Update(key("pgup"))
	m = modelOf(t, next)
	if cmd != nil || m.DetailOffset != 0 {
		t.Fatalf("PageUp at start returned cmd %v, offset %d", cmd != nil, m.DetailOffset)
	}
	next, cmd = m.Update(key("pgdown"))
	m = modelOf(t, next)
	if cmd != nil || m.DetailOffset != visible-1 {
		t.Fatalf("PageDown returned cmd %v, offset %d; want %d", cmd != nil, m.DetailOffset, visible-1)
	}

	for range 20 {
		m = press(t, m, "pgdown")
	}
	if want := total - visible; m.DetailOffset != want {
		t.Fatalf("offset at end = %d, want %d", m.DetailOffset, want)
	}

	next, cmd = m.Update(tea.WindowSizeMsg{Width: 80, Height: 15})
	m = modelOf(t, next)
	if cmd != nil {
		t.Fatal("resize returned a command")
	}
	total = m.detailBodyLineCount()
	visible = m.detailVisibleBodyRows(total)
	if want := total - visible; m.DetailOffset != want {
		t.Fatalf("offset after resize = %d, want %d", m.DetailOffset, want)
	}
}

func TestDetailPagingInShortPane(t *testing.T) {
	m := New(dictionary.Eiji, nil, nil, nil)
	m.Width = 80
	m.Height = 7
	m.Query = "entry"
	rightWidth := m.Width - 1 - m.leftWidth()
	m.Results = []search.Entry{{ID: 1, Headword: "entry", Body: strings.Repeat("x", rightWidth*3)}}
	m.Selected = 0

	if visible := m.detailVisibleBodyRows(m.detailBodyLineCount()); visible != 1 {
		t.Fatalf("visible body rows = %d, want 1", visible)
	}
	if step := m.detailPageStep(); step != 1 {
		t.Fatalf("page step = %d, want minimum step 1", step)
	}
	m = press(t, m, "pgdown")
	if m.DetailOffset != 1 {
		t.Fatalf("offset = %d, want 1", m.DetailOffset)
	}
}

func TestDetailPagingKeepsBodyRowWhenIndicatorCannotFit(t *testing.T) {
	m := New(dictionary.Eiji, nil, nil, nil)
	m.Width = 80
	m.Height = 6
	m.Query = "entry"
	rightWidth := m.Width - 1 - m.leftWidth()
	m.Results = []search.Entry{{
		ID:       1,
		Headword: "entry",
		Body: strings.Repeat("a", rightWidth) +
			strings.Repeat("b", rightWidth) +
			strings.Repeat("c", rightWidth),
	}}
	m.Selected = 0

	total := m.detailBodyLineCount()
	if visible := m.detailVisibleBodyRows(total); visible != 1 {
		t.Fatalf("visible body rows = %d, want 1", visible)
	}
	for range 10 {
		m = press(t, m, "pgdown")
	}
	if want := total - 1; m.DetailOffset != want {
		t.Fatalf("offset at end = %d, want %d", m.DetailOffset, want)
	}
	rows := m.renderRightRows(rightWidth)
	if len(rows) != m.paneHeight() {
		t.Fatalf("rendered rows = %d, pane height = %d", len(rows), m.paneHeight())
	}
	if rows[len(rows)-1] != strings.Repeat("c", rightWidth) {
		t.Errorf("last body row = %q", rows[len(rows)-1])
	}
	if strings.Contains(strings.Join(rows, "\n"), "PageUp/PageDown") {
		t.Errorf("indicator rendered without an available row: %q", rows)
	}
}

func TestRenderQueryRowKeepsCursorVisibleWithinWidth(t *testing.T) {
	combining := "e\u0301"
	tests := []struct {
		name       string
		query      string
		cursor     int
		wantCursor string
		wantText   string
		omitText   string
	}{
		{
			name:       "ASCII end cursor",
			query:      "START-" + strings.Repeat("x", 100) + "-END",
			cursor:     110,
			wantCursor: styleReverse + " " + styleReset,
			wantText:   "-END",
			omitText:   "START-",
		},
		{
			name:       "wide cursor cluster",
			query:      strings.Repeat("界", 50) + "語" + strings.Repeat("界", 50),
			cursor:     50,
			wantCursor: styleReverse + "語" + styleReset,
		},
		{
			name:       "combining cursor cluster",
			query:      strings.Repeat("a", 100) + combining + strings.Repeat("b", 100),
			cursor:     100,
			wantCursor: styleReverse + combining + styleReset,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(dictionary.Eiji, nil, nil, nil)
			m.Width = 80
			m.Query = tt.query
			m.QueryCursor = tt.cursor
			row := m.renderQueryRow()
			if strings.ContainsRune(row, '\n') {
				t.Fatalf("query row wrapped: %q", row)
			}
			if width := uniseg.StringWidth(stripANSI(row)); width > m.Width {
				t.Fatalf("query row width = %d, terminal width = %d", width, m.Width)
			}
			if !strings.Contains(row, tt.wantCursor) {
				t.Errorf("query row does not show cursor cluster: %q", row)
			}
			if tt.wantText != "" && !strings.Contains(row, tt.wantText) {
				t.Errorf("query row = %q, want text %q", row, tt.wantText)
			}
			if tt.omitText != "" && strings.Contains(row, tt.omitText) {
				t.Errorf("query row = %q, unexpectedly contains clipped text %q", row, tt.omitText)
			}
		})
	}
}

func TestTruncateToWidthAlwaysAddsEllipsis(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{name: "exact ASCII width", in: "abcdef", max: 6, want: "abcdef"},
		{name: "ASCII overflow", in: "abcdefg", max: 6, want: "abcde…"},
		{name: "exact wide width", in: "界界界", max: 6, want: "界界界"},
		{name: "wide overflow", in: "界界界a", max: 6, want: "界界…"},
		{name: "combining cluster", in: "e\u0301xy", max: 2, want: "e\u0301…"},
		{name: "ellipsis only", in: "界a", max: 1, want: "…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateToWidth(tt.in, tt.max)
			if got != tt.want {
				t.Errorf("truncateToWidth(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
			}
			if width := uniseg.StringWidth(got); width > tt.max {
				t.Errorf("result width = %d, max = %d", width, tt.max)
			}
		})
	}
}

func TestSearchLoggingUsesNormalizedQuery(t *testing.T) {
	t.Run("success is one DEBUG line", func(t *testing.T) {
		m := newTestModel(t)
		path := filepath.Join(t.TempDir(), "debug.log")
		logger, err := logging.OpenFile(path, true)
		if err != nil {
			t.Fatal(err)
		}
		m.logger = logger

		next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: " CARE "}))
		m = modelOf(t, next)
		drainCmd(t, m, cmd)
		if err := logger.Close(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		logText := string(data)
		if strings.Count(logText, " DEBUG ") != 1 || strings.Contains(logText, " ERROR ") {
			t.Fatalf("log = %q, want one DEBUG line", logText)
		}
		if !strings.Contains(logText, `request=1 query="care"`) || strings.Contains(logText, " CARE ") {
			t.Errorf("log does not contain only the normalized query: %q", logText)
		}
	})

	t.Run("failure is one ERROR line", func(t *testing.T) {
		m := newTestModel(t)
		if err := m.services[dictionary.Eiji].Close(); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "error.log")
		logger, err := logging.OpenFile(path, true)
		if err != nil {
			t.Fatal(err)
		}
		m.logger = logger

		next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: " CARE "}))
		m = modelOf(t, next)
		if cmd == nil {
			t.Fatal("query did not start a search")
		}
		msg := cmd()
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(before) != 0 {
			t.Fatalf("failed command logged before Update: %q", before)
		}
		m.Update(msg)
		if err := logger.Close(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		logText := string(data)
		if strings.Count(logText, " ERROR ") != 1 || strings.Contains(logText, " DEBUG ") {
			t.Fatalf("log = %q, want one ERROR line", logText)
		}
		if !strings.Contains(logText, `dict=eiji request=1 query="care"`) || strings.Contains(logText, " CARE ") {
			t.Errorf("error log does not contain only the normalized query: %q", logText)
		}
	})
}

func TestIgnoredSearchErrorsAreNotLogged(t *testing.T) {
	tests := []struct {
		name string
		msg  searchResultMsg
	}{
		{
			name: "stale error",
			msg:  searchResultMsg{requestID: 1, normalizedQuery: "care", err: errors.New("stale")},
		},
		{
			name: "current cancellation",
			msg:  searchResultMsg{requestID: 2, normalizedQuery: "care", err: context.Canceled},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ignored.log")
			logger, err := logging.OpenFile(path, true)
			if err != nil {
				t.Fatal(err)
			}
			m := New(dictionary.Eiji, nil, nil, logger)
			m.RequestID = 2
			m.Update(tt.msg)
			if err := logger.Close(); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(data) != 0 {
				t.Errorf("ignored error was logged: %q", data)
			}
		})
	}
}
