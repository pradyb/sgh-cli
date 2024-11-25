package ui

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"github.com/prady-lab/sgh-cli/internal/model"
)

var (
	re = lipgloss.NewRenderer(os.Stdout)

	HeaderStyle  = re.NewStyle().Padding(0, 1).Foreground(White).Bold(true).Align(lipgloss.Center)
	FooterStyle  = re.NewStyle().Padding(0, 1).Foreground(Turquoise).Bold(true).Align(lipgloss.Center)
	CellStyle    = re.NewStyle().Padding(0, 1)
	OddRowStyle  = CellStyle.Foreground(Gray)
	EvenRowStyle = CellStyle.Foreground(LightGray)
	BorderStyle  = lipgloss.NewStyle().Foreground(White)

	CommitMessageStyle = lipgloss.NewStyle().Foreground(White).Bold(true)
	CommitAuthorStyle  = lipgloss.NewStyle().Foreground(Gray).Italic(true)
	CommitDateStyle    = lipgloss.NewStyle().Foreground(Gray).Italic(true)
	CommitShaStyle     = lipgloss.NewStyle().Foreground(LightGray).Italic(true)
)

type TableRowType interface {
	model.Repository | model.RefUIResponse | model.ProtectedBranch | model.OrgTeam
}

type rowsConvertHandler[T TableRowType] func(data T) []string

func convertToRows[T TableRowType](data []T, rowsConvertHandler rowsConvertHandler[T]) [][]string {
	rows := make([][]string, 0, len(data))
	for _, d := range data {
		row := rowsConvertHandler(d)
		if len(row) > 0 {
			rows = append(rows, row)
		}
	}
	return rows
}

func defaultTableStyle(row, col, totalRows, repoColIndex int, isFooterPresent bool) lipgloss.Style {
	var style lipgloss.Style
	switch {
	case row == -1:
		return HeaderStyle
	case row%2 == 0:
		style = EvenRowStyle
	default:
		style = OddRowStyle
	}

	if col == repoColIndex {
		// style = style.Width(22)
		style = style.Foreground(Green)
	}

	if isFooterPresent && row == totalRows-1 {
		style = FooterStyle
	}

	return style
}

func printNoDataMessage(message string) {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		PaddingTop(1).
		PaddingLeft(2).
		PaddingBottom(1)

	fmt.Println(style.Render(message))
}

func printErrorMessageMap(errorMessageMap map[string][]string) {
	if len(errorMessageMap) > 0 {
		fmt.Println()
		fmt.Println("Failed to process the request the following repositories")
		for errorMessage, repos := range errorMessageMap {
			fmt.Println(lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(Red)).
				Render(errorMessage))

			fmt.Println(lipgloss.NewStyle().
				Italic(true).
				Render(strings.Join(repos, "\n")))
		}
		fmt.Println()
	}
}

func PrintRepositories(repos []model.Repository) {
	rows := convertToRows(repos, func(repo model.Repository) []string {
		prCount := strconv.Itoa(repo.OpenPullRequestsCount)
		if repo.OpenPullRequestsCount != 0 {
			prCount = fmt.Sprintf(HyperLinkFormat, repo.HTMLUrl+"/pulls", prCount)
		}
		/*issueCount := strconv.Itoa(repo.OpenIssuesCount)
		if repo.OpenIssuesCount != 0 {
			issueCount = fmt.Sprintf(HyperLinkFormat, repo.HTMLUrl+"/issues", issueCount)
		}*/

		return []string{
			repo.Name,
			repo.Description,
			repo.DefaultBranch,
			repo.Language,
			strconv.FormatBool(repo.Private),
			repo.SSHUrl,
			fmt.Sprintf(HyperLinkFormat, repo.HTMLUrl, "Link"),
			prCount,
			// issueCount,
		}
	})

	rows = append(rows, []string{"Total Repositories", strconv.Itoa(len(repos))})

	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := defaultTableStyle(row, col, len(rows), 0, true)

			if col == 1 {
				style = style.Width(50)
			}
			if row > -1 && row < len(rows)-1 {
				if (col == 7 || col == 8) && rows[row][col] != "0" {
					style = style.Foreground(lipgloss.Color(Red))
				}
			}
			return style
		}).
		// Headers(repositoryNameDisplayName, "Description", "Default branch", "Language", "Is Private", "SSH URL", "HTML Page", "Open PRs", "Open Issues").
		Headers(repositoryNameDisplayName, "Description", "Default branch", "Language", "Is Private", "SSH URL", "HTML Page", "Open PRs").
		Rows(rows...)

	fmt.Println(t)
}

func PrintResponses(responses []model.RefUIResponse) {
	if len(responses) == 0 {
		printNoDataMessage("No data found for the given input")
		return
	}
	failedRows := make([][]string, 0)
	rows := convertToRows(responses, func(response model.RefUIResponse) []string {
		if response.ErrorMessage != "" {
			failedRows = append(failedRows, []string{response.RepositoryName, response.ErrorMessage})
			return []string{}
		}
		return []string{response.RepositoryName, response.SuccessMessage}
	})

	if len(rows) > 0 {
		rows = append(rows, []string{"Total items", strconv.Itoa(len(rows))})
		fmt.Println()
		t := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(BorderStyle).
			BorderRow(true).
			StyleFunc(func(row, col int) lipgloss.Style {
				style := defaultTableStyle(row, col, len(rows), 0, true)
				return style
			}).
			Headers(repositoryNameDisplayName, "Status Message").
			Rows(rows...)

		fmt.Println(t)
	}

	if len(failedRows) > 0 {
		failedRows = append(failedRows, []string{"Total items", strconv.Itoa(len(failedRows))})
		fmt.Println()
		fmt.Println("Failed to process following repositories")
		t := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(BorderStyle).
			BorderRow(true).
			StyleFunc(func(row, col int) lipgloss.Style {
				style := defaultTableStyle(row, col, len(failedRows), 0, true)
				if col == 1 && row != -1 && row != len(failedRows)-1 {
					style = style.Foreground(lipgloss.Color(Red))
				}
				return style
			}).
			Headers(repositoryNameDisplayName, errorMessageDisplayName).
			Rows(failedRows...)

		fmt.Println(t)
	}
}

func PrintPullRequestResponses(prResponses []model.PullRequestResponse) {
	if len(prResponses) == 0 {
		printNoDataMessage("No Pull Requests found for the given input")
		return
	}
	errorMessageMap := map[string][]string{}
	rows := make([][]string, 0, len(prResponses)+1)
	for _, pr := range prResponses {
		refs := fmt.Sprintf("%s <- %s", pr.Base.Ref, pr.Head.Ref)
		if pr.ErrorMessage != "" {
			errorMessageMap[pr.ErrorMessage] = append(errorMessageMap[pr.ErrorMessage], pr.RepositoryName())
		} else {
			rows = append(rows, []string{
				strconv.Itoa(pr.PRNumber),
				pr.RepositoryName(),
				pr.TitleName,
				pr.AuthorName(),
				pr.AssigneesName(),
				pr.ReviewersName(),
				pr.State + " / " + pr.MergeStateStatus,
				refs,
				fmt.Sprintf(HyperLinkFormat, pr.HTMLUrl, "Open"),
			})
		}
	}

	if len(rows) > 0 {
		rows = append(rows, []string{"", "Total Pull Requests", strconv.Itoa(len(prResponses))})
		fmt.Println()
		t := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(BorderStyle).
			BorderRow(true).
			StyleFunc(func(row, col int) lipgloss.Style {
				return pullRequestStyle(row, col, rows)
			}).
			Headers("Id", repositoryNameDisplayName, "Title", "Created User", "Assignees", "Reviewers", "Status/Merge State", "Refs", "HTMLUrl").
			Rows(rows...)

		fmt.Println(t)
	}

	printErrorMessageMap(errorMessageMap)
}

func pullRequestStyle(row int, col int, rows [][]string) lipgloss.Style {
	style := defaultTableStyle(row, col, len(rows), 1, true)

	if col == 2 {
		style = style.Width(40)
	}
	if row > 0 && row < len(rows)-1 {
		if col == 6 && rows[row][6] == "closed" {
			style = style.Strikethrough(true).Foreground(lipgloss.Color("#BFD641"))
		} else if col == 6 {
			style = style.Foreground(lipgloss.Color("#DFC57B"))
		}
	}
	return style
}

func PrintMergeResponses(mergeResponses []model.MergeResponse) {
	if len(mergeResponses) == 0 {
		printNoDataMessage("No Merge Responses found for the given input")
		return
	}
	rows := make([][]string, 0, len(mergeResponses)+1)
	for _, merge := range mergeResponses {
		message := merge.Message
		if merge.ErrorMessage != "" {
			message = merge.ErrorMessage
		}
		rows = append(rows, []string{merge.RepositoryName, message, merge.SHA})
	}
	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := defaultTableStyle(row, col, len(mergeResponses), 0, true)
			if row > 0 && row < len(rows) {
				if col == 1 && strings.Contains(rows[row][col], "documentation_url") {
					style = style.Foreground(lipgloss.Color(Red))
				}
			}
			return style
		}).
		Headers(repositoryNameDisplayName, "Message", "Sha").
		Rows(rows...)

	fmt.Println(t)
}

func PrintProtectedBranches(pbResponses []model.ProtectedBranch) {
	if len(pbResponses) == 0 {
		printNoDataMessage("No Protected Branches found for the given input")
		return
	}
	rows, failedRows := getProtectedBranches(pbResponses)
	rows = append(rows, []string{"Total Protected Branches", strconv.Itoa(len(rows))})

	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := defaultTableStyle(row, col, len(rows), 0, true)
			return style
		}).
		Headers(repositoryNameDisplayName, "Type", "Reviewers", "Code Owner Reviews", "Last Push Approval", "Dismiss Stale reviews", "Status Checks", "Lock branch", "Bypass allowed Users", "Restrictions Users", "Rule set names").
		Rows(rows...)

	fmt.Println(t)

	if len(failedRows) > 0 {
		fmt.Println()
		fmt.Println("Failed to process following repositories")
		t = table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(BorderStyle).
			BorderRow(true).
			StyleFunc(func(row, col int) lipgloss.Style {
				var style lipgloss.Style
				if col == 1 && row != -1 {
					style = style.Foreground(lipgloss.Color(Red))
				}
				return style
			}).
			Headers(repositoryNameDisplayName, errorMessageDisplayName).
			Rows(failedRows...)

		fmt.Println(t)
	}
}

func getProtectedBranches(pbResponses []model.ProtectedBranch) ([][]string, [][]string) {
	sort.Slice(pbResponses, func(i, j int) bool {
		if pbResponses[i].Type == pbResponses[j].Type {
			return pbResponses[i].RepositoryName < pbResponses[j].RepositoryName
		}
		return pbResponses[i].Type < pbResponses[j].Type
	})

	failedRows := make([][]string, 0)
	rows := convertToRows(pbResponses, func(pb model.ProtectedBranch) []string {
		var bypassUsers []string
		var restrictionUsers []string
		for _, user := range pb.RequiredPullRequestReviews.BypassPullRequestAllowances.Users {
			bypassUsers = append(bypassUsers, user.Name)
		}
		for _, user := range pb.Restrictions.Users {
			restrictionUsers = append(restrictionUsers, user.Name)
		}
		lockBranch := strconv.FormatBool(pb.LockBranch)
		if len(pb.RepositoryRulesetNames) != 0 {
			lockBranch = ""
		}
		if pb.ErrorMessage == "" {
			if pb.Type == "NA" {
				return []string{
					pb.RepositoryName,
					pb.Type,
				}
			} else {
				return []string{
					pb.RepositoryName,
					pb.Type,
					strconv.Itoa(pb.RequiredPullRequestReviews.RequiredApprovingReviewCount),
					strconv.FormatBool(pb.RequiredPullRequestReviews.RequireCodeOwnerReviews),
					strconv.FormatBool(pb.RequiredPullRequestReviews.RequireLastPushApproval),
					strconv.FormatBool(pb.RequiredPullRequestReviews.DismissStaleReviews),
					strings.Join(pb.RequiredStatusChecks.Contexts, ","),
					lockBranch,
					strings.Join(bypassUsers, "\n"),
					strings.Join(restrictionUsers, "\n"),
					strings.Join(pb.RepositoryRulesetNames, "\n"),
				}
			}
		} else {
			failedRows = append(failedRows, []string{pb.RepositoryName, pb.ErrorMessage})
			return []string{}
		}
	})
	return rows, failedRows
}

func PrintPostReleaseResponses(prResponses []model.PostReleaseResponse) {
	if len(prResponses) == 0 {
		printNoDataMessage("No Post Release activity performed for the given input")
		return
	}
	errorMessageMap := map[string][]string{}
	rows := make([][]string, 0, len(prResponses)+1)
	for _, pr := range prResponses {
		if pr.ErrorMessage != "" {
			errorMessageMap[pr.ErrorMessage] = append(errorMessageMap[pr.ErrorMessage], pr.RepositoryName)
		} else {
			rows = append(rows, []string{
				strconv.Itoa(pr.PRNumber),
				pr.RepositoryName,
				fmt.Sprintf(HyperLinkFormat, pr.PRHtmlUrl, "PR"),
				fmt.Sprintf(HyperLinkFormat, pr.TagHtmlUrl, "Tag"),
				pr.TagCommitSHA,
			})
		}
	}

	if len(rows) > 0 {

		rows = append(rows, []string{"", "Total", strconv.Itoa(len(rows))})
		fmt.Println()
		t := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(BorderStyle).
			BorderRow(true).
			StyleFunc(func(row, col int) lipgloss.Style {
				style := defaultTableStyle(row, col, len(prResponses), 1, true)
				return style
			}).
			Headers("PR #", repositoryNameDisplayName, "PR URL", "Tag URL", "Tag CommitSha").
			Rows(rows...)

		fmt.Println(t)
	}

	printErrorMessageMap(errorMessageMap)
}

func PrintCommitResponses(commitResponses []model.CommitResponse, includeMergeCommits bool) {
	if len(commitResponses) == 0 {
		printNoDataMessage("No Commits found for the given input")
		return
	}
	rows, failedRows := getCommitResponses(commitResponses, includeMergeCommits)

	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := defaultTableStyle(row, col, len(rows), 0, true)

			if col == 1 && row != -1 && row != len(rows)-1 {
				style = style.UnsetForeground()
			}
			if col == 0 && row != -1 && row != len(rows)-1 {
				style = style.Foreground(Green)
			}

			return style
		}).
		Headers(repositoryNameDisplayName, "Author Name", "Author Date", "Message", "Comment Count", "Url").
		Rows(rows...)

	fmt.Println(t)

	if len(failedRows) > 0 {
		fmt.Println()
		fmt.Println("Failed to fetch details the following repositories")
		t = table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(BorderStyle).
			BorderRow(true).
			StyleFunc(func(row, col int) lipgloss.Style {
				var style lipgloss.Style
				if col == 1 && row != -1 {
					style = style.Foreground(lipgloss.Color(Red))
				}
				return style
			}).
			Headers(repositoryNameDisplayName, errorMessageDisplayName).
			Rows(failedRows...)

		fmt.Println(t)
	}
}

func getCommitResponses(commitResponses []model.CommitResponse, includeMergeCommits bool) ([][]string, [][]string) {
	rows := make([][]string, 0, len(commitResponses))
	failedRows := make([][]string, 0)
	sort.Slice(commitResponses, func(i, j int) bool {
		dateI, _ := time.Parse(time.RFC3339, commitResponses[i].Commit.Author.Date)
		dateJ, _ := time.Parse(time.RFC3339, commitResponses[j].Commit.Author.Date)
		return dateI.After(dateJ)
	})
	for _, commit := range commitResponses {
		if includeMergeCommits || !includeMergeCommits && commit.Commit.Committer.Name != "GitHub" {
			if commit.ErrorMessage != "" {
				failedRows = append(failedRows, []string{commit.RepositoryName, commit.ErrorMessage})
			} else {
				rows = append(rows, []string{
					commit.RepoName(),
					commit.Commit.Author.Name,
					commit.Commit.Author.Date,
					commit.Commit.Message,
					strconv.Itoa(commit.Commit.CommentCount),
					fmt.Sprintf(HyperLinkFormat, commit.HtmlUrl, "Link"),
				})
			}
		}
	}
	rows = append(rows, []string{"Total Commits", strconv.Itoa(len(commitResponses))})
	return rows, failedRows
}

func PrintCommitSummary(commitResponses []model.CommitResponse, includeMergeCommits bool) {
	if len(commitResponses) == 0 {
		printNoDataMessage("No Commits found for the given input")
		return
	}
	repoCommits := getRepoCommitsMap(commitResponses, includeMergeCommits)
	keys := make([]string, 0, len(repoCommits))
	for k := range repoCommits {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows := make([][]string, 0, len(repoCommits)+1)
	for _, repo := range keys {
		commits := repoCommits[repo]
		t := table.New().Border(lipgloss.HiddenBorder())
		t.StyleFunc(func(row, col int) lipgloss.Style {
			var style lipgloss.Style
			style.Padding(1)
			if col != 0 {
				style = style.Italic(true)
			} else {
				style = style.Bold(true)
			}
			return style
		})
		commitsRows := make([][]string, 0, len(commits))
		for _, commit := range commits {
			commitsRows = append(commitsRows, []string{
				commit.Commit.Message,
				commit.Commit.Author.Name,
				commit.Commit.Author.Date,
				fmt.Sprintf(HyperLinkFormat, commit.HtmlUrl, commit.Commit.Tree.Sha[:7]),
			})
		}
		t.Rows(commitsRows...)
		rows = append(rows, []string{repo, strconv.Itoa(len(commits)), t.Render()})
	}

	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			var style lipgloss.Style
			if row == -1 {
				return HeaderStyle
			}
			if col == 0 {
				style = CellStyle.Foreground(lipgloss.Color(Gray)).AlignVertical(lipgloss.Center)
			} else if col == 1 {
				style = CellStyle.Foreground(lipgloss.Color(Gray)).Align(lipgloss.Center, lipgloss.Center)
			}

			return style
		}).
		Headers(repositoryNameDisplayName, "Total", "Commit Messages").
		Rows(rows...)

	fmt.Println(t)
}

func getRepoCommitsMap(commitResponses []model.CommitResponse, includeMergeCommits bool) map[string][]model.CommitResponse {
	repoCommits := make(map[string][]model.CommitResponse)
	for _, commit := range commitResponses {
		if commit.ErrorMessage == "" {
			if includeMergeCommits || !includeMergeCommits && commit.Commit.Committer.Name != "GitHub" {
				repoCommits[commit.RepoName()] = append(repoCommits[commit.RepoName()], commit)
			}
		}
	}
	return repoCommits
}

func PrintTeams(teams []model.OrgTeam) {
	if len(teams) == 0 {
		printNoDataMessage("No Teams found for the given input")
		return
	}
	rows := convertToRows(teams, func(team model.OrgTeam) []string {
		members := make([]string, 0, len(team.Members))
		for _, member := range team.Members {
			members = append(members, fmt.Sprintf(HyperLinkFormat, member.PeopleUrl, member.Name))
		}
		return []string{
			fmt.Sprintf(HyperLinkFormat, team.Url, team.Name),
			strconv.Itoa(team.TotalMembers),
			strings.Join(members, "\n"),
			strconv.Itoa(team.RepositoriesCount),
		}
	})

	rows = append(rows, []string{"Total Teams", strconv.Itoa(len(teams))})

	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := CellStyle
			if row == -1 {
				return HeaderStyle
			}
			if col == 0 {
				style = CellStyle.Foreground(lipgloss.Color(Gray)).AlignVertical(lipgloss.Center)
			} else if col == 1 || col == 3 {
				style = CellStyle.Foreground(lipgloss.Color(Gray)).Align(lipgloss.Center, lipgloss.Center)
			}
			if row == len(rows)-1 {
				style = FooterStyle
			}
			return style
		}).
		Headers("Team Name", "Total Members", "Members", "Repositories Count").
		Rows(rows...)

	fmt.Println(t)
}
