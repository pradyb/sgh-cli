// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package repo

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/prady-lab/sgh-cli/pkg/repo"
	"github.com/prady-lab/sgh-cli/pkg/ui"
)

var (
	isAllRepos  bool
	searchQuery string
	language    string
	topic       string
)

func NewRepoCommand(ctx *context.Context) *cobra.Command {
	repoCmd := &cobra.Command{
		Use:   "repo <command>",
		Short: "List and search repositories for the given owner/organization",
		Long:  `List and search repositories for the given owner/organization.`,
		Example: heredoc.Doc(`
			$ sgh repo list --org sample-org
			$ sgh repo search --org sample-org --query "api"
		`),
	}

	repoCmd.AddCommand(ListCommand(ctx))
	repoCmd.AddCommand(SearchCommand(ctx))
	return repoCmd
}

func ListCommand(ctx *context.Context) *cobra.Command {
	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List all the selected repositories for the given owner/organization",
		Long:    `List all the selected repositories for the given owner/organization.`,
		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			$ sgh repo list --org sample-org
			$ sgh repo list --org sample-org --all
		`),
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			// Backward compatibility: accept positional arg as org name
			if orgName == "" && len(args) > 0 {
				orgName = args[0]
			}
			if orgName == "" {
				logger.Glog.Error().Msg("Organization name is required")
				cmd.Help()
				return
			}
			logger.Flog.Info().Msgf("Listing the repositories for the %s owner/organization", orgName)
			repositories, err := repo.GetReposForOrg(ctx, orgName, isAllRepos)
			if err != nil {
				logger.Glog.Error().Err(err).Msgf("Error in getting the repositories for the organization %s", orgName)
				return
			}
			if len(repositories) > 0 {
				if ctx.Limit > 0 && len(repositories) > ctx.Limit {
					repositories = repositories[:ctx.Limit]
				}
				if ctx.JSON {
					ui.PrintJSON(repositories)
					return
				}
				ui.PrintRepositories(repositories)
			} else {
				ui.PrintNoDataMessage("No repositories found for "+orgName+".",
					"Hint: use the --all flag to include all repositories.",
					"Hint: verify the organization name is correct.")
			}
		},
	}

	listCmd.Flags().BoolVarP(&isAllRepos, "all", "a", false, "list all repositories")

	return listCmd
}

func SearchCommand(ctx *context.Context) *cobra.Command {
	searchCmd := &cobra.Command{
		Use:   "search",
		Short: "Search repositories by name, topic, or language",
		Long: `Search repositories within an organization using name, topic, or language filters.
The query flag searches in repository name and description.`,
		Aliases: []string{"find"},
		Example: heredoc.Doc(`
			$ sgh repo search --org sample-org --query "api"
			$ sgh repo search --org sample-org --language go
			$ sgh repo search --org sample-org --topic microservice
			$ sgh repo search --org sample-org --query "service" --language java --topic backend
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if orgName == "" {
				logger.Glog.Error().Msg("Organization name is required")
				cmd.Help()
				return
			}
			if searchQuery == "" && language == "" && topic == "" {
				logger.Glog.Error().Msg("At least one search filter is required")
				cmd.Help()
				return
			}
			repositories, err := repo.SearchRepos(ctx, orgName, searchQuery, language, topic)
			if err != nil {
				logger.Glog.Error().Err(err).Msgf("Error searching repositories in %s", orgName)
				return
			}
			if len(repositories) > 0 {
				if ctx.Limit > 0 && len(repositories) > ctx.Limit {
					repositories = repositories[:ctx.Limit]
				}
				if ctx.JSON {
					ui.PrintJSON(repositories)
					return
				}
				ui.PrintRepositories(repositories)
			} else {
				ui.PrintNoDataMessage("No repositories matched the search criteria.",
					"Hint: try a broader query or different filters.")
			}
		},
	}

	searchCmd.Flags().StringVarP(&searchQuery, "query", "q", "", "search by `name` or description (partial match)")
	searchCmd.Flags().StringVar(&language, "language", "", "filter by programming `language`")
	searchCmd.Flags().StringVar(&topic, "topic", "", "filter by repository `topic`")

	return searchCmd
}
