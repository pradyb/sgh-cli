// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package clone

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/prady-lab/sgh-cli/pkg/clone"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
)

var (
	branch    string
	repoNames []string
)

func NewCloneCommand(ctx *context.Context) *cobra.Command {
	cloneCmd := &cobra.Command{
		Use:   "clone",
		Short: "Clone all the selected repositories for the given owner/organization",
		Long:  `Clone all the selected repositories for the given owner/organization.`,
		Example: heredoc.Doc(`
			$ sgh clone --org sample-org
			$ sgh clone --org sample-org --branch <branch>
			$ sgh clone --org sample-org --branch <branch> -r sample-repo1 -r sample-repo2
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			logger.Flog.Info().Msgf("Cloning the Repositories for the %s owner/organization", orgName)
			err := clone.CloneRepositories(ctx, orgName, repoNames, branch)
			if err != nil {
				logger.Glog.Error().Err(err).Msgf("Error in getting the Repos for the organization %s", orgName)
			}
		},
	}

	cloneCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "The `repository` names to clone. If not provided, it will clone all repositories in the organization")
	cloneCmd.Flags().StringVarP(&branch, "branch", "b", "", "The `branch` for which you want to clone the repositories")

	return cloneCmd
}
