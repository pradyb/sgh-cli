package protectedbranch

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/prady-lab/sgh-cli/pkg/context"
	pb "github.com/prady-lab/sgh-cli/pkg/protected_branch"
	"github.com/prady-lab/sgh-cli/pkg/ui"

	"github.com/spf13/cobra"
)

func NewProtectedBranchCommand(ctx *context.Context) *cobra.Command {

	var pbCmd = &cobra.Command{
		Use:   "pb <command>",
		Short: "Manage protected branches",
		Long:  `Perform operations like list/update/delete protected branches.`,
	}

	pbCmd.AddCommand(ListCommand(ctx))
	pbCmd.AddCommand(UpdateCommand(ctx))
	pbCmd.AddCommand(DeleteCommand(ctx))
	return pbCmd
}

var repoNames []string
var branchName string

func ListCommand(ctx *context.Context) *cobra.Command {
	var listCmd = &cobra.Command{
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
			repoNames, _ := cmd.Flags().GetStringArray("repository")
			branchResponses := pb.ListProtectedBranches(ctx, orgName, repoNames, branchName)
			ui.PrintProtectedBranches(branchResponses)
		},
	}

	listCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "The `repository` names for which you want to list the protected branches. If not provided, it will list for all the repositories in the organization")
	listCmd.Flags().StringVarP(&branchName, "branch", "b", "", "The `branch` for which you want to list the protected branches")

	listCmd.MarkPersistentFlagRequired("org")
	listCmd.MarkFlagRequired("branch")
	return listCmd
}

var lock bool
var removeStatus bool

func UpdateCommand(ctx *context.Context) *cobra.Command {
	var updateCmd = &cobra.Command{
		Use:     "update",
		Short:   "Update protected branches",
		Long:    `Update protected branches for given repos or all the selected reps in the given org/owner`,
		Aliases: []string{"edit"},
		Example: heredoc.Doc(`
			$ sgh pb update --org sample-org --branch sample-branch 
			$ sgh pb update --org sample-org --branch sample-branch -r sample-repo1 -r sample-repo2
			$ sgh pb update --org sample-org --branch sample-branch -r sample-repo1 -l -d
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			repoNames, _ := cmd.Flags().GetStringArray("repository")
			branchName, _ := cmd.Flags().GetString("branch")
			branchResponses := pb.UpdateProtectedBranch(ctx, orgName, repoNames, branchName, lock, removeStatus)
			ui.PrintProtectedBranches(branchResponses)
		},
	}

	updateCmd.Flags().StringVarP(&branchName, "branch", "b", "", "The `branch` for which you want to update the protected branch")
	updateCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "The `repository` names for which you want to update the protected branches. If not provided, it will update for all the repositories in the organization")
	updateCmd.Flags().BoolVarP(&lock, "lock", "l", false, "lock the protected branch")
	updateCmd.Flags().BoolVarP(&removeStatus, "delete", "d", false, "remove the status checks in protected branch")

	updateCmd.MarkPersistentFlagRequired("org")
	updateCmd.MarkFlagRequired("branch")
	return updateCmd
}

func DeleteCommand(ctx *context.Context) *cobra.Command {
	var deleteCmd = &cobra.Command{
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
			branchResponses := pb.DeleteProtectedBranch(ctx, orgName, repoNames, branchName)
			ui.PrintResponses(branchResponses)
		},
	}

	deleteCmd.Flags().StringVarP(&branchName, "branch", "b", "", "The `branch` for which you want to delete the protected branch")
	deleteCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "The `repository` names for which you want to delete the protected branches. If not provided, it will delete for all the repositories in the organization")

	deleteCmd.MarkPersistentFlagRequired("org")
	deleteCmd.MarkFlagRequired("branch")
	return deleteCmd
}
