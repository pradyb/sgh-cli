package tag

import (
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/tag"
	"github.com/prady-lab/sgh-cli/pkg/ui"

	logger "github.com/prady-lab/sgh-cli/utils"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
)

var tagName string
var refBranchName string
var repoNames []string
var message string

func NewTagCommand(ctx *context.Context) *cobra.Command {

	var tagCmd = &cobra.Command{
		Use:   "tag <command>",
		Short: "Manage tags.",
		Long:  `Perform Tag operations like create/delete .`,
	}

	tagCmd.AddCommand(CreateCommand(ctx))
	tagCmd.AddCommand(DeleteCommand(ctx))
	return tagCmd
}

func CreateCommand(ctx *context.Context) *cobra.Command {
	var createCmd = &cobra.Command{
		Use:     "create",
		Short:   "Create a new tag from a existing branch",
		Long:    `Create a new tag from a existing branch for given repos or all the selected reps in the given org/owner`,
		Aliases: []string{"add"},
		Example: heredoc.Doc(`
			$ sgh tag create --Tag Release-1.0 --Head Release-1.0 -o sample-org -m 'Tag for Release 1.0'
			$ sgh tag create --Tag Release-1.0 --Head Release-1.0 -o sample-org -m 'Tag for Release 1.0' -r sample-repo1 -r sample-repo2
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if orgName == "" {
				logger.Glog.Error().Msgf("Organization name is required")
				cmd.Help()
				return
			}
			responses := tag.CreateNewTags(ctx, orgName, tagName, refBranchName, repoNames, message)
			if len(responses) > 0 {
				ui.PrintResponses(responses)
			}
		},
	}

	createCmd.Flags().StringVarP(&tagName, "tag", "T", "", "The new `tag` which you want to be created")
	createCmd.Flags().StringVarP(&refBranchName, "head", "H", "", "The `branch` from which you want to use as reference")
	createCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "The `repository` names for which you want to create the tag. If not provided, it will create for all the repositories in the organization")
	createCmd.MarkFlagRequired("tag")
	createCmd.MarkFlagRequired("head")
	createCmd.Flags().StringVarP(&message, "message", "m", "", "The `message` for the tagging")
	createCmd.MarkFlagRequired("message")
	return createCmd
}

func DeleteCommand(ctx *context.Context) *cobra.Command {
	var createCmd = &cobra.Command{
		Use:     "delete",
		Short:   "Delete a new tag.",
		Long:    `Delete a new tag for given repos or all the selected repos in the given org/owner.`,
		Aliases: []string{"rm"},
		Example: heredoc.Doc(`
			$ sgh tag delete --Tag Release-1.0 --org sample-org
			$ sgh tag delete --Tag Release-1.0 --org sample-org -r sample-repo1 -r sample-repo2
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if orgName == "" {
				logger.Glog.Error().Msgf("Organization name is required")
				cmd.Help()
				return
			}
			responses := tag.DeleteTags(ctx, orgName, tagName, repoNames)
			if len(responses) > 0 {
				ui.PrintResponses(responses)
			}
		},
	}
	createCmd.Flags().StringVarP(&tagName, "tag", "T", "", "The new `tag` which you want to be created")
	createCmd.MarkFlagRequired("tag")
	createCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names")
	return createCmd
}
