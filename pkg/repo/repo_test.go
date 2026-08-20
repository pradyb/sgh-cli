// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"testing"

	"github.com/pradyb/sgh-cli/internal/model"
	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

// graphqlRepoSearchBody returns a two-repo GraphQL search response, deliberately
// out of alphabetical order so tests can also assert on the name-sort behaviour.
func graphqlRepoSearchBody() map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"search": map[string]interface{}{
				"repositoryCount": 2,
				"pageInfo":        map[string]interface{}{"endCursor": "", "hasNextPage": false},
				"edges": []map[string]interface{}{
					{"node": map[string]interface{}{
						"name":             "repo-b",
						"nameWithOwner":    "testorg/repo-b",
						"url":              "https://github.com/testorg/repo-b",
						"sshUrl":           "git@github.com:testorg/repo-b.git",
						"description":      "desc b",
						"isPrivate":        false,
						"isArchived":       false,
						"isDisabled":       false,
						"defaultBranchRef": map[string]interface{}{"name": "main"},
						"primaryLanguage":  map[string]interface{}{"name": "Go"},
						"pullRequests":     map[string]interface{}{"totalCount": 1},
					}},
					{"node": map[string]interface{}{
						"name":             "repo-a",
						"nameWithOwner":    "testorg/repo-a",
						"url":              "https://github.com/testorg/repo-a",
						"sshUrl":           "git@github.com:testorg/repo-a.git",
						"description":      "desc a",
						"isPrivate":        true,
						"isArchived":       false,
						"isDisabled":       false,
						"defaultBranchRef": map[string]interface{}{"name": "main"},
						"primaryLanguage":  map[string]interface{}{"name": "Python"},
						"pullRequests":     map[string]interface{}{"totalCount": 0},
					}},
				},
			},
		},
	}
}

// emptyGraphqlRepoSearchBody returns a zero-result GraphQL search response
// shaped to match model.SearchRepositoriesQuery. The mock server's built-in
// default GraphQL response is shaped for a different query and fails to
// decode against this one, so tests that expect zero repos still need an
// explicit, correctly-shaped override.
func emptyGraphqlRepoSearchBody() map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"search": map[string]interface{}{
				"repositoryCount": 0,
				"pageInfo":        map[string]interface{}{"endCursor": "", "hasNextPage": false},
				"edges":           []map[string]interface{}{},
			},
		},
	}
}

// ---- ownerQualifier ----

func TestOwnerQualifier_Cached(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)

	ctx.Config.SetOwnerType("testorg", "User")
	if got := ownerQualifier(ctx, "testorg"); got != "user" {
		t.Errorf("ownerQualifier() = %q, want user", got)
	}

	ctx.Config.SetOwnerType("testorg2", "Organization")
	if got := ownerQualifier(ctx, "testorg2"); got != "org" {
		t.Errorf("ownerQualifier() = %q, want org", got)
	}
}

func TestOwnerQualifier_FetchesAndCaches(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/users/testorg", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"type": "Organization"},
	})

	ctx := service.NewMockContext(t, mockServer)

	if got := ownerQualifier(ctx, "testorg"); got != "org" {
		t.Errorf("ownerQualifier() = %q, want org", got)
	}
	if cached := ctx.Config.OwnerTypeFor("testorg"); cached != "Organization" {
		t.Errorf("OwnerTypeFor() = %q, want Organization to be cached", cached)
	}
}

func TestOwnerQualifier_FetchesUser(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/users/someuser", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"type": "User"},
	})

	ctx := service.NewMockContext(t, mockServer)

	if got := ownerQualifier(ctx, "someuser"); got != "user" {
		t.Errorf("ownerQualifier() = %q, want user", got)
	}
}

func TestOwnerQualifier_ErrorFallsBackToOrg(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	// No override for /users/testorg — the catch-all default returns 404.

	ctx := service.NewMockContext(t, mockServer)

	if got := ownerQualifier(ctx, "testorg"); got != "org" {
		t.Errorf("ownerQualifier() = %q, want org (fallback)", got)
	}
	if cached := ctx.Config.OwnerTypeFor("testorg"); cached != "" {
		t.Errorf("OwnerTypeFor() = %q, want uncached on error", cached)
	}
}

// ---- GetReposForOrg ----

func TestGetReposForOrg_All(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{StatusCode: http.StatusOK, Body: graphqlRepoSearchBody()})

	ctx := service.NewMockContext(t, mockServer)

	repos, err := GetReposForOrg(ctx, "testorg", true)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("len(repos) = %d, want 2", len(repos))
	}
	if repos[0].Name != "repo-a" || repos[1].Name != "repo-b" {
		t.Fatalf("repos not sorted by name: %+v", repos)
	}
	got := repos[0]
	if got.HTMLUrl != "https://github.com/testorg/repo-a" || got.SSHUrl != "git@github.com:testorg/repo-a.git" {
		t.Errorf("repo-a URLs = %+v", got)
	}
	if got.Description != "desc a" || got.DefaultBranch != "main" || got.Language != "Python" {
		t.Errorf("repo-a fields = %+v", got)
	}
	if !got.Private {
		t.Error("repo-a should be private")
	}
	if got.OpenPullRequestsCount != 0 {
		t.Errorf("repo-a OpenPullRequestsCount = %d, want 0", got.OpenPullRequestsCount)
	}
	if repos[1].OpenPullRequestsCount != 1 {
		t.Errorf("repo-b OpenPullRequestsCount = %d, want 1", repos[1].OpenPullRequestsCount)
	}
}

func TestGetReposForOrg_All_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := service.NewMockContext(t, mockServer)

	repos, err := GetReposForOrg(ctx, "testorg", true)

	if err == nil {
		t.Fatal("expected an error")
	}
	if repos != nil {
		t.Errorf("expected nil repos on error, got %v", repos)
	}
}

func TestGetReposForOrg_NotConfigured(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{StatusCode: http.StatusOK, Body: emptyGraphqlRepoSearchBody()})

	ctx := service.NewMockContext(t, mockServer)

	repos, err := GetReposForOrg(ctx, "testorg", false)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 0 {
		t.Errorf("len(repos) = %d, want 0 for an unconfigured org", len(repos))
	}
}

func TestGetReposForOrg_Configured_NoPatterns(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{StatusCode: http.StatusOK, Body: graphqlRepoSearchBody()})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Config.AddOrganization("testorg")

	repos, err := GetReposForOrg(ctx, "testorg", false)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("len(repos) = %d, want 2", len(repos))
	}

	// GetReposForOrg should have cached the repo names for fuzzy search.
	names := ctx.Config.RepositoriesNames("testorg")
	if len(names) != 2 {
		t.Errorf("cached repo names = %v, want 2 entries", names)
	}
}

func TestGetReposForOrg_Configured_ExcludePattern(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{StatusCode: http.StatusOK, Body: graphqlRepoSearchBody()})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Config.AddOrganization("testorg")
	ctx.Config.AddRepositoryPattern("testorg", false, true, "^repo-a$")

	repos, err := GetReposForOrg(ctx, "testorg", false)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 1 || repos[0].Name != "repo-b" {
		t.Fatalf("repos = %+v, want only repo-b", repos)
	}
}

// ---- SearchRepos ----

func TestSearchRepos_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{StatusCode: http.StatusOK, Body: graphqlRepoSearchBody()})

	ctx := service.NewMockContext(t, mockServer)

	repos, err := SearchRepos(ctx, "testorg", "widget", "go", "cli")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("len(repos) = %d, want 2", len(repos))
	}
	if repos[0].Name != "repo-a" || repos[1].Name != "repo-b" {
		t.Errorf("repos not sorted: %+v", repos)
	}

	var sawQuery bool
	for _, r := range mockServer.GetRequests() {
		if r.Path == "/graphql" {
			sawQuery = true
		}
	}
	if !sawQuery {
		t.Error("expected a /graphql request to have been recorded")
	}
}

func TestSearchRepos_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := service.NewMockContext(t, mockServer)

	repos, err := SearchRepos(ctx, "testorg", "", "", "")

	if err == nil {
		t.Fatal("expected an error")
	}
	if repos != nil {
		t.Errorf("expected nil repos on error, got %v", repos)
	}
}

// ---- GetSelectedRepoNames ----

func TestGetSelectedRepoNames_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{StatusCode: http.StatusOK, Body: graphqlRepoSearchBody()})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Config.AddOrganization("testorg")

	names, err := GetSelectedRepoNames(ctx, "testorg")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 || names[0] != "repo-a" || names[1] != "repo-b" {
		t.Errorf("names = %v, want [repo-a repo-b]", names)
	}
}

func TestGetSelectedRepoNames_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := service.NewMockContext(t, mockServer)

	names, err := GetSelectedRepoNames(ctx, "testorg")

	if err == nil {
		t.Fatal("expected an error")
	}
	if names != nil {
		t.Errorf("expected nil names on error, got %v", names)
	}
}

// ---- filteredRepos ----

func TestFilteredRepos(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)
	ctx.Config.AddRepositoryPattern("testorg", true, false, "^keep-")

	repos := []model.Repository{{Name: "keep-me"}, {Name: "drop-me"}}

	got, err := filteredRepos(ctx, repos, "testorg")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "keep-me" {
		t.Errorf("got = %+v, want only keep-me", got)
	}
}

// ---- resolveRepoList ----

func TestResolveRepoList_ExplicitReposWithConfigExcludePattern(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)
	ctx.Config.AddRepositoryPattern("testorg", false, true, "^test-")

	got := resolveRepoList(ctx, "testorg", []string{"test-a", "prod-b"}, nil)

	if len(got) != 1 || got[0] != "prod-b" {
		t.Errorf("got = %v, want [prod-b]", got)
	}
}

func TestResolveRepoList_ExplicitExclude(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)

	got := resolveRepoList(ctx, "testorg", []string{"repo1", "repo2"}, []string{"repo1"})

	if len(got) != 1 || got[0] != "repo2" {
		t.Errorf("got = %v, want [repo2]", got)
	}
}

func TestResolveRepoList_NoExplicitRepos_ResolvesFromOrg(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{StatusCode: http.StatusOK, Body: graphqlRepoSearchBody()})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Config.AddOrganization("testorg")

	got := resolveRepoList(ctx, "testorg", nil, nil)

	if len(got) != 2 || got[0] != "repo-a" || got[1] != "repo-b" {
		t.Errorf("got = %v, want [repo-a repo-b]", got)
	}
}

func TestResolveRepoList_NoExplicitRepos_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := service.NewMockContext(t, mockServer)

	got := resolveRepoList(ctx, "testorg", nil, nil)

	if got != nil {
		t.Errorf("got = %v, want nil on resolution error", got)
	}
}

// ---- ArchiveRepos ----

func TestArchiveRepos_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := ArchiveRepos(ctx, "testorg", []string{"repo1"}, nil, true)

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", responses[0].ErrorMessage)
	}
	if responses[0].Type != "ARCHIVE" {
		t.Errorf("Type = %q, want ARCHIVE", responses[0].Type)
	}
}

func TestArchiveRepos_Unarchive(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := ArchiveRepos(ctx, "testorg", []string{"repo1"}, nil, false)

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", responses[0].ErrorMessage)
	}
	if responses[0].Type != "UNARCHIVE" {
		t.Errorf("Type = %q, want UNARCHIVE", responses[0].Type)
	}
}

func TestArchiveRepos_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := ArchiveRepos(ctx, "testorg", []string{"repo1"}, nil, true)

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
}

func TestArchiveRepos_NoReposResolved(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{StatusCode: http.StatusOK, Body: emptyGraphqlRepoSearchBody()})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := ArchiveRepos(ctx, "testorg", nil, nil, true)

	if len(responses) != 0 {
		t.Errorf("len(responses) = %d, want 0 for an unconfigured org with no explicit repos", len(responses))
	}
}

// ---- SetRepoVisibility ----

func TestSetRepoVisibility_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := SetRepoVisibility(ctx, "testorg", []string{"repo1"}, nil, "private")

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", responses[0].ErrorMessage)
	}
	if responses[0].Type != "SET_VISIBILITY" {
		t.Errorf("Type = %q, want SET_VISIBILITY", responses[0].Type)
	}
}

func TestSetRepoVisibility_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "not found"},
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := SetRepoVisibility(ctx, "testorg", []string{"repo1"}, nil, "public")

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
}
