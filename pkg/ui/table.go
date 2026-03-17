// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

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

	HeaderStyle  = re.NewStyle().Padding(0, 1).Foreground(Cyan).Bold(true).Align(lipgloss.Center)
	FooterStyle  = re.NewStyle().Padding(0, 1).Foreground(Cyan).Bold(true).Align(lipgloss.Center)
	CellStyle    = re.NewStyle().Padding(0, 1)
	OddRowStyle  = CellStyle.Foreground(White)
	EvenRowStyle = CellStyle.Foreground(Subtle)
	BorderStyle  = lipgloss.NewStyle().Foreground(Dimmed)

	CommitMessageStyle = lipgloss.NewStyle().Foreground(White).Bold(true)
	CommitAuthorStyle  = lipgloss.NewStyle().Foreground(Subtle).Italic(true)
	CommitDateStyle    = lipgloss.NewStyle().Foreground(Subtle).Italic(true)
	CommitShaStyle     = lipgloss.NewStyle().Foreground(Dimmed).Italic(true)
)

type TableRowType interface {
	model.Repository | model.RefUIResponse | model.ProtectedBranch | model.OrgTeam | model.WorkflowRun | model.BranchResponse | model.TagResponse
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

func PrintNoDataMessage(message string, hints ...string) {
	iconStyle := lipgloss.NewStyle().Foreground(Yellow)
	msgStyle := lipgloss.NewStyle().Bold(true).Foreground(White)
	hintStyle := lipgloss.NewStyle().Foreground(Dimmed).Italic(true)
	wrapStyle := lipgloss.NewStyle().PaddingTop(1).PaddingLeft(2).PaddingBottom(1)

	lines := iconStyle.Render("⚠ ") + msgStyle.Render(message)
	for _, hint := range hints {
		lines += "\n" + hintStyle.Render("  "+hint)
	}

	fmt.Println(wrapStyle.Render(lines))
}

func printErrorMessageMap(errorMessageMap map[string][]string) {
	if len(errorMessageMap) > 0 {
		headingStyle := lipgloss.NewStyle().Bold(true).Foreground(Red).PaddingTop(1)
		errorMsgStyle := lipgloss.NewStyle().Bold(true).Foreground(Red).PaddingLeft(2)
		repoStyle := lipgloss.NewStyle().Italic(true).Foreground(Dimmed).PaddingLeft(4)

		totalRepos := 0
		for _, repos := range errorMessageMap {
			totalRepos += len(repos)
		}

		fmt.Println(headingStyle.Render(fmt.Sprintf("✗ Failed to process %d %s:", totalRepos, pluralize("repository", totalRepos))))
		for errorMessage, repos := range errorMessageMap {
			fmt.Println(errorMsgStyle.Render(errorMessage))
			fmt.Println(repoStyle.Render(strings.Join(repos, ", ")))
		}
		fmt.Println()
	}
}

const (
	maxLenDescription = 60
	maxLenTitle       = 50
	maxLenSSHUrl      = 45
	maxLenWorkflow    = 40
	maxLenRefs        = 35
	maxLenBranch      = 30
	maxLenName        = 20
)

func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	if maxLen <= 3 {
		return text[:maxLen]
	}
	return text[:maxLen-3] + "..."
}

func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	if strings.HasSuffix(word, "y") {
		return word[:len(word)-1] + "ies"
	}
	return word + "s"
}

func PrintRepositories(repos []model.Repository) {
	rows := convertToRows(repos, func(repo model.Repository) []string {
		prCount := strconv.Itoa(repo.OpenPullRequestsCount)
		if repo.OpenPullRequestsCount != 0 {
			prCount = fmt.Sprintf(HyperLinkFormat, repo.HTMLUrl+"/pulls", prCount)
		}

		return []string{
			repo.Name,
			truncateText(repo.Description, maxLenDescription),
			repo.DefaultBranch,
			repo.Language,
			strconv.FormatBool(repo.Private),
			truncateText(repo.SSHUrl, maxLenSSHUrl),
			fmt.Sprintf(HyperLinkFormat, repo.HTMLUrl, "Link"),
			prCount,
		}
	})

	totalPRs := 0
	for _, repo := range repos {
		totalPRs += repo.OpenPullRequestsCount
	}

	rows = append(rows, []string{"Total Repositories", strconv.Itoa(len(repos)), "", "", "", "", "", strconv.Itoa(totalPRs)})

	fmt.Println()
	t := table.New().
		Width(TerminalWidth()).
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			return repositoryTableStyle(row, col, rows)
		}).
		Headers(repositoryNameDisplayName, "Description", "Default branch", "Language", "Is Private", "SSH URL", "HTML Page", "Open PRs").
		Rows(rows...)

	fmt.Println(t)
}

func repositoryTableStyle(row, col int, rows [][]string) lipgloss.Style {
	style := defaultTableStyle(row, col, len(rows), 0, true)

	if row >= 0 {
		switch col {
		case 3, 4, 6:
			style = style.Align(lipgloss.Center)
		case 7:
			style = style.Align(lipgloss.Right)
		}
		if row < len(rows)-1 && col == 7 && rows[row][col] != "0" {
			style = style.Foreground(Red)
		}
	}
	return style
}

func PrintResponses(responses []model.RefUIResponse) {
	if len(responses) == 0 {
		PrintNoDataMessage("No data found for the given input.",
			"Hint: verify the org, repo, and branch flags are correct.")
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
					style = style.Foreground(Red)
				}
				return style
			}).
			Headers(repositoryNameDisplayName, errorMessageDisplayName).
			Rows(failedRows...)

		fmt.Println(t)
	}
}

func PrintBranches(branches []model.BranchResponse, orgName string, compact bool, sortBy string) {
	if len(branches) == 0 {
		PrintNoDataMessage("No branches found.",
			"Hint: verify the org and repo flags are correct.")
		return
	}

	SortBranches(branches, sortBy)

	if compact {
		headers := []string{"Repository", "Branch", "SHA", "Protected", "Open"}
		rows := make([][]string, 0, len(branches))
		for _, b := range branches {
			prot := "no"
			if b.Protected {
				prot = "yes"
			}
			branchURL := fmt.Sprintf("https://github.com/%s/%s/tree/%s", orgName, b.RepositoryName, b.Name)
			rows = append(rows, []string{b.RepositoryName, b.Name, ShortSHA(b.Commit.SHA), prot, branchURL})
		}
		PrintCompactTable(headers, rows)
		return
	}

	rows := make([][]string, 0, len(branches))
	for _, b := range branches {
		prot := "no"
		if b.Protected {
			prot = "yes"
		}
		branchURL := fmt.Sprintf("https://github.com/%s/%s/tree/%s", orgName, b.RepositoryName, b.Name)
		rows = append(rows, []string{b.RepositoryName, b.Name, ShortSHA(b.Commit.SHA), prot, fmt.Sprintf(HyperLinkFormat, branchURL, "Open")})
	}
	rows = append(rows, []string{"Total", strconv.Itoa(len(branches)), "", "", ""})

	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		Width(TerminalWidth()).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := defaultTableStyle(row, col, len(rows), 0, true)
			if col == 3 && row >= 0 && row < len(rows)-1 {
				if rows[row][3] == "yes" {
					style = style.Foreground(Yellow)
				}
			}
			return style
		}).
		Headers(
			SortIndicator(repositoryNameDisplayName, sortBy, "repo"),
			SortIndicator("Branch", sortBy, "name"),
			"SHA",
			SortIndicator("Protected", sortBy, "protected"),
			"Open",
		).
		Rows(rows...)

	fmt.Println(t)
}

func SortBranches(branches []model.BranchResponse, sortBy string) {
	switch strings.ToLower(sortBy) {
	case "repo":
		sort.Slice(branches, func(i, j int) bool { return branches[i].RepositoryName < branches[j].RepositoryName })
	case "name":
		sort.Slice(branches, func(i, j int) bool { return branches[i].Name < branches[j].Name })
	case "protected":
		sort.Slice(branches, func(i, j int) bool { return branches[i].Protected && !branches[j].Protected })
	}
}

func PrintTags(tags []model.TagResponse, compact bool, sortBy string) {
	if len(tags) == 0 {
		PrintNoDataMessage("No tags found.",
			"Hint: verify the org and repo flags are correct.")
		return
	}

	SortTags(tags, sortBy)

	if compact {
		headers := []string{"Repository", "Tag", "SHA"}
		rows := make([][]string, 0, len(tags))
		for _, t := range tags {
			rows = append(rows, []string{t.RepositoryName, t.Name, ShortSHA(t.Commit.SHA)})
		}
		PrintCompactTable(headers, rows)
		return
	}

	rows := make([][]string, 0, len(tags))
	for _, tg := range tags {
		rows = append(rows, []string{tg.RepositoryName, tg.Name, ShortSHA(tg.Commit.SHA)})
	}
	rows = append(rows, []string{"Total", strconv.Itoa(len(tags)), ""})

	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		Width(TerminalWidth()).
		StyleFunc(func(row, col int) lipgloss.Style {
			return defaultTableStyle(row, col, len(rows), 0, true)
		}).
		Headers(
			SortIndicator(repositoryNameDisplayName, sortBy, "repo"),
			SortIndicator("Tag", sortBy, "tag"),
			"SHA",
		).
		Rows(rows...)

	fmt.Println(t)
}

func SortTags(tags []model.TagResponse, sortBy string) {
	switch strings.ToLower(sortBy) {
	case "repo":
		sort.Slice(tags, func(i, j int) bool { return tags[i].RepositoryName < tags[j].RepositoryName })
	case "tag":
		sort.Slice(tags, func(i, j int) bool { return tags[i].Name < tags[j].Name })
	}
}

func PrintPullRequestResponses(prResponses []model.PullRequestResponse, sortBy string, compact bool) {
	if len(prResponses) == 0 {
		PrintNoDataMessage("No Pull Requests found.",
			"Hint: try --state all to include closed PRs.")
		return
	}

	SortPullRequests(prResponses, sortBy)

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
				truncateText(pr.TitleName, maxLenTitle),
				truncateText(pr.AuthorName(), maxLenName),
				truncateText(pr.AssigneesName(), maxLenName),
				truncateText(pr.ReviewersName(), maxLenName),
				pr.State + " / " + pr.MergeStateStatus,
				truncateText(refs, maxLenRefs),
				fmt.Sprintf(HyperLinkFormat, pr.HTMLUrl, "Open"),
			})
		}
	}

	if len(rows) > 0 {
		headers := []string{"ID", "Repository", "Title", "Created User", "Assignees", "Reviewers", "Status/Merge State", "Refs", "HTMLUrl"}
		if compact {
			PrintCompactTable(headers, rows)
			return
		}
		rows = append(rows, []string{"", "Total Pull Requests", strconv.Itoa(len(prResponses))})
		fmt.Println()
		t := table.New().
			Width(TerminalWidth()).
			Border(lipgloss.RoundedBorder()).
			BorderStyle(BorderStyle).
			BorderRow(true).
			StyleFunc(func(row, col int) lipgloss.Style {
				return pullRequestStyle(row, col, rows)
			}).
			Headers(
				"ID",
				SortIndicator(repositoryNameDisplayName, sortBy, "repo"),
				SortIndicator("Title", sortBy, "title"),
				SortIndicator("Created User", sortBy, "author"),
				"Assignees", "Reviewers",
				SortIndicator("Status/Merge State", sortBy, "status"),
				"Refs", "HTMLUrl",
			).
			Rows(rows...)

		fmt.Println(t)
	}

	printErrorMessageMap(errorMessageMap)
}

func SortPullRequests(prs []model.PullRequestResponse, sortBy string) {
	switch strings.ToLower(sortBy) {
	case "repo":
		sort.Slice(prs, func(i, j int) bool { return prs[i].RepositoryName() < prs[j].RepositoryName() })
	case "title":
		sort.Slice(prs, func(i, j int) bool { return prs[i].TitleName < prs[j].TitleName })
	case "author":
		sort.Slice(prs, func(i, j int) bool { return prs[i].AuthorName() < prs[j].AuthorName() })
	case "status":
		sort.Slice(prs, func(i, j int) bool { return prs[i].State < prs[j].State })
	}
}

func pullRequestStyle(row int, col int, rows [][]string) lipgloss.Style {
	style := defaultTableStyle(row, col, len(rows), 1, true)

	if row >= 0 {
		switch col {
		case 0, 6, 8:
			style = style.Align(lipgloss.Center)
		}
	}

	if row >= 0 && row < len(rows)-1 && col == 6 {
		status := strings.SplitN(rows[row][6], " / ", 2)[0]
		if status == "closed" {
			style = style.Strikethrough(true)
		}
		style = style.Foreground(StatusColor(status))
	}
	return style
}

func PrintMergeResponses(mergeResponses []model.MergeResponse) {
	if len(mergeResponses) == 0 {
		PrintNoDataMessage("No Merge Responses found.",
			"Hint: ensure the PRs exist and are mergeable.")
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
					style = style.Foreground(Red)
				}
			}
			return style
		}).
		Headers(repositoryNameDisplayName, "Message", "Sha").
		Rows(rows...)

	fmt.Println(t)
}

func PrintPRDetail(prResponse model.PullRequestResponse, filesResponse model.PullRequestFilesResponse, checkRunResponse model.CheckRunResponse, reviews []model.ReviewPullRequestResponse) {
	if prResponse.ErrorMessage != "" {
		errStyle := lipgloss.NewStyle().Foreground(Red).Bold(true)
		fmt.Println()
		fmt.Println(errStyle.Render("  ✗ " + prResponse.ErrorMessage))
		fmt.Println()
		return
	}

	titleStyle := lipgloss.NewStyle().Foreground(Cyan).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(Dimmed)
	valueStyle := lipgloss.NewStyle().Foreground(White)
	greenStyle := lipgloss.NewStyle().Foreground(Green)
	redStyle := lipgloss.NewStyle().Foreground(Red)

	stateIcon := StatusIcon(prResponse.State)
	stateColor := StatusColor(prResponse.State)
	stateStyle := lipgloss.NewStyle().Foreground(stateColor).Bold(true)

	fmt.Println()
	fmt.Printf("  %s %s\n", titleStyle.Render(fmt.Sprintf("#%d %s", prResponse.PRNumber, prResponse.TitleName)),
		stateStyle.Render(stateIcon+" "+prResponse.State))
	fmt.Printf("  %s\n", lipgloss.NewStyle().Foreground(Subtle).Render(
		prResponse.Head.Ref+" → "+prResponse.Base.Ref+"  ("+prResponse.RepositoryName()+")"))
	fmt.Println()

	printField := func(label, value string) {
		if value != "" && value != "-" && value != "0" {
			fmt.Printf("  %s  %s\n", labelStyle.Width(18).Align(lipgloss.Right).Render(label), valueStyle.Render(value))
		}
	}

	printField("Author", prResponse.AuthorName())
	if prResponse.AssigneesName() != "" {
		printField("Assignees", strings.ReplaceAll(prResponse.AssigneesName(), "\n", ", "))
	}
	if prResponse.ReviewersName() != "" {
		printField("Reviewers", strings.ReplaceAll(prResponse.ReviewersName(), "\n", ", "))
	}
	printField("Mergeable", prResponse.MergeStateStatus)
	if prResponse.MergeAt != "" {
		printField("Merged At", RelativeTime(prResponse.MergeAt))
	}
	if prResponse.MergedBy.Login != "" {
		mergedByName := prResponse.MergedBy.Name
		if mergedByName == "" {
			mergedByName = prResponse.MergedBy.Login
		}
		printField("Merged By", mergedByName)
	}

	changes := greenStyle.Render(fmt.Sprintf("+%d", prResponse.Additions)) + "  " +
		redStyle.Render(fmt.Sprintf("-%d", prResponse.Deletions))
	fmt.Printf("  %s  %s  %s\n",
		labelStyle.Width(18).Align(lipgloss.Right).Render("Changes"),
		changes,
		lipgloss.NewStyle().Foreground(Subtle).Render(fmt.Sprintf("(%d files, %d commits)", prResponse.ChangedFiles, prResponse.Commits)))

	if prResponse.Comments > 0 || prResponse.ReviewComments > 0 {
		printField("Comments", fmt.Sprintf("%d comments, %d review comments", prResponse.Comments, prResponse.ReviewComments))
	}

	printField("URL", prResponse.HTMLUrl)

	if prResponse.Body != "" {
		body := prResponse.Body
		if len(body) > 300 {
			body = body[:300] + "..."
		}
		fmt.Println()
		fmt.Println(lipgloss.NewStyle().Foreground(Subtle).Padding(0, 2).Render(body))
	}

	// Files changed
	if len(filesResponse.Files) > 0 {
		fmt.Println()
		fmt.Println(titleStyle.Render("  Files Changed"))
		for i, file := range filesResponse.Files {
			if i >= 10 {
				fmt.Printf("  %s\n", lipgloss.NewStyle().Foreground(Subtle).Render(
					fmt.Sprintf("  ... and %d more files", len(filesResponse.Files)-10)))
				break
			}
			changeIcon := "M"
			changeColor := Yellow
			switch file.ChangeType {
			case "added":
				changeIcon = "A"
				changeColor = Green
			case "removed":
				changeIcon = "D"
				changeColor = Red
			case "renamed":
				changeIcon = "R"
				changeColor = Cyan
			}
			fmt.Printf("  %s %s %s\n",
				lipgloss.NewStyle().Foreground(changeColor).Bold(true).Render("  "+changeIcon),
				valueStyle.Render(file.Filename),
				lipgloss.NewStyle().Foreground(Subtle).Render(
					fmt.Sprintf("(+%d -%d)", file.Additions, file.Deletions)))
		}
	}

	// Check runs
	if len(checkRunResponse.CheckRuns) > 0 {
		fmt.Println()
		fmt.Println(titleStyle.Render("  Check Runs"))
		for _, cr := range checkRunResponse.CheckRuns {
			icon := StatusIcon(cr.Conclusion)
			color := StatusColor(cr.Conclusion)
			fmt.Printf("  %s %s  %s\n",
				lipgloss.NewStyle().Foreground(color).Render("  "+icon),
				valueStyle.Render(cr.Name),
				lipgloss.NewStyle().Foreground(Subtle).Render(cr.Status))
		}
	}

	// Reviews
	if len(reviews) > 0 {
		fmt.Println()
		fmt.Println(titleStyle.Render("  Reviews"))
		for _, review := range reviews {
			reviewer := review.User.Name
			if reviewer == "" {
				reviewer = review.User.Login
			}
			icon := StatusIcon(review.State)
			color := StatusColor(review.State)
			submitted := RelativeTime(review.SubmittedAt)
			fmt.Printf("  %s %s  %s  %s\n",
				lipgloss.NewStyle().Foreground(color).Render("  "+icon),
				valueStyle.Render(reviewer),
				lipgloss.NewStyle().Foreground(color).Render(review.State),
				lipgloss.NewStyle().Foreground(Subtle).Render(submitted))
		}
	}

	fmt.Println()
}

func PrintReviewResponse(response model.ReviewPullRequestResponse) {
	if response.ErrorMessage != "" {
		fmt.Println()
		errStyle := lipgloss.NewStyle().Foreground(Red).Bold(true)
		fmt.Println(errStyle.Render("  ✗ " + response.ErrorMessage))
		fmt.Println()
		return
	}

	stateColor := StatusColor(response.State)
	stateStyle := lipgloss.NewStyle().Foreground(stateColor).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(White).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(Subtle)

	fmt.Println()
	fmt.Printf("  %s Review submitted for %s#%d\n",
		stateStyle.Render(StatusIcon(response.State)),
		lipgloss.NewStyle().Foreground(Green).Render(response.RepositoryName),
		response.PRNumber,
	)
	fmt.Printf("    %s %s\n", labelStyle.Render("State:"), stateStyle.Render(response.State))
	if response.Body != "" {
		fmt.Printf("    %s %s\n", labelStyle.Render("Body:"), valueStyle.Render(response.Body))
	}
	if response.SubmittedAt != "" {
		fmt.Printf("    %s %s\n", labelStyle.Render("Submitted:"), valueStyle.Render(RelativeTime(response.SubmittedAt)))
	}
	fmt.Println()
}

func PrintProtectedBranches(pbResponses []model.ProtectedBranch) {
	if len(pbResponses) == 0 {
		PrintNoDataMessage("No Protected Branches found.",
			"Hint: check that branch protection rules are configured for the target repos.")
		return
	}
	rows, failedRows := getProtectedBranches(pbResponses)
	rows = append(rows, []string{"Total Protected Branches", "", "", strconv.Itoa(len(rows))})

	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := defaultTableStyle(row, col, len(rows), 0, true)
			// Apply red color if Enforce Admins (column 3) is "false"
			if col == 3 && row != -1 && row < len(rows)-1 && len(rows[row]) > 3 && rows[row][3] == "false" {
				style = style.Foreground(Red)
			}
			return style
		}).
		Headers(repositoryNameDisplayName, "Branch", "Type", "Enforce Admins", "Reviewers", "Code Owner Reviews", "Last Push Approval", "Dismiss Stale reviews", "Status Checks", "Lock branch", "Bypass allowed Users", "Restrictions Users", "Rule set names").
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
					style = style.Foreground(Red)
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
					pb.Name,
					pb.Type,
					"",
				}
			} else {
				return []string{
					pb.RepositoryName,
					pb.Name,
					pb.Type,
					strconv.FormatBool(pb.EnforceAdmins),
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

func PrintPostReleaseResponses(responses []model.PostReleaseResponse) {
	if len(responses) == 0 {
		PrintNoDataMessage("No post-release activity performed.",
			"Hint: verify that --ref points to an existing branch and that at least one of --branch or --tag is set.")
		return
	}

	errorMessageMap := map[string][]string{}
	rows := make([][]string, 0, len(responses)+1)

	for _, r := range responses {
		if r.ErrorMessage != "" {
			errorMessageMap[r.ErrorMessage] = append(errorMessageMap[r.ErrorMessage], r.RepositoryName)
			continue
		}

		tagCell := "—"
		if r.TagURL != "" {
			tagCell = fmt.Sprintf(HyperLinkFormat, r.TagURL, r.TagName)
		} else if r.TagName != "" {
			tagCell = r.TagName
		}

		shaCell := r.BranchSHA
		if shaCell == "" {
			shaCell = r.TagSHA
		}

		rows = append(rows, []string{
			r.RepositoryName,
			r.BranchName,
			tagCell,
			shaCell,
		})
	}

	if len(rows) > 0 {
		rows = append(rows, []string{"Total", strconv.Itoa(len(rows)), "", ""})
		fmt.Println()
		t := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(BorderStyle).
			BorderRow(true).
			StyleFunc(func(row, col int) lipgloss.Style {
				return defaultTableStyle(row, col, len(rows), 0, true)
			}).
			Headers(repositoryNameDisplayName, "Branch", "Tag", "SHA").
			Rows(rows...)
		fmt.Println(t)
	}

	printErrorMessageMap(errorMessageMap)
}

func PrintCommitResponses(commitResponses []model.CommitResponse, includeMergeCommits bool, sortBy string, compact bool) {
	if len(commitResponses) == 0 {
		PrintNoDataMessage("No Commits found.",
			"Hint: try a wider date range with -n (e.g. -n 7).")
		return
	}

	SortCommits(commitResponses, sortBy)

	errorMessageMap := map[string][]string{}
	rows := make([][]string, 0, len(commitResponses)+1)
	for _, commit := range commitResponses {
		if commit.ErrorMessage != "" {
			errorMessageMap[commit.ErrorMessage] = append(errorMessageMap[commit.ErrorMessage], commit.RepositoryName)
			continue
		}
		if !includeMergeCommits && commit.Commit.Committer.Name == "GitHub" {
			continue
		}
		msg := commit.Commit.Message
		if idx := strings.IndexByte(msg, '\n'); idx >= 0 {
			msg = msg[:idx]
		}
		rows = append(rows, []string{
			commit.RepoName(),
			ShortSHA(commit.Sha),
			commit.Commit.Author.Name,
			RelativeTime(commit.Commit.Author.Date),
			truncateText(msg, 60),
			fmt.Sprintf(HyperLinkFormat, commit.HtmlUrl, "Open"),
		})
	}

	if len(rows) > 0 {
		headers := []string{"Repository", "SHA", "Author", "Date", "Message", "URL"}
		if compact {
			PrintCompactTable(headers, rows)
		} else {
			rows = append(rows, []string{"Total Commits", strconv.Itoa(len(rows)), "", "", "", ""})
			fmt.Println()
			t := table.New().
				Width(TerminalWidth()).
				Border(lipgloss.RoundedBorder()).
				BorderStyle(BorderStyle).
				BorderRow(true).
				StyleFunc(func(row, col int) lipgloss.Style {
					style := defaultTableStyle(row, col, len(rows), 0, true)
					if row >= 0 && row < len(rows)-1 {
						switch col {
						case 1:
							style = style.Foreground(Dimmed).Italic(true)
						case 5:
							style = style.Align(lipgloss.Center)
						}
					}
					return style
				}).
				Headers(
					SortIndicator(repositoryNameDisplayName, sortBy, "repo"),
					"SHA",
					SortIndicator("Author", sortBy, "author"),
					SortIndicator("Date", sortBy, "date"),
					"Message",
					"URL",
				).
				Rows(rows...)
			fmt.Println(t)
		}
	}

	printErrorMessageMap(errorMessageMap)
}

func SortCommits(commits []model.CommitResponse, sortBy string) {
	switch strings.ToLower(sortBy) {
	case "repo":
		sort.Slice(commits, func(i, j int) bool { return commits[i].RepositoryName < commits[j].RepositoryName })
	case "author":
		sort.Slice(commits, func(i, j int) bool { return commits[i].Commit.Author.Name < commits[j].Commit.Author.Name })
	default: // "date" or anything else — newest first
		sort.Slice(commits, func(i, j int) bool {
			ti, _ := time.Parse(time.RFC3339, commits[i].Commit.Author.Date)
			tj, _ := time.Parse(time.RFC3339, commits[j].Commit.Author.Date)
			return ti.After(tj)
		})
	}
}

func PrintCommitSummary(commitResponses []model.CommitResponse, includeMergeCommits bool, sortBy string) {
	if len(commitResponses) == 0 {
		PrintNoDataMessage("No Commits found.",
			"Hint: try a wider date range with -n (e.g. -n 7).")
		return
	}

	errorMessageMap := map[string][]string{}
	for _, c := range commitResponses {
		if c.ErrorMessage != "" {
			errorMessageMap[c.ErrorMessage] = append(errorMessageMap[c.ErrorMessage], c.RepositoryName)
		}
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
		SortCommits(commits, sortBy)
		inner := table.New().
			Border(lipgloss.HiddenBorder()).
			StyleFunc(func(row, col int) lipgloss.Style {
				switch col {
				case 0:
					return CellStyle.Foreground(White)
				case 1:
					return CellStyle.Foreground(Subtle).Italic(true)
				case 2:
					return CellStyle.Foreground(Dimmed).Italic(true)
				default:
					return CellStyle.Foreground(Dimmed)
				}
			})
		commitsRows := make([][]string, 0, len(commits))
		for _, commit := range commits {
			msg := commit.Commit.Message
			if idx := strings.IndexByte(msg, '\n'); idx >= 0 {
				msg = msg[:idx]
			}
			commitsRows = append(commitsRows, []string{
				truncateText(msg, 60),
				commit.Commit.Author.Name,
				RelativeTime(commit.Commit.Author.Date),
				fmt.Sprintf(HyperLinkFormat, commit.HtmlUrl, ShortSHA(commit.Sha)),
			})
		}
		inner.Rows(commitsRows...)
		rows = append(rows, []string{repo, strconv.Itoa(len(commits)), inner.Render()})
	}
	rows = append(rows, []string{"Total Commits", strconv.Itoa(len(commitResponses)), ""})

	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == -1 {
				return HeaderStyle
			}
			if row == len(rows)-1 {
				return FooterStyle
			}
			switch col {
			case 0:
				return CellStyle.Foreground(Green).AlignVertical(lipgloss.Center)
			case 1:
				return CellStyle.Foreground(Subtle).Align(lipgloss.Center, lipgloss.Center)
			}
			return CellStyle
		}).
		Headers(SortIndicator(repositoryNameDisplayName, sortBy, "repo"), "Total", "Commits").
		Rows(rows...)

	fmt.Println(t)
	printErrorMessageMap(errorMessageMap)
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
		PrintNoDataMessage("No Teams found.",
			"Hint: ensure the org name is correct and you have team read access.")
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
			switch col {
			case 0:
				style = CellStyle.Foreground(Subtle).AlignVertical(lipgloss.Center)
			case 1, 3:
				style = CellStyle.Foreground(Subtle).Align(lipgloss.Center, lipgloss.Center)
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

func PrintWorkflowRuns(runs []model.WorkflowRun, sortBy string, compact bool) {
	if len(runs) == 0 {
		PrintNoDataMessage("No Workflow Runs found.",
			"Hint: try --status in_progress or remove filters to see all runs.")
		return
	}

	SortWorkflowRuns(runs, sortBy)

	errorMessageMap := map[string][]string{}
	rows := make([][]string, 0, len(runs)+1)
	for _, run := range runs {
		if run.ErrorMessage != "" {
			errorMessageMap[run.ErrorMessage] = append(errorMessageMap[run.ErrorMessage], run.RepositoryName)
			continue
		}
		statusConclusion := run.Status
		if run.Conclusion != "" {
			statusConclusion = run.Conclusion
		}
		actorName := run.Actor.Login
		if actorName == "" {
			actorName = run.Actor.Name
		}
		rows = append(rows, []string{
			run.RepositoryName,
			strconv.Itoa(run.ID),
			strconv.Itoa(run.RunNumber),
			truncateText(run.Name, maxLenWorkflow),
			statusConclusion,
			truncateText(run.HeadBranch, maxLenBranch),
			run.Event,
			actorName,
			RelativeTime(run.CreatedAt),
			fmt.Sprintf(HyperLinkFormat, run.HTMLUrl, "Open"),
		})
	}

	if len(rows) > 0 {
		headers := []string{"Repository", "Run ID", "Run #", "Workflow", "Status", "Branch", "Event", "Actor", "Created At", "URL"}
		if compact {
			PrintCompactTable(headers, rows)
			return
		}
		rows = append(rows, []string{"Total Workflow Runs", strconv.Itoa(len(rows))})
		fmt.Println()
		t := table.New().
			Width(TerminalWidth()).
			Border(lipgloss.RoundedBorder()).
			BorderStyle(BorderStyle).
			BorderRow(true).
			StyleFunc(func(row, col int) lipgloss.Style {
				return workflowRunTableStyle(row, col, rows)
			}).
			Headers(
				SortIndicator(repositoryNameDisplayName, sortBy, "repo"),
				"Run ID", "Run #",
				SortIndicator("Workflow", sortBy, "name"),
				SortIndicator("Status", sortBy, "status"),
				"Branch", "Event", "Actor",
				SortIndicator("Created At", sortBy, "created"),
				"URL",
			).
			Rows(rows...)

		fmt.Println(t)
	}

	printErrorMessageMap(errorMessageMap)
}

func SortWorkflowRuns(runs []model.WorkflowRun, sortBy string) {
	switch strings.ToLower(sortBy) {
	case "repo":
		sort.Slice(runs, func(i, j int) bool { return runs[i].RepositoryName < runs[j].RepositoryName })
	case "name":
		sort.Slice(runs, func(i, j int) bool { return runs[i].Name < runs[j].Name })
	case "status":
		sort.Slice(runs, func(i, j int) bool {
			si, sj := runs[i].Conclusion, runs[j].Conclusion
			if si == "" {
				si = runs[i].Status
			}
			if sj == "" {
				sj = runs[j].Status
			}
			return si < sj
		})
	case "created":
		sort.Slice(runs, func(i, j int) bool {
			ti, _ := time.Parse(time.RFC3339, runs[i].CreatedAt)
			tj, _ := time.Parse(time.RFC3339, runs[j].CreatedAt)
			return ti.After(tj)
		})
	}
}

func PrintWorkflowRunDetail(detail model.WorkflowRunDetail) {
	fmt.Print(RenderWorkflowRunDetail(detail))
}

func RenderWorkflowRunDetail(detail model.WorkflowRunDetail) string {
	var b strings.Builder

	if detail.ErrorMessage != "" {
		errStyle := lipgloss.NewStyle().Bold(true).Foreground(Red).PaddingTop(1).PaddingLeft(2).PaddingBottom(1)
		b.WriteString(errStyle.Render(fmt.Sprintf("✗ Error: %s", detail.ErrorMessage)))
		b.WriteString("\n")
		return b.String()
	}

	run := detail.Run
	statusConclusion := run.Status
	if run.Conclusion != "" {
		statusConclusion = run.Conclusion
	}
	actorName := run.Actor.Login
	if actorName == "" {
		actorName = run.Actor.Name
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(White).PaddingLeft(1)
	labelStyle := lipgloss.NewStyle().Foreground(Dimmed).PaddingLeft(2)
	valueStyle := lipgloss.NewStyle().Foreground(White)
	statusStyle := lipgloss.NewStyle().Bold(true).Foreground(StatusColor(statusConclusion))

	b.WriteString("\n")
	b.WriteString(titleStyle.Render(fmt.Sprintf("%s Workflow Run #%d - %s", StatusIcon(statusConclusion), run.RunNumber, run.Name)))
	b.WriteString("\n\n")
	b.WriteString(labelStyle.Render("Repository:  ") + valueStyle.Render(run.RepositoryName) + "\n")
	b.WriteString(labelStyle.Render("Run ID:      ") + valueStyle.Render(strconv.Itoa(run.ID)) + "\n")
	b.WriteString(labelStyle.Render("Run Number:  ") + valueStyle.Render(strconv.Itoa(run.RunNumber)) + "\n")
	b.WriteString(labelStyle.Render("Status:      ") + statusStyle.Render(statusConclusion) + "\n")
	b.WriteString(labelStyle.Render("Branch:      ") + valueStyle.Render(run.HeadBranch) + "\n")
	b.WriteString(labelStyle.Render("Event:       ") + valueStyle.Render(run.Event) + "\n")
	b.WriteString(labelStyle.Render("Actor:       ") + valueStyle.Render(actorName) + "\n")
	b.WriteString(labelStyle.Render("Attempt:     ") + valueStyle.Render(strconv.Itoa(run.RunAttempt)) + "\n")
	b.WriteString(labelStyle.Render("Created:     ") + valueStyle.Render(RelativeTime(run.CreatedAt)) + "\n")
	b.WriteString(labelStyle.Render("Updated:     ") + valueStyle.Render(RelativeTime(run.UpdatedAt)) + "\n")
	b.WriteString(labelStyle.Render("Commit:      ") + valueStyle.Render(run.HeadSha) + "\n")
	b.WriteString(labelStyle.Render("URL:         ") + valueStyle.Render(fmt.Sprintf(HyperLinkFormat, run.HTMLUrl, run.HTMLUrl)) + "\n")

	if len(detail.Jobs) == 0 {
		iconStyle := lipgloss.NewStyle().Foreground(Yellow)
		msgStyle := lipgloss.NewStyle().Bold(true).Foreground(White)
		hintStyle := lipgloss.NewStyle().Foreground(Dimmed).Italic(true)
		wrapStyle := lipgloss.NewStyle().PaddingTop(1).PaddingLeft(2).PaddingBottom(1)
		b.WriteString(wrapStyle.Render(
			iconStyle.Render("⚠ ") + msgStyle.Render("No jobs found for this workflow run.") +
				"\n" + hintStyle.Render("  The run may still be queuing or was skipped.")))
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Jobs"))

	rows := make([][]string, 0)
	for _, job := range detail.Jobs {
		jobConclusion := job.Status
		if job.Conclusion != "" {
			jobConclusion = job.Conclusion
		}

		stepsDetail := make([]string, 0, len(job.Steps))
		for _, step := range job.Steps {
			stepConclusion := step.Status
			if step.Conclusion != "" {
				stepConclusion = step.Conclusion
			}
			stepsDetail = append(stepsDetail, fmt.Sprintf("%s %s", StatusIcon(stepConclusion), step.Name))
		}

		rows = append(rows, []string{
			job.Name,
			jobConclusion,
			RelativeTime(job.StartedAt),
			RelativeTime(job.CompletedAt),
			strings.Join(stepsDetail, "\n"),
			fmt.Sprintf(HyperLinkFormat, job.HTMLUrl, "Open"),
		})
	}

	b.WriteString("\n\n")
	t := table.New().
		Width(TerminalWidth()).
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := defaultTableStyle(row, col, len(rows), -1, false)
			if row >= 0 {
				switch col {
				case 1, 5:
					style = style.Align(lipgloss.Center)
				}
				if col == 1 {
					style = style.Foreground(StatusColor(rows[row][1]))
				}
			}
			return style
		}).
		Headers("Job", "Status", "Started", "Completed", "Steps", "URL").
		Rows(rows...)

	b.WriteString(t.String())
	b.WriteString("\n")
	return b.String()
}

func workflowRunTableStyle(row, col int, rows [][]string) lipgloss.Style {
	style := defaultTableStyle(row, col, len(rows), 0, true)

	if row >= 0 {
		switch col {
		case 4, 6, 9:
			style = style.Align(lipgloss.Center)
		case 1, 2:
			style = style.Align(lipgloss.Right)
		}
		if row < len(rows)-1 && col == 4 {
			style = style.Foreground(StatusColor(rows[row][4]))
		}
	}
	return style
}

// -- Secret Scanning Alerts --

func PrintSecretScanningAlerts(alerts []model.SecretScanningAlert, sortBy string, compact bool) {
	if len(alerts) == 0 {
		PrintNoDataMessage("No secret scanning alerts found.",
			"Hint: try --state open or remove filters to see all alerts.")
		return
	}

	SortSecretAlerts(alerts, sortBy)

	errorMessageMap := map[string][]string{}
	rows := make([][]string, 0, len(alerts)+1)
	for _, alert := range alerts {
		if alert.ErrorMessage != "" {
			errorMessageMap[alert.ErrorMessage] = append(errorMessageMap[alert.ErrorMessage], alert.RepositoryName)
			continue
		}
		location := alert.Location.Path
		if alert.Location.StartLine > 0 {
			location = fmt.Sprintf("%s:%d", location, alert.Location.StartLine)
		}
		rows = append(rows, []string{
			alert.RepositoryName,
			strconv.Itoa(alert.Number),
			alert.State,
			alert.SecretType,
			truncateText(location, 40),
			RelativeTime(alert.CreatedAt),
			fmt.Sprintf(HyperLinkFormat, alert.HTMLUrl, "Open"),
		})
	}

	if len(rows) > 0 {
		headers := []string{"Repository", "Alert #", "State", "Secret Type", "Location", "Created", "URL"}
		if compact {
			PrintCompactTable(headers, rows)
			return
		}
		rows = append(rows, []string{"Total Alerts", strconv.Itoa(len(rows))})
		fmt.Println()
		t := table.New().
			Width(TerminalWidth()).
			Border(lipgloss.RoundedBorder()).
			BorderStyle(BorderStyle).
			BorderRow(true).
			StyleFunc(func(row, col int) lipgloss.Style {
				return secretAlertTableStyle(row, col, rows)
			}).
			Headers(
				SortIndicator(repositoryNameDisplayName, sortBy, "repo"),
				"Alert #",
				SortIndicator("State", sortBy, "state"),
				SortIndicator("Secret Type", sortBy, "type"),
				"Location",
				SortIndicator("Created", sortBy, "created"),
				"URL",
			).
			Rows(rows...)

		fmt.Println(t)
	}

	printErrorMessageMap(errorMessageMap)
}

func PrintSecretAlertDetail(alert model.SecretScanningAlert) {
	if alert.ErrorMessage != "" {
		fmt.Println(lipgloss.NewStyle().Foreground(Red).Bold(true).Render("  Error: " + alert.ErrorMessage))
		return
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(Blue)
	repoStyle := lipgloss.NewStyle().Foreground(Cyan)
	valueStyle := lipgloss.NewStyle().Foreground(White)
	secretTypeStyle := lipgloss.NewStyle().Foreground(Yellow)

	stateStyle := lipgloss.NewStyle().Foreground(StatusColor(alert.State)).Bold(true)

	fmt.Println()
	fmt.Printf("  %s\n", repoStyle.Render(alert.RepositoryName))
	fmt.Println(headerStyle.Render("  " + strings.Repeat("─", 45)))
	fmt.Printf("  %s: %s\n", headerStyle.Render("Alert #"), valueStyle.Render(strconv.Itoa(alert.Number)))
	fmt.Printf("  %s: %s\n", headerStyle.Render("Secret Type"), secretTypeStyle.Render(alert.SecretType))
	if alert.SecretTypeDisplay != "" {
		fmt.Printf("  %s: %s\n", headerStyle.Render("Display Name"), valueStyle.Render(alert.SecretTypeDisplay))
	}
	fmt.Printf("  %s: %s\n", headerStyle.Render("State"), stateStyle.Render(alert.State))

	fmt.Println()
	fmt.Println(headerStyle.Render("  Location:"))
	fmt.Printf("    %s: %s\n", headerStyle.Render("Path"), valueStyle.Render(alert.Location.Path))
	if alert.Location.StartLine > 0 {
		fmt.Printf("    %s: %d\n", headerStyle.Render("Start Line"), alert.Location.StartLine)
		fmt.Printf("    %s: %d\n", headerStyle.Render("End Line"), alert.Location.EndLine)
	}
	if alert.Location.CommitSHA != "" {
		fmt.Printf("    %s: %s\n", headerStyle.Render("Commit SHA"), valueStyle.Render(ShortSHA(alert.Location.CommitSHA)))
	}
	if alert.Location.BlobURL != "" {
		fmt.Printf("    %s: %s\n", headerStyle.Render("Blob URL"), valueStyle.Render(alert.Location.BlobURL))
	}

	fmt.Println()
	fmt.Printf("  %s: %s\n", headerStyle.Render("Created At"), valueStyle.Render(RelativeTime(alert.CreatedAt)))
	if alert.UpdatedAt != "" {
		fmt.Printf("  %s: %s\n", headerStyle.Render("Updated At"), valueStyle.Render(RelativeTime(alert.UpdatedAt)))
	}

	if alert.Resolution != "" {
		fmt.Println()
		fmt.Println(headerStyle.Render("  Resolution:"))
		fmt.Printf("    %s: %s\n", headerStyle.Render("Resolution"), valueStyle.Render(alert.Resolution))
		if alert.ResolvedBy.Login != "" {
			fmt.Printf("    %s: %s\n", headerStyle.Render("Resolved By"), valueStyle.Render(alert.ResolvedBy.Login))
		}
		if alert.ResolvedAt != "" {
			fmt.Printf("    %s: %s\n", headerStyle.Render("Resolved At"), valueStyle.Render(RelativeTime(alert.ResolvedAt)))
		}
		if alert.ResolutionComment != "" {
			fmt.Printf("    %s: %s\n", headerStyle.Render("Comment"), valueStyle.Render(alert.ResolutionComment))
		}
	}

	if alert.HTMLUrl != "" {
		fmt.Println()
		fmt.Printf("  %s: %s\n", headerStyle.Render("URL"), valueStyle.Render(alert.HTMLUrl))
	}
	fmt.Println()
}

func secretAlertTableStyle(row, col int, rows [][]string) lipgloss.Style {
	style := defaultTableStyle(row, col, len(rows), 0, true)

	if row >= 0 {
		switch col {
		case 1, 6:
			style = style.Align(lipgloss.Center)
		}
		if row < len(rows)-1 && col == 2 {
			style = style.Foreground(StatusColor(rows[row][2]))
		}
	}
	return style
}

func SortSecretAlerts(alerts []model.SecretScanningAlert, sortBy string) {
	switch strings.ToLower(sortBy) {
	case "repo":
		sort.Slice(alerts, func(i, j int) bool { return alerts[i].RepositoryName < alerts[j].RepositoryName })
	case "state":
		sort.Slice(alerts, func(i, j int) bool { return alerts[i].State < alerts[j].State })
	case "type":
		sort.Slice(alerts, func(i, j int) bool { return alerts[i].SecretType < alerts[j].SecretType })
	case "created":
		sort.Slice(alerts, func(i, j int) bool {
			ti, _ := time.Parse(time.RFC3339, alerts[i].CreatedAt)
			tj, _ := time.Parse(time.RFC3339, alerts[j].CreatedAt)
			return ti.After(tj)
		})
	}
}

// -- Issues --

func PrintIssues(issues []model.IssueResponse, sortBy string, compact bool) {
	if len(issues) == 0 {
		PrintNoDataMessage("No issues found.",
			"Hint: try --state all or remove filters to see more issues.")
		return
	}

	SortIssues(issues, sortBy)

	errorMessageMap := map[string][]string{}
	rows := make([][]string, 0, len(issues)+1)
	for _, issue := range issues {
		if issue.ErrorMessage != "" {
			errorMessageMap[issue.ErrorMessage] = append(errorMessageMap[issue.ErrorMessage], issue.RepositoryName)
			continue
		}
		labels := issue.LabelNames()
		if len(labels) > 30 {
			labels = labels[:27] + "..."
		}
		rows = append(rows, []string{
			issue.RepositoryName,
			strconv.Itoa(issue.Number),
			truncateText(issue.Title, 50),
			issue.AuthorName(),
			issue.State,
			labels,
			strconv.Itoa(issue.Comments),
			RelativeTime(issue.CreatedAt),
			fmt.Sprintf(HyperLinkFormat, issue.HTMLUrl, "Open"),
		})
	}

	if len(rows) > 0 {
		headers := []string{"Repository", "#", "Title", "Author", "State", "Labels", "Comments", "Created", "URL"}
		if compact {
			PrintCompactTable(headers, rows)
			return
		}
		rows = append(rows, []string{"Total Issues", strconv.Itoa(len(rows))})
		fmt.Println()
		t := table.New().
			Width(TerminalWidth()).
			Border(lipgloss.RoundedBorder()).
			BorderStyle(BorderStyle).
			BorderRow(true).
			StyleFunc(func(row, col int) lipgloss.Style {
				return issueTableStyle(row, col, rows)
			}).
			Headers(
				SortIndicator(repositoryNameDisplayName, sortBy, "repo"),
				"#",
				SortIndicator("Title", sortBy, "title"),
				SortIndicator("Author", sortBy, "author"),
				SortIndicator("State", sortBy, "state"),
				"Labels",
				"Comments",
				SortIndicator("Created", sortBy, "created"),
				"URL",
			).
			Rows(rows...)

		fmt.Println(t)
	}

	printErrorMessageMap(errorMessageMap)
}

func PrintIssueDetail(issue model.IssueResponse, comments []model.IssueComment) {
	if issue.ErrorMessage != "" {
		fmt.Println(lipgloss.NewStyle().Foreground(Red).Bold(true).Render("  Error: " + issue.ErrorMessage))
		return
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(Blue)
	repoStyle := lipgloss.NewStyle().Foreground(Cyan)
	valueStyle := lipgloss.NewStyle().Foreground(White)
	stateStyle := lipgloss.NewStyle().Foreground(StatusColor(issue.State)).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(Dimmed)

	fmt.Println()
	fmt.Printf("  %s #%d\n", repoStyle.Render(issue.RepositoryName), issue.Number)
	fmt.Println(headerStyle.Render("  " + strings.Repeat("─", 50)))
	fmt.Printf("  %s: %s\n", headerStyle.Render("Title"), valueStyle.Render(issue.Title))
	fmt.Printf("  %s: %s\n", headerStyle.Render("State"), stateStyle.Render(issue.State))
	fmt.Printf("  %s: %s\n", headerStyle.Render("Author"), valueStyle.Render(issue.AuthorName()))

	if len(issue.Assignees) > 0 {
		names := make([]string, 0, len(issue.Assignees))
		for _, a := range issue.Assignees {
			if a.Login != "" {
				names = append(names, a.Login)
			}
		}
		fmt.Printf("  %s: %s\n", headerStyle.Render("Assignees"), valueStyle.Render(strings.Join(names, ", ")))
	}

	if issue.LabelNames() != "" {
		fmt.Printf("  %s: %s\n", headerStyle.Render("Labels"), valueStyle.Render(issue.LabelNames()))
	}

	if issue.Milestone != nil {
		fmt.Printf("  %s: %s\n", headerStyle.Render("Milestone"), valueStyle.Render(issue.Milestone.Title))
	}

	fmt.Printf("  %s: %s\n", headerStyle.Render("Comments"), valueStyle.Render(strconv.Itoa(issue.Comments)))
	fmt.Printf("  %s: %s\n", headerStyle.Render("Created"), valueStyle.Render(RelativeTime(issue.CreatedAt)))
	fmt.Printf("  %s: %s\n", headerStyle.Render("Updated"), valueStyle.Render(RelativeTime(issue.UpdatedAt)))

	if issue.ClosedAt != "" {
		fmt.Printf("  %s: %s\n", headerStyle.Render("Closed"), valueStyle.Render(RelativeTime(issue.ClosedAt)))
		if issue.ClosedBy.Login != "" {
			fmt.Printf("  %s: %s\n", headerStyle.Render("Closed By"), valueStyle.Render(issue.ClosedBy.Login))
		}
	}

	if issue.Body != "" {
		fmt.Println()
		fmt.Println(headerStyle.Render("  Body:"))
		body := issue.Body
		if len(body) > 500 {
			body = body[:497] + "..."
		}
		for _, line := range strings.Split(body, "\n") {
			fmt.Printf("    %s\n", dimStyle.Render(line))
		}
	}

	if len(comments) > 0 {
		fmt.Println()
		fmt.Println(headerStyle.Render(fmt.Sprintf("  Comments (%d):", len(comments))))
		for i, c := range comments {
			if i >= 5 {
				fmt.Printf("    %s\n", dimStyle.Render(fmt.Sprintf("... and %d more", len(comments)-5)))
				break
			}
			author := c.Author.Login
			if author == "" {
				author = c.Author.Name
			}
			body := c.Body
			if len(body) > 120 {
				body = body[:117] + "..."
			}
			body = strings.ReplaceAll(body, "\n", " ")
			fmt.Printf("    %s (%s): %s\n",
				valueStyle.Render(author),
				dimStyle.Render(RelativeTime(c.CreatedAt)),
				dimStyle.Render(body),
			)
		}
	}

	if issue.HTMLUrl != "" {
		fmt.Println()
		fmt.Printf("  %s: %s\n", headerStyle.Render("URL"), valueStyle.Render(issue.HTMLUrl))
	}
	fmt.Println()
}

func issueTableStyle(row, col int, rows [][]string) lipgloss.Style {
	style := defaultTableStyle(row, col, len(rows), 0, true)

	if row >= 0 {
		switch col {
		case 1, 6, 8:
			style = style.Align(lipgloss.Center)
		}
		if row < len(rows)-1 && col == 4 {
			style = style.Foreground(StatusColor(rows[row][4]))
		}
	}
	return style
}

func PrintAuditLog(entries []model.AuditLogEntry, compact bool) {
	if len(entries) == 0 {
		PrintNoDataMessage("No audit log entries found.",
			"Hint: check that your token has the 'admin:org' scope and audit log is enabled for your org.")
		return
	}
	rows := make([][]string, 0, len(entries))
	for _, e := range entries {
		repo := e.Repo
		if repo == "" {
			repo = "-"
		}
		ts := ""
		if e.CreatedAt > 0 {
			ts = time.UnixMilli(e.CreatedAt).UTC().Format("2006-01-02 15:04:05")
		}
		rows = append(rows, []string{ts, e.Actor, e.Action, repo})
	}
	headers := []string{"Time", "Actor", "Action", "Repo"}
	if compact {
		PrintCompactTable(headers, rows)
		return
	}
	rows = append(rows, []string{"Total Entries", strconv.Itoa(len(entries))})
	t := table.New().
		Width(TerminalWidth()).
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		StyleFunc(func(row, col int) lipgloss.Style {
			return defaultTableStyle(row, col, len(rows), -1, true)
		}).
		Headers(headers...).
		Rows(rows...)
	fmt.Println(t)
}

func SortIssues(issues []model.IssueResponse, sortBy string) {
	switch strings.ToLower(sortBy) {
	case "repo":
		sort.Slice(issues, func(i, j int) bool { return issues[i].RepositoryName < issues[j].RepositoryName })
	case "title":
		sort.Slice(issues, func(i, j int) bool { return issues[i].Title < issues[j].Title })
	case "author":
		sort.Slice(issues, func(i, j int) bool { return issues[i].AuthorName() < issues[j].AuthorName() })
	case "state":
		sort.Slice(issues, func(i, j int) bool { return issues[i].State < issues[j].State })
	case "created":
		sort.Slice(issues, func(i, j int) bool {
			ti, _ := time.Parse(time.RFC3339, issues[i].CreatedAt)
			tj, _ := time.Parse(time.RFC3339, issues[j].CreatedAt)
			return ti.After(tj)
		})
	}
}

// PrintOrganizations renders a table of GitHub organization details.
func PrintOrganizations(orgs []model.OrgDetail) {
	if len(orgs) == 0 {
		PrintNoDataMessage("No organization details found.",
			"Hint: verify the organization name(s) and your access token scope (read:org).")
		return
	}

	boolYN := func(b bool) string {
		if b {
			return "Yes"
		}
		return "No"
	}

	rows := make([][]string, 0, len(orgs)+1)
	for _, o := range orgs {
		name := o.Name
		if name == "" {
			name = o.Login
		}
		disk := fmt.Sprintf("%.1f MB", o.DiskUsageMB)
		rows = append(rows, []string{
			fmt.Sprintf(HyperLinkFormat, o.URL, name),
			o.Login,
			o.Description,
			o.Email,
			o.Location,
			o.WebsiteURL,
			strconv.Itoa(o.MembersCount),
			strconv.Itoa(o.TeamsCount),
			strconv.Itoa(o.ReposCount),
			boolYN(o.IsVerified),
			boolYN(o.RequiresTwoFA),
			disk,
			o.CreatedAt,
		})
	}
	rows = append(rows, []string{"Total Organizations", strconv.Itoa(len(orgs)), "", "", "", "", "", "", "", "", "", "", ""})

	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == -1 {
				return HeaderStyle
			}
			if row == len(rows)-1 {
				return FooterStyle
			}
			switch col {
			case 6, 7, 8:
				return CellStyle.Foreground(Subtle).Align(lipgloss.Center, lipgloss.Center)
			case 9, 10:
				return CellStyle.Foreground(Green).Align(lipgloss.Center, lipgloss.Center)
			}
			if row%2 == 0 {
				return EvenRowStyle
			}
			return OddRowStyle
		}).
		Headers("Name", "Login", "Description", "Email", "Location", "Website",
			"Members", "Teams", "Repos", "Verified", "2FA Required", "Disk Usage", "Created").
		Rows(rows...)

	fmt.Println(t)
}

// PrintWhoAmI renders the authenticated user's profile as a styled detail view.
func PrintWhoAmI(u *model.UserInfo) {
	if u == nil {
		PrintNoDataMessage("Could not fetch user info.",
			"Hint: verify GITHUB_TOKEN is set and valid.")
		return
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(Cyan)
	labelStyle := lipgloss.NewStyle().Foreground(Subtle).Width(20)
	valueStyle := lipgloss.NewStyle().Foreground(White)
	dimStyle := lipgloss.NewStyle().Foreground(Dimmed).Italic(true)
	numStyle := lipgloss.NewStyle().Foreground(Green).Bold(true)

	row := func(label, value string) {
		if value == "" {
			return
		}
		fmt.Printf("  %s %s\n", labelStyle.Render(label), valueStyle.Render(value))
	}
	numRow := func(label string, value int) {
		fmt.Printf("  %s %s\n", labelStyle.Render(label), numStyle.Render(strconv.Itoa(value)))
	}

	displayName := u.Name
	if displayName == "" {
		displayName = u.Login
	}

	fmt.Println()
	fmt.Printf("  %s\n", titleStyle.Render("  "+displayName))
	fmt.Println()

	row("Login", u.Login)
	row("Name", u.Name)
	row("Email", u.Email)
	row("Company", u.Company)
	row("Location", u.Location)
	row("Bio", u.Bio)
	row("Website", u.Blog)
	row("Twitter", u.TwitterUsername)

	fmt.Println()
	numRow("Public Repos", u.PublicRepos)
	numRow("Followers", u.Followers)
	numRow("Following", u.Following)

	if u.TotalPrivateRepos > 0 {
		numRow("Private Repos", u.TotalPrivateRepos)
	}
	if u.DiskUsage > 0 {
		row("Disk Usage", fmt.Sprintf("%.1f MB", float64(u.DiskUsage)/1024.0))
	}
	if u.Plan.Name != "" {
		row("Plan", u.Plan.Name)
	}

	fmt.Println()
	if u.HTMLUrl != "" {
		row("Profile", u.HTMLUrl)
	}
	if u.CreatedAt != "" {
		created := u.CreatedAt
		if t, err := time.Parse(time.RFC3339, u.CreatedAt); err == nil {
			created = t.Format("2006-01-02")
		}
		row("Member since", created)
	}

	fmt.Println()
	fmt.Printf("  %s\n", dimStyle.Render("Tip: use -J/--json for full profile data"))
	fmt.Println()
}
