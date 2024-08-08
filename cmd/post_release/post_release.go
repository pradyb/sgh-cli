package postrelease

import (
	"github.com/prady-lab/sgh-cli/pkg/context"
	postrelease "github.com/prady-lab/sgh-cli/pkg/post_release"
	"github.com/prady-lab/sgh-cli/pkg/ui"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
)

var title string
var body string
var baseRef string
var headRef string
var repoNames []string
var createTag bool
var tagName string

func NewPostReleaseCommand(ctx *context.Context) *cobra.Command {

	var postReleaseCmd = &cobra.Command{
		Use:   "post-release",
		Short: "Perform Post release activities like merging to main/develop and tagging",
		Long:  `Perform Post release activities like merging to main/develop and tagging`,
		Example: heredoc.Doc(`
			$ sgh post-release -o sample-org --base "main" --head "Release-1.0" --create-tag --title "Release 1.0"
			$ sgh post-release -o sample-org -r sample-repo1 -r sample-repo2 --base "main" --head "Release-1.0"
			$ sgh post-release -o sample-org -r sample-repo1 -r sample-repo2 --base "main" --head "Release-1.0" --create-tag
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			postReleaseResponses := postrelease.ProcessPostRelease(ctx, orgName, repoNames, baseRef, headRef, title, body, createTag, tagName)
			ui.PrintPostReleaseResponses(postReleaseResponses)
		},
	}

	postReleaseCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names")
	postReleaseCmd.Flags().StringVarP(&baseRef, "base", "B", "", "The `branch` into which you want your code merged")
	postReleaseCmd.Flags().StringVarP(&headRef, "head", "H", "", "The `branch` that contains commits for your pull request")
	postReleaseCmd.Flags().StringVarP(&title, "title", "t", "", "Title for the pull request")
	postReleaseCmd.Flags().StringVarP(&body, "body", "b", "", "Body for the pull request")
	postReleaseCmd.Flags().BoolVarP(&createTag, "create-tag", "c", false, "Create tag for the release")
	postReleaseCmd.Flags().StringVarP(&tagName, "tag-name", "T", "", "Tag name for the release")

	postReleaseCmd.MarkFlagRequired("title")
	postReleaseCmd.MarkFlagRequired("branch")
	postReleaseCmd.MarkFlagRequired("head")
	postReleaseCmd.MarkFlagsRequiredTogether("create-tag", "tag-name")

	return postReleaseCmd
}
