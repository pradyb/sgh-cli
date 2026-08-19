// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package commit

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/pradyb/sgh-cli/pkg/commit"
	"github.com/pradyb/sgh-cli/pkg/context"
	"github.com/pradyb/sgh-cli/pkg/logger"
	"github.com/pradyb/sgh-cli/pkg/ui"
	"github.com/pradyb/sgh-cli/utils"
)

func repoCompletionFn(ctx *context.Context) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		orgName, _ := cmd.Root().PersistentFlags().GetString("org")
		return ctx.Config.RepositoriesNames(orgName), cobra.ShellCompDirectiveNoFileComp
	}
}

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

func ListCommand(ctx *context.Context) *cobra.Command {
	var repoNames []string
	var excludeRepos []string
	var branchName string
	var noOfDays int
	var since string
	var until string
	var details bool
	var includeMergeCommits bool
	var sortBy string
	var compact bool

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
			$ sgh commit list --org sample-org --since 2026-08-01
			$ sgh commit list --org sample-org --since 2026-08-01 --until 2026-08-15
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			req := commit.CommitListRequest{
				OrgName:      orgName,
				RepoNames:    repoNames,
				ExcludeRepos: excludeRepos,
				BranchName:   branchName,
				NoOfDays:     noOfDays,
				Since:        since,
				Until:        until,
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
				ui.PrintCommitResponses(responses, includeMergeCommits, sortBy, compact)
			} else {
				logger.Flog.Info().Msgf("Printing commit summary for past %d days", noOfDays)
				ui.PrintCommitSummary(responses, includeMergeCommits, sortBy)
			}
		},
	}

	listCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names to include")
	listCmd.Flags().StringArrayVarP(&excludeRepos, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "repository names to exclude")
	listCmd.RegisterFlagCompletionFunc("repository", repoCompletionFn(ctx))
	listCmd.Flags().StringVarP(&branchName, "branch", "b", "", "The `branch` for which you want to fetch commits (defaults to each repo's default branch)")
	listCmd.Flags().IntVarP(&noOfDays, "days", "n", 3, "number of past days to fetch commits (ignored when --since is set)")
	listCmd.Flags().StringVar(&since, "since", "", "fetch commits on or after `date` (YYYY-MM-DD); overrides --days")
	listCmd.Flags().StringVar(&until, "until", "", "fetch commits on or before `date` (YYYY-MM-DD)")
	listCmd.Flags().BoolVarP(&details, "details", "d", false, "show detailed commit information")
	listCmd.Flags().BoolVarP(&includeMergeCommits, "include-merge-commits", "m", false, "include merge commits")
	listCmd.Flags().StringVarP(&sortBy, "sort", "s", "date", "sort commits by: date, repo, author")
	listCmd.Flags().BoolVarP(&compact, "compact", "c", false, "compact tab-separated output (useful for piping)")

	return listCmd
}
