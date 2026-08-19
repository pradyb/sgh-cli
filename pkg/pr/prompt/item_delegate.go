// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package prompt

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pradyb/sgh-cli/internal/model"
	"github.com/pradyb/sgh-cli/pkg/context"
	"github.com/pradyb/sgh-cli/pkg/pr"
	"github.com/pradyb/sgh-cli/pkg/ui"
)

type delegateKeyMap struct {
	status       key.Binding
	approve      key.Binding
	merge        key.Binding
	approveMerge key.Binding
	closePR      key.Binding
	openBrowser  key.Binding
	diff         key.Binding
}

type eventMsg struct {
	eventType  string
	ctx        *context.Context
	orgName    string
	repoName   string
	selectedPR model.PullRequestResponse
}

// confirmMsg is sent when the user triggers a destructive action (approve, merge, close).
// The actual action is deferred until the user confirms with 'y'.
type confirmMsg struct {
	eventType  string
	ctx        *context.Context
	orgName    string
	repoName   string
	selectedPR model.PullRequestResponse
	prompt     string
}

// browserOpenMsg signals that a URL should be opened in the default browser.
type browserOpenMsg struct {
	url string
}

func newDelegateKeyMap() *delegateKeyMap {
	return &delegateKeyMap{
		status: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "status"),
		),
		approve: key.NewBinding(
			key.WithKeys("A"),
			key.WithHelp("A", "approve"),
		),
		merge: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "merge"),
		),
		approveMerge: key.NewBinding(
			key.WithKeys("M"),
			key.WithHelp("M", "approve and merge"),
		),
		closePR: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "close PR"),
		),
		openBrowser: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open in browser"),
		),
		diff: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "view diff"),
		),
	}
}

func openURL(url string) error {
	return ui.OpenURL(url)
}

func newItemDelegate(ctx *context.Context, orgName string, keys *delegateKeyMap) list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles.SelectedTitle = lipgloss.NewStyle().Bold(true).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(ui.CrayolaGreen).
		Foreground(ui.CrayolaGreen).
		Padding(0, 0, 0, 1)
	d.Styles.SelectedDesc = lipgloss.NewStyle().Italic(true).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(ui.CrayolaGreen).
		Foreground(ui.Subtle).
		Padding(0, 0, 0, 2)

	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
		selectedPR, ok := m.SelectedItem().(model.PullRequestResponse)
		if !ok {
			return nil
		}
		title := selectedPR.Title()

		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch {
			case key.Matches(msg, keys.status):
				return func() tea.Msg {
					return eventMsg{eventType: "STATUS", selectedPR: selectedPR, ctx: ctx, orgName: orgName, repoName: selectedPR.RepositoryName()}
				}

			case key.Matches(msg, keys.approve):
				return func() tea.Msg {
					return confirmMsg{
						eventType:  "APPROVE",
						ctx:        ctx,
						orgName:    orgName,
						repoName:   selectedPR.RepositoryName(),
						selectedPR: selectedPR,
						prompt:     "Approve PR " + title + "? (y/n)",
					}
				}

			case key.Matches(msg, keys.merge):
				return func() tea.Msg {
					return confirmMsg{
						eventType:  "MERGE",
						ctx:        ctx,
						orgName:    orgName,
						repoName:   selectedPR.RepositoryName(),
						selectedPR: selectedPR,
						prompt:     "Merge PR " + title + "? (y/n)",
					}
				}

			case key.Matches(msg, keys.approveMerge):
				return func() tea.Msg {
					return confirmMsg{
						eventType:  "APPROVE_MERGE",
						ctx:        ctx,
						orgName:    orgName,
						repoName:   selectedPR.RepositoryName(),
						selectedPR: selectedPR,
						prompt:     "Approve and Merge PR " + title + "? (y/n)",
					}
				}

			case key.Matches(msg, keys.closePR):
				return func() tea.Msg {
					return confirmMsg{
						eventType:  "CLOSE",
						ctx:        ctx,
						orgName:    orgName,
						repoName:   selectedPR.RepositoryName(),
						selectedPR: selectedPR,
						prompt:     "Close PR " + title + "? (y/n)",
					}
				}

			case key.Matches(msg, keys.openBrowser):
				return func() tea.Msg {
					return browserOpenMsg{url: selectedPR.HTMLUrl}
				}

			case key.Matches(msg, keys.diff):
				p := selectedPR
				c := ctx
				o := orgName
				return func() tea.Msg {
					filesResp := pr.GetPullRequestFiles(c, o, p.RepositoryName(), p.PRNumber)
					return diffLoadedMsg{
						title: fmt.Sprintf("PR #%d · %s", p.PRNumber, p.TitleName),
						lines: pr.ParsePatchLines(filesResp),
					}
				}
			}
		}

		return nil
	}

	help := []key.Binding{keys.status, keys.approve, keys.merge, keys.approveMerge, keys.closePR, keys.openBrowser, keys.diff}

	d.ShortHelpFunc = func() []key.Binding {
		return help
	}

	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{help}
	}

	return d
}
