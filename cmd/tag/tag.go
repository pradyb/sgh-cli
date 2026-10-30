// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package tag

import (
	"fmt"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	wf "github.com/pradyb/sgh-cli/cmd/workflow"
	"github.com/pradyb/sgh-cli/internal/processor"
	"github.com/pradyb/sgh-cli/pkg/context"
	"github.com/pradyb/sgh-cli/pkg/logger"
	"github.com/pradyb/sgh-cli/pkg/tag"
	"github.com/pradyb/sgh-cli/pkg/ui"
	"github.com/pradyb/sgh-cli/pkg/workflow"
	"github.com/pradyb/sgh-cli/utils"
)

var (
	tagName          string
	refBranchName    string
	repoNames        []string
	excludeRepoNames []string
	message          string
	sortBy           string
	filter           string
)

func NewTagCommand(ctx *context.Context) *cobra.Command {
	tagCmd := &cobra.Command{
		Use:     "tag <command>",
		Short:   "List, create, and delete tags",
		Long:    `Perform tag operations like list/create/delete across repositories.`,
		Aliases: []string{"tg"},
	}

	tagCmd.AddCommand(ListCommand(ctx))
	tagCmd.AddCommand(CreateCommand(ctx))
	tagCmd.AddCommand(DeleteCommand(ctx))
	return tagCmd
}

func ListCommand(ctx *context.Context) *cobra.Command {
	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List tags across repositories",
		Long:    `List tags for given repos or all the selected repos in the given org/owner.`,
		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			$ sgh tag list --org sample-org
			$ sgh tag list --org sample-org -r sample-repo1 -r sample-repo2
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			req := tag.TagListRequest{
				OrgName:          orgName,
				RepoNames:        repoNames,
				ExcludeRepoNames: excludeRepoNames,
				Filter:           filter,
			}
			responses := tag.ListTags(ctx, req)
			ui.SortTags(responses, sortBy)
			if ctx.Limit > 0 && len(responses) > ctx.Limit {
				responses = responses[:ctx.Limit]
			}
			if ctx.JSON {
				ui.PrintJSON(responses)
				return
			}
			ui.PrintTags(responses, ctx.Compact, sortBy)
		},
	}

	listCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names to include")
	listCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "repository names to exclude")
	listCmd.Flags().StringVarP(&filter, "filter", "f", "", "filter tags by `name` (partial match or regex)")
	listCmd.Flags().StringVar(&sortBy, "sort", "", "sort results by: repo, tag")

	return listCmd
}

func CreateCommand(ctx *context.Context) *cobra.Command {
	var watch bool
	var watchInterval int
	var watchTimeout time.Duration

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new tag from a existing branch",
		Long: heredoc.Doc(`
			Create a new tag from a existing branch for given repos or all the selected reps in the given org/owner

			With --watch, follow the run(s) each new tag triggers (typically a release workflow), one
			repository at a time, like 'workflow view --watch' (--watch --json streams NDJSON). The
			runs are matched by the tag, so an earlier run for the same commit on a branch is never
			followed; a repository with no workflow for tag pushes is reported and skipped after a minute.
		`),
		Aliases: []string{"add"},
		Example: heredoc.Doc(`
			$ sgh tag create --org sample-org --tag Release-1.0 --head Release-1.0 -m 'Tag for Release 1.0'
			$ sgh tag create --org sample-org --tag Release-1.0 --head Release-1.0 -m 'Tag for Release 1.0' -r sample-repo1 -r sample-repo2
			$ sgh tag create --org sample-org --tag v1.0.0 --head main -m 'Release 1.0.0' -r sample-repo1 --watch --timeout 30m
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if orgName == "" {
				logger.Glog.Error().Msgf("Organization name is required")
				cmd.Help()
				return
			}
			if (watchTimeout != 0 || cmd.Flags().Changed("interval")) && !watch {
				fmt.Fprintln(cmd.ErrOrStderr(), "  ✗ --interval and --timeout require --watch")
				ctx.HasError = true
				return
			}
			if watch && !wf.ValidateFollowInterval(cmd, ctx, watchInterval) {
				return
			}
			if ctx.DryRun {
				ui.PrintDryRunBanner()
				repos, _ := processor.ResolveRepositoryNames(ctx, orgName, repoNames, excludeRepoNames)
				ui.PrintDryRunActions("Create Tag", orgName, repos, map[string]string{
					"Tag": tagName, "From Branch": refBranchName, "Message": message,
				})
				return
			}
			req := tag.TagCreateRequest{
				OrgName:          orgName,
				RepoNames:        repoNames,
				ExcludeRepoNames: excludeRepoNames,
				TagName:          tagName,
				RefBranchName:    refBranchName,
				Message:          message,
			}
			if watch && ctx.JSON {
				ctx.Silent = true // the progress bar and summary banner would land on stdout
			}
			responses := tag.CreateNewTags(ctx, req)
			if !watch {
				ui.PrintResponses(responses)
				return
			}
			if ctx.JSON {
				for _, r := range responses {
					if r.IsSuccess() {
						wf.EmitFollowEvent("tag_created", r.RepositoryName, 0, r.CommitSHA, "")
					} else {
						wf.EmitFollowEvent("tag_error", r.RepositoryName, 0, "", r.ErrorMessage)
					}
				}
			} else {
				ui.PrintResponses(responses)
			}
			for _, r := range responses {
				if !r.IsSuccess() {
					ctx.HasError = true
					continue
				}
				// head_branch is the tag name for a tag push; matching it keeps the run a branch
				// push of the same commit already started from being mistaken for this tag's run.
				wf.FollowCommitRuns(cmd, ctx, workflow.FindRunsRequest{
					OrgName: orgName, RepoName: r.RepositoryName, HeadSHA: r.CommitSHA, Event: "push", Branch: tagName,
				}, wf.FollowOptions{Interval: watchInterval, Timeout: watchTimeout})
			}
		},
	}

	createCmd.Flags().BoolVarP(&watch, "watch", "W", false, "follow the run(s) each new tag triggers until they complete")
	createCmd.Flags().IntVar(&watchInterval, "interval", 10, "polling interval in seconds when using --watch (minimum 5)")
	createCmd.Flags().DurationVar(&watchTimeout, "timeout", 0, "give up on each watched run after this long when using --watch, e.g. 30m (0 = no timeout)")
	createCmd.Flags().StringVarP(&tagName, "tag", "t", "", "The new `tag` which you want to be created")
	createCmd.Flags().StringVarP(&refBranchName, "head", "H", "", "The `branch` from which you want to use as reference")
	createCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "The `repository` names for which you want to create the tag. If not provided, it will create for all the repositories in the organization")
	createCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "The `repository` names which you want to exclude from creating the tag")
	createCmd.Flags().StringVarP(&message, "message", "m", "", "The `message` for the tagging")

	createCmd.MarkFlagRequired("tag")
	createCmd.MarkFlagRequired("head")
	createCmd.MarkFlagRequired("message")
	return createCmd
}

func DeleteCommand(ctx *context.Context) *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete a tag across repositories",
		Long:    `Delete a tag for given repos or all the selected repos in the given org/owner`,
		Aliases: []string{"rm"},
		Example: heredoc.Doc(`
			$ sgh tag delete --Tag Release-1.0 --org sample-org
			$ sgh tag delete --tag Release-1.0 --org sample-org -r sample-repo1 -r sample-repo2
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if orgName == "" {
				logger.Glog.Error().Msgf("Organization name is required")
				cmd.Help()
				return
			}
			if ctx.DryRun {
				ui.PrintDryRunBanner()
				repos, _ := processor.ResolveRepositoryNames(ctx, orgName, repoNames, excludeRepoNames)
				ui.PrintDryRunActions("Delete Tag", orgName, repos, map[string]string{"Tag": tagName})
				return
			}
			req := tag.TagDeleteRequest{
				OrgName:          orgName,
				RepoNames:        repoNames,
				ExcludeRepoNames: excludeRepoNames,
				TagName:          tagName,
			}
			responses := tag.DeleteTags(ctx, req)
			ui.PrintResponses(responses)
		},
	}

	deleteCmd.Flags().StringVarP(&tagName, "tag", "t", "", "The `tag` which you want to be deleted")
	deleteCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names")
	deleteCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "The `repository` names which you want to exclude from deleting the tag")

	deleteCmd.MarkFlagRequired("tag")
	return deleteCmd
}
