// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package issue

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/issue"
	"github.com/prady-lab/sgh-cli/pkg/ui"
	"github.com/prady-lab/sgh-cli/utils"
)

var (
	repoNames        []string
	excludeRepoNames []string
	issueState       string
	labels           string
	assignee         string
	creator          string
	lastCount        int
	sortBy           string
)

func NewIssueCommand(ctx *context.Context) *cobra.Command {
	issueCmd := &cobra.Command{
		Use:     "issue <command>",
		Aliases: []string{"is"},
		Short:   "List, view, and create issues across repositories",
		Long:    `Perform issue operations like list, view, and create across repositories.`,
	}

	issueCmd.AddCommand(ListCommand(ctx))
	issueCmd.AddCommand(ViewCommand(ctx))
	issueCmd.AddCommand(CreateCommand(ctx))
	return issueCmd
}

func ListCommand(ctx *context.Context) *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List issues across repositories",
		Long: `List issues for given repos or all selected repos in the organization.
Supports filtering by state (open, closed, all), labels, assignee, and creator.
By default fetches open issues.`,
		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			$ sgh issue list --org sample-org
			$ sgh issue list --org sample-org --state all
			$ sgh issue list --org sample-org --state closed
			$ sgh issue list --org sample-org -r sample-repo1 -r sample-repo2
			$ sgh issue list --org sample-org --labels "bug,enhancement"
			$ sgh issue list --org sample-org --assignee "john-doe"
			$ sgh issue list --org sample-org --creator "jane-doe"
			$ sgh issue list --org sample-org --sort state
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")

			req := issue.IssueListRequest{
				OrgName:          orgName,
				RepoNames:        repoNames,
				ExcludeRepoNames: excludeRepoNames,
				State:            issueState,
				Labels:           labels,
				Assignee:         assignee,
				Creator:          creator,
				LastCount:        lastCount,
			}
			issues := issue.ListIssues(ctx, req)
			ui.SortIssues(issues, sortBy)
			if ctx.Limit > 0 && len(issues) > ctx.Limit {
				issues = issues[:ctx.Limit]
			}
			if ctx.JSON {
				ui.PrintJSON(issues)
				return
			}
			ui.PrintIssues(issues, sortBy, ctx.Compact)
		},
	}

	listCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names to include")
	listCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "repository names to exclude")
	listCmd.Flags().StringVarP(&issueState, "state", "s", "open", "filter by `state`: open, closed, all")
	listCmd.Flags().StringVar(&labels, "labels", "", "filter by comma-separated `labels`")
	listCmd.Flags().StringVarP(&assignee, "assignee", "a", "", "filter by `assignee` login")
	listCmd.Flags().StringVar(&creator, "creator", "", "filter by `creator` login")
	listCmd.Flags().IntVarP(&lastCount, "last", "l", 30, "the `number` of issues to fetch per repo")
	listCmd.Flags().StringVar(&sortBy, "sort", "", "sort results by: repo, title, author, state, created")

	return listCmd
}

func ViewCommand(ctx *context.Context) *cobra.Command {
	var viewRepo string
	var issueNumber int

	viewCmd := &cobra.Command{
		Use:     "view",
		Short:   "View issue details",
		Long:    `View detailed information about a specific issue including body and comments.`,
		Aliases: []string{"detail", "info"},
		Example: heredoc.Doc(`
			$ sgh issue view --org sample-org -r sample-repo --issue 42
			$ sgh issue view --org sample-org -r sample-repo --issue 42 --json
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")

			req := issue.IssueViewRequest{
				OrgName:     orgName,
				RepoName:    viewRepo,
				IssueNumber: issueNumber,
			}
			result := issue.GetIssue(ctx, req)
			if ctx.JSON {
				ui.PrintJSON(result)
				return
			}
			if result.ErrorMessage != "" {
				ui.PrintIssueDetail(result, nil)
				return
			}
			comments := issue.GetIssueComments(ctx, orgName, result.RepositoryName, issueNumber)
			ui.PrintIssueDetail(result, comments)
		},
	}

	viewCmd.Flags().StringVarP(&viewRepo, "repository", "r", "", "repository `name`")
	viewCmd.Flags().IntVarP(&issueNumber, "issue", "i", 0, "issue `number`")

	viewCmd.MarkFlagRequired("repository")
	viewCmd.MarkFlagRequired("issue")

	return viewCmd
}

func CreateCommand(ctx *context.Context) *cobra.Command {
	var createRepo string
	var issueTitle string
	var issueBody string
	var issueAssignee string
	var issueLabels string

	createCmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new issue in a repository",
		Long:    `Create a new GitHub issue in the specified repository.`,
		Aliases: []string{"add", "new"},
		Example: heredoc.Doc(`
			$ sgh issue create --org sample-org -r sample-repo --title "Bug: login fails"
			$ sgh issue create --org sample-org -r sample-repo --title "Feature request" --body "Please add dark mode" --assignee john-doe
			$ sgh issue create --org sample-org -r sample-repo --title "Fix crash" --labels "bug,high-priority"
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")

			var labels []string
			if issueLabels != "" {
				for _, l := range strings.Split(issueLabels, ",") {
					if trimmed := strings.TrimSpace(l); trimmed != "" {
						labels = append(labels, trimmed)
					}
				}
			}

			req := issue.IssueCreateRequest{
				OrgName:  orgName,
				RepoName: createRepo,
				Title:    issueTitle,
				Body:     issueBody,
				Assignee: issueAssignee,
				Labels:   labels,
			}
			result := issue.CreateIssue(ctx, req)
			if ctx.JSON {
				ui.PrintJSON(result)
				return
			}
			if result.ErrorMessage != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), ui.ErrorMessage("%s", result.ErrorMessage))
				return
			}
			fmt.Printf("  ✓ Issue #%d created in %s\n",
				result.Number,
				result.RepositoryName,
			)
			if result.HTMLUrl != "" {
				fmt.Printf("  %s\n", result.HTMLUrl)
			}
		},
	}

	createCmd.Flags().StringVarP(&createRepo, "repository", "r", "", "repository `name`")
	createCmd.Flags().StringVarP(&issueTitle, "title", "t", "", "issue `title`")
	createCmd.Flags().StringVarP(&issueBody, "body", "b", "", "issue `body` (description)")
	createCmd.Flags().StringVarP(&issueAssignee, "assignee", "a", "", "assign issue to `user` login")
	createCmd.Flags().StringVar(&issueLabels, "labels", "", "comma-separated `labels` to apply")

	createCmd.MarkFlagRequired("repository")
	createCmd.MarkFlagRequired("title")

	return createCmd
}
