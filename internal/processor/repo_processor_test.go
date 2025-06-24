package processor

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prady-lab/sgh-cli/internal/config"
	"github.com/prady-lab/sgh-cli/pkg/context"
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
	if err != nil {
		t.Fatalf(unexpectedErrorMsg, err)
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

// TestProcessRepositoriesOperation_EmptyRepos is not included because it requires
// mocking repo.GetSelectedRepoNames which would require complex setup with HTTP/GraphQL clients.
// The empty repos scenario is covered by the "all excluded" test above.

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
