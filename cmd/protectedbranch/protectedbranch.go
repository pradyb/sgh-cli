// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package protectedbranch

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/pradyb/sgh-cli/internal/processor"
	"github.com/pradyb/sgh-cli/pkg/context"
	pb "github.com/pradyb/sgh-cli/pkg/protectedbranch"
	"github.com/pradyb/sgh-cli/pkg/ui"
	"github.com/pradyb/sgh-cli/utils"
)

func NewProtectedBranchCommand(ctx *context.Context) *cobra.Command {
	pbCmd := &cobra.Command{
		Use:     "protected-branch <command>",
		Aliases: []string{"pb"},
		Short:   "Perform operations like list/update/delete protected branches.",
		Long:    `Perform operations like list/update/delete protected branches.`,
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
			if ctx.JSON {
				ui.PrintJSON(branchResponses)
				return
			}
			ui.PrintProtectedBranches(branchResponses)
		},
	}

	listCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "The `repository` names for which you want to list the protected branches. If not provided, it will list for all the repositories in the organization")
	listCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "The `repository` names to exclude from listing the protected branches")
	listCmd.Flags().StringVarP(&branchName, "branch", "b", "", "filter by `branch` name (optional; omit to list all protected branches)")

	return listCmd
}

var (
	lock               bool
	removeStatusChecks bool
	addBypassUsers     []string
	removeBypassUsers  []string
	addPushUsers       []string
	removePushUsers    []string
)

func UpdateCommand(ctx *context.Context) *cobra.Command {
	updateCmd := &cobra.Command{
		Use:     "update",
		Short:   "Update protected branches",
		Long:    `Update protected branches for given repos or all the selected reps in the given org/owner`,
		Aliases: []string{"edit"},
		Example: heredoc.Doc(`
			$ sgh protected-branch update --org sample-org --branch sample-branch
			$ sgh protected-branch update --org sample-org --branch sample-branch -r sample-repo1 -r sample-repo2
			$ sgh protected-branch update --org sample-org --branch sample-branch -r sample-repo1 --lock --remove-status-checks
			$ sgh protected-branch update --org sample-org --branch sample-branch -r sample-repo1 --lock --add-bypass-user john-doe --add-push-user jane-doe
			$ sgh protected-branch update --org sample-org --branch sample-branch -r sample-repo1 --remove-bypass-user john-doe --remove-push-user jane-doe
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			// Resolve repo names once so fuzzy matching and its warning appear only once
			resolvedRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, repoNames)
			if ctx.DryRun {
				ui.PrintDryRunBanner()
				repos, _ := processor.ResolveRepositoryNames(ctx, orgName, resolvedRepoNames, excludeRepoNames)
				details := map[string]string{"Branch": branchName}
				if lock {
					details["Lock"] = "true"
				}
				if removeStatusChecks {
					details["Remove Status Checks"] = "true"
				}
				if len(addBypassUsers) > 0 {
					details["Add Bypass Users"] = strings.Join(addBypassUsers, ", ")
				}
				if len(removeBypassUsers) > 0 {
					details["Remove Bypass Users"] = strings.Join(removeBypassUsers, ", ")
				}
				if len(addPushUsers) > 0 {
					details["Add Push Users"] = strings.Join(addPushUsers, ", ")
				}
				if len(removePushUsers) > 0 {
					details["Remove Push Users"] = strings.Join(removePushUsers, ", ")
				}
				ui.PrintDryRunActions("Update Protected Branch", orgName, repos, details)
				return
			}
			pb.UpdateProtectedBranch(ctx, pb.ProtectedBranchRequest{OrgName: orgName, RepoNames: resolvedRepoNames, BranchName: branchName, Lock: lock, RemoveStatus: removeStatusChecks, AddBypassUsers: addBypassUsers, RemoveBypassUsers: removeBypassUsers, AddPushUsers: addPushUsers, RemovePushUsers: removePushUsers}, excludeRepoNames)
			fmt.Println()
			branchResponses := pb.ListProtectedBranches(ctx, orgName, resolvedRepoNames, excludeRepoNames, branchName)
			ui.PrintProtectedBranches(branchResponses)
		},
	}

	updateCmd.Flags().StringVarP(&branchName, "branch", "b", "", "The `branch` for which you want to update the protected branch")
	updateCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "The `repository` names for which you want to update the protected branches. If not provided, it will update for all the repositories in the organization")
	updateCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "The `repository` names to exclude from updating the protected branches")
	updateCmd.Flags().BoolVarP(&lock, "lock", "l", false, "lock the protected branch")
	updateCmd.Flags().BoolVarP(&removeStatusChecks, "remove-status-checks", "d", false, "remove all required status checks from the protected branch")
	updateCmd.Flags().StringArrayVar(&addBypassUsers, "add-bypass-user", []string{}, "add user(s) to bypass required pull requests")
	updateCmd.Flags().StringArrayVar(&removeBypassUsers, "remove-bypass-user", []string{}, "remove user(s) from bypass required pull requests")
	updateCmd.Flags().StringArrayVar(&addPushUsers, "add-push-user", []string{}, "specify user(s) allowed to push to matching branches")
	updateCmd.Flags().StringArrayVar(&removePushUsers, "remove-push-user", []string{}, "remove user(s) allowed to push to matching branches")

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
			if ctx.DryRun {
				ui.PrintDryRunBanner()
				repos, _ := processor.ResolveRepositoryNames(ctx, orgName, repoNames, excludeRepoNames)
				ui.PrintDryRunActions("Delete Protected Branch", orgName, repos, map[string]string{"Branch": branchName})
				return
			}
			branchResponses := pb.DeleteProtectedBranch(ctx, orgName, repoNames, excludeRepoNames, branchName)
			ui.PrintResponses(branchResponses)
		},
	}

	deleteCmd.Flags().StringVarP(&branchName, "branch", "b", "", "The `branch` for which you want to delete the protected branch")
	deleteCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "The `repository` names for which you want to delete the protected branches. If not provided, it will delete for all the repositories in the organization")
	deleteCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "The `repository` names to exclude from deleting the protected branches")

	deleteCmd.MarkFlagRequired("branch")
	return deleteCmd
}
