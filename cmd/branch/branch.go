package branch

import (
	"fmt"

	"github.com/prady-lab/sgh-cli/pkg/branch"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/ui"

	logger "github.com/prady-lab/sgh-cli/utils"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
)

func NewBranchCommand(ctx *context.Context) *cobra.Command {

	var branchCmd = &cobra.Command{
		Use:   "branch <command>",
		Short: "Manage Branches",
		Long:  `Perform branch operations like create/delete`,
	}

	branchCmd.AddCommand(CreateCommand(ctx))
	branchCmd.AddCommand(DeleteCommand(ctx))
	return branchCmd
}

var branchName string
var refBranchName string
var commitSHA string
var repoNames []string

func CreateCommand(ctx *context.Context) *cobra.Command {
	var createCmd = &cobra.Command{
		Use:     "create",
		Short:   "Create a new branch from a existing branch",
		Long:    `Create a new branch from a existing branch for given repos or all the selected reps in the given org/owner`,
		Aliases: []string{"add"},
		Example: heredoc.Doc(`
			$ sgh branch create --branch Release-1.1 --head Release-1.0 --org sample-org
			$ sgh branch create --branch Release-1.1 --commit da500aa4f54cbf8f3eb47a1dc2c136715c9197b9 --org sample-org --repo sample-repo1
			$ sgh branch create --branch Release-1.1 --head Release-1.0 --org sample-org -r sample-repo1 -r sample-repo2
		`),

		Args: func(cmd *cobra.Command, args []string) error {
			if commitSHA != "" && len(repoNames) != 1 {
				return fmt.Errorf("if commitSHA is specified then only one repository name is allowed")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if orgName == "" {
				logger.Glog.Error().Msgf("Organization name is required")
				cmd.Help()
				return
			}
			if commitSHA != "" {
				branchResponses := branch.CreateNewBranchFromCommit(ctx, orgName, branchName, commitSHA, repoNames[0])
				if len(branchResponses) > 0 {
					ui.PrintResponses(branchResponses)
				}
				return
			}
			branchResponses := branch.CreateNewBranches(ctx, orgName, branchName, refBranchName, repoNames)
			if len(branchResponses) > 0 {
				ui.PrintResponses(branchResponses)
			}
		},
	}

	createCmd.Flags().StringVarP(&branchName, "branch", "B", "", "The new `branch` which you want to be created")
	createCmd.Flags().StringVarP(&refBranchName, "head", "H", "", "The `branch` from which you want to use as reference")
	createCmd.Flags().StringVarP(&commitSHA, "commit", "c", "", "The `commit sha` from which you want to use as reference")
	createCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "The `repository` names for which you want to create the branch. If not provided, it will create for all the repositories in the organization")
	createCmd.MarkFlagRequired("branch")
	createCmd.MarkFlagsOneRequired("head", "commit")
	createCmd.MarkFlagsMutuallyExclusive("head", "commit")

	return createCmd
}

func DeleteCommand(ctx *context.Context) *cobra.Command {
	var deleteCmd = &cobra.Command{
		Use:     "delete",
		Short:   "Delete a new branch.",
		Long:    `Delete a new branch for given repos or all the selected repos in the given org/owner.`,
		Aliases: []string{"rm"},
		Example: heredoc.Doc(`
			$ sgh branch delete --branch Release-1.0 --org sample-org
			$ sgh branch delete --branch Release-1.0 --org sample-org -r sample-repo1 -r sample-repo2
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			repos, _ := cmd.Flags().GetStringArray("repository")
			if orgName == "" {
				logger.Glog.Error().Msgf("Organization name is required")
				cmd.Help()
				return
			}
			branchResponses := branch.DeleteBranches(ctx, orgName, branchName, repos)
			if len(branchResponses) > 0 {
				ui.PrintResponses(branchResponses)
			}
		},
	}

	deleteCmd.Flags().StringVarP(&branchName, "branch", "B", "", "The `branch` which you want to be deleted")
	deleteCmd.MarkFlagRequired("branch")
	deleteCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "The `repository` names for which you want to delete the branch. If not provided, it will delete for all the repositories in the organization")
	return deleteCmd
}
