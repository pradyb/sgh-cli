package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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
}

func newContent() contentModel {
	return contentModel{}
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
}

func (m *contentModel) filteredRows() []int {
	indices := make([]int, 0, len(m.rows))
	for i, row := range m.rows {
		if m.filter == "" {
			indices = append(indices, i)
			continue
		}
		for _, cell := range row {
			if strings.Contains(strings.ToLower(cell), strings.ToLower(m.filter)) {
				indices = append(indices, i)
				break
			}
		}
	}
	return indices
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
		return contentRowDimStyle.Render(strings.Join([]string{
			"",
			"  ∅ No data found",
			"",
			"  Try different repos or refresh with 'r'",
			"",
		}, "\n"))
	}

	if m.filtering {
		b.WriteString(filterInputStyle.Render("/ " + m.filter + "█"))
		b.WriteString("\n")
	}

	colWidths := m.computeColumnWidths()
	header := m.renderHeader(colWidths)
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

		line := m.renderRow(rowIdx, row, colWidths, isCursor, isAlt)

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

func (m contentModel) computeColumnWidths() []int {
	if len(m.columns) == 0 {
		return nil
	}
	available := m.width - 4 - (len(m.columns)-1)*2
	if available < len(m.columns)*4 {
		available = len(m.columns) * 4
	}

	// Measure actual content widths
	widths := make([]int, len(m.columns))
	minWidths := make([]int, len(m.columns))
	maxWidths := make([]int, len(m.columns))

	for i, col := range m.columns {
		// Start with header width
		widths[i] = len(col)
		minWidths[i] = 8

		// Set max widths based on content type
		switch col {
		case "Title", "Message", "Workflow", "Description":
			maxWidths[i] = 60
		case "Repo", "Branch", "Tag", "Team", "Author":
			maxWidths[i] = 30
		default:
			maxWidths[i] = 20
		}
	}

	// Sample up to 50 rows to measure actual content widths
	sampleSize := min(len(m.rows), 50)
	for rowIdx := 0; rowIdx < sampleSize; rowIdx++ {
		row := m.rows[rowIdx]
		for colIdx := 0; colIdx < len(m.columns) && colIdx < len(row); colIdx++ {
			cellLen := len(row[colIdx])
			if cellLen > widths[colIdx] {
				widths[colIdx] = min(cellLen, maxWidths[colIdx])
			}
		}
	}

	// Ensure minimum widths
	totalRequired := 0
	for i := range widths {
		if widths[i] < minWidths[i] {
			widths[i] = minWidths[i]
		}
		totalRequired += widths[i]
	}

	// If we exceed available space, scale down proportionally
	if totalRequired > available {
		scale := float64(available) / float64(totalRequired)
		for i := range widths {
			widths[i] = max(minWidths[i], int(float64(widths[i])*scale))
		}
	}

	// If we have extra space, distribute it proportionally
	if totalRequired < available {
		extra := available - totalRequired
		totalWeight := 0
		for i := range widths {
			totalWeight += widths[i]
		}
		for i := range widths {
			addition := (extra * widths[i]) / totalWeight
			widths[i] += addition
		}
	}

	return widths
}

func (m contentModel) renderHeader(widths []int) string {
	parts := make([]string, len(m.columns))
	hasOverflow := false
	for i, col := range m.columns {
		w := 8
		if i < len(widths) {
			w = widths[i]
		}
		if len(col) > w {
			hasOverflow = true
			col = col[:w-1] + "›"
		}
		parts[i] = contentHeaderStyle.Width(w).Render(col)
	}
	header := strings.Join(parts, "  ")
	if hasOverflow {
		header += " " + contentRowDimStyle.Render("(columns truncated)")
	}
	return header
}

func (m contentModel) renderRow(rowIdx int, row []string, widths []int, isCursor bool, isAlt bool) string {
	parts := make([]string, len(m.columns))
	for i := range m.columns {
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
			style = lipgloss.NewStyle().Foreground(m.rowColors[rowIdx][i])
			if isAlt {
				style = style.Background(lipgloss.Color("#1E1E2E"))
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
		parts[i] = style.Width(w).Render(cell)
	}

	prefix := "  "
	if isCursor {
		prefix = fmt.Sprintf("%s ", contentCursorStyle.Render("▸"))
	}
	return prefix + strings.Join(parts, "  ")
}
