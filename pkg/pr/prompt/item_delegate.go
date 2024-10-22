package prompt

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/ui"
)

type delegateKeyMap struct {
	status        key.Binding
	approve       key.Binding
	merge         key.Binding
	approve_merge key.Binding
}

type eventMsg struct {
	eventType  string
	ctx        *context.Context
	orgName    string
	repoName   string
	selectedPR model.PullRequestResponse
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
		approve_merge: key.NewBinding(
			key.WithKeys("M"),
			key.WithHelp("M", "approve and merge"),
		),
	}
}

func newItemDelegate(ctx *context.Context, orgName string, keys *delegateKeyMap) list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles.SelectedTitle = lipgloss.NewStyle().Bold(true).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#25A065"}).
		Foreground(ui.CrayolaGreen).
		Padding(0, 0, 0, 1)
	d.Styles.SelectedDesc = lipgloss.NewStyle().Italic(true).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#25A065"}).
		Foreground(ui.CrayolaGreen).
		Padding(0, 0, 0, 2)

	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
		var title string

		selectedPR, ok := m.SelectedItem().(model.PullRequestResponse)
		if ok {
			title = selectedPR.Title()
		} else {
			return nil
		}

		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch {
			case key.Matches(msg, keys.status):
				statusMsgCmd := m.NewStatusMessage(statusMessageStyle("PR status " + title))
				eventCmd := func() tea.Msg {
					return eventMsg{eventType: "STATUS", selectedPR: selectedPR, ctx: ctx, orgName: orgName, repoName: selectedPR.RepositoryName()}
				}
				return tea.Batch(statusMsgCmd, eventCmd)
			case key.Matches(msg, keys.approve):
				statusMsgCmd := m.NewStatusMessage(statusMessageStyle("Approving the PR " + title))
				eventCmd := func() tea.Msg {
					return eventMsg{eventType: "APPROVE", selectedPR: selectedPR, ctx: ctx, orgName: orgName, repoName: selectedPR.RepositoryName()}
				}
				return tea.Batch(statusMsgCmd, eventCmd)
			case key.Matches(msg, keys.merge):
				statusMsgCmd := m.NewStatusMessage(statusMessageStyle("TBD: Merging the PR " + title))
				eventCmd := func() tea.Msg {
					return eventMsg{eventType: "MERGE", selectedPR: selectedPR, ctx: ctx, orgName: orgName, repoName: selectedPR.RepositoryName()}
				}
				return tea.Batch(statusMsgCmd, eventCmd)
			case key.Matches(msg, keys.approve_merge):
				statusMsgCmd := m.NewStatusMessage(statusMessageStyle("TBD: Approve and Merging the PR " + title))
				eventCmd := func() tea.Msg {
					return eventMsg{eventType: "APPROVE_MERGE", selectedPR: selectedPR, ctx: ctx, orgName: orgName, repoName: selectedPR.RepositoryName()}
				}
				return tea.Batch(statusMsgCmd, eventCmd)
			}
		}

		return nil
	}

	// help := []key.Binding{keys.status}
	help := []key.Binding{keys.status, keys.approve, keys.merge, keys.approve_merge}

	d.ShortHelpFunc = func() []key.Binding {
		return help
	}

	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{help}
	}

	return d
}
