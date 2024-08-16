package prompt

import (
	"sync"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/pkg/commit"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/pr"
)

type delegateKeyMap struct {
	status        key.Binding
	approve       key.Binding
	merge         key.Binding
	approve_merge key.Binding
}

type eventMsg struct {
	eventType        string
	prResponse       model.PullRequestResponse
	commitResponse   model.CommitResponse
	checkRunResponse model.CheckRunResponse
	prReviews        []model.ReviewPullRequestResponse
	message          string
}

func newDelegateKeyMap() *delegateKeyMap {
	return &delegateKeyMap{
		status: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "status"),
		),
		approve: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "approve"),
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
		Foreground(lipgloss.Color("#25A065")).
		Padding(0, 0, 0, 1)
	d.Styles.SelectedDesc = lipgloss.NewStyle().Italic(true).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#25A065"}).
		Foreground(lipgloss.Color("#25A035")).
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
				m.StartSpinner()
				statusMsgCmd := m.NewStatusMessage(statusMessageStyle("PR status " + title))
				pullRequestResponse, commitResponse, checkRunResponse, prReviews := getPRDetails(ctx, orgName, selectedPR.RepositoryName(), selectedPR.PRNumber, selectedPR.Head.Sha)
				eventCmd := func() tea.Msg {
					return eventMsg{eventType: "STATUS", prResponse: pullRequestResponse, commitResponse: commitResponse, checkRunResponse: checkRunResponse, prReviews: prReviews}
				}
				m.StopSpinner()
				return tea.Batch(statusMsgCmd, eventCmd)
			case key.Matches(msg, keys.approve):
				statusMsgCmd := m.NewStatusMessage(statusMessageStyle("Approving the PR " + title))
				eventCmd := func() tea.Msg { return eventMsg{eventType: "APPROVE", message: "Checking status " + title} }
				return tea.Batch(statusMsgCmd, eventCmd)
			case key.Matches(msg, keys.merge):
				statusMsgCmd := m.NewStatusMessage(statusMessageStyle("Merging the PR " + title))
				eventCmd := func() tea.Msg { return eventMsg{eventType: "MERGE", message: "Checking status " + title} }
				return tea.Batch(statusMsgCmd, eventCmd)
			case key.Matches(msg, keys.approve_merge):
				statusMsgCmd := m.NewStatusMessage(statusMessageStyle("Approve and Merging the PR " + title))
				eventCmd := func() tea.Msg { return eventMsg{eventType: "APPROVE_MERGE", message: "Checking status " + title} }
				return tea.Batch(statusMsgCmd, eventCmd)
			}

		}

		return nil
	}

	help := []key.Binding{keys.status, keys.approve, keys.merge, keys.approve_merge}

	d.ShortHelpFunc = func() []key.Binding {
		return help
	}

	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{help}
	}

	return d
}

func getPRDetails(ctx *context.Context, orgName string, repoName string, prNumber int, lastSha string) (model.PullRequestResponse, model.CommitResponse, model.CheckRunResponse, []model.ReviewPullRequestResponse) {

	var wg sync.WaitGroup
	var pullRequestResponse model.PullRequestResponse
	var commitResponse model.CommitResponse
	var checkRunResponse model.CheckRunResponse
	var prReviews []model.ReviewPullRequestResponse

	wg.Add(4)
	go func() {
		defer wg.Done()
		pullRequestResponse = pr.GetPullRequestInfo(ctx, orgName, repoName, prNumber)
	}()
	go func() {
		defer wg.Done()
		commitResponse = commit.GetCommitInfo(ctx, orgName, repoName, lastSha)
	}()
	go func() {
		defer wg.Done()
		checkRunResponse = commit.GetCommitCheckRuns(ctx, orgName, repoName, lastSha)
	}()
	go func() {
		defer wg.Done()
		prReviews = pr.ListPullRequestReviews(ctx, orgName, repoName, prNumber)
	}()

	wg.Wait()
	return pullRequestResponse, commitResponse, checkRunResponse, prReviews
}
