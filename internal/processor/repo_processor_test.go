package processor

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prady-lab/sgh-cli/internal/config"
	"github.com/prady-lab/sgh-cli/pkg/context"
)

func makeTestConfig(orgName string, repos []string, noOfWorkers int) *config.Config {
	cfg := &config.Config{NoOfWorkers: noOfWorkers}
	cfg.AddOrganization(orgName)
	for _, repo := range repos {
		cfg.AddRepository(orgName, repo)
	}
	return cfg
}

func TestProcessRepositoriesOperation_BasicHandlers(t *testing.T) {
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opCalls.Load() != 3 || resCalls.Load() != 2 || errCalls.Load() != 1 {
		t.Errorf("calls: op=%d res=%d err=%d", opCalls.Load(), resCalls.Load(), errCalls.Load())
	}
}

func TestProcessRepositoriesOperation_Exclusion(t *testing.T) {
	repos := []string{"repo1", "repo2", "repo3"}
	ctx := &context.Context{}
	ctx.Config = makeTestConfig("org", repos, 1)

	opHandler := func(ctx *context.Context, org, repo string) (bool, error) {
		if repo == "repo2" {
			t.Errorf("repo2 should be excluded")
		}
		return true, nil
	}
	resHandler := func(repo string, result RepoOperationResult[bool]) {}
	errHandler := func(repo string, err error) {}

	err := ProcessRepositoriesOperation(ctx, "org", repos, []string{"repo2"}, OperationCreateBranch, opHandler, resHandler, errHandler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProcessRepositoriesOperation_AllExcluded(t *testing.T) {
	repos := []string{"repo1"}
	ctx := &context.Context{}
	ctx.Config = makeTestConfig("org", repos, 1)

	called := false
	opHandler := func(ctx *context.Context, org, repo string) (bool, error) {
		called = true
		return true, nil
	}
	resHandler := func(repo string, result RepoOperationResult[bool]) {}
	errHandler := func(repo string, err error) {}

	err := ProcessRepositoriesOperation(ctx, "org", repos, []string{"repo1"}, OperationCreateBranch, opHandler, resHandler, errHandler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("operation handler should not be called when all repos are excluded")
	}
}

// TestProcessRepositoriesOperation_EmptyRepos is not included because it requires
// mocking repo.GetSelectedRepoNames which would require complex setup with HTTP/GraphQL clients.
// The empty repos scenario is covered by the "all excluded" test above.

func TestProcessRepositoriesOperation_Concurrency(t *testing.T) {
	repos := []string{"repo1", "repo2", "repo3", "repo4"}
	ctx := &context.Context{}
	ctx.Config = makeTestConfig("org", repos, 2)

	var concurrentOps atomic.Int32
	var maxConcurrent int32
	var done atomic.Int32

	opHandler := func(ctx *context.Context, org, repo string) (bool, error) {
		c := concurrentOps.Add(1)
		if c > maxConcurrent {
			maxConcurrent = c
		}
		// Simulate work with a small delay to make concurrency observable
		time.Sleep(10 * time.Millisecond)
		concurrentOps.Add(-1)
		done.Add(1)
		return true, nil
	}
	resHandler := func(repo string, result RepoOperationResult[bool]) {}
	errHandler := func(repo string, err error) {}

	err := ProcessRepositoriesOperation(ctx, "org", repos, nil, OperationCreateBranch, opHandler, resHandler, errHandler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if maxConcurrent < 2 {
		t.Errorf("expected at least 2 concurrent workers, got %d", maxConcurrent)
	}
	if done.Load() != int32(len(repos)) {
		t.Errorf("expected %d jobs done, got %d", len(repos), done.Load())
	}
}
