// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package tui

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/prady-lab/sgh-cli/internal/model"
	pkgaudit "github.com/prady-lab/sgh-cli/pkg/audit"
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

type (
	reposLoadedMsg          struct{ repos []string }
	repoSelectionChangedMsg struct{ repos []string }
	dataLoadedMsg           struct {
		command string
		columns []string
		rows    [][]string
		colors  [][]lipgloss.Color
		raw     any
	}
)
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
type (
	watchTickMsg  struct{}
	errMsg        struct{ err error }
	diffLoadedMsg struct {
		title string
		lines []string
	}
)

// -- root model --

// toastLevel controls the colour of a toast notification.
type toastLevel int

const (
	toastSuccess toastLevel = iota
	toastInfo
	toastWarning
	toastError
)

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
	// confirmation dialog
	confirming    bool
	confirmPrompt string // short one-liner shown in status bar while confirming
	confirmTitle  string // longer title shown in modal box
	confirmDetail string // resource context shown in modal box (e.g. PR title)
	confirmAction func() tea.Msg
	showHelp      bool
	// toast notification
	toastMsg    string
	toastLevel  toastLevel
	toastExpiry time.Time
	// workflow watch
	watching      bool
	watchRunID    int
	watchRepoName string
	watchInterval time.Duration
	// filters
	cmdFilters     map[string]*commandFilter
	contentFilters map[string]string // persisted filters per command
	// action menu
	showMenu   bool
	menuItems  []menuItem
	menuCursor int
	// diff overlay
	showDiff   bool
	diffLines  []string
	diffTitle  string
	diffScroll int
	// narrow-mode flag (set by relayout)
	narrowMode bool
}

type menuItem struct {
	label  string
	action func() tea.Cmd
}

func initialModel(ctx *context.Context, orgName string) rootModel {
	sb := newStatusBar(orgName)
	sb.loading = true
	sb.loadingMsg = "loading repos"
	sb.command = "loading repos"
	filters := make(map[string]*commandFilter)
	for k, f := range defaultFilters {
		filters[k] = &commandFilter{options: f.options, current: f.current}
	}
	return rootModel{
		repoSelector:   newRepoSelector(nil),
		sidebar:        newSidebar(),
		content:        newContent(),
		detail:         newDetail(),
		statusBar:      sb,
		cache:          newDataCache(5 * time.Minute),
		focus:          focusRepoSelector,
		ctx:            ctx,
		orgName:        orgName,
		cmdFilters:     filters,
		contentFilters: make(map[string]string),
	}
}

// showToast sets a timed notification with severity-based styling.
func (m *rootModel) showToast(msg string, level toastLevel) {
	m.toastMsg = msg
	m.toastLevel = level
	m.toastExpiry = time.Now().Add(5 * time.Second)
}

// startConfirm displays the framed confirmation modal with resource context.
func (m *rootModel) startConfirm(title, detail string, action func() tea.Msg) {
	m.confirming = true
	m.confirmTitle = title
	m.confirmDetail = detail
	m.confirmPrompt = title + " (y/n)"
	m.confirmAction = action
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
			m.statusBar.loadingMsg = fmt.Sprintf("loading %s", m.sidebar.activeCommand().name)
			cmds = append(cmds, m.loadCommand(m.sidebar.activeCommand(), false))
		}

	case dataLoadedMsg:
		m.content.setData(msg.command, msg.columns, msg.rows, msg.colors, msg.raw)
		m.content.noRepos = len(m.selectedRepos) == 0 && msg.command != "team" && msg.command != "audit"
		m.statusBar.loading = false
		m.statusBar.command = msg.command
		m.statusBar.cacheAge = m.cache.age(m.cacheKeyFor(msg.command))
		if f, ok := m.cmdFilters[msg.command]; ok {
			m.statusBar.activeFilter = f.value()
		} else {
			m.statusBar.activeFilter = ""
		}
		m.statusBar.totalCount = len(msg.rows)
		// Restore saved filter for this command
		if savedFilter, ok := m.contentFilters[msg.command]; ok {
			m.content.filter = savedFilter
		} else {
			m.content.filter = ""
		}
		m.statusBar.filteredCount = len(m.content.filteredRows())

	case detailLoadedMsg:
		m.detail.setData(msg.title, msg.fields)
		m.relayout()
		m.statusBar.loading = false
		if m.watching {
			if strings.HasSuffix(msg.title, "(completed)") {
				m.watching = false
				m.showToast("Workflow run completed", toastSuccess)
			} else if strings.HasSuffix(msg.title, "(watching...)") {
				cmds = append(cmds, tea.Tick(m.watchInterval, func(time.Time) tea.Msg { return watchTickMsg{} }))
			}
		} else if msg.autoWatch {
			m.watching = true
			m.watchRunID = msg.watchRunID
			m.watchRepoName = msg.watchRepoName
			m.watchInterval = 10 * time.Second
			m.showToast(fmt.Sprintf("Auto-watching workflow %d", msg.watchRunID), toastInfo)
			cmds = append(cmds, tea.Tick(m.watchInterval, func(time.Time) tea.Msg { return watchTickMsg{} }))
		}
		m.applyFocus()

	case actionResultMsg:
		m.statusBar.loading = false
		if msg.success {
			m.showToast(msg.message, toastSuccess)
			m.cache.invalidate(msg.command)
			if cmd := m.sidebar.activeCommand(); cmd != nil && cmd.key == msg.command {
				m.content.loading = true
				m.statusBar.loading = true
				m.statusBar.loadingMsg = fmt.Sprintf("refreshing %s", cmd.name)
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

	case diffLoadedMsg:
		m.statusBar.loading = false
		m.diffTitle = msg.title
		m.diffLines = msg.lines
		m.diffScroll = 0
		m.showDiff = true

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

		if m.showMenu {
			switch msg.String() {
			case "up", "k":
				if m.menuCursor > 0 {
					m.menuCursor--
				}
			case "down", "j":
				if m.menuCursor < len(m.menuItems)-1 {
					m.menuCursor++
				}
			case "enter":
				if m.menuCursor >= 0 && m.menuCursor < len(m.menuItems) {
					action := m.menuItems[m.menuCursor].action
					m.showMenu = false
					m.menuItems = nil
					m.menuCursor = 0
					if action != nil {
						if cmd := action(); cmd != nil {
							cmds = append(cmds, cmd)
						}
					}
				}
			case "esc", "m":
				m.showMenu = false
				m.menuItems = nil
				m.menuCursor = 0
			}
			return m, tea.Batch(cmds...)
		}

		if m.showDiff {
			m.handleDiffKey(msg.String())
			return m, tea.Batch(cmds...)
		}

		if m.repoSelector.filtering {
			m.repoSelector.handleFilterKey(key.Binding{}, msg.String())
			return m, nil
		}
		if m.content.filtering {
			m.content.handleFilterKey(msg.String())
			m.statusBar.filteredCount = len(m.content.filteredRows())
			// Save filter when exiting filter mode
			if !m.content.filtering && m.content.filter != "" {
				if cmd := m.sidebar.activeCommand(); cmd != nil {
					m.contentFilters[cmd.key] = m.content.filter
				}
			}
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
	panelNav := "tab:next"
	switch m.focus {
	case focusRepoSelector:
		m.repoSelector.focused = true
		m.statusBar.focusHint = "space:toggle a:all n:none /:filter " + panelNav
	case focusCommandMenu:
		m.sidebar.focused = true
		m.statusBar.focusHint = "enter:select 1-3:jump " + panelNav
	case focusContent:
		m.content.focused = true
		hint := "enter:detail o:open y:copy S:sort"
		if cmd := m.sidebar.activeCommand(); cmd != nil {
			switch cmd.key {
			case "pr":
				hint += " d:diff A:approve ctrl+m:merge m:menu"
			case "wf":
				hint += " m:menu"
			case "issue", "branch", "tag":
				hint += " m:menu"
			}
			if _, ok := m.cmdFilters[cmd.key]; ok {
				hint += " s:filter"
			}
		}
		hint += " r:refresh ctrl+r:hard-refresh /:search"
		m.statusBar.focusHint = hint
	case focusDetail:
		m.detail.focused = true
		hint := "enter/o:open y:copy"
		if cmd := m.sidebar.activeCommand(); cmd != nil && cmd.key == "wf" {
			if m.watching {
				hint += " w:stop-watch"
			} else {
				hint += " w:watch"
			}
		}
		hint += " r:refresh esc:close"
		m.statusBar.focusHint = hint
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
	case key.Matches(msg, keys.PageUp):
		m.repoSelector.pageUp()
	case key.Matches(msg, keys.PageDown):
		m.repoSelector.pageDown()
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
		// If a command is already loaded and repos are selected, jump straight to content
		if m.content.command != "" && len(m.selectedRepos) > 0 {
			m.focus = focusContent
		} else {
			m.focus = focusCommandMenu
		}
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
		idx := m.sidebar.selectCommand()
		if idx >= 0 && idx < len(commands) {
			m.content.loading = true
			m.statusBar.loading = true
			m.statusBar.loadingMsg = fmt.Sprintf("loading %s", commands[idx].name)
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
	case key.Matches(msg, keys.PageUp):
		m.content.pageUp()
	case key.Matches(msg, keys.PageDown):
		m.content.pageDown()
	case key.Matches(msg, keys.Enter):
		return m.loadDetail()
	case key.Matches(msg, keys.Filter):
		m.content.filtering = true
		m.content.filter = ""
	case key.Matches(msg, keys.Refresh):
		if cmd != nil {
			m.content.loading = true
			m.statusBar.loading = true
			m.statusBar.loadingMsg = fmt.Sprintf("refreshing %s", cmd.name)
			return m.loadCommand(cmd, true)
		}
	case key.Matches(msg, keys.HardRefresh):
		if cmd != nil {
			m.cache.invalidate(cmd.key)
			m.content.loading = true
			m.statusBar.loading = true
			m.statusBar.loadingMsg = fmt.Sprintf("hard refresh %s", cmd.name)
			m.showToast("Cache cleared", toastInfo)
			return m.loadCommand(cmd, true)
		}
	case key.Matches(msg, keys.Sort):
		m.content.cycleSort()

	case key.Matches(msg, keys.CycleFilter):
		if cmd != nil {
			if f, ok := m.cmdFilters[cmd.key]; ok {
				f.cycle()
				m.statusBar.activeFilter = f.value()
				m.cache.invalidate(cmd.key)
				m.content.loading = true
				m.statusBar.loading = true
				m.statusBar.loadingMsg = fmt.Sprintf("loading %s (%s)", cmd.name, f.value())
				return m.loadCommand(cmd, true)
			}
		}
	case key.Matches(msg, keys.Open):
		if url := m.getSelectedURL(); url != "" {
			_ = openURL(url)
			m.showToast("Opened in browser", toastInfo)
		}
	case key.Matches(msg, keys.Yank):
		if url := m.getSelectedURL(); url != "" {
			if err := ui.CopyToClipboard(url); err == nil {
				m.showToast("URL copied to clipboard", toastSuccess)
			} else {
				m.showToast("Copy failed: "+err.Error(), toastWarning)
			}
		}
	case key.Matches(msg, keys.Esc):
		if m.detail.visible {
			m.detail.clear()
			m.relayout()
		} else if m.content.filter != "" {
			// Save current filter before clearing
			if cmd := m.sidebar.activeCommand(); cmd != nil {
				m.contentFilters[cmd.key] = m.content.filter
			}
			m.content.filter = ""
			m.statusBar.filteredCount = len(m.content.filteredRows())
		} else {
			m.clearAllFocus()
			m.focus = focusCommandMenu
			m.applyFocus()
		}

	case key.Matches(msg, keys.Diff):
		if cmd != nil && cmd.key == "pr" {
			return m.loadPRDiff()
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
	case key.Matches(msg, keys.Menu):
		if cmd != nil {
			m.buildContextMenu(cmd.key)
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

	m.startConfirm(prompt, fmt.Sprintf("PR #%d · %s", p.PRNumber, truncateField(p.TitleName, 50)), func() tea.Msg {
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
	})
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

	m.startConfirm(prompt, fmt.Sprintf("Run #%d · %s @ %s", w.ID, w.Name, w.HeadBranch), func() tea.Msg {
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
	})
	return nil
}

func (m *rootModel) issueAction(action, prompt string) tea.Cmd {
	rowIdx := m.content.selectedRowIndex()
	results, ok := m.content.rawData.([]model.IssueResponse)
	if !ok || rowIdx < 0 || rowIdx >= len(results) {
		return nil
	}
	is := results[rowIdx]
	ctx := m.ctx
	org := m.orgName

	m.startConfirm(prompt, fmt.Sprintf("Issue #%d · %s", is.Number, truncateField(is.Title, 50)), func() tea.Msg {
		resp := issue.UpdateIssue(ctx, issue.IssueUpdateRequest{
			OrgName: org, RepoName: is.RepositoryName, IssueNumber: is.Number, State: action,
		})
		if resp.ErrorMessage != "" {
			return actionResultMsg{false, resp.ErrorMessage, "issue"}
		}
		verb := "updated"
		if action == "close" {
			verb = "closed"
		} else if action == "open" {
			verb = "reopened"
		}
		return actionResultMsg{true, fmt.Sprintf("Issue #%d %s", is.Number, verb), "issue"}
	})
	return nil
}

func (m *rootModel) branchAction(action, prompt string) tea.Cmd {
	rowIdx := m.content.selectedRowIndex()
	results, ok := m.content.rawData.([]model.BranchResponse)
	if !ok || rowIdx < 0 || rowIdx >= len(results) {
		return nil
	}
	b := results[rowIdx]
	ctx := m.ctx
	org := m.orgName

	m.startConfirm(prompt, fmt.Sprintf("%s / %s", b.RepositoryName, b.Name), func() tea.Msg {
		switch action {
		case "delete":
			resps := branch.DeleteBranches(ctx, branch.BranchDeleteRequest{
				OrgName: org, RepoNames: []string{b.RepositoryName}, BranchName: b.Name,
			})
			for _, r := range resps {
				if r.ErrorMessage != "" {
					return actionResultMsg{false, r.ErrorMessage, "branch"}
				}
			}
			return actionResultMsg{true, fmt.Sprintf("Branch %s deleted", b.Name), "branch"}
		}
		return nil
	})
	return nil
}

func (m *rootModel) tagAction(action, prompt string) tea.Cmd {
	rowIdx := m.content.selectedRowIndex()
	results, ok := m.content.rawData.([]model.TagResponse)
	if !ok || rowIdx < 0 || rowIdx >= len(results) {
		return nil
	}
	t := results[rowIdx]
	ctx := m.ctx
	org := m.orgName

	m.startConfirm(prompt, fmt.Sprintf("%s / %s", t.RepositoryName, t.Name), func() tea.Msg {
		switch action {
		case "delete":
			resps := tag.DeleteTags(ctx, tag.TagDeleteRequest{
				OrgName: org, RepoNames: []string{t.RepositoryName}, TagName: t.Name,
			})
			for _, r := range resps {
				if r.ErrorMessage != "" {
					return actionResultMsg{false, r.ErrorMessage, "tag"}
				}
			}
			return actionResultMsg{true, fmt.Sprintf("Tag %s deleted", t.Name), "tag"}
		}
		return nil
	})
	return nil
}

func (m *rootModel) buildContextMenu(cmdKey string) {
	m.menuItems = nil
	m.menuCursor = 0

	openBrowserItem := menuItem{"Open in Browser (o)", func() tea.Cmd {
		if url := m.getSelectedURL(); url != "" {
			_ = openURL(url)
			m.showToast("Opened in browser", toastSuccess)
		}
		return nil
	}}
	refreshItem := menuItem{"Refresh (r)", func() tea.Cmd {
		if cmd := m.sidebar.activeCommand(); cmd != nil {
			m.content.loading = true
			m.statusBar.loading = true
			m.statusBar.loadingMsg = fmt.Sprintf("refreshing %s", cmd.name)
			return m.loadCommand(cmd, true)
		}
		return nil
	}}

	switch cmdKey {
	case "pr":
		m.menuItems = []menuItem{
			{"Approve PR (A)", func() tea.Cmd { return m.prAction("APPROVE", "Approve this PR?") }},
			{"Merge PR (ctrl+m)", func() tea.Cmd { return m.prAction("merge", "Merge this PR?") }},
			{"Approve & Merge (M)", func() tea.Cmd { return m.prAction("approve+merge", "Approve and merge this PR?") }},
			{"Close PR (c)", func() tea.Cmd { return m.prAction("close", "Close this PR?") }},
			{"Diff Preview (d)", func() tea.Cmd { return m.loadPRDiff() }},
			openBrowserItem,
			refreshItem,
		}
	case "wf":
		m.menuItems = []menuItem{
			{"Rerun Workflow (R)", func() tea.Cmd { return m.wfAction("rerun", "Rerun this workflow?") }},
			{"Cancel Workflow (X)", func() tea.Cmd { return m.wfAction("cancel", "Cancel this workflow?") }},
			openBrowserItem,
			refreshItem,
		}
	case "issue":
		m.menuItems = []menuItem{
			{"Close Issue", func() tea.Cmd { return m.issueAction("close", "Close this issue?") }},
			{"Reopen Issue", func() tea.Cmd { return m.issueAction("open", "Reopen this issue?") }},
			openBrowserItem,
			refreshItem,
		}
	case "branch":
		m.menuItems = []menuItem{
			{"Delete Branch", func() tea.Cmd { return m.branchAction("delete", "Delete this branch?") }},
			openBrowserItem,
			refreshItem,
		}
	case "tag":
		m.menuItems = []menuItem{
			{"Delete Tag", func() tea.Cmd { return m.tagAction("delete", "Delete this tag?") }},
			openBrowserItem,
			refreshItem,
		}
	default:
		m.menuItems = []menuItem{openBrowserItem, refreshItem}
	}

	m.showMenu = true
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
	case key.Matches(msg, keys.PageUp):
		m.detail.pageUp()
	case key.Matches(msg, keys.PageDown):
		m.detail.pageDown()
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
				m.showToast("Watch mode stopped", toastInfo)
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
			m.showToast("Opened in browser", toastInfo)
		}
	case key.Matches(msg, keys.Enter):
		if url := m.getSelectedURL(); url != "" {
			_ = openURL(url)
			m.showToast("Opened in browser", toastInfo)
		}
	case key.Matches(msg, keys.Yank):
		if url := m.getSelectedURL(); url != "" {
			if err := ui.CopyToClipboard(url); err == nil {
				m.showToast("URL copied to clipboard", toastSuccess)
			} else {
				m.showToast("Copy failed: "+err.Error(), toastWarning)
			}
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
	m.showToast(fmt.Sprintf("Watching workflow %d (every 10s)", w.ID), toastInfo)
	return tea.Tick(m.watchInterval, func(time.Time) tea.Msg { return watchTickMsg{} })
}

// openURL opens a URL in the system default browser (delegates to pkg/ui).
func openURL(url string) error {
	return ui.OpenURL(url)
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

// effectiveSidebarWidth computes the sidebar inner content width based on
// the longest repo name, clamped between 24 and 42, with a tighter cap on
// narrow/compact terminals.
func (m rootModel) effectiveSidebarWidth() int {
	maxRepoLen := 0
	for _, r := range m.repoSelector.repos {
		if len(r.name) > maxRepoLen {
			maxRepoLen = len(r.name)
		}
	}
	w := maxRepoLen + 10
	if w < 24 {
		w = 24
	}
	if w > 42 {
		w = 42
	}
	if m.narrowMode || m.width < 110 {
		if w > 28 {
			w = 28
		}
	}
	return w
}

func (m *rootModel) relayout() {
	// renderBorderedPanel(title, body, w, h): outer width = w+4, outer height = h+3
	// h = body height (excludes title line); +1 title +2 border = +3 total
	widthOH := 4 // 2 border chars + 2 padding chars
	panelOH := 3 // 1 title line + 2 border lines (top+bottom)

	availH := m.height - 1 // 1 line for status bar

	// Two-tier responsive layout:
	//   narrowMode    (< 80):  detail becomes overlay, sidebar slightly narrower
	//   compactSidebar (< 110): sidebar reduced but detail still side-by-side
	m.narrowMode = m.width < 80

	effectiveSidebarW := m.effectiveSidebarWidth()
	sbOuterW := effectiveSidebarW + widthOH
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

	// In narrow mode the detail panel floats as an overlay; content gets full main width
	if m.detail.visible && !m.narrowMode {
		detailOuterW := mainW * 2 / 5 // 40% for detail, 60% for content
		if detailOuterW < 44 {
			detailOuterW = 44
		}
		if detailOuterW > mainW-40 {
			detailOuterW = mainW - 40
		}
		contentOuterW := mainW - detailOuterW
		m.detail.width = detailOuterW - widthOH
		m.content.width = contentOuterW - widthOH
	} else {
		m.content.width = mainW - widthOH
		// Keep a usable detail width for the overlay
		overlayW := m.width - 8
		if overlayW < 30 {
			overlayW = 30
		}
		m.detail.width = overlayW - widthOH
	}
	if m.content.width < 10 {
		m.content.width = 10
	}
	if m.detail.width < 10 {
		m.detail.width = 10
	}
	m.content.height = mainBodyH
	if m.narrowMode {
		m.detail.height = mainBodyH - 4 // slightly shorter for overlay padding
		if m.detail.height < 5 {
			m.detail.height = 5
		}
	} else {
		m.detail.height = mainBodyH
	}
	m.statusBar.width = m.width
}

// -- data loading --

// cacheKeyFor returns a composite cache key that includes the active filter state
// so that "pr:open" and "pr:merged" are stored separately.
func (m *rootModel) cacheKeyFor(cmdKey string) string {
	if f, ok := m.cmdFilters[cmdKey]; ok {
		return cmdKey + ":" + f.value()
	}
	return cmdKey
}

func (m *rootModel) loadCommand(cmd *commandDef, forceRefresh bool) tea.Cmd {
	repos := m.selectedRepos
	ctx := m.ctx
	org := m.orgName

	if len(repos) == 0 && cmd.key != "team" {
		return func() tea.Msg {
			return dataLoadedMsg{command: cmd.key, columns: cmd.columns, rows: nil, raw: nil}
		}
	}

	cacheKey := m.cacheKeyFor(cmd.key)
	if !forceRefresh {
		if cached, hit, stale := m.cache.get(cacheKey, repos); hit && !stale {
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
	case "audit":
		return m.loadAuditLog(ctx, org, cmd.columns)
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
		m.cache.set(m.cacheKeyFor("pr"), repos, msg)
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
		m.cache.set(m.cacheKeyFor("issue"), repos, msg)
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
		m.cache.set("branch", repos, msg) // branch has no filter, key is plain
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
		m.cache.set(m.cacheKeyFor("wf"), repos, msg)
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

func (m *rootModel) loadAuditLog(ctx *context.Context, org string, cols []string) tea.Cmd {
	return func() tea.Msg {
		resp := pkgaudit.ListAuditLog(ctx, pkgaudit.AuditListRequest{OrgName: org, Count: 100})
		if resp.ErrorMessage != "" {
			return errMsg{errors.New(resp.ErrorMessage)}
		}
		rows := make([][]string, 0, len(resp.Entries))
		colors := make([][]lipgloss.Color, 0, len(resp.Entries))
		for _, e := range resp.Entries {
			ts := ""
			if e.CreatedAt > 0 {
				ts = pkgaudit.FormatTimestamp(e.CreatedAt)
			}
			repoName := e.Repo
			if repoName == "" {
				repoName = "-"
			}
			rows = append(rows, []string{ts, e.Actor, e.Action, repoName})
			colors = append(colors, []lipgloss.Color{"", "", "", ""})
		}
		msg := &dataLoadedMsg{command: "audit", columns: cols, rows: rows, colors: colors, raw: resp.Entries}
		m.cache.set("audit", nil, msg)
		return *msg
	}
}

func (m *rootModel) loadAuditDetail(rowIdx int) tea.Cmd {
	results, ok := m.content.rawData.([]model.AuditLogEntry)
	if !ok || rowIdx < 0 || rowIdx >= len(results) {
		return nil
	}
	e := results[rowIdx]
	return func() tea.Msg {
		ts := ""
		if e.CreatedAt > 0 {
			ts = pkgaudit.FormatTimestamp(e.CreatedAt)
		}
		fields := []detailField{
			{"Action", e.Action, ""},
			{"Actor", e.Actor, ""},
			{"Time", ts, ""},
		}
		if e.Repo != "" {
			fields = append(fields, detailField{"Repo", e.Repo, ""})
		}
		if e.User != "" && e.User != e.Actor {
			fields = append(fields, detailField{"User", e.User, ""})
		}
		if e.ActorIP != "" {
			fields = append(fields, detailField{"Actor IP", e.ActorIP, ""})
		}
		if e.OrgName != "" {
			fields = append(fields, detailField{"Org", e.OrgName, ""})
		}
		if len(e.Data) > 0 {
			fields = append(fields, detailField{"", "", ""})
			dataKeys := make([]string, 0, len(e.Data))
			for k := range e.Data {
				dataKeys = append(dataKeys, k)
			}
			sort.Strings(dataKeys)
			for _, k := range dataKeys {
				fields = append(fields, detailField{k, fmt.Sprintf("%v", e.Data[k]), ""})
			}
		}
		return detailLoadedMsg{title: e.Action, fields: fields}
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
	case "branch":
		return m.loadBranchDetail(rowIdx)
	case "tag":
		return m.loadTagDetail(rowIdx)
	case "commit":
		return m.loadCommitDetail(rowIdx)
	case "team":
		return m.loadTeamDetail(rowIdx)
	case "pb":
		return m.loadPBDetail(rowIdx)
	case "audit":
		return m.loadAuditDetail(rowIdx)
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
			{"Title", truncateField(is.Title, 60), ""},
			{"Author", is.AuthorName(), ""},
			{"State", is.State, statusColor(is.State)},
			{"Labels", truncateField(is.LabelNames(), 60), ""},
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
			{"Title", truncateField(prResp.TitleName, 60), ""},
			{"Author", prResp.AuthorName(), ""},
			{"State", prResp.State, statusColor(prResp.State)},
			{"Branch", truncateField(prResp.Base.Ref+" ← "+prResp.Head.Ref, 50), ""},
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
				filename := f.Filename
				if len(filename) > 45 {
					filename = "..." + filename[len(filename)-42:]
				}
				fileLine := fmt.Sprintf("%s %s  %s %s",
					dimStyle.Render(changeIcon),
					filename,
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

func (m *rootModel) loadBranchDetail(rowIdx int) tea.Cmd {
	results, ok := m.content.rawData.([]model.BranchResponse)
	if !ok || rowIdx < 0 || rowIdx >= len(results) {
		return nil
	}
	b := results[rowIdx]
	org := m.orgName
	return func() tea.Msg {
		protected := "no"
		if b.Protected {
			protected = "yes"
		}
		fields := []detailField{
			{"Repository", b.RepositoryName, ""},
			{"Branch", b.Name, ""},
			{"SHA", b.Commit.SHA, ""},
			{"Protected", protected, statusColor(protected)},
			{"", "", ""},
			{"URL", fmt.Sprintf("https://github.com/%s/%s/tree/%s", org, b.RepositoryName, b.Name), ""},
		}
		return detailLoadedMsg{title: b.Name, fields: fields}
	}
}

func (m *rootModel) loadTagDetail(rowIdx int) tea.Cmd {
	results, ok := m.content.rawData.([]model.TagResponse)
	if !ok || rowIdx < 0 || rowIdx >= len(results) {
		return nil
	}
	t := results[rowIdx]
	org := m.orgName
	return func() tea.Msg {
		fields := []detailField{
			{"Repository", t.RepositoryName, ""},
			{"Tag", t.Name, ""},
			{"SHA", t.Commit.SHA, ""},
			{"", "", ""},
			{"Release URL", fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", org, t.RepositoryName, t.Name), ""},
			{"Zipball", t.ZipballURL, ""},
			{"Tarball", t.TarballURL, ""},
		}
		return detailLoadedMsg{title: t.Name, fields: fields}
	}
}

func (m *rootModel) loadCommitDetail(rowIdx int) tea.Cmd {
	results, ok := m.content.rawData.([]model.CommitResponse)
	if !ok || rowIdx < 0 || rowIdx >= len(results) {
		return nil
	}
	c := results[rowIdx]
	return func() tea.Msg {
		authorName := c.Commit.Author.Name
		if authorName == "" {
			authorName = c.Author.Login
		}
		committerName := c.Commit.Committer.Name
		if committerName == "" {
			committerName = c.Committer.Login
		}
		msg := c.Commit.Message
		if idx := strings.Index(msg, "\n"); idx > 0 {
			msg = msg[:idx]
		}
		fields := []detailField{
			{"Repository", c.RepositoryName, ""},
			{"SHA", ui.ShortSHA(c.Sha), ""},
			{"Author", authorName, ""},
			{"Date", ui.RelativeTime(c.Commit.Author.Date), ""},
			{"Committer", committerName, ""},
			{"Message", truncateField(msg, 80), ""},
			{"Changes", fmt.Sprintf("+%d -%d (total: %d)", c.Stats.Additions, c.Stats.Deletions, c.Stats.Total), ""},
		}
		if len(c.Files) > 0 {
			addStyle := lipgloss.NewStyle().Foreground(ui.Green)
			delStyle := lipgloss.NewStyle().Foreground(ui.Red)
			dimStyle := lipgloss.NewStyle().Foreground(ui.Dimmed)
			fields = append(fields, detailField{"", "", ""})
			fields = append(fields, detailField{"Files", fmt.Sprintf("%d changed", len(c.Files)), ""})
			for i, f := range c.Files {
				if i >= 8 {
					fields = append(fields, detailField{"", dimStyle.Render(fmt.Sprintf("... and %d more", len(c.Files)-8)), ""})
					break
				}
				name := f.Filename
				if len(name) > 45 {
					name = "..." + name[len(name)-42:]
				}
				line := fmt.Sprintf("%s  %s %s",
					name,
					addStyle.Render(fmt.Sprintf("+%d", f.Additions)),
					delStyle.Render(fmt.Sprintf("-%d", f.Deletions)),
				)
				fields = append(fields, detailField{"", line, ""})
			}
		}
		if c.HtmlUrl != "" {
			fields = append(fields, detailField{"", "", ""})
			fields = append(fields, detailField{"URL", c.HtmlUrl, ""})
		}
		return detailLoadedMsg{title: ui.ShortSHA(c.Sha) + " " + truncateField(msg, 40), fields: fields}
	}
}

func (m *rootModel) loadTeamDetail(rowIdx int) tea.Cmd {
	results, ok := m.content.rawData.([]model.OrgTeam)
	if !ok || rowIdx < 0 || rowIdx >= len(results) {
		return nil
	}
	t := results[rowIdx]
	return func() tea.Msg {
		fields := []detailField{
			{"Team", t.Name, ""},
			{"Members", fmt.Sprintf("%d", t.TotalMembers), ""},
			{"Repos", fmt.Sprintf("%d", t.RepositoriesCount), ""},
		}
		if t.Url != "" {
			fields = append(fields, detailField{"URL", t.Url, ""})
		}
		if len(t.Members) > 0 {
			fields = append(fields, detailField{"", "", ""})
			fields = append(fields, detailField{"Member List", fmt.Sprintf("%d members", len(t.Members)), ""})
			for i, mem := range t.Members {
				if i >= 15 {
					dimStyle := lipgloss.NewStyle().Foreground(ui.Dimmed)
					fields = append(fields, detailField{"", dimStyle.Render(fmt.Sprintf("... and %d more", len(t.Members)-15)), ""})
					break
				}
				name := mem.Login
				if mem.Name != "" {
					name = mem.Name + " (" + mem.Login + ")"
				}
				fields = append(fields, detailField{"", name, ""})
			}
		}
		return detailLoadedMsg{title: t.Name, fields: fields}
	}
}

func (m *rootModel) loadPBDetail(rowIdx int) tea.Cmd {
	results, ok := m.content.rawData.([]model.ProtectedBranch)
	if !ok || rowIdx < 0 || rowIdx >= len(results) {
		return nil
	}
	pb := results[rowIdx]
	org := m.orgName
	return func() tea.Msg {
		boolStr := func(b bool) string {
			if b {
				return "yes"
			}
			return "no"
		}
		rpr := pb.RequiredPullRequestReviews
		rsc := pb.RequiredStatusChecks
		fields := []detailField{
			{"Repository", pb.RepositoryName, ""},
			{"Branch", pb.Name, ""},
			{"", "", ""},
			{"Enforce Admins", boolStr(pb.EnforceAdmins), ""},
			{"Lock Branch", boolStr(pb.LockBranch), ""},
			{"Conv. Resolution", boolStr(pb.RequiredConversationResolution), ""},
			{"", "", ""},
			{"Required Approvals", fmt.Sprintf("%d", rpr.RequiredApprovingReviewCount), ""},
			{"Dismiss Stale", boolStr(rpr.DismissStaleReviews), ""},
			{"Code Owner Reviews", boolStr(rpr.RequireCodeOwnerReviews), ""},
			{"Last Push Approval", boolStr(rpr.RequireLastPushApproval), ""},
		}
		if rsc.Strict || len(rsc.Contexts) > 0 {
			fields = append(fields, detailField{"", "", ""})
			fields = append(fields, detailField{"Status Checks Strict", boolStr(rsc.Strict), ""})
			if len(rsc.Contexts) > 0 {
				fields = append(fields, detailField{"Required Checks", strings.Join(rsc.Contexts, ", "), ""})
			}
		}
		if len(pb.RepositoryRulesetNames) > 0 {
			fields = append(fields, detailField{"", "", ""})
			fields = append(fields, detailField{"Rulesets", strings.Join(pb.RepositoryRulesetNames, ", "), ""})
		}
		fields = append(fields, detailField{"", "", ""})
		fields = append(fields, detailField{"Settings URL", fmt.Sprintf("https://github.com/%s/%s/settings/branches", org, pb.RepositoryName), ""})
		return detailLoadedMsg{title: pb.RepositoryName + "/" + pb.Name, fields: fields}
	}
}

// -- helpers --

func truncateField(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func buildWorkflowDetailFields(detail model.WorkflowRunDetail) []detailField {
	fields := []detailField{
		{"Repository", detail.Run.RepositoryName, ""},
		{"Workflow", truncateField(detail.Run.Name, 50), ""},
		{"Run ID", fmt.Sprintf("%d", detail.Run.ID), ""},
		{"Status", detail.Run.Status, statusColor(detail.Run.Status)},
		{"Conclusion", detail.Run.Conclusion, statusColor(detail.Run.Conclusion)},
		{"Branch", truncateField(detail.Run.HeadBranch, 40), ""},
		{"Event", detail.Run.Event, ""},
		{"Created", ui.RelativeTime(detail.Run.CreatedAt), ""},
		{"Updated", ui.RelativeTime(detail.Run.UpdatedAt), ""},
	}

	for _, j := range detail.Jobs {
		status := j.Conclusion
		if status == "" {
			status = j.Status
		}
		jobName := j.Name
		if len(jobName) > 50 {
			jobName = jobName[:47] + "..."
		}
		fields = append(fields, detailField{"Job: " + jobName, status, statusColor(status)})
		for _, s := range j.Steps {
			stepStatus := s.Conclusion
			if stepStatus == "" {
				stepStatus = s.Status
			}
			stepName := s.Name
			if len(stepName) > 48 {
				stepName = stepName[:45] + "..."
			}
			fields = append(fields, detailField{"  " + stepName, stepStatus, statusColor(stepStatus)})
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

	if m.ctx.HttpClient != nil {
		m.statusBar.apiCalls = int(m.ctx.HttpClient.APICallCount())
	}

	sidebar := m.renderSidebar()
	mainContent := m.renderMain()

	var statusLine string
	if m.confirming {
		statusLine = confirmStyle.Width(m.width).Render(m.confirmPrompt)
	} else if m.toastMsg != "" && time.Now().Before(m.toastExpiry) {
		statusLine = m.renderToast()
	} else {
		statusLine = m.statusBar.view()
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, mainContent)
	view := lipgloss.JoinVertical(lipgloss.Left, body, statusLine)

	if m.confirming {
		view = m.renderConfirmModal(view)
	}
	if m.narrowMode && m.detail.visible {
		view = m.renderNarrowDetailOverlay(view)
	}
	if m.showMenu {
		view = m.renderMenuOverlay(view)
	}
	if m.showDiff {
		view = m.renderDiffOverlay(view)
	}
	if m.showHelp {
		view = m.renderHelpOverlay(view)
	}

	return view
}

func (m rootModel) renderToast() string {
	var style lipgloss.Style
	prefix := ""
	switch m.toastLevel {
	case toastSuccess:
		style = lipgloss.NewStyle().Foreground(ui.Green).Bold(true).Padding(0, 1)
		prefix = "✓ "
	case toastWarning:
		style = lipgloss.NewStyle().Foreground(ui.Yellow).Bold(true).Padding(0, 1)
		prefix = "⚠ "
	case toastError:
		style = lipgloss.NewStyle().Foreground(ui.Red).Bold(true).Padding(0, 1)
		prefix = "✗ "
	default: // info
		style = lipgloss.NewStyle().Foreground(ui.Cyan).Italic(true).Padding(0, 1)
		prefix = "ℹ "
	}
	return style.Width(m.width).Render(prefix + m.toastMsg)
}

func (m rootModel) renderConfirmModal(base string) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.Yellow)
	detailStyle := lipgloss.NewStyle().Foreground(ui.White)
	hintStyle := lipgloss.NewStyle().Foreground(ui.Dimmed).Italic(true)

	inner := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(m.confirmTitle),
		"",
		detailStyle.Render(m.confirmDetail),
		"",
		hintStyle.Render("Press y to confirm, any other key to cancel"),
	)

	boxWidth := lipgloss.Width(inner) + 6
	if boxWidth < 40 {
		boxWidth = 40
	}
	box := panelFocusedStyle.
		Width(boxWidth).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(ui.Yellow).
		BorderTop(true).BorderBottom(true).BorderLeft(true).BorderRight(true).
		Render(inner)

	return m.overlayCenter(base, box)
}

func (m rootModel) renderMenuOverlay(base string) string {
	if len(m.menuItems) == 0 {
		return base
	}

	// Build menu content
	var menuLines []string
	menuLines = append(menuLines, panelTitleFocusedStyle.Render(" Actions "))
	menuLines = append(menuLines, "")

	maxWidth := 0
	for i, item := range m.menuItems {
		label := item.label
		if i == m.menuCursor {
			label = "▸ " + label
			label = lipgloss.NewStyle().Foreground(ui.Cyan).Bold(true).Render(label)
		} else {
			label = "  " + label
		}
		menuLines = append(menuLines, label)
		if len(item.label)+3 > maxWidth {
			maxWidth = len(item.label) + 3
		}
	}

	menuLines = append(menuLines, "")
	menuLines = append(menuLines, contentRowDimStyle.Render("  ↑/↓: navigate  Enter: select  Esc: cancel"))

	menuContent := strings.Join(menuLines, "\n")
	menuBox := panelFocusedStyle.
		Width(maxWidth + 4).
		BorderTop(true).BorderBottom(true).BorderLeft(true).BorderRight(true).
		Render(menuContent)

	return m.overlayCenter(base, menuBox)
}

// overlayCenter places a pre-rendered box string centered over base using
// lipgloss.Place, which is ANSI-aware and handles escape codes correctly.
func (m rootModel) overlayCenter(base, box string) string {
	return lipgloss.Place(m.width, m.height-1, lipgloss.Center, lipgloss.Center, box)
}

// -- diff overlay --

// diffVisible returns the number of visible lines for the diff overlay.
func (m rootModel) diffVisible() int {
	v := m.height - 8
	if v < 5 {
		v = 5
	}
	return v
}

// handleDiffKey processes a key press while the diff overlay is open.
func (m *rootModel) handleDiffKey(k string) {
	visible := m.diffVisible()
	maxScroll := len(m.diffLines) - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	switch k {
	case "esc", "q":
		m.showDiff = false
		m.diffLines = nil
	case "up", "k":
		if m.diffScroll > 0 {
			m.diffScroll--
		}
	case "down", "j":
		if m.diffScroll < maxScroll {
			m.diffScroll++
		}
	case "ctrl+u", "pgup":
		m.diffScroll -= visible
		if m.diffScroll < 0 {
			m.diffScroll = 0
		}
	case "ctrl+d", "pgdown":
		m.diffScroll += visible
		if m.diffScroll > maxScroll {
			m.diffScroll = maxScroll
		}
	case "g":
		m.diffScroll = 0
	case "G":
		m.diffScroll = maxScroll
	}
}

func (m rootModel) renderDiffOverlay(base string) string {
	if len(m.diffLines) == 0 {
		return base
	}
	visible := m.diffVisible()
	end := m.diffScroll + visible
	if end > len(m.diffLines) {
		end = len(m.diffLines)
	}

	addStyle := lipgloss.NewStyle().Foreground(ui.Green)
	delStyle := lipgloss.NewStyle().Foreground(ui.Red)
	dimStyle := lipgloss.NewStyle().Foreground(ui.Dimmed)
	fileHeaderStyle := lipgloss.NewStyle().Foreground(ui.Cyan).Bold(true)

	var lines []string
	for _, l := range m.diffLines[m.diffScroll:end] {
		switch {
		case strings.HasPrefix(l, "--- ") || strings.HasPrefix(l, "+++ "):
			lines = append(lines, fileHeaderStyle.Render(l))
		case strings.HasPrefix(l, "+"):
			lines = append(lines, addStyle.Render(l))
		case strings.HasPrefix(l, "-"):
			lines = append(lines, delStyle.Render(l))
		case strings.HasPrefix(l, "@@"):
			lines = append(lines, dimStyle.Render(l))
		default:
			lines = append(lines, l)
		}
	}
	if len(m.diffLines) > visible {
		lines = append(lines, dimStyle.Render(fmt.Sprintf("  ─ %d-%d of %d lines  ─", m.diffScroll+1, end, len(m.diffLines))))
	}

	overlayWidth := m.width - 8
	if overlayWidth < 60 {
		overlayWidth = 60
	}
	box := panelFocusedStyle.
		Width(overlayWidth).
		BorderTop(true).BorderBottom(true).BorderLeft(true).BorderRight(true).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			panelTitleFocusedStyle.Render(" Diff: "+truncateField(m.diffTitle, overlayWidth-10)+" "),
			strings.Join(lines, "\n"),
			"",
			dimStyle.Render("  j/k: scroll  Esc: close"),
		))

	return m.overlayCenter(base, box)
}

// loadPRDiff fetches the diff/patch for the selected PR and sets showDiff.
func (m *rootModel) loadPRDiff() tea.Cmd {
	rowIdx := m.content.selectedRowIndex()
	results, ok := m.content.rawData.([]model.PullRequestResponse)
	if !ok || rowIdx < 0 || rowIdx >= len(results) {
		return nil
	}
	p := results[rowIdx]
	ctx := m.ctx
	org := m.orgName

	m.statusBar.loading = true
	m.statusBar.loadingMsg = "loading diff"
	return func() tea.Msg {
		filesResp := pr.GetPullRequestFiles(ctx, org, p.RepositoryName(), p.PRNumber)
		return diffLoadedMsg{
			title: fmt.Sprintf("PR #%d · %s", p.PRNumber, p.TitleName),
			lines: pr.ParsePatchLines(filesResp),
		}
	}
}

func (m rootModel) renderSidebar() string {
	panelOH := 3 // 1 title line + 2 border lines
	availH := m.height - 1

	effectiveSidebarW := m.effectiveSidebarWidth()

	cmdBodyH := len(commands)
	cmdOuterH := cmdBodyH + panelOH
	repoBodyH := availH - cmdOuterH - panelOH
	if repoBodyH < 3 {
		repoBodyH = 3
	}

	repoPanel := renderBorderedPanel(
		m.repoSelector.title(),
		m.repoSelector.view(),
		effectiveSidebarW, repoBodyH,
		m.focus == focusRepoSelector,
	)

	cmdPanel := renderBorderedPanel(
		"Commands",
		m.sidebar.view(),
		effectiveSidebarW, cmdBodyH,
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
				if m.content.filter != "" && !m.content.filtering {
					contentTitle += fmt.Sprintf(" [/%s]", m.content.filter)
					contentTitle += fmt.Sprintf(" (%d/%d)", filtered, total)
				} else if m.content.filter != "" {
					contentTitle += fmt.Sprintf(" (%d/%d)", filtered, total)
				} else {
					contentTitle += fmt.Sprintf(" (%d)", total)
				}
			}
			if m.content.sortColumn >= 0 && m.content.sortColumn < len(cmd.columns) {
				arrow := "▲"
				if !m.content.sortAscending {
					arrow = "▼"
				}
				contentTitle += fmt.Sprintf(" %s%s", arrow, cmd.columns[m.content.sortColumn])
			}
		}
	}

	contentPanel := renderBorderedPanel(
		contentTitle,
		m.content.view(),
		m.content.width, m.content.height,
		m.focus == focusContent,
	)

	if m.detail.visible && !m.narrowMode {
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

func (m rootModel) renderNarrowDetailOverlay(base string) string {
	detailTitle := m.detail.title
	if detailTitle == "" {
		detailTitle = "Detail"
	}
	detailPanel := renderBorderedPanel(
		detailTitle,
		m.detail.view(),
		m.detail.width, m.detail.height,
		true,
	)
	return m.overlayCenter(base, detailPanel)
}

// -- help --

func (m rootModel) renderHelpOverlay(base string) string {
	content := m.fullHelpView()
	overlayWidth := 64
	if m.width-8 < overlayWidth {
		overlayWidth = m.width - 8
	}
	box := panelFocusedStyle.
		Width(overlayWidth).
		BorderTop(true).BorderBottom(true).BorderLeft(true).BorderRight(true).
		Render(content)
	return lipgloss.Place(m.width, m.height-1, lipgloss.Center, lipgloss.Center, box)
}

func (m rootModel) fullHelpView() string {
	var b strings.Builder

	b.WriteString(helpOverlayTitleStyle.Render("sgh TUI — Keyboard Shortcuts"))
	b.WriteString("\n")
	b.WriteString(separatorStyle.Render(strings.Repeat("─", 44)))
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
			{"ctrl+m", "Merge PR"},
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
		b.WriteString(helpOverlaySectionStyle.Render(sec.title))
		b.WriteString("\n")
		for _, bind := range sec.binds {
			b.WriteString(fmt.Sprintf("  %s %s\n",
				helpOverlayKeyStyle.Render(bind.key),
				helpOverlayDescStyle.Render(bind.desc),
			))
		}
	}

	b.WriteString("\n")
	b.WriteString(cachedStyle.Render("Press any key to close"))
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
