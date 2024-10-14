package clone

import (
	"github.com/prady-lab/sgh-cli/pkg/context"

	"github.com/MakeNowJust/heredoc"
	"github.com/prady-lab/sgh-cli/pkg/clone"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/spf13/cobra"
)

var branch string
var repoNames []string

func NewCloneCommand(ctx *context.Context) *cobra.Command {
	var cloneCmd = &cobra.Command{
		Use:     "clone",
		Short:   "Clone all the selected repositories for the given owner/organization",
		Long:    `Clone all the selected repositories for the given owner/organization.`,
		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			$ sgh clone -o sample-org
			$ sgh clone -o sample-org -b <branch>
			$ sgh clone -o sample-org -b <branch> -r sample-repo1 -r sample-repo2
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			logger.Glog.Info().Msgf("Cloning the Repositories for the %s owner/organization", orgName)
			err := clone.CloneRepositories(ctx, orgName, repoNames, branch)
			if err != nil {
				logger.Glog.Error().Err(err).Msgf("Error in getting the Repos for the organization %s", orgName)
			}

		},
	}

	cloneCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "The `repository` names for which you want to create the branch. If not provided, it will create for all the repositories in the organization")
	cloneCmd.Flags().StringVarP(&branch, "branch", "b", "", "The `branch` for which you want to clone the repositories")

	cloneCmd.MarkPersistentFlagRequired("org")
	return cloneCmd
}
