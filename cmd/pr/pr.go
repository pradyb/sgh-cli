package pr

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/prady-lab/sgh-cli/pkg/pr"
	"github.com/prady-lab/sgh-cli/pkg/pr/prompt"
	"github.com/prady-lab/sgh-cli/pkg/ui"
	"github.com/prady-lab/sgh-cli/utils"
)

func NewPRCommand(ctx *context.Context) *cobra.Command {
	prCmd := &cobra.Command{
		Use:   "pr <command>",
		Short: "Perform PR operations like create/review/merge/close/update/list",
		Long:  `Perform PR operations like create/review/merge/close/update/list`,
	}

	prCmd.AddCommand(CreateCommand(ctx))
	prCmd.AddCommand(ListCommand(ctx))
	prCmd.AddCommand(ViewCommand(ctx))
	prCmd.AddCommand(ReviewCommand(ctx))
	prCmd.AddCommand(UpdateCommand(ctx))
	prCmd.AddCommand(MergeCommand(ctx))
	return prCmd
}

var (
	title           string
	body            string
	baseRef         string
	headRef         string
	repoNames       []string
	exclueRepoNames []string
)

func CreateCommand(ctx *context.Context) *cobra.Command {
	createCmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a pull request",
		Long:    `Create a pull request on GitHub for given repos or all the selected reps in the given org/owner`,
		Aliases: []string{"add"},
		Example: heredoc.Doc(`
			$ sgh pr create --org sample-org --title "PR for feature" --body "This PR is for feature" --head "feature-branch" --base "develop"
			$ sgh pr create --org sample-org --title "PR for feature" --body "This PR is for feature" --head "feature-branch" --base "main" -r sample-repo1 -r sample-repo2
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if ctx.DryRun {
				ui.PrintDryRunBanner()
				repos, _ := processor.ResolveRepositoryNames(ctx, orgName, repoNames, exclueRepoNames)
				ui.PrintDryRunActions("Create Pull Request", orgName, repos, map[string]string{
					"Title": title, "Base": baseRef, "Head": headRef,
				})
				return
			}
			responses := pr.CreateNewPullRequest(ctx, pr.PRRequest{OrgName: orgName, RepoNames: repoNames, ExcludeRepoNames: exclueRepoNames, BaseRef: baseRef, HeadRef: headRef, Title: title, Body: body})
			logger.Flog.Info().Msg("Pull request created successfully")
			ui.PrintPullRequestResponses(responses, "", ctx.Compact)
		},
	}

	createCmd.Flags().StringVarP(&title, "title", "t", "", "title for the pull request")
	createCmd.Flags().StringVarP(&body, "body", "b", "", "body for the pull request")
	createCmd.Flags().StringVarP(&baseRef, "base", "B", "", "The `branch` into which you want your code merged")
	createCmd.Flags().StringVarP(&headRef, "head", "H", "", "The `branch` that contains commits for your pull request")
	createCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names")
	createCmd.Flags().StringArrayVarP(&exclueRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "repository names to exclude")

	createCmd.MarkPersistentFlagRequired("org")
	createCmd.MarkFlagRequired("title")
	createCmd.MarkFlagRequired("branch")
	createCmd.MarkFlagRequired("head")
	return createCmd
}

var (
	allPullRequests bool
	interactive     bool
	lastCount       int
	author          string
	assignee        string
	reviewer        string
	label           string
	since           string
	prSortBy        string
)

func ListCommand(ctx *context.Context) *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List pull requests",
		Long: `List pull requests on GitHub for given repos or all the selected reps in the given org/owner
Default fetches all open Pull Requests, use -a flag to fetches all Pull Requests`,

		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			$ sgh pr list --org sample-org
			$ sgh pr list --org sample-org -r sample-repo1 -r sample-repo2 --all-status
			$ sgh pr list --org sample-org -r sample-repo1 -r sample-repo2 --head "feature-branch" --base "develop"
			$ sgh pr list --org sample-org -r sample-repo1 -r sample-repo2 --base "develop" --author "john-doe" --assignee "jane-doe"
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			req := pr.PRRequest{
				OrgName:          orgName,
				RepoNames:        repoNames,
				ExcludeRepoNames: exclueRepoNames,
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
	listCmd.Flags().StringArrayVarP(&exclueRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "repository names to exclude")
	listCmd.Flags().BoolVar(&allPullRequests, "all-status", false, "to fetch all the pull requests including closed ones. Default is false")
	listCmd.Flags().StringVarP(&baseRef, "base", "B", "", "The `branch` into which you want your code merged")
	listCmd.Flags().StringVarP(&headRef, "head", "H", "", "The `branch` that contains commits for your pull request")
	listCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "interactive mode to select the PR to merge")
	listCmd.Flags().IntVarP(&lastCount, "last", "l", 20, "The `number` of pull requests to fetch")
	listCmd.Flags().StringVarP(&author, "author", "a", "", "The `author` of the pull request")
	listCmd.Flags().StringVarP(&assignee, "assignee", "A", "", "The `assignee` of the pull request")
	listCmd.Flags().StringVarP(&reviewer, "reviewer", "R", "", "The `reviewer` of the pull request")
	listCmd.Flags().StringVar(&label, "label", "", "filter by `label` name")
	listCmd.Flags().StringVar(&since, "since", "", "filter PRs created on or after `date` (YYYY-MM-DD)")
	listCmd.Flags().StringVar(&prSortBy, "sort", "", "sort results by: repo, title, author, status")

	listCmd.MarkPersistentFlagRequired("org")

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
	viewCmd.Flags().IntVarP(&viewPR, "pr", "P", 0, "pull request `number`")

	viewCmd.MarkPersistentFlagRequired("org")
	viewCmd.MarkFlagRequired("repository")
	viewCmd.MarkFlagRequired("pr")

	return viewCmd
}

var (
	prNumber    int
	action      string
	repoName    string
	reviewEvent string
	reviewBody  string
)

func ReviewCommand(ctx *context.Context) *cobra.Command {
	reviewCmd := &cobra.Command{
		Use:   "review",
		Short: "Review a pull request (approve, request changes, or comment)",
		Long: `Submit a review on a pull request. Supported events:
  approve            Approve the pull request
  request_changes    Request changes on the pull request
  comment            Leave a general comment`,
		Example: heredoc.Doc(`
			$ sgh pr review --org sample-org -r sample-repo --pr 42 --event approve
			$ sgh pr review --org sample-org -r sample-repo --pr 42 --event request_changes --body "Please fix the tests"
			$ sgh pr review --org sample-org -r sample-repo --pr 42 --event comment --body "Looks good overall"
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			event := strings.ToUpper(reviewEvent)
			validEvents := map[string]bool{"APPROVE": true, "REQUEST_CHANGES": true, "COMMENT": true}
			if !validEvents[event] {
				logger.Glog.Error().Msgf("Invalid event: %s. Must be one of: approve, request_changes, comment", reviewEvent)
				cmd.Help()
				return
			}
			if (event == "REQUEST_CHANGES" || event == "COMMENT") && reviewBody == "" {
				logger.Glog.Error().Msg("--body is required for request_changes and comment events")
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
	reviewCmd.Flags().IntVarP(&prNumber, "pr", "P", 0, "pull request `number`")
	reviewCmd.Flags().StringVarP(&reviewEvent, "event", "E", "", "review event: approve, request_changes, comment")
	reviewCmd.Flags().StringVarP(&reviewBody, "body", "b", "", "review comment `body` (required for request_changes and comment)")

	reviewCmd.MarkPersistentFlagRequired("org")
	reviewCmd.MarkFlagRequired("repository")
	reviewCmd.MarkFlagRequired("pr")
	reviewCmd.MarkFlagRequired("event")

	return reviewCmd
}

func UpdateCommand(ctx *context.Context) *cobra.Command {
	updateCmd := &cobra.Command{
		Use:     "update",
		Short:   "Update a pull request",
		Long:    `Update a pull request on GitHub for given repo`,
		Aliases: []string{"edit"},
		Example: heredoc.Doc(`
			$ sgh pr update --org sample-org -r sample-repo1 --pr 1 --action close 
			$ sgh pr update --org sample-org -r sample-repo1 --pr 1 --action open
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if action != "close" && action != "open" {
				logger.Glog.Error().Msgf("Invalid action provided. Please provide either close or open")
				cmd.Help()
				return
			}
			if ctx.DryRun {
				ui.PrintDryRunBanner()
				ui.PrintDryRunActions("Update Pull Request", orgName, []string{repoName}, map[string]string{
					"PR Number": fmt.Sprintf("%d", prNumber), "Action": action,
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

	updateCmd.Flags().IntVarP(&prNumber, "pr", "P", 0, "The `PR number` into which you want to update")
	updateCmd.Flags().StringVarP(&action, "action", "a", "", "The `action` you want to perform on the PR. Possible values are close or open")
	updateCmd.Flags().StringVarP(&repoName, "repository", "r", "", "repository name")

	updateCmd.MarkPersistentFlagRequired("org")
	updateCmd.MarkFlagRequired("repository")
	updateCmd.MarkFlagRequired("pr")
	updateCmd.MarkFlagRequired("action")

	return updateCmd
}

func MergeCommand(ctx *context.Context) *cobra.Command {
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

	mergeCmd.Flags().IntVarP(&prNumber, "pr", "P", 0, "The `PR number` into which you want to update")
	mergeCmd.Flags().StringVarP(&title, "title", "t", "", "custom commit title (optional, GitHub auto-generates if omitted)")
	mergeCmd.Flags().StringVarP(&body, "body", "b", "", "extra detail to append to commit message (optional)")
	mergeCmd.Flags().StringVarP(&repoName, "repository", "r", "", "repository name")

	mergeCmd.MarkPersistentFlagRequired("org")
	mergeCmd.MarkFlagRequired("repository")
	mergeCmd.MarkFlagRequired("pr")

	return mergeCmd
}
