// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package workflow

import (
	"bufio"
	stdctx "context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/pradyb/sgh-cli/internal/model"
	"github.com/pradyb/sgh-cli/pkg/context"
	"github.com/pradyb/sgh-cli/pkg/logger"
	"github.com/pradyb/sgh-cli/pkg/ui"
	"github.com/pradyb/sgh-cli/pkg/workflow"
	"github.com/pradyb/sgh-cli/utils"
)

func repoCompletionFn(ctx *context.Context) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		orgName, _ := cmd.Root().PersistentFlags().GetString("org")
		return ctx.Config.RepositoriesNames(orgName), cobra.ShellCompDirectiveNoFileComp
	}
}

func NewWorkflowCommand(ctx *context.Context) *cobra.Command {
	workflowCmd := &cobra.Command{
		Use:     "workflow <command>",
		Aliases: []string{"wf"},
		Short:   "Manage GitHub Actions workflow runs across repositories",
		Long: heredoc.Doc(`
			Manage GitHub Actions workflow runs across repositories in your organization.

			Available Operations:
			  list      List workflow runs across all or selected repositories
			  view      View detailed run info with jobs and steps (supports --watch for live polling)
			  rerun     Re-trigger a specific workflow run
			  cancel    Cancel an in-progress workflow run
			  dispatch  Trigger a workflow_dispatch event across repositories
			  approve   Approve or reject workflow runs waiting on environment approval gates

			Quick Filters (list command):
			  --running   Show only in-progress runs
			  --queued    Show only queued runs
			  --failed    Show only failed runs
			  --status    Filter by any status: completed, in_progress, queued, failure, success, cancelled

			Live Monitoring (view command):
			  --watch          Poll every 10s and refresh until the run completes
			  --interval N     Set the polling interval in seconds (default: 10)
			  --watch --json   Stream each job/step status transition as NDJSON instead
		`),
		Example: heredoc.Doc(`
			List all workflow runs:
			  $ sgh workflow list --org my-org

			List only running workflows:
			  $ sgh workflow list --org my-org --running

			List failed workflows on a specific branch:
			  $ sgh workflow list --org my-org --failed --branch main

			View run details with jobs and steps:
			  $ sgh workflow view --org my-org -r my-app --run 123456789

			Watch a run until it completes:
			  $ sgh workflow view --org my-org -r my-app --run 123456789 --watch

			Rerun a failed workflow:
			  $ sgh workflow rerun --org my-org -r my-app --run 123456789

			Cancel a running workflow:
			  $ sgh workflow cancel --org my-org -r my-app --run 123456789

			Trigger a workflow across all repos:
			  $ sgh workflow dispatch --org my-org --workflow deploy.yml --ref main
		`),
	}

	workflowCmd.AddCommand(ListCommand(ctx))
	workflowCmd.AddCommand(ViewCommand(ctx))
	workflowCmd.AddCommand(rerunCommand(ctx))
	workflowCmd.AddCommand(cancelCommand(ctx))
	workflowCmd.AddCommand(dispatchCommand(ctx))
	workflowCmd.AddCommand(approveCommand(ctx))
	return workflowCmd
}

func ListCommand(ctx *context.Context) *cobra.Command {
	var repoNames []string
	var excludeRepoNames []string
	var status string
	var branch string
	var lastCount int
	var running bool
	var queued bool
	var failed bool
	var sortBy string
	var workflowName string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List workflow runs across repositories",
		Long: `List GitHub Actions workflow runs for given repos or all the selected repos in the given org/owner.
Supports filtering by status and branch name. Use shorthand flags for common filters:
  --running   only in-progress runs
  --queued    only queued/waiting runs
  --failed    only failed runs`,
		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			$ sgh workflow list --org sample-org
			$ sgh workflow list --org sample-org --running
			$ sgh workflow list --org sample-org --queued
			$ sgh workflow list --org sample-org --failed
			$ sgh workflow list --org sample-org --status failure
			$ sgh workflow list --org sample-org --branch main --last 5
			$ sgh workflow list --org sample-org --workflow "CI Build"
			$ sgh workflow list --org sample-org -r sample-repo1 -r sample-repo2 --status in_progress
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")

			effectiveStatus := status
			if running {
				effectiveStatus = "in_progress"
			} else if queued {
				effectiveStatus = "queued"
			} else if failed {
				effectiveStatus = "failure"
			}

			req := workflow.WorkflowListRequest{
				OrgName:          orgName,
				RepoNames:        repoNames,
				ExcludeRepoNames: excludeRepoNames,
				Branch:           branch,
				Status:           effectiveStatus,
				Count:            lastCount,
				WorkflowName:     workflowName,
			}
			responses := workflow.ListWorkflowRuns(ctx, req)
			ui.SortWorkflowRuns(responses, sortBy)
			if ctx.Limit > 0 && len(responses) > ctx.Limit {
				responses = responses[:ctx.Limit]
			}
			if ctx.JSON {
				ui.PrintJSON(responses)
				return
			}
			ui.PrintWorkflowRuns(responses, sortBy, ctx.Compact)
		},
	}

	listCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names")
	listCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "repository names to exclude")
	listCmd.RegisterFlagCompletionFunc("repository", repoCompletionFn(ctx))
	listCmd.Flags().StringVarP(&status, "status", "s", "", "filter by status: completed, in_progress, queued, failure, success, cancelled")
	listCmd.Flags().BoolVar(&running, "running", false, "show only in-progress workflow runs")
	listCmd.Flags().BoolVar(&queued, "queued", false, "show only queued workflow runs")
	listCmd.Flags().BoolVar(&failed, "failed", false, "show only failed workflow runs")
	listCmd.Flags().StringVarP(&branch, "branch", "b", "", "filter by `branch` name")
	listCmd.Flags().StringVarP(&workflowName, "workflow", "n", "", "filter by `workflow` name (partial match)")
	listCmd.Flags().IntVarP(&lastCount, "last", "l", 10, "number of workflow runs to fetch per repository")
	listCmd.Flags().StringVar(&sortBy, "sort", "", "sort results by: repo, name, status, created")
	listCmd.MarkFlagsMutuallyExclusive("status", "running", "queued", "failed")

	return listCmd
}

func ViewCommand(ctx *context.Context) *cobra.Command {
	var repoName string
	var runID int
	var watch bool
	var watchInterval int
	viewCmd := &cobra.Command{
		Use:   "view",
		Short: "View details of a workflow run including jobs and steps",
		Long: `View detailed information about a specific GitHub Actions workflow run, including all jobs and their step-level status.
Use --watch to poll for updates until the run completes. --watch --json streams each job/step
status transition as NDJSON instead of the interactive view, for scripting or logging.
If --run is omitted, automatically picks the latest in-progress or most recent run.`,
		Aliases: []string{"detail", "info"},
		Example: heredoc.Doc(`
			$ sgh workflow view --org sample-org -r sample-repo1
			$ sgh workflow view --org sample-org -r sample-repo1 --run 123456789
			$ sgh workflow view --org sample-org -r sample-repo1 --watch
			$ sgh workflow view --org sample-org -r sample-repo1 --run 123456789 --watch --interval 5
			$ sgh workflow view --org sample-org -r sample-repo1 --watch --json
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			resolvedNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, []string{repoName})
			if len(resolvedNames) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ repository not found: %s\n", repoName)
				return
			}
			resolvedRepo := resolvedNames[0]

			effectiveRunID := runID
			if effectiveRunID == 0 {
				resolved, err := workflow.GetLatestRunID(ctx, orgName, resolvedRepo)
				if err != nil {
					logger.Glog.Error().Err(err).Msg("Could not resolve latest workflow run")
					return
				}
				effectiveRunID = resolved
				// Informational, not data: stderr so it never lands in --json/--output output.
				fmt.Fprintf(cmd.ErrOrStderr(), "  Using latest workflow run: %d\n", effectiveRunID)
			}

			req := workflow.WorkflowRunRequest{
				OrgName:  orgName,
				RepoName: resolvedRepo,
				RunID:    effectiveRunID,
			}

			detail := workflow.GetWorkflowRunDetail(ctx, req)

			if watch {
				detail = followRun(cmd, ctx, req, detail, watchInterval, 0)
				if ctx.JSON && detail.ErrorMessage != "" {
					ctx.HasError = true
				}
				return
			}

			if ctx.JSON {
				ui.PrintJSON(detail)
				if detail.ErrorMessage != "" {
					ctx.HasError = true
				}
				return
			}

			ui.PrintWorkflowRunDetail(detail)
		},
	}

	viewCmd.Flags().StringVarP(&repoName, "repository", "r", "", "repository name")
	viewCmd.Flags().IntVarP(&runID, "run", "R", 0, "workflow run ID (defaults to latest in-progress or most recent run)")
	viewCmd.Flags().BoolVarP(&watch, "watch", "W", false, "poll for updates until the workflow run completes")
	viewCmd.Flags().IntVar(&watchInterval, "interval", 10, "polling interval in seconds when using --watch")
	viewCmd.MarkFlagRequired("repository")

	return viewCmd
}

// followRun follows a run from its already-fetched initial detail until it completes: the
// interactive view for a human, NDJSON events under --json. Shared by view --watch and
// dispatch --watch. A positive timeout gives up on a run still in progress after that long,
// reporting it on stderr and setting HasError. Returns the last detail fetched (only
// meaningful under --json).
func followRun(cmd *cobra.Command, ctx *context.Context, req workflow.WorkflowRunRequest, initial model.WorkflowRunDetail, interval int, timeout time.Duration) model.WorkflowRunDetail {
	wctx, cancel := watchContext(cmd, timeout)
	defer cancel()

	detail := initial
	switch {
	case ctx.JSON:
		detail = runWatchLoopJSON(wctx, ctx, req, initial, interval)
	case !initial.IsInProgress():
		ui.PrintWorkflowRunDetail(initial)
		return initial
	default:
		if err := runWatchLoop(wctx, ctx, req, initial, interval); err != nil {
			// The run is untouched and still going; don't let this exit 0 as if it had been watched.
			fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ %s: could not open the live view (%v); run %d is still in progress: %s\n",
				req.RepoName, err, req.RunID, initial.Run.HTMLUrl)
			ctx.HasError = true
			return initial
		}
		detail.Run.Status = "in_progress" // only the deadline check below needs this
	}
	if timeout > 0 && wctx.Err() == stdctx.DeadlineExceeded && detail.IsInProgress() {
		fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ %s: %s\n", req.RepoName, workflow.ErrWatchTimeout)
		ctx.HasError = true
	}
	return detail
}

// --- Bubble Tea watch model ---

type (
	watchTickMsg struct{}
	watchDataMsg struct{ detail model.WorkflowRunDetail }
)

type watchModel struct {
	ctx      *context.Context
	req      workflow.WorkflowRunRequest
	interval time.Duration
	detail   model.WorkflowRunDetail
	loading  bool
	done     bool
}

func newWatchModel(ctx *context.Context, req workflow.WorkflowRunRequest, interval time.Duration, initial model.WorkflowRunDetail) watchModel {
	return watchModel{ctx: ctx, req: req, interval: interval, detail: initial, loading: false}
}

func (m watchModel) Init() tea.Cmd {
	return tea.Tick(m.interval, func(time.Time) tea.Msg { return watchTickMsg{} })
}

func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case watchDataMsg:
		m.detail = msg.detail
		m.loading = false
		if !m.detail.IsInProgress() {
			m.done = true
			return m, tea.Quit
		}
		return m, tea.Tick(m.interval, func(time.Time) tea.Msg { return watchTickMsg{} })

	case watchTickMsg:
		m.loading = true
		return m, m.fetchDetail

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.done = true
			return m, tea.Quit
		case "r":
			m.loading = true
			return m, m.fetchDetail
		}
	}
	return m, nil
}

func (m watchModel) View() string {
	if m.loading && m.detail.Run.ID == 0 {
		return "\n  Fetching workflow run details...\n"
	}
	rendered := ui.RenderWorkflowRunDetail(m.detail)
	if !m.done && m.detail.IsInProgress() {
		hint := lipgloss.NewStyle().Foreground(ui.Dimmed).Italic(true).PaddingLeft(2)
		rendered += hint.Render(fmt.Sprintf("Watching — refreshes every %ds · r to refresh · q to quit", int(m.interval.Seconds())))
		rendered += "\n"
	}
	return rendered
}

func (m watchModel) fetchDetail() tea.Msg {
	detail := workflow.GetWorkflowRunDetail(m.ctx, m.req)
	return watchDataMsg{detail: detail}
}

// runWatchProgram runs the interactive view; a var so tests can fail it without needing a terminal.
var runWatchProgram = func(p *tea.Program) (tea.Model, error) { return p.Run() }

// runWatchLoop runs the interactive live view. It returns an error only when the view itself
// could not run (e.g. no terminal); a cancelled wctx (timeout, Ctrl-C) is not an error.
func runWatchLoop(wctx stdctx.Context, ctx *context.Context, req workflow.WorkflowRunRequest, initial model.WorkflowRunDetail, watchInterval int) error {
	interval := time.Duration(watchInterval) * time.Second
	m := newWatchModel(ctx, req, interval, initial)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(wctx))
	finalModel, err := runWatchProgram(p)
	if err != nil {
		if wctx.Err() != nil {
			return nil
		}
		return err
	}
	if wm, ok := finalModel.(watchModel); ok && wm.detail.Run.ID != 0 {
		ui.PrintWorkflowRunDetail(wm.detail)
	}
	return nil
}

// viewWatchSleep is a var so tests can force polling not to really wait.
var viewWatchSleep = time.Sleep

// runWatchLoopJSON is `view --watch --json`'s loop: instead of the interactive bubbletea view,
// it streams each job/step status transition as a compact single-line JSON object (NDJSON),
// terminated by a final run_done event once the run completes.
func runWatchLoopJSON(wctx stdctx.Context, ctx *context.Context, req workflow.WorkflowRunRequest, initial model.WorkflowRunDetail, watchInterval int) model.WorkflowRunDetail {
	notify := func(e workflow.ViewWatchEvent) {
		b, _ := json.Marshal(e)
		fmt.Println(string(b))
	}
	return workflow.WatchViewRun(wctx, ctx, req, initial, workflow.WatchViewOptions{
		Interval: time.Duration(watchInterval) * time.Second,
		Notify:   notify,
		Sleep:    viewWatchSleep,
	})
}

func rerunCommand(ctx *context.Context) *cobra.Command {
	var repoName string
	var runID int
	var watch bool
	var watchInterval int
	var watchTimeout time.Duration

	rerunCmd := &cobra.Command{
		Use:   "rerun",
		Short: "Rerun a workflow run",
		Long: heredoc.Doc(`
			Rerun a specific GitHub Actions workflow run in a repository.

			With --watch, follow the rerun until it completes, like 'workflow view --watch'
			(--watch --json streams NDJSON). A rerun keeps the run ID, so the watch starts once
			the new attempt is visible; --timeout gives up on it after that long.
		`),
		Example: heredoc.Doc(`
			$ sgh workflow rerun --org sample-org -r sample-repo1 --run 123456789
			$ sgh workflow rerun --org sample-org -r sample-repo1 --run 123456789 --watch --timeout 30m
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if (watchTimeout != 0 || cmd.Flags().Changed("interval")) && !watch {
				fmt.Fprintln(cmd.ErrOrStderr(), "  ✗ --interval and --timeout require --watch")
				ctx.HasError = true
				return
			}
			if watch && !ValidateFollowInterval(cmd, ctx, watchInterval) {
				return
			}
			resolvedNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, []string{repoName})
			if len(resolvedNames) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ repository not found: %s\n", repoName)
				return
			}
			resolvedRepo := resolvedNames[0]
			if ctx.DryRun {
				ui.PrintDryRunBanner()
				ui.PrintDryRunActions("Rerun Workflow", orgName, []string{resolvedRepo}, map[string]string{
					"Run ID": fmt.Sprintf("%d", runID),
				})
				return
			}
			req := workflow.WorkflowRunRequest{
				OrgName:  orgName,
				RepoName: resolvedRepo,
				RunID:    runID,
			}
			// Read before rerunning: a rerun keeps the run ID, and the attempt number is how the
			// watch tells the new attempt from the old, already-completed one.
			var prevAttempt int
			if watch {
				prevAttempt = workflow.GetWorkflowRunDetail(ctx, req).Run.RunAttempt
			}
			response := workflow.RerunWorkflowRun(ctx, req)
			if response.ErrorMessage != "" {
				logger.Glog.Error().Msg(response.ErrorMessage)
				if watch {
					ctx.HasError = true
					if ctx.JSON {
						EmitFollowEvent("rerun_error", resolvedRepo, runID, "", response.ErrorMessage)
					}
				}
				return
			}
			switch {
			case watch && ctx.JSON:
				EmitFollowEvent("rerun_requested", resolvedRepo, runID, "", "")
			default:
				ui.PrintWorkflowRuns([]model.WorkflowRun{response}, "", false)
			}
			if watch {
				FollowRerun(cmd, ctx, req, prevAttempt, FollowOptions{Interval: watchInterval, Timeout: watchTimeout})
			}
		},
	}

	rerunCmd.Flags().StringVarP(&repoName, "repository", "r", "", "repository name")
	rerunCmd.Flags().IntVarP(&runID, "run", "R", 0, "workflow run ID")
	rerunCmd.Flags().BoolVarP(&watch, "watch", "W", false, "follow the rerun until it completes")
	rerunCmd.Flags().IntVar(&watchInterval, "interval", 10, "polling interval in seconds when using --watch (minimum 5)")
	rerunCmd.Flags().DurationVar(&watchTimeout, "timeout", 0, "give up watching after this long when using --watch, e.g. 30m (0 = no timeout)")
	rerunCmd.MarkFlagRequired("repository")
	rerunCmd.MarkFlagRequired("run")

	return rerunCmd
}

func cancelCommand(ctx *context.Context) *cobra.Command {
	var repoName string
	var runID int
	var wait bool
	var waitInterval int
	var waitTimeout time.Duration

	cancelCmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel a workflow run",
		Long: heredoc.Doc(`
			Cancel a specific GitHub Actions workflow run in a repository.

			GitHub accepts the request immediately and cancels asynchronously, so a successful
			cancel only means "cancel_requested". With --wait, poll until the run has finished and
			report how it ended (it may have completed before the cancel took effect).
		`),
		Example: heredoc.Doc(`
			$ sgh workflow cancel --org sample-org -r sample-repo1 --run 123456789
			$ sgh workflow cancel --org sample-org -r sample-repo1 --run 123456789 --wait
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if (waitTimeout != 0 || cmd.Flags().Changed("interval")) && !wait {
				fmt.Fprintln(cmd.ErrOrStderr(), "  ✗ --interval and --timeout require --wait")
				ctx.HasError = true
				return
			}
			if wait && !ValidateFollowInterval(cmd, ctx, waitInterval) {
				return
			}
			resolvedNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, []string{repoName})
			if len(resolvedNames) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ repository not found: %s\n", repoName)
				return
			}
			resolvedRepo := resolvedNames[0]
			if ctx.DryRun {
				ui.PrintDryRunBanner()
				ui.PrintDryRunActions("Cancel Workflow", orgName, []string{resolvedRepo}, map[string]string{
					"Run ID": fmt.Sprintf("%d", runID),
				})
				return
			}
			req := workflow.WorkflowRunRequest{
				OrgName:  orgName,
				RepoName: resolvedRepo,
				RunID:    runID,
			}
			response := workflow.CancelWorkflowRun(ctx, req)
			if response.ErrorMessage != "" {
				logger.Glog.Error().Msg(response.ErrorMessage)
				if wait {
					ctx.HasError = true
					if ctx.JSON {
						EmitFollowEvent("cancel_error", resolvedRepo, runID, "", response.ErrorMessage)
					}
				}
				return
			}
			if wait && ctx.JSON {
				EmitFollowEvent("cancel_requested", resolvedRepo, runID, "", "")
			} else {
				ui.PrintWorkflowRuns([]model.WorkflowRun{response}, "", false)
			}
			if wait {
				waitForCancel(cmd, ctx, req, FollowOptions{Interval: waitInterval, Timeout: waitTimeout})
			}
		},
	}

	cancelCmd.Flags().StringVarP(&repoName, "repository", "r", "", "repository name")
	cancelCmd.Flags().IntVarP(&runID, "run", "R", 0, "workflow run ID")
	cancelCmd.Flags().BoolVar(&wait, "wait", false, "wait until the run has finished and report how it ended")
	cancelCmd.Flags().IntVar(&waitInterval, "interval", 10, "polling interval in seconds when using --wait (minimum 5)")
	cancelCmd.Flags().DurationVar(&waitTimeout, "timeout", 0, "give up waiting after this long when using --wait, e.g. 5m (0 = no timeout)")
	cancelCmd.MarkFlagRequired("repository")
	cancelCmd.MarkFlagRequired("run")

	return cancelCmd
}

// waitForCancel polls a run whose cancellation was requested until it has finished, then reports
// its conclusion. A run can finish for another reason before the cancel takes effect.
func waitForCancel(cmd *cobra.Command, ctx *context.Context, req workflow.WorkflowRunRequest, o FollowOptions) {
	wctx, cancel := watchContext(cmd, o.Timeout)
	defer cancel()
	detail := workflow.WatchViewRun(wctx, ctx, req, workflow.GetWorkflowRunDetail(ctx, req), workflow.WatchViewOptions{
		Interval: time.Duration(o.Interval) * time.Second,
		Sleep:    viewWatchSleep,
	})
	switch {
	case detail.ErrorMessage != "":
		ctx.HasError = true
		fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ %s: %s\n", req.RepoName, detail.ErrorMessage)
	case detail.IsInProgress():
		ctx.HasError = true
		fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ %s: run %d was still in progress when waiting stopped (%s)\n", req.RepoName, req.RunID, wctxReason(wctx))
	case ctx.JSON:
		emitFollowEvent(followEvent{Kind: "cancel_done", Repository: req.RepoName, RunID: req.RunID, Conclusion: detail.Run.Conclusion})
	case detail.Run.Conclusion == "cancelled":
		fmt.Printf("  ✓ %s: run %d cancelled\n", req.RepoName, req.RunID)
	default:
		fmt.Printf("  ! %s: run %d finished as '%s' (it completed before the cancel took effect)\n", req.RepoName, req.RunID, detail.Run.Conclusion)
	}
}

func wctxReason(c stdctx.Context) string {
	if c.Err() == stdctx.DeadlineExceeded {
		return "timed out"
	}
	return "interrupted"
}

func DispatchCommand(ctx *context.Context) *cobra.Command {
	return dispatchCommand(ctx)
}

func dispatchCommand(ctx *context.Context) *cobra.Command {
	var repoNames []string
	var excludeRepoNames []string
	var workflowID string
	var ref string
	var inputPairs []string
	var watch bool
	var watchInterval int
	var watchTimeout time.Duration
	var approve bool
	var environments []string
	var yes bool

	dispatchCmd := &cobra.Command{
		Use:     "dispatch",
		Aliases: []string{"wfd"},
		Short:   "Trigger a workflow_dispatch event across repositories",
		Long: heredoc.Doc(`
			Trigger a workflow_dispatch event for a named workflow file across one or more
			repositories in the organization.

			The workflow must have a 'workflow_dispatch' trigger defined in its YAML file.
			Use --input key=value to pass input parameters to the workflow.

			The run ID and URL of each dispatched run are shown when GitHub returns them (always,
			except on older GitHub Enterprise Server). --watch then follows each returned run in turn,
			one repository at a time, like 'workflow view --watch'; with --json it streams NDJSON.
			--timeout gives up on a run after that long (per run).

			With --watch, --approve also decides the run's environment approval gates as they appear,
			like 'workflow approve --watch' (it asks for confirmation unless --yes is given, and
			--yes requires --environment so an unattended run never approves a gate that wasn't
			named). All of this is checked before anything is dispatched.
		`),
		Example: heredoc.Doc(`
			$ sgh workflow dispatch --org my-org --workflow deploy.yml --ref main
			$ sgh workflow dispatch --org my-org -r app1 -r app2 --workflow build.yml --ref develop
			$ sgh workflow dispatch --org my-org --workflow release.yml --ref main --input env=production --input dry_run=false
			$ sgh workflow dispatch --org my-org -r app1 --workflow deploy.yml --ref main --watch
			$ sgh workflow dispatch --org my-org --workflow build.yml --ref main --json
			$ sgh workflow dispatch --org my-org -r app1 --workflow deploy.yml --ref main --watch --timeout 30m
			$ sgh workflow dispatch --org my-org -r app1 --workflow deploy.yml --ref main --watch --approve --yes --environment staging
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")

			inputs, err := parseInputPairs(inputPairs)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ %v\n", err)
				ctx.HasError = true
				return
			}

			if !watch && (approve || watchTimeout != 0) {
				fmt.Fprintln(cmd.ErrOrStderr(), "  ✗ --approve and --timeout require --watch")
				ctx.HasError = true
				return
			}
			if !approve && (yes || len(environments) > 0) {
				fmt.Fprintln(cmd.ErrOrStderr(), "  ✗ --yes and --environment require --approve")
				ctx.HasError = true
				return
			}
			if watch && watchInterval < 5 {
				fmt.Fprintln(cmd.ErrOrStderr(), "  ✗ --interval must be at least 5 seconds when using --watch")
				ctx.HasError = true
				return
			}
			watchOpt := watchOptions{
				orgName: orgName, environments: environments, yes: yes,
				interval: watchInterval, timeout: watchTimeout, verb: "approve",
			}
			if approve {
				if ctx.JSON && !yes {
					fmt.Fprintln(cmd.ErrOrStderr(), "  ✗ --json cannot prompt for confirmation; pass --yes")
					ctx.HasError = true
					return
				}
				if !ctx.DryRun && !validateApproveWatch(cmd, ctx, watchOpt) {
					return
				}
			}

			if ctx.DryRun {
				ui.PrintDryRunBanner()
				details := map[string]string{
					"Workflow": workflowID,
					"Ref":      ref,
				}
				for k, v := range inputs {
					details["Input: "+k] = v
				}
				ui.PrintDryRunActions("Dispatch Workflow", orgName, repoNames, details)
				return
			}

			req := workflow.WorkflowDispatchRequest{
				OrgName:          orgName,
				RepoNames:        repoNames,
				ExcludeRepoNames: excludeRepoNames,
				WorkflowID:       workflowID,
				Ref:              ref,
				Inputs:           inputs,
			}
			results := workflow.DispatchWorkflow(ctx, req)
			if watch {
				followDispatched(cmd, ctx, results, watchOpt, approve)
				return
			}
			if ctx.JSON {
				ui.PrintJSON(results)
				return
			}
			printDispatchResults(cmd, results)
		},
	}

	dispatchCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names to include")
	dispatchCmd.Flags().StringArrayVarP(&excludeRepoNames, "exclude-repository", "e", []string{}, "repository names to exclude")
	dispatchCmd.Flags().StringVarP(&workflowID, "workflow", "W", "", "workflow filename or ID (e.g. deploy.yml)")
	dispatchCmd.Flags().StringVarP(&ref, "ref", "f", "", "branch or tag to run the workflow on")
	dispatchCmd.Flags().StringArrayVarP(&inputPairs, "input", "i", []string{}, "workflow input as key=value (repeatable)")

	dispatchCmd.Flags().BoolVar(&watch, "watch", false, "follow each dispatched run until it completes, one repository at a time")
	dispatchCmd.Flags().IntVar(&watchInterval, "interval", 10, "polling interval in seconds when using --watch (minimum 5)")
	dispatchCmd.Flags().DurationVar(&watchTimeout, "timeout", 0, "give up on each watched run after this long, e.g. 30m (0 = no timeout)")
	dispatchCmd.Flags().BoolVar(&approve, "approve", false, "with --watch, also decide each run's environment approval gates as they appear")
	dispatchCmd.Flags().StringArrayVar(&environments, "environment", []string{}, "with --approve, only decide this environment's gate (repeatable)")
	dispatchCmd.Flags().BoolVarP(&yes, "yes", "y", false, "with --approve, skip the confirmation prompt (requires --environment)")

	dispatchCmd.MarkFlagRequired("workflow")
	dispatchCmd.MarkFlagRequired("ref")

	return dispatchCmd
}

func printDispatchResults(cmd *cobra.Command, results []workflow.WorkflowDispatchResult) {
	for _, r := range results {
		switch {
		case r.ErrorMessage != "":
			logger.Glog.Error().Str("repo", r.RepositoryName).Msg(r.ErrorMessage)
			fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ %s: %s\n", r.RepositoryName, r.ErrorMessage)
		case r.RunID != 0:
			fmt.Printf("  ✓ %s: workflow '%s' dispatched on '%s' (run %d: %s)\n", r.RepositoryName, r.WorkflowID, r.Ref, r.RunID, r.HTMLURL)
		default:
			fmt.Printf("  ✓ %s: workflow '%s' dispatched on '%s' (run ID not returned by GitHub)\n", r.RepositoryName, r.WorkflowID, r.Ref)
		}
	}
}

// followDispatched follows each dispatched run sequentially: with approve it decides the run's
// gates too (the same code as `workflow approve --watch`), otherwise it only watches. Repos that
// failed to dispatch, or whose dispatch returned no run ID (204), are reported and skipped.
// Stdout stays clean NDJSON under --json. ponytail: sequential, concurrent following needs an
// interleaved-output design.
func followDispatched(cmd *cobra.Command, ctx *context.Context, results []workflow.WorkflowDispatchResult, opt watchOptions, approve bool) {
	emit := emitFollowEvent
	if !ctx.JSON {
		printDispatchResults(cmd, results)
	}
	for _, r := range results {
		switch {
		case r.ErrorMessage != "":
			ctx.HasError = true
			if ctx.JSON {
				emit(followEvent{Kind: "dispatch_error", Repository: r.RepositoryName, Reason: r.ErrorMessage})
			}
		case r.RunID == 0:
			if ctx.JSON {
				emit(followEvent{Kind: "watch_skipped", Repository: r.RepositoryName, Reason: "run ID not returned by GitHub"})
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "  ! %s: not watching, GitHub did not return a run ID\n", r.RepositoryName)
			}
		default:
			if ctx.JSON {
				emit(followEvent{Kind: "dispatched", Repository: r.RepositoryName, RunID: r.RunID})
			}
			if approve {
				o := opt
				o.repoName, o.runID = r.RepositoryName, r.RunID
				runApproveWatch(cmd, ctx, o) // prints its own "Watching <repo> run <id>"
				continue
			}
			if !ctx.JSON {
				fmt.Printf("\n  Watching %s run %d\n", r.RepositoryName, r.RunID)
			}
			req := workflow.WorkflowRunRequest{OrgName: opt.orgName, RepoName: r.RepositoryName, RunID: r.RunID}
			if detail := followRun(cmd, ctx, req, workflow.GetWorkflowRunDetail(ctx, req), opt.interval, opt.timeout); detail.ErrorMessage != "" {
				ctx.HasError = true
			}
		}
	}
}

func approveCommand(ctx *context.Context) *cobra.Command {
	var repoNames []string
	var runID int
	var environments []string
	var reject bool
	var comment string
	var yes bool
	var watch bool
	var interval int
	var timeout time.Duration

	approveCmd := &cobra.Command{
		Use:   "approve",
		Short: "Approve or reject workflow runs waiting on environment approval gates",
		Long: heredoc.Doc(`
			Approve (or reject with --reject) the pending environment deployment gates of a
			workflow run, without opening the Actions UI.

			If --run is omitted, the latest run waiting for approval in each repository is used.
			Only gates you are a required reviewer of can be decided; others are skipped.
			A run with several sequential gates exposes one at a time, so run the command again
			after each approval — or pass --watch to stay attached and decide each new gate as
			it appears, until the run completes, showing each job/step as it starts and finishes
			(--watch --json streams these and the gate events as NDJSON). Asks for confirmation
			unless --yes is given; --watch --yes additionally requires --environment, so an
			unattended watch never approves a gate that didn't exist when it started.
		`),
		Example: heredoc.Doc(`
			$ sgh workflow approve --org sample-org -r sample-repo1
			$ sgh workflow approve --org sample-org -r sample-repo1 --run 123456789 --yes
			$ sgh workflow approve --org sample-org -r sample-repo1 --environment production --comment "ship it"
			$ sgh workflow approve --org sample-org -r sample-repo1 --run 123456789 --reject --comment "not now"
			$ sgh workflow approve --org sample-org -r app1 -r app2 --yes
			$ sgh workflow approve --org sample-org -r sample-repo1 --watch
			$ sgh workflow approve --org sample-org -r sample-repo1 --watch --yes --environment approval-1 --environment approval-2
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if (runID != 0 || watch) && len(repoNames) != 1 {
				flag := "--run"
				if watch {
					flag = "--watch"
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ %s requires exactly one --repository\n", flag)
				ctx.HasError = true
				return
			}
			resolved := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, repoNames)
			if len(resolved) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ repository not found: %s\n", strings.Join(repoNames, ", "))
				return
			}

			if ctx.JSON && !yes && !ctx.DryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), "  ✗ --json cannot prompt for confirmation; pass --yes")
				ctx.HasError = true
				return
			}

			verb := "approve"
			if reject {
				verb = "reject"
			}

			if watch && !ctx.DryRun {
				runApproveWatch(cmd, ctx, watchOptions{
					orgName: orgName, repoName: resolved[0], runID: runID,
					environments: environments, reject: reject, comment: comment,
					yes: yes, interval: interval, timeout: timeout, verb: verb,
				})
				return
			}

			stdin := bufio.NewReader(cmd.InOrStdin())
			confirm := func(r workflow.ApproveResult) bool {
				fmt.Fprintf(cmd.ErrOrStderr(), "  %s %s (run %d) gates [%s]? Type 'yes' to confirm: ",
					strings.ToUpper(verb[:1])+verb[1:], r.Repository, r.RunID, strings.Join(r.Decided, ", "))
				line, _ := stdin.ReadString('\n')
				return strings.EqualFold(strings.TrimSpace(line), "yes")
			}
			if yes {
				confirm = nil
			}

			results := make([]workflow.ApproveResult, 0, len(resolved))
			for _, repo := range resolved {
				results = append(results, workflow.ApproveWorkflowRun(ctx, workflow.ApproveRequest{
					OrgName:      orgName,
					RepoName:     repo,
					RunID:        runID,
					Environments: environments,
					Reject:       reject,
					Comment:      comment,
					Confirm:      confirm,
				}))
			}

			if ctx.JSON {
				ui.PrintJSON(results)
			} else {
				if ctx.DryRun {
					ui.PrintDryRunBanner()
				}
				for _, r := range results {
					printApproveResult(cmd, r, verb)
				}
			}
			for _, r := range results {
				if r.Error != "" {
					ctx.HasError = true
				}
			}
		},
	}

	approveCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names (repeatable)")
	approveCmd.Flags().IntVarP(&runID, "run", "R", 0, "workflow run ID (defaults to the latest run waiting for approval)")
	approveCmd.Flags().StringArrayVarP(&environments, "environment", "e", []string{}, "only decide these environment gates (repeatable)")
	approveCmd.Flags().BoolVar(&reject, "reject", false, "reject instead of approve")
	approveCmd.Flags().StringVar(&comment, "comment", "", "comment recorded with the decision")
	approveCmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	approveCmd.Flags().BoolVarP(&watch, "watch", "W", false, "stay attached and decide each new approval gate as it appears, until the run completes")
	approveCmd.Flags().IntVar(&interval, "interval", 10, "polling interval in seconds when using --watch (minimum 5)")
	approveCmd.Flags().DurationVar(&timeout, "timeout", 0, "give up watching after this long when using --watch, e.g. 30m (0 = no timeout)")
	approveCmd.MarkFlagRequired("repository")

	return approveCmd
}

// isStdinTerminal is a var so tests can force the non-interactive path deterministically
// without needing a real terminal.
var isStdinTerminal = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

// watchSleep defaults to time.Sleep; tests override it so a --watch test needing
// several poll iterations doesn't really wait --interval seconds between each.
var watchSleep = time.Sleep

type watchOptions struct {
	orgName, repoName string
	runID             int
	environments      []string
	reject            bool
	comment           string
	yes               bool
	interval          int
	timeout           time.Duration
	verb              string
}

// validateApproveWatch checks --watch's flag combination, reporting the first problem on stderr
// and setting HasError. dispatch --approve calls it before dispatching, so nothing is sent
// when the watch could not run.
func validateApproveWatch(cmd *cobra.Command, ctx *context.Context, opt watchOptions) bool {
	switch {
	case opt.yes && len(opt.environments) == 0:
		fmt.Fprintln(cmd.ErrOrStderr(), "  ✗ --watch with --yes requires --environment so unseen gates are never approved blindly")
	case opt.interval < 5:
		fmt.Fprintln(cmd.ErrOrStderr(), "  ✗ --interval must be at least 5 seconds when using --watch")
	case !opt.yes && !isStdinTerminal():
		fmt.Fprintln(cmd.ErrOrStderr(), "  ✗ --watch needs to prompt for confirmation, but stdin is not a terminal — pass --yes (with --environment)")
	default:
		return true
	}
	ctx.HasError = true
	return false
}

// runApproveWatch validates --watch's flag combination, wires workflow.WatchApprovals
// to this command's stdin/stdout/stderr, and reports the outcome.
func runApproveWatch(cmd *cobra.Command, ctx *context.Context, opt watchOptions) {
	if !validateApproveWatch(cmd, ctx, opt) {
		return
	}

	stdin := bufio.NewReader(cmd.InOrStdin())
	runID := opt.runID // replaced by the resolved ID on watch_started, which precedes any prompt
	var confirm func(workflow.WatchGate) bool
	if !opt.yes {
		confirm = func(g workflow.WatchGate) bool {
			reviewers := "none"
			if len(g.Reviewers) > 0 {
				reviewers = strings.Join(g.Reviewers, ", ")
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "  New gate on %s (run %d): environment %q (reviewers: %s)\n",
				opt.repoName, runID, g.Environment, reviewers)
			fmt.Fprint(cmd.ErrOrStderr(), "  Approve? Type 'yes' to confirm, anything else to skip: ")
			line, _ := stdin.ReadString('\n')
			return strings.EqualFold(strings.TrimSpace(line), "yes")
		}
	}

	notify := func(e workflow.WatchEvent) {
		if ctx.JSON {
			// One compact object per line (true NDJSON) — ui.PrintJSON pretty-prints
			// across multiple lines, which a streaming consumer can't process incrementally.
			if b, err := json.Marshal(e); err == nil {
				fmt.Println(string(b))
			}
			return
		}
		switch e.Kind {
		case "watch_started":
			runID = e.RunID
			fmt.Printf("  Watching %s run %d   [Ctrl-C to stop]\n", opt.repoName, runID)
		case "job_started":
			fmt.Printf("  %s %s\n", ui.StatusIcon("in_progress"), e.Job)
		case "job_completed":
			fmt.Printf("  %s %s (%s)\n", ui.StatusIcon(e.Conclusion), e.Job, e.Conclusion)
		case "step_started":
			fmt.Printf("    %s %s / %s\n", ui.StatusIcon("in_progress"), e.Job, e.Step)
		case "step_completed":
			fmt.Printf("    %s %s / %s (%s)\n", ui.StatusIcon(e.Conclusion), e.Job, e.Step, e.Conclusion)
		case "gate_decided":
			fmt.Printf("  New gate: %s ... %s\n", e.Environment, e.State)
		case "gate_skipped":
			fmt.Fprintf(cmd.ErrOrStderr(), "  ! %s: skipped (%s)\n", e.Environment, e.Reason)
		case "gate_error":
			fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ %s: %s\n", e.Environment, e.Reason)
		case "run_done":
			fmt.Printf("  Run completed: %s\n", e.Conclusion)
		}
	}

	res, err := workflow.WatchApprovals(cmd.Context(), ctx, workflow.WatchApproveRequest{
		OrgName:      opt.orgName,
		RepoName:     opt.repoName,
		RunID:        opt.runID,
		Environments: opt.environments,
		Reject:       opt.reject,
		Comment:      opt.comment,
		Interval:     time.Duration(opt.interval) * time.Second,
		Timeout:      opt.timeout,
		Yes:          opt.yes,
		Confirm:      confirm,
		Notify:       notify,
		Sleep:        watchSleep,
	})

	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ %s: %s\n", opt.repoName, err)
		ctx.HasError = true
		return
	}
	if res.HadFailures {
		ctx.HasError = true
	}
}

func printApproveResult(cmd *cobra.Command, r workflow.ApproveResult, verb string) {
	switch {
	case r.Error != "":
		fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ %s: %s\n", r.Repository, r.Error)
	case r.Aborted:
		fmt.Printf("  - %s: aborted, nothing sent\n", r.Repository)
	case r.DryRun:
		fmt.Printf("  [dry-run] would %s %s (run %d): %s\n", verb, r.Repository, r.RunID, strings.Join(r.Decided, ", "))
	default:
		fmt.Printf("  ✓ %s (run %d): %s %s\n", r.Repository, r.RunID, r.State, strings.Join(r.Decided, ", "))
	}
	if len(r.Skipped) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "  ! %s: skipped (not a required reviewer): %s\n", r.Repository, strings.Join(r.Skipped, ", "))
	}
}

// parseInputPairs turns repeated --input key=value flags into a map. It splits on the first
// '=' only (values may contain '='), and rejects malformed pairs so a typo never results in a
// workflow being dispatched with the intended input silently missing.
func parseInputPairs(pairs []string) (map[string]string, error) {
	inputs := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		key, value, found := strings.Cut(pair, "=")
		if !found {
			return nil, fmt.Errorf("invalid --input %q: expected key=value", pair)
		}
		if key == "" {
			return nil, fmt.Errorf("invalid --input %q: key must not be empty", pair)
		}
		inputs[key] = value
	}
	return inputs, nil
}
