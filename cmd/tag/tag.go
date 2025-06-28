package tag

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/prady-lab/sgh-cli/pkg/tag"
	"github.com/prady-lab/sgh-cli/pkg/ui"
	"github.com/prady-lab/sgh-cli/utils"
)

var (
	tagName          string
	refBranchName    string
	repoNames        []string
	excludeRepoNames []string
	message          string
)

func NewTagCommand(ctx *context.Context) *cobra.Command {
	tagCmd := &cobra.Command{
		Use:   "tag <command>",
		Short: "Create and delete tags",
		Long:  `Perform Tag operations like create/delete .`,
	}

	tagCmd.AddCommand(CreateCommand(ctx))
	tagCmd.AddCommand(DeleteCommand(ctx))
	return tagCmd
}

func CreateCommand(ctx *context.Context) *cobra.Command {
	createCmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new tag from a existing branch",
		Long:    `Create a new tag from a existing branch for given repos or all the selected reps in the given org/owner`,
		Aliases: []string{"add"},
		Example: heredoc.Doc(`
			$ sgh tag create --org sample-org --tag Release-1.0 --Head Release-1.0 -m 'Tag for Release 1.0'
			$ sgh tag create --org sample-org --tag Release-1.0 --Head Release-1.0 -m 'Tag for Release 1.0' --repo sample-repo1 --repo sample-repo2
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if orgName == "" {
				logger.Glog.Error().Msgf("Organization name is required")
				cmd.Help()
				return
			}
			req := tag.TagCreateRequest{
				OrgName:          orgName,
				RepoNames:        repoNames,
				ExcludeRepoNames: excludeRepoNames,
				TagName:          tagName,
				RefBranchName:    refBranchName,
				Message:          message,
			}
			responses := tag.CreateNewTags(ctx, req)
			ui.PrintResponses(responses)
		},
	}

	createCmd.Flags().StringVarP(&tagName, "tag", "T", "", "The new `tag` which you want to be created")
	createCmd.Flags().StringVarP(&refBranchName, "head", "H", "", "The `branch` from which you want to use as reference")
	createCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "The `repository` names for which you want to create the tag. If not provided, it will create for all the repositories in the organization")
	createCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "The `repository` names which you want to exclude from creating the tag")
	createCmd.Flags().StringVarP(&message, "message", "m", "", "The `message` for the tagging")

	createCmd.MarkPersistentFlagRequired("org")
	createCmd.MarkFlagRequired("tag")
	createCmd.MarkFlagRequired("head")
	createCmd.MarkFlagRequired("message")
	return createCmd
}

func DeleteCommand(ctx *context.Context) *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete a new tag",
		Long:    `Delete a new tag for given repos or all the selected repos in the given org/owner`,
		Aliases: []string{"rm"},
		Example: heredoc.Doc(`
			$ sgh tag delete --Tag Release-1.0 --org sample-org
			$ sgh tag delete --Tag Release-1.0 --org sample-org --repo sample-repo1 --repo sample-repo2
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if orgName == "" {
				logger.Glog.Error().Msgf("Organization name is required")
				cmd.Help()
				return
			}
			req := tag.TagDeleteRequest{
				OrgName:          orgName,
				RepoNames:        repoNames,
				ExcludeRepoNames: excludeRepoNames,
				TagName:          tagName,
			}
			responses := tag.DeleteTags(ctx, req)
			ui.PrintResponses(responses)
		},
	}

	deleteCmd.Flags().StringVarP(&tagName, "tag", "T", "", "The new `tag` which you want to be created")
	deleteCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names")
	deleteCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "The `repository` names which you want to exclude from deleting the tag")

	deleteCmd.MarkPersistentFlagRequired("org")
	deleteCmd.MarkFlagRequired("tag")
	return deleteCmd
}
