// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pradyb/sgh-cli/internal/testutils"
	appcontext "github.com/pradyb/sgh-cli/pkg/context"
)

// newTestCtx spins up a fresh mock GitHub server and a context wired to it,
// isolated per-test so response overrides never leak across test cases.
func newTestCtx(t *testing.T) (*testutils.MockGitHubServer, *appcontext.Context) {
	t.Helper()
	mockServer := testutils.NewMockGitHubServer()
	t.Cleanup(mockServer.Close)
	ctx := NewMockContext(t, mockServer)
	return mockServer, ctx
}

// ---------------------------------------------------------------------
// parseLinkNext
// ---------------------------------------------------------------------

func TestParseLinkNext(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"empty header", "", ""},
		{"no next rel", `<https://api.github.com/repos/o/r/branches?page=1>; rel="prev"`, ""},
		{"single next", `<https://api.github.com/repos/o/r/branches?page=2>; rel="next"`, "https://api.github.com/repos/o/r/branches?page=2"},
		{
			"next and last",
			`<https://api.github.com/repos/o/r/branches?page=2>; rel="next", <https://api.github.com/repos/o/r/branches?page=5>; rel="last"`,
			"https://api.github.com/repos/o/r/branches?page=2",
		},
		{
			"prev and next reversed order",
			`<https://api.github.com/repos/o/r/branches?page=5>; rel="last", <https://api.github.com/repos/o/r/branches?page=2>; rel="next", <https://api.github.com/repos/o/r/branches?page=1>; rel="prev"`,
			"https://api.github.com/repos/o/r/branches?page=2",
		},
		{"malformed missing angle brackets", `https://api.github.com/repos/o/r/branches?page=2; rel="next"`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLinkNext(tt.header)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ---------------------------------------------------------------------
// GetOwnerType
// ---------------------------------------------------------------------

func TestGetOwnerType(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/users/octo-org", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       map[string]interface{}{"type": "Organization"},
		})

		ownerType, err := GetOwnerType(ctx, "octo-org")

		require.NoError(t, err)
		assert.Equal(t, "Organization", ownerType)
	})

	t.Run("not found", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/users/ghost", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		_, err := GetOwnerType(ctx, "ghost")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "404")
	})
}

// ---------------------------------------------------------------------
// UpdateRepoArchived / UpdateRepoVisibility
// ---------------------------------------------------------------------

func TestUpdateRepoArchived(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       map[string]interface{}{"archived": true},
		})

		err := UpdateRepoArchived(ctx, testOrgName, testRepoName, true)

		require.NoError(t, err)
		requests := mockServer.GetRequests()
		require.Len(t, requests, 1)
		assert.Equal(t, "PATCH", requests[0].Method)
		assert.Contains(t, requests[0].Body, `"archived":true`)
	})

	t.Run("forbidden", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo", testutils.MockResponse{
			StatusCode: http.StatusForbidden,
			Body:       map[string]interface{}{"message": "Forbidden"},
		})

		err := UpdateRepoArchived(ctx, testOrgName, testRepoName, true)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "403")
	})
}

func TestUpdateRepoVisibility(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       map[string]interface{}{"visibility": "private"},
		})

		err := UpdateRepoVisibility(ctx, testOrgName, testRepoName, "private")

		require.NoError(t, err)
		requests := mockServer.GetRequests()
		require.Len(t, requests, 1)
		assert.Contains(t, requests[0].Body, `"visibility":"private"`)
	})

	t.Run("forbidden", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo", testutils.MockResponse{
			StatusCode: http.StatusForbidden,
			Body:       map[string]interface{}{"message": "Forbidden"},
		})

		err := UpdateRepoVisibility(ctx, testOrgName, testRepoName, "public")

		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------
// CreateNewBranchFromCommit / DeleteBranch / RenameBranch
// ---------------------------------------------------------------------

func TestCreateNewBranchFromCommit(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/git/refs", testutils.MockResponse{
			StatusCode: http.StatusCreated,
			Body: map[string]interface{}{
				"ref": "refs/heads/feature-x",
				"object": map[string]interface{}{
					"sha":  "commit-sha-123",
					"type": "commit",
				},
			},
		})

		resp, err := CreateNewBranchFromCommit(ctx, testOrgName, testRepoName, "feature-x", "commit-sha-123")

		require.NoError(t, err)
		assert.Equal(t, "refs/heads/feature-x", resp.Ref)
		assert.Equal(t, "commit-sha-123", resp.Object.SHA)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/git/refs", testutils.MockResponse{
			StatusCode: http.StatusUnprocessableEntity,
			Body:       map[string]interface{}{"message": "Reference already exists"},
		})

		_, err := CreateNewBranchFromCommit(ctx, testOrgName, testRepoName, "feature-x", "commit-sha-123")

		require.Error(t, err)
	})
}

func TestDeleteBranch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/git/refs/heads/feature-x", testutils.MockResponse{
			StatusCode: http.StatusNoContent,
		})

		ok, err := DeleteBranch(ctx, testOrgName, testRepoName, "feature-x")

		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("not found", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/git/refs/heads/ghost", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		ok, err := DeleteBranch(ctx, testOrgName, testRepoName, "ghost")

		require.Error(t, err)
		assert.False(t, ok)
	})
}

func TestRenameBranch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/branches/old-name/rename", testutils.MockResponse{
			StatusCode: http.StatusCreated,
			Body:       map[string]interface{}{"name": "new-name"},
		})

		err := RenameBranch(ctx, testOrgName, testRepoName, "old-name", "new-name")

		require.NoError(t, err)
		requests := mockServer.GetRequests()
		require.Len(t, requests, 1)
		assert.Contains(t, requests[0].Body, `"new_name":"new-name"`)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/branches/old-name/rename", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		err := RenameBranch(ctx, testOrgName, testRepoName, "old-name", "new-name")

		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------
// ListBranches pagination (Link-header cursor)
// ---------------------------------------------------------------------

func TestListBranches_Pagination(t *testing.T) {
	_, ctx := newTestCtx(t)

	var paginationServer *httptest.Server
	callCount := 0
	paginationServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			w.Header().Set("Link", `<`+paginationServer.URL+`/repos/testorg/test-repo/branches?per_page=100&page=2>; rel="next"`)
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"name": "branch-a", "protected": false, "commit": map[string]interface{}{"sha": "sha-a"}},
			})
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{"name": "branch-b", "protected": true, "commit": map[string]interface{}{"sha": "sha-b"}},
		})
	}))
	defer paginationServer.Close()
	t.Cleanup(SetGitHubBaseURLForTesting(paginationServer.URL))

	branches, err := ListBranches(ctx, testOrgName, testRepoName)

	require.NoError(t, err)
	require.Len(t, branches, 2)
	assert.Equal(t, "branch-a", branches[0].Name)
	assert.Equal(t, "branch-b", branches[1].Name)
	assert.Equal(t, 2, callCount)
}

// ---------------------------------------------------------------------
// CreateNewTag / ListTags / DeleteTag
// ---------------------------------------------------------------------

func TestCreateNewTag(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/git/ref/heads/main", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       map[string]interface{}{"object": map[string]interface{}{"sha": "base-sha"}},
		})
		mockServer.SetResponse("/repos/testorg/test-repo/git/tags", testutils.MockResponse{
			StatusCode: http.StatusCreated,
			Body:       map[string]interface{}{"sha": "tag-object-sha"},
		})
		mockServer.SetResponse("/repos/testorg/test-repo/git/refs", testutils.MockResponse{
			StatusCode: http.StatusCreated,
			Body: map[string]interface{}{
				"ref":    "refs/tags/v1.0.0",
				"object": map[string]interface{}{"sha": "tag-object-sha", "type": "tag"},
			},
		})

		resp, err := CreateNewTag(ctx, testOrgName, testRepoName, "v1.0.0", "main", "Release v1.0.0")

		require.NoError(t, err)
		assert.Equal(t, "refs/tags/v1.0.0", resp.Ref)
		assert.Equal(t, "tag-object-sha", resp.Object.SHA)

		requests := mockServer.GetRequests()
		require.Len(t, requests, 3)
		assert.Contains(t, requests[1].Body, `"tag":"v1.0.0"`)
		assert.Contains(t, requests[1].Body, `"object":"base-sha"`)
		assert.Contains(t, requests[1].Body, `"message":"Release v1.0.0"`)
	})

	t.Run("error fetching ref sha", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/git/ref/heads/main", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		_, err := CreateNewTag(ctx, testOrgName, testRepoName, "v1.0.0", "main", "Release v1.0.0")

		require.Error(t, err)
	})

	t.Run("error creating tag object", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/git/ref/heads/main", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       map[string]interface{}{"object": map[string]interface{}{"sha": "base-sha"}},
		})
		mockServer.SetResponse("/repos/testorg/test-repo/git/tags", testutils.MockResponse{
			StatusCode: http.StatusForbidden,
			Body:       map[string]interface{}{"message": "Forbidden"},
		})

		_, err := CreateNewTag(ctx, testOrgName, testRepoName, "v1.0.0", "main", "Release v1.0.0")

		require.Error(t, err)
	})
}

func TestListTags(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/tags", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body: []map[string]interface{}{
				{"name": "v1.0.0", "commit": map[string]interface{}{"sha": "sha-1"}},
			},
		})

		tags, err := ListTags(ctx, testOrgName, testRepoName)

		require.NoError(t, err)
		require.Len(t, tags, 1)
		assert.Equal(t, "v1.0.0", tags[0].Name)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/tags", testutils.MockResponse{
			StatusCode: http.StatusForbidden,
			Body:       map[string]interface{}{"message": "Forbidden"},
		})

		_, err := ListTags(ctx, testOrgName, testRepoName)

		require.Error(t, err)
	})
}

func TestDeleteTag(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/git/refs/tags/v1.0.0", testutils.MockResponse{
			StatusCode: http.StatusNoContent,
		})

		ok, err := DeleteTag(ctx, testOrgName, testRepoName, "v1.0.0")

		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/git/refs/tags/ghost", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		ok, err := DeleteTag(ctx, testOrgName, testRepoName, "ghost")

		require.Error(t, err)
		assert.False(t, ok)
	})
}

// ---------------------------------------------------------------------
// Pull request functions
// ---------------------------------------------------------------------

func TestCreateNewPullRequest(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/pulls", testutils.MockResponse{
			StatusCode: http.StatusCreated,
			Body: map[string]interface{}{
				"number": 5, "title": "My PR", "body": "desc", "state": "open",
				"html_url": "https://github.com/testorg/test-repo/pull/5",
			},
		})

		resp, err := CreateNewPullRequest(ctx, testOrgName, testRepoName, "My PR", "desc", "main", "feature")

		require.NoError(t, err)
		assert.Equal(t, 5, resp.PRNumber)
		assert.Equal(t, "My PR", resp.TitleName)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/pulls", testutils.MockResponse{
			StatusCode: http.StatusUnprocessableEntity,
			Body:       map[string]interface{}{"message": "Validation Failed"},
		})

		_, err := CreateNewPullRequest(ctx, testOrgName, testRepoName, "My PR", "desc", "main", "feature")

		require.Error(t, err)
	})
}

func TestGetPullRequestInfo(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/pulls/5", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       map[string]interface{}{"number": 5, "title": "My PR", "state": "open"},
		})

		resp, err := GetPullRequestInfo(ctx, testOrgName, testRepoName, 5)

		require.NoError(t, err)
		assert.Equal(t, 5, resp.PRNumber)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/pulls/999", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		_, err := GetPullRequestInfo(ctx, testOrgName, testRepoName, 999)

		require.Error(t, err)
	})
}

func TestAddIssueAssignees(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/issues/5/assignees", testutils.MockResponse{
			StatusCode: http.StatusCreated,
			Body:       map[string]interface{}{"number": 5},
		})

		msg, err := AddIssueAssignees(ctx, testOrgName, testRepoName, 5, []string{"alice", "bob"})

		require.NoError(t, err)
		assert.Contains(t, fmt.Sprint(msg), "alice")
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/issues/5/assignees", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		_, err := AddIssueAssignees(ctx, testOrgName, testRepoName, 5, []string{"alice"})

		require.Error(t, err)
	})
}

func TestAddReviewers(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/pulls/5/requested_reviewers", testutils.MockResponse{
			StatusCode: http.StatusCreated,
			Body:       map[string]interface{}{"number": 5},
		})

		msg, err := AddReviewers(ctx, testOrgName, testRepoName, 5, []string{"carol"})

		require.NoError(t, err)
		assert.Contains(t, fmt.Sprint(msg), "carol")
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/pulls/5/requested_reviewers", testutils.MockResponse{
			StatusCode: http.StatusUnprocessableEntity,
			Body:       map[string]interface{}{"message": "Validation Failed"},
		})

		_, err := AddReviewers(ctx, testOrgName, testRepoName, 5, []string{"carol"})

		require.Error(t, err)
	})
}

func TestListPullRequests(t *testing.T) {
	t.Run("success with filters", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/pulls", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body: []map[string]interface{}{
				{"number": 1, "title": "PR one", "state": "open"},
			},
		})

		prs, err := ListPullRequests(ctx, testOrgName, testRepoName, "main", "feature", true)

		require.NoError(t, err)
		require.Len(t, prs, 1)
		assert.Equal(t, 1, prs[0].PRNumber)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/pulls", testutils.MockResponse{
			StatusCode: http.StatusForbidden,
			Body:       map[string]interface{}{"message": "Forbidden"},
		})

		_, err := ListPullRequests(ctx, testOrgName, testRepoName, "", "", false)

		require.Error(t, err)
	})
}

func TestUpdatePullRequest(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/pulls/5", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       map[string]interface{}{"number": 5, "state": "closed"},
		})

		resp, err := UpdatePullRequest(ctx, testOrgName, testRepoName, 5, "closed")

		require.NoError(t, err)
		assert.Equal(t, "closed", resp.State)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/pulls/999", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		_, err := UpdatePullRequest(ctx, testOrgName, testRepoName, 999, "closed")

		require.Error(t, err)
	})
}

func TestListPullRequestReviews(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/pulls/5/reviews", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body: []map[string]interface{}{
				{"id": 1, "state": "APPROVED", "body": "lgtm"},
			},
		})

		reviews, err := ListPullRequestReviews(ctx, testOrgName, testRepoName, 5)

		require.NoError(t, err)
		require.Len(t, reviews, 1)
		assert.Equal(t, "APPROVED", reviews[0].State)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/pulls/999/reviews", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		_, err := ListPullRequestReviews(ctx, testOrgName, testRepoName, 999)

		require.Error(t, err)
	})
}

func TestGetPullRequestFiles(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/pulls/5/files", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body: []map[string]interface{}{
				{"filename": "main.go", "additions": 10, "deletions": 2, "status": "modified"},
			},
		})

		files, err := GetPullRequestFiles(ctx, testOrgName, testRepoName, 5)

		require.NoError(t, err)
		require.Len(t, files, 1)
		assert.Equal(t, "main.go", files[0].Filename)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/pulls/999/files", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		_, err := GetPullRequestFiles(ctx, testOrgName, testRepoName, 999)

		require.Error(t, err)
	})
}

func TestReviewPullRequest(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/pulls/5/reviews", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       map[string]interface{}{"id": 1, "state": "APPROVED", "body": "lgtm"},
		})

		resp, err := ReviewPullRequest(ctx, testOrgName, testRepoName, 5, "approve", "lgtm")

		require.NoError(t, err)
		assert.Equal(t, testRepoName, resp.RepositoryName)
		assert.Equal(t, 5, resp.PRNumber)

		requests := mockServer.GetRequests()
		require.Len(t, requests, 1)
		assert.Contains(t, requests[0].Body, `"event":"APPROVE"`)
	})

	t.Run("error still populates repo/pr metadata", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/pulls/5/reviews", testutils.MockResponse{
			StatusCode: http.StatusUnprocessableEntity,
			Body:       map[string]interface{}{"message": "Validation Failed"},
		})

		resp, err := ReviewPullRequest(ctx, testOrgName, testRepoName, 5, "reject", "no")

		require.Error(t, err)
		assert.Equal(t, testRepoName, resp.RepositoryName)
		assert.Equal(t, 5, resp.PRNumber)
	})
}

func TestMergePullRequest(t *testing.T) {
	t.Run("success with title and body", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/pulls/5/merge", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       map[string]interface{}{"merged": true, "message": "OK", "sha": "merge-sha"},
		})

		resp, err := MergePullRequest(ctx, testOrgName, testRepoName, 5, "Merge title", "Merge body")

		require.NoError(t, err)
		assert.True(t, resp.Merged)
		requests := mockServer.GetRequests()
		require.Len(t, requests, 1)
		assert.Contains(t, requests[0].Body, `"commit_title":"Merge title"`)
		assert.Contains(t, requests[0].Body, `"commit_message":"Merge body"`)
	})

	t.Run("success without title or body", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/pulls/6/merge", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       map[string]interface{}{"merged": true, "message": "OK", "sha": "merge-sha-2"},
		})

		resp, err := MergePullRequest(ctx, testOrgName, testRepoName, 6, "", "")

		require.NoError(t, err)
		assert.True(t, resp.Merged)
		requests := mockServer.GetRequests()
		require.Len(t, requests, 1)
		assert.Equal(t, "{}", requests[0].Body)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/pulls/7/merge", testutils.MockResponse{
			StatusCode: http.StatusMethodNotAllowed,
			Body:       map[string]interface{}{"message": "Not mergeable"},
		})

		_, err := MergePullRequest(ctx, testOrgName, testRepoName, 7, "", "")

		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------
// Commits / check runs
// ---------------------------------------------------------------------

func TestListCommits(t *testing.T) {
	t.Run("success with until", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/commits", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body: []map[string]interface{}{
				{"sha": "sha-1", "commit": map[string]interface{}{"message": "first commit"}},
			},
		})

		commits, err := ListCommits(ctx, testOrgName, testRepoName, mainBranchName, "2024-01-01T00:00:00Z", "2024-02-01T00:00:00Z")

		require.NoError(t, err)
		require.Len(t, commits, 1)
		assert.Equal(t, "sha-1", commits[0].Sha)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/commits", testutils.MockResponse{
			StatusCode: http.StatusForbidden,
			Body:       map[string]interface{}{"message": "Forbidden"},
		})

		_, err := ListCommits(ctx, testOrgName, testRepoName, mainBranchName, "2024-01-01T00:00:00Z", "")

		require.Error(t, err)
	})
}

func TestGetCommitInfo(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/commits/abc123", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       map[string]interface{}{"sha": "abc123", "commit": map[string]interface{}{"message": "a commit"}},
		})

		commit, err := GetCommitInfo(ctx, testOrgName, testRepoName, "abc123")

		require.NoError(t, err)
		assert.Equal(t, "abc123", commit.Sha)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/commits/ghost", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		_, err := GetCommitInfo(ctx, testOrgName, testRepoName, "ghost")

		require.Error(t, err)
	})
}

func TestGetCommitCheckRuns(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/commits/abc123/check-runs", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body: map[string]interface{}{
				"total": 1,
				"check_runs": []map[string]interface{}{
					{"name": "build", "status": "completed", "conclusion": "success"},
				},
			},
		})

		checkRuns, err := GetCommitCheckRuns(ctx, testOrgName, testRepoName, "abc123")

		require.NoError(t, err)
		require.Len(t, checkRuns.CheckRuns, 1)
		assert.Equal(t, "build", checkRuns.CheckRuns[0].Name)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/commits/ghost/check-runs", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		_, err := GetCommitCheckRuns(ctx, testOrgName, testRepoName, "ghost")

		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------
// Workflows
// ---------------------------------------------------------------------

func TestListWorkflowRuns(t *testing.T) {
	t.Run("success with branch and status", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/actions/runs", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body: map[string]interface{}{
				"total_count": 1,
				"workflow_runs": []map[string]interface{}{
					{"id": 100, "name": "CI", "status": "completed"},
				},
			},
		})

		runs, err := ListWorkflowRuns(ctx, testOrgName, testRepoName, mainBranchName, "completed", 30)

		require.NoError(t, err)
		require.Len(t, runs, 1)
		assert.Equal(t, 100, runs[0].ID)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/actions/runs", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		_, err := ListWorkflowRuns(ctx, testOrgName, testRepoName, "", "", 30)

		require.Error(t, err)
	})
}

func TestDispatchWorkflow(t *testing.T) {
	t.Run("success with inputs", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/actions/workflows/ci.yml/dispatches", testutils.MockResponse{
			StatusCode: http.StatusNoContent,
		})

		err := DispatchWorkflow(ctx, testOrgName, testRepoName, "ci.yml", mainBranchName, map[string]string{"env": "prod"})

		require.NoError(t, err)
		requests := mockServer.GetRequests()
		require.Len(t, requests, 1)
		assert.Contains(t, requests[0].Body, `"inputs"`)
	})

	t.Run("success without inputs", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/actions/workflows/ci.yml/dispatches", testutils.MockResponse{
			StatusCode: http.StatusNoContent,
		})

		err := DispatchWorkflow(ctx, testOrgName, testRepoName, "ci.yml", mainBranchName, nil)

		require.NoError(t, err)
		requests := mockServer.GetRequests()
		require.Len(t, requests, 1)
		assert.NotContains(t, requests[0].Body, `"inputs"`)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/actions/workflows/ghost.yml/dispatches", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		err := DispatchWorkflow(ctx, testOrgName, testRepoName, "ghost.yml", mainBranchName, nil)

		require.Error(t, err)
	})
}

func TestRerunWorkflowRun(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/actions/runs/100/rerun", testutils.MockResponse{
			StatusCode: http.StatusCreated,
		})

		ok, err := RerunWorkflowRun(ctx, testOrgName, testRepoName, 100)

		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/actions/runs/999/rerun", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		ok, err := RerunWorkflowRun(ctx, testOrgName, testRepoName, 999)

		require.Error(t, err)
		assert.False(t, ok)
	})
}

func TestCancelWorkflowRun(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/actions/runs/100/cancel", testutils.MockResponse{
			StatusCode: http.StatusAccepted,
		})

		ok, err := CancelWorkflowRun(ctx, testOrgName, testRepoName, 100)

		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/actions/runs/999/cancel", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		ok, err := CancelWorkflowRun(ctx, testOrgName, testRepoName, 999)

		require.Error(t, err)
		assert.False(t, ok)
	})
}

func TestGetWorkflowRun(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/actions/runs/100", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       map[string]interface{}{"id": 100, "name": "CI", "status": "completed"},
		})

		run, err := GetWorkflowRun(ctx, testOrgName, testRepoName, 100)

		require.NoError(t, err)
		assert.Equal(t, 100, run.ID)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/actions/runs/999", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		_, err := GetWorkflowRun(ctx, testOrgName, testRepoName, 999)

		require.Error(t, err)
	})
}

func TestGetWorkflowRunJobs(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/actions/runs/100/jobs", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body: map[string]interface{}{
				"total_count": 1,
				"jobs": []map[string]interface{}{
					{"id": 1, "run_id": 100, "name": "build", "status": "completed"},
				},
			},
		})

		jobs, err := GetWorkflowRunJobs(ctx, testOrgName, testRepoName, 100)

		require.NoError(t, err)
		require.Len(t, jobs, 1)
		assert.Equal(t, "build", jobs[0].Name)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/actions/runs/999/jobs", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		_, err := GetWorkflowRunJobs(ctx, testOrgName, testRepoName, 999)

		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------
// Secret scanning alerts
// ---------------------------------------------------------------------

func TestListSecretScanningAlerts(t *testing.T) {
	t.Run("success injects repository name", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/secret-scanning/alerts", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body: []map[string]interface{}{
				{"number": 1, "secret_type": "generic", "state": "open"},
			},
		})

		alerts, err := ListSecretScanningAlerts(ctx, testOrgName, testRepoName, "open")

		require.NoError(t, err)
		require.Len(t, alerts, 1)
		assert.Equal(t, testRepoName, alerts[0].RepositoryName)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/secret-scanning/alerts", testutils.MockResponse{
			StatusCode: http.StatusForbidden,
			Body:       map[string]interface{}{"message": "Forbidden"},
		})

		_, err := ListSecretScanningAlerts(ctx, testOrgName, testRepoName, "")

		require.Error(t, err)
	})
}

func TestGetSecretScanningAlert(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/secret-scanning/alerts/3", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       map[string]interface{}{"number": 3, "secret_type": "generic", "state": "open"},
		})

		alert, err := GetSecretScanningAlert(ctx, testOrgName, testRepoName, 3)

		require.NoError(t, err)
		assert.Equal(t, testRepoName, alert.RepositoryName)
		assert.Equal(t, 3, alert.Number)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/secret-scanning/alerts/999", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		_, err := GetSecretScanningAlert(ctx, testOrgName, testRepoName, 999)

		require.Error(t, err)
	})
}

func TestUpdateSecretScanningAlert(t *testing.T) {
	t.Run("resolved with comment", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/secret-scanning/alerts/3", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       map[string]interface{}{"number": 3, "state": "resolved", "resolution": "false_positive"},
		})

		alert, err := UpdateSecretScanningAlert(ctx, testOrgName, testRepoName, 3, "resolved", "false_positive", "not a real secret")

		require.NoError(t, err)
		assert.Equal(t, "resolved", alert.State)
		requests := mockServer.GetRequests()
		require.Len(t, requests, 1)
		assert.Contains(t, requests[0].Body, `"resolution":"false_positive"`)
		assert.Contains(t, requests[0].Body, `"resolution_comment":"not a real secret"`)
	})

	t.Run("dismissed without resolution", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/secret-scanning/alerts/4", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       map[string]interface{}{"number": 4, "state": "open"},
		})

		_, err := UpdateSecretScanningAlert(ctx, testOrgName, testRepoName, 4, "open", "", "")

		require.NoError(t, err)
		requests := mockServer.GetRequests()
		require.Len(t, requests, 1)
		assert.NotContains(t, requests[0].Body, `"resolution"`)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/secret-scanning/alerts/999", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		_, err := UpdateSecretScanningAlert(ctx, testOrgName, testRepoName, 999, "resolved", "wont_fix", "")

		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------
// Issues
// ---------------------------------------------------------------------

func TestListIssues(t *testing.T) {
	t.Run("success with filters and repo name injection", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/issues", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body: []map[string]interface{}{
				{"number": 1, "title": "Bug report", "state": "open"},
			},
		})

		issues, err := ListIssues(ctx, testOrgName, testRepoName, "open", "bug", "alice", "bob", 0)

		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, testRepoName, issues[0].RepositoryName)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/issues", testutils.MockResponse{
			StatusCode: http.StatusForbidden,
			Body:       map[string]interface{}{"message": "Forbidden"},
		})

		_, err := ListIssues(ctx, testOrgName, testRepoName, "", "", "", "", 10)

		require.Error(t, err)
	})
}

func TestGetIssue(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/issues/9", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       map[string]interface{}{"number": 9, "title": "Bug", "state": "open"},
		})

		issue, err := GetIssue(ctx, testOrgName, testRepoName, 9)

		require.NoError(t, err)
		assert.Equal(t, testRepoName, issue.RepositoryName)
		assert.Equal(t, 9, issue.Number)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/issues/999", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		_, err := GetIssue(ctx, testOrgName, testRepoName, 999)

		require.Error(t, err)
	})
}

func TestCreateIssue(t *testing.T) {
	t.Run("success with body, assignee and labels", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/issues", testutils.MockResponse{
			StatusCode: http.StatusCreated,
			Body:       map[string]interface{}{"number": 10, "title": "New bug", "state": "open"},
		})

		issue, err := CreateIssue(ctx, testOrgName, testRepoName, "New bug", "details", "alice", []string{"bug", "urgent"})

		require.NoError(t, err)
		assert.Equal(t, testRepoName, issue.RepositoryName)
		requests := mockServer.GetRequests()
		require.Len(t, requests, 1)
		assert.Contains(t, requests[0].Body, `"assignees":["alice"]`)
		assert.Contains(t, requests[0].Body, `"labels":["bug","urgent"]`)
	})

	t.Run("success minimal (title only)", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/issues", testutils.MockResponse{
			StatusCode: http.StatusCreated,
			Body:       map[string]interface{}{"number": 11, "title": "Minimal", "state": "open"},
		})

		_, err := CreateIssue(ctx, testOrgName, testRepoName, "Minimal", "", "", nil)

		require.NoError(t, err)
		requests := mockServer.GetRequests()
		require.Len(t, requests, 1)
		assert.NotContains(t, requests[0].Body, `"assignees"`)
		assert.NotContains(t, requests[0].Body, `"labels"`)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/issues", testutils.MockResponse{
			StatusCode: http.StatusUnprocessableEntity,
			Body:       map[string]interface{}{"message": "Validation Failed"},
		})

		_, err := CreateIssue(ctx, testOrgName, testRepoName, "New bug", "", "", nil)

		require.Error(t, err)
	})
}

func TestUpdateIssue(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/issues/9", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       map[string]interface{}{"number": 9, "state": "closed"},
		})

		err := UpdateIssue(ctx, testOrgName, testRepoName, 9, "closed")

		require.NoError(t, err)
		requests := mockServer.GetRequests()
		require.Len(t, requests, 1)
		assert.Equal(t, `{"state":"closed"}`, requests[0].Body)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/issues/999", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		err := UpdateIssue(ctx, testOrgName, testRepoName, 999, "closed")

		require.Error(t, err)
	})
}

func TestListIssueComments(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/issues/9/comments", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body: []map[string]interface{}{
				{"id": 1, "body": "a comment"},
			},
		})

		comments, err := ListIssueComments(ctx, testOrgName, testRepoName, 9)

		require.NoError(t, err)
		require.Len(t, comments, 1)
		assert.Equal(t, "a comment", comments[0].Body)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/repos/testorg/test-repo/issues/999/comments", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]interface{}{"message": "Not Found"},
		})

		_, err := ListIssueComments(ctx, testOrgName, testRepoName, 999)

		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------
// Audit log
// ---------------------------------------------------------------------

func TestGetAuditLog(t *testing.T) {
	t.Run("success with phrase and include", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/orgs/testorg/audit-log", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body: []map[string]interface{}{
				{"action": "repo.create", "actor": "alice", "created_at": 1700000000000},
			},
		})

		entries, err := GetAuditLog(ctx, testOrgName, "action:repo.create", "web", 0)

		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, "repo.create", entries[0].Action)
	})

	t.Run("error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/orgs/testorg/audit-log", testutils.MockResponse{
			StatusCode: http.StatusForbidden,
			Body:       map[string]interface{}{"message": "Forbidden"},
		})

		_, err := GetAuditLog(ctx, testOrgName, "", "", 25)

		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------
// GetCurrentUser
// ---------------------------------------------------------------------

func TestGetCurrentUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/user", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       map[string]interface{}{"login": "octocat", "name": "The Octocat", "public_repos": 8},
		})

		user, err := GetCurrentUser(ctx)

		require.NoError(t, err)
		require.NotNil(t, user)
		assert.Equal(t, "octocat", user.Login)
		assert.Equal(t, 8, user.PublicRepos)
	})

	t.Run("malformed response body triggers unmarshal error", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/user", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       []string{"not", "a", "user", "object"},
		})

		user, err := GetCurrentUser(ctx)

		require.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "failed to parse user info")
	})

	// A single server-error case: also exercises the client's retry path
	// (3 attempts with backoff) before the error is finally surfaced.
	t.Run("server error after retries", func(t *testing.T) {
		mockServer, ctx := newTestCtx(t)
		mockServer.SetResponse("/user", testutils.MockResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       map[string]interface{}{"message": "Internal Server Error"},
		})

		user, err := GetCurrentUser(ctx)

		require.Error(t, err)
		assert.Nil(t, user)
	})
}

// ---------------------------------------------------------------------
// ListOrganizations (GraphQL, including cursor pagination)
// ---------------------------------------------------------------------

func orgGraphQLNode(login string, hasNextPage bool, endCursor string) map[string]interface{} {
	return map[string]interface{}{
		"login":                           login,
		"name":                            "Org " + login,
		"description":                     "An org",
		"email":                           login + "@example.test",
		"websiteUrl":                      "https://" + login + ".example.test",
		"location":                        "Earth",
		"twitterUsername":                 login,
		"createdAt":                       "2020-01-02T00:00:00Z",
		"updatedAt":                       "2023-05-06T00:00:00Z",
		"url":                             "https://github.com/" + login,
		"avatarUrl":                       "https://avatars.example/" + login + ".png",
		"isVerified":                      true,
		"requiresTwoFactorAuthentication": true,
		"membersWithRole":                 map[string]interface{}{"totalCount": 5},
		"teams":                           map[string]interface{}{"totalCount": 2},
		"repositories":                    map[string]interface{}{"totalCount": 3, "totalDiskUsage": 2048},
		"publicRepositories":              map[string]interface{}{"totalCount": 7},
	}
}

func TestListOrganizations_SinglePage(t *testing.T) {
	mockServer, ctx := newTestCtx(t)
	mockServer.SetResponse(graphqlPath, testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"data": map[string]interface{}{
				"viewer": map[string]interface{}{
					"organizations": map[string]interface{}{
						"pageInfo": map[string]interface{}{"startCursor": "", "hasPreviousPage": false, "endCursor": "", "hasNextPage": false},
						"nodes":    []map[string]interface{}{orgGraphQLNode("acme-org", false, "")},
					},
				},
			},
		},
	})

	orgs, err := ListOrganizations(ctx)

	require.NoError(t, err)
	require.Len(t, orgs, 1)
	got := orgs[0]
	assert.Equal(t, "acme-org", got.Login)
	assert.Equal(t, "Org acme-org", got.Name)
	assert.Equal(t, 5, got.MembersCount)
	assert.Equal(t, 2, got.TeamsCount)
	assert.Equal(t, 10, got.ReposCount) // 3 private + 7 public
	assert.Equal(t, 3, got.PrivateReposCount)
	assert.Equal(t, 7, got.PublicReposCount)
	assert.InDelta(t, 2.0, got.DiskUsageMB, 0.001) // 2048 KB / 1024
	assert.Equal(t, "2020-01-02", got.CreatedAt)
	assert.Equal(t, "2023-05-06", got.UpdatedAt)
	assert.True(t, got.IsVerified)
	assert.True(t, got.RequiresTwoFA)
}

// decodeGraphQLVariables extracts the "variables" object from a GraphQL POST body.
func decodeGraphQLVariables(r *http.Request) map[string]interface{} {
	var req struct {
		Variables map[string]interface{} `json:"variables"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	return req.Variables
}

func TestListOrganizations_Pagination(t *testing.T) {
	_, ctx := newTestCtx(t)

	graphqlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := decodeGraphQLVariables(r)
		cursor, _ := vars["cursor"].(string)

		w.Header().Set("Content-Type", "application/json")
		var orgsPage map[string]interface{}
		if cursor == "" {
			orgsPage = map[string]interface{}{
				"pageInfo": map[string]interface{}{"startCursor": "", "hasPreviousPage": false, "endCursor": "org-cursor-1", "hasNextPage": true},
				"nodes":    []map[string]interface{}{orgGraphQLNode("first-org", true, "org-cursor-1")},
			}
		} else {
			orgsPage = map[string]interface{}{
				"pageInfo": map[string]interface{}{"startCursor": "", "hasPreviousPage": false, "endCursor": "", "hasNextPage": false},
				"nodes":    []map[string]interface{}{orgGraphQLNode("second-org", false, "")},
			}
		}
		body := map[string]interface{}{
			"data": map[string]interface{}{
				"viewer": map[string]interface{}{
					"organizations": orgsPage,
				},
			},
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer graphqlServer.Close()

	ctx.GraphqlClient.Client = githubv4.NewEnterpriseClient(graphqlServer.URL+"/graphql", &http.Client{Transport: ctx.HttpClient.Client.Transport})

	orgs, err := ListOrganizations(ctx)

	require.NoError(t, err)
	require.Len(t, orgs, 2)
	assert.Equal(t, "first-org", orgs[0].Login)
	assert.Equal(t, "second-org", orgs[1].Login)
}

func TestListOrganizations_Error(t *testing.T) {
	mockServer, ctx := newTestCtx(t)
	mockServer.SetResponse(graphqlPath, testutils.MockResponse{
		StatusCode: http.StatusBadRequest,
		Body: map[string]interface{}{
			"errors": []map[string]interface{}{
				{"message": "Invalid query"},
			},
		},
	})

	orgs, err := ListOrganizations(ctx)

	require.Error(t, err)
	assert.Nil(t, orgs)
	assert.Contains(t, err.Error(), "graphql query failed for viewer organizations")
}

// ---------------------------------------------------------------------
// Query (thin wrapper around QueryWithContext)
// ---------------------------------------------------------------------

func TestQuery(t *testing.T) {
	mockServer, ctx := newTestCtx(t)
	mockServer.SetResponse(graphqlPath, testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"data": map[string]interface{}{
				"viewer": map[string]interface{}{
					"login": "octocat",
				},
			},
		},
	})

	var q struct {
		Viewer struct {
			Login githubv4.String
		}
	}
	err := Query(ctx, &q, nil)

	require.NoError(t, err)
	assert.Equal(t, githubv4.String("octocat"), q.Viewer.Login)
}

// ---------------------------------------------------------------------
// UpdateProtectedBranch / DeleteProtectedBranch error paths
// (success paths for both are already covered by TestGitHubAPIIntegration
// in integration_test.go; here we round out the error branches.)
// ---------------------------------------------------------------------

func TestUpdateProtectedBranch_Error(t *testing.T) {
	mockServer, ctx := newTestCtx(t)
	mockServer.SetResponse("/repos/testorg/test-repo/branches/main/protection", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "Forbidden"},
	})

	payload := []byte(`{"required_pull_request_reviews":{"required_approving_review_count":1}}`)
	resp, err := UpdateProtectedBranch(ctx, testOrgName, testRepoName, mainBranchName, payload)

	require.Error(t, err)
	assert.Equal(t, testRepoName, resp.RepositoryName)
}

func TestDeleteProtectedBranch_Error(t *testing.T) {
	mockServer, ctx := newTestCtx(t)
	mockServer.SetResponse("/repos/testorg/test-repo/branches/main/protection", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	ok, err := DeleteProtectedBranch(ctx, testOrgName, testRepoName, mainBranchName)

	require.Error(t, err)
	assert.False(t, ok)
}
