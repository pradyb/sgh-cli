package prompt

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/prady-lab/sgh-cli/internal/model"
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
)

type prModel struct {
	list           list.Model
	delegateKeys   *delegateKeyMap
	showEventPanel bool
	sections       []string
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
		m.showEventPanel = true
		var actionMessage string
		actionFailed := true
		pullRequestResponse, commitResponse, checkRunResponse, prReviews := pr.GetPRDetails(msg.ctx, msg.orgName, msg.repoName, msg.selectedPR.PRNumber, msg.selectedPR.Head.Sha)
		if eventType == "APPROVE" {
			if canApprove(pullRequestResponse, prReviews) {
				reviewResponse := pr.ReviewPullRequest(msg.ctx, msg.orgName, msg.repoName, msg.selectedPR.PRNumber, "APPROVE", "Changes approved")
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
		m.sections = getPRStatusSections(pullRequestResponse, commitResponse, checkRunResponse, prReviews, eventType, actionMessage, actionFailed)
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

func canApprove(prResponse model.PullRequestResponse, _ []model.ReviewPullRequestResponse) bool {
	return prResponse.Mergeable
}

func getPRStatusSections(prResponse model.PullRequestResponse, commitResponse model.CommitResponse, checkRunResponse model.CheckRunResponse, prReviews []model.ReviewPullRequestResponse, eventType, actionResponse string, actionFailed bool) []string {
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

	if actionResponse != "" {
		sections = append(sections, "\n")
		sections = append(sections, lipgloss.NewStyle().Foreground(ui.White).Bold(true).Render("Action Status: "+eventType))
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
			style := CellStyle

			if col == 0 {
				style = style.Foreground(ui.Gray).AlignHorizontal(lipgloss.Right)
			} else {
				style = style.Foreground(ui.White)
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
			style := CellStyle

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
		prReviewsRows = append(prReviewsRows, []string{review.User.Login, review.State, review.SubmittedAt, review.Body})
	}

	reviewsTable := ltable.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := CellStyle

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
		Headers("Reviewed By", "State", "Submitted At", "Body").
		Rows(prReviewsRows...)

	reviewsTableView := lipgloss.NewStyle().BorderForeground(ui.White)
	return reviewsTableView.Render(reviewsTable.String())
}
