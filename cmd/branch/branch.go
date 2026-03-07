package branch

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/pkg/branch"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/prady-lab/sgh-cli/pkg/ui"
	"github.com/prady-lab/sgh-cli/utils"
)

func NewBranchCommand(ctx *context.Context) *cobra.Command {
	branchCmd := &cobra.Command{
		Use:     "branch <command>",
		Short:   "List, create, and delete branches",
		Long:    `Perform branch operations like list/create/delete across repositories.`,
		Aliases: []string{"br"},
	}

	branchCmd.AddCommand(ListCommand(ctx))
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
	filter           string
	sortBy           string
)

func ListCommand(ctx *context.Context) *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List branches across repositories",
		Long: `List branches for given repos or all the selected repos in the given org/owner.
Supports filtering by branch name using partial match or regex pattern.`,
		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			$ sgh branch list --org sample-org
			$ sgh branch list --org sample-org --filter "Release-*"
			$ sgh branch list --org sample-org --filter "feature/" -r sample-repo1
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			req := branch.BranchListRequest{
				OrgName:          orgName,
				RepoNames:        repoNames,
				ExcludeRepoNames: excludeRepoNames,
				Filter:           filter,
			}
			responses := branch.ListBranches(ctx, req)
			ui.SortBranches(responses, sortBy)
			if ctx.Limit > 0 && len(responses) > ctx.Limit {
				responses = responses[:ctx.Limit]
			}
			if ctx.JSON {
				ui.PrintJSON(responses)
				return
			}
			ui.PrintBranches(responses, orgName, ctx.Compact, sortBy)
		},
	}

	listCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names to include")
	listCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "repository names to exclude")
	listCmd.Flags().StringVarP(&filter, "filter", "f", "", "filter branches by `name` (partial match or regex)")
	listCmd.Flags().StringVar(&sortBy, "sort", "", "sort results by: repo, name, protected")

	listCmd.MarkPersistentFlagRequired("org")
	return listCmd
}

func CreateCommand(ctx *context.Context) *cobra.Command {
	createCmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new branch from a existing branch",
		Long:    `Create a new branch from a existing branch for given repos or all the selected reps in the given org/owner`,
		Aliases: []string{"add"},
		Example: heredoc.Doc(`
			$ sgh branch create --org sample-org --new Release-1.1 --ref Release-1.0
			$ sgh branch create --org sample-org --new Release-1.1 --commit da500aa4f54cbf8f3eb47a1dc2c136715c9197b9 -r sample-repo1
			$ sgh branch create --org sample-org --new Release-1.1 --ref Release-1.0 -r sample-repo1 -r sample-repo2
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
			if ctx.DryRun {
				ui.PrintDryRunBanner()
				details := map[string]string{"New Branch": branchName}
				if commitSHA != "" {
					details["From Commit"] = commitSHA
					details["Repository"] = repoNames[0]
				} else {
					details["From Branch"] = refBranchName
				}
				repos, _ := processor.ResolveRepositoryNames(ctx, orgName, repoNames, excludeRepoNames)
				ui.PrintDryRunActions("Create Branch", orgName, repos, details)
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
			$ sgh branch delete --org sample-org --branch Release-1.0 -r sample-repo1 -r sample-repo2
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			repos, _ := cmd.Flags().GetStringArray("repository")
			if orgName == "" {
				logger.Glog.Error().Msgf("Organization name is required")
				cmd.Help()
				return
			}
			if ctx.DryRun {
				ui.PrintDryRunBanner()
				resolved, _ := processor.ResolveRepositoryNames(ctx, orgName, repos, excludeRepoNames)
				ui.PrintDryRunActions("Delete Branch", orgName, resolved, map[string]string{"Branch": branchName})
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
