// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package issue

import (
	"net/http"
	"testing"

	"github.com/pradyb/sgh-cli/internal/model"
	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

func TestCreateIssue_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/issues", testutils.MockResponse{
		StatusCode: http.StatusCreated,
		Body: map[string]interface{}{
			"number":   1,
			"title":    "Bug report",
			"body":     "steps to reproduce",
			"state":    "open",
			"html_url": "https://github.com/testorg/repo1/issues/1",
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	issue := CreateIssue(ctx, IssueCreateRequest{
		OrgName: "testorg", RepoName: "repo1", Title: "Bug report", Body: "steps to reproduce",
		Assignee: "jane-doe", Labels: []string{"bug"},
	})

	if issue.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", issue.ErrorMessage)
	}
	if issue.Number != 1 || issue.Title != "Bug report" {
		t.Errorf("unexpected issue: %+v", issue)
	}
	if issue.RepositoryName != "repo1" {
		t.Errorf("RepositoryName = %q, want %q", issue.RepositoryName, "repo1")
	}
}

func TestCreateIssue_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/issues", testutils.MockResponse{
		StatusCode: http.StatusUnprocessableEntity,
		Body:       map[string]interface{}{"message": "validation failed"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	issue := CreateIssue(ctx, IssueCreateRequest{OrgName: "testorg", RepoName: "repo1", Title: "Bug report"})

	if issue.ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
	if issue.RepositoryName != "repo1" {
		t.Errorf("RepositoryName = %q, want %q", issue.RepositoryName, "repo1")
	}
}

func TestUpdateIssue_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/issues/5", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"number": 5, "state": "closed"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	resp := UpdateIssue(ctx, IssueUpdateRequest{OrgName: "testorg", RepoName: "repo1", IssueNumber: 5, State: "closed"})

	if resp.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", resp.ErrorMessage)
	}
	if resp.RepositoryName != "repo1" || resp.IssueNumber != 5 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestUpdateIssue_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/issues/5", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	resp := UpdateIssue(ctx, IssueUpdateRequest{OrgName: "testorg", RepoName: "repo1", IssueNumber: 5, State: "closed"})

	if resp.ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
}

func TestListIssues_GraphQL_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"data": map[string]interface{}{
				"search": map[string]interface{}{
					"issueCount": 1,
					"pageInfo":   map[string]interface{}{"endCursor": "", "hasNextPage": false},
					"edges": []map[string]interface{}{
						{
							"node": map[string]interface{}{
								"number":    42,
								"title":     "Bug",
								"url":       "https://github.com/testorg/repo1/issues/42",
								"body":      "desc",
								"state":     "OPEN",
								"createdAt": "2024-01-01T00:00:00Z",
								"updatedAt": "2024-01-02T00:00:00Z",
								"author":    map[string]interface{}{"login": "jane", "name": "Jane Doe"},
								"repository": map[string]interface{}{
									"name": "repo1", "nameWithOwner": "testorg/repo1",
									"url": "https://github.com/testorg/repo1", "sshUrl": "git@github.com:testorg/repo1.git",
								},
								"assignees": map[string]interface{}{
									"totalCount": 1,
									"edges":      []map[string]interface{}{{"node": map[string]interface{}{"login": "bob", "name": "Bob"}}},
								},
								"labels": map[string]interface{}{
									"totalCount": 1,
									"edges":      []map[string]interface{}{{"node": map[string]interface{}{"name": "bug"}}},
								},
								"comments": map[string]interface{}{"totalCount": 3},
							},
						},
					},
				},
			},
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	responses := ListIssues(ctx, IssueListRequest{OrgName: "testorg", State: "open"})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	got := responses[0]
	if got.Number != 42 || got.Title != "Bug" {
		t.Errorf("unexpected issue: %+v", got)
	}
	if got.RepositoryName != "repo1" {
		t.Errorf("RepositoryName = %q, want %q", got.RepositoryName, "repo1")
	}
	if got.Author.Login != "jane" || got.Author.Name != "Jane Doe" {
		t.Errorf("unexpected author: %+v", got.Author)
	}
	if len(got.Assignees) != 1 || got.Assignees[0].Login != "bob" {
		t.Errorf("unexpected assignees: %+v", got.Assignees)
	}
	if len(got.Labels) != 1 || got.Labels[0].Name != "bug" {
		t.Errorf("unexpected labels: %+v", got.Labels)
	}
	if got.Comments != 3 {
		t.Errorf("Comments = %d, want 3", got.Comments)
	}
}

func TestListIssues_GraphQL_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"errors": []map[string]interface{}{{"message": "boom"}}},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	responses := ListIssues(ctx, IssueListRequest{OrgName: "testorg", State: "open"})

	if len(responses) != 0 {
		t.Fatalf("len(responses) = %d, want 0", len(responses))
	}
}

func TestListIssues_REST_MultiRepo_FiltersPullRequests(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/issues", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: []map[string]interface{}{
			{"number": 1, "title": "A real issue", "state": "open"},
			{"number": 2, "title": "Actually a PR", "state": "open", "pull_request": map[string]interface{}{"url": "https://api.github.com/repos/testorg/repo1/pulls/2"}},
		},
	})
	mockServer.SetResponse("/repos/testorg/repo2/issues", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: []map[string]interface{}{
			{"number": 3, "title": "Another issue", "state": "closed"},
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := ListIssues(ctx, IssueListRequest{OrgName: "testorg", RepoNames: []string{"repo1", "repo2"}})

	if len(responses) != 2 {
		t.Fatalf("len(responses) = %d, want 2 (PR should be filtered out): %+v", len(responses), responses)
	}
	for _, r := range responses {
		if r.Number == 2 {
			t.Errorf("expected pull request #2 to be filtered out of issue results")
		}
	}
}

func TestListIssues_REST_MultiRepo_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/issues", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       []map[string]interface{}{{"number": 1, "title": "ok"}},
	})
	mockServer.SetResponse("/repos/testorg/repo2/issues", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := ListIssues(ctx, IssueListRequest{OrgName: "testorg", RepoNames: []string{"repo1", "repo2"}})

	if len(responses) != 2 {
		t.Fatalf("len(responses) = %d, want 2", len(responses))
	}
	var sawError bool
	for _, r := range responses {
		if r.RepositoryName == "repo2" && r.ErrorMessage != "" {
			sawError = true
		}
	}
	if !sawError {
		t.Error("expected repo2 to report an error")
	}
}

func TestGetIssue_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/issues/7", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"number": 7, "title": "Investigate flaky test", "state": "open"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	issue := GetIssue(ctx, IssueViewRequest{OrgName: "testorg", RepoName: "repo1", IssueNumber: 7})

	if issue.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", issue.ErrorMessage)
	}
	if issue.Number != 7 {
		t.Errorf("Number = %d, want 7", issue.Number)
	}
	if issue.RepositoryName != "repo1" {
		t.Errorf("RepositoryName = %q, want %q", issue.RepositoryName, "repo1")
	}
}

func TestGetIssue_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/issues/99", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	issue := GetIssue(ctx, IssueViewRequest{OrgName: "testorg", RepoName: "repo1", IssueNumber: 99})

	if issue.ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
	if issue.RepositoryName != "repo1" {
		t.Errorf("RepositoryName = %q, want %q", issue.RepositoryName, "repo1")
	}
}

func TestGetIssueComments_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/issues/7/comments", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: []map[string]interface{}{
			{"id": 1, "body": "first comment", "user": map[string]interface{}{"login": "jane"}},
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	comments := GetIssueComments(ctx, "testorg", "repo1", 7)

	if len(comments) != 1 {
		t.Fatalf("len(comments) = %d, want 1", len(comments))
	}
	if comments[0].Body != "first comment" {
		t.Errorf("Body = %q, want %q", comments[0].Body, "first comment")
	}
}

func TestGetIssueComments_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/issues/7/comments", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	comments := GetIssueComments(ctx, "testorg", "repo1", 7)

	if comments != nil {
		t.Errorf("expected nil comments on error, got %v", comments)
	}
}

func TestPopulateAssignees(t *testing.T) {
	tests := []struct {
		name      string
		assignees model.AssigneesFragment
		want      []model.User
	}{
		{
			name:      "no assignees returns empty slice",
			assignees: model.AssigneesFragment{},
			want:      []model.User{},
		},
		{
			name: "single assignee",
			assignees: model.AssigneesFragment{
				TotalCount: 1,
				Edges: []struct {
					Node model.UserFragment
				}{
					{Node: model.UserFragment{Login: "jane", Name: "Jane Doe"}},
				},
			},
			want: []model.User{{Login: "jane", Name: "Jane Doe"}},
		},
		{
			name: "multiple assignees preserve order",
			assignees: model.AssigneesFragment{
				TotalCount: 2,
				Edges: []struct {
					Node model.UserFragment
				}{
					{Node: model.UserFragment{Login: "jane", Name: "Jane Doe"}},
					{Node: model.UserFragment{Login: "bob", Name: ""}},
				},
			},
			want: []model.User{{Login: "jane", Name: "Jane Doe"}, {Login: "bob", Name: ""}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := populateAssignees(tt.assignees)
			if len(got) != len(tt.want) {
				t.Fatalf("len(got) = %d, want %d (%+v)", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i].Login != tt.want[i].Login || got[i].Name != tt.want[i].Name {
					t.Errorf("got[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
