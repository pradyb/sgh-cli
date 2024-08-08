package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/prady-lab/sgh-cli/internal/model"
)

const (
	hyperLinkFormat = "\x1b]8;;%s\x07%s\x1b]8;;\x07\u001b[0m"

	white     = lipgloss.Color("#FFFFFF")
	gray      = lipgloss.Color("#CCC9C9")
	lightGray = lipgloss.Color("#959393")
	turquoise = lipgloss.Color("#5DE2E7")
	red       = lipgloss.Color("#FF0000")
)

var (
	re = lipgloss.NewRenderer(os.Stdout)

	HeaderStyle  = re.NewStyle().Foreground(white).Bold(true).Align(lipgloss.Center)
	CellStyle    = re.NewStyle().Padding(0, 1)
	OddRowStyle  = CellStyle.Foreground(gray)
	EvenRowStyle = CellStyle.Foreground(lightGray)
	BorderStyle  = lipgloss.NewStyle().Foreground(white)
)

type TableRowType interface {
	model.Repository | model.RefUIResponse | model.ProtectedBranch
}

type rowsConvertHandler[T TableRowType] func(data T) []string

func convertToRows[T TableRowType](data []T, rowsConvertHandler rowsConvertHandler[T]) [][]string {
	rows := make([][]string, 0, len(data))
	for _, d := range data {
		row := rowsConvertHandler(d)
		if len(row) > 0 {
			rows = append(rows, rowsConvertHandler(d))
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
		style = style.Foreground(lipgloss.Color("#00B500"))
	}

	if isFooterPresent && row == totalRows+1 {
		if col == 1 {
			style = style.Foreground(lipgloss.Color(turquoise))
		}
		if col == 2 {
			style = style.Foreground(lipgloss.Color(turquoise))
		}
	}

	return style
}

func PrintRepositories(repos []model.Repository) {

	rows := convertToRows(repos, func(repo model.Repository) []string {
		return []string{
			strconv.Itoa(repo.Id),
			repo.Name,
			repo.Description,
			repo.Language,
			repo.SSHUrl,
			strconv.Itoa(repo.OpenIssuesCount),
			strconv.Itoa(repo.Size),
		}
	})

	rows = append(rows, []string{"", "Total Repositories", strconv.Itoa(len(repos))})

	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := defaultTableStyle(row, col, len(repos), true)

			if row != 0 && row < len(rows) {
				if col == 5 && rows[row-1][col] != "0" {
					style = style.Foreground(lipgloss.Color(red))
				}
			}
			return style
		}).
		Headers("Id", "Name", "Description", "Language", "SSH URL", "Open Issues", "Size").
		Rows(rows...)

	fmt.Println(t)
}

func PrintResponses(responses []model.RefUIResponse) {

	rows := convertToRows(responses, func(response model.RefUIResponse) []string {
		message := response.SuccessMessage
		if response.ErrorMessage != "" {
			message = response.ErrorMessage
		}
		return []string{response.RepositoryName,
			message,
		}
	})

	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := defaultTableStyle(row, col, len(responses), false)

			if row != 0 && row < len(rows)+1 {
				if col == 1 && strings.Contains(rows[row-1][col], "documentation_url") {
					style = style.Foreground(lipgloss.Color(red))
				}
			}
			return style
		}).
		Headers("Repository", "Status Message").
		Rows(rows...)

	fmt.Println(t)
}

func PrintPullRequestResponses(prResponses []model.PullRequestResponse) {
	rows := make([][]string, 0, len(prResponses)+1)
	for _, pr := range prResponses {
		refs := fmt.Sprintf("%s <- %s", pr.Base.Ref, pr.Head.Ref)
		if pr.ErrorMessage != "" {
			rows = append(rows, []string{pr.RepositoryName(), pr.ErrorMessage})
		} else {
			rows = append(rows, []string{
				strconv.Itoa(pr.PRNumber),
				pr.RepositoryName(),
				pr.Title,
				pr.UserName(),
				pr.AssigneesName(),
				pr.ReviewersName(),
				pr.Status,
				refs,
				fmt.Sprintf(hyperLinkFormat, pr.HTMLUrl, "Open"),
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
			style := defaultTableStyle(row, col, len(prResponses), true)

			if row != 0 && row < len(rows) {
				if col == 6 && rows[row-1][6] == "closed" {
					style = style.Strikethrough(true).Foreground(lipgloss.Color("#BFD641"))
				} else if col == 6 {
					style = style.Foreground(lipgloss.Color("#DFC57B"))
				}
			}
			return style
		}).
		Headers("Id", "Repository", "Title", "Created User", "Assignees", "Reviewers", "Status", "Refs", "HTMLUrl").
		Rows(rows...)

	fmt.Println(t)
}

func PrintMergeResponses(mergeResponses []model.MergeResponse) {
	rows := make([][]string, 0, len(mergeResponses)+1)
	for _, merge := range mergeResponses {
		rows = append(rows, []string{
			strconv.FormatBool(merge.Merged), merge.Message, merge.SHA,
		})
	}
	rows = append(rows, []string{"", "Total Merge Requests", strconv.Itoa(len(mergeResponses))})

	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := defaultTableStyle(row, col, len(mergeResponses), true)
			return style
		}).
		Headers("Merged", "Message", "Sha").
		Rows(rows...)

	fmt.Println(t)
}

func PrintProtectedBranches(pbResponses []model.ProtectedBranch) {

	rows, failedRows := getProtectedBranches(pbResponses)
	rows = append(rows, []string{"Total Protected Branches", strconv.Itoa(len(pbResponses))})

	fmt.Println()
	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := defaultTableStyle(row, col, len(pbResponses), true)

			if col == 1 {
				style = style.UnsetForeground()
			}
			if col == 0 && row != 0 {
				style = style.Foreground(lipgloss.Color("#00B500"))
			}

			if row == len(rows) {
				if col == 0 {
					style = style.Foreground(lipgloss.Color(turquoise))
				}
				if col == 1 {
					style = style.Foreground(lipgloss.Color(turquoise))
				}
			}

			return style
		}).
		Headers("Project Name", "Reviewers", "Code Owner Reviews", "Last Push Approval", "Dismiss Stale reviews", "Status Checks", "Lock branch", "Bypass allowed Users", "Restrictions Users").
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
					style = style.Foreground(lipgloss.Color(red))
				}
				return style
			}).
			Headers("Project Name", "Error Message").
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
	rows := make([][]string, 0, len(prResponses)+1)
	for _, pr := range prResponses {
		if pr.ErrorMessage != "" {
			rows = append(rows, []string{pr.RepositoryName, pr.ErrorMessage})
		} else {
			rows = append(rows, []string{
				strconv.Itoa(pr.PRNumber),
				pr.RepositoryName,
				fmt.Sprintf(hyperLinkFormat, pr.PRHtmlUrl, "PR"),
				fmt.Sprintf(hyperLinkFormat, pr.TagHtmlUrl, "Tag"),
				pr.TagCommitSHA,
			})
		}
	}
	rows = append(rows, []string{"", "Total", strconv.Itoa(len(prResponses))})

	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := defaultTableStyle(row, col, len(prResponses), true)

			return style
		}).
		Headers("PR #", "Repository", "PR URL", "Tag URL", "Tag CommitSha").
		Rows(rows...)

	fmt.Println(t)
}

func PrintCommitResponses(commitResponses []model.CommitResponse) {
	rows := make([][]string, 0, len(commitResponses)+1)
	failedRows := make([][]string, 0)
	for _, commit := range commitResponses {
		if commit.ErrorMessage != "" {
			failedRows = append(failedRows, []string{commit.RepositoryName, commit.ErrorMessage})
		} else {
			rows = append(rows, []string{
				commit.RepoName(),
				commit.Commit.Author.Name,
				commit.Commit.Author.Date,
				commit.Commit.Message,
				strconv.Itoa(commit.Commit.CommentCount),
				fmt.Sprintf(hyperLinkFormat, commit.HtmlUrl, "Link"),
			})
		}
	}
	rows = append(rows, []string{"Total Commits", strconv.Itoa(len(commitResponses))})

	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := defaultTableStyle(row, col, len(commitResponses), false)

			if col == 1 {
				style = style.UnsetForeground()
			}
			if col == 0 && row != 0 {
				style = style.Foreground(lipgloss.Color("#00B500"))
			}

			if row == len(rows) {
				if col == 0 {
					style = style.Foreground(lipgloss.Color(turquoise))
				}
				if col == 1 {
					style = style.Foreground(lipgloss.Color(turquoise))
				}
			}
			return style
		}).
		Headers("Repository", "Author Name", "Author Date", "Message", "Comment Count", "Url").
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
					style = style.Foreground(lipgloss.Color(red))
				}
				return style
			}).
			Headers("Project Name", "Error Message").
			Rows(failedRows...)

		fmt.Println(t)
	}
}
