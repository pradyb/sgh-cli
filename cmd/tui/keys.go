// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package tui

import "github.com/charmbracelet/bubbles/key"

type globalKeyMap struct {
	Quit        key.Binding
	Tab         key.Binding
	ShiftTab    key.Binding
	Help        key.Binding
	Up          key.Binding
	Down        key.Binding
	Enter       key.Binding
	Esc         key.Binding
	Space       key.Binding
	Filter      key.Binding
	Refresh     key.Binding
	HardRefresh key.Binding
	CycleFilter key.Binding
	Sort        key.Binding
	Diff        key.Binding
	Open        key.Binding
	Yank        key.Binding
	SelectAll   key.Binding
	SelectNon   key.Binding
	JumpRepo    key.Binding
	JumpCmd     key.Binding
	JumpCont    key.Binding
	GoTop       key.Binding
	GoBottom    key.Binding
	PageUp      key.Binding
	PageDown    key.Binding
	Menu        key.Binding
}

var keys = globalKeyMap{
	Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Tab:         key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next panel")),
	ShiftTab:    key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev panel")),
	Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Up:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("k/↑", "up")),
	Down:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("j/↓", "down")),
	Enter:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	Esc:         key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Space:       key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
	Filter:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	Refresh:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	HardRefresh: key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "hard refresh")),
	CycleFilter: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "cycle status")),
	Sort:        key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "sort column")),
	Diff:        key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "diff preview")),
	Open:        key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open in browser")),
	Yank:        key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy URL")),
	SelectAll:   key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "select all")),
	SelectNon:   key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "select none")),
	JumpRepo:    key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "repos")),
	JumpCmd:     key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "commands")),
	JumpCont:    key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "content")),
	GoTop:       key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "go to top")),
	GoBottom:    key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "go to bottom")),
	PageUp:      key.NewBinding(key.WithKeys("ctrl+u", "pgup"), key.WithHelp("ctrl+u", "page up")),
	PageDown:    key.NewBinding(key.WithKeys("ctrl+d", "pgdown"), key.WithHelp("ctrl+d", "page down")),
	Menu:        key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "action menu")),
}

type prKeyMap struct {
	Approve      key.Binding
	Merge        key.Binding
	ApproveMerge key.Binding
	Close        key.Binding
}

var prKeys = prKeyMap{
	Approve:      key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "approve")),
	Merge:        key.NewBinding(key.WithKeys("ctrl+m"), key.WithHelp("ctrl+m", "merge")),
	ApproveMerge: key.NewBinding(key.WithKeys("M"), key.WithHelp("M", "approve+merge")),
	Close:        key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "close PR")),
}

type wfKeyMap struct {
	Rerun  key.Binding
	Cancel key.Binding
	Watch  key.Binding
}

var wfKeys = wfKeyMap{
	Rerun:  key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "rerun")),
	Cancel: key.NewBinding(key.WithKeys("X"), key.WithHelp("X", "cancel")),
	Watch:  key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "watch")),
}
