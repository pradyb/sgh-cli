package postrelease

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/prady-lab/sgh-cli/pkg/context"
	postrelease "github.com/prady-lab/sgh-cli/pkg/postrelease"
	"github.com/prady-lab/sgh-cli/pkg/ui"
	"github.com/prady-lab/sgh-cli/utils"
)

var (
	title        string
	body         string
	baseRef      string
	headRef      string
	repoNames    []string
	excludeRepos []string
	createTag    bool
	tagName      string
)

func NewPostReleaseCommand(ctx *context.Context) *cobra.Command {
	postReleaseCmd := &cobra.Command{
		Use:   "post-release",
		Short: "Perform Post release activities like merging to main/develop and tagging",
		Long:  `Perform Post release activities like merging to main/develop and tagging`,
		Example: heredoc.Doc(`
			$ sgh post-release --org sample-org --base "main" --head "Release-1.0" --create-tag --title "Release 1.0"
			$ sgh post-release --org sample-org --base "main" --head "Release-1.0" --repo sample-repo1 --repo sample-repo2
			$ sgh post-release --org sample-org --base "main" --head "Release-1.0" --create-tag --repo sample-repo1 --repo sample-repo2 
			$ sgh post-release --org sample-org --base "main" --head "Release-1.0" --create-tag --repo sample-repo1 --repo sample-repo2 --exclude-repos sample-repo1
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			postReleaseResponses := postrelease.ProcessPostRelease(ctx, postrelease.PostReleaseRequest{OrgName: orgName, RepoNames: repoNames, ExcludeRepos: excludeRepos, BaseRef: baseRef, HeadRef: headRef, Title: title, Body: body, CreateTag: createTag, TagName: tagName})
			ui.PrintPostReleaseResponses(postReleaseResponses)
		},
	}

	postReleaseCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names")
	postReleaseCmd.Flags().StringArrayVarP(&excludeRepos, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "repository names to exclude from post release activities")
	postReleaseCmd.Flags().StringVarP(&baseRef, "base", "B", "", "The `branch` into which you want your code merged")
	postReleaseCmd.Flags().StringVarP(&headRef, "head", "H", "", "The `branch` that contains commits for your pull request")
	postReleaseCmd.Flags().StringVarP(&title, "title", "t", "", "title for the pull request")
	postReleaseCmd.Flags().StringVarP(&body, "body", "b", "", "body for the pull request")
	postReleaseCmd.Flags().BoolVarP(&createTag, "create-tag", "c", false, "create tag for the release")
	postReleaseCmd.Flags().StringVarP(&tagName, "tag-name", "T", "", "tag name for the release")

	postReleaseCmd.MarkPersistentFlagRequired("org")
	postReleaseCmd.MarkFlagRequired("base")
	postReleaseCmd.MarkFlagRequired("head")
	postReleaseCmd.MarkFlagRequired("title")
	postReleaseCmd.MarkFlagsRequiredTogether("create-tag", "tag-name")

	return postReleaseCmd
}
