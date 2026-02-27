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
	maxBodyPreview      = 200
)

var (
	listStyle = lipgloss.NewStyle().Padding(1, 0, 1, 1)

	statusModelStyle = lipgloss.NewStyle().
				Padding(1, 0, 1, 1).
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(ui.Dimmed)

	helpBarStyle = lipgloss.NewStyle().Padding(0, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(ui.White).
			Background(ui.CrayolaGreen).
			Padding(0, 1)

	statusMessageStyle = lipgloss.NewStyle().
				Foreground(ui.CrayolaGreen).
				Render

	re                = lipgloss.NewRenderer(os.Stdout)
	CellStyle         = re.NewStyle().Padding(0, 1)
	promptBorderStyle = lipgloss.NewStyle().Foreground(ui.Dimmed)
	promptHeaderStyle = re.NewStyle().Foreground(ui.Cyan).Bold(true).Align(lipgloss.Center)
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
	termWidth      int
	termHeight     int
	confirmPending *confirmMsg
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
	for i, prItem := range pullRequests {
		items[i] = prItem
	}

	delegateKeys := newDelegateKeyMap()
	listKeys := newListKeyMap()

	delegate := newItemDelegate(ctx, prRequest.OrgName, delegateKeys)
	prList := list.New(items, delegate, 0, 0)
	prList.Title = "Interactive Pull Request Options"
	prList.Styles.Title = titleStyle
	prList.SetShowHelp(false)
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
	s.Style = lipgloss.NewStyle().Foreground(ui.Cyan)

	return prModel{
		list:           prList,
		keys:           listKeys,
		delegateKeys:   delegateKeys,
		showEventPanel: false,
		showSpinner:    false,
		spinner:        s,
		ctx:            ctx,
		prRequest:      prRequest,
		termWidth:      ui.TerminalWidth(),
	}
}

func (m *prModel) resizeList() {
	if m.termWidth == 0 {
		return
	}
	h, v := listStyle.GetFrameSize()
	listWidth := m.termWidth - h
	if m.showEventPanel {
		listWidth = m.termWidth*2/5 - h
	}
	m.list.SetSize(listWidth, m.termHeight-v)
}

func (m prModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m prModel) helpView() string {
	m.list.Help.Width = m.termWidth
	return helpBarStyle.Render(m.list.Help.View(m.list))
}

func (m prModel) View() string {
	helpBar := m.helpView()

	if m.confirmPending != nil {
		listView := listStyle.Render(m.list.View())
		confirmStyle := lipgloss.NewStyle().
			Foreground(ui.Yellow).
			Bold(true).
			Padding(0, 2)
		confirmBar := confirmStyle.Render("  " + m.confirmPending.prompt)
		return lipgloss.JoinVertical(lipgloss.Left, listView, confirmBar, helpBar)
	}

	if !m.showEventPanel {
		listView := listStyle.Render(m.list.View())
		return lipgloss.JoinVertical(lipgloss.Left, listView, helpBar)
	}

	listWidth := m.termWidth * 2 / 5
	if listWidth < 30 {
		listWidth = 30
	}
	listView := listStyle.Width(listWidth).Render(m.list.View())

	panelWidth := m.termWidth - listWidth - 3
	if panelWidth < 50 {
		panelWidth = 50
	}
	panelStyle := statusModelStyle.Width(panelWidth)

	var content string
	if m.showSpinner {
		content = lipgloss.JoinHorizontal(
			lipgloss.Top,
			listView,
			panelStyle.Render(fmt.Sprintf("\n\n  %s Processing ...\n\n", m.spinner.View())),
		)
	} else {
		content = lipgloss.JoinHorizontal(
			lipgloss.Top,
			listView,
			panelStyle.Render(lipgloss.JoinVertical(lipgloss.Center, m.sections...)),
		)
	}
	return lipgloss.JoinVertical(lipgloss.Left, content, helpBar)
}

func (m prModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle confirmation prompt keys first
	if m.confirmPending != nil {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "y", "Y":
				pending := m.confirmPending
				m.confirmPending = nil
				m.showEventPanel = true
				m.showSpinner = true
				m.resizeList()
				statusMsgCmd := m.list.NewStatusMessage(statusMessageStyle("Processing " + pending.eventType + "..."))
				eventCmd := func() tea.Msg {
					return eventMsg{eventType: pending.eventType, selectedPR: pending.selectedPR, ctx: pending.ctx, orgName: pending.orgName, repoName: pending.repoName}
				}
				return m, tea.Batch(statusMsgCmd, eventCmd)
			default:
				m.confirmPending = nil
				statusMsgCmd := m.list.NewStatusMessage(statusMessageStyle("Cancelled"))
				newListModel, cmd := m.list.Update(msg)
				m.list = newListModel
				return m, tea.Batch(statusMsgCmd, cmd)
			}
		}
	}

	switch msg := msg.(type) {
	case confirmMsg:
		m.confirmPending = &msg
		return m, nil

	case browserOpenMsg:
		if msg.url != "" {
			_ = openURL(msg.url)
		}
		return m, nil

	case eventMsg:
		eventType := msg.eventType
		m.showEventPanel = true
		m.showSpinner = true
		m.resizeList()

		cmd := func() tea.Msg {
			sections := <-processEventAndGetSectionRenders(msg.ctx, msg.orgName, msg.repoName, msg.selectedPR.PRNumber, msg.selectedPR.Head.Sha, eventType)
			return sectionEvent(sections)
		}
		cmds = append(cmds, cmd)

	case sectionEvent:
		m.sections = []string(msg)
		m.showSpinner = false

	case refreshEvent:
		items := make([]list.Item, len(msg))
		for i, prItem := range msg {
			items[i] = prItem
		}
		m.list.SetItems(items)
		m.showSpinner = false
		m.showEventPanel = false
		m.resizeList()

	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		m.resizeList()

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
			m.resizeList()

			cmd := func() tea.Msg {
				return refreshEvent(pr.ListPullRequests(m.ctx, m.prRequest))
			}
			cmds = append(cmds, cmd)
		}
		if key.Matches(msg, m.list.KeyMap.CursorDown, m.list.KeyMap.CursorUp, m.list.KeyMap.Filter, m.list.KeyMap.ClearFilter, m.list.KeyMap.Quit, m.list.KeyMap.GoToStart, m.list.KeyMap.GoToEnd) {
			m.showEventPanel = false
			m.sections = nil
			m.resizeList()
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

	req := pr.PRDetailsRequest{
		OrgName:  orgName,
		RepoName: repoName,
		PRNumber: prNumber,
		LastSHA:  lastSha,
	}
	pullRequestResponse, pullRequestFilesResponse, checkRunResponse, prReviews := pr.GetPRDetailsGraphQL(ctx, req)

	needsRefresh := false
	switch eventType {
	case "APPROVE":
		actionMessage, actionSuccess = approvePR(ctx, orgName, repoName, prNumber, pullRequestResponse)
		needsRefresh = actionSuccess
	case "MERGE", "APPROVE_MERGE":
		if eventType == "APPROVE_MERGE" {
			actionMessage, actionSuccess = approvePR(ctx, orgName, repoName, prNumber, pullRequestResponse)
		}
		if actionSuccess {
			actionMessage, actionSuccess = mergePR(ctx, orgName, repoName, prNumber, pullRequestResponse, prReviews, eventType)
		}
		needsRefresh = actionSuccess
	case "CLOSE":
		actionMessage, actionSuccess = closePR(ctx, orgName, repoName, prNumber, pullRequestResponse)
		needsRefresh = actionSuccess
	}

	if needsRefresh {
		pullRequestResponse, pullRequestFilesResponse, checkRunResponse, prReviews = pr.GetPRDetailsGraphQL(ctx, req)
	}

	return eventStatusResponse{eventType: eventType, pullRequestResponse: pullRequestResponse, pullRequestFilesResponse: pullRequestFilesResponse, checkRunResponse: checkRunResponse, prReviews: prReviews, actionMessage: actionMessage, actionSuccess: actionSuccess}
}

func canApprovePR(prResponse model.PullRequestResponse) bool {
	return prResponse.State == "OPEN" && prResponse.Mergeable == "MERGEABLE"
}

func approvePR(ctx *context.Context, orgName, repoName string, prNumber int, prResponse model.PullRequestResponse) (string, bool) {
	if canApprovePR(prResponse) {
		req := pr.PRReviewRequest{
			OrgName:  orgName,
			RepoName: repoName,
			PRNumber: prNumber,
			Event:    "APPROVE",
			Body:     "Changes approved",
		}
		reviewResponse := pr.ReviewPullRequest(ctx, req)
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

func closePR(ctx *context.Context, orgName, repoName string, prNumber int, prResponse model.PullRequestResponse) (string, bool) {
	if prResponse.State != "OPEN" {
		return "PR is not open — cannot close", false
	}
	req := pr.PRUpdateRequest{
		OrgName:  orgName,
		RepoName: repoName,
		PRNumber: prNumber,
		State:    "closed",
	}
	response := pr.UpdatePullRequest(ctx, req)
	if response.ErrorMessage != "" {
		return response.ErrorMessage, false
	}
	logger.Flog.Info().Str("org", orgName).Str("repo", repoName).Int("pr", prNumber).Msg("PR Closed successfully")
	return "PR Closed successfully", true
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
		req := pr.PRMergeRequest{
			OrgName:  orgName,
			RepoName: repoName,
			PRNumber: prNumber,
			Title:    title,
			Body:     prResponse.TitleName,
		}
		mergeResponse := pr.MergePullRequest(ctx, req)
		if mergeResponse.ErrorMessage != "" {
			return mergeResponse.ErrorMessage, false
		}
		return "PR Merged successfully", true
	} else {
		logger.Flog.Error().Str("org", orgName).Str("repo", repoName).Int("pr", prNumber).Str("MergeStateStatus", prResponse.MergeStateStatus).Msgf("PR cannot be merged at this moment")
		return "PR is not mergeable", false
	}
}

func truncateBody(body string, maxLen int) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	body = strings.ReplaceAll(body, "\r\n", "\n")
	if len(body) > maxLen {
		body = body[:maxLen] + "..."
	}
	return body
}

func getSectionsRenders(status eventStatusResponse) []string {
	var sections []string

	sectionTitle := lipgloss.NewStyle().Foreground(ui.White).Background(ui.CrayolaGreen).Padding(0, 1).Align(lipgloss.Center)
	sectionLabel := lipgloss.NewStyle().Foreground(ui.White).Bold(true)
	bodyStyle := lipgloss.NewStyle().Foreground(ui.Subtle).Italic(true).PaddingLeft(2).PaddingRight(2)

	sections = append(sections, sectionTitle.Render(status.pullRequestResponse.Title()))

	body := truncateBody(status.pullRequestResponse.Body, maxBodyPreview)
	if body != "" {
		sections = append(sections, bodyStyle.Render(body))
	}

	sections = append(sections, "")
	sections = append(sections, getPRResponseTable(status.pullRequestResponse, status.pullRequestFilesResponse))

	if len(status.checkRunResponse.CheckRuns) > 0 {
		conclusionColor := ui.StatusColor(status.checkRunResponse.OverallConclusion)
		finalStatus := lipgloss.NewStyle().Foreground(conclusionColor).Bold(true).Render(status.checkRunResponse.OverallConclusion)
		sections = append(sections, sectionLabel.Render("Check Runs: ")+finalStatus)
		sections = append(sections, getCheckRunsTable(status.checkRunResponse))
	}

	if len(status.prReviews) > 0 {
		sections = append(sections, "")
		sections = append(sections, sectionLabel.Render("Reviews Status"))
		sections = append(sections, getReviewTable(status.prReviews))
	}

	if status.actionMessage != "" {
		sections = append(sections, "")
		sections = append(sections, sectionLabel.Render("Action Status: "+status.eventType))
		msgStyle := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).PaddingLeft(10).PaddingRight(10).Bold(true)
		if status.actionSuccess {
			sections = append(sections, msgStyle.Foreground(ui.Green).Render(status.actionMessage))
		} else {
			sections = append(sections, msgStyle.Foreground(ui.Red).Render(status.actionMessage))
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

	additionsStyle := lipgloss.NewStyle().Foreground(ui.Green)
	deletionsStyle := lipgloss.NewStyle().Foreground(ui.Red)
	changesValue := additionsStyle.Render("+"+strconv.Itoa(prResponse.Additions)) + "  " + deletionsStyle.Render("-"+strconv.Itoa(prResponse.Deletions))

	mergedAt := ui.RelativeTime(prResponse.MergeAt)
	if mergedAt == "" {
		mergedAt = "-"
	}

	responseRows = append(responseRows, []string{"PR Number", strconv.Itoa(prResponse.PRNumber)})
	responseRows = append(responseRows, []string{"Repository", prResponse.RepositoryName()})
	responseRows = append(responseRows, []string{"User", prResponse.AuthorName()})
	responseRows = append(responseRows, []string{"Assignees", strings.ReplaceAll(prResponse.AssigneesName(), "\n", ", ")})
	responseRows = append(responseRows, []string{"Reviewers", strings.ReplaceAll(prResponse.ReviewersName(), "\n", ", ")})
	responseRows = append(responseRows, []string{"State", prResponse.State})
	responseRows = append(responseRows, []string{mergeableTitle, prResponse.Mergeable})
	responseRows = append(responseRows, []string{mergeableStateTitle, prResponse.MergeStateStatus})
	responseRows = append(responseRows, []string{"Merged At", mergedAt})
	responseRows = append(responseRows, []string{"Merged By", prResponse.MergedBy.Login})
	responseRows = append(responseRows, []string{"Review comments", strconv.Itoa(prResponse.ReviewComments)})
	responseRows = append(responseRows, []string{"Comments", strconv.Itoa(prResponse.Comments)})
	responseRows = append(responseRows, []string{"Commits", strconv.Itoa(prResponse.Commits)})
	responseRows = append(responseRows, []string{"Total files #", strconv.Itoa(prResponse.ChangedFiles)})
	responseRows = append(responseRows, []string{"Files changed", strings.Join(modifiedFiles, "\n")})
	responseRows = append(responseRows, []string{"Changes", changesValue})
	responseRows = append(responseRows, []string{"Review link", fmt.Sprintf(ui.HyperLinkFormat, prResponse.HTMLUrl, "Open")})

	responseTable := ltable.New().
		Border(lipgloss.HiddenBorder()).
		BorderStyle(promptBorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			return getPRTableStyle(col, responseRows, row)
		}).
		Rows(responseRows...)

	responseTableView := lipgloss.NewStyle().BorderForeground(ui.Dimmed)
	return responseTableView.Render(responseTable.String())
}

func getPRTableStyle(col int, responseRows [][]string, row int) lipgloss.Style {
	style := CellStyle

	if row < 0 || row >= len(responseRows) {
		return style
	}

	if col == 0 {
		return style.Foreground(ui.Dimmed).AlignHorizontal(lipgloss.Right)
	}

	style = style.Foreground(ui.White)
	label := responseRows[row][0]
	value := responseRows[row][1]
	switch {
	case label == "State":
		style = style.Foreground(ui.StatusColor(value))
	case label == mergeableTitle && value == "MERGEABLE":
		style = style.Foreground(ui.Green)
	case label == mergeableTitle && (value == "CONFLICTING" || value == "UNKNOWN"):
		style = style.Foreground(ui.Red)
	case label == mergeableStateTitle:
		style = style.Foreground(ui.StatusColor(value))
	}
	return style
}

func getCheckRunsTable(checkRunResponse model.CheckRunResponse) string {
	checkRunRows := make([][]string, 0)
	for _, checkRun := range checkRunResponse.CheckRuns {
		checkRunRows = append(checkRunRows, []string{
			checkRun.Name,
			checkRun.Status,
			ui.StatusIcon(checkRun.Conclusion) + " " + checkRun.Conclusion,
			ui.RelativeTime(checkRun.StartedAt),
			ui.RelativeTime(checkRun.CompletedAt),
		})
	}

	checkRunTable := ltable.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(promptBorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == -1 {
				return promptHeaderStyle
			}
			if row < 0 || row >= len(checkRunRows) {
				return CellStyle
			}

			switch col {
			case 0:
				return CellStyle.Foreground(ui.Dimmed)
			case 1:
				return CellStyle.Foreground(ui.StatusColor(checkRunRows[row][1]))
			}
			return CellStyle
		}).
		Headers("Name", "Status", "Conclusion", "Started", "Completed").
		Rows(checkRunRows...)

	checkRunTableView := lipgloss.NewStyle().BorderForeground(ui.Dimmed)
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
		reviewer := review.User.Name
		if reviewer == "" {
			reviewer = review.User.Login
		}
		prReviewsRows = append(prReviewsRows, []string{reviewer, review.State, ui.RelativeTime(review.SubmittedAt), review.Body})
	}

	reviewsTable := ltable.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(promptBorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == -1 {
				return promptHeaderStyle
			}
			if row < 0 || row >= len(prReviewsRows) {
				return CellStyle
			}

			switch col {
			case 0:
				return CellStyle.Foreground(ui.Dimmed)
			case 1:
				return CellStyle.Foreground(ui.StatusColor(prReviewsRows[row][1]))
			}
			return CellStyle
		}).
		Headers("Reviewed By", "State", "Submitted", "Body").
		Rows(prReviewsRows...)

	reviewsTableView := lipgloss.NewStyle().BorderForeground(ui.Dimmed)
	return reviewsTableView.Render(reviewsTable.String())
}
