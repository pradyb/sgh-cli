package pr

import (
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/pr"
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
			ui.PrintPullRequestResponses(responses)
		},
	}

	createCmd.Flags().StringVarP(&title, "title", "t", "", "Title for the pull request")
	createCmd.Flags().StringVarP(&body, "body", "b", "", "Body for the pull request")
	createCmd.Flags().StringVarP(&baseRef, "base", "B", "", "The `branch` into which you want your code merged")
	createCmd.Flags().StringVarP(&headRef, "head", "H", "", "The `branch` that contains commits for your pull request")
	createCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names")
	createCmd.MarkFlagRequired("title")
	createCmd.MarkFlagRequired("branch")
	createCmd.MarkFlagRequired("head")
	createCmd.MarkFlagRequired("org")
	return createCmd
}

var allPullRequests bool

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
			responses := pr.ListPullRequests(ctx, orgName, repoNames, baseRef, headRef, allPullRequests)
			ui.PrintPullRequestResponses(responses)
		},
	}

	listCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names")
	listCmd.Flags().BoolVarP(&allPullRequests, "all", "a", false, "to fetch all the pull requests including closed ones")
	listCmd.Flags().StringVarP(&baseRef, "base", "B", "", "The `branch` into which you want your code merged")
	listCmd.Flags().StringVarP(&headRef, "head", "H", "", "The `branch` that contains commits for your pull request")
	listCmd.MarkFlagRequired("org")
	return listCmd
}
