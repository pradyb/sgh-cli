// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/prady-lab/sgh-cli/pkg/ui"
)

type contentModel struct {
	command            string
	columns            []string
	rows               [][]string
	rowColors          [][]lipgloss.Color // per-cell color overrides (same shape as rows)
	rawData            any
	cursor             int
	offset             int
	filter             string
	filtering          bool
	focused            bool
	width              int
	height             int
	loading            bool
	noData             bool
	noRepos            bool
	filterHistory      []string
	filterHistoryIndex int
	sortColumn         int // -1 = unsorted
	sortAscending      bool
}

func newContent() contentModel {
	return contentModel{sortColumn: -1, sortAscending: true}
}

func (m *contentModel) setData(command string, columns []string, rows [][]string, colors [][]lipgloss.Color, raw any) {
	m.command = command
	m.columns = columns
	m.rows = rows
	m.rowColors = colors
	m.rawData = raw
	m.cursor = 0
	m.offset = 0
	m.loading = false
	m.noData = len(rows) == 0
	m.sortColumn = -1
	m.sortAscending = true
}

// filterExcludedColumns lists column names that carry enum/badge values and
// should not be matched by the text search filter (to avoid e.g. typing "m"
// instantly matching every "Merged" PR row).
var filterExcludedColumns = map[string]bool{
	"Status":     true,
	"State":      true,
	"Conclusion": true,
	"Protected":  true,
	"Enforce":    true,
}

func (m *contentModel) filteredRows() []int {
	indices := make([]int, 0, len(m.rows))
	lower := strings.ToLower(m.filter)
	for i, row := range m.rows {
		if m.filter == "" {
			indices = append(indices, i)
			continue
		}
		for ci, cell := range row {
			if ci < len(m.columns) && filterExcludedColumns[m.columns[ci]] {
				continue
			}
			if strings.Contains(strings.ToLower(cell), lower) {
				indices = append(indices, i)
				break
			}
		}
	}
	if m.sortColumn >= 0 && m.sortColumn < len(m.columns) {
		col := m.sortColumn
		asc := m.sortAscending
		sort.SliceStable(indices, func(a, b int) bool {
			va, vb := "", ""
			if col < len(m.rows[indices[a]]) {
				va = m.rows[indices[a]][col]
			}
			if col < len(m.rows[indices[b]]) {
				vb = m.rows[indices[b]][col]
			}
			if asc {
				return va < vb
			}
			return va > vb
		})
	}
	return indices
}

// cycleSort advances the sort column (or flips direction on the current column).
func (m *contentModel) cycleSort() {
	if len(m.columns) == 0 {
		return
	}
	if m.sortColumn < 0 {
		m.sortColumn = 0
		m.sortAscending = true
	} else if m.sortAscending {
		m.sortAscending = false
	} else {
		m.sortColumn = (m.sortColumn + 1) % len(m.columns)
		m.sortAscending = true
	}
}

func (m *contentModel) moveUp() {
	if m.cursor > 0 {
		m.cursor--
		if m.cursor < m.offset {
			m.offset = m.cursor
		}
	}
}

func (m *contentModel) moveDown() {
	filtered := m.filteredRows()
	if m.cursor < len(filtered)-1 {
		m.cursor++
		visible := m.height - 4
		if visible < 1 {
			visible = 1
		}
		if m.cursor >= m.offset+visible {
			m.offset = m.cursor - visible + 1
		}
	}
}

func (m *contentModel) goTop() {
	m.cursor = 0
	m.offset = 0
}

func (m *contentModel) goBottom() {
	filtered := m.filteredRows()
	if len(filtered) == 0 {
		return
	}
	m.cursor = len(filtered) - 1
	visible := m.height - 4
	if visible < 1 {
		visible = 1
	}
	if m.cursor >= visible {
		m.offset = m.cursor - visible + 1
	}
}

func (m *contentModel) pageUp() {
	visible := m.height - 4
	if visible < 1 {
		visible = 1
	}
	m.cursor -= visible
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
}

func (m *contentModel) pageDown() {
	filtered := m.filteredRows()
	visible := m.height - 4
	if visible < 1 {
		visible = 1
	}
	m.cursor += visible
	if m.cursor >= len(filtered) {
		m.cursor = len(filtered) - 1
	}
	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
	}
}

func (m *contentModel) handleFilterKey(keyStr string) {
	if keyStr == "up" && len(m.filterHistory) > 0 {
		if m.filterHistoryIndex > 0 {
			m.filterHistoryIndex--
			m.filter = m.filterHistory[m.filterHistoryIndex]
		}
		return
	}
	if keyStr == "down" && len(m.filterHistory) > 0 {
		if m.filterHistoryIndex < len(m.filterHistory)-1 {
			m.filterHistoryIndex++
			m.filter = m.filterHistory[m.filterHistoryIndex]
		} else {
			m.filterHistoryIndex = len(m.filterHistory)
			m.filter = ""
		}
		return
	}
	if keyStr == "backspace" {
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
			m.cursor = 0
			m.offset = 0
		}
		return
	}
	if keyStr == "esc" {
		m.filtering = false
		m.filter = ""
		m.cursor = 0
		m.offset = 0
		m.filterHistoryIndex = len(m.filterHistory)
		return
	}
	if keyStr == "enter" {
		m.filtering = false
		// Add to history if non-empty and not duplicate of last entry
		if m.filter != "" {
			if len(m.filterHistory) == 0 || m.filterHistory[len(m.filterHistory)-1] != m.filter {
				m.filterHistory = append(m.filterHistory, m.filter)
				// Keep only last 20 entries
				if len(m.filterHistory) > 20 {
					m.filterHistory = m.filterHistory[1:]
				}
			}
			m.filterHistoryIndex = len(m.filterHistory)
		}
		return
	}
	if len(keyStr) == 1 {
		m.filter += keyStr
		m.cursor = 0
		m.offset = 0
	}
}

func (m *contentModel) selectedRowIndex() int {
	filtered := m.filteredRows()
	if m.cursor < 0 || m.cursor >= len(filtered) {
		return -1
	}
	return filtered[m.cursor]
}

func (m contentModel) view() string {
	var b strings.Builder

	if m.command == "" {
		return contentRowDimStyle.Render(strings.Join([]string{
			"",
			"  ┌───────────────────────────┐",
			"  │  Select command to begin  │",
			"  └───────────────────────────┘",
			"",
			"  Press 2 to jump to commands",
			"",
		}, "\n"))
	}

	if m.loading {
		return contentRowDimStyle.Render(strings.Join([]string{
			"",
			"  ⏳ Loading data...",
			"",
		}, "\n"))
	}

	if m.noRepos {
		return contentRowDimStyle.Render(strings.Join([]string{
			"",
			"  ┌────────────────────────────┐",
			"  │  No repositories selected  │",
			"  └────────────────────────────┘",
			"",
			"  Press 1 for repo selector",
			"  Space to select repos",
			"  'a' to select all",
			"",
		}, "\n"))
	}

	if m.noData {
		msg := "No data found"
		hint := "Try different repos or refresh with 'r'"
		switch m.command {
		case "pr":
			msg = "No pull requests found"
			hint = "Try changing the filter with 's' or select more repos"
		case "issue":
			msg = "No issues found"
			hint = "Try changing the filter with 's' or select more repos"
		case "wf":
			msg = "No workflow runs found"
			hint = "Try changing the filter with 's' or select more repos"
		case "branch":
			msg = "No branches found"
			hint = "Select repos and refresh with 'r'"
		case "tag":
			msg = "No tags found"
			hint = "Select repos and refresh with 'r'"
		case "commit":
			msg = "No commits in the last 14 days"
			hint = "Select repos and refresh with 'r'"
		case "team":
			msg = "No teams found"
			hint = "Check org permissions or refresh with 'r'"
		case "pb":
			msg = "No protected branches found"
			hint = "Select repos and refresh with 'r'"
		case "audit":
			msg = "No audit log entries found"
			hint = "Check org permissions or refresh with 'r'"
		}
		return contentRowDimStyle.Render(strings.Join([]string{
			"",
			"  ∅ " + msg,
			"",
			"  " + hint,
			"",
		}, "\n"))
	}

	if m.filtering {
		b.WriteString(filterInputStyle.Render("/ " + m.filter + "█"))
		b.WriteString("\n")
	}

	colWidths, visibleCols := m.computeColumnWidths()
	header := m.renderHeader(colWidths, visibleCols)
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(separatorStyle.Render(strings.Repeat("─", max(1, m.width-6))))
	b.WriteString("\n")

	filtered := m.filteredRows()
	visible := m.height - 2 // subtract header + separator lines
	if m.filtering {
		visible--
	}
	if visible < 1 {
		visible = 1
	}
	end := m.offset + visible
	if end > len(filtered) {
		end = len(filtered)
	}

	for vi := m.offset; vi < end; vi++ {
		rowIdx := filtered[vi]
		row := m.rows[rowIdx]
		isCursor := m.focused && vi == m.cursor
		isAlt := vi%2 != 0

		line := m.renderRow(rowIdx, row, colWidths, visibleCols, isCursor, isAlt)

		// Scrollbar logic
		scrollChar := ""
		if len(filtered) > visible {
			pct := float64(m.offset) / float64(len(filtered)-visible)
			if m.offset == 0 {
				pct = 0
			} else if end == len(filtered) {
				pct = 1
			}
			scrollPos := int(pct * float64(visible-1))
			if vi-m.offset == scrollPos {
				scrollChar = "█"
			} else {
				scrollChar = "│"
			}
		}

		b.WriteString(line)
		if scrollChar != "" {
			lineWidth := lipgloss.Width(line)
			pad := m.width - 6 - lineWidth
			if pad > 0 {
				b.WriteString(strings.Repeat(" ", pad))
			}
			b.WriteString(separatorStyle.Render(scrollChar))
		}

		if vi < end-1 {
			b.WriteString("\n")
		}
	}

	if len(filtered) == 0 && m.filter != "" {
		b.WriteString(contentRowDimStyle.Render(strings.Join([]string{
			"",
			fmt.Sprintf("  No matches for \"%s\"", m.filter),
			"",
			"  Try a different search term",
			"  or press Esc to clear filter",
			"",
		}, "\n")))
	}

	return b.String()
}

// columnDropPriority defines columns to drop (last = first dropped) when
// the content panel is too narrow to fit all columns at minimum width.
// Columns not listed are always kept (they are the "anchor" columns).
var columnDropPriority = map[string][]string{
	"pr":     {"Updated", "Author"},
	"wf":     {"Updated", "Branch", "Conclusion"},
	"commit": {"Date", "Author"},
	"issue":  {"Updated", "Comments", "Labels", "Author"},
	"branch": {"Protected", "SHA"},
	"pb":     {"Enforce", "Approvals"},
	"tag":    {"SHA"},
	"audit":  {"Repo", "Time"},
}

// minColWidth and maxColWidth by column name category.
func colMinMax(col string) (min_, max_ int) {
	switch col {
	case "Title", "Message", "Workflow", "Description":
		return 14, 60
	case "Repo", "Branch", "Tag", "Team", "Author", "Actor", "Action":
		return 10, 30
	default:
		return 8, 20
	}
}

// computeColumnWidths returns widths only for columns that fit at minimum
// width. Low-priority columns are dropped first when space is tight.
func (m contentModel) computeColumnWidths() ([]int, []int) {
	if len(m.columns) == 0 {
		return nil, nil
	}

	dropList := columnDropPriority[m.command]

	// Build drop-priority index (higher index = dropped first).
	dropRank := make(map[string]int, len(dropList))
	for i, c := range dropList {
		dropRank[c] = i + 1 // 1-based; 0 means "never drop"
	}

	// Column metadata
	type colMeta struct {
		origIdx int
		min_    int
		max_    int
		ideal   int // header width to start with
		rank    int // drop rank (0 = keep always)
	}
	metas := make([]colMeta, len(m.columns))
	for i, col := range m.columns {
		mn, mx := colMinMax(col)
		metas[i] = colMeta{origIdx: i, min_: mn, max_: mx, ideal: len(col), rank: dropRank[col]}
	}

	// Sample rows to set ideal widths
	sampleSize := min(len(m.rows), 50)
	for rowIdx := 0; rowIdx < sampleSize; rowIdx++ {
		row := m.rows[rowIdx]
		for ci := range metas {
			if ci < len(row) && len(row[ci]) > metas[ci].ideal {
				metas[ci].ideal = min(len(row[ci]), metas[ci].max_)
			}
		}
	}

	// Available width: 4 for cursor prefix + padding, 2 per gap between cols
	// We iterate dropping the lowest-priority column until everything fits.
	active := make([]bool, len(m.columns))
	for i := range active {
		active[i] = true
	}

	fits := func() (int, bool) {
		n := 0
		for i := range active {
			if active[i] {
				n++
			}
		}
		if n == 0 {
			return 0, true
		}
		avail := m.width - 4 - (n-1)*2
		total := 0
		for i := range metas {
			if active[i] {
				total += metas[i].min_
			}
		}
		return avail, avail >= total
	}

	// Drop columns in priority order (highest rank first) until they fit
	for {
		avail, ok := fits()
		if ok || avail < 0 {
			break
		}
		// Find the highest-rank active column to drop
		bestRank, bestIdx := 0, -1
		for i := range metas {
			if active[i] && metas[i].rank > bestRank {
				bestRank = metas[i].rank
				bestIdx = i
			}
		}
		if bestIdx < 0 {
			break // nothing left to drop
		}
		active[bestIdx] = false
	}

	// Compute active count and available space
	n := 0
	for i := range active {
		if active[i] {
			n++
		}
	}
	avail := m.width - 4 - (n-1)*2
	if avail < n*4 {
		avail = n * 4
	}

	// Set widths = ideal, clamped to [min, max]
	widths := make([]int, len(m.columns))
	total := 0
	for i := range metas {
		if active[i] {
			w := metas[i].ideal
			if w < metas[i].min_ {
				w = metas[i].min_
			}
			if w > metas[i].max_ {
				w = metas[i].max_
			}
			widths[i] = w
			total += w
		}
	}

	// Shrink proportionally if over budget
	if total > avail {
		scale := float64(avail) / float64(total)
		for i := range metas {
			if active[i] {
				widths[i] = max(metas[i].min_, int(float64(widths[i])*scale))
			}
		}
	}

	// Distribute extra space proportionally among active columns
	total = 0
	for i := range metas {
		if active[i] {
			total += widths[i]
		}
	}
	if extra := avail - total; extra > 0 {
		weight := total
		if weight == 0 {
			weight = 1
		}
		for i := range metas {
			if active[i] {
				widths[i] += (extra * widths[i]) / weight
			}
		}
	}

	// Build the list of active column original indices
	visibleCols := make([]int, 0, n)
	for i := range metas {
		if active[i] {
			visibleCols = append(visibleCols, i)
		}
	}

	return widths, visibleCols
}

func (m contentModel) renderHeader(widths []int, visibleCols []int) string {
	parts := make([]string, 0, len(visibleCols))
	hasOverflow := false
	sortIndicator := lipgloss.NewStyle().Foreground(ui.Yellow).Bold(true)
	for _, i := range visibleCols {
		col := m.columns[i]
		w := 8
		if i < len(widths) {
			w = widths[i]
		}
		label := col
		if i == m.sortColumn {
			arrow := "▲"
			if !m.sortAscending {
				arrow = "▼"
			}
			label = sortIndicator.Render(arrow) + col
		}
		if lipgloss.Width(label) > w {
			hasOverflow = true
			r := []rune(col)
			if w-1 < len(r) {
				r = r[:w-1]
			}
			label = string(r) + "›"
		}
		parts = append(parts, contentHeaderStyle.Width(w).Render(label))
	}
	header := strings.Join(parts, "  ")
	if hasOverflow {
		header += " " + contentRowDimStyle.Render("(columns truncated)")
	}
	return header
}

// highlightFilterMatch wraps the matching substring in a yellow bold style.
func highlightFilterMatch(cell, filter string) string {
	if filter == "" {
		return cell
	}
	lower := strings.ToLower(cell)
	idx := strings.Index(lower, strings.ToLower(filter))
	if idx < 0 {
		return cell
	}
	hlStyle := lipgloss.NewStyle().Foreground(ui.Yellow).Bold(true).Underline(true)
	return cell[:idx] + hlStyle.Render(cell[idx:idx+len(filter)]) + cell[idx+len(filter):]
}

func (m contentModel) renderRow(rowIdx int, row []string, widths []int, visibleCols []int, isCursor bool, isAlt bool) string {
	parts := make([]string, 0, len(visibleCols))
	for _, i := range visibleCols {
		w := 8
		if i < len(widths) {
			w = widths[i]
		}
		cell := ""
		if i < len(row) {
			cell = row[i]
		}
		if lipgloss.Width(cell) > w {
			cell = cell[:w-1] + "…"
		}

		style := contentRowStyle
		if isCursor {
			style = contentCursorStyle
		} else if m.rowColors != nil && rowIdx < len(m.rowColors) && i < len(m.rowColors[rowIdx]) && m.rowColors[rowIdx][i] != "" {
			icon := ui.StatusIcon(cell)
			if icon != "" {
				cell = icon + " " + cell
			}
			style = lipgloss.NewStyle().Foreground(m.rowColors[rowIdx][i])
			if isAlt {
				style = style.Background(lipgloss.ANSIColor(236))
			}
		} else if isAlt {
			if i > 0 {
				style = contentRowDimAltStyle
			} else {
				style = contentRowAltStyle
			}
		} else if i > 0 {
			style = contentRowDimStyle
		}

		// Highlight filter matches (skip on cursor row — inverted bg makes it hard to read)
		if !isCursor && m.filter != "" {
			cell = highlightFilterMatch(cell, m.filter)
		}

		parts = append(parts, style.Width(w).Render(cell))
	}

	prefix := "  "
	if isCursor {
		prefix = fmt.Sprintf("%s ", contentCursorStyle.Render("▸"))
	}
	return prefix + strings.Join(parts, "  ")
}
