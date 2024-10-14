package pr

import (
	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/prady-lab/sgh-cli/pkg/pr"
	"github.com/prady-lab/sgh-cli/pkg/pr/prompt"
	"github.com/prady-lab/sgh-cli/pkg/ui"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
)

func NewPRCommand(ctx *context.Context) *cobra.Command {

	var prCmd = &cobra.Command{
		Use:   "pr <command>",
		Short: "Manage pull requests",
		Long:  `Perform PR operations like create/review/merge/close/update`,
	}

	prCmd.AddCommand(CreateCommand(ctx))
	prCmd.AddCommand(ListCommand(ctx))
	prCmd.AddCommand(UpdateCommand(ctx))
	prCmd.AddCommand(MergeCommand(ctx))
	return prCmd
}

var title string
var body string
var baseRef string
var headRef string
var repoNames []string

func CreateCommand(ctx *context.Context) *cobra.Command {
	var createCmd = &cobra.Command{
		Use:     "create",
		Short:   "Create a pull request",
		Long:    `Create a pull request on GitHub for given repos or all the selected reps in the given org/owner`,
		Aliases: []string{"add"},
		Example: heredoc.Doc(`
			$ sgh pr create --title "PR for feature" --body "This PR is for feature" --head "feature-branch" --base "develop" --org sample-org
			$ sgh pr create --title "PR for feature" --body "This PR is for feature" --head "feature-branch" --base "main" --org sample-org -r sample-repo1 -r sample-repo2
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			responses := pr.CreateNewPullRequest(ctx, orgName, repoNames, baseRef, headRef, title, body)
			logger.Flog.Info().Msg("Pull request created successfully")
			ui.PrintPullRequestResponses(responses)
		},
	}

	createCmd.Flags().StringVarP(&title, "title", "t", "", "title for the pull request")
	createCmd.Flags().StringVarP(&body, "body", "b", "", "body for the pull request")
	createCmd.Flags().StringVarP(&baseRef, "base", "B", "", "The `branch` into which you want your code merged")
	createCmd.Flags().StringVarP(&headRef, "head", "H", "", "The `branch` that contains commits for your pull request")
	createCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names")

	createCmd.MarkPersistentFlagRequired("org")
	createCmd.MarkFlagRequired("title")
	createCmd.MarkFlagRequired("branch")
	createCmd.MarkFlagRequired("head")
	return createCmd
}

var allPullRequests bool
var interactive bool

func ListCommand(ctx *context.Context) *cobra.Command {
	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "List pull requests",
		Long: `List pull requests on GitHub for given repos or all the selected reps in the given org/owner
Default fetches all open Pull Requests, use -a flag to fetches all Pull Requests`,

		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			$ sgh pr list --org sample-org
			$ sgh pr list --org sample-org -r sample-repo1 -r sample-repo2 -a
			$ sgh pr list --org sample-org -r sample-repo1 -r sample-repo2 --head "feature-branch" --base "develop"
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if !interactive {
				responses := pr.ListPullRequests(ctx, orgName, repoNames, baseRef, headRef, allPullRequests)
				ui.PrintPullRequestResponses(responses)
			} else {
				prompt.RunInteractivePR(ctx, orgName, repoNames, baseRef, headRef, allPullRequests)
			}
		},
	}

	listCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names")
	listCmd.Flags().BoolVarP(&allPullRequests, "all", "a", false, "to fetch all the pull requests including closed ones")
	listCmd.Flags().StringVarP(&baseRef, "base", "B", "", "The `branch` into which you want your code merged")
	listCmd.Flags().StringVarP(&headRef, "head", "H", "", "The `branch` that contains commits for your pull request")
	listCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "interactive mode to select the PR to merge")

	listCmd.MarkPersistentFlagRequired("org")

	return listCmd
}

var prNumber int
var action string
var repoName string

func UpdateCommand(ctx *context.Context) *cobra.Command {
	var updateCmd = &cobra.Command{
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
			response := pr.UpdatePullRequest(ctx, orgName, repoName, prNumber, action)
			ui.PrintPullRequestResponses([]model.PullRequestResponse{response})
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
	var mergeCmd = &cobra.Command{
		Use:     "merge",
		Short:   "merge a pull request",
		Long:    `merge a pull request on GitHub for given repo`,
		Aliases: []string{"edit"},
		Example: heredoc.Doc(`
			$ sgh pr merge --org sample-org -r sample-repo1 --pr 1 --title "Post Release merge" 
			$ sgh pr merge --org sample-org -r sample-repo1 --pr 1 --title "Post Release merge" --body "This PR is for post release merge"
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			response := pr.MergePullRequest(ctx, orgName, repoName, prNumber, title, body)
			ui.PrintMergeResponses([]model.MergeResponse{response})
		},
	}

	mergeCmd.Flags().IntVarP(&prNumber, "pr", "P", 0, "The `PR number` into which you want to update")
	mergeCmd.Flags().StringVarP(&title, "title", "t", "", "title for the automatic commit message")
	mergeCmd.Flags().StringVarP(&body, "body", "b", "", "extra detail to append to automatic commit message")
	mergeCmd.Flags().StringVarP(&repoName, "repository", "r", "", "repository name")

	mergeCmd.MarkPersistentFlagRequired("org")
	mergeCmd.MarkFlagRequired("repository")
	mergeCmd.MarkFlagRequired("pr")
	mergeCmd.MarkFlagRequired("title")

	return mergeCmd
}
