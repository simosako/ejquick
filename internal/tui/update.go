package tui

import (
	"context"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/rivo/uniseg"

	"github.com/simosako/ejquick/internal/normalize"
)

// keyPress aliases keep the update loop readable.
type keyMsg = tea.KeyPressMsg

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = int(msg.Width)
		m.Height = int(msg.Height)
		// Re-clamp scroll positions for the new pane sizes; results
		// themselves do not change.
		m.ensureSelectionVisible()
		m.clampDetailOffset()
		return m, nil

	case searchResultMsg:
		if msg.requestID != m.RequestID {
			// Stale result or stale error: drop silently.
			return m, nil
		}
		if msg.err != nil {
			if ctxCanceled(msg.err) {
				return m, nil
			}
			// Log the full detail once for the current request only.
			m.logger.Error("search dict=%s request=%d query=%q: %v",
				m.Dictionary, msg.requestID, msg.normalizedQuery, msg.err)
			m.SearchError = msg.err.Error()
			m.clearResults()
			return m, nil
		}
		m.SearchError = ""
		m.applyResults(msg.entries)
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// handleKey dispatches one key press. Query-changing keys return a new
// search command; movement keys never do.
func (m *Model) handleKey(msg keyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		if m.CancelSearch != nil {
			m.CancelSearch()
		}
		return m, tea.Quit

	case "tab":
		return m, m.switchDictionary()

	// Query cursor movement (no search).
	case "left":
		if m.QueryCursor > 0 {
			m.QueryCursor--
		}
		return m, nil
	case "right":
		if m.QueryCursor < graphemeCount(m.Query) {
			m.QueryCursor++
		}
		return m, nil
	case "home", "ctrl+a":
		m.QueryCursor = 0
		return m, nil
	case "end", "ctrl+e":
		m.QueryCursor = graphemeCount(m.Query)
		return m, nil

	// Query edits (search when the content really changes).
	case "backspace":
		if q, ok := deleteBeforeCursor(m.Query, m.QueryCursor); ok {
			m.Query = q
			m.QueryCursor--
			return m, m.startSearchIfChanged()
		}
		return m, nil
	case "delete":
		if q, ok := deleteAfterCursor(m.Query, m.QueryCursor); ok {
			m.Query = q
			return m, m.startSearchIfChanged()
		}
		return m, nil
	case "ctrl+w":
		if q, c, ok := deleteWordBefore(m.Query, m.QueryCursor); ok {
			m.Query = q
			m.QueryCursor = c
			return m, m.startSearchIfChanged()
		}
		return m, nil
	case "ctrl+u":
		if m.Query != "" {
			m.Query = ""
			m.QueryCursor = 0
			return m, m.startSearchIfChanged()
		}
		return m, nil

	// Selection movement (no DB query).
	case "up", "ctrl+p":
		if m.Selected > 0 {
			m.Selected--
			m.selectedID = m.Results[m.Selected].ID
			m.DetailOffset = 0
			m.ensureSelectionVisible()
		}
		return m, nil
	case "down", "ctrl+n":
		if m.Selected >= 0 && m.Selected < len(m.Results)-1 {
			m.Selected++
			m.selectedID = m.Results[m.Selected].ID
			m.DetailOffset = 0
			m.ensureSelectionVisible()
		}
		return m, nil

	// Detail pane paging (no query change, no focus change).
	case "pgup":
		m.DetailOffset -= m.detailPageStep()
		m.clampDetailOffset()
		return m, nil
	case "pgdown":
		m.DetailOffset += m.detailPageStep()
		m.clampDetailOffset()
		return m, nil
	}

	// Text input inserts at the cursor. Text is populated only for
	// printable characters, which is exactly what belongs in a query.
	if msg.Text != "" {
		m.Query, m.QueryCursor = insertText(m.Query, m.QueryCursor, msg.Text)
		return m, m.startSearchIfChanged()
	}
	return m, nil
}

// lastSearchedQuery remembers the query string of the current request so
// no-op edits do not restart a search.
func (m *Model) startSearchIfChanged() tea.Cmd {
	normQuery, _ := normalize.Normalize(m.Dictionary, m.Query)
	if normQuery == "" {
		return m.startSearch(m.Query, "")
	}
	// Any content change restarts: identical strings would only occur
	// for pure whitespace/normalization rewrites, and those clear the
	// view anyway. Re-running the search is harmless and keeps the logic
	// simple.
	return m.startSearch(m.Query, normQuery)
}

// ctxCanceled reports whether err wraps context.Canceled.
func ctxCanceled(err error) bool {
	for err != nil {
		if err == context.Canceled {
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// --- grapheme cluster editing helpers ---

// graphemeCount returns the number of grapheme clusters in s.
func graphemeCount(s string) int {
	if s == "" {
		return 0
	}
	return len(splitGraphemes(s))
}

// splitGraphemes splits s into grapheme clusters.
func splitGraphemes(s string) []string {
	var out []string
	state := -1
	for len(s) > 0 {
		var cluster string
		cluster, _, _, state = uniseg.FirstGraphemeClusterInString(s, state)
		out = append(out, cluster)
		s = s[len(cluster):]
	}
	return out
}

// insertText inserts text at grapheme index i and returns the cursor
// position after re-segmenting the complete result.
func insertText(s string, i int, text string) (string, int) {
	gs := splitGraphemes(s)
	if i < 0 {
		i = 0
	}
	if i > len(gs) {
		i = len(gs)
	}
	prefix := joinGraphemes(gs[:i])
	out := prefix + text + joinGraphemes(gs[i:])
	cursorByte := len(prefix) + len(text)
	consumed := 0
	cursor := 0
	for _, g := range splitGraphemes(out) {
		if cursorByte == 0 {
			break
		}
		consumed += len(g)
		cursor++
		if consumed >= cursorByte {
			break
		}
	}
	return out, cursor
}

// deleteBeforeCursor removes the grapheme cluster before index i.
func deleteBeforeCursor(s string, i int) (string, bool) {
	if i <= 0 {
		return s, false
	}
	gs := splitGraphemes(s)
	return joinGraphemes(append(gs[:i-1], gs[i:]...)), true
}

// deleteAfterCursor removes the grapheme cluster at index i.
func deleteAfterCursor(s string, i int) (string, bool) {
	gs := splitGraphemes(s)
	if i >= len(gs) {
		return s, false
	}
	return joinGraphemes(append(gs[:i], gs[i+1:]...)), true
}

// deleteWordBefore removes preceding Unicode whitespace clusters and
// then the preceding non-whitespace run, per design 10.3.
func deleteWordBefore(s string, i int) (string, int, bool) {
	gs := splitGraphemes(s)
	if i <= 0 {
		return s, i, false
	}
	j := i
	for j > 0 && isWhitespaceGrapheme(gs[j-1]) {
		j--
	}
	if j == 0 {
		// Only whitespace before the cursor: delete just that.
		return joinGraphemes(gs[i:]), j, true
	}
	for j > 0 && !isWhitespaceGrapheme(gs[j-1]) {
		j--
	}
	return joinGraphemes(append(gs[:j], gs[i:]...)), j, true
}

// joinGraphemes concatenates clusters.
func joinGraphemes(gs []string) string {
	out := ""
	for _, g := range gs {
		out += g
	}
	return out
}

// isWhitespaceGrapheme reports whether a cluster is Unicode whitespace.
func isWhitespaceGrapheme(g string) bool {
	for _, r := range g {
		return unicode.IsSpace(r)
	}
	return false
}
