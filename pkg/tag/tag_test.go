// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package tag

import (
	"net/http"
	"testing"

	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/internal/testutils"
	"github.com/pradyb/sgh-cli/pkg/context"
)

func TestBuildTagSearchQuery(t *testing.T) {
	tests := []struct {
		name string
		req  TagListRequest
		want string
	}{
		{
			name: "org scope with no repos",
			req:  TagListRequest{OrgName: "sample-org"},
			want: "org:sample-org",
		},
		{
			name: "single repo narrows to repo scope",
			req:  TagListRequest{OrgName: "sample-org", RepoNames: []string{"sample-repo"}},
			want: "repo:sample-org/sample-repo",
		},
		{
			name: "multiple repos keeps org scope",
			req:  TagListRequest{OrgName: "sample-org", RepoNames: []string{"repo-a", "repo-b"}},
			want: "org:sample-org",
		},
	}

	ctx := &context.Context{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildTagSearchQuery(ctx, tt.req); got != tt.want {
				t.Errorf("buildTagSearchQuery() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCreateNewTag_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/git/ref/heads/main", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"object": map[string]interface{}{"sha": "base-sha-1"}},
	})
	mockServer.SetResponse("/repos/testorg/repo1/git/tags", testutils.MockResponse{
		StatusCode: http.StatusCreated,
		Body:       map[string]interface{}{"sha": "tag-object-sha"},
	})

	ctx := service.NewMockContext(t, mockServer)

	response, err := CreateNewTag(ctx, TagCreateSingleRequest{
		OrgName:       "testorg",
		RepoName:      "repo1",
		TagName:       "v1.0.0",
		RefBranchName: "main",
		Message:       "release",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response.Object.SHA != "1234567890abcdef1234567890abcdef12345678" {
		t.Errorf("Object.SHA = %q, want the mocked ref SHA", response.Object.SHA)
	}
}

func TestCreateNewTag_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/git/ref/heads/main", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "branch not found"},
	})

	ctx := service.NewMockContext(t, mockServer)

	_, err := CreateNewTag(ctx, TagCreateSingleRequest{
		OrgName:       "testorg",
		RepoName:      "repo1",
		TagName:       "v1.0.0",
		RefBranchName: "main",
		Message:       "release",
	})

	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestCreateNewTags_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/git/ref/heads/main", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"object": map[string]interface{}{"sha": "base-sha-1"}},
	})
	mockServer.SetResponse("/repos/testorg/repo1/git/tags", testutils.MockResponse{
		StatusCode: http.StatusCreated,
		Body:       map[string]interface{}{"sha": "tag-object-sha"},
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := CreateNewTags(ctx, TagCreateRequest{
		OrgName:       "testorg",
		RepoNames:     []string{"repo1"},
		TagName:       "v1.0.0",
		RefBranchName: "main",
		Message:       "release",
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	got := responses[0]
	if got.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", got.ErrorMessage)
	}
	if got.RepositoryName != "repo1" || got.Ref != "v1.0.0" {
		t.Errorf("response = %+v", got)
	}
}

func TestCreateNewTags_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/git/ref/heads/main", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "branch not found"},
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := CreateNewTags(ctx, TagCreateRequest{
		OrgName:       "testorg",
		RepoNames:     []string{"repo1"},
		TagName:       "v1.0.0",
		RefBranchName: "main",
		Message:       "release",
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
}

func TestDeleteTags_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := DeleteTags(ctx, TagDeleteRequest{
		OrgName:   "testorg",
		RepoNames: []string{"repo1"},
		TagName:   "v0.9.0",
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", responses[0].ErrorMessage)
	}
	if responses[0].RepositoryName != "repo1" || responses[0].Ref != "v0.9.0" {
		t.Errorf("response = %+v", responses[0])
	}
}

func TestDeleteTags_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/git/refs/tags/v0.9.0", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := DeleteTags(ctx, TagDeleteRequest{
		OrgName:   "testorg",
		RepoNames: []string{"repo1"},
		TagName:   "v0.9.0",
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
}

func TestListTags_InvalidFilterRegex(t *testing.T) {
	ctx := &context.Context{}

	responses := ListTags(ctx, TagListRequest{OrgName: "testorg", Filter: "("})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].RepositoryName != "(filter)" {
		t.Errorf("RepositoryName = %q, want (filter)", responses[0].RepositoryName)
	}
}

func graphqlTagSearchBody() map[string]interface{} {
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
										"name":   "v1.0.0",
										"target": map[string]interface{}{"oid": "sha-v1"},
									}},
									{"node": map[string]interface{}{
										"name":   "v2.0.0",
										"target": map[string]interface{}{"oid": "sha-v2"},
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

func TestListTags_GraphQL_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       graphqlTagSearchBody(),
	})

	ctx := service.NewMockContext(t, mockServer)

	responses := ListTags(ctx, TagListRequest{OrgName: "testorg", RepoNames: []string{"repo1"}})

	if len(responses) != 2 {
		t.Fatalf("len(responses) = %d, want 2", len(responses))
	}
	shaByName := map[string]string{}
	for _, r := range responses {
		shaByName[r.Name] = r.Commit.SHA
		if r.RepositoryName != "repo1" {
			t.Errorf("RepositoryName = %q, want repo1", r.RepositoryName)
		}
	}
	if shaByName["v1.0.0"] != "sha-v1" || shaByName["v2.0.0"] != "sha-v2" {
		t.Errorf("shaByName = %+v", shaByName)
	}
}

func TestListTags_GraphQL_Filter(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       graphqlTagSearchBody(),
	})

	ctx := service.NewMockContext(t, mockServer)

	responses := ListTags(ctx, TagListRequest{OrgName: "testorg", RepoNames: []string{"repo1"}, Filter: "^v2"})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].Name != "v2.0.0" {
		t.Errorf("Name = %q, want v2.0.0", responses[0].Name)
	}
}

func TestListTags_GraphQL_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := service.NewMockContext(t, mockServer)

	responses := ListTags(ctx, TagListRequest{OrgName: "testorg"})

	if len(responses) != 0 {
		t.Fatalf("len(responses) = %d, want 0 on error", len(responses))
	}
}

func TestListTags_REST_MultiRepo(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	tagsBody := []map[string]interface{}{
		{"name": "v1.0.0", "commit": map[string]interface{}{"sha": "sha-v1"}},
	}
	mockServer.SetResponse("/repos/testorg/repo1/tags", testutils.MockResponse{StatusCode: http.StatusOK, Body: tagsBody})
	mockServer.SetResponse("/repos/testorg/repo2/tags", testutils.MockResponse{StatusCode: http.StatusOK, Body: tagsBody})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := ListTags(ctx, TagListRequest{OrgName: "testorg", RepoNames: []string{"repo1", "repo2"}})

	if len(responses) != 2 {
		t.Fatalf("len(responses) = %d, want 2", len(responses))
	}
	for _, r := range responses {
		if r.Name != "v1.0.0" {
			t.Errorf("Name = %q, want v1.0.0", r.Name)
		}
	}
}

func TestListTags_REST_MultiRepo_PartialError(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/tags", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       []map[string]interface{}{{"name": "v1.0.0", "commit": map[string]interface{}{"sha": "sha-v1"}}},
	})
	mockServer.SetResponse("/repos/testorg/repo2/tags", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "not found"},
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := ListTags(ctx, TagListRequest{OrgName: "testorg", RepoNames: []string{"repo1", "repo2"}})

	if len(responses) != 2 {
		t.Fatalf("len(responses) = %d, want 2 (1 tag for repo1 + 1 error entry for repo2)", len(responses))
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
