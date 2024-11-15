package protectedbranch

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/prady-lab/sgh-cli/pkg/context"
	pb "github.com/prady-lab/sgh-cli/pkg/protected_branch"
	"github.com/prady-lab/sgh-cli/pkg/ui"
	"github.com/prady-lab/sgh-cli/utils"
)

func NewProtectedBranchCommand(ctx *context.Context) *cobra.Command {
	pbCmd := &cobra.Command{
		Use:   "pb <command>",
		Short: "Manage protected branches",
		Long:  `Perform operations like list/update/delete protected branches.`,
	}

	pbCmd.AddCommand(ListCommand(ctx))
	pbCmd.AddCommand(UpdateCommand(ctx))
	pbCmd.AddCommand(DeleteCommand(ctx))
	return pbCmd
}

var (
	repoNames        []string
	excludeRepoNames []string
	branchName       string
)

func ListCommand(ctx *context.Context) *cobra.Command {
	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List protected branches",
		Long:    `List protected branches for given repos or all the selected reps in the given org/owner`,
		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			$ sgh pb list --org sample-org --branch sample-branch
			$ sgh pb list --org sample-org --branch sample-branch -r sample-repo1 -r sample-repo2
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			branchResponses := pb.ListProtectedBranches(ctx, orgName, repoNames, excludeRepoNames, branchName)
			ui.PrintProtectedBranches(branchResponses)
		},
	}

	listCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "The `repository` names for which you want to list the protected branches. If not provided, it will list for all the repositories in the organization")
	listCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "The `repository` names to exclude from listing the protected branches")
	listCmd.Flags().StringVarP(&branchName, "branch", "b", "", "The `branch` for which you want to list the protected branches")

	listCmd.MarkPersistentFlagRequired("org")
	listCmd.MarkFlagRequired("branch")
	return listCmd
}

var (
	lock         bool
	removeStatus bool
	addUsers     []string
	removeUsers  []string
)

func UpdateCommand(ctx *context.Context) *cobra.Command {
	updateCmd := &cobra.Command{
		Use:     "update",
		Short:   "Update protected branches",
		Long:    `Update protected branches for given repos or all the selected reps in the given org/owner`,
		Aliases: []string{"edit"},
		Example: heredoc.Doc(`
			$ sgh pb update --org sample-org --branch sample-branch 
			$ sgh pb update --org sample-org --branch sample-branch -r sample-repo1 -r sample-repo2
			$ sgh pb update --org sample-org --branch sample-branch -r sample-repo1 -l -d
			$ sgh pb update --org sample-org --branch sample-branch -r sample-repo1 -l -d -a john-doe -a jane-doe
			$ sgh pb update --org sample-org --branch sample-branch -r sample-repo1 -l -d -u john-doe -u jane-doe
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			pb.UpdateProtectedBranch(ctx, pb.ProtectedBranchRequest{OrgName: orgName, RepoNames: repoNames, BranchName: branchName, Lock: lock, RemoveStatus: removeStatus, AddUsers: addUsers, RemoveUsers: removeUsers}, excludeRepoNames)
			branchResponses := pb.ListProtectedBranches(ctx, orgName, repoNames, excludeRepoNames, branchName)
			ui.PrintProtectedBranches(branchResponses)
		},
	}

	updateCmd.Flags().StringVarP(&branchName, "branch", "b", "", "The `branch` for which you want to update the protected branch")
	updateCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "The `repository` names for which you want to update the protected branches. If not provided, it will update for all the repositories in the organization")
	updateCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "The `repository` names to exclude from updating the protected branches")
	updateCmd.Flags().BoolVarP(&lock, "lock", "l", false, "lock the protected branch")
	updateCmd.Flags().BoolVarP(&removeStatus, "delete", "d", false, "remove the status checks in protected branch")
	updateCmd.Flags().StringArrayVarP(&addUsers, "add-user", "a", []string{}, "add user(s) to the protected branch")
	updateCmd.Flags().StringArrayVarP(&removeUsers, "remove-user", "u", []string{}, "remove user(s) from the protected branch")

	updateCmd.MarkPersistentFlagRequired("org")
	updateCmd.MarkFlagRequired("branch")
	return updateCmd
}

func DeleteCommand(ctx *context.Context) *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete protected branches",
		Long:    `Delete protected branches for given repos or all the selected reps in the given org/owner`,
		Aliases: []string{"rm"},
		Example: heredoc.Doc(`
			$ sgh pb delete --org sample-org --branch sample-branch
			$ sgh pb delete --org sample-org --branch sample-branch -r sample-repo1 -r sample-repo2
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			branchResponses := pb.DeleteProtectedBranch(ctx, orgName, repoNames, excludeRepoNames, branchName)
			ui.PrintResponses(branchResponses)
		},
	}

	deleteCmd.Flags().StringVarP(&branchName, "branch", "b", "", "The `branch` for which you want to delete the protected branch")
	deleteCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "The `repository` names for which you want to delete the protected branches. If not provided, it will delete for all the repositories in the organization")
	deleteCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "The `repository` names to exclude from deleting the protected branches")

	deleteCmd.MarkPersistentFlagRequired("org")
	deleteCmd.MarkFlagRequired("branch")
	return deleteCmd
}
