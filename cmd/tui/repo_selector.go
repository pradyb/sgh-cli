package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

type repoItem struct {
	name     string
	selected bool
}

type repoSelectorModel struct {
	repos      []repoItem
	cursor     int
	filter     string
	filtering  bool
	height     int
	offset     int
	focused    bool
}

func newRepoSelector(repoNames []string) repoSelectorModel {
	items := make([]repoItem, len(repoNames))
	for i, name := range repoNames {
		items[i] = repoItem{name: name, selected: false}
	}
	return repoSelectorModel{repos: items}
}

func (m *repoSelectorModel) filteredIndices() []int {
	indices := make([]int, 0, len(m.repos))
	for i, r := range m.repos {
		if m.filter == "" || strings.Contains(strings.ToLower(r.name), strings.ToLower(m.filter)) {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m *repoSelectorModel) selectedNames() []string {
	names := make([]string, 0)
	for _, r := range m.repos {
		if r.selected {
			names = append(names, r.name)
		}
	}
	return names
}

func (m *repoSelectorModel) selectedCount() int {
	count := 0
	for _, r := range m.repos {
		if r.selected {
			count++
		}
	}
	return count
}

func (m *repoSelectorModel) toggle() bool {
	indices := m.filteredIndices()
	if len(indices) == 0 {
		return false
	}
	idx := indices[m.cursor]
	m.repos[idx].selected = !m.repos[idx].selected
	return true
}

func (m *repoSelectorModel) selectAll() {
	indices := m.filteredIndices()
	for _, idx := range indices {
		m.repos[idx].selected = true
	}
}

func (m *repoSelectorModel) selectNone() {
	indices := m.filteredIndices()
	for _, idx := range indices {
		m.repos[idx].selected = false
	}
}

func (m *repoSelectorModel) moveUp() {
	if m.cursor > 0 {
		m.cursor--
		if m.cursor < m.offset {
			m.offset = m.cursor
		}
	}
}

func (m *repoSelectorModel) moveDown() {
	indices := m.filteredIndices()
	if m.cursor < len(indices)-1 {
		m.cursor++
		visible := m.height
		if m.filtering {
			visible--
		}
		if visible < 1 {
			visible = 1
		}
		if m.cursor >= m.offset+visible {
			m.offset = m.cursor - visible + 1
		}
	}
}

func (m *repoSelectorModel) goTop() {
	m.cursor = 0
	m.offset = 0
}

func (m *repoSelectorModel) goBottom() {
	indices := m.filteredIndices()
	if len(indices) == 0 {
		return
	}
	m.cursor = len(indices) - 1
	visible := m.height
	if m.filtering {
		visible--
	}
	if visible < 1 {
		visible = 1
	}
	if m.cursor >= visible {
		m.offset = m.cursor - visible + 1
	}
}

func (m *repoSelectorModel) handleFilterKey(keyMsg key.Binding, keyStr string) {
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

func (m repoSelectorModel) title() string {
	badge := fmt.Sprintf("%d/%d", m.selectedCount(), len(m.repos))
	return fmt.Sprintf("Repos %s", badgeStyle.Render(badge))
}

func (m repoSelectorModel) view() string {
	var b strings.Builder

	if m.filtering {
		b.WriteString(filterInputStyle.Render("/ "+m.filter+"█"))
		b.WriteString("\n")
	}

	indices := m.filteredIndices()
	visible := m.height
	if m.filtering {
		visible--
	}
	if visible < 1 {
		visible = 1
	}

	end := m.offset + visible
	if end > len(indices) {
		end = len(indices)
	}

	maxNameLen := sidebarWidth - 8
	if maxNameLen < 4 {
		maxNameLen = 4
	}

	for vi := m.offset; vi < end; vi++ {
		idx := indices[vi]
		r := m.repos[idx]

		name := r.name
		if len(name) > maxNameLen {
			name = name[:maxNameLen-1] + "…"
		}

		checkbox := "[ ]"
		nameStyle := repoNormalStyle
		if r.selected {
			checkbox = "[x]"
			nameStyle = repoSelectedStyle
		}
		if m.focused && vi == m.cursor {
			if r.selected {
				nameStyle = repoSelectedCursorStyle
			} else {
				nameStyle = repoCursorStyle
			}
		}

		line := fmt.Sprintf(" %s %s", checkbox, nameStyle.Render(name))
		b.WriteString(line)
		if vi < end-1 {
			b.WriteString("\n")
		}
	}

	if len(indices) == 0 {
		b.WriteString(contentRowDimStyle.Render("  no matches"))
	}

	return b.String()
}
