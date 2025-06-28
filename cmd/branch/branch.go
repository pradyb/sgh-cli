package branch

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/prady-lab/sgh-cli/pkg/branch"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/prady-lab/sgh-cli/pkg/ui"
	"github.com/prady-lab/sgh-cli/utils"
)

func NewBranchCommand(ctx *context.Context) *cobra.Command {
	branchCmd := &cobra.Command{
		Use:   "branch <command>",
		Short: "Create and delete branches",
		Long:  `Perform branch operations like create/delete`,
	}

	branchCmd.AddCommand(CreateCommand(ctx))
	branchCmd.AddCommand(DeleteCommand(ctx))
	return branchCmd
}

var (
	branchName       string
	refBranchName    string
	commitSHA        string
	repoNames        []string
	excludeRepoNames []string
)

func CreateCommand(ctx *context.Context) *cobra.Command {
	createCmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new branch from a existing branch",
		Long:    `Create a new branch from a existing branch for given repos or all the selected reps in the given org/owner`,
		Aliases: []string{"add"},
		Example: heredoc.Doc(`
			$ sgh branch create --org sample-org --new Release-1.1 --ref Release-1.0
			$ sgh branch create --org sample-org --new Release-1.1 --commit da500aa4f54cbf8f3eb47a1dc2c136715c9197b9 --repo sample-repo1
			$ sgh branch create --org sample-org --new Release-1.1 --ref Release-1.0 --repo sample-repo1 --repo sample-repo2
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
				req := branch.BranchCreateFromCommitRequest{
					OrgName:       orgName,
					RepoName:      repoNames[0],
					NewBranchName: branchName,
					CommitSHA:     commitSHA,
				}
				branchResponses := branch.CreateNewBranchFromCommit(ctx, req)
				ui.PrintResponses(branchResponses)
				return
			}
			req := branch.BranchCreateRequest{
				OrgName:          orgName,
				RepoNames:        repoNames,
				ExcludeRepoNames: excludeRepoNames,
				NewBranchName:    branchName,
				RefBranchName:    refBranchName,
			}
			branchResponses := branch.CreateNewBranches(ctx, req)
			ui.PrintResponses(branchResponses)
		},
	}

	createCmd.Flags().StringVarP(&branchName, "new", "N", "", "The new `branch` which you want to be created")
	createCmd.Flags().StringVarP(&refBranchName, "ref", "R", "", "The `branch` from which you want to use as reference")
	createCmd.Flags().StringVarP(&commitSHA, "commit", "c", "", "The `commit sha` from which you want to use as reference")
	createCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "The `repository` names for which you want to create the branch. If not provided, it will create for all the repositories in the organization")
	createCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "The `repository` names to exclude from branch creation")

	createCmd.MarkPersistentFlagRequired("org")
	createCmd.MarkFlagRequired("new")
	createCmd.MarkFlagsOneRequired("ref", "commit")
	createCmd.MarkFlagsMutuallyExclusive("ref", "commit")

	return createCmd
}

func DeleteCommand(ctx *context.Context) *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete a new branch.",
		Long:    `Delete a new branch for given repositories or all the selected repositories in the given organization/owner.`,
		Aliases: []string{"rm"},
		Example: heredoc.Doc(`
			$ sgh branch delete --org sample-org --branch Release-1.0 
			$ sgh branch delete --org sample-org --branch Release-1.0 --repo sample-repo1 --repo sample-repo2
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			repos, _ := cmd.Flags().GetStringArray("repository")
			if orgName == "" {
				logger.Glog.Error().Msgf("Organization name is required")
				cmd.Help()
				return
			}
			req := branch.BranchDeleteRequest{
				OrgName:          orgName,
				RepoNames:        repos,
				ExcludeRepoNames: excludeRepoNames,
				BranchName:       branchName,
			}
			branchResponses := branch.DeleteBranches(ctx, req)
			ui.PrintResponses(branchResponses)
		},
	}

	deleteCmd.Flags().StringVarP(&branchName, "branch", "B", "", "The `branch` which you want to be deleted")
	deleteCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "The `repository` names for which you want to delete the branch. If not provided, it will delete for all the repositories in the organization")
	deleteCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "The `repository` names to exclude from branch deletion")

	deleteCmd.MarkPersistentFlagRequired("org")
	deleteCmd.MarkFlagRequired("branch")
	return deleteCmd
}
