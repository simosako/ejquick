package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/rivo/uniseg"

	"github.com/simosako/ejquick/internal/search"
)

// Layout constants from the design.
const (
	minColumns     = 80
	leftMinWidth   = 24
	leftMaxWidth   = 50
	scrollOverlap  = 1 // one overlapping row between pages
)

// Styling: only reverse, bold, and dim on the terminal's default colors.
var (
	styleReverse = "\x1b[7m"
	styleBold    = "\x1b[1m"
	styleDim     = "\x1b[2m"
	styleReset   = "\x1b[0m"
)

// View implements tea.Model.
func (m *Model) View() tea.View {
	if m.Width < minColumns {
		return tea.NewView(m.narrowView())
	}
	var b strings.Builder

	panes := m.renderPanes()
	for i, row := range panes {
		b.WriteString(row)
		if i < len(panes)-1 {
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')
	b.WriteString(m.renderSeparator())
	b.WriteByte('\n')
	b.WriteString(m.renderStatus())
	b.WriteByte('\n')
	b.WriteString(m.renderQueryRow())
	return tea.NewView(b.String())
}

// narrowView replaces the two panes with a warning when the terminal is
// narrower than the minimum width; state is kept for restoration. The
// view is padded to the full height so no previous frame remains.
func (m *Model) narrowView() string {
	var b strings.Builder
	b.WriteString(m.bold("Terminal is too narrow") + "\n\n")
	fmt.Fprintf(&b, "EJQuick needs at least %d columns; current width is %d.\n\n", minColumns, m.Width)
	b.WriteString(m.dim("Query and results are preserved. Widen the window to continue."))
	for i := 4; i < m.Height; i++ {
		b.WriteByte('\n')
	}
	return b.String()
}

// paneHeight returns the number of rows available for the panes: total
// height minus the separator, status, and query rows.
func (m *Model) paneHeight() int {
	h := m.Height - 3
	if h < 1 {
		h = 1
	}
	return h
}

// listHeight returns the visible row count of the left pane.
func (m *Model) listHeight() int { return m.paneHeight() }

// leftWidth computes the left pane width: about a third of the usable
// width, clamped to 24..50.
func (m *Model) leftWidth() int {
	w := (m.Width - 1) / 3
	if w < leftMinWidth {
		w = leftMinWidth
	}
	if w > leftMaxWidth {
		w = leftMaxWidth
	}
	return w
}

// detailPageStep is one PageUp/PageDown step: page height minus one
// overlapping row.
func (m *Model) detailPageStep() int {
	h := m.detailHeight() - scrollOverlap
	if h < 1 {
		h = 1
	}
	return h
}

// detailHeight is the body height of the right pane: pane height minus
// the headword row and its blank separator.
func (m *Model) detailHeight() int {
	h := m.paneHeight() - 2
	if h < 1 {
		h = 1
	}
	return h
}

// renderPanes draws the two-pane area row by row.
func (m *Model) renderPanes() []string {
	rows := make([]string, m.paneHeight())
	lw := m.leftWidth()
	rw := m.Width - 1 - lw

	left := m.renderLeftRows()
	right := m.renderRightRows(rw)

	for i := 0; i < len(rows); i++ {
		l := ""
		if i < len(left) {
			l = left[i]
		}
		r := ""
		if i < len(right) {
			r = right[i]
		}
		rows[i] = padRight(l, lw) + "|" + r
	}
	return rows
}

// renderLeftRows renders the visible slice of the result list.
func (m *Model) renderLeftRows() []string {
	lw := m.leftWidth()
	visible := m.listHeight()
	if len(m.Results) == 0 {
		return nil
	}
	start := m.ListOffset
	if start < 0 {
		start = 0
	}
	if start > len(m.Results) {
		start = len(m.Results)
	}
	end := start + visible
	if end > len(m.Results) {
		end = len(m.Results)
	}
	rows := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		e := m.Results[i]
		head := stripMarker(e.Headword)
		text := truncateToWidth(head, lw-2)
		if i == m.Selected {
			rows = append(rows, styleReverse+"> "+text+padSpaces(text, lw-2)+styleReset)
		} else {
			rows = append(rows, "  "+text)
		}
	}
	return rows
}

// renderRightRows renders the detail pane: headword, blank, wrapped body,
// and a scroll indicator when the body overflows. The indicator takes one
// row from the body window so it always fits the pane height.
func (m *Model) renderRightRows(w int) []string {
	if m.SearchError != "" {
		return []string{m.bold("Search error: " + firstLine(m.SearchError))}
	}
	if m.Query == "" {
		return m.guideRows(w)
	}
	if len(m.Results) == 0 {
		return []string{m.dim("No results")}
	}
	if m.Selected < 0 || m.Selected >= len(m.Results) {
		return nil
	}
	e := m.Results[m.Selected]

	head := truncateToWidth(stripMarker(e.Headword), w)
	bodyLines := wrapToWidth(e.Body, w)
	total := len(bodyLines)

	// Body window; reserve one row for the scroll indicator when needed.
	avail := m.detailHeight()
	needIndicator := total > avail
	if needIndicator {
		avail--
	}
	off := m.DetailOffset
	if off > total {
		off = total
	}
	end := off + avail
	if end > total {
		end = total
	}

	rows := make([]string, 0, m.paneHeight())
	rows = append(rows, m.bold(head))
	rows = append(rows, "")
	for _, l := range bodyLines[off:end] {
		rows = append(rows, l)
	}
	if needIndicator {
		rows = append(rows, m.dim(fmt.Sprintf("[%d/%d] PageUp/PageDown to scroll", off+1, total)))
	}
	return rows
}

// guideRows renders the static empty-query guide. The Tab hint is hidden
// when only one dictionary is available.
func (m *Model) guideRows(w int) []string {
	rows := []string{m.dim("Type to search")}
	if len(m.Available) >= 2 {
		rows = append(rows, m.dim("Tab: switch dictionary"))
	}
	rows = append(rows, m.dim("Ctrl-C: quit"))
	_ = w
	return rows
}

// renderSeparator draws the dim horizontal rule above the status line.
func (m *Model) renderSeparator() string {
	return m.dim(strings.Repeat("─", m.Width))
}

// renderStatus shows the dictionary label, or a transient warning.
func (m *Model) renderStatus() string {
	if m.StatusWarning != "" {
		return m.bold(m.StatusWarning)
	}
	return m.Dictionary.Label()
}

// renderQueryRow draws the prompt, query text, and cursor.
func (m *Model) renderQueryRow() string {
	gs := splitGraphemes(m.Query)
	var b strings.Builder
	b.WriteString("> ")
	for i, g := range gs {
		if i == m.QueryCursor {
			b.WriteString(styleReverse + g + styleReset)
		} else {
			b.WriteString(g)
		}
	}
	if m.QueryCursor >= len(gs) {
		b.WriteString(styleReverse + " " + styleReset)
	}
	return b.String()
}

// clampDetailOffset keeps the detail scroll position valid, reserving one
// row for the scroll indicator exactly like the renderer does.
func (m *Model) clampDetailOffset() {
	if m.DetailOffset < 0 {
		m.DetailOffset = 0
	}
	if m.Selected < 0 || m.Selected >= len(m.Results) {
		return
	}
	body := wrapToWidth(m.Results[m.Selected].Body, m.Width-1-m.leftWidth())
	total := len(body)
	avail := m.detailHeight()
	if total > avail {
		avail--
	}
	max := total - avail
	if max < 0 {
		max = 0
	}
	if m.DetailOffset > max {
		m.DetailOffset = max
	}
}

// stripMarker removes the leading structural marker for display.
func stripMarker(s string) string {
	const marker = "■"
	if strings.HasPrefix(s, marker) {
		return strings.TrimPrefix(s, marker)
	}
	return s
}

// truncateToWidth shortens s to the given display width, appending an
// ellipsis when characters were removed. Grapheme clusters are never
// split.
func truncateToWidth(s string, max int) string {
	if max <= 0 {
		return ""
	}
	var b strings.Builder
	width := 0
	state := -1
	rest := s
	for len(rest) > 0 {
		var cluster string
		var w int
		cluster, w, state = nextCluster(rest, state)
		if width+w > max {
			// Try to fit an ellipsis within the budget.
			if width+1 <= max {
				b.WriteString("…")
			}
			return b.String()
		}
		b.WriteString(cluster)
		width += w
		rest = rest[len(cluster):]
	}
	return b.String()
}

// nextCluster returns the first grapheme cluster in s, its display
// width, and the new iterator state.
func nextCluster(s string, state int) (string, int, int) {
	cluster, _, w, newState := uniseg.FirstGraphemeClusterInString(s, state)
	return cluster, w, newState
}

// padSpaces appends spaces so text fills width columns.
func padSpaces(s string, width int) string {
	pad := width - uniseg.StringWidth(s)
	if pad <= 0 {
		return ""
	}
	return strings.Repeat(" ", pad)
}

// padRight pads a styled left cell to the pane width using plain spaces
// computed from the unstyled text.
func padRight(s string, width int) string {
	pad := width - uniseg.StringWidth(stripANSI(s))
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

// wrapToWidth wraps s to the display width using grapheme clusters; any
// cluster sequence longer than the width hard-breaks.
func wrapToWidth(s string, width int) []string {
	if width <= 0 {
		return []string{""}
	}
	var lines []string
	var cur strings.Builder
	curW := 0
	state := -1
	rest := s
	flush := func() {
		lines = append(lines, cur.String())
		cur.Reset()
		curW = 0
	}
	for len(rest) > 0 {
		var cluster string
		var w int
		cluster, w, state = nextCluster(rest, state)
		if curW+w > width {
			flush()
		}
		cur.WriteString(cluster)
		curW += w
		rest = rest[len(cluster):]
	}
	flush()
	return lines
}

// stripANSI removes CSI/SGR sequences for width computations.
func stripANSI(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && !isFinalByte(s[j]) {
				j++
			}
			if j < len(s) {
				j++
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func isFinalByte(c byte) bool {
	return c >= 0x40 && c <= 0x7e
}

// firstLine trims an error message to its first line for display.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func (m *Model) bold(s string) string   { return styleBold + s + styleReset }
func (m *Model) dim(s string) string    { return styleDim + s + styleReset }

var _ = search.Entry{} // keep the search import for the entry type docs
