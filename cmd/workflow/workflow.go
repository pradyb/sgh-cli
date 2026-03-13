// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package workflow

import (
	"fmt"
	"time"

	"github.com/MakeNowJust/heredoc"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/prady-lab/sgh-cli/pkg/ui"
	"github.com/prady-lab/sgh-cli/pkg/workflow"
	"github.com/prady-lab/sgh-cli/utils"
)

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

			Quick Filters (list command):
			  --running   Show only in-progress runs
			  --queued    Show only queued runs
			  --failed    Show only failed runs
			  --status    Filter by any status: completed, in_progress, queued, failure, success, cancelled

			Live Monitoring (view command):
			  --watch       Poll every 10s and refresh until the run completes
			  --interval N  Set the polling interval in seconds (default: 10)
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
	return workflowCmd
}

var (
	repoNames        []string
	excludeRepoNames []string
	status           string
	branch           string
	lastCount        int
	running          bool
	queued           bool
	failed           bool
	sortBy           string
	workflowName     string
)

func ListCommand(ctx *context.Context) *cobra.Command {
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

var (
	repoName string
	runID    int
)

var (
	watch         bool
	watchInterval int
)

func ViewCommand(ctx *context.Context) *cobra.Command {
	viewCmd := &cobra.Command{
		Use:   "view",
		Short: "View details of a workflow run including jobs and steps",
		Long: `View detailed information about a specific GitHub Actions workflow run, including all jobs and their step-level status.
Use --watch to poll for updates until the run completes.
If --run is omitted, automatically picks the latest in-progress or most recent run.`,
		Aliases: []string{"detail", "info"},
		Example: heredoc.Doc(`
			$ sgh workflow view --org sample-org -r sample-repo1
			$ sgh workflow view --org sample-org -r sample-repo1 --run 123456789
			$ sgh workflow view --org sample-org -r sample-repo1 --watch
			$ sgh workflow view --org sample-org -r sample-repo1 --run 123456789 --watch --interval 5
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
				fmt.Printf("  Using latest workflow run: %d\n", effectiveRunID)
			}

			req := workflow.WorkflowRunRequest{
				OrgName:  orgName,
				RepoName: resolvedRepo,
				RunID:    effectiveRunID,
			}

			detail := workflow.GetWorkflowRunDetail(ctx, req)

			if !watch || !detail.IsInProgress() {
				ui.PrintWorkflowRunDetail(detail)
				return
			}

			runWatchLoop(ctx, req, detail)
		},
	}

	viewCmd.Flags().StringVarP(&repoName, "repository", "r", "", "repository name")
	viewCmd.Flags().IntVarP(&runID, "run", "R", 0, "workflow run ID (defaults to latest in-progress or most recent run)")
	viewCmd.Flags().BoolVarP(&watch, "watch", "W", false, "poll for updates until the workflow run completes")
	viewCmd.Flags().IntVar(&watchInterval, "interval", 10, "polling interval in seconds when using --watch")
	viewCmd.MarkFlagRequired("repository")

	return viewCmd
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

func runWatchLoop(ctx *context.Context, req workflow.WorkflowRunRequest, initial model.WorkflowRunDetail) {
	interval := time.Duration(watchInterval) * time.Second
	m := newWatchModel(ctx, req, interval, initial)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		logger.Glog.Error().Err(err).Msg("Watch mode error")
		return
	}
	if wm, ok := finalModel.(watchModel); ok && wm.detail.Run.ID != 0 {
		ui.PrintWorkflowRunDetail(wm.detail)
	}
}

func rerunCommand(ctx *context.Context) *cobra.Command {
	rerunCmd := &cobra.Command{
		Use:   "rerun",
		Short: "Rerun a workflow run",
		Long:  `Rerun a specific GitHub Actions workflow run in a repository.`,
		Example: heredoc.Doc(`
			$ sgh workflow rerun --org sample-org -r sample-repo1 --run 123456789
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
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
			response := workflow.RerunWorkflowRun(ctx, req)
			if response.ErrorMessage != "" {
				logger.Glog.Error().Msg(response.ErrorMessage)
			} else {
				ui.PrintWorkflowRuns([]model.WorkflowRun{response}, "", false)
			}
		},
	}

	rerunCmd.Flags().StringVarP(&repoName, "repository", "r", "", "repository name")
	rerunCmd.Flags().IntVarP(&runID, "run", "R", 0, "workflow run ID")
	rerunCmd.MarkFlagRequired("repository")
	rerunCmd.MarkFlagRequired("run")

	return rerunCmd
}

func cancelCommand(ctx *context.Context) *cobra.Command {
	cancelCmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel a workflow run",
		Long:  `Cancel a specific GitHub Actions workflow run in a repository.`,
		Example: heredoc.Doc(`
			$ sgh workflow cancel --org sample-org -r sample-repo1 --run 123456789
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
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
			} else {
				ui.PrintWorkflowRuns([]model.WorkflowRun{response}, "", false)
			}
		},
	}

	cancelCmd.Flags().StringVarP(&repoName, "repository", "r", "", "repository name")
	cancelCmd.Flags().IntVarP(&runID, "run", "R", 0, "workflow run ID")
	cancelCmd.MarkFlagRequired("repository")
	cancelCmd.MarkFlagRequired("run")

	return cancelCmd
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

	dispatchCmd := &cobra.Command{
		Use:     "dispatch",
		Aliases: []string{"wfd"},
		Short:   "Trigger a workflow_dispatch event across repositories",
		Long: heredoc.Doc(`
			Trigger a workflow_dispatch event for a named workflow file across one or more
			repositories in the organization.

			The workflow must have a 'workflow_dispatch' trigger defined in its YAML file.
			Use --input key=value to pass input parameters to the workflow.
		`),
		Example: heredoc.Doc(`
			$ sgh workflow dispatch --org my-org --workflow deploy.yml --ref main
			$ sgh workflow dispatch --org my-org -r app1 -r app2 --workflow build.yml --ref develop
			$ sgh workflow dispatch --org my-org --workflow release.yml --ref main --input env=production --input dry_run=false
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")

			inputs := make(map[string]string)
			for _, pair := range inputPairs {
				for i, ch := range pair {
					if ch == '=' {
						inputs[pair[:i]] = pair[i+1:]
						break
					}
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
			for _, r := range results {
				if r.ErrorMessage != "" {
					logger.Glog.Error().Str("repo", r.RepositoryName).Msg(r.ErrorMessage)
					fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ %s: %s\n", r.RepositoryName, r.ErrorMessage)
				} else {
					fmt.Printf("  ✓ %s: workflow '%s' dispatched on '%s'\n", r.RepositoryName, r.WorkflowID, r.Ref)
				}
			}
		},
	}

	dispatchCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names to include")
	dispatchCmd.Flags().StringArrayVarP(&excludeRepoNames, "exclude-repository", "e", []string{}, "repository names to exclude")
	dispatchCmd.Flags().StringVarP(&workflowID, "workflow", "W", "", "workflow filename or ID (e.g. deploy.yml)")
	dispatchCmd.Flags().StringVarP(&ref, "ref", "f", "", "branch or tag to run the workflow on")
	dispatchCmd.Flags().StringArrayVarP(&inputPairs, "input", "i", []string{}, "workflow input as key=value (repeatable)")

	dispatchCmd.MarkFlagRequired("workflow")
	dispatchCmd.MarkFlagRequired("ref")

	return dispatchCmd
}
