package prompt

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/pkg/commit"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/pr"
	"github.com/prady-lab/sgh-cli/pkg/ui"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	ltable "github.com/charmbracelet/lipgloss/table"
)

const (
	mergeableTitle = "Mergeable ?"
)

var (
	listStyle = lipgloss.NewStyle().Padding(1, 2)

	statusModelStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(ui.White).
			Background(lipgloss.Color(ui.CrayolaGreen)).
			Padding(0, 1)

	statusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}).
				Render

	re          = lipgloss.NewRenderer(os.Stdout)
	CellStyle   = re.NewStyle().Padding(0, 1)
	BorderStyle = lipgloss.NewStyle().Foreground(ui.White).BorderBottom(true)
	HeaderStyle = re.NewStyle().Foreground(ui.White).Bold(true).Align(lipgloss.Center)

	actionItemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	actionSelectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
)

type prModel struct {
	list           list.Model
	delegateKeys   *delegateKeyMap
	showEventPanel bool
	sections       []string

	//actionList     list.Model
	//selectedAction string
	//actionExit     bool
}

func newModel(ctx *context.Context, orgName string, repoNames []string, baseRef, headRef string, all bool) prModel {
	pullRequests := pr.ListPullRequests(ctx, orgName, repoNames, baseRef, headRef, all)
	items := make([]list.Item, len(pullRequests))
	for i, pr := range pullRequests {
		items[i] = pr
	}

	var delegateKeys = newDelegateKeyMap()

	delegate := newItemDelegate(ctx, orgName, delegateKeys)
	prList := list.New(items, delegate, 0, 0)
	prList.Title = "Interactive Pull Request options"
	prList.Styles.Title = titleStyle

	return prModel{
		list:           prList,
		delegateKeys:   delegateKeys,
		showEventPanel: false,
	}
}

func (m prModel) Init() tea.Cmd {
	return nil
}

func (m prModel) View() string {
	if !m.showEventPanel {
		return listStyle.Render(m.list.View())
	} else {
		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			listStyle.Render(m.list.View()),
			statusModelStyle.Render(lipgloss.JoinVertical(lipgloss.Center, m.sections...)),
		)
	}
}

func (m prModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case eventMsg:
		eventType := msg.eventType
		switch eventType {
		case "STATUS", "APPROVE":
			m.showEventPanel = true
			var reviewResponse model.ReviewPullRequestResponse
			var actionMessage string
			actionFailed := true
			pullRequestResponse, commitResponse, checkRunResponse, prReviews := getPRDetails(msg.ctx, msg.orgName, msg.repoName, msg.selectedPR.PRNumber, msg.selectedPR.Head.Sha)
			if eventType == "APPROVE" {
				if canApprove(msg.selectedPR, prReviews) {
					reviewResponse = pr.ReviewPullRequest(msg.ctx, msg.orgName, msg.repoName, msg.selectedPR.PRNumber, "APPROVE", "Changes approved")
					if reviewResponse.ErrorMessage != "" {
						actionMessage = reviewResponse.ErrorMessage
					} else {
						actionMessage = "PR Approved successfully"
						prReviews = pr.ListPullRequestReviews(msg.ctx, msg.orgName, msg.repoName, msg.selectedPR.PRNumber)
						actionFailed = false
					}
				} else {
					actionMessage = "PR is not mergeable"
				}
			}
			m.sections = statusSections(pullRequestResponse, commitResponse, checkRunResponse, prReviews, actionMessage, actionFailed)
		case "MERGE":
			m.showEventPanel = true
		case "APPROVE_MERGE":
			m.showEventPanel = true
		}
	case tea.WindowSizeMsg:
		h, v := listStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}
		m.showEventPanel = false
		m.sections = nil
	}

	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func RunInteractivePR(ctx *context.Context, orgName string, repoNames []string, baseRef, headRef string, all bool) error {
	m := newModel(ctx, orgName, repoNames, baseRef, headRef, all)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
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

func canApprove(prResponse model.PullRequestResponse, _ []model.ReviewPullRequestResponse) bool {
	return prResponse.Mergeable
}

func statusSections(prResponse model.PullRequestResponse, commitResponse model.CommitResponse, checkRunResponse model.CheckRunResponse, prReviews []model.ReviewPullRequestResponse, actionResponse string, actionFailed bool) []string {
	var sections []string

	titleView := lipgloss.NewStyle().Foreground(ui.White).Background(lipgloss.Color(ui.CrayolaGreen)).Padding(0, 1).Align(lipgloss.Center)

	sections = append(sections, titleView.Render(prResponse.Title()))
	sections = append(sections, "\n")
	sections = append(sections, getPRResponseTable(prResponse, commitResponse))

	if len(checkRunResponse.CheckRuns) > 0 {
		sections = append(sections, lipgloss.NewStyle().Foreground(ui.White).Bold(true).Render("Check Runs"))
		sections = append(sections, getCheckRunsTable(checkRunResponse))
	}

	if len(prReviews) > 0 {
		sections = append(sections, "\n")
		sections = append(sections, lipgloss.NewStyle().Foreground(ui.White).Bold(true).Render("Reviews Status"))
		sections = append(sections, getReviewTable(prReviews))
	}

	//sections = append(sections, "\n")
	//sections = append(sections, lipgloss.NewStyle().Foreground(ui.White).Bold(true).Render("Actions"))
	//sections = append(sections, getActionsList(prResponse))

	if actionResponse != "" {
		sections = append(sections, "\n")
		sections = append(sections, lipgloss.NewStyle().Foreground(ui.White).Bold(true).Render("Action Status"))
		if !actionFailed {
			sections = append(sections, lipgloss.NewStyle().Foreground(ui.Green).BorderStyle(lipgloss.RoundedBorder()).PaddingLeft(10).PaddingRight(10).Bold(true).Blink(true).Render(actionResponse))
		} else {
			sections = append(sections, lipgloss.NewStyle().Foreground(ui.Red).BorderStyle(lipgloss.RoundedBorder()).PaddingLeft(10).PaddingRight(10).Bold(true).Render(actionResponse))
		}
	}

	return sections
}

func getPRResponseTable(prResponse model.PullRequestResponse, commitResponse model.CommitResponse) string {
	responseRows := make([][]string, 0)
	modifiedFiles := make([]string, 0)

	if len(commitResponse.Files) > 0 {
		for _, file := range commitResponse.Files {
			modifiedFiles = append(modifiedFiles, file.Filename)
			if len(modifiedFiles) == 5 {
				modifiedFiles = append(modifiedFiles, "...")
				break
			}
		}
	}

	responseRows = append(responseRows, []string{"PR Number", strconv.Itoa(prResponse.PRNumber)})
	responseRows = append(responseRows, []string{"Repository", prResponse.RepositoryName()})
	responseRows = append(responseRows, []string{"User", prResponse.UserName()})
	responseRows = append(responseRows, []string{"Assignees", strings.ReplaceAll(prResponse.AssigneesName(), "\n", ", ")})
	responseRows = append(responseRows, []string{"Reviewers", strings.ReplaceAll(prResponse.ReviewersName(), "\n", ", ")})
	responseRows = append(responseRows, []string{mergeableTitle, strconv.FormatBool(prResponse.Mergeable)})
	responseRows = append(responseRows, []string{"Mergeable State", prResponse.MergeableState})
	responseRows = append(responseRows, []string{"Review comments", strconv.Itoa(prResponse.ReviewComments)})
	responseRows = append(responseRows, []string{"Comments", strconv.Itoa(prResponse.Comments)})
	responseRows = append(responseRows, []string{"Commits", strconv.Itoa(prResponse.Commits)})
	responseRows = append(responseRows, []string{"Total files #", strconv.Itoa(prResponse.ChangedFiles)})
	responseRows = append(responseRows, []string{"Modified files in last commit", strings.Join(modifiedFiles, "\n")})
	responseRows = append(responseRows, []string{"Changes", strconv.Itoa(prResponse.Additions) + " Additions, " + strconv.Itoa(prResponse.Deletions) + " Deletions"})
	responseRows = append(responseRows, []string{"Review link", fmt.Sprintf(ui.HyperLinkFormat, prResponse.HTMLUrl, "Open")})

	responseTable := ltable.New().
		Border(lipgloss.HiddenBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			var style lipgloss.Style

			if col == 0 {
				style = CellStyle.Foreground(ui.Gray).AlignHorizontal(lipgloss.Right)
			} else {
				style = CellStyle.Foreground(ui.White)
				if responseRows[row-1][0] == mergeableTitle && responseRows[row-1][1] == "true" {
					style = style.Foreground(lipgloss.Color(ui.CrayolaGreen)).Blink(true)
				} else if responseRows[row-1][0] == mergeableTitle && responseRows[row-1][1] == "false" {
					style = style.Foreground(lipgloss.Color(ui.Red))
				}
			}

			return style
		}).
		Rows(responseRows...)

	responseTableView := lipgloss.NewStyle().BorderForeground(ui.White)
	tableRender := responseTable.String()

	return responseTableView.Render(tableRender)
}

func getCheckRunsTable(checkRunResponse model.CheckRunResponse) string {
	checkRunRows := make([][]string, 0)
	for _, checkRun := range checkRunResponse.CheckRuns {
		checkRunRows = append(checkRunRows, []string{checkRun.Name, checkRun.Status, checkRun.Conclusion, checkRun.StartedAt, checkRun.CompletedAt})
	}

	checkRunTable := ltable.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			var style lipgloss.Style

			if row == 0 {
				return HeaderStyle
			}
			if col == 0 {
				style = CellStyle.Foreground(ui.Gray).AlignHorizontal(lipgloss.Left)
			} else if col == 2 {
				style = CellStyle.Foreground(ui.White)
				if checkRunRows[row-1][2] == "success" {
					style = style.Foreground(lipgloss.Color(ui.Green)).Blink(true)
				} else if checkRunRows[row-1][2] == "failure" {
					style = style.Foreground(lipgloss.Color(ui.Red))
				} else if checkRunRows[row-1][2] == "skipped" {
					style = style.Foreground(lipgloss.Color("#FFA500"))
				}
			}

			return style
		}).
		Headers("Name", "Status", "Conclusion", "Started At", "Completed At").
		Rows(checkRunRows...)

	checkRunTableView := lipgloss.NewStyle().BorderForeground(ui.White)
	return checkRunTableView.Render(checkRunTable.String())
}

func getReviewTable(prReviews []model.ReviewPullRequestResponse) string {
	prReviewsRows := make([][]string, 0)

	sort.Slice(prReviews, func(i, j int) bool {
		submittedAtI, _ := time.Parse(time.RFC3339, prReviews[i].SubmittedAt)
		submittedAtJ, _ := time.Parse(time.RFC3339, prReviews[j].SubmittedAt)
		return submittedAtI.After(submittedAtJ)
	})
	for _, review := range prReviews {
		prReviewsRows = append(prReviewsRows, []string{review.User.Login, review.State, review.SubmittedAt, review.CommitId})
	}

	reviewsTable := ltable.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			var style lipgloss.Style

			if row == 0 {
				return HeaderStyle
			}
			if col == 0 {
				style = CellStyle.Foreground(ui.Gray).AlignHorizontal(lipgloss.Left)
			} else if col == 1 && row == 1 {
				if prReviewsRows[row-1][1] == "APPROVED" {
					style = style.Foreground(lipgloss.Color(ui.Green)).Blink(true)
				} else if prReviewsRows[row-1][1] == "CHANGES_REQUESTED" {
					style = style.Foreground(lipgloss.Color(ui.Red))
				} else {
					style = style.Foreground(lipgloss.Color("#FFA500"))
				}
			}

			return style
		}).
		Headers("Reviewed By", "State", "Submitted At", "Commit Id").
		Rows(prReviewsRows...)

	reviewsTableView := lipgloss.NewStyle().BorderForeground(ui.White)
	return reviewsTableView.Render(reviewsTable.String())
}

type item string

func (i item) FilterValue() string { return "" }

func getActionsList(prResponse model.PullRequestResponse) string {

	actions := make([]string, 0)

	actions = append(actions, "No Action")
	if prResponse.Mergeable {
		actions = append(actions, "Approve")
		actions = append(actions, "Merge")
		actions = append(actions, "Approve and Merge")
	} else {
		actions = append(actions, "Approve")
	}

	items := make([]list.Item, len(actions))
	for i, action := range actions {
		items[i] = item(action)
	}

	l := list.New(items, itemDelegate{}, 0, 0)
	l.Title = "Actions ?"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle

	return listStyle.Render(l.View())
}

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%d. %s", index+1, i)

	fn := actionItemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return actionSelectedItemStyle.Render("> " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}
