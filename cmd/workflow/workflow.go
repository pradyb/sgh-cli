package workflow

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/MakeNowJust/heredoc"
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
		Use:   "workflow <command>",
		Short: "Manage GitHub Actions workflow runs across repositories",
		Long: heredoc.Doc(`
			Manage GitHub Actions workflow runs across repositories in your organization.

			Available Operations:
			  list     List workflow runs across all or selected repositories
			  view     View detailed run info with jobs and steps (supports --watch for live polling)
			  rerun    Re-trigger a specific workflow run
			  cancel   Cancel an in-progress workflow run

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
			  $ sgh workflow view --org my-org --repo my-app --run 123456789

			Watch a run until it completes:
			  $ sgh workflow view --org my-org --repo my-app --run 123456789 --watch

			Rerun a failed workflow:
			  $ sgh workflow rerun --org my-org --repo my-app --run 123456789

			Cancel a running workflow:
			  $ sgh workflow cancel --org my-org --repo my-app --run 123456789
		`),
	}

	workflowCmd.AddCommand(listCommand(ctx))
	workflowCmd.AddCommand(viewCommand(ctx))
	workflowCmd.AddCommand(rerunCommand(ctx))
	workflowCmd.AddCommand(cancelCommand(ctx))
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
)

func listCommand(ctx *context.Context) *cobra.Command {
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
			$ sgh workflow list --org sample-org --repo sample-repo1 --repo sample-repo2 --status in_progress
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
			}
			responses := workflow.ListWorkflowRuns(ctx, req)
			ui.PrintWorkflowRuns(responses)
		},
	}

	listCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names")
	listCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "repository names to exclude")
	listCmd.Flags().StringVarP(&status, "status", "s", "", "filter by status: completed, in_progress, queued, failure, success, cancelled")
	listCmd.Flags().BoolVar(&running, "running", false, "show only in-progress workflow runs")
	listCmd.Flags().BoolVar(&queued, "queued", false, "show only queued workflow runs")
	listCmd.Flags().BoolVar(&failed, "failed", false, "show only failed workflow runs")
	listCmd.Flags().StringVarP(&branch, "branch", "B", "", "filter by `branch` name")
	listCmd.Flags().IntVarP(&lastCount, "last", "l", 10, "number of workflow runs to fetch per repository")
	listCmd.MarkFlagsMutuallyExclusive("status", "running", "queued", "failed")
	listCmd.MarkPersistentFlagRequired("org")

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

func viewCommand(ctx *context.Context) *cobra.Command {
	viewCmd := &cobra.Command{
		Use:   "view",
		Short: "View details of a workflow run including jobs and steps",
		Long: `View detailed information about a specific GitHub Actions workflow run, including all jobs and their step-level status.
Use --watch to poll for updates until the run completes.`,
		Aliases: []string{"detail", "info"},
		Example: heredoc.Doc(`
			$ sgh workflow view --org sample-org --repo sample-repo1 --run 123456789
			$ sgh workflow view --org sample-org --repo sample-repo1 --run 123456789 --watch
			$ sgh workflow view --org sample-org --repo sample-repo1 --run 123456789 --watch --interval 5
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			req := workflow.WorkflowRunRequest{
				OrgName:  orgName,
				RepoName: repoName,
				RunID:    runID,
			}

			if !watch {
				detail := workflow.GetWorkflowRunDetail(ctx, req)
				ui.PrintWorkflowRunDetail(detail)
				return
			}

			runWatchLoop(ctx, req)
		},
	}

	viewCmd.Flags().StringVarP(&repoName, "repository", "r", "", "repository name")
	viewCmd.Flags().IntVarP(&runID, "run", "R", 0, "workflow run ID")
	viewCmd.Flags().BoolVarP(&watch, "watch", "W", false, "poll for updates until the workflow run completes")
	viewCmd.Flags().IntVar(&watchInterval, "interval", 10, "polling interval in seconds when using --watch")
	viewCmd.MarkPersistentFlagRequired("org")
	viewCmd.MarkFlagRequired("repository")
	viewCmd.MarkFlagRequired("run")

	return viewCmd
}

func runWatchLoop(ctx *context.Context, req workflow.WorkflowRunRequest) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	interval := time.Duration(watchInterval) * time.Second
	var prevLineCount int

	for {
		detail := workflow.GetWorkflowRunDetail(ctx, req)

		// Move cursor up to overwrite previous output
		if prevLineCount > 0 {
			fmt.Fprintf(os.Stdout, "\033[%dA\033[J", prevLineCount)
		}

		rendered := ui.RenderWorkflowRunDetail(detail)
		if detail.IsInProgress() {
			watchHint := fmt.Sprintf("\n  Watching... refreshing every %ds (Ctrl+C to stop)\n", watchInterval)
			rendered += watchHint
		}
		fmt.Print(rendered)
		prevLineCount = strings.Count(rendered, "\n")

		if !detail.IsInProgress() {
			return
		}

		select {
		case <-sigChan:
			fmt.Println("\n  Watch stopped.")
			return
		case <-time.After(interval):
		}
	}
}

func rerunCommand(ctx *context.Context) *cobra.Command {
	rerunCmd := &cobra.Command{
		Use:   "rerun",
		Short: "Rerun a workflow run",
		Long:  `Rerun a specific GitHub Actions workflow run in a repository.`,
		Example: heredoc.Doc(`
			$ sgh workflow rerun --org sample-org --repo sample-repo1 --run 123456789
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			req := workflow.WorkflowRunRequest{
				OrgName:  orgName,
				RepoName: repoName,
				RunID:    runID,
			}
			response := workflow.RerunWorkflowRun(ctx, req)
			if response.ErrorMessage != "" {
				logger.Glog.Error().Msg(response.ErrorMessage)
			} else {
				ui.PrintWorkflowRuns([]model.WorkflowRun{response})
			}
		},
	}

	rerunCmd.Flags().StringVarP(&repoName, "repository", "r", "", "repository name")
	rerunCmd.Flags().IntVarP(&runID, "run", "R", 0, "workflow run ID")
	rerunCmd.MarkPersistentFlagRequired("org")
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
			$ sgh workflow cancel --org sample-org --repo sample-repo1 --run 123456789
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			req := workflow.WorkflowRunRequest{
				OrgName:  orgName,
				RepoName: repoName,
				RunID:    runID,
			}
			response := workflow.CancelWorkflowRun(ctx, req)
			if response.ErrorMessage != "" {
				logger.Glog.Error().Msg(response.ErrorMessage)
			} else {
				ui.PrintWorkflowRuns([]model.WorkflowRun{response})
			}
		},
	}

	cancelCmd.Flags().StringVarP(&repoName, "repository", "r", "", "repository name")
	cancelCmd.Flags().IntVarP(&runID, "run", "R", 0, "workflow run ID")
	cancelCmd.MarkPersistentFlagRequired("org")
	cancelCmd.MarkFlagRequired("repository")
	cancelCmd.MarkFlagRequired("run")

	return cancelCmd
}
