// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package commit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

func TestListCommits_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/commits", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: []map[string]interface{}{
			{
				"sha":        "abc123",
				"node_id":    "MDY6Q29tbWl0",
				"html_url":   "https://github.com/testorg/repo1/commit/abc123",
				"commit_url": "https://api.github.com/repos/testorg/repo1/commits/abc123",
				"author":     map[string]interface{}{"login": "jane-doe"},
				"commit": map[string]interface{}{
					"message": "Fix the bug",
					"author":  map[string]interface{}{"name": "Jane Doe", "email": "jane@example.com", "date": "2024-01-01T00:00:00Z"},
				},
			},
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := ListCommits(ctx, CommitListRequest{
		OrgName:   "testorg",
		RepoNames: []string{"repo1"},
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	got := responses[0]
	if got.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", got.ErrorMessage)
	}
	if got.RepositoryName != "repo1" {
		t.Errorf("RepositoryName = %q, want %q", got.RepositoryName, "repo1")
	}
	if got.Sha != "abc123" {
		t.Errorf("Sha = %q, want %q", got.Sha, "abc123")
	}
	if got.Commit.Message != "Fix the bug" {
		t.Errorf("Commit.Message = %q, want %q", got.Commit.Message, "Fix the bug")
	}
}

func TestListCommits_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/commits", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := ListCommits(ctx, CommitListRequest{
		OrgName:   "testorg",
		RepoNames: []string{"repo1"},
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
	if responses[0].RepositoryName != "repo1" {
		t.Errorf("RepositoryName = %q, want %q", responses[0].RepositoryName, "repo1")
	}
}

// TestListCommits_QueryParameters verifies the "since"/"until" query string
// building logic (default-from-NoOfDays vs. explicit overrides). It uses a
// bespoke httptest server (rather than testutils.MockGitHubServer, which
// only records the request path, not its raw query) so the exact query
// string can be inspected.
func TestListCommits_QueryParameters(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	tests := []struct {
		name string
		req  CommitListRequest
		want []string // substrings expected in the request's raw query
	}{
		{
			name: "default since derived from NoOfDays",
			req:  CommitListRequest{OrgName: "testorg", RepoNames: []string{"repo1"}, NoOfDays: 5},
			want: []string{"since=" + time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -5).Format(time.RFC3339)},
		},
		{
			name: "explicit since/until override NoOfDays",
			req: CommitListRequest{
				OrgName: "testorg", RepoNames: []string{"repo1"}, NoOfDays: 30,
				Since: "2024-06-01T00:00:00Z", Until: "2024-06-30T00:00:00Z",
			},
			want: []string{"since=2024-06-01", "until=2024-06-30"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedQuery string
			captureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedQuery = r.URL.RawQuery
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("[]"))
			}))
			defer captureServer.Close()

			restore := servicetest.SetGitHubBaseURL(captureServer.URL)
			defer restore()

			ListCommits(ctx, tt.req)

			for _, want := range tt.want {
				if !strings.Contains(capturedQuery, want) {
					t.Errorf("query = %q, want substring %q", capturedQuery, want)
				}
			}
		})
	}
}

func TestGetCommitInfo_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/commits/abc123", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"sha":      "abc123",
			"html_url": "https://github.com/testorg/repo1/commit/abc123",
			"commit":   map[string]interface{}{"message": "Initial commit"},
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	resp := GetCommitInfo(ctx, CommitInfoRequest{OrgName: "testorg", RepoName: "repo1", CommitSHA: "abc123"})

	if resp.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", resp.ErrorMessage)
	}
	if resp.Sha != "abc123" {
		t.Errorf("Sha = %q, want %q", resp.Sha, "abc123")
	}
	if resp.Commit.Message != "Initial commit" {
		t.Errorf("Commit.Message = %q, want %q", resp.Commit.Message, "Initial commit")
	}
}

func TestGetCommitInfo_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/commits/deadbeef", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	resp := GetCommitInfo(ctx, CommitInfoRequest{OrgName: "testorg", RepoName: "repo1", CommitSHA: "deadbeef"})

	if resp.ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
	if resp.RepositoryName != "repo1" {
		t.Errorf("RepositoryName = %q, want %q", resp.RepositoryName, "repo1")
	}
}

func TestGetCommitCheckRuns_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/commits/abc123/check-runs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"total": 1,
			"check_runs": []map[string]interface{}{
				{"name": "build", "status": "completed", "conclusion": "success"},
			},
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	resp := GetCommitCheckRuns(ctx, CommitCheckRunsRequest{OrgName: "testorg", RepoName: "repo1", CommitSHA: "abc123"})

	if resp.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", resp.ErrorMessage)
	}
	if resp.Total != 1 {
		t.Errorf("Total = %d, want 1", resp.Total)
	}
	if len(resp.CheckRuns) != 1 || resp.CheckRuns[0].Name != "build" {
		t.Errorf("CheckRuns = %+v, want a single 'build' entry", resp.CheckRuns)
	}
}

func TestGetCommitCheckRuns_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/commits/abc123/check-runs", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	resp := GetCommitCheckRuns(ctx, CommitCheckRunsRequest{OrgName: "testorg", RepoName: "repo1", CommitSHA: "abc123"})

	if resp.ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
	if resp.RepositoryName != "repo1" {
		t.Errorf("RepositoryName = %q, want %q", resp.RepositoryName, "repo1")
	}
}
