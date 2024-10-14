package repo

import (
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/prady-lab/sgh-cli/pkg/repo"
	"github.com/prady-lab/sgh-cli/pkg/ui"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
)

var isAllRepos bool

func NewRepoCommand(ctx *context.Context) *cobra.Command {

	var repoCmd = &cobra.Command{
		Use:   "repo <command>",
		Short: "Repo operations for the given organization",
		Long:  `Repo operations for the given organization`,
		Example: heredoc.Doc(`
			$ sgh repo list <owner>
		`),
	}

	repoCmd.AddCommand(listCommand(ctx))
	return repoCmd
}

func listCommand(ctx *context.Context) *cobra.Command {
	var listCmd = &cobra.Command{
		Use:     "list <owner> -a",
		Short:   "List all the selected repositories for the given owner/organization",
		Long:    `List all the selected repositories for the given owner/organization`,
		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			$ sgh repo list <owner>
			$ sgh repo list <owner> -a
		`),
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			orgName := args[0]
			logger.Glog.Info().Msgf("Listing the Repositories for the %s owner/organization", orgName)
			repositories, err := repo.GetReposForOrg(ctx, orgName, isAllRepos)
			if err != nil {
				logger.Glog.Error().Err(err).Msgf("Error in getting the Repos for the organization %s", orgName)
			}
			if len(repositories) > 0 {
				ui.PrintRepositories(repositories)
			} else {
				logger.Glog.Info().Msgf("No Repositories found for the organization %s", orgName)
			}
		},
	}

	listCmd.Flags().BoolVarP(&isAllRepos, "all", "a", false, "list all repos")

	return listCmd
}
