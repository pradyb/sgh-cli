package commit

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/prady-lab/sgh-cli/pkg/commit"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/ui"
	logger "github.com/prady-lab/sgh-cli/utils"

	"github.com/spf13/cobra"
)

func NewCommitCommand(ctx *context.Context) *cobra.Command {

	var commitCmd = &cobra.Command{
		Use:   "commit <command>",
		Short: "Manage commits",
		Long:  `Perform Commits operations like list/commit status`,
	}

	commitCmd.AddCommand(ListCommand(ctx))
	return commitCmd
}

var repoNames []string
var branchName string
var noOfDays int
var details bool
var includeMergeCommits bool

func ListCommand(ctx *context.Context) *cobra.Command {
	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "List commits for all the repositories in the given org/owner",
		Long: `List commits on GitHub for given repos or all the selected reps in the given org/owner
Default fetches all commits for past 3 days, use -n flag to fetch commits for specific number of days (since n days)`,

		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			$ sgh commit list --org sample-org
			$ sgh commit list --org sample-org -r sample-repo1 -r sample-repo2
			$ sgh commit list --org sample-org -r sample-repo1 -r sample-repo2 -n 5"
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			responses := commit.ListCommits(ctx, orgName, repoNames, branchName, noOfDays)
			if details {
				logger.Glog.Info().Msgf("Printing commit responses for past %d days", noOfDays)
				ui.PrintCommitResponses(responses, includeMergeCommits)
			} else {
				logger.Glog.Info().Msgf("Printing commit summary for past %d days", noOfDays)
				ui.PrintCommitSummary(responses, includeMergeCommits)
			}
		},
	}

	listCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names")
	listCmd.Flags().StringVarP(&branchName, "branch", "b", "main", "The `branch` for which you want to fetch commits")
	listCmd.Flags().IntVarP(&noOfDays, "days", "n", 3, "number of days to fetch commits")
	listCmd.Flags().BoolVarP(&details, "details", "d", false, "show detailed commit information")
	listCmd.Flags().BoolVarP(&includeMergeCommits, "include-merge-commits", "i", false, "include merge commits")

	listCmd.MarkPersistentFlagRequired("org")
	listCmd.MarkFlagRequired("branch")
	return listCmd
}
