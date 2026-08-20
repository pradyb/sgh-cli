// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package pr

import (
	"net/http"
	"testing"

	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

func TestCreateNewPullRequestForRepo_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls", testutils.MockResponse{
		StatusCode: http.StatusCreated,
		Body: map[string]interface{}{
			"number":   42,
			"title":    "Add feature",
			"body":     "does a thing",
			"html_url": "https://github.com/testorg/test-repo/pull/42",
			"state":    "open",
			"head":     map[string]interface{}{"ref": "feature", "repo": map[string]interface{}{"name": "test-repo"}},
			"base":     map[string]interface{}{"ref": "main", "repo": map[string]interface{}{"name": "test-repo"}},
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	resp, err := CreateNewPullRequestForRepo(ctx, PullRequestRequest{
		OrgName:  "testorg",
		RepoName: "test-repo",
		BaseRef:  "main",
		HeadRef:  "feature",
		Title:    "Add feature",
		Body:     "does a thing",
	}, false)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.PRNumber != 42 {
		t.Errorf("PRNumber = %d, want 42", resp.PRNumber)
	}
	if resp.TitleName != "Add feature" {
		t.Errorf("TitleName = %q, want %q", resp.TitleName, "Add feature")
	}
	if resp.HTMLUrl != "https://github.com/testorg/test-repo/pull/42" {
		t.Errorf("HTMLUrl = %q, want github URL", resp.HTMLUrl)
	}
}

func TestCreateNewPullRequestForRepo_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls", testutils.MockResponse{
		StatusCode: http.StatusUnprocessableEntity,
		Body:       map[string]interface{}{"message": "Validation Failed"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	resp, err := CreateNewPullRequestForRepo(ctx, PullRequestRequest{
		OrgName:  "testorg",
		RepoName: "test-repo",
		BaseRef:  "main",
		HeadRef:  "feature",
	}, false)

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resp.PRNumber != 0 {
		t.Errorf("expected zero-value response, got %+v", resp)
	}
}

func TestCreateNewPullRequest_MultiRepo(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo-a/pulls", testutils.MockResponse{
		StatusCode: http.StatusCreated,
		Body: map[string]interface{}{
			"number":   1,
			"title":    "PR in repo-a",
			"html_url": "https://github.com/testorg/repo-a/pull/1",
		},
	})
	mockServer.SetResponse("/repos/testorg/repo-b/pulls", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := CreateNewPullRequest(ctx, PRRequest{
		OrgName:   "testorg",
		RepoNames: []string{"repo-a", "repo-b"},
		BaseRef:   "main",
		HeadRef:   "feature",
		Title:     "combined title",
	})

	if len(responses) != 2 {
		t.Fatalf("len(responses) = %d, want 2", len(responses))
	}

	var sawSuccess, sawError bool
	for _, r := range responses {
		if r.ErrorMessage != "" {
			sawError = true
			continue
		}
		if r.PRNumber == 1 && r.TitleName == "PR in repo-a" {
			sawSuccess = true
		}
	}
	if !sawSuccess {
		t.Errorf("expected a successful create response, got %+v", responses)
	}
	if !sawError {
		t.Errorf("expected a failed create response, got %+v", responses)
	}
}

func TestListPullRequests_RESTMultiRepo(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo-a/pulls", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: []map[string]interface{}{
			{"number": 1, "title": "Alice's PR", "user": map[string]interface{}{"login": "alice"}},
			{"number": 2, "title": "Bob's PR", "user": map[string]interface{}{"login": "bob"}},
		},
	})
	mockServer.SetResponse("/repos/testorg/repo-b/pulls", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := ListPullRequests(ctx, PRRequest{
		OrgName:   "testorg",
		RepoNames: []string{"repo-a", "repo-b"},
		Author:    "alice",
	})

	var sawAlice, sawBob, sawError bool
	for _, r := range responses {
		switch {
		case r.ErrorMessage != "":
			sawError = true
		case r.Author.Login == "alice":
			sawAlice = true
		case r.Author.Login == "bob":
			sawBob = true
		}
	}
	if !sawAlice {
		t.Errorf("expected alice's PR to survive the author filter, got %+v", responses)
	}
	if sawBob {
		t.Errorf("expected bob's PR to be filtered out, got %+v", responses)
	}
	if !sawError {
		t.Errorf("expected an error entry for repo-b, got %+v", responses)
	}
}

func TestReviewPullRequest_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7/reviews", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"id":    99,
			"state": "APPROVED",
			"body":  "looks good",
			"user":  map[string]interface{}{"login": "reviewer1"},
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	resp := ReviewPullRequest(ctx, PRReviewRequest{
		OrgName:  "testorg",
		RepoName: "test-repo",
		PRNumber: 7,
		Event:    "approve",
		Body:     "looks good",
	})

	if resp.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", resp.ErrorMessage)
	}
	if resp.State != "APPROVED" {
		t.Errorf("State = %q, want %q", resp.State, "APPROVED")
	}
	if resp.User.Login != "reviewer1" {
		t.Errorf("User.Login = %q, want %q", resp.User.Login, "reviewer1")
	}
}

func TestReviewPullRequest_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7/reviews", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	resp := ReviewPullRequest(ctx, PRReviewRequest{
		OrgName:  "testorg",
		RepoName: "test-repo",
		PRNumber: 7,
		Event:    "approve",
	})

	if resp.ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
}

func TestListPullRequestReviews_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7/reviews", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: []map[string]interface{}{
			{"id": 1, "state": "APPROVED", "user": map[string]interface{}{"login": "reviewer1"}},
			{"id": 2, "state": "COMMENTED", "user": map[string]interface{}{"login": "reviewer2"}},
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	reviews := ListPullRequestReviews(ctx, "testorg", "test-repo", 7)

	if len(reviews) != 2 {
		t.Fatalf("len(reviews) = %d, want 2", len(reviews))
	}
	if reviews[0].State != "APPROVED" || reviews[1].State != "COMMENTED" {
		t.Errorf("unexpected review states: %+v", reviews)
	}
}

func TestListPullRequestReviews_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7/reviews", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	reviews := ListPullRequestReviews(ctx, "testorg", "test-repo", 7)

	if len(reviews) != 1 || reviews[0].ErrorMessage == "" {
		t.Fatalf("expected a single error entry, got %+v", reviews)
	}
}

func TestGetPullRequestFiles_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7/files", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: []map[string]interface{}{
			{"filename": "main.go", "additions": 10, "deletions": 2, "status": "modified", "patch": "@@ -1,2 +1,3 @@\n+line"},
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	resp := GetPullRequestFiles(ctx, "testorg", "test-repo", 7)

	if resp.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", resp.ErrorMessage)
	}
	if resp.RepositoryName != "test-repo" || resp.PRNumber != 7 {
		t.Errorf("unexpected metadata: %+v", resp)
	}
	if len(resp.Files) != 1 || resp.Files[0].Filename != "main.go" {
		t.Errorf("unexpected files: %+v", resp.Files)
	}
}

func TestGetPullRequestFiles_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7/files", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	resp := GetPullRequestFiles(ctx, "testorg", "test-repo", 7)

	if resp.ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
	if resp.RepositoryName != "test-repo" || resp.PRNumber != 7 {
		t.Errorf("unexpected metadata on error: %+v", resp)
	}
}

func TestGetPullRequestInfo_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"number": 7,
			"title":  "Info PR",
			"state":  "open",
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	resp := GetPullRequestInfo(ctx, "testorg", "test-repo", 7)

	if resp.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", resp.ErrorMessage)
	}
	if resp.PRNumber != 7 || resp.TitleName != "Info PR" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestGetPullRequestInfo_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	resp := GetPullRequestInfo(ctx, "testorg", "test-repo", 7)

	if resp.ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
}

func TestUpdatePullRequest_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"number": 7,
			"title":  "Updated PR",
			"state":  "closed",
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	resp := UpdatePullRequest(ctx, PRUpdateRequest{
		OrgName:  "testorg",
		RepoName: "test-repo",
		PRNumber: 7,
		State:    "closed",
	})

	if resp.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", resp.ErrorMessage)
	}
	if resp.State != "closed" {
		t.Errorf("State = %q, want %q", resp.State, "closed")
	}
}

func TestUpdatePullRequest_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	resp := UpdatePullRequest(ctx, PRUpdateRequest{
		OrgName:  "testorg",
		RepoName: "test-repo",
		PRNumber: 7,
		State:    "closed",
	})

	if resp.ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
}

func TestMergePullRequest_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7/merge", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"merged":  true,
			"message": "Pull Request successfully merged",
			"sha":     "abc123",
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	resp := MergePullRequest(ctx, PRMergeRequest{
		OrgName:  "testorg",
		RepoName: "test-repo",
		PRNumber: 7,
		Title:    "merge it",
	})

	if resp.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", resp.ErrorMessage)
	}
	if !resp.Merged {
		t.Error("expected Merged to be true")
	}
	if resp.RepositoryName != "test-repo" {
		t.Errorf("RepositoryName = %q, want %q", resp.RepositoryName, "test-repo")
	}
	if resp.SHA != "abc123" {
		t.Errorf("SHA = %q, want %q", resp.SHA, "abc123")
	}
}

func TestMergePullRequest_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7/merge", testutils.MockResponse{
		StatusCode: http.StatusMethodNotAllowed,
		Body:       map[string]interface{}{"message": "Pull Request is not mergeable"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	resp := MergePullRequest(ctx, PRMergeRequest{
		OrgName:  "testorg",
		RepoName: "test-repo",
		PRNumber: 7,
	})

	if resp.ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
	if resp.RepositoryName != "test-repo" {
		t.Errorf("RepositoryName = %q, want %q even on error", resp.RepositoryName, "test-repo")
	}
}

func TestGetPRDetails_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"number": 7, "title": "Detail PR"},
	})
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7/files", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       []map[string]interface{}{{"filename": "main.go"}},
	})
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7/reviews", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       []map[string]interface{}{{"id": 1, "state": "APPROVED"}},
	})
	mockServer.SetResponse("/repos/testorg/test-repo/commits/deadbeef/check-runs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"total":      1,
			"check_runs": []map[string]interface{}{{"name": "build", "status": "completed", "conclusion": "success"}},
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	prResp, filesResp, checkResp, reviews := GetPRDetails(ctx, PRDetailsRequest{
		OrgName:  "testorg",
		RepoName: "test-repo",
		PRNumber: 7,
		LastSHA:  "deadbeef",
	})

	if prResp.ErrorMessage != "" || prResp.TitleName != "Detail PR" {
		t.Errorf("unexpected PR response: %+v", prResp)
	}
	if filesResp.ErrorMessage != "" || len(filesResp.Files) != 1 {
		t.Errorf("unexpected files response: %+v", filesResp)
	}
	if checkResp.ErrorMessage != "" || checkResp.Total != 1 || len(checkResp.CheckRuns) != 1 {
		t.Errorf("unexpected check run response: %+v", checkResp)
	}
	if len(reviews) != 1 || reviews[0].State != "APPROVED" {
		t.Errorf("unexpected reviews: %+v", reviews)
	}
}

func TestGetPRDetails_PartialError(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"number": 7, "title": "Detail PR"},
	})
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7/files", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7/reviews", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       []map[string]interface{}{},
	})
	mockServer.SetResponse("/repos/testorg/test-repo/commits/deadbeef/check-runs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"total": 0, "check_runs": []map[string]interface{}{}},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	prResp, filesResp, _, _ := GetPRDetails(ctx, PRDetailsRequest{
		OrgName:  "testorg",
		RepoName: "test-repo",
		PRNumber: 7,
		LastSHA:  "deadbeef",
	})

	if prResp.ErrorMessage != "" {
		t.Errorf("expected the PR info call to succeed, got %+v", prResp)
	}
	if filesResp.ErrorMessage == "" {
		t.Error("expected the files call to report an error")
	}
}
