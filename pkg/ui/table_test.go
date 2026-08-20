// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package ui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/pradyb/sgh-cli/internal/model"
)

// ---------------------------------------------------------------------------
// Pure helpers
// ---------------------------------------------------------------------------

func TestConvertToRows(t *testing.T) {
	t.Run("maps every element", func(t *testing.T) {
		data := []model.Repository{{Name: "a"}, {Name: "b"}, {Name: "c"}}
		rows := convertToRows(data, func(r model.Repository) []string {
			return []string{r.Name}
		})
		if len(rows) != 3 || rows[0][0] != "a" || rows[2][0] != "c" {
			t.Errorf("got %v, want 3 rows starting with a and ending with c", rows)
		}
	})

	t.Run("skips rows the handler declines", func(t *testing.T) {
		data := []model.Repository{{Name: "a"}, {Name: "b"}, {Name: "c"}}
		rows := convertToRows(data, func(r model.Repository) []string {
			if r.Name == "b" {
				return nil
			}
			return []string{r.Name}
		})
		if len(rows) != 2 || rows[0][0] != "a" || rows[1][0] != "c" {
			t.Errorf("got %v, want [a c] (b skipped)", rows)
		}
	})

	t.Run("empty input yields empty output", func(t *testing.T) {
		rows := convertToRows[model.Repository](nil, func(r model.Repository) []string { return []string{r.Name} })
		if len(rows) != 0 {
			t.Errorf("got %v, want empty", rows)
		}
	})
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		maxLen int
		want   string
	}{
		{"shorter than max unchanged", "short", 10, "short"},
		{"exactly max unchanged", "exact", 5, "exact"},
		{"longer gets ellipsis", "this is a long description", 10, "this is..."},
		{"maxLen at 3 has no room for ellipsis text", "abcdef", 3, "abc"},
		{"maxLen below 3", "abcdef", 2, "ab"},
		{"empty text", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncateText(tt.text, tt.maxLen); got != tt.want {
				t.Errorf("truncateText(%q, %d) = %q, want %q", tt.text, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		word  string
		count int
		want  string
	}{
		{"repository", 1, "repository"},
		{"repository", 2, "repositories"},
		{"repository", 0, "repositories"},
		{"item", 1, "item"},
		{"item", 5, "items"},
	}
	for _, tt := range tests {
		t.Run(tt.word+"_"+strconv.Itoa(tt.count), func(t *testing.T) {
			if got := pluralize(tt.word, tt.count); got != tt.want {
				t.Errorf("pluralize(%q, %d) = %q, want %q", tt.word, tt.count, got, tt.want)
			}
		})
	}
}

func TestDefaultTableStyle(t *testing.T) {
	t.Run("header row", func(t *testing.T) {
		s := defaultTableStyle(-1, 0, 5, 0, true)
		if !s.GetBold() || s.GetForeground() != Cyan || s.GetAlignHorizontal() != lipgloss.Center {
			t.Errorf("header style = %+v, want bold cyan centered", s)
		}
	})

	t.Run("even data row uses EvenRowStyle", func(t *testing.T) {
		s := defaultTableStyle(0, 1, 5, 3, false)
		if s.GetForeground() != Subtle {
			t.Errorf("foreground = %v, want Subtle", s.GetForeground())
		}
	})

	t.Run("odd data row uses OddRowStyle", func(t *testing.T) {
		s := defaultTableStyle(1, 1, 5, 3, false)
		if s.GetForeground() != White {
			t.Errorf("foreground = %v, want White", s.GetForeground())
		}
	})

	t.Run("repo column forced green regardless of parity", func(t *testing.T) {
		s := defaultTableStyle(1, 0, 5, 0, false)
		if s.GetForeground() != Green {
			t.Errorf("foreground = %v, want Green", s.GetForeground())
		}
	})

	t.Run("footer row overrides everything when present", func(t *testing.T) {
		s := defaultTableStyle(4, 0, 5, 0, true)
		if !s.GetBold() || s.GetForeground() != Cyan {
			t.Errorf("footer style = %+v, want bold cyan (FooterStyle)", s)
		}
	})

	t.Run("last row is not footer when isFooterPresent is false", func(t *testing.T) {
		s := defaultTableStyle(4, 1, 5, 3, false)
		if s.GetForeground() == Cyan {
			t.Errorf("foreground = %v, should not have become FooterStyle", s.GetForeground())
		}
	})
}

// ---------------------------------------------------------------------------
// Style functions (StyleFunc callbacks for each table)
// ---------------------------------------------------------------------------

func TestRepositoryTableStyle(t *testing.T) {
	rows := [][]string{
		{"repo1", "d", "main", "Go", "false", "ssh1", "link1", "0"},
		{"repo2", "d", "main", "Go", "false", "ssh2", "link2", "3"},
		{"Total Repositories", "2", "", "", "", "", "", "3"},
	}

	for _, col := range []int{3, 4, 6} {
		s := repositoryTableStyle(0, col, rows)
		if s.GetAlignHorizontal() != lipgloss.Center {
			t.Errorf("col %d not centered: %+v", col, s)
		}
	}

	if s := repositoryTableStyle(0, 7, rows); s.GetAlignHorizontal() != lipgloss.Right {
		t.Errorf("open-PR column not right-aligned: %+v", s)
	}

	// Non-zero open-PR count on a non-footer row is highlighted red.
	if s := repositoryTableStyle(1, 7, rows); s.GetForeground() != Red {
		t.Errorf("nonzero PR count should be red, got %v", s.GetForeground())
	}
	// Zero stays unstyled red.
	if s := repositoryTableStyle(0, 7, rows); s.GetForeground() == Red {
		t.Errorf("zero PR count should not be red")
	}
	// Footer row is excluded from the red-highlight check.
	if s := repositoryTableStyle(2, 7, rows); s.GetForeground() == Red {
		t.Errorf("footer row should not be red-highlighted")
	}
}

func TestPullRequestStyle(t *testing.T) {
	rows := [][]string{
		{"1", "repo", "title", "author", "assignee", "reviewer", "open / MERGEABLE", "refs", "link"},
		{"2", "repo2", "title2", "author2", "", "", "closed / DIRTY", "refs2", "link2"},
		{"", "Total Pull Requests", "2"},
	}

	for _, col := range []int{0, 6, 8} {
		s := pullRequestStyle(0, col, rows)
		if s.GetAlignHorizontal() != lipgloss.Center {
			t.Errorf("col %d not centered: %+v", col, s)
		}
	}

	if s := pullRequestStyle(1, 6, rows); !s.GetStrikethrough() {
		t.Errorf("closed PR status should be struck through")
	}
	if s := pullRequestStyle(0, 6, rows); s.GetStrikethrough() {
		t.Errorf("open PR status should not be struck through")
	}
	if s := pullRequestStyle(0, 6, rows); s.GetForeground() != StatusColor("open") {
		t.Errorf("status color mismatch: got %v want %v", s.GetForeground(), StatusColor("open"))
	}

	// Footer row must not be indexed into rows[row][6].
	if s := pullRequestStyle(2, 6, rows); s.GetForeground() == Red && false {
		_ = s // just ensure no panic occurred getting here
	}
}

func TestWorkflowRunTableStyle(t *testing.T) {
	rows := [][]string{
		{"repo", "1", "1", "CI", "success", "main", "push", "alice", "just now", "link"},
		{"repo2", "2", "2", "CI2", "failure", "dev", "push", "bob", "just now", "link2"},
		{"Total Workflow Runs", "2"},
	}

	for _, col := range []int{4, 6, 9} {
		s := workflowRunTableStyle(0, col, rows)
		if s.GetAlignHorizontal() != lipgloss.Center {
			t.Errorf("col %d not centered: %+v", col, s)
		}
	}
	for _, col := range []int{1, 2} {
		s := workflowRunTableStyle(0, col, rows)
		if s.GetAlignHorizontal() != lipgloss.Right {
			t.Errorf("col %d not right-aligned: %+v", col, s)
		}
	}
	if s := workflowRunTableStyle(1, 4, rows); s.GetForeground() != StatusColor("failure") {
		t.Errorf("status color mismatch: got %v want %v", s.GetForeground(), StatusColor("failure"))
	}
}

func TestSecretAlertTableStyle(t *testing.T) {
	rows := [][]string{
		{"repo", "1", "open", "aws_key", "path.go:1", "just now", "link"},
		{"Total Alerts", "1"},
	}
	for _, col := range []int{1, 6} {
		s := secretAlertTableStyle(0, col, rows)
		if s.GetAlignHorizontal() != lipgloss.Center {
			t.Errorf("col %d not centered: %+v", col, s)
		}
	}
	if s := secretAlertTableStyle(0, 2, rows); s.GetForeground() != StatusColor("open") {
		t.Errorf("state color mismatch: got %v want %v", s.GetForeground(), StatusColor("open"))
	}
}

func TestIssueTableStyle(t *testing.T) {
	rows := [][]string{
		{"repo", "1", "title", "author", "closed", "bug", "2", "just now", "link"},
		{"Total Issues", "1"},
	}
	for _, col := range []int{1, 6, 8} {
		s := issueTableStyle(0, col, rows)
		if s.GetAlignHorizontal() != lipgloss.Center {
			t.Errorf("col %d not centered: %+v", col, s)
		}
	}
	if s := issueTableStyle(0, 4, rows); s.GetForeground() != StatusColor("closed") {
		t.Errorf("state color mismatch: got %v want %v", s.GetForeground(), StatusColor("closed"))
	}
}

// ---------------------------------------------------------------------------
// getProtectedBranches / getRepoCommitsMap
// ---------------------------------------------------------------------------

func TestGetProtectedBranches(t *testing.T) {
	pbs := []model.ProtectedBranch{
		{
			RepositoryName: "repoB",
			Name:           "main",
			Type:           "branch_protection",
			EnforceAdmins:  true,
			LockBranch:     true,
			RequiredPullRequestReviews: model.RequiredPullRequestReviews{
				RequiredApprovingReviewCount: 2,
				RequireCodeOwnerReviews:      true,
				RequireLastPushApproval:      false,
				DismissStaleReviews:          true,
				BypassPullRequestAllowances: model.UserTeam{
					Users: []model.User{{Name: "alice"}},
				},
			},
			RequiredStatusChecks: model.RequiredStatusChecks{Contexts: []string{"ci", "lint"}},
			Restrictions:         model.Restriction{Users: []model.User{{Name: "bob"}}},
		},
		{RepositoryName: "repoA", Name: "main", Type: "NA"},
		{
			RepositoryName:         "repoC",
			Name:                   "develop",
			Type:                   "ruleset",
			LockBranch:             true,
			RepositoryRulesetNames: []string{"rule1"},
		},
		{RepositoryName: "repoD", Name: "x", ErrorMessage: "boom"},
	}

	rows, failed := getProtectedBranches(pbs)

	if len(failed) != 1 || failed[0][0] != "repoD" || failed[0][1] != "boom" {
		t.Fatalf("failedRows = %v, want [[repoD boom]]", failed)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %v, want 3 entries", rows)
	}

	// Sorted by Type first ("NA" < "branch_protection" < "ruleset"), then repo name.
	if rows[0][0] != "repoA" || rows[0][2] != "NA" {
		t.Errorf("rows[0] = %v, want repoA/NA first", rows[0])
	}
	for i := 3; i < len(rows[0]); i++ {
		if rows[0][i] != "" {
			t.Errorf("NA row should have blank details, got %v", rows[0])
			break
		}
	}

	if rows[1][0] != "repoB" {
		t.Fatalf("rows[1] = %v, want repoB second", rows[1])
	}
	want := []string{"repoB", "main", "branch_protection", "true", "2", "true", "false", "true", "ci,lint", "true", "alice", "bob", ""}
	for i, w := range want {
		if rows[1][i] != w {
			t.Errorf("rows[1][%d] = %q, want %q (full row: %v)", i, rows[1][i], w, rows[1])
		}
	}

	if rows[2][0] != "repoC" {
		t.Fatalf("rows[2] = %v, want repoC third", rows[2])
	}
	// LockBranch column is blanked out because rulesets are present.
	if rows[2][9] != "" {
		t.Errorf("rows[2] lockBranch column = %q, want empty (rulesets present)", rows[2][9])
	}
	if rows[2][12] != "rule1" {
		t.Errorf("rows[2] rulesets column = %q, want rule1", rows[2][12])
	}
}

func newCommit(repo, committerName string, errMsg string) model.CommitResponse {
	var c model.CommitResponse
	c.RepositoryName = repo
	c.ErrorMessage = errMsg
	c.Commit.Committer.Name = committerName
	c.Commit.Author.Name = "author-" + repo
	c.Commit.Author.Date = "2026-01-01T00:00:00Z"
	c.Commit.Message = "a commit message"
	c.Sha = "abcdef1234567890"
	c.HtmlUrl = "https://github.com/org/" + repo + "/commit/abc"
	return c
}

func TestGetRepoCommitsMap(t *testing.T) {
	commits := []model.CommitResponse{
		newCommit("repo1", "alice", ""),
		newCommit("repo1", "GitHub", ""),  // merge commit
		newCommit("repo2", "bob", "boom"), // errored, always excluded
	}

	t.Run("excludes merge commits by default", func(t *testing.T) {
		m := getRepoCommitsMap(commits, false)
		if len(m["repo1"]) != 1 {
			t.Errorf("repo1 commits = %d, want 1 (GitHub committer excluded)", len(m["repo1"]))
		}
		if _, ok := m["repo2"]; ok {
			t.Errorf("repo2 should be excluded entirely (errored commit)")
		}
	})

	t.Run("includes merge commits when requested", func(t *testing.T) {
		m := getRepoCommitsMap(commits, true)
		if len(m["repo1"]) != 2 {
			t.Errorf("repo1 commits = %d, want 2", len(m["repo1"]))
		}
	})
}

// ---------------------------------------------------------------------------
// RenderWorkflowRunDetail
// ---------------------------------------------------------------------------

func TestRenderWorkflowRunDetail(t *testing.T) {
	t.Run("error message short-circuits", func(t *testing.T) {
		out := RenderWorkflowRunDetail(model.WorkflowRunDetail{ErrorMessage: "boom"})
		if !strings.Contains(out, "boom") || !strings.Contains(out, "Error") {
			t.Errorf("output = %q, want it to mention the error", out)
		}
	})

	t.Run("no jobs", func(t *testing.T) {
		detail := model.WorkflowRunDetail{
			Run: model.WorkflowRun{
				RepositoryName: "repo1",
				ID:             1,
				RunNumber:      2,
				Name:           "CI",
				Status:         "completed",
				Conclusion:     "success",
				HeadBranch:     "main",
				Event:          "push",
				Actor:          model.User{Login: "alice"},
				HTMLUrl:        "https://github.com/org/repo1/actions/runs/1",
			},
		}
		out := RenderWorkflowRunDetail(detail)
		for _, want := range []string{"repo1", "CI", "main", "alice", "No jobs found"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("with jobs and steps", func(t *testing.T) {
		detail := model.WorkflowRunDetail{
			Run: model.WorkflowRun{
				RepositoryName: "repo1",
				RunNumber:      2,
				Name:           "CI",
				Status:         "completed",
				Conclusion:     "success",
				Actor:          model.User{Name: "Alice"},
				HTMLUrl:        "https://github.com/org/repo1/actions/runs/1",
			},
			Jobs: []model.WorkflowJob{
				{
					Name:       "build",
					Status:     "completed",
					Conclusion: "success",
					HTMLUrl:    "https://github.com/org/repo1/actions/runs/1/jobs/1",
					Steps: []model.WorkflowStep{
						{Name: "checkout", Status: "completed", Conclusion: "success"},
					},
				},
			},
		}
		out := RenderWorkflowRunDetail(detail)
		for _, want := range []string{"Jobs", "build", "checkout", detail.Run.HTMLUrl} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})
}

func TestPrintWorkflowRunDetail(t *testing.T) {
	out := captureStdout(t, func() {
		PrintWorkflowRunDetail(model.WorkflowRunDetail{ErrorMessage: "boom"})
	})
	if !strings.Contains(out, "boom") {
		t.Errorf("output missing error message:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Print* functions
// ---------------------------------------------------------------------------

func TestPrintRepositories(t *testing.T) {
	repos := []model.Repository{
		{Name: "api-gateway", Description: "The API gateway", DefaultBranch: "main", Language: "Go",
			Private: true, SSHUrl: "git@github.com:org/api-gateway.git", HTMLUrl: "https://github.com/org/api-gateway",
			OpenPullRequestsCount: 3},
		{Name: "docs", DefaultBranch: "main", HTMLUrl: "https://github.com/org/docs", OpenPullRequestsCount: 0},
	}
	out := captureStdout(t, func() { PrintRepositories(repos) })
	for _, want := range []string{"api-gateway", "docs", "Total Repositories", "2"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPrintResponses(t *testing.T) {
	t.Run("empty triggers no-data message", func(t *testing.T) {
		out := captureStdout(t, func() { PrintResponses(nil) })
		if !strings.Contains(out, "No data found") {
			t.Errorf("output = %q, want no-data message", out)
		}
	})

	t.Run("mixed success and failure", func(t *testing.T) {
		responses := []model.RefUIResponse{
			{RepositoryName: "repo1", SuccessMessage: "created"},
			{RepositoryName: "repo2", ErrorMessage: "already exists"},
		}
		out := captureStdout(t, func() { PrintResponses(responses) })
		for _, want := range []string{"repo1", "created", "repo2", "already exists", "Failed to process"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})
}

func TestPrintBranches(t *testing.T) {
	branches := []model.BranchResponse{
		{Name: "main", RepositoryName: "repo1", Protected: true},
		{Name: "feature", RepositoryName: "repo2", Protected: false},
	}
	branches[0].Commit.SHA = "abc1234567890"
	branches[1].Commit.SHA = "def4567890123"

	t.Run("empty", func(t *testing.T) {
		out := captureStdout(t, func() { PrintBranches(nil, "org", false, "") })
		if !strings.Contains(out, "No branches found") {
			t.Errorf("output = %q, want no-data message", out)
		}
	})

	t.Run("table mode", func(t *testing.T) {
		out := captureStdout(t, func() { PrintBranches(branches, "myorg", false, "repo") })
		for _, want := range []string{"main", "repo1", "feature", "repo2", "Total"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("compact mode is tab separated", func(t *testing.T) {
		out := captureStdout(t, func() { PrintBranches(branches, "myorg", true, "") })
		if !strings.Contains(out, "main\t") {
			t.Errorf("compact output not tab-separated:\n%s", out)
		}
	})
}

func TestPrintTags(t *testing.T) {
	tags := []model.TagResponse{
		{Name: "v1.0.0", RepositoryName: "repo1"},
		{Name: "v2.0.0", RepositoryName: "repo2"},
	}
	tags[0].Commit.SHA = "abc1234567890"
	tags[1].Commit.SHA = "def4567890123"

	t.Run("empty", func(t *testing.T) {
		out := captureStdout(t, func() { PrintTags(nil, false, "") })
		if !strings.Contains(out, "No tags found") {
			t.Errorf("output = %q, want no-data message", out)
		}
	})

	t.Run("table mode", func(t *testing.T) {
		out := captureStdout(t, func() { PrintTags(tags, false, "tag") })
		for _, want := range []string{"v1.0.0", "repo1", "v2.0.0", "Total"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("compact mode", func(t *testing.T) {
		out := captureStdout(t, func() { PrintTags(tags, true, "") })
		if !strings.Contains(out, "v1.0.0\t") {
			t.Errorf("compact output not tab-separated:\n%s", out)
		}
	})
}

func makePR(repo string, num int, state, mergeState string) model.PullRequestResponse {
	return model.PullRequestResponse{
		PRNumber:         num,
		TitleName:        "Fix the bug",
		State:            state,
		MergeStateStatus: mergeState,
		HTMLUrl:          "https://github.com/org/" + repo + "/pull/" + strconv.Itoa(num),
		Base:             model.PRBranch{Ref: "main", Repo: model.Repository{Name: repo}},
		Head:             model.PRBranch{Ref: "feature"},
		Author:           model.User{Login: "alice"},
	}
}

func TestPrintPullRequestResponses(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := captureStdout(t, func() { PrintPullRequestResponses(nil, "", false) })
		if !strings.Contains(out, "No Pull Requests found") {
			t.Errorf("output = %q, want no-data message", out)
		}
	})

	t.Run("mixed success and error", func(t *testing.T) {
		errPR := makePR("repo2", 2, "closed", "DIRTY")
		errPR.ErrorMessage = "not found"
		prs := []model.PullRequestResponse{makePR("repo1", 1, "open", "MERGEABLE"), errPR}
		out := captureStdout(t, func() { PrintPullRequestResponses(prs, "repo", false) })
		for _, want := range []string{"repo1", "Fix the bug", "alice", "Total Pull Requests", "Failed to process"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("compact mode", func(t *testing.T) {
		prs := []model.PullRequestResponse{makePR("repo1", 1, "open", "MERGEABLE")}
		out := captureStdout(t, func() { PrintPullRequestResponses(prs, "", true) })
		if !strings.Contains(out, "repo1\t") {
			t.Errorf("compact output not tab-separated:\n%s", out)
		}
	})
}

func TestPrintMergeResponses(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := captureStdout(t, func() { PrintMergeResponses(nil) })
		if !strings.Contains(out, "No Merge Responses found") {
			t.Errorf("output = %q, want no-data message", out)
		}
	})

	t.Run("mixed success and error", func(t *testing.T) {
		responses := []model.MergeResponse{
			{RepositoryName: "repo1", Message: "merged ok", SHA: "abc123"},
			{RepositoryName: "repo2", ErrorMessage: `{"documentation_url":"https://docs"}`},
		}
		out := captureStdout(t, func() { PrintMergeResponses(responses) })
		for _, want := range []string{"repo1", "merged ok", "repo2", "documentation_url"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})
}

func TestPrintPRDetail(t *testing.T) {
	t.Run("error short-circuits", func(t *testing.T) {
		out := captureStdout(t, func() {
			PrintPRDetail(model.PullRequestResponse{ErrorMessage: "boom"}, model.PullRequestFilesResponse{}, model.CheckRunResponse{}, nil)
		})
		if !strings.Contains(out, "boom") {
			t.Errorf("output missing error message:\n%s", out)
		}
	})

	t.Run("full detail with files, checks and reviews", func(t *testing.T) {
		pr := model.PullRequestResponse{
			PRNumber:         7,
			TitleName:        "Add feature",
			State:            "open",
			MergeStateStatus: "MERGEABLE",
			HTMLUrl:          "https://github.com/org/repo1/pull/7",
			Base:             model.PRBranch{Ref: "main", Repo: model.Repository{Name: "repo1"}},
			Head:             model.PRBranch{Ref: "feature"},
			Author:           model.User{Login: "alice"},
			Assignees:        []model.User{{Login: "bob"}},
			Reviewers:        []model.Actor{{Type: "User", User: model.User{Login: "carol"}}},
			MergedBy:         model.User{Login: "dave"},
			MergeAt:          "2026-01-01T00:00:00Z",
			Additions:        10,
			Deletions:        5,
			ChangedFiles:     15,
			Commits:          3,
			Comments:         2,
			ReviewComments:   1,
			Body:             strings.Repeat("x", 400),
		}
		files := model.PullRequestFilesResponse{
			Files: []model.PullRequestFile{
				{Filename: "a.go", ChangeType: "added", Additions: 5},
				{Filename: "b.go", ChangeType: "removed", Deletions: 3},
				{Filename: "c.go", ChangeType: "renamed"},
				{Filename: "d.go", ChangeType: "modified"},
			},
		}
		// Push past the "10 files" truncation branch.
		for i := 0; i < 10; i++ {
			files.Files = append(files.Files, model.PullRequestFile{Filename: "extra.go", ChangeType: "modified"})
		}
		checks := model.CheckRunResponse{CheckRuns: []model.CheckRun{{Name: "build", Status: "completed", Conclusion: "success"}}}
		reviews := []model.ReviewPullRequestResponse{{User: model.User{Login: "carol"}, State: "approved", SubmittedAt: "2026-01-01T00:00:00Z"}}

		out := captureStdout(t, func() { PrintPRDetail(pr, files, checks, reviews) })
		for _, want := range []string{
			"Add feature", "alice", "bob", "carol", "dave", "a.go", "b.go", "c.go",
			"and 4 more files", "build", "approved",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})
}

func TestPrintReviewResponse(t *testing.T) {
	t.Run("error short-circuits", func(t *testing.T) {
		out := captureStdout(t, func() {
			PrintReviewResponse(model.ReviewPullRequestResponse{ErrorMessage: "boom"})
		})
		if !strings.Contains(out, "boom") {
			t.Errorf("output missing error message:\n%s", out)
		}
	})

	t.Run("success", func(t *testing.T) {
		out := captureStdout(t, func() {
			PrintReviewResponse(model.ReviewPullRequestResponse{
				RepositoryName: "repo1",
				PRNumber:       3,
				State:          "approved",
				Body:           "looks good",
				SubmittedAt:    "2026-01-01T00:00:00Z",
			})
		})
		for _, want := range []string{"repo1", "approved", "looks good"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})
}

func TestPrintProtectedBranches(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := captureStdout(t, func() { PrintProtectedBranches(nil) })
		if !strings.Contains(out, "No Protected Branches found") {
			t.Errorf("output = %q, want no-data message", out)
		}
	})

	t.Run("mixed success and failure", func(t *testing.T) {
		pbs := []model.ProtectedBranch{
			{RepositoryName: "repo1", Name: "main", Type: "branch_protection", EnforceAdmins: false},
			{RepositoryName: "repo2", Name: "main", ErrorMessage: "not protected"},
		}
		out := captureStdout(t, func() { PrintProtectedBranches(pbs) })
		for _, want := range []string{"repo1", "Total Protected Branches", "repo2", "not protected", "Failed to process"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})
}

func TestPrintPostReleaseResponses(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := captureStdout(t, func() { PrintPostReleaseResponses(nil) })
		if !strings.Contains(out, "No post-release activity performed") {
			t.Errorf("output = %q, want no-data message", out)
		}
	})

	t.Run("branch, tag and error variants", func(t *testing.T) {
		responses := []model.PostReleaseResponse{
			{RepositoryName: "repo1", BranchName: "release/1.0", BranchSHA: "abc123", TagURL: "https://x/tag", TagName: "v1.0.0"},
			{RepositoryName: "repo2", BranchName: "release/2.0", TagName: "v2.0.0", TagSHA: "def456"},
			{RepositoryName: "repo3", ErrorMessage: "branch not found"},
		}
		out := captureStdout(t, func() { PrintPostReleaseResponses(responses) })
		for _, want := range []string{"repo1", "release/1.0", "v1.0.0", "repo2", "v2.0.0", "def456", "branch not found"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})
}

func TestPrintCommitResponses(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := captureStdout(t, func() { PrintCommitResponses(nil, false, "", false) })
		if !strings.Contains(out, "No Commits found") {
			t.Errorf("output = %q, want no-data message", out)
		}
	})

	buildCommits := func() []model.CommitResponse {
		var normal, merge, errored model.CommitResponse
		normal.RepositoryName = "repo1"
		normal.Sha = "abc1234567890"
		normal.HtmlUrl = "https://github.com/org/repo1/commit/abc"
		normal.Commit.Author.Name = "Alice"
		normal.Commit.Author.Date = "2026-01-01T00:00:00Z"
		normal.Commit.Committer.Name = "Alice"
		normal.Commit.Message = "Fix the thing\n\nLonger body here"

		merge.RepositoryName = "repo1"
		merge.Commit.Committer.Name = "GitHub"
		merge.Commit.Author.Name = "GitHub"
		merge.Commit.Author.Date = "2026-01-02T00:00:00Z"
		merge.Commit.Message = "Merge pull request #1"

		errored.RepositoryName = "repo2"
		errored.ErrorMessage = "not found"

		return []model.CommitResponse{normal, merge, errored}
	}

	t.Run("excludes merge commits by default", func(t *testing.T) {
		out := captureStdout(t, func() { PrintCommitResponses(buildCommits(), false, "", false) })
		for _, want := range []string{"repo1", "Fix the thing", "Total Commits", "not found"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, "Merge pull request") {
			t.Errorf("merge commit should have been excluded:\n%s", out)
		}
	})

	t.Run("includes merge commits when requested", func(t *testing.T) {
		out := captureStdout(t, func() { PrintCommitResponses(buildCommits(), true, "author", false) })
		if !strings.Contains(out, "Merge pull request") {
			t.Errorf("merge commit should be included:\n%s", out)
		}
	})

	t.Run("compact mode", func(t *testing.T) {
		out := captureStdout(t, func() { PrintCommitResponses(buildCommits(), false, "", true) })
		if !strings.Contains(out, "repo1\t") {
			t.Errorf("compact output not tab-separated:\n%s", out)
		}
	})
}

func TestPrintCommitSummary(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := captureStdout(t, func() { PrintCommitSummary(nil, false, "") })
		if !strings.Contains(out, "No Commits found") {
			t.Errorf("output = %q, want no-data message", out)
		}
	})

	t.Run("groups by repository", func(t *testing.T) {
		var c1, c2, errored model.CommitResponse
		c1.RepositoryName = "repo1"
		c1.Sha = "abc1234567890"
		c1.HtmlUrl = "https://x/1"
		c1.Commit.Author.Name = "Alice"
		c1.Commit.Author.Date = "2026-01-01T00:00:00Z"
		c1.Commit.Message = "First commit"

		c2.RepositoryName = "repo2"
		c2.Sha = "def4567890123"
		c2.HtmlUrl = "https://x/2"
		c2.Commit.Author.Name = "Bob"
		c2.Commit.Author.Date = "2026-01-02T00:00:00Z"
		c2.Commit.Message = "Second commit"

		errored.RepositoryName = "repo3"
		errored.ErrorMessage = "boom"

		out := captureStdout(t, func() {
			PrintCommitSummary([]model.CommitResponse{c1, c2, errored}, true, "repo")
		})
		for _, want := range []string{"repo1", "repo2", "First commit", "Second commit", "Total Commits", "boom"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})
}

func TestPrintTeams(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := captureStdout(t, func() { PrintTeams(nil) })
		if !strings.Contains(out, "No Teams found") {
			t.Errorf("output = %q, want no-data message", out)
		}
	})

	t.Run("with members", func(t *testing.T) {
		teams := []model.OrgTeam{
			{
				Name:              "platform",
				TotalMembers:      2,
				Url:               "https://github.com/orgs/x/teams/platform",
				RepositoriesCount: 5,
				Members: []model.OrgTeamMember{
					{Name: "Alice", PeopleUrl: "https://github.com/alice"},
					{Name: "Bob", PeopleUrl: "https://github.com/bob"},
				},
			},
		}
		out := captureStdout(t, func() { PrintTeams(teams) })
		for _, want := range []string{"platform", "Alice", "Bob", "Total Teams"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})
}

func makeRun(repo string, id int, status, conclusion string) model.WorkflowRun {
	return model.WorkflowRun{
		RepositoryName: repo,
		ID:             id,
		RunNumber:      id,
		Name:           "CI",
		Status:         status,
		Conclusion:     conclusion,
		HeadBranch:     "main",
		Event:          "push",
		Actor:          model.User{Login: "alice"},
		CreatedAt:      "2026-01-01T00:00:00Z",
		HTMLUrl:        "https://github.com/org/" + repo + "/actions/runs/" + strconv.Itoa(id),
	}
}

func TestPrintWorkflowRuns(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := captureStdout(t, func() { PrintWorkflowRuns(nil, "", false) })
		if !strings.Contains(out, "No Workflow Runs found") {
			t.Errorf("output = %q, want no-data message", out)
		}
	})

	t.Run("mixed success and error, actor login fallback", func(t *testing.T) {
		runInProgress := makeRun("repo1", 1, "in_progress", "")
		runDone := makeRun("repo1", 2, "completed", "success")
		runDone.Actor = model.User{Name: "Bob Actor"} // no login, falls back to Name
		errRun := model.WorkflowRun{RepositoryName: "repo2", ErrorMessage: "no access"}

		out := captureStdout(t, func() {
			PrintWorkflowRuns([]model.WorkflowRun{runInProgress, runDone, errRun}, "repo", false)
		})
		for _, want := range []string{"repo1", "in_progress", "success", "Bob Actor", "Total Workflow Runs", "no access"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("compact mode", func(t *testing.T) {
		out := captureStdout(t, func() { PrintWorkflowRuns([]model.WorkflowRun{makeRun("repo1", 1, "completed", "success")}, "", true) })
		if !strings.Contains(out, "repo1\t") {
			t.Errorf("compact output not tab-separated:\n%s", out)
		}
	})
}

func TestPrintSecretScanningAlerts(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := captureStdout(t, func() { PrintSecretScanningAlerts(nil, "", false) })
		if !strings.Contains(out, "No secret scanning alerts found") {
			t.Errorf("output = %q, want no-data message", out)
		}
	})

	t.Run("mixed success and error, location with line", func(t *testing.T) {
		alerts := []model.SecretScanningAlert{
			{
				Number: 1, RepositoryName: "repo1", State: "open", SecretType: "aws_access_key",
				Location:  model.SecretLocation{Path: "config.yml", StartLine: 12},
				CreatedAt: "2026-01-01T00:00:00Z", HTMLUrl: "https://x/1",
			},
			{RepositoryName: "repo2", ErrorMessage: "forbidden"},
		}
		out := captureStdout(t, func() { PrintSecretScanningAlerts(alerts, "state", false) })
		for _, want := range []string{"repo1", "aws_access_key", "config.yml:12", "Total Alerts", "forbidden"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("compact mode", func(t *testing.T) {
		alerts := []model.SecretScanningAlert{{Number: 1, RepositoryName: "repo1", State: "open", SecretType: "aws_access_key"}}
		out := captureStdout(t, func() { PrintSecretScanningAlerts(alerts, "", true) })
		if !strings.Contains(out, "repo1\t") {
			t.Errorf("compact output not tab-separated:\n%s", out)
		}
	})
}

func TestPrintSecretAlertDetail(t *testing.T) {
	t.Run("error short-circuits", func(t *testing.T) {
		out := captureStdout(t, func() { PrintSecretAlertDetail(model.SecretScanningAlert{ErrorMessage: "boom"}) })
		if !strings.Contains(out, "boom") {
			t.Errorf("output missing error message:\n%s", out)
		}
	})

	t.Run("full detail with resolution", func(t *testing.T) {
		alert := model.SecretScanningAlert{
			Number: 5, RepositoryName: "repo1", State: "resolved", SecretType: "aws_access_key",
			SecretTypeDisplay: "AWS Access Key",
			Location:          model.SecretLocation{Path: "config.yml", StartLine: 3, EndLine: 4, CommitSHA: "abc1234567890", BlobURL: "https://x/blob"},
			CreatedAt:         "2026-01-01T00:00:00Z",
			UpdatedAt:         "2026-01-02T00:00:00Z",
			Resolution:        "false_positive",
			ResolvedBy:        model.User{Login: "alice"},
			ResolvedAt:        "2026-01-03T00:00:00Z",
			ResolutionComment: "not a real secret",
			HTMLUrl:           "https://x/alert/5",
		}
		out := captureStdout(t, func() { PrintSecretAlertDetail(alert) })
		for _, want := range []string{
			"repo1", "AWS Access Key", "resolved", "config.yml", "abc1234", "https://x/blob",
			"false_positive", "alice", "not a real secret", "https://x/alert/5",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})
}

func TestPrintIssues(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := captureStdout(t, func() { PrintIssues(nil, "", false) })
		if !strings.Contains(out, "No issues found") {
			t.Errorf("output = %q, want no-data message", out)
		}
	})

	t.Run("mixed success and error, long labels truncated", func(t *testing.T) {
		longLabels := []model.IssueLabel{
			{Name: "bug-report-needs-triage"}, {Name: "high-priority-customer-facing"},
		}
		issues := []model.IssueResponse{
			{
				RepositoryName: "repo1", Number: 1, Title: "Something broke", State: "open",
				Author: model.User{Login: "alice"}, Labels: longLabels, Comments: 2,
				CreatedAt: "2026-01-01T00:00:00Z", HTMLUrl: "https://x/1",
			},
			{RepositoryName: "repo2", ErrorMessage: "forbidden"},
		}
		out := captureStdout(t, func() { PrintIssues(issues, "state", false) })
		for _, want := range []string{"repo1", "Something broke", "alice", "Total Issues", "forbidden"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
		if !strings.Contains(out, "...") {
			t.Errorf("labels over 30 chars should be truncated with ellipsis:\n%s", out)
		}
	})

	t.Run("compact mode", func(t *testing.T) {
		issues := []model.IssueResponse{{RepositoryName: "repo1", Number: 1, Title: "x", State: "open"}}
		out := captureStdout(t, func() { PrintIssues(issues, "", true) })
		if !strings.Contains(out, "repo1\t") {
			t.Errorf("compact output not tab-separated:\n%s", out)
		}
	})
}

func TestPrintIssueDetail(t *testing.T) {
	t.Run("error short-circuits", func(t *testing.T) {
		out := captureStdout(t, func() { PrintIssueDetail(model.IssueResponse{ErrorMessage: "boom"}, nil) })
		if !strings.Contains(out, "boom") {
			t.Errorf("output missing error message:\n%s", out)
		}
	})

	t.Run("full detail with all optional fields", func(t *testing.T) {
		issue := model.IssueResponse{
			RepositoryName: "repo1", Number: 42, Title: "Something broke", State: "closed",
			Author:    model.User{Login: "alice"},
			Assignees: []model.User{{Login: "bob"}, {Login: ""}}, // blank login is filtered out
			Labels:    []model.IssueLabel{{Name: "bug"}},
			Milestone: &model.IssueMilestone{Title: "v1.0"},
			Comments:  7,
			CreatedAt: "2026-01-01T00:00:00Z",
			UpdatedAt: "2026-01-02T00:00:00Z",
			ClosedAt:  "2026-01-03T00:00:00Z",
			ClosedBy:  model.User{Login: "carol"},
			Body:      strings.Repeat("y", 600),
			HTMLUrl:   "https://x/42",
		}
		comments := make([]model.IssueComment, 0, 7)
		for i := 0; i < 7; i++ {
			comments = append(comments, model.IssueComment{
				Author: model.User{Login: "dave"}, Body: "a comment", CreatedAt: "2026-01-01T00:00:00Z",
			})
		}
		out := captureStdout(t, func() { PrintIssueDetail(issue, comments) })
		for _, want := range []string{
			"Something broke", "closed", "alice", "bob", "bug", "v1.0", "carol",
			"and 2 more", "dave", "https://x/42",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("minimal detail with no optional fields", func(t *testing.T) {
		issue := model.IssueResponse{RepositoryName: "repo1", Number: 1, Title: "x", State: "open", CreatedAt: "2026-01-01T00:00:00Z"}
		out := captureStdout(t, func() { PrintIssueDetail(issue, nil) })
		if !strings.Contains(out, "repo1") {
			t.Errorf("output missing repo name:\n%s", out)
		}
	})
}

func TestPrintAuditLog(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := captureStdout(t, func() { PrintAuditLog(nil, false) })
		if !strings.Contains(out, "No audit log entries found") {
			t.Errorf("output = %q, want no-data message", out)
		}
	})

	t.Run("entries with and without repo/timestamp", func(t *testing.T) {
		entries := []model.AuditLogEntry{
			{Actor: "alice", Action: "repo.create", Repo: "repo1", CreatedAt: 1735689600000},
			{Actor: "bob", Action: "org.update_member", CreatedAt: 0}, // no repo, no timestamp
		}
		out := captureStdout(t, func() { PrintAuditLog(entries, false) })
		for _, want := range []string{"alice", "repo.create", "repo1", "bob", "org.update_member", "Total Entries"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("compact mode", func(t *testing.T) {
		entries := []model.AuditLogEntry{{Actor: "alice", Action: "repo.create"}}
		out := captureStdout(t, func() { PrintAuditLog(entries, true) })
		if !strings.Contains(out, "alice\t") {
			t.Errorf("compact output not tab-separated:\n%s", out)
		}
	})
}

func TestPrintOrganizations(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := captureStdout(t, func() { PrintOrganizations(nil) })
		if !strings.Contains(out, "No organization details found") {
			t.Errorf("output = %q, want no-data message", out)
		}
	})

	t.Run("name falls back to login", func(t *testing.T) {
		orgs := []model.OrgDetail{
			{Login: "acme-org", MembersCount: 10, TeamsCount: 2, ReposCount: 30, IsVerified: true, RequiresTwoFA: true, DiskUsageMB: 12.5},
			{Login: "second-org", Name: "Second Org", MembersCount: 1},
		}
		out := captureStdout(t, func() { PrintOrganizations(orgs) })
		for _, want := range []string{"acme-org", "Second Org", "Total Organizations", "12.5 MB", "Yes"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})
}

func TestPrintWhoAmI(t *testing.T) {
	t.Run("nil user", func(t *testing.T) {
		out := captureStdout(t, func() { PrintWhoAmI(nil) })
		if !strings.Contains(out, "Could not fetch user info") {
			t.Errorf("output = %q, want no-data message", out)
		}
	})

	t.Run("full profile", func(t *testing.T) {
		u := &model.UserInfo{
			Login: "alice", Name: "Alice Smith", Email: "alice@example.com", Company: "Acme",
			Location: "Earth", Bio: "hi", Blog: "https://alice.dev", TwitterUsername: "alice",
			PublicRepos: 10, Followers: 5, Following: 3, HTMLUrl: "https://github.com/alice",
			CreatedAt: "2020-01-01T00:00:00Z", TotalPrivateRepos: 4, DiskUsage: 2048,
		}
		u.Plan.Name = "pro"
		out := captureStdout(t, func() { PrintWhoAmI(u) })
		for _, want := range []string{"Alice Smith", "alice@example.com", "Acme", "10", "pro", "2020-01-01", "2.0 MB"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("minimal profile falls back to login and skips optional rows", func(t *testing.T) {
		u := &model.UserInfo{Login: "bob", CreatedAt: "not-a-timestamp"}
		out := captureStdout(t, func() { PrintWhoAmI(u) })
		for _, want := range []string{"bob", "not-a-timestamp"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// printErrorMessageMap (unexported, exercised indirectly above; direct
// coverage here pins the pluralization and formatting behaviour).
// ---------------------------------------------------------------------------

func TestPrintErrorMessageMap(t *testing.T) {
	t.Run("empty map prints nothing", func(t *testing.T) {
		out := captureStdout(t, func() { printErrorMessageMap(map[string][]string{}) })
		if out != "" {
			t.Errorf("output = %q, want empty", out)
		}
	})

	t.Run("singular vs plural repository count", func(t *testing.T) {
		out := captureStdout(t, func() {
			printErrorMessageMap(map[string][]string{"not found": {"repo1"}})
		})
		for _, want := range []string{"1 repository", "not found", "repo1"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}

		out = captureStdout(t, func() {
			printErrorMessageMap(map[string][]string{"not found": {"repo1", "repo2"}})
		})
		if !strings.Contains(out, "2 repositories") {
			t.Errorf("output missing plural count:\n%s", out)
		}
	})
}
