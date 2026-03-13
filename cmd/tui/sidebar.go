// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/prady-lab/sgh-cli/pkg/ui"
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
	{"Prot. Branches", "pb", []string{"Repo", "Branch", "Approvals", "Enforce"}},
	{"Issues", "issue", []string{"Repo", "#", "Title", "Author", "State", "Labels", "Comments", "Updated"}},
	{"Branches", "branch", []string{"Repo", "Branch", "SHA", "Protected"}},
	{"Tags", "tag", []string{"Repo", "Tag", "SHA"}},
	{"Teams", "team", []string{"Team", "Members", "Repos"}},
	{"Audit Log", "audit", []string{"Time", "Actor", "Action", "Repo"}},
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
	"issue": {options: []string{"open", "closed", "all"}, current: 0},
	"pr":    {options: []string{"open", "merged", "closed", "all"}, current: 0},
	"wf":    {options: []string{"all", "completed", "in_progress", "queued"}, current: 0},
}

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

func (m *sidebarModel) selectCommand() int {
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

	numStyle := lipgloss.NewStyle().Foreground(ui.Dimmed)
	numActiveStyle := lipgloss.NewStyle().Foreground(ui.Cyan).Bold(true)

	for i, cmd := range commands {
		style := commandNormalStyle
		isActive := i == m.active
		isCursor := m.focused && i == m.cursor

		numStr := fmt.Sprintf("%d", i+1)
		if isActive || isCursor {
			numStr = numActiveStyle.Render(numStr)
		} else {
			numStr = numStyle.Render(numStr)
		}

		cursor := " "
		if isActive || isCursor {
			cursor = "▸"
			if isCursor {
				style = commandCursorStyle
			} else {
				style = commandActiveStyle
			}
		}

		b.WriteString(fmt.Sprintf(" %s %s %s", numStr, cursor, style.Render(cmd.name)))
		if i < len(commands)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}
