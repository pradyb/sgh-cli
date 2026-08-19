// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package postrelease

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/pradyb/sgh-cli/internal/processor"
	"github.com/pradyb/sgh-cli/pkg/context"
	postrelease "github.com/pradyb/sgh-cli/pkg/postrelease"
	"github.com/pradyb/sgh-cli/pkg/ui"
	"github.com/pradyb/sgh-cli/utils"
)

var (
	ref          string
	branchName   string
	tagName      string
	message      string
	repoNames    []string
	excludeRepos []string
)

func NewPostReleaseCommand(ctx *context.Context) *cobra.Command {
	postReleaseCmd := &cobra.Command{
		Use:   "post-release",
		Short: "Create a hotfix branch and/or release tag across repositories",
		Long: heredoc.Doc(`
			Automate post-release activities across multiple repositories in an organization.

			Creates a hotfix branch and/or a release tag from a given source branch.
			At least one of --branch or --tag must be provided.

			Note: --ref must be an existing branch name (not a tag or SHA).
		`),
		Example: heredoc.Doc(`
			# Create a hotfix branch and tag from main
			$ sgh post-release --org my-org --ref main --branch hotfix/1.0.1 --tag v1.0.1 --message "Hotfix 1.0.1"

			# Create only a hotfix branch from a release branch
			$ sgh post-release --org my-org --ref release/1.0 --branch hotfix/1.0.1

			# Create only a release tag on main
			$ sgh post-release --org my-org --ref main --tag v2.0.0 --message "Release 2.0.0"

			# Scope to specific repositories
			$ sgh post-release --org my-org --ref main --branch hotfix/1.0.1 --tag v1.0.1 -r repo1 -r repo2

			# Exclude specific repositories
			$ sgh post-release --org my-org --ref main --tag v1.0.1 -e legacy-repo
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")

			if ctx.DryRun {
				ui.PrintDryRunBanner()
				repos, _ := processor.ResolveRepositoryNames(ctx, orgName, repoNames, excludeRepos)
				details := map[string]string{
					"Source Ref": ref,
				}
				if branchName != "" {
					details["Hotfix Branch"] = branchName
				}
				if tagName != "" {
					details["Tag"] = tagName
					details["Message"] = message
				}
				ui.PrintDryRunActions("Post Release", orgName, repos, details)
				return
			}

			responses := postrelease.ProcessPostRelease(ctx, postrelease.PostReleaseRequest{
				OrgName:      orgName,
				RepoNames:    repoNames,
				ExcludeRepos: excludeRepos,
				Ref:          ref,
				BranchName:   branchName,
				TagName:      tagName,
				Message:      message,
			})
			ui.PrintPostReleaseResponses(responses)
		},
	}

	postReleaseCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names to include")
	postReleaseCmd.Flags().StringArrayVarP(&excludeRepos, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "repository names to exclude")
	postReleaseCmd.Flags().StringVarP(&ref, "ref", "R", "", "source `branch` to create the hotfix branch and/or tag from")
	postReleaseCmd.Flags().StringVarP(&branchName, "branch", "b", "", "name of the hotfix `branch` to create")
	postReleaseCmd.Flags().StringVarP(&tagName, "tag", "t", "", "name of the release `tag` to create")
	postReleaseCmd.Flags().StringVarP(&message, "message", "m", "", "tag annotation message (defaults to tag name if omitted)")

	postReleaseCmd.MarkFlagRequired("ref")
	postReleaseCmd.MarkFlagsOneRequired("branch", "tag")

	return postReleaseCmd
}
