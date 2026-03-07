package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/pkg/branch"
	"github.com/prady-lab/sgh-cli/pkg/commit"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/issue"
	"github.com/prady-lab/sgh-cli/pkg/pr"
	"github.com/prady-lab/sgh-cli/pkg/protectedbranch"
	"github.com/prady-lab/sgh-cli/pkg/repo"
	"github.com/prady-lab/sgh-cli/pkg/tag"
	"github.com/prady-lab/sgh-cli/pkg/team"
	"github.com/prady-lab/sgh-cli/pkg/ui"
	"github.com/prady-lab/sgh-cli/pkg/workflow"
)

type panelFocus int

const (
	focusRepoSelector panelFocus = iota
	focusCommandMenu
	focusContent
	focusDetail
)

// -- messages --

type reposLoadedMsg struct{ repos []string }
type repoSelectionChangedMsg struct{ repos []string }
type dataLoadedMsg struct {
	command string
	columns []string
	rows    [][]string
	colors  [][]lipgloss.Color
	raw     any
}
type detailLoadedMsg struct {
	title         string
	fields        []detailField
	autoWatch     bool
	watchRunID    int
	watchRepoName string
}
type actionResultMsg struct {
	success bool
	message string
	command string
}
type watchTickMsg struct{}
type errMsg struct{ err error }

// -- root model --

type rootModel struct {
	repoSelector  repoSelectorModel
	sidebar       sidebarModel
	content       contentModel
	detail        detailModel
	statusBar     statusBarModel
	cache         *dataCache
	focus         panelFocus
	ctx           *context.Context
	orgName       string
	selectedRepos []string
	width         int
	height        int
	ready         bool
	confirming     bool
	confirmPrompt  string
	confirmAction  func() tea.Msg
	showHelp       bool
	toastMsg       string
	toastExpiry    time.Time
	watching       bool
	watchRunID     int
	watchRepoName  string
	watchInterval  time.Duration
	cmdFilters     map[string]*commandFilter
}

func initialModel(ctx *context.Context, orgName string) rootModel {
	sb := newStatusBar(orgName)
	sb.loading = true
	sb.command = "loading repos"
	filters := make(map[string]*commandFilter)
	for k, f := range defaultFilters {
		filters[k] = &commandFilter{options: f.options, current: f.current}
	}
	return rootModel{
		repoSelector: newRepoSelector(nil),
		sidebar:      newSidebar(),
		content:      newContent(),
		detail:       newDetail(),
		statusBar:    sb,
		cache:        newDataCache(5 * time.Minute),
		focus:        focusRepoSelector,
		ctx:          ctx,
		orgName:      orgName,
		cmdFilters:   filters,
	}
}

func (m rootModel) Init() tea.Cmd {
	return tea.Batch(
		m.statusBar.spinner.Tick,
		fetchRepos(m.ctx, m.orgName),
	)
}

func fetchRepos(ctx *context.Context, orgName string) tea.Cmd {
	return func() tea.Msg {
		repos, err := repo.GetSelectedRepoNames(ctx, orgName)
		if err != nil {
			return errMsg{err}
		}
		sort.Strings(repos)
		return reposLoadedMsg{repos}
	}
}

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.relayout()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.statusBar.spinner, cmd = m.statusBar.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case reposLoadedMsg:
		m.repoSelector = newRepoSelector(msg.repos)
		m.focus = focusRepoSelector
		m.clearAllFocus()
		m.applyFocus()
		m.selectedRepos = nil
		m.statusBar.totalRepo = len(msg.repos)
		m.statusBar.selectedRepo = 0
		m.statusBar.loading = false
		m.statusBar.command = ""
		m.relayout()

	case repoSelectionChangedMsg:
		m.selectedRepos = msg.repos
		m.statusBar.selectedRepo = len(msg.repos)
		if m.sidebar.activeCommand() != nil {
			m.content.loading = true
			m.statusBar.loading = true
			cmds = append(cmds, m.loadCommand(m.sidebar.activeCommand(), false))
		}

	case dataLoadedMsg:
		m.content.setData(msg.command, msg.columns, msg.rows, msg.colors, msg.raw)
		m.content.noRepos = len(m.selectedRepos) == 0 && msg.command != "team"
		m.statusBar.loading = false
		m.statusBar.command = msg.command
		m.statusBar.cacheAge = m.cache.age(msg.command)

	case detailLoadedMsg:
		m.detail.setData(msg.title, msg.fields)
		m.relayout()
		m.statusBar.loading = false
		if m.watching {
			if strings.HasSuffix(msg.title, "(completed)") {
				m.watching = false
				m.toastMsg = "Workflow run completed"
				m.toastExpiry = time.Now().Add(5 * time.Second)
			} else if strings.HasSuffix(msg.title, "(watching...)") {
				cmds = append(cmds, tea.Tick(m.watchInterval, func(time.Time) tea.Msg { return watchTickMsg{} }))
			}
		} else if msg.autoWatch {
			m.watching = true
			m.watchRunID = msg.watchRunID
			m.watchRepoName = msg.watchRepoName
			m.watchInterval = 10 * time.Second
			m.toastMsg = fmt.Sprintf("Auto-watching in-progress workflow %d", msg.watchRunID)
			m.toastExpiry = time.Now().Add(3 * time.Second)
			cmds = append(cmds, tea.Tick(m.watchInterval, func(time.Time) tea.Msg { return watchTickMsg{} }))
		}
		m.applyFocus()

	case actionResultMsg:
		m.statusBar.loading = false
		if msg.success {
			m.toastMsg = msg.message
			m.toastExpiry = time.Now().Add(5 * time.Second)
			m.cache.invalidate(msg.command)
			if cmd := m.sidebar.activeCommand(); cmd != nil && cmd.key == msg.command {
				m.content.loading = true
				m.statusBar.loading = true
				cmds = append(cmds, m.loadCommand(cmd, true))
			}
		} else {
			m.statusBar.lastErr = msg.message
			m.statusBar.errExpiry = time.Now().Add(10 * time.Second)
		}

	case watchTickMsg:
		if m.watching && m.watchRunID > 0 {
			ctx := m.ctx
			org := m.orgName
			repoName := m.watchRepoName
			runID := m.watchRunID
			cmds = append(cmds, func() tea.Msg {
				detail := workflow.GetWorkflowRunDetail(ctx, workflow.WorkflowRunRequest{
					OrgName: org, RepoName: repoName, RunID: runID,
				})
				fields := buildWorkflowDetailFields(detail)
				if !detail.IsInProgress() {
					return detailLoadedMsg{title: detail.Run.Name + " (completed)", fields: fields}
				}
				return detailLoadedMsg{title: detail.Run.Name + " (watching...)", fields: fields}
			})
		}

	case errMsg:
		m.statusBar.lastErr = msg.err.Error()
		m.statusBar.errExpiry = time.Now().Add(10 * time.Second)
		m.statusBar.loading = false

	case tea.KeyMsg:
		if m.confirming {
			switch msg.String() {
			case "y", "Y":
				m.confirming = false
				m.statusBar.loading = true
				action := m.confirmAction
				m.confirmAction = nil
				cmds = append(cmds, func() tea.Msg { return action() })
			default:
				m.confirming = false
				m.confirmAction = nil
				m.confirmPrompt = ""
			}
			return m, tea.Batch(cmds...)
		}

		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		if m.repoSelector.filtering {
			m.repoSelector.handleFilterKey(key.Binding{}, msg.String())
			return m, nil
		}
		if m.content.filtering {
			m.content.handleFilterKey(msg.String())
			return m, nil
		}

		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, keys.Help):
			m.showHelp = !m.showHelp

		case key.Matches(msg, keys.JumpRepo):
			m.clearAllFocus()
			m.focus = focusRepoSelector
			m.applyFocus()

		case key.Matches(msg, keys.JumpCmd):
			m.clearAllFocus()
			m.focus = focusCommandMenu
			m.applyFocus()

		case key.Matches(msg, keys.JumpCont):
			if m.content.command != "" {
				m.clearAllFocus()
				m.focus = focusContent
				m.applyFocus()
			}

		case key.Matches(msg, keys.Tab):
			m.advanceFocus()

		case key.Matches(msg, keys.ShiftTab):
			m.retreatFocus()

		default:
			cmd := m.handlePanelKey(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *rootModel) advanceFocus() {
	m.clearAllFocus()
	switch m.focus {
	case focusRepoSelector:
		m.focus = focusCommandMenu
	case focusCommandMenu:
		if m.content.command != "" {
			m.focus = focusContent
		} else {
			m.focus = focusRepoSelector
		}
	case focusContent:
		if m.detail.visible {
			m.focus = focusDetail
		} else {
			m.focus = focusRepoSelector
		}
	case focusDetail:
		m.focus = focusRepoSelector
	}
	m.applyFocus()
}

func (m *rootModel) retreatFocus() {
	m.clearAllFocus()
	switch m.focus {
	case focusRepoSelector:
		if m.detail.visible {
			m.focus = focusDetail
		} else if m.content.command != "" {
			m.focus = focusContent
		} else {
			m.focus = focusCommandMenu
		}
	case focusCommandMenu:
		m.focus = focusRepoSelector
	case focusContent:
		m.focus = focusCommandMenu
	case focusDetail:
		m.focus = focusContent
	}
	m.applyFocus()
}

func (m *rootModel) clearAllFocus() {
	m.repoSelector.focused = false
	m.sidebar.focused = false
	m.content.focused = false
	m.detail.focused = false
}

func (m *rootModel) applyFocus() {
	panelNav := "1:repos 2:cmds 3:content"
	switch m.focus {
	case focusRepoSelector:
		m.repoSelector.focused = true
		m.statusBar.focusHint = "space:toggle a:all n:none /:filter g/G:top/bottom " + panelNav
	case focusCommandMenu:
		m.sidebar.focused = true
		m.statusBar.focusHint = "enter:load esc:back " + panelNav
	case focusContent:
		m.content.focused = true
		hint := "enter:detail o:open r:refresh /:filter"
		if cmd := m.sidebar.activeCommand(); cmd != nil {
			if _, ok := m.cmdFilters[cmd.key]; ok {
				hint = "s:status " + hint
			}
			switch cmd.key {
			case "pr":
				hint = "A:approve m:merge M:both c:close " + hint
			case "wf":
				hint = "R:rerun X:cancel " + hint
			}
		}
		m.statusBar.focusHint = hint + " " + panelNav
	case focusDetail:
		m.detail.focused = true
		hint := "j/k:scroll o:open r:refresh esc:close"
		if cmd := m.sidebar.activeCommand(); cmd != nil && cmd.key == "wf" {
			hint = "w:watch " + hint
			if m.watching {
				hint = "(watching) " + hint
			}
		}
		m.statusBar.focusHint = hint + " " + panelNav
	}
}

func (m *rootModel) handlePanelKey(msg tea.KeyMsg) tea.Cmd {
	switch m.focus {
	case focusRepoSelector:
		return m.handleRepoSelectorKey(msg)
	case focusCommandMenu:
		return m.handleCommandMenuKey(msg)
	case focusContent:
		return m.handleContentKey(msg)
	case focusDetail:
		return m.handleDetailKey(msg)
	}
	return nil
}

func (m *rootModel) handleRepoSelectorKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, keys.Up):
		m.repoSelector.moveUp()
	case key.Matches(msg, keys.Down):
		m.repoSelector.moveDown()
	case key.Matches(msg, keys.GoTop):
		m.repoSelector.goTop()
	case key.Matches(msg, keys.GoBottom):
		m.repoSelector.goBottom()
	case key.Matches(msg, keys.Space):
		if m.repoSelector.toggle() {
			m.selectedRepos = m.repoSelector.selectedNames()
			return func() tea.Msg {
				return repoSelectionChangedMsg{m.selectedRepos}
			}
		}
	case key.Matches(msg, keys.SelectAll):
		m.repoSelector.selectAll()
		m.selectedRepos = m.repoSelector.selectedNames()
		return func() tea.Msg {
			return repoSelectionChangedMsg{m.selectedRepos}
		}
	case key.Matches(msg, keys.SelectNon):
		m.repoSelector.selectNone()
		m.selectedRepos = m.repoSelector.selectedNames()
		return func() tea.Msg {
			return repoSelectionChangedMsg{m.selectedRepos}
		}
	case key.Matches(msg, keys.Esc):
		if m.repoSelector.filter != "" {
			m.repoSelector.filter = ""
			m.repoSelector.cursor = 0
			m.repoSelector.offset = 0
		}
	case key.Matches(msg, keys.Filter):
		m.repoSelector.filtering = true
		m.repoSelector.filter = ""
	case key.Matches(msg, keys.Enter):
		m.clearAllFocus()
		m.focus = focusCommandMenu
		m.applyFocus()
	}
	return nil
}

func (m *rootModel) handleCommandMenuKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, keys.Up):
		m.sidebar.moveUp()
	case key.Matches(msg, keys.Down):
		m.sidebar.moveDown()
	case key.Matches(msg, keys.Enter):
		idx := m.sidebar.select_()
		if idx >= 0 && idx < len(commands) {
			m.content.loading = true
			m.statusBar.loading = true
			m.detail.clear()
			m.relayout()
			m.clearAllFocus()
			m.focus = focusContent
			m.applyFocus()
			return m.loadCommand(&commands[idx], false)
		}
	case key.Matches(msg, keys.Esc):
		m.clearAllFocus()
		m.focus = focusRepoSelector
		m.applyFocus()
	}
	return nil
}

func (m *rootModel) handleContentKey(msg tea.KeyMsg) tea.Cmd {
	cmd := m.sidebar.activeCommand()

	switch {
	case key.Matches(msg, keys.Up):
		m.content.moveUp()
	case key.Matches(msg, keys.Down):
		m.content.moveDown()
	case key.Matches(msg, keys.GoTop):
		m.content.goTop()
	case key.Matches(msg, keys.GoBottom):
		m.content.goBottom()
	case key.Matches(msg, keys.Enter):
		return m.loadDetail()
	case key.Matches(msg, keys.Filter):
		m.content.filtering = true
		m.content.filter = ""
	case key.Matches(msg, keys.Refresh):
		if cmd != nil {
			m.content.loading = true
			m.statusBar.loading = true
			return m.loadCommand(cmd, true)
		}
	case key.Matches(msg, keys.CycleFilter):
		if cmd != nil {
			if f, ok := m.cmdFilters[cmd.key]; ok {
				f.cycle()
				m.cache.invalidate(cmd.key)
				m.content.loading = true
				m.statusBar.loading = true
				return m.loadCommand(cmd, true)
			}
		}
	case key.Matches(msg, keys.Open):
		if url := m.getSelectedURL(); url != "" {
			_ = openURL(url)
			m.toastMsg = "Opened in browser"
			m.toastExpiry = time.Now().Add(3 * time.Second)
		}
	case key.Matches(msg, keys.Esc):
		if m.detail.visible {
			m.detail.clear()
			m.relayout()
		} else {
			m.clearAllFocus()
			m.focus = focusCommandMenu
			m.applyFocus()
		}

	case key.Matches(msg, prKeys.Approve):
		if cmd != nil && cmd.key == "pr" {
			return m.prAction("APPROVE", "Approve this PR?")
		}
	case key.Matches(msg, prKeys.Merge):
		if cmd != nil && cmd.key == "pr" {
			return m.prAction("merge", "Merge this PR?")
		}
	case key.Matches(msg, prKeys.ApproveMerge):
		if cmd != nil && cmd.key == "pr" {
			return m.prAction("approve+merge", "Approve and merge this PR?")
		}
	case key.Matches(msg, prKeys.Close):
		if cmd != nil && cmd.key == "pr" {
			return m.prAction("close", "Close this PR?")
		}
	case key.Matches(msg, wfKeys.Rerun):
		if cmd != nil && cmd.key == "wf" {
			return m.wfAction("rerun", "Rerun this workflow?")
		}
	case key.Matches(msg, wfKeys.Cancel):
		if cmd != nil && cmd.key == "wf" {
			return m.wfAction("cancel", "Cancel this workflow?")
		}
	}
	return nil
}

func (m *rootModel) prAction(action, prompt string) tea.Cmd {
	rowIdx := m.content.selectedRowIndex()
	results, ok := m.content.rawData.([]model.PullRequestResponse)
	if !ok || rowIdx < 0 || rowIdx >= len(results) {
		return nil
	}
	p := results[rowIdx]
	ctx := m.ctx
	org := m.orgName

	m.confirming = true
	m.confirmPrompt = fmt.Sprintf("%s (y/n)", prompt)
	m.confirmAction = func() tea.Msg {
		repoName := p.RepositoryName()
		switch action {
		case "APPROVE":
			resp := pr.ReviewPullRequest(ctx, pr.PRReviewRequest{
				OrgName: org, RepoName: repoName, PRNumber: p.PRNumber, Event: "APPROVE",
			})
			if resp.ErrorMessage != "" {
				return actionResultMsg{false, resp.ErrorMessage, "pr"}
			}
			return actionResultMsg{true, fmt.Sprintf("PR #%d approved", p.PRNumber), "pr"}

		case "merge":
			resp := pr.MergePullRequest(ctx, pr.PRMergeRequest{
				OrgName: org, RepoName: repoName, PRNumber: p.PRNumber,
			})
			if resp.ErrorMessage != "" {
				return actionResultMsg{false, resp.ErrorMessage, "pr"}
			}
			return actionResultMsg{true, fmt.Sprintf("PR #%d merged", p.PRNumber), "pr"}

		case "approve+merge":
			resp := pr.ReviewPullRequest(ctx, pr.PRReviewRequest{
				OrgName: org, RepoName: repoName, PRNumber: p.PRNumber, Event: "APPROVE",
			})
			if resp.ErrorMessage != "" {
				return actionResultMsg{false, resp.ErrorMessage, "pr"}
			}
			mResp := pr.MergePullRequest(ctx, pr.PRMergeRequest{
				OrgName: org, RepoName: repoName, PRNumber: p.PRNumber,
			})
			if mResp.ErrorMessage != "" {
				return actionResultMsg{false, mResp.ErrorMessage, "pr"}
			}
			return actionResultMsg{true, fmt.Sprintf("PR #%d approved and merged", p.PRNumber), "pr"}

		case "close":
			resp := pr.UpdatePullRequest(ctx, pr.PRUpdateRequest{
				OrgName: org, RepoName: repoName, PRNumber: p.PRNumber, State: "closed",
			})
			if resp.ErrorMessage != "" {
				return actionResultMsg{false, resp.ErrorMessage, "pr"}
			}
			return actionResultMsg{true, fmt.Sprintf("PR #%d closed", p.PRNumber), "pr"}
		}
		return nil
	}
	return nil
}

func (m *rootModel) wfAction(action, prompt string) tea.Cmd {
	rowIdx := m.content.selectedRowIndex()
	results, ok := m.content.rawData.([]model.WorkflowRun)
	if !ok || rowIdx < 0 || rowIdx >= len(results) {
		return nil
	}
	w := results[rowIdx]
	ctx := m.ctx
	org := m.orgName

	m.confirming = true
	m.confirmPrompt = fmt.Sprintf("%s (y/n)", prompt)
	m.confirmAction = func() tea.Msg {
		req := workflow.WorkflowRunRequest{OrgName: org, RepoName: w.RepositoryName, RunID: w.ID}
		switch action {
		case "rerun":
			resp := workflow.RerunWorkflowRun(ctx, req)
			if resp.ErrorMessage != "" {
				return actionResultMsg{false, resp.ErrorMessage, "wf"}
			}
			return actionResultMsg{true, fmt.Sprintf("Workflow %d rerun started", w.ID), "wf"}
		case "cancel":
			resp := workflow.CancelWorkflowRun(ctx, req)
			if resp.ErrorMessage != "" {
				return actionResultMsg{false, resp.ErrorMessage, "wf"}
			}
			return actionResultMsg{true, fmt.Sprintf("Workflow %d cancelled", w.ID), "wf"}
		}
		return nil
	}
	return nil
}

func (m *rootModel) handleDetailKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, keys.Up):
		m.detail.scrollUp()
	case key.Matches(msg, keys.Down):
		m.detail.scrollDown()
	case key.Matches(msg, keys.GoTop):
		m.detail.goTop()
	case key.Matches(msg, keys.GoBottom):
		m.detail.goBottom()
	case key.Matches(msg, keys.Esc):
		m.watching = false
		m.detail.clear()
		m.relayout()
		m.clearAllFocus()
		m.focus = focusContent
		m.applyFocus()
	case key.Matches(msg, wfKeys.Watch):
		if cmd := m.sidebar.activeCommand(); cmd != nil && cmd.key == "wf" {
			if m.watching {
				m.watching = false
				m.toastMsg = "Watch mode stopped"
				m.toastExpiry = time.Now().Add(3 * time.Second)
				m.applyFocus()
				return nil
			}
			return m.startWatch()
		}
	case key.Matches(msg, keys.Refresh):
		if cmd := m.sidebar.activeCommand(); cmd != nil {
			return m.loadDetail()
		}
	case key.Matches(msg, keys.Open):
		if url := m.getSelectedURL(); url != "" {
			_ = openURL(url)
			m.toastMsg = "Opened in browser"
			m.toastExpiry = time.Now().Add(3 * time.Second)
		}
	}
	return nil
}

func (m *rootModel) startWatch() tea.Cmd {
	rowIdx := m.content.selectedRowIndex()
	results, ok := m.content.rawData.([]model.WorkflowRun)
	if !ok || rowIdx < 0 || rowIdx >= len(results) {
		return nil
	}
	w := results[rowIdx]
	m.watching = true
	m.watchRunID = w.ID
	m.watchRepoName = w.RepositoryName
	m.watchInterval = 10 * time.Second
	m.toastMsg = fmt.Sprintf("Watching workflow %d (every 10s)", w.ID)
	m.toastExpiry = time.Now().Add(3 * time.Second)
	return tea.Tick(m.watchInterval, func(time.Time) tea.Msg { return watchTickMsg{} })
}

func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func (m *rootModel) getSelectedURL() string {
	cmd := m.sidebar.activeCommand()
	if cmd == nil {
		return ""
	}
	rowIdx := m.content.selectedRowIndex()
	if rowIdx < 0 {
		return ""
	}
	org := m.orgName

	switch cmd.key {
	case "issue":
		if results, ok := m.content.rawData.([]model.IssueResponse); ok && rowIdx < len(results) {
			return results[rowIdx].HTMLUrl
		}
	case "pr":
		if results, ok := m.content.rawData.([]model.PullRequestResponse); ok && rowIdx < len(results) {
			return results[rowIdx].HTMLUrl
		}
	case "wf":
		if results, ok := m.content.rawData.([]model.WorkflowRun); ok && rowIdx < len(results) {
			return results[rowIdx].HTMLUrl
		}
	case "commit":
		if results, ok := m.content.rawData.([]model.CommitResponse); ok && rowIdx < len(results) {
			return results[rowIdx].HtmlUrl
		}
	case "team":
		if results, ok := m.content.rawData.([]model.OrgTeam); ok && rowIdx < len(results) {
			return results[rowIdx].Url
		}
	case "branch":
		if results, ok := m.content.rawData.([]model.BranchResponse); ok && rowIdx < len(results) {
			b := results[rowIdx]
			return fmt.Sprintf("https://github.com/%s/%s/tree/%s", org, b.RepositoryName, b.Name)
		}
	case "tag":
		if results, ok := m.content.rawData.([]model.TagResponse); ok && rowIdx < len(results) {
			t := results[rowIdx]
			return fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", org, t.RepositoryName, t.Name)
		}
	case "pb":
		if results, ok := m.content.rawData.([]model.ProtectedBranch); ok && rowIdx < len(results) {
			pb := results[rowIdx]
			return fmt.Sprintf("https://github.com/%s/%s/settings/branches", org, pb.RepositoryName)
		}
	}
	return ""
}

func (m *rootModel) relayout() {
	// renderBorderedPanel(title, body, w, h): outer width = w+4, outer height = h+3
	// where h is body height (excludes title line), +1 title +2 border = +3 total
	widthOH := 4 // 2 border chars + 2 padding chars
	panelOH := 3 // 1 title line + 2 border lines (top+bottom)

	availH := m.height - 1 // 1 line for status bar

	// Sidebar column
	sbOuterW := sidebarWidth + widthOH
	cmdBodyH := len(commands)
	cmdOuterH := cmdBodyH + panelOH
	repoOuterH := availH - cmdOuterH
	repoBodyH := repoOuterH - panelOH
	if repoBodyH < 3 {
		repoBodyH = 3
	}
	m.repoSelector.height = repoBodyH

	// Main panels
	mainW := m.width - sbOuterW
	mainBodyH := availH - panelOH

	if m.detail.visible {
		detailOuterW := mainW * 2 / 5
		contentOuterW := mainW - detailOuterW
		m.detail.width = detailOuterW - widthOH
		m.content.width = contentOuterW - widthOH
	} else {
		m.content.width = mainW - widthOH
	}
	if m.content.width < 10 {
		m.content.width = 10
	}
	if m.detail.width < 10 {
		m.detail.width = 10
	}
	m.content.height = mainBodyH
	m.detail.height = mainBodyH
	m.statusBar.width = m.width
}

// -- data loading --

func (m *rootModel) loadCommand(cmd *commandDef, forceRefresh bool) tea.Cmd {
	repos := m.selectedRepos
	ctx := m.ctx
	org := m.orgName

	if len(repos) == 0 && cmd.key != "team" {
		return func() tea.Msg {
			return dataLoadedMsg{command: cmd.key, columns: cmd.columns, rows: nil, raw: nil}
		}
	}

	if !forceRefresh {
		if cached, hit, stale := m.cache.get(cmd.key, repos); hit && !stale {
			if dm, ok := cached.(*dataLoadedMsg); ok {
				return func() tea.Msg { return *dm }
			}
		}
	}

	switch cmd.key {
	case "issue":
		return m.loadIssues(ctx, org, repos, cmd.columns)
	case "pr":
		return m.loadPRs(ctx, org, repos, cmd.columns)
	case "branch":
		return m.loadBranches(ctx, org, repos, cmd.columns)
	case "tag":
		return m.loadTags(ctx, org, repos, cmd.columns)
	case "wf":
		return m.loadWorkflows(ctx, org, repos, cmd.columns)
	case "commit":
		return m.loadCommits(ctx, org, repos, cmd.columns)
	case "team":
		return m.loadTeams(ctx, org, cmd.columns)
	case "pb":
		return m.loadProtectedBranches(ctx, org, repos, cmd.columns)
	}
	return nil
}

func (m *rootModel) loadPRs(ctx *context.Context, org string, repos []string, cols []string) tea.Cmd {
	stateFilter := "open"
	if f, ok := m.cmdFilters["pr"]; ok {
		stateFilter = f.value()
	}
	return func() tea.Msg {
		req := pr.PRRequest{OrgName: org, RepoNames: repos, LastCount: 30, All: stateFilter != "open"}
		results := pr.ListPullRequests(ctx, req)
		valid := make([]model.PullRequestResponse, 0, len(results))
		rows := make([][]string, 0, len(results))
		colors := make([][]lipgloss.Color, 0, len(results))
		for _, p := range results {
			if p.ErrorMessage != "" {
				continue
			}
			if stateFilter != "all" && !strings.EqualFold(p.State, stateFilter) {
				continue
			}
			valid = append(valid, p)
			rows = append(rows, []string{
				p.RepositoryName(),
				fmt.Sprintf("#%d", p.PRNumber),
				p.TitleName,
				p.AuthorName(),
				p.State,
				ui.RelativeTime(p.UpdatedAt),
			})
			colors = append(colors, []lipgloss.Color{"", "", "", "", statusColor(p.State), ""})
		}
		msg := &dataLoadedMsg{command: "pr", columns: cols, rows: rows, colors: colors, raw: valid}
		m.cache.set("pr", repos, msg)
		return *msg
	}
}

func (m *rootModel) loadIssues(ctx *context.Context, org string, repos []string, cols []string) tea.Cmd {
	stateFilter := "open"
	if f, ok := m.cmdFilters["issue"]; ok {
		stateFilter = f.value()
	}
	return func() tea.Msg {
		req := issue.IssueListRequest{OrgName: org, RepoNames: repos, State: stateFilter, LastCount: 30}
		results := issue.ListIssues(ctx, req)
		valid := make([]model.IssueResponse, 0, len(results))
		rows := make([][]string, 0, len(results))
		colors := make([][]lipgloss.Color, 0, len(results))
		for _, is := range results {
			if is.ErrorMessage != "" {
				continue
			}
			valid = append(valid, is)
			labels := is.LabelNames()
			if len(labels) > 20 {
				labels = labels[:17] + "..."
			}
			rows = append(rows, []string{
				is.RepositoryName,
				fmt.Sprintf("#%d", is.Number),
				is.Title,
				is.AuthorName(),
				is.State,
				labels,
				fmt.Sprintf("%d", is.Comments),
				ui.RelativeTime(is.UpdatedAt),
			})
			colors = append(colors, []lipgloss.Color{"", "", "", "", statusColor(is.State), "", "", ""})
		}
		msg := &dataLoadedMsg{command: "issue", columns: cols, rows: rows, colors: colors, raw: valid}
		m.cache.set("issue", repos, msg)
		return *msg
	}
}

func (m *rootModel) loadBranches(ctx *context.Context, org string, repos []string, cols []string) tea.Cmd {
	return func() tea.Msg {
		req := branch.BranchListRequest{OrgName: org, RepoNames: repos}
		results := branch.ListBranches(ctx, req)
		rows := make([][]string, 0, len(results))
		for _, b := range results {
			protected := "no"
			if b.Protected {
				protected = "yes"
			}
			rows = append(rows, []string{
				b.RepositoryName,
				b.Name,
				ui.ShortSHA(b.Commit.SHA),
				protected,
			})
		}
		msg := &dataLoadedMsg{command: "branch", columns: cols, rows: rows, raw: results}
		m.cache.set("branch", repos, msg)
		return *msg
	}
}

func (m *rootModel) loadTags(ctx *context.Context, org string, repos []string, cols []string) tea.Cmd {
	return func() tea.Msg {
		req := tag.TagListRequest{OrgName: org, RepoNames: repos}
		results := tag.ListTags(ctx, req)
		rows := make([][]string, 0, len(results))
		for _, t := range results {
			rows = append(rows, []string{
				t.RepositoryName,
				t.Name,
				ui.ShortSHA(t.Commit.SHA),
			})
		}
		msg := &dataLoadedMsg{command: "tag", columns: cols, rows: rows, raw: results}
		m.cache.set("tag", repos, msg)
		return *msg
	}
}

func (m *rootModel) loadWorkflows(ctx *context.Context, org string, repos []string, cols []string) tea.Cmd {
	statusFilter := "all"
	if f, ok := m.cmdFilters["wf"]; ok {
		statusFilter = f.value()
	}
	return func() tea.Msg {
		req := workflow.WorkflowListRequest{OrgName: org, RepoNames: repos, Count: 30}
		if statusFilter != "all" {
			req.Status = statusFilter
		}
		results := workflow.ListWorkflowRuns(ctx, req)
		valid := make([]model.WorkflowRun, 0, len(results))
		rows := make([][]string, 0, len(results))
		colors := make([][]lipgloss.Color, 0, len(results))
		for _, w := range results {
			if w.ErrorMessage != "" {
				continue
			}
			valid = append(valid, w)
			conclusion := w.Conclusion
			if conclusion == "" {
				conclusion = w.Status
			}
			rows = append(rows, []string{
				w.RepositoryName,
				w.Name,
				w.Status,
				conclusion,
				w.HeadBranch,
				ui.RelativeTime(w.UpdatedAt),
			})
			colors = append(colors, []lipgloss.Color{"", "", statusColor(w.Status), statusColor(conclusion), "", ""})
		}
		msg := &dataLoadedMsg{command: "wf", columns: cols, rows: rows, colors: colors, raw: valid}
		m.cache.set("wf", repos, msg)
		return *msg
	}
}

func (m *rootModel) loadCommits(ctx *context.Context, org string, repos []string, cols []string) tea.Cmd {
	return func() tea.Msg {
		req := commit.CommitListRequest{OrgName: org, RepoNames: repos, NoOfDays: 14}
		results := commit.ListCommits(ctx, req)
		rows := make([][]string, 0)
		for _, c := range results {
			if c.ErrorMessage != "" {
				continue
			}
			msg := c.Commit.Message
			if idx := strings.Index(msg, "\n"); idx > 0 {
				msg = msg[:idx]
			}
			authorName := c.Commit.Author.Name
			if authorName == "" {
				authorName = c.Author.Login
			}
			rows = append(rows, []string{
				c.RepositoryName,
				msg,
				authorName,
				ui.RelativeTime(c.Commit.Author.Date),
			})
		}
		msg := &dataLoadedMsg{command: "commit", columns: cols, rows: rows, raw: results}
		m.cache.set("commit", repos, msg)
		return *msg
	}
}

func (m *rootModel) loadTeams(ctx *context.Context, org string, cols []string) tea.Cmd {
	return func() tea.Msg {
		req := team.TeamMembersRequest{OrgName: org, NoOfMembers: 50}
		results, err := team.GetTeamAndMembers(ctx, req)
		if err != nil {
			return errMsg{err}
		}
		rows := make([][]string, 0, len(results))
		for _, t := range results {
			rows = append(rows, []string{
				t.Name,
				fmt.Sprintf("%d", t.TotalMembers),
				fmt.Sprintf("%d repos", t.RepositoriesCount),
			})
		}
		msg := &dataLoadedMsg{command: "team", columns: cols, rows: rows, raw: results}
		m.cache.set("team", nil, msg)
		return *msg
	}
}

func (m *rootModel) loadProtectedBranches(ctx *context.Context, org string, repos []string, cols []string) tea.Cmd {
	return func() tea.Msg {
		results := protectedbranch.ListProtectedBranches(ctx, org, repos, nil, "")
		rows := make([][]string, 0)
		for _, pb := range results {
			if pb.ErrorMessage != "" {
				continue
			}
			approvals := fmt.Sprintf("%d", pb.RequiredPullRequestReviews.RequiredApprovingReviewCount)
			enforce := "no"
			if pb.EnforceAdmins {
				enforce = "yes"
			}
			rows = append(rows, []string{
				pb.RepositoryName,
				pb.Name,
				approvals,
				enforce,
			})
		}
		msg := &dataLoadedMsg{command: "pb", columns: cols, rows: rows, raw: results}
		m.cache.set("pb", repos, msg)
		return *msg
	}
}

func (m *rootModel) loadDetail() tea.Cmd {
	cmd := m.sidebar.activeCommand()
	if cmd == nil {
		return nil
	}
	rowIdx := m.content.selectedRowIndex()
	if rowIdx < 0 {
		return nil
	}

	m.statusBar.loading = true

	switch cmd.key {
	case "issue":
		return m.loadIssueDetail(rowIdx)
	case "pr":
		return m.loadPRDetail(rowIdx)
	case "wf":
		return m.loadWorkflowDetail(rowIdx)
	default:
		return m.loadGenericDetail(cmd, rowIdx)
	}
}

func (m *rootModel) loadIssueDetail(rowIdx int) tea.Cmd {
	results, ok := m.content.rawData.([]model.IssueResponse)
	if !ok || rowIdx < 0 || rowIdx >= len(results) {
		return nil
	}
	is := results[rowIdx]
	ctx := m.ctx
	org := m.orgName

	return func() tea.Msg {
		fields := []detailField{
			{"Repository", is.RepositoryName, ""},
			{"Issue", fmt.Sprintf("#%d", is.Number), ""},
			{"Title", is.Title, ""},
			{"Author", is.AuthorName(), ""},
			{"State", is.State, statusColor(is.State)},
			{"Labels", is.LabelNames(), ""},
			{"Comments", fmt.Sprintf("%d", is.Comments), ""},
			{"Created", ui.RelativeTime(is.CreatedAt), ""},
			{"Updated", ui.RelativeTime(is.UpdatedAt), ""},
		}

		if is.ClosedAt != "" {
			fields = append(fields, detailField{"Closed", ui.RelativeTime(is.ClosedAt), ""})
			if is.ClosedBy.Login != "" {
				fields = append(fields, detailField{"Closed By", is.ClosedBy.Login, ""})
			}
		}

		if len(is.Assignees) > 0 {
			names := make([]string, 0, len(is.Assignees))
			for _, a := range is.Assignees {
				if a.Login != "" {
					names = append(names, a.Login)
				}
			}
			if len(names) > 0 {
				fields = append(fields, detailField{"Assignees", strings.Join(names, ", "), ""})
			}
		}

		if is.Milestone != nil {
			fields = append(fields, detailField{"Milestone", is.Milestone.Title, ""})
		}

		if is.Body != "" {
			body := is.Body
			if len(body) > 300 {
				body = body[:297] + "..."
			}
			body = strings.ReplaceAll(body, "\r\n", "\n")
			fields = append(fields, detailField{"", "", ""})
			fields = append(fields, detailField{"Body", body, ""})
		}

		comments := issue.GetIssueComments(ctx, org, is.RepositoryName, is.Number)
		if len(comments) > 0 {
			dimStyle := lipgloss.NewStyle().Foreground(ui.Dimmed)
			fields = append(fields, detailField{"", "", ""})
			fields = append(fields, detailField{"Comments", fmt.Sprintf("%d comments", len(comments)), ""})
			for i, c := range comments {
				if i >= 5 {
					fields = append(fields, detailField{"", dimStyle.Render(fmt.Sprintf("... and %d more", len(comments)-5)), ""})
					break
				}
				author := c.Author.Login
				if author == "" {
					author = c.Author.Name
				}
				body := c.Body
				if len(body) > 120 {
					body = body[:117] + "..."
				}
				body = strings.ReplaceAll(body, "\n", " ")
				fields = append(fields, detailField{author, dimStyle.Render(body), ""})
			}
		}

		return detailLoadedMsg{title: is.Title, fields: fields}
	}
}

func (m *rootModel) loadPRDetail(rowIdx int) tea.Cmd {
	results, ok := m.content.rawData.([]model.PullRequestResponse)
	if !ok || rowIdx < 0 || rowIdx >= len(results) {
		return nil
	}
	p := results[rowIdx]
	ctx := m.ctx
	org := m.orgName

	return func() tea.Msg {
		prResp, filesResp, checkResp, reviews := pr.GetPRDetailsGraphQL(ctx, pr.PRDetailsRequest{
			OrgName:  org,
			RepoName: p.RepositoryName(),
			PRNumber: p.PRNumber,
		})

		fields := []detailField{
			{"Repository", prResp.RepositoryName(), ""},
			{"PR Number", fmt.Sprintf("#%d", prResp.PRNumber), ""},
			{"Title", prResp.TitleName, ""},
			{"Author", prResp.AuthorName(), ""},
			{"State", prResp.State, statusColor(prResp.State)},
			{"Branch", prResp.Base.Ref + " ← " + prResp.Head.Ref, ""},
			{"Mergeable", prResp.Mergeable, ""},
			{"Review", prResp.ReviewDecision, ""},
			{"Created", ui.RelativeTime(prResp.CreatedAt), ""},
			{"Updated", ui.RelativeTime(prResp.UpdatedAt), ""},
			{"Commits", fmt.Sprintf("%d", prResp.Commits), ""},
			{"Changes", fmt.Sprintf("+%d -%d (%d files)", prResp.Additions, prResp.Deletions, prResp.ChangedFiles), ""},
		}

		addStyle := lipgloss.NewStyle().Foreground(ui.Green)
		delStyle := lipgloss.NewStyle().Foreground(ui.Red)
		dimStyle := lipgloss.NewStyle().Foreground(ui.Dimmed)

		changesVal := addStyle.Render(fmt.Sprintf("+%d", prResp.Additions)) + "  " +
			delStyle.Render(fmt.Sprintf("-%d", prResp.Deletions)) + "  " +
			fmt.Sprintf("%d files", prResp.ChangedFiles)
		fields[len(fields)-1] = detailField{"Changes", changesVal, ""}

		if len(filesResp.Files) > 0 {
			fields = append(fields, detailField{"", "", ""})
			fields = append(fields, detailField{"Files Changed", fmt.Sprintf("%d files", len(filesResp.Files)), ""})
			for i, f := range filesResp.Files {
				if i >= 10 {
					remaining := fmt.Sprintf("... and %d more", len(filesResp.Files)-10)
					fields = append(fields, detailField{"", dimStyle.Render(remaining), ""})
					break
				}
				changeIcon := "M"
				switch strings.ToUpper(f.ChangeType) {
				case "ADDED":
					changeIcon = "A"
				case "DELETED", "REMOVED":
					changeIcon = "D"
				case "RENAMED":
					changeIcon = "R"
				case "COPIED":
					changeIcon = "C"
				}
				fileLine := fmt.Sprintf("%s %s  %s %s",
					dimStyle.Render(changeIcon),
					f.Filename,
					addStyle.Render(fmt.Sprintf("+%d", f.Additions)),
					delStyle.Render(fmt.Sprintf("-%d", f.Deletions)),
				)
				fields = append(fields, detailField{"", fileLine, ""})
			}
		}

		if len(checkResp.CheckRuns) > 0 {
			fields = append(fields, detailField{"Checks", fmt.Sprintf("%d (%s)", len(checkResp.CheckRuns), checkResp.OverallConclusion), statusColor(checkResp.OverallConclusion)})
		}

		for _, r := range reviews {
			reviewer := r.User.Login
			if r.User.Name != "" {
				reviewer = r.User.Name
			}
			fields = append(fields, detailField{"Review: " + reviewer, r.State, statusColor(r.State)})
		}

		return detailLoadedMsg{title: prResp.TitleName, fields: fields}
	}
}

func (m *rootModel) loadWorkflowDetail(rowIdx int) tea.Cmd {
	results, ok := m.content.rawData.([]model.WorkflowRun)
	if !ok || rowIdx < 0 || rowIdx >= len(results) {
		return nil
	}
	w := results[rowIdx]
	ctx := m.ctx
	org := m.orgName

	return func() tea.Msg {
		detail := workflow.GetWorkflowRunDetail(ctx, workflow.WorkflowRunRequest{
			OrgName:  org,
			RepoName: w.RepositoryName,
			RunID:    w.ID,
		})
		fields := buildWorkflowDetailFields(detail)
		msg := detailLoadedMsg{title: detail.Run.Name, fields: fields}
		if detail.IsInProgress() {
			msg.autoWatch = true
			msg.watchRunID = w.ID
			msg.watchRepoName = w.RepositoryName
		}
		return msg
	}
}

func buildWorkflowDetailFields(detail model.WorkflowRunDetail) []detailField {
	fields := []detailField{
		{"Repository", detail.Run.RepositoryName, ""},
		{"Workflow", detail.Run.Name, ""},
		{"Run ID", fmt.Sprintf("%d", detail.Run.ID), ""},
		{"Status", detail.Run.Status, statusColor(detail.Run.Status)},
		{"Conclusion", detail.Run.Conclusion, statusColor(detail.Run.Conclusion)},
		{"Branch", detail.Run.HeadBranch, ""},
		{"Event", detail.Run.Event, ""},
		{"Created", ui.RelativeTime(detail.Run.CreatedAt), ""},
		{"Updated", ui.RelativeTime(detail.Run.UpdatedAt), ""},
	}

	for _, j := range detail.Jobs {
		status := j.Conclusion
		if status == "" {
			status = j.Status
		}
		fields = append(fields, detailField{"Job: " + j.Name, status, statusColor(status)})
		for _, s := range j.Steps {
			stepStatus := s.Conclusion
			if stepStatus == "" {
				stepStatus = s.Status
			}
			fields = append(fields, detailField{"  " + s.Name, stepStatus, statusColor(stepStatus)})
		}
	}
	return fields
}

func (m *rootModel) loadGenericDetail(cmd *commandDef, rowIdx int) tea.Cmd {
	row := m.content.rows[rowIdx]
	fields := make([]detailField, 0, len(cmd.columns))
	for i, col := range cmd.columns {
		val := ""
		if i < len(row) {
			val = row[i]
		}
		fields = append(fields, detailField{col, val, ""})
	}
	return func() tea.Msg {
		title := ""
		if len(row) > 0 {
			title = row[0]
		}
		return detailLoadedMsg{title: title, fields: fields}
	}
}

// -- view --

func (m rootModel) View() string {
	if !m.ready {
		return "\n  Loading sgh TUI..."
	}

	if m.showHelp {
		return m.fullHelpView()
	}

	if m.ctx.HttpClient != nil {
		m.statusBar.apiCalls = int(m.ctx.HttpClient.APICallCount())
	}

	sidebar := m.renderSidebar()
	mainContent := m.renderMain()

	var statusLine string
	if m.confirming {
		statusLine = confirmStyle.Width(m.width).Render(m.confirmPrompt)
	} else if m.toastMsg != "" && time.Now().Before(m.toastExpiry) {
		statusLine = toastStyle.Width(m.width).Render(m.toastMsg)
	} else {
		statusLine = m.statusBar.view()
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, mainContent)
	return lipgloss.JoinVertical(lipgloss.Left, body, statusLine)
}

func (m rootModel) renderSidebar() string {
	panelOH := 3
	availH := m.height - 1

	cmdBodyH := len(commands)
	cmdOuterH := cmdBodyH + panelOH
	repoBodyH := availH - cmdOuterH - panelOH
	if repoBodyH < 3 {
		repoBodyH = 3
	}

	repoPanel := renderBorderedPanel(
		m.repoSelector.title(),
		m.repoSelector.view(),
		sidebarWidth, repoBodyH,
		m.focus == focusRepoSelector,
	)

	cmdPanel := renderBorderedPanel(
		"Commands",
		m.sidebar.view(),
		sidebarWidth, cmdBodyH,
		m.focus == focusCommandMenu,
	)

	return lipgloss.JoinVertical(lipgloss.Left, repoPanel, cmdPanel)
}

func (m rootModel) renderMain() string {
	contentTitle := "Content"
	if m.content.command != "" {
		if cmd := m.sidebar.activeCommand(); cmd != nil {
			contentTitle = cmd.name
			if f, ok := m.cmdFilters[cmd.key]; ok {
				contentTitle += " [" + f.label() + "]"
			}
			total := len(m.content.rows)
			filtered := len(m.content.filteredRows())
			if total > 0 {
				if m.content.filter != "" {
					contentTitle += fmt.Sprintf(" (%d/%d)", filtered, total)
				} else {
					contentTitle += fmt.Sprintf(" (%d)", total)
				}
			}
		}
	}

	contentPanel := renderBorderedPanel(
		contentTitle,
		m.content.view(),
		m.content.width, m.content.height,
		m.focus == focusContent,
	)

	if m.detail.visible {
		detailTitle := m.detail.title
		if detailTitle == "" {
			detailTitle = "Detail"
		}
		detailPanel := renderBorderedPanel(
			detailTitle,
			m.detail.view(),
			m.detail.width, m.detail.height,
			m.focus == focusDetail,
		)
		return lipgloss.JoinHorizontal(lipgloss.Top, contentPanel, detailPanel)
	}

	return contentPanel
}

// -- help --

func (m rootModel) fullHelpView() string {
	var b strings.Builder

	b.WriteString(helpOverlayTitleStyle.Render("  sgh TUI — Keyboard Shortcuts"))
	b.WriteString("\n")
	b.WriteString(separatorStyle.Render("  " + strings.Repeat("─", 40)))
	b.WriteString("\n")

	sections := []struct {
		title string
		binds []struct{ key, desc string }
	}{
		{"Global", []struct{ key, desc string }{
			{"Tab / Shift+Tab", "Cycle panels"},
			{"1 / 2 / 3", "Jump to repos / commands / content"},
			{"q / Ctrl+C", "Quit"},
			{"?", "Toggle this help"},
		}},
		{"Repo Selector", []struct{ key, desc string }{
			{"j/k, Up/Down", "Navigate repos"},
			{"g / G", "Jump to top / bottom"},
			{"Space", "Toggle repo"},
			{"a", "Select all"},
			{"n", "Deselect all"},
			{"/", "Filter repos"},
			{"Enter", "Go to commands"},
		}},
		{"Command Menu", []struct{ key, desc string }{
			{"j/k, Up/Down", "Navigate commands"},
			{"Enter", "Load command"},
			{"Esc", "Back to repos"},
		}},
		{"Content Panel", []struct{ key, desc string }{
			{"j/k, Up/Down", "Navigate rows"},
			{"g / G", "Jump to top / bottom"},
			{"Enter", "Open detail"},
			{"o", "Open in browser"},
			{"/", "Filter rows"},
			{"s", "Cycle status filter (PR/WF)"},
			{"r", "Refresh data"},
			{"Esc", "Close detail / back to commands"},
		}},
		{"PR Actions", []struct{ key, desc string }{
			{"A", "Approve PR"},
			{"m", "Merge PR"},
			{"M", "Approve + Merge"},
			{"c", "Close PR"},
		}},
		{"Workflow Actions", []struct{ key, desc string }{
			{"R", "Rerun workflow"},
			{"X", "Cancel workflow"},
			{"w", "Watch (auto-refresh every 10s)"},
		}},
		{"Detail Panel", []struct{ key, desc string }{
			{"j/k, Up/Down", "Scroll detail"},
			{"g / G", "Jump to top / bottom"},
			{"o", "Open in browser"},
			{"r", "Refresh detail"},
			{"Esc", "Close detail"},
		}},
	}

	for _, sec := range sections {
		b.WriteString(helpOverlaySectionStyle.Render("  " + sec.title))
		b.WriteString("\n")
		for _, bind := range sec.binds {
			b.WriteString(fmt.Sprintf("    %s %s\n",
				helpOverlayKeyStyle.Render(bind.key),
				helpOverlayDescStyle.Render(bind.desc),
			))
		}
	}

	b.WriteString("\n")
	b.WriteString(cachedStyle.Render("  Press any key to close"))
	b.WriteString("\n")

	return b.String()
}

// -- command --

func NewTUICommand(ctx *context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Interactive TUI dashboard",
		Long:  "Launch the full-screen interactive TUI dashboard for managing GitHub resources.",
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if orgName == "" {
				fmt.Println("Error: --org flag is required for the TUI")
				return
			}
			ctx.Silent = true
			m := initialModel(ctx, orgName)
			p := tea.NewProgram(m, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				fmt.Printf("Error running TUI: %v\n", err)
			}
			ctx.Silent = false
		},
	}
}
