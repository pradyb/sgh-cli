// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package processor

import (
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pradyb/sgh-cli/internal/config"
	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
	"github.com/pradyb/sgh-cli/pkg/context"
)

const (
	unexpectedErrorMsg = "unexpected error: %v"
)

func makeTestConfig(orgName string, repos []string, noOfWorkers int) *config.Config {
	cfg := &config.Config{NoOfWorkers: noOfWorkers}
	cfg.AddOrganization(orgName)
	for _, repo := range repos {
		cfg.AddRepository(orgName, repo)
	}
	return cfg
}

func TestProcessRepositoriesOperationBasicHandlers(t *testing.T) {
	repos := []string{"repo1", "repo2", "repo3"}
	ctx := &context.Context{}
	ctx.Config = makeTestConfig("org", repos, 2)

	var opCalls, resCalls, errCalls atomic.Int32

	opHandler := func(ctx *context.Context, org, repo string) (bool, error) {
		opCalls.Add(1)
		if repo == "repo2" {
			return false, errors.New("fail")
		}
		return true, nil
	}
	resHandler := func(repo string, result RepoOperationResult[bool]) {
		resCalls.Add(1)
		if repo == "repo2" && result.Result {
			t.Errorf("repo2 should not succeed")
		}
	}
	errHandler := func(repo string, err error) {
		errCalls.Add(1)
		if repo != "repo2" {
			t.Errorf("unexpected error for %s", repo)
		}
	}

	err := ProcessRepositoriesOperation(ctx, "org", repos, nil, OperationCreateBranch, opHandler, resHandler, errHandler)
	if err == nil {
		t.Fatal("expected error when repo2 fails, got nil")
	}
	if opCalls.Load() != 3 || resCalls.Load() != 2 || errCalls.Load() != 1 {
		t.Errorf("calls: op=%d res=%d err=%d", opCalls.Load(), resCalls.Load(), errCalls.Load())
	}
}

func TestProcessRepositoriesOperationExclusion(t *testing.T) {
	repos := []string{"repo1", "repo2", "repo3"}
	ctx := &context.Context{}
	ctx.Config = makeTestConfig("org", repos, 1)

	opHandler := func(ctx *context.Context, org, repo string) (bool, error) {
		if repo == "repo2" {
			t.Errorf("repo2 should be excluded")
		}
		return true, nil
	}
	resHandler := func(repo string, result RepoOperationResult[bool]) {
		// This function is empty because we're only testing exclusion logic
	}
	errHandler := func(repo string, err error) {
		// This function is empty because we don't expect errors in this test
	}
	err := ProcessRepositoriesOperation(ctx, "org", repos, []string{"repo2"}, OperationCreateBranch, opHandler, resHandler, errHandler)
	if err != nil {
		t.Fatalf(unexpectedErrorMsg, err)
	}
}

func TestProcessRepositoriesOperationAllExcluded(t *testing.T) {
	repos := []string{"repo1"}
	ctx := &context.Context{}
	ctx.Config = makeTestConfig("org", repos, 1)

	called := false
	opHandler := func(ctx *context.Context, org, repo string) (bool, error) {
		called = true
		return true, nil
	}
	resHandler := func(repo string, result RepoOperationResult[bool]) {
		// This function is empty because no operations should be performed
	}
	errHandler := func(repo string, err error) {
		// This function is empty because no operations should be performed
	}
	err := ProcessRepositoriesOperation(ctx, "org", repos, []string{"repo1"}, OperationCreateBranch, opHandler, resHandler, errHandler)
	if err != nil {
		t.Fatalf(unexpectedErrorMsg, err)
	}
	if called {
		t.Error("operation handler should not be called when all repos are excluded")
	}
}

// graphqlRepoSearchBody returns a two-repo GraphQL search response shaped to
// match model.SearchRepositoriesQuery, as used by repo.GetSelectedRepoNames.
// The mock server's built-in default GraphQL response is shaped for a
// different query, so tests exercising the "no explicit repos" branch need
// this explicit, correctly-shaped override.
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

// ---- ResolveRepositoryNames ----

func TestResolveRepositoryNames_EmptyRepos_ResolvesFromOrg(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{StatusCode: http.StatusOK, Body: graphqlRepoSearchBody()})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Config.AddOrganization("testorg")

	names, err := ResolveRepositoryNames(ctx, "testorg", nil, nil)
	if err != nil {
		t.Fatalf(unexpectedErrorMsg, err)
	}
	if len(names) != 2 || names[0] != "repo-a" || names[1] != "repo-b" {
		t.Errorf("names = %v, want [repo-a repo-b]", names)
	}
}

func TestResolveRepositoryNames_EmptyRepos_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Config.AddOrganization("testorg")

	names, err := ResolveRepositoryNames(ctx, "testorg", nil, nil)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if names != nil {
		t.Errorf("expected nil names on error, got %v", names)
	}
}

func TestResolveRepositoryNames_ExplicitRepos_FuzzyMatch(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Config = makeTestConfig("testorg", []string{"repo1", "repo2", "repo3"}, 1)

	names, err := ResolveRepositoryNames(ctx, "testorg", []string{"repo1", "repo3"}, nil)
	if err != nil {
		t.Fatalf(unexpectedErrorMsg, err)
	}
	if len(names) != 2 || names[0] != "repo1" || names[1] != "repo3" {
		t.Errorf("names = %v, want [repo1 repo3]", names)
	}
}

func TestResolveRepositoryNames_PatternFiltering(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Config = makeTestConfig("testorg", []string{"keep-a", "keep-b", "drop-c"}, 1)
	ctx.Config.AddRepositoryPattern("testorg", true, false, "^keep-")

	names, err := ResolveRepositoryNames(ctx, "testorg", []string{"keep-a", "keep-b", "drop-c"}, nil)
	if err != nil {
		t.Fatalf(unexpectedErrorMsg, err)
	}
	if len(names) != 2 || names[0] != "keep-a" || names[1] != "keep-b" {
		t.Errorf("names = %v, want [keep-a keep-b]", names)
	}
}

func TestResolveRepositoryNames_ExcludeList(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Config = makeTestConfig("testorg", []string{"repo1", "repo2", "repo3"}, 1)

	names, err := ResolveRepositoryNames(ctx, "testorg", []string{"repo1", "repo2", "repo3"}, []string{"repo2"})
	if err != nil {
		t.Fatalf(unexpectedErrorMsg, err)
	}
	if len(names) != 2 || names[0] != "repo1" || names[1] != "repo3" {
		t.Errorf("names = %v, want [repo1 repo3]", names)
	}
}

// ---- ProcessRepositoriesOperation: empty repos / resolution errors ----

func TestProcessRepositoriesOperation_EmptyRepos_ResolvesFromOrg(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{StatusCode: http.StatusOK, Body: graphqlRepoSearchBody()})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true
	ctx.Config.AddOrganization("testorg")

	var handled []string
	var mu sync.Mutex
	opHandler := func(ctx *context.Context, org, repo string) (bool, error) {
		mu.Lock()
		handled = append(handled, repo)
		mu.Unlock()
		return true, nil
	}
	resHandler := func(repo string, result RepoOperationResult[bool]) {}
	errHandler := func(repo string, err error) {}

	err := ProcessRepositoriesOperation(ctx, "testorg", nil, nil, OperationCreateBranch, opHandler, resHandler, errHandler)
	if err != nil {
		t.Fatalf(unexpectedErrorMsg, err)
	}
	if len(handled) != 2 {
		t.Errorf("expected 2 repos handled, got %v", handled)
	}
}

func TestProcessRepositoriesOperation_EmptyRepos_ResolutionError(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true
	ctx.Config.AddOrganization("testorg")

	called := false
	opHandler := func(ctx *context.Context, org, repo string) (bool, error) {
		called = true
		return true, nil
	}
	resHandler := func(repo string, result RepoOperationResult[bool]) {}
	errHandler := func(repo string, err error) {}

	err := ProcessRepositoriesOperation(ctx, "testorg", nil, nil, OperationCreateBranch, opHandler, resHandler, errHandler)
	if err == nil {
		t.Fatal("expected an error when repo resolution fails, got nil")
	}
	if called {
		t.Error("operation handler should not be called when repo resolution fails")
	}
}

func TestProcessRepositoriesOperation_NoOfWorkersDefaultsToOne(t *testing.T) {
	repos := []string{"repo1", "repo2"}
	ctx := &context.Context{}
	ctx.Config = makeTestConfig("org", repos, 0) // NoOfWorkers unset (0) should default to 1

	var maxConcurrent, concurrentOps atomic.Int32
	var mu sync.Mutex

	opHandler := func(ctx *context.Context, org, repo string) (bool, error) {
		c := concurrentOps.Add(1)
		mu.Lock()
		if c > maxConcurrent.Load() {
			maxConcurrent.Store(c)
		}
		mu.Unlock()
		time.Sleep(5 * time.Millisecond)
		concurrentOps.Add(-1)
		return true, nil
	}
	resHandler := func(repo string, result RepoOperationResult[bool]) {}
	errHandler := func(repo string, err error) {}

	err := ProcessRepositoriesOperation(ctx, "org", repos, nil, OperationCreateBranch, opHandler, resHandler, errHandler)
	if err != nil {
		t.Fatalf(unexpectedErrorMsg, err)
	}
	if maxConcurrent.Load() != 1 {
		t.Errorf("expected exactly 1 concurrent worker with NoOfWorkers=0, got %d", maxConcurrent.Load())
	}
}

func TestProcessRepositoriesOperationAllErrors(t *testing.T) {
	repos := []string{"repo1", "repo2"}
	ctx := &context.Context{}
	ctx.Config = makeTestConfig("org", repos, 2)

	var errCalls, resCalls atomic.Int32
	opHandler := func(ctx *context.Context, org, repo string) (bool, error) {
		return false, errors.New("boom")
	}
	resHandler := func(repo string, result RepoOperationResult[bool]) {
		resCalls.Add(1)
	}
	errHandler := func(repo string, err error) {
		errCalls.Add(1)
	}

	err := ProcessRepositoriesOperation(ctx, "org", repos, nil, OperationCreateBranch, opHandler, resHandler, errHandler)
	if err == nil {
		t.Fatal("expected an error when all repos fail, got nil")
	}
	if !ctx.HasError {
		t.Error("expected ctx.HasError to be set to true")
	}
	if errCalls.Load() != 2 || resCalls.Load() != 0 {
		t.Errorf("errCalls=%d resCalls=%d, want errCalls=2 resCalls=0", errCalls.Load(), resCalls.Load())
	}
}

// ---- OperationEnum.String ----

func TestOperationEnumString(t *testing.T) {
	tests := []struct {
		op   OperationEnum
		want string
	}{
		{OperationCreateBranch, "CreateBranch"},
		{OperationDeleteBranch, "DeleteBranch"},
		{OperationCreateTag, "CreateTag"},
		{OperationDeleteTag, "DeleteTag"},
		{OperationCreatePullRequest, "CreatePullRequest"},
		{OperationListPullRequest, "ListPullRequest"},
		{OperationUpdatePullRequest, "UpdatePullRequest"},
		{OperationReviewPullRequest, "ReviewPullRequest"},
		{OperationMergePullRequest, "MergePullRequest"},
		{OperationListProtectedBranch, "ListProtectedBranch"},
		{OperationUpdateProtectedBranch, "UpdateProtectedBranch"},
		{OperationDeleteProtectedBranch, "DeleteProtectedBranch"},
		{OperationPostRelease, "PostRelease"},
		{OperationListCommits, "ListCommits"},
		{OperationListWorkflowRuns, "ListWorkflowRuns"},
		{OperationRerunWorkflow, "RerunWorkflow"},
		{OperationCancelWorkflow, "CancelWorkflow"},
		{OperationListBranches, "ListBranches"},
		{OperationListTags, "ListTags"},
		{OperationListSecretScanningAlerts, "ListSecretScanningAlerts"},
		{OperationListIssues, "ListIssues"},
		{OperationEnum(999), "UnknownOperation(999)"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.op.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProcessRepositoriesOperationConcurrency(t *testing.T) {
	repos := []string{"repo1", "repo2", "repo3", "repo4"}
	ctx := &context.Context{}
	ctx.Config = makeTestConfig("org", repos, 2)

	var concurrentOps atomic.Int32
	var maxConcurrent atomic.Int32 // Use atomic for thread safety
	var done atomic.Int32
	var mu sync.Mutex // Add mutex for maxConcurrent updates

	opHandler := func(ctx *context.Context, org, repo string) (bool, error) {
		c := concurrentOps.Add(1)

		// Safely update maxConcurrent
		mu.Lock()
		current := maxConcurrent.Load()
		if c > current {
			maxConcurrent.Store(c)
		}
		mu.Unlock()

		// Simulate work with a small delay to make concurrency observable
		time.Sleep(10 * time.Millisecond)
		concurrentOps.Add(-1)
		done.Add(1)
		return true, nil
	}
	resHandler := func(repo string, result RepoOperationResult[bool]) {
		// This function is empty because we're only testing concurrency behavior
	}
	errHandler := func(repo string, err error) {
		// This function is empty because we don't expect errors in this test
	}
	err := ProcessRepositoriesOperation(ctx, "org", repos, nil, OperationCreateBranch, opHandler, resHandler, errHandler)
	if err != nil {
		t.Fatalf(unexpectedErrorMsg, err)
	}
	if maxConcurrent.Load() < 2 {
		t.Errorf("expected at least 2 concurrent workers, got %d", maxConcurrent.Load())
	}
	if done.Load() != int32(len(repos)) {
		t.Errorf("expected %d jobs done, got %d", len(repos), done.Load())
	}
}
