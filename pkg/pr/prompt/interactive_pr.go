package prompt

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	ltable "github.com/charmbracelet/lipgloss/table"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/prady-lab/sgh-cli/pkg/pr"
	"github.com/prady-lab/sgh-cli/pkg/ui"
)

const (
	mergeableTitle      = "Mergeable ?"
	mergeableStateTitle = "Mergeable State"
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

type listKeyMap struct {
	refresh key.Binding
}

func newListKeyMap() *listKeyMap {
	return &listKeyMap{
		refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh pr list"),
		),
	}
}

type prModel struct {
	list           list.Model
	keys           *listKeyMap
	delegateKeys   *delegateKeyMap
	showEventPanel bool
	sections       []string
	showSpinner    bool
	spinner        spinner.Model
	ctx            *context.Context
	prRequest      pr.PRRequest
}

type eventStatusResponse struct {
	eventType                string
	pullRequestResponse      model.PullRequestResponse
	pullRequestFilesResponse model.PullRequestFilesResponse
	checkRunResponse         model.CheckRunResponse
	prReviews                []model.ReviewPullRequestResponse
	actionMessage            string
	actionSuccess            bool
}

type (
	sectionEvent []string
	refreshEvent []model.PullRequestResponse
)

func newModel(ctx *context.Context, prRequest pr.PRRequest) prModel {
	pullRequests := pr.ListPullRequests(ctx, prRequest)
	items := make([]list.Item, len(pullRequests))
	for i, pr := range pullRequests {
		items[i] = pr
	}

	delegateKeys := newDelegateKeyMap()
	listKeys := newListKeyMap()

	delegate := newItemDelegate(ctx, prRequest.OrgName, delegateKeys)
	prList := list.New(items, delegate, 0, 0)
	prList.Title = "Interactive Pull Request options"
	prList.Styles.Title = titleStyle
	prList.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			listKeys.refresh,
		}
	}
	prList.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			listKeys.refresh,
		}
	}

	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = lipgloss.NewStyle().Foreground(ui.Green)

	return prModel{
		list:           prList,
		keys:           listKeys,
		delegateKeys:   delegateKeys,
		showEventPanel: false,
		showSpinner:    false,
		spinner:        s,
		ctx:            ctx,
		prRequest:      prRequest,
	}
}

func (m prModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m prModel) View() string {
	if !m.showEventPanel {
		return listStyle.Render(m.list.View())
	} else {
		if m.showSpinner {
			return lipgloss.JoinHorizontal(
				lipgloss.Top,
				listStyle.Render(m.list.View()),
				statusModelStyle.Render(fmt.Sprintf("\n\n  %s Processing ...\n\n", m.spinner.View())),
			)
		} else {
			return lipgloss.JoinHorizontal(
				lipgloss.Top,
				listStyle.Render(m.list.View()),
				statusModelStyle.Render(lipgloss.JoinVertical(lipgloss.Center, m.sections...)),
			)
		}
	}
}

func (m prModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case eventMsg:
		eventType := msg.eventType
		m.showEventPanel = true
		m.showSpinner = true

		cmd := func() tea.Msg {
			sections := <-processEventAndGetSectionRenders(msg.ctx, msg.orgName, msg.repoName, msg.selectedPR.PRNumber, msg.selectedPR.Head.Sha, eventType)
			return sectionEvent(sections)
		}
		cmds = append(cmds, cmd)

	case sectionEvent:
		m.sections = []string(msg)
		m.showSpinner = false

	case refreshEvent:
		pullRequests := msg
		for i, pr := range pullRequests {
			m.list.InsertItem(i, pr)
		}
		m.showSpinner = false
		m.showEventPanel = false

	case tea.WindowSizeMsg:
		h, v := listStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch {
		case key.Matches(msg, m.keys.refresh):
			m.list.SetItems(nil)
			m.showEventPanel = true
			m.sections = nil
			m.showSpinner = true

			cmd := func() tea.Msg {
				return refreshEvent(pr.ListPullRequests(m.ctx, m.prRequest))
			}
			cmds = append(cmds, cmd)
		}
		if key.Matches(msg, m.list.KeyMap.CursorDown, m.list.KeyMap.CursorUp, m.list.KeyMap.Filter, m.list.KeyMap.ClearFilter, m.list.KeyMap.Quit, m.list.KeyMap.GoToStart, m.list.KeyMap.GoToEnd) {
			m.showEventPanel = false
			m.sections = nil
		}
	}

	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, cmd)
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func RunInteractivePR(ctx *context.Context, prRequest pr.PRRequest) error {
	m := newModel(ctx, prRequest)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func processEventAndGetSectionRenders(ctx *context.Context, orgName, repoName string, prNumber int, lastSha, eventType string) <-chan []string {
	ch := make(chan []string)
	go func() {
		eventStatusResponse := processEventMsg(ctx, orgName, repoName, prNumber, lastSha, eventType)
		sections := getSectionsRenders(eventStatusResponse)
		ch <- sections
	}()
	return ch
}

func processEventMsg(ctx *context.Context, orgName, repoName string, prNumber int, lastSha, eventType string) eventStatusResponse {
	logger.Flog.Info().Str("org", orgName).Str("repo", repoName).Int("pr", prNumber).Str("eventType", eventType).Msg("Processing Event")
	var actionMessage string
	actionSuccess := true
	pullRequestResponse, pullRequestFilesResponse, checkRunResponse, prReviews := pr.GetPRDetailsGraphQL(ctx, orgName, repoName, prNumber, lastSha)
	switch eventType {
	case "APPROVE":
		actionMessage, actionSuccess = approvePR(ctx, orgName, repoName, prNumber, pullRequestResponse)
		if actionSuccess {
			pullRequestResponse, pullRequestFilesResponse, checkRunResponse, prReviews = pr.GetPRDetailsGraphQL(ctx, orgName, repoName, prNumber, lastSha)
		}
	case "MERGE", "APPROVE_MERGE":
		if eventType == "APPROVE_MERGE" {
			actionMessage, actionSuccess = approvePR(ctx, orgName, repoName, prNumber, pullRequestResponse)
		}

		if actionSuccess {
			actionMessage, actionSuccess = mergePR(ctx, orgName, repoName, prNumber, pullRequestResponse, prReviews, eventType)
			if actionSuccess {
				pullRequestResponse, pullRequestFilesResponse, checkRunResponse, prReviews = pr.GetPRDetailsGraphQL(ctx, orgName, repoName, prNumber, lastSha)
			}
		}
	}
	return eventStatusResponse{eventType: eventType, pullRequestResponse: pullRequestResponse, pullRequestFilesResponse: pullRequestFilesResponse, checkRunResponse: checkRunResponse, prReviews: prReviews, actionMessage: actionMessage, actionSuccess: actionSuccess}
}

func canApprovePR(prResponse model.PullRequestResponse) bool {
	return prResponse.State == "OPEN" && prResponse.Mergeable == "MERGEABLE"
}

func approvePR(ctx *context.Context, orgName, repoName string, prNumber int, prResponse model.PullRequestResponse) (string, bool) {
	if canApprovePR(prResponse) {
		reviewResponse := pr.ReviewPullRequest(ctx, orgName, repoName, prNumber, "APPROVE", "Changes approved")
		if reviewResponse.ErrorMessage != "" {
			return reviewResponse.ErrorMessage, false
		}
		logger.Flog.Info().Str("org", orgName).Str("repo", repoName).Int("pr", prNumber).Msg("PR Approved successfully")
		return "PR Approved successfully", true
	} else {
		logger.Flog.Error().Str("org", orgName).Str("repo", repoName).Int("pr", prNumber).Str("mergeStateStatus", prResponse.MergeStateStatus).Str("mergeable", prResponse.Mergeable).Str("state", prResponse.State).Msgf("PR cannot be approved at this moment")
		return "PR cannot be approved at this moment", false
	}
}

func canMergePR(prResponse model.PullRequestResponse, prReviews []model.ReviewPullRequestResponse, eventType string) bool {
	if !canApprovePR(prResponse) {
		return false
	}
	if prResponse.MergeStateStatus == "CLEAN" || (eventType == "APPROVE_MERGE" && prResponse.MergeStateStatus == "BLOCKED") {
		if eventType != "APPROVE_MERGE" && (len(prReviews) == 0 || prReviews[0].State != "APPROVED") {
			logger.Flog.Error().Str("state", prResponse.MergeStateStatus).Str("eventType", eventType).Int("prReviews", len(prReviews)).Msgf("PR cannot be merged at this moment")
			return false
		}
		return true
	}
	return false
}

func mergePR(ctx *context.Context, orgName, repoName string, prNumber int, prResponse model.PullRequestResponse, prReviews []model.ReviewPullRequestResponse, eventType string) (string, bool) {
	if canMergePR(prResponse, prReviews, eventType) {
		title := "Merge pull request #" + strconv.Itoa(prNumber) + " from " + orgName + "/" + prResponse.Head.Ref
		mergeResponse := pr.MergePullRequest(ctx, orgName, repoName, prNumber, title, prResponse.TitleName)
		if mergeResponse.ErrorMessage != "" {
			return mergeResponse.ErrorMessage, false
		}
		return "PR Merged successfully", true
	} else {
		logger.Flog.Error().Str("org", orgName).Str("repo", repoName).Int("pr", prNumber).Str("MergeStateStatus", prResponse.MergeStateStatus).Msgf("PR cannot be merged at this moment")
		return "PR is not mergeable", false
	}
}

func getSectionsRenders(status eventStatusResponse) []string {
	var sections []string

	titleView := lipgloss.NewStyle().Foreground(ui.White).Background(lipgloss.Color(ui.CrayolaGreen)).Padding(0, 1).Align(lipgloss.Center)

	sections = append(sections, titleView.Render(status.pullRequestResponse.Title()))
	sections = append(sections, "\n")
	sections = append(sections, getPRResponseTable(status.pullRequestResponse, status.pullRequestFilesResponse))

	if len(status.checkRunResponse.CheckRuns) > 0 {
		var finalStatus string
		switch status.checkRunResponse.OverallConclusion {
		case "SUCCESS":
			finalStatus = lipgloss.NewStyle().Foreground(ui.Green).Bold(true).Render(status.checkRunResponse.OverallConclusion)
		case "FAILURE":
			finalStatus = lipgloss.NewStyle().Foreground(ui.Red).Bold(true).Render(status.checkRunResponse.OverallConclusion)
		default:
			finalStatus = lipgloss.NewStyle().Foreground(ui.Gray).Bold(true).Render(status.checkRunResponse.OverallConclusion)
		}
		sections = append(sections, lipgloss.NewStyle().Foreground(ui.White).Bold(true).Render("Check Runs: ")+finalStatus)
		sections = append(sections, getCheckRunsTable(status.checkRunResponse))
	}

	if len(status.prReviews) > 0 {
		sections = append(sections, "\n")
		sections = append(sections, lipgloss.NewStyle().Foreground(ui.White).Bold(true).Render("Reviews Status"))
		sections = append(sections, getReviewTable(status.prReviews))
	}

	if status.actionMessage != "" {
		sections = append(sections, "\n")
		sections = append(sections, lipgloss.NewStyle().Foreground(ui.White).Bold(true).Render("Action Status: "+status.eventType))
		if status.actionSuccess {
			sections = append(sections, lipgloss.NewStyle().Foreground(ui.Green).BorderStyle(lipgloss.RoundedBorder()).PaddingLeft(10).PaddingRight(10).Bold(true).Blink(true).Render(status.actionMessage))
		} else {
			sections = append(sections, lipgloss.NewStyle().Foreground(ui.Red).BorderStyle(lipgloss.RoundedBorder()).PaddingLeft(10).PaddingRight(10).Bold(true).Render(status.actionMessage))
		}
	}

	return sections
}

func getPRResponseTable(prResponse model.PullRequestResponse, pullRequestFilesResponse model.PullRequestFilesResponse) string {
	responseRows := make([][]string, 0)
	modifiedFiles := make([]string, 0)

	if len(pullRequestFilesResponse.Files) > 0 {
		for _, file := range pullRequestFilesResponse.Files {
			modifiedFiles = append(modifiedFiles, file.Filename)
			if len(modifiedFiles) == 5 {
				modifiedFiles = append(modifiedFiles, "...")
				break
			}
		}
	}

	responseRows = append(responseRows, []string{"PR Number", strconv.Itoa(prResponse.PRNumber)})
	responseRows = append(responseRows, []string{"Repository", prResponse.RepositoryName()})
	responseRows = append(responseRows, []string{"User", prResponse.AuthorName()})
	responseRows = append(responseRows, []string{"Assignees", strings.ReplaceAll(prResponse.AssigneesName(), "\n", ", ")})
	responseRows = append(responseRows, []string{"Reviewers", strings.ReplaceAll(prResponse.ReviewersName(), "\n", ", ")})
	responseRows = append(responseRows, []string{"State", prResponse.State})
	responseRows = append(responseRows, []string{mergeableTitle, prResponse.Mergeable})
	responseRows = append(responseRows, []string{mergeableStateTitle, prResponse.MergeStateStatus})
	responseRows = append(responseRows, []string{"Merged At", prResponse.MergeAt})
	responseRows = append(responseRows, []string{"Merged By", prResponse.MergedBy.Login})
	responseRows = append(responseRows, []string{"Review comments", strconv.Itoa(prResponse.ReviewComments)})
	responseRows = append(responseRows, []string{"Comments", strconv.Itoa(prResponse.Comments)})
	responseRows = append(responseRows, []string{"Commits", strconv.Itoa(prResponse.Commits)})
	responseRows = append(responseRows, []string{"Total files #", strconv.Itoa(prResponse.ChangedFiles)})
	responseRows = append(responseRows, []string{"Files changed", strings.Join(modifiedFiles, "\n")})
	responseRows = append(responseRows, []string{"Changes", strconv.Itoa(prResponse.Additions) + " Additions, " + strconv.Itoa(prResponse.Deletions) + " Deletions"})
	responseRows = append(responseRows, []string{"Review link", fmt.Sprintf(ui.HyperLinkFormat, prResponse.HTMLUrl, "Open")})

	responseTable := ltable.New().
		Border(lipgloss.HiddenBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := getPRTableStyle(col, responseRows, row)
			return style
		}).
		Rows(responseRows...)

	responseTableView := lipgloss.NewStyle().BorderForeground(ui.White)
	tableRender := responseTable.String()

	return responseTableView.Render(tableRender)
}

func getPRTableStyle(col int, responseRows [][]string, row int) lipgloss.Style {
	style := CellStyle

	if col == 0 {
		style = style.Foreground(ui.Gray).AlignHorizontal(lipgloss.Right)
	} else {
		style = style.Foreground(ui.White)
		if responseRows[row][0] == mergeableTitle && responseRows[row][1] == "true" {
			style = style.Foreground(lipgloss.Color(ui.CrayolaGreen)).Blink(true)
		} else if responseRows[row][0] == mergeableTitle && responseRows[row][1] == "false" {
			style = style.Foreground(lipgloss.Color(ui.Red))
		} else if responseRows[row][0] == mergeableStateTitle && responseRows[row][1] == "clean" {
			style = style.Foreground(lipgloss.Color(ui.Green)).Blink(true)
		} else if responseRows[row][0] == mergeableStateTitle {
			style = style.Foreground(lipgloss.Color(ui.Red))
		}
	}
	return style
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

			if row == -1 {
				return HeaderStyle
			}
			if col == 0 {
				style = CellStyle.Foreground(ui.Gray).AlignHorizontal(lipgloss.Left)
			} else if col == 2 && row != -1 {
				style = CellStyle.Foreground(ui.White)
				switch checkRunRows[row][2] {
				case "SUCCESS":
					style = style.Foreground(lipgloss.Color(ui.Green))
				case "FAILURE":
					style = style.Foreground(lipgloss.Color(ui.Red))
				case "SKIPPED":
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
		prReviewsRows = append(prReviewsRows, []string{review.User.Name, review.State, review.SubmittedAt, review.Body})
	}

	reviewsTable := ltable.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := CellStyle

			if row == -1 {
				return HeaderStyle
			}
			if col == 0 {
				style = CellStyle.Foreground(ui.Gray).AlignHorizontal(lipgloss.Left)
			} else if col == 1 && row == 0 {
				switch prReviewsRows[row][1] {
				case "APPROVED":
					style = style.Foreground(lipgloss.Color(ui.Green)).Blink(true)
				case "CHANGES_REQUESTED":
					style = style.Foreground(lipgloss.Color(ui.Red))
				default:
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
