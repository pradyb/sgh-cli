// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package branch

import (
	"net/http"
	"testing"

	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/internal/testutils"
	"github.com/pradyb/sgh-cli/pkg/context"
)

func TestBuildBranchSearchQuery(t *testing.T) {
	tests := []struct {
		name string
		req  BranchListRequest
		want string
	}{
		{
			name: "org scope with no repos",
			req:  BranchListRequest{OrgName: "sample-org"},
			want: "org:sample-org",
		},
		{
			name: "single repo narrows to repo scope",
			req:  BranchListRequest{OrgName: "sample-org", RepoNames: []string{"sample-repo"}},
			want: "repo:sample-org/sample-repo",
		},
		{
			name: "multiple repos keeps org scope",
			req:  BranchListRequest{OrgName: "sample-org", RepoNames: []string{"repo-a", "repo-b"}},
			want: "org:sample-org",
		},
	}

	ctx := &context.Context{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildBranchSearchQuery(ctx, tt.req); got != tt.want {
				t.Errorf("buildBranchSearchQuery() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCreateNewBranchFromCommit_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()

	ctx := service.NewMockContext(t, mockServer)

	responses := CreateNewBranchFromCommit(ctx, BranchCreateFromCommitRequest{
		OrgName:       "testorg",
		RepoName:      "repo1",
		NewBranchName: "feature-x",
		CommitSHA:     "abc123",
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	got := responses[0]
	if got.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", got.ErrorMessage)
	}
	if got.RepositoryName != "repo1" || got.Ref != "feature-x" {
		t.Errorf("response = %+v, want repo1/feature-x", got)
	}
	if got.SuccessMessage != "1234567890abcdef1234567890abcdef12345678" {
		t.Errorf("SuccessMessage = %q, want the mocked commit SHA", got.SuccessMessage)
	}
	if !got.IsSuccess() {
		t.Error("expected IsSuccess() to be true")
	}
}

func TestCreateNewBranchFromCommit_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/git/refs", testutils.MockResponse{
		StatusCode: http.StatusUnprocessableEntity,
		Body:       map[string]interface{}{"message": "Reference already exists"},
	})

	ctx := service.NewMockContext(t, mockServer)

	responses := CreateNewBranchFromCommit(ctx, BranchCreateFromCommitRequest{
		OrgName:       "testorg",
		RepoName:      "repo1",
		NewBranchName: "feature-x",
		CommitSHA:     "abc123",
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
}

func TestCreateNewBranches_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/git/ref/heads/main", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"object": map[string]interface{}{"sha": "base-sha-1"}},
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := CreateNewBranches(ctx, BranchCreateRequest{
		OrgName:       "testorg",
		RepoNames:     []string{"repo1"},
		NewBranchName: "feature-x",
		RefBranchName: "main",
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	got := responses[0]
	if got.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", got.ErrorMessage)
	}
	if got.RepositoryName != "repo1" || got.Ref != "feature-x" {
		t.Errorf("response = %+v", got)
	}
	if got.SuccessMessage != "1234567890abcdef1234567890abcdef12345678" {
		t.Errorf("SuccessMessage = %q, want the mocked new-branch SHA", got.SuccessMessage)
	}
}

func TestCreateNewBranches_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/git/ref/heads/main", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "not found"},
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := CreateNewBranches(ctx, BranchCreateRequest{
		OrgName:       "testorg",
		RepoNames:     []string{"repo1"},
		NewBranchName: "feature-x",
		RefBranchName: "main",
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
}

func TestDeleteBranches_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := DeleteBranches(ctx, BranchDeleteRequest{
		OrgName:    "testorg",
		RepoNames:  []string{"repo1"},
		BranchName: "old-feature",
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", responses[0].ErrorMessage)
	}
	if responses[0].RepositoryName != "repo1" || responses[0].Ref != "old-feature" {
		t.Errorf("response = %+v", responses[0])
	}
}

func TestDeleteBranches_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/git/refs/heads/old-feature", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := DeleteBranches(ctx, BranchDeleteRequest{
		OrgName:    "testorg",
		RepoNames:  []string{"repo1"},
		BranchName: "old-feature",
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
}

func TestRenameBranches_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := RenameBranches(ctx, BranchRenameRequest{
		OrgName:   "testorg",
		RepoNames: []string{"repo1"},
		OldName:   "old-name",
		NewName:   "new-name",
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", responses[0].ErrorMessage)
	}
	if responses[0].Ref != "new-name" {
		t.Errorf("Ref = %q, want new-name", responses[0].Ref)
	}
}

func TestRenameBranches_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/branches/old-name/rename", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "branch not found"},
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := RenameBranches(ctx, BranchRenameRequest{
		OrgName:   "testorg",
		RepoNames: []string{"repo1"},
		OldName:   "old-name",
		NewName:   "new-name",
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
	if responses[0].Ref != "old-name" {
		t.Errorf("Ref = %q, want old-name on failure", responses[0].Ref)
	}
}

func TestListBranches_InvalidFilterRegex(t *testing.T) {
	ctx := &context.Context{}

	responses := ListBranches(ctx, BranchListRequest{OrgName: "testorg", Filter: "("})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].RepositoryName != "(filter)" {
		t.Errorf("RepositoryName = %q, want (filter)", responses[0].RepositoryName)
	}
}

func graphqlBranchSearchBody() map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"search": map[string]interface{}{
				"repositoryCount": 1,
				"pageInfo":        map[string]interface{}{"endCursor": "", "hasNextPage": false},
				"edges": []map[string]interface{}{
					{
						"node": map[string]interface{}{
							"name": "repo1",
							"refs": map[string]interface{}{
								"totalCount": 2,
								"edges": []map[string]interface{}{
									{"node": map[string]interface{}{
										"name":                 "main",
										"target":               map[string]interface{}{"oid": "sha-main"},
										"branchProtectionRule": map[string]interface{}{"isAdminEnforced": true, "pattern": ""},
									}},
									{"node": map[string]interface{}{
										"name":                 "develop",
										"target":               map[string]interface{}{"oid": "sha-dev"},
										"branchProtectionRule": map[string]interface{}{"isAdminEnforced": false, "pattern": ""},
									}},
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestListBranches_GraphQL_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       graphqlBranchSearchBody(),
	})

	ctx := service.NewMockContext(t, mockServer)

	responses := ListBranches(ctx, BranchListRequest{OrgName: "testorg", RepoNames: []string{"repo1"}})

	if len(responses) != 2 {
		t.Fatalf("len(responses) = %d, want 2", len(responses))
	}
	byName := map[string]bool{}
	for _, r := range responses {
		byName[r.Name] = r.Protected
		if r.RepositoryName != "repo1" {
			t.Errorf("RepositoryName = %q, want repo1", r.RepositoryName)
		}
	}
	if protected, ok := byName["main"]; !ok || !protected {
		t.Errorf("expected main to be protected, got %+v", byName)
	}
	if protected, ok := byName["develop"]; !ok || protected {
		t.Errorf("expected develop to be unprotected, got %+v", byName)
	}
}

func TestListBranches_GraphQL_Filter(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       graphqlBranchSearchBody(),
	})

	ctx := service.NewMockContext(t, mockServer)

	responses := ListBranches(ctx, BranchListRequest{OrgName: "testorg", RepoNames: []string{"repo1"}, Filter: "^main$"})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].Name != "main" {
		t.Errorf("Name = %q, want main", responses[0].Name)
	}
}

func TestListBranches_GraphQL_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := service.NewMockContext(t, mockServer)

	responses := ListBranches(ctx, BranchListRequest{OrgName: "testorg"})

	if len(responses) != 0 {
		t.Fatalf("len(responses) = %d, want 0 on error", len(responses))
	}
}

func TestListBranches_REST_MultiRepo(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := ListBranches(ctx, BranchListRequest{OrgName: "testorg", RepoNames: []string{"repo1", "repo2"}, Filter: "dev"})

	if len(responses) != 2 {
		t.Fatalf("len(responses) = %d, want 2 (one 'develop' branch per repo)", len(responses))
	}
	for _, r := range responses {
		if r.Name != "develop" {
			t.Errorf("Name = %q, want develop", r.Name)
		}
	}
}

func TestListBranches_REST_MultiRepo_PartialError(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo2/branches", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "not found"},
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := ListBranches(ctx, BranchListRequest{OrgName: "testorg", RepoNames: []string{"repo1", "repo2"}})

	if len(responses) != 3 {
		t.Fatalf("len(responses) = %d, want 3 (2 branches for repo1 + 1 error entry for repo2)", len(responses))
	}
	var sawError bool
	for _, r := range responses {
		if r.RepositoryName == "repo2" {
			sawError = true
		}
	}
	if !sawError {
		t.Error("expected an error entry for repo2")
	}
}
