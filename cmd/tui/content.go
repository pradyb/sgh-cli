package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type contentModel struct {
	command   string
	columns   []string
	rows      [][]string
	rowColors [][]lipgloss.Color // per-cell color overrides (same shape as rows)
	rawData   any
	cursor    int
	offset    int
	filter    string
	filtering bool
	focused   bool
	width     int
	height    int
	loading   bool
	noData    bool
	noRepos   bool
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

func (m *contentModel) handleFilterKey(keyStr string) {
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
		return
	}
	if keyStr == "enter" {
		m.filtering = false
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
		return contentRowDimStyle.Render("Select a command from the sidebar")
	}

	if m.loading {
		return contentRowDimStyle.Render("Loading...")
	}

	if m.noRepos {
		return contentRowDimStyle.Render("No repos selected — press 1 to jump to repo selector")
	}

	if m.noData {
		return contentRowDimStyle.Render("No data found")
	}

	if m.filtering {
		b.WriteString(filterInputStyle.Render("/ "+m.filter+"█"))
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

		line := m.renderRow(rowIdx, row, colWidths, isCursor)
		b.WriteString(line)
		if vi < end-1 {
			b.WriteString("\n")
		}
	}

	if len(filtered) == 0 && m.filter != "" {
		b.WriteString(contentRowDimStyle.Render("  no matches"))
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

	widths := make([]int, len(m.columns))
	totalRatio := 0
	ratios := make([]int, len(m.columns))
	for i, col := range m.columns {
		r := 1
		switch col {
		case "Title", "Message", "Workflow":
			r = 3
		case "Repo", "Branch", "Tag", "Team":
			r = 2
		}
		ratios[i] = r
		totalRatio += r
	}
	for i, r := range ratios {
		widths[i] = available * r / totalRatio
		if widths[i] < 4 {
			widths[i] = 4
		}
	}
	return widths
}

func (m contentModel) renderHeader(widths []int) string {
	parts := make([]string, len(m.columns))
	for i, col := range m.columns {
		w := 8
		if i < len(widths) {
			w = widths[i]
		}
		parts[i] = contentHeaderStyle.Width(w).Render(col)
	}
	return strings.Join(parts, "  ")
}

func (m contentModel) renderRow(rowIdx int, row []string, widths []int, isCursor bool) string {
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
