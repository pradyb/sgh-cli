// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package commit

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/prady-lab/sgh-cli/pkg/commit"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/prady-lab/sgh-cli/pkg/ui"
	"github.com/prady-lab/sgh-cli/utils"
)

func NewCommitCommand(ctx *context.Context) *cobra.Command {
	commitCmd := &cobra.Command{
		Use:     "commit <command>",
		Aliases: []string{"ci"},
		Short:   "List recent commits for all the repositories",
		Long:    `List recent commits for all the repositories`,
	}

	commitCmd.AddCommand(ListCommand(ctx))
	return commitCmd
}

var (
	repoNames           []string
	excludeRepos        []string
	branchName          string
	noOfDays            int
	details             bool
	includeMergeCommits bool
)

func ListCommand(ctx *context.Context) *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List commits for all the repositories in the given org/owner",
		Long: `List commits on GitHub for given repos or all the selected repos in the given org/owner.
Default fetches all commits for the past 3 days, use -n flag to fetch commits for a specific number of days.
If --branch is omitted, each repository's default branch is used.`,

		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			$ sgh commit list --org sample-org
			$ sgh commit list --org sample-org -r sample-repo1 -r sample-repo2
			$ sgh commit list --org sample-org -r sample-repo1 -r sample-repo2 -n 5
			$ sgh commit list --org sample-org -e sample-repo3 -e sample-repo4
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			req := commit.CommitListRequest{
				OrgName:      orgName,
				RepoNames:    repoNames,
				ExcludeRepos: excludeRepos,
				BranchName:   branchName,
				NoOfDays:     noOfDays,
			}
			responses := commit.ListCommits(ctx, req)
			if ctx.Limit > 0 && len(responses) > ctx.Limit {
				responses = responses[:ctx.Limit]
			}
			if ctx.JSON {
				ui.PrintJSON(responses)
				return
			}
			if details {
				logger.Flog.Info().Msgf("Printing commit responses for past %d days", noOfDays)
				ui.PrintCommitResponses(responses, includeMergeCommits)
			} else {
				logger.Flog.Info().Msgf("Printing commit summary for past %d days", noOfDays)
				ui.PrintCommitSummary(responses, includeMergeCommits)
			}
		},
	}

	listCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names to include")
	listCmd.Flags().StringArrayVarP(&excludeRepos, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "repository names to exclude")
	listCmd.Flags().StringVarP(&branchName, "branch", "b", "", "The `branch` for which you want to fetch commits (defaults to each repo's default branch)")
	listCmd.Flags().IntVarP(&noOfDays, "days", "n", 3, "number of days to fetch commits")
	listCmd.Flags().BoolVarP(&details, "details", "d", false, "show detailed commit information")
	listCmd.Flags().BoolVarP(&includeMergeCommits, "include-merge-commits", "M", false, "include merge commits")

	return listCmd
}
