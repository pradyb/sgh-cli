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
	model.Repository | model.RefUIResponse | model.ProtectedBranch | model.Team
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

func defaultTableStyle(row, col, totalRows int, isFooterPresent bool) lipgloss.Style {
	var style lipgloss.Style
	switch {
	case row == 0:
		return HeaderStyle
	case row%2 == 0:
		style = EvenRowStyle
	default:
		style = OddRowStyle
	}

	if col == 1 {
		//style = style.Width(22)
		style = style.Foreground(Green)
	}

	if isFooterPresent && row == totalRows {
		style = FooterStyle
	}

	return style
}

func printNoDataMessage(message string) {
	var style = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		PaddingTop(1).
		PaddingLeft(2).
		PaddingBottom(1)

	fmt.Println(style.Render(message))
}

func PrintRepositories(repos []model.Repository) {

	rows := convertToRows(repos, func(repo model.Repository) []string {
		prCount := strconv.Itoa(repo.OpenPullRequestsCount)
		if repo.OpenPullRequestsCount != 0 {
			prCount = fmt.Sprintf(HyperLinkFormat, repo.HTMLUrl+"/pulls", prCount)
		}
		issueCount := strconv.Itoa(repo.OpenIssuesCount)
		if repo.OpenIssuesCount != 0 {
			issueCount = fmt.Sprintf(HyperLinkFormat, repo.HTMLUrl+"/issues", issueCount)
		}

		return []string{
			strconv.Itoa(repo.Id),
			repo.Name,
			repo.Description,
			repo.DefaultBranch,
			repo.Language,
			repo.SSHUrl,
			fmt.Sprintf(HyperLinkFormat, repo.HTMLUrl, "Link"),
			prCount,
			issueCount,
		}
	})

	rows = append(rows, []string{"", "Total Repositories", strconv.Itoa(len(repos))})

	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := defaultTableStyle(row, col, len(rows), true)

			if col == 2 {
				style = style.Width(40)
			}
			if row != 0 && row < len(rows) {
				if (col == 7 || col == 8) && rows[row-1][col] != "0" {
					style = style.Foreground(lipgloss.Color(Red))
				}
			}
			return style
		}).
		Headers("Id", repositoryNameDisplayName, "Description", "Default branch", "Language", "SSH URL", "HTML Page", "Open PRs", "Open Issues").
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
				style := defaultTableStyle(row, col, len(rows), true)
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
				style := defaultTableStyle(row, col, len(failedRows), true)
				if col == 1 && row != 0 && row != len(failedRows) {
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
	rows := make([][]string, 0, len(prResponses)+1)
	for _, pr := range prResponses {
		refs := fmt.Sprintf("%s <- %s", pr.Base.Ref, pr.Head.Ref)
		if pr.ErrorMessage != "" {
			rows = append(rows, []string{pr.RepositoryName(), pr.ErrorMessage})
		} else {
			rows = append(rows, []string{
				strconv.Itoa(pr.PRNumber),
				pr.RepositoryName(),
				pr.TitleName,
				pr.UserName(),
				pr.AssigneesName(),
				pr.ReviewersName(),
				pr.State,
				refs,
				fmt.Sprintf(HyperLinkFormat, pr.HTMLUrl, "Open"),
			})
		}
	}
	rows = append(rows, []string{"", "Total Pull Requests", strconv.Itoa(len(prResponses))})

	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := defaultTableStyle(row, col, len(rows), true)

			if col == 2 {
				style = style.Width(40)
			}
			if row != 0 && row < len(rows) {
				if col == 6 && rows[row-1][6] == "closed" {
					style = style.Strikethrough(true).Foreground(lipgloss.Color("#BFD641"))
				} else if col == 6 {
					style = style.Foreground(lipgloss.Color("#DFC57B"))
				}
			}
			return style
		}).
		Headers("Id", repositoryNameDisplayName, "Title", "Created User", "Assignees", "Reviewers", "Status", "Refs", "HTMLUrl").
		Rows(rows...)

	fmt.Println(t)
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
			style := defaultTableStyle(row, col, len(mergeResponses), true)
			if row != 0 && row < len(rows)+1 {
				if col == 1 && strings.Contains(rows[row-1][col], "documentation_url") {
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
	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := defaultTableStyle(row, col, len(rows), true)

			if col == 1 && row != 0 && row != len(rows) {
				style = style.UnsetForeground()
			}
			if col == 0 && row != 0 && row != len(rows) {
				style = style.Foreground(Green)
			}

			return style
		}).
		Headers(repositoryNameDisplayName, "Reviewers", "Code Owner Reviews", "Last Push Approval", "Dismiss Stale reviews", "Status Checks", "Lock branch", "Bypass allowed Users", "Restrictions Users").
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
				if col == 1 && row != 0 {
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
	failedRows := make([][]string, 0)
	rows := convertToRows(pbResponses, func(pb model.ProtectedBranch) []string {
		var bypassUsers []string
		var restrictionUsers []string
		for _, user := range pb.RequiredPullRequestReviews.BypassPullRequestAllowances.Users {
			bypassUsers = append(bypassUsers, user.Login)
		}
		for _, user := range pb.Restrictions.Users {
			restrictionUsers = append(restrictionUsers, user.Login)
		}
		if pb.ErrorMessage == "" {
			return []string{
				pb.RepositoryName,
				strconv.Itoa(pb.RequiredPullRequestReviews.RequiredApprovingReviewCount),
				strconv.FormatBool(pb.RequiredPullRequestReviews.RequireCodeOwnerReviews),
				strconv.FormatBool(pb.RequiredPullRequestReviews.RequireLastPushApproval),
				strconv.FormatBool(pb.RequiredPullRequestReviews.DismissStaleReviews),
				strings.Join(pb.RequiredStatusChecks.Contexts, ","),
				strconv.FormatBool(pb.LockBranch.Enabled),
				strings.Join(bypassUsers, ","),
				strings.Join(restrictionUsers, ","),
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
	failedRows := make([][]string, 0)
	rows := make([][]string, 0, len(prResponses)+1)
	for _, pr := range prResponses {
		if pr.ErrorMessage != "" {
			failedRows = append(failedRows, []string{pr.RepositoryName, pr.ErrorMessage})
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
	rows = append(rows, []string{"", "Total", strconv.Itoa(len(rows))})

	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := defaultTableStyle(row, col, len(prResponses), true)

			return style
		}).
		Headers("PR #", repositoryNameDisplayName, "PR URL", "Tag URL", "Tag CommitSha").
		Rows(rows...)

	fmt.Println(t)

	if len(failedRows) > 0 {
		fmt.Println()
		fmt.Println("Failed to process post release activity the following repositories")
		t = table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(BorderStyle).
			BorderRow(true).
			StyleFunc(func(row, col int) lipgloss.Style {
				var style lipgloss.Style
				if col == 1 && row != 0 {
					style = style.Foreground(lipgloss.Color(Red))
				}
				return style
			}).
			Headers(repositoryNameDisplayName, errorMessageDisplayName).
			Rows(failedRows...)

		fmt.Println(t)
	}
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
			style := defaultTableStyle(row, col, len(rows), true)

			if col == 1 && row != 0 && row != len(rows) {
				style = style.UnsetForeground()
			}
			if col == 0 && row != 0 && row != len(rows) {
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
				if col == 1 && row != 0 {
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
			if row == 0 {
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

func PrintTeams(teams []model.Team) {
	if len(teams) == 0 {
		printNoDataMessage("No Teams found for the given input")
		return
	}
	rows := convertToRows(teams, func(team model.Team) []string {
		members := make([]string, 0, len(team.Members))
		for _, member := range team.Members {
			members = append(members, fmt.Sprintf(HyperLinkFormat, member.PeopleUrl, member.Name))
		}
		return []string{
			fmt.Sprintf(HyperLinkFormat, team.Url, team.Name),
			strconv.Itoa(team.TotalMembers),
			strings.Join(members, "\n"),
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
			if row == 0 {
				return HeaderStyle
			}
			if col == 0 {
				style = CellStyle.Foreground(lipgloss.Color(Gray)).AlignVertical(lipgloss.Center)
			} else if col == 1 {
				style = CellStyle.Foreground(lipgloss.Color(Gray)).Align(lipgloss.Center, lipgloss.Center)
			}
			if row == len(rows) {
				style = FooterStyle
			}
			return style
		}).
		Headers("Team Name", "Total Members", "Members").
		Rows(rows...)

	fmt.Println(t)
}
