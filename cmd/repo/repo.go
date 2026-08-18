// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/prady-lab/sgh-cli/pkg/repo"
	"github.com/prady-lab/sgh-cli/pkg/ui"
	"github.com/prady-lab/sgh-cli/utils"
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
		Short: "List, search, archive, and manage repository visibility",
		Long:  `List, search, archive, and manage visibility for repositories in an organization.`,
		Example: heredoc.Doc(`
			$ sgh repo list --org sample-org
			$ sgh repo search --org sample-org --query "api"
			$ sgh repo archive --org sample-org -r old-repo1 -r old-repo2
			$ sgh repo visibility --org sample-org -r my-repo --visibility private
		`),
	}

	repoCmd.AddCommand(ListCommand(ctx))
	repoCmd.AddCommand(SearchCommand(ctx))
	repoCmd.AddCommand(ArchiveCommand(ctx))
	repoCmd.AddCommand(VisibilityCommand(ctx))
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
					"Hint: use the --all flag to list all repositories without config filtering.",
					"Hint: if this owner is missing from config, add it: sgh config add org "+orgName,
					"Hint: verify the organization/username is correct.")
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

func ArchiveCommand(ctx *context.Context) *cobra.Command {
	var repoNames []string
	var excludeRepoNames []string
	var unarchive bool

	archiveCmd := &cobra.Command{
		Use:   "archive",
		Short: "Archive or unarchive repositories",
		Long: heredoc.Doc(`
			Archive or unarchive repositories in the organization.

			Archived repositories become read-only and are hidden from default views.
			Use --unarchive to reverse the operation.
		`),
		Example: heredoc.Doc(`
			$ sgh repo archive --org my-org -r old-service -r legacy-api
			$ sgh repo archive --org my-org --unarchive -r old-service
			$ sgh repo archive --org my-org -e active-repo1 -e active-repo2
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			archive := !unarchive

			if ctx.DryRun {
				resolved, err := processor.ResolveRepositoryNames(ctx, orgName, repoNames, excludeRepoNames)
				if err != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), ui.ErrorMessage("failed to resolve repositories: %v", err))
					return
				}
				ui.PrintDryRunBanner()
				action := "Archive"
				if !archive {
					action = "Unarchive"
				}
				ui.PrintDryRunActions(action+" Repositories", orgName, resolved, nil)
				return
			}

			responses := repo.ArchiveRepos(ctx, orgName, repoNames, excludeRepoNames, archive)
			ui.PrintResponses(responses)
		},
	}

	archiveCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names to include")
	archiveCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "repository names to exclude")
	archiveCmd.Flags().BoolVarP(&unarchive, "unarchive", "u", false, "unarchive instead of archive")

	return archiveCmd
}

func VisibilityCommand(ctx *context.Context) *cobra.Command {
	var repoNames []string
	var excludeRepoNames []string
	var visibility string

	visibilityCmd := &cobra.Command{
		Use:   "visibility",
		Short: "Set repository visibility (public or private)",
		Long: heredoc.Doc(`
			Change the visibility of repositories in the organization.

			Valid values for --visibility: public, private
			Note: changing visibility may have billing and security implications.
		`),
		Example: heredoc.Doc(`
			$ sgh repo visibility --org my-org -r my-repo --visibility private
			$ sgh repo visibility --org my-org -r internal-tool --visibility public
			$ sgh repo visibility --org my-org -e keep-public --visibility private
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")

			if visibility != "public" && visibility != "private" {
				fmt.Fprintln(cmd.ErrOrStderr(), ui.ErrorMessage("--visibility must be 'public' or 'private', got: %q", visibility))
				return
			}

			if ctx.DryRun {
				resolved, err := processor.ResolveRepositoryNames(ctx, orgName, repoNames, excludeRepoNames)
				if err != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), ui.ErrorMessage("failed to resolve repositories: %v", err))
					return
				}
				ui.PrintDryRunBanner()
				ui.PrintDryRunActions("Set Visibility", orgName, resolved, map[string]string{"Visibility": visibility})
				return
			}

			responses := repo.SetRepoVisibility(ctx, orgName, repoNames, excludeRepoNames, visibility)
			ui.PrintResponses(responses)
		},
	}

	visibilityCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names to include")
	visibilityCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "repository names to exclude")
	visibilityCmd.Flags().StringVarP(&visibility, "visibility", "V", "", "`visibility` to set: public or private")

	visibilityCmd.MarkFlagRequired("visibility")

	return visibilityCmd
}
