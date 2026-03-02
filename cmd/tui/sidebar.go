package tui

import (
	"fmt"
	"strings"
)

type commandDef struct {
	name    string
	key     string
	columns []string
}

var commands = []commandDef{
	{"Pull Requests", "pr", []string{"Repo", "#", "Title", "Author", "Status", "Updated"}},
	{"Workflows", "wf", []string{"Repo", "Workflow", "Status", "Conclusion", "Branch", "Updated"}},
	{"Commits", "commit", []string{"Repo", "Message", "Author", "Date"}},
	{"Teams", "team", []string{"Team", "Members", "Repos"}},
	{"Prot. Branches", "pb", []string{"Repo", "Branch", "Approvals", "Enforce"}},
	{"Branches", "branch", []string{"Repo", "Branch", "SHA", "Protected"}},
	{"Tags", "tag", []string{"Repo", "Tag", "SHA"}},
}

type commandFilter struct {
	options []string // available filter values
	current int      // index into options (0 = default)
}

func (f *commandFilter) value() string {
	return f.options[f.current]
}

func (f *commandFilter) cycle() {
	f.current = (f.current + 1) % len(f.options)
}

func (f *commandFilter) label() string {
	return f.options[f.current]
}

var defaultFilters = map[string]*commandFilter{
	"pr": {options: []string{"open", "merged", "closed", "all"}, current: 0},
	"wf": {options: []string{"all", "completed", "in_progress", "queued"}, current: 0},
}

const commandMenuHeight = 10

type sidebarModel struct {
	cursor  int
	active  int
	focused bool
}

func newSidebar() sidebarModel {
	return sidebarModel{cursor: 0, active: -1}
}

func (m *sidebarModel) moveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

func (m *sidebarModel) moveDown() {
	if m.cursor < len(commands)-1 {
		m.cursor++
	}
}

func (m *sidebarModel) select_() int {
	m.active = m.cursor
	return m.active
}

func (m *sidebarModel) activeCommand() *commandDef {
	if m.active < 0 || m.active >= len(commands) {
		return nil
	}
	return &commands[m.active]
}

func (m sidebarModel) view() string {
	var b strings.Builder

	for i, cmd := range commands {
		style := commandNormalStyle
		marker := "   "

		if i == m.active && m.focused && i == m.cursor {
			style = commandCursorStyle
			marker = " > "
		} else if i == m.active {
			style = commandActiveStyle
			marker = " > "
		} else if m.focused && i == m.cursor {
			style = commandCursorStyle
			marker = " > "
		}

		b.WriteString(fmt.Sprintf("%s%s", marker, style.Render(cmd.name)))
		if i < len(commands)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}
