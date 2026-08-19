// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package pr

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/pradyb/sgh-cli/internal/model"
	"github.com/pradyb/sgh-cli/internal/processor"
	"github.com/pradyb/sgh-cli/pkg/context"
	"github.com/pradyb/sgh-cli/pkg/logger"
	"github.com/pradyb/sgh-cli/pkg/pr"
	"github.com/pradyb/sgh-cli/pkg/pr/prompt"
	"github.com/pradyb/sgh-cli/pkg/ui"
	"github.com/pradyb/sgh-cli/utils"
)

func repoCompletionFn(ctx *context.Context) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		orgName, _ := cmd.Root().PersistentFlags().GetString("org")
		return ctx.Config.RepositoriesNames(orgName), cobra.ShellCompDirectiveNoFileComp
	}
}

func NewPRCommand(ctx *context.Context) *cobra.Command {
	prCmd := &cobra.Command{
		Use:   "pr <command>",
		Short: "Perform PR operations like create/list/view/review/merge/close/reopen",
		Long:  `Perform PR operations like create/list/view/review/merge/close/reopen`,
	}

	prCmd.AddCommand(CreateCommand(ctx))
	prCmd.AddCommand(ListCommand(ctx))
	prCmd.AddCommand(ViewCommand(ctx))
	prCmd.AddCommand(ReviewCommand(ctx))
	prCmd.AddCommand(UpdateCommand(ctx))
	prCmd.AddCommand(MergeCommand(ctx))
	prCmd.AddCommand(CloseCommand(ctx))
	prCmd.AddCommand(ReopenCommand(ctx))
	return prCmd
}

func CreateCommand(ctx *context.Context) *cobra.Command {
	var title string
	var body string
	var baseRef string
	var headRef string
	var label string
	var repoNames []string
	var excludeRepoNames []string

	createCmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a pull request",
		Long:    `Create a pull request on GitHub for the given org — targeting all repos or a subset via -r.`,
		Aliases: []string{"add"},
		Example: heredoc.Doc(`
			$ sgh pr create --org sample-org --title "PR for feature" --head "feature-branch" --base "develop"
			$ sgh pr create --org sample-org --title "PR for feature" --head "feature-branch" --base "main" -r sample-repo1 -r sample-repo2
			$ sgh pr create --org sample-org --title "Bug fix" --head "fix/login" --base "main" --label "bug"
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if ctx.DryRun {
				ui.PrintDryRunBanner()
				repos, _ := processor.ResolveRepositoryNames(ctx, orgName, repoNames, excludeRepoNames)
				ui.PrintDryRunActions("Create Pull Request", orgName, repos, map[string]string{
					"Title": title, "Base": baseRef, "Head": headRef,
				})
				return
			}
			responses := pr.CreateNewPullRequest(ctx, pr.PRRequest{OrgName: orgName, RepoNames: repoNames, ExcludeRepoNames: excludeRepoNames, BaseRef: baseRef, HeadRef: headRef, Title: title, Body: body})
			logger.Flog.Info().Msg("Pull request created successfully")
			ui.PrintPullRequestResponses(responses, "", ctx.Compact)
		},
	}

	createCmd.Flags().StringVarP(&title, "title", "t", "", "title for the pull request")
	createCmd.Flags().StringVarP(&body, "body", "b", "", "body for the pull request")
	createCmd.Flags().StringVarP(&baseRef, "base", "B", "", "the base `branch` to merge into")
	createCmd.Flags().StringVarP(&headRef, "head", "H", "", "the head `branch` containing commits for the PR")
	createCmd.Flags().StringVarP(&label, "label", "l", "", "add a `label` by name")
	createCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names")
	createCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "repository names to exclude")

	createCmd.MarkFlagRequired("title")
	createCmd.MarkFlagRequired("base")
	createCmd.MarkFlagRequired("head")
	return createCmd
}

func ListCommand(ctx *context.Context) *cobra.Command {
	var allPullRequests bool
	var prState string
	var interactive bool
	var lastCount int
	var author string
	var assignee string
	var reviewer string
	var label string
	var since string
	var prSortBy string
	var repoNames []string
	var excludeRepoNames []string
	var baseRef string
	var headRef string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List pull requests",
		Long: `List pull requests across repos in the given organization.
By default lists open pull requests. Use --state to filter by state.`,

		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			$ sgh pr list --org sample-org
			$ sgh pr list --org sample-org --state all
			$ sgh pr list --org sample-org --state closed
			$ sgh pr list --org sample-org -r sample-repo1 -r sample-repo2 --head "feature-branch" --base "develop"
			$ sgh pr list --org sample-org --base "develop" --author "john-doe" --assignee "jane-doe"
			$ sgh pr list --org sample-org --label bug --last 50
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			// support deprecated --all and --all-status aliases
			if v, _ := cmd.Flags().GetBool("all"); v {
				prState = "all"
			}
			if v, _ := cmd.Flags().GetBool("all-status"); v {
				prState = "all"
			}
			allPullRequests = prState != "open"
			req := pr.PRRequest{
				OrgName:          orgName,
				RepoNames:        repoNames,
				ExcludeRepoNames: excludeRepoNames,
				BaseRef:          baseRef,
				HeadRef:          headRef,
				LastCount:        lastCount,
				Author:           author,
				Assignee:         assignee,
				Reviewer:         reviewer,
				Label:            label,
				Since:            since,
				All:              allPullRequests,
				IsInteractive:    interactive,
			}
			if !interactive {
				responses := pr.ListPullRequests(ctx, req)
				ui.SortPullRequests(responses, prSortBy)
				if ctx.Limit > 0 && len(responses) > ctx.Limit {
					responses = responses[:ctx.Limit]
				}
				if ctx.JSON {
					ui.PrintJSON(responses)
					return
				}
				ui.PrintPullRequestResponses(responses, prSortBy, ctx.Compact)
			} else {
				prompt.RunInteractivePR(ctx, req)
			}
		},
	}

	listCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names")
	listCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "repository names to exclude")
	listCmd.RegisterFlagCompletionFunc("repository", repoCompletionFn(ctx))
	listCmd.Flags().StringVarP(&prState, "state", "s", "open", "filter by state: open, closed, merged, all")
	listCmd.Flags().Bool("all", false, "alias for --state all (deprecated)")
	listCmd.Flags().MarkHidden("all")
	listCmd.Flags().Bool("all-status", false, "alias for --state all (deprecated)")
	listCmd.Flags().MarkHidden("all-status")
	listCmd.Flags().StringVarP(&baseRef, "base", "B", "", "filter by base `branch`")
	listCmd.Flags().StringVarP(&headRef, "head", "H", "", "filter by head `branch`")
	listCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "interactive mode to select the PR to merge")
	listCmd.Flags().IntVar(&lastCount, "last", 20, "max pull requests to fetch per repo (use global --limit to cap combined output)")
	listCmd.Flags().StringVarP(&assignee, "assignee", "a", "", "filter by `assignee` login")
	listCmd.Flags().StringVarP(&author, "author", "A", "", "filter by `author` login")
	listCmd.Flags().StringVarP(&reviewer, "reviewer", "R", "", "filter by `reviewer` login")
	listCmd.Flags().StringVarP(&label, "label", "l", "", "filter by `label` name")
	listCmd.Flags().StringVar(&since, "since", "", "filter PRs created on or after `date` (YYYY-MM-DD)")
	listCmd.Flags().StringVar(&prSortBy, "sort", "", "sort results by: repo, title, author, status")

	return listCmd
}

func ViewCommand(ctx *context.Context) *cobra.Command {
	var viewRepo string
	var viewPR int

	viewCmd := &cobra.Command{
		Use:   "view",
		Short: "View pull request details",
		Long: `View detailed information about a pull request, including files changed,
check runs, and reviews.`,
		Aliases: []string{"detail", "info"},
		Example: heredoc.Doc(`
			$ sgh pr view --org sample-org -r sample-repo --pr 42
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, []string{viewRepo})
			if len(actualRepoNames) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), ui.ErrorMessage("repository not found: %s", viewRepo))
				return
			}
			repoN := actualRepoNames[0]

			prResp, filesResp, checkRunResp, reviews := pr.GetPRDetailsGraphQL(ctx, pr.PRDetailsRequest{
				OrgName:  orgName,
				RepoName: repoN,
				PRNumber: viewPR,
			})

			if ctx.JSON {
				ui.PrintJSON(map[string]any{
					"pull_request": prResp,
					"files":        filesResp,
					"check_runs":   checkRunResp,
					"reviews":      reviews,
				})
				return
			}
			ui.PrintPRDetail(prResp, filesResp, checkRunResp, reviews)
		},
	}

	viewCmd.Flags().StringVarP(&viewRepo, "repository", "r", "", "repository name")
	viewCmd.Flags().IntVarP(&viewPR, "pr", "n", 0, "pull request `number`")

	viewCmd.MarkFlagRequired("repository")
	viewCmd.MarkFlagRequired("pr")

	return viewCmd
}

func ReviewCommand(ctx *context.Context) *cobra.Command {
	var prNumber int
	var repoName string
	var reviewEvent string
	var reviewBody string
	var reviewApprove bool
	var reviewComment bool
	var reviewReqChanges bool
	reviewCmd := &cobra.Command{
		Use:   "review",
		Short: "Review a pull request (approve, request changes, or comment)",
		Long: `Submit a review on a pull request.
Use exactly one action flag: --approve, --comment, or --request-changes.`,
		Example: heredoc.Doc(`
			$ sgh pr review --org sample-org -r sample-repo --pr 42 --approve
			$ sgh pr review --org sample-org -r sample-repo --pr 42 --request-changes --body "Please fix the tests"
			$ sgh pr review --org sample-org -r sample-repo --pr 42 --comment --body "Looks good overall"
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")

			// resolve event from boolean flags (matching gh pr review style)
			var event string
			switch {
			case reviewApprove:
				event = "APPROVE"
			case reviewReqChanges:
				event = "REQUEST_CHANGES"
			case reviewComment:
				event = "COMMENT"
			default:
				// legacy --event string fallback
				event = strings.ToUpper(reviewEvent)
			}

			validEvents := map[string]bool{"APPROVE": true, "REQUEST_CHANGES": true, "COMMENT": true}
			if !validEvents[event] {
				logger.Glog.Error().Msg("specify one of: --approve, --request-changes, --comment")
				cmd.Help()
				return
			}
			if (event == "REQUEST_CHANGES" || event == "COMMENT") && reviewBody == "" {
				logger.Glog.Error().Msg("--body is required with --request-changes and --comment")
				cmd.Help()
				return
			}
			actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, []string{repoName})
			if ctx.DryRun {
				ui.PrintDryRunBanner()
				ui.PrintDryRunActions("Review Pull Request", orgName, actualRepoNames, map[string]string{
					"PR Number": fmt.Sprintf("%d", prNumber), "Event": event,
				})
				return
			}
			req := pr.PRReviewRequest{
				OrgName:  orgName,
				RepoName: actualRepoNames[0],
				PRNumber: prNumber,
				Event:    event,
				Body:     reviewBody,
			}
			response := pr.ReviewPullRequest(ctx, req)
			if ctx.JSON {
				ui.PrintJSON(response)
				return
			}
			ui.PrintReviewResponse(response)
		},
	}

	reviewCmd.Flags().StringVarP(&repoName, "repository", "r", "", "repository name")
	reviewCmd.Flags().IntVarP(&prNumber, "pr", "n", 0, "pull request `number`")
	reviewCmd.Flags().BoolVarP(&reviewApprove, "approve", "a", false, "approve the pull request")
	reviewCmd.Flags().BoolVarP(&reviewComment, "comment", "c", false, "leave a general comment")
	reviewCmd.Flags().BoolVar(&reviewReqChanges, "request-changes", false, "request changes on the pull request")
	reviewCmd.Flags().StringVarP(&reviewBody, "body", "b", "", "review comment `body` (required with --comment and --request-changes)")
	// legacy flag kept as hidden for backward compat
	reviewCmd.Flags().StringVar(&reviewEvent, "event", "", "review event (deprecated: use --approve/--comment/--request-changes)")
	reviewCmd.Flags().MarkHidden("event")

	reviewCmd.MarkFlagRequired("repository")
	reviewCmd.MarkFlagRequired("pr")

	return reviewCmd
}

func UpdateCommand(ctx *context.Context) *cobra.Command {
	var prNumber int
	var action string
	var repoName string

	updateCmd := &cobra.Command{
		Use:     "update",
		Short:   "Update a pull request",
		Long:    `Update a pull request on GitHub for given repo`,
		Aliases: []string{"edit"},
		Example: heredoc.Doc(`
			$ sgh pr update --org sample-org -r sample-repo1 --pr 1 --state closed
			$ sgh pr update --org sample-org -r sample-repo1 --pr 1 --state open
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if action != "closed" && action != "open" {
				logger.Glog.Error().Msgf("Invalid state provided. Please provide either 'open' or 'closed'")
				cmd.Help()
				return
			}
			if ctx.DryRun {
				ui.PrintDryRunBanner()
				ui.PrintDryRunActions("Update Pull Request", orgName, []string{repoName}, map[string]string{
					"PR Number": fmt.Sprintf("%d", prNumber), "State": action,
				})
				return
			}
			req := pr.PRUpdateRequest{
				OrgName:  orgName,
				RepoName: repoName,
				PRNumber: prNumber,
				State:    action,
			}
			response := pr.UpdatePullRequest(ctx, req)
			ui.PrintPullRequestResponses([]model.PullRequestResponse{response}, "", ctx.Compact)
		},
	}

	updateCmd.Flags().IntVarP(&prNumber, "pr", "n", 0, "pull request `number`")
	updateCmd.Flags().StringVarP(&action, "state", "s", "", "new `state` for the PR: open or closed")
	updateCmd.Flags().StringVarP(&repoName, "repository", "r", "", "repository name")

	updateCmd.MarkFlagRequired("repository")
	updateCmd.MarkFlagRequired("pr")
	updateCmd.MarkFlagRequired("state")

	return updateCmd
}

func MergeCommand(ctx *context.Context) *cobra.Command {
	var prNumber int
	var repoName string
	var title string
	var body string

	mergeCmd := &cobra.Command{
		Use:   "merge",
		Short: "Merge a pull request",
		Long:  `Merge a pull request on GitHub for given repo.`,
		Example: heredoc.Doc(`
			$ sgh pr merge --org sample-org -r sample-repo1 --pr 1
			$ sgh pr merge --org sample-org -r sample-repo1 --pr 1 --title "Post Release merge"
			$ sgh pr merge --org sample-org -r sample-repo1 --pr 1 --title "Post Release merge" --body "Merging release branch"
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if ctx.DryRun {
				ui.PrintDryRunBanner()
				details := map[string]string{"PR Number": fmt.Sprintf("%d", prNumber)}
				if title != "" {
					details["Title"] = title
				}
				ui.PrintDryRunActions("Merge Pull Request", orgName, []string{repoName}, details)
				return
			}
			req := pr.PRMergeRequest{
				OrgName:  orgName,
				RepoName: repoName,
				PRNumber: prNumber,
				Title:    title,
				Body:     body,
			}
			response := pr.MergePullRequest(ctx, req)
			ui.PrintMergeResponses([]model.MergeResponse{response})
		},
	}

	mergeCmd.Flags().IntVarP(&prNumber, "pr", "n", 0, "pull request `number`")
	mergeCmd.Flags().StringVarP(&title, "title", "t", "", "custom commit title (optional, GitHub auto-generates if omitted)")
	mergeCmd.Flags().StringVarP(&body, "body", "b", "", "extra detail to append to commit message (optional)")
	mergeCmd.Flags().StringVarP(&repoName, "repository", "r", "", "repository name")

	mergeCmd.MarkFlagRequired("repository")
	mergeCmd.MarkFlagRequired("pr")

	return mergeCmd
}

func CloseCommand(ctx *context.Context) *cobra.Command {
	var prNumber int
	var repoName string

	closeCmd := &cobra.Command{
		Use:   "close",
		Short: "Close a pull request",
		Long:  `Close an open pull request in the specified repository.`,
		Example: heredoc.Doc(`
			$ sgh pr close --org sample-org -r sample-repo1 --pr 42
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if ctx.DryRun {
				ui.PrintDryRunBanner()
				ui.PrintDryRunActions("Close Pull Request", orgName, []string{repoName}, map[string]string{
					"PR Number": fmt.Sprintf("%d", prNumber),
				})
				return
			}
			req := pr.PRUpdateRequest{
				OrgName:  orgName,
				RepoName: repoName,
				PRNumber: prNumber,
				State:    "closed",
			}
			response := pr.UpdatePullRequest(ctx, req)
			ui.PrintPullRequestResponses([]model.PullRequestResponse{response}, "", ctx.Compact)
		},
	}

	closeCmd.Flags().IntVarP(&prNumber, "pr", "n", 0, "pull request `number`")
	closeCmd.Flags().StringVarP(&repoName, "repository", "r", "", "repository name")

	closeCmd.MarkFlagRequired("repository")
	closeCmd.MarkFlagRequired("pr")
	return closeCmd
}

func ReopenCommand(ctx *context.Context) *cobra.Command {
	var prNumber int
	var repoName string

	reopenCmd := &cobra.Command{
		Use:     "reopen",
		Short:   "Reopen a closed pull request",
		Long:    `Reopen a previously closed pull request in the specified repository.`,
		Aliases: []string{"open"},
		Example: heredoc.Doc(`
			$ sgh pr reopen --org sample-org -r sample-repo1 --pr 42
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if ctx.DryRun {
				ui.PrintDryRunBanner()
				ui.PrintDryRunActions("Reopen Pull Request", orgName, []string{repoName}, map[string]string{
					"PR Number": fmt.Sprintf("%d", prNumber),
				})
				return
			}
			req := pr.PRUpdateRequest{
				OrgName:  orgName,
				RepoName: repoName,
				PRNumber: prNumber,
				State:    "open",
			}
			response := pr.UpdatePullRequest(ctx, req)
			ui.PrintPullRequestResponses([]model.PullRequestResponse{response}, "", ctx.Compact)
		},
	}

	reopenCmd.Flags().IntVarP(&prNumber, "pr", "n", 0, "pull request `number`")
	reopenCmd.Flags().StringVarP(&repoName, "repository", "r", "", "repository name")

	reopenCmd.MarkFlagRequired("repository")
	reopenCmd.MarkFlagRequired("pr")
	return reopenCmd
}
