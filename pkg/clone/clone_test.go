// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package clone

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/pradyb/sgh-cli/internal/model"
	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

// requireGit skips the test if the git binary isn't available on PATH.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available in PATH")
	}
}

// runGit runs a git command in dir, failing the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

// newLocalGitRepo creates a small real git repository under a fresh temp
// directory named exactly `name` (so a later `git clone <path>` — which
// names the destination after the source's basename — produces a directory
// called `name`). It has a "main" branch with a committed README.md, and a
// "feature" branch (checked out last) with an additional feature.txt commit.
func newLocalGitRepo(t *testing.T, name string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.name", "Test User")
	runGit(t, dir, "config", "user.email", "test@example.com")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello from "+name), 0o644); err != nil {
		t.Fatalf("failed to write README.md: %v", err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-q", "-m", "initial commit")

	runGit(t, dir, "checkout", "-q", "-b", "feature")
	if err := os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature content"), 0o644); err != nil {
		t.Fatalf("failed to write feature.txt: %v", err)
	}
	runGit(t, dir, "add", "feature.txt")
	runGit(t, dir, "commit", "-q", "-m", "feature commit")
	runGit(t, dir, "checkout", "-q", "main")

	return dir
}

// chdir switches the process's working directory to dir for the duration of
// the test, restoring the original on cleanup.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir to %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

func TestExecuteCloneCmd_Success(t *testing.T) {
	requireGit(t)
	srcDir := newLocalGitRepo(t, "repo1")
	destRoot := t.TempDir()
	chdir(t, destRoot)

	if err := executeCloneCmd(model.Repository{Name: "repo1", SSHUrl: srcDir}, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(destRoot, "repo1", "README.md")); err != nil {
		t.Errorf("expected cloned README.md to exist: %v", err)
	}
	// Cloning the default branch (main) should not pull in feature.txt.
	if _, err := os.Stat(filepath.Join(destRoot, "repo1", "feature.txt")); err == nil {
		t.Errorf("did not expect feature.txt on the default branch clone")
	}
}

func TestExecuteCloneCmd_WithBranch(t *testing.T) {
	requireGit(t)
	srcDir := newLocalGitRepo(t, "repo1")
	destRoot := t.TempDir()
	chdir(t, destRoot)

	if err := executeCloneCmd(model.Repository{Name: "repo1", SSHUrl: srcDir}, "feature"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(destRoot, "repo1", "feature.txt")); err != nil {
		t.Errorf("expected feature.txt from the 'feature' branch to exist: %v", err)
	}
}

func TestExecuteCloneCmd_Error(t *testing.T) {
	requireGit(t)
	destRoot := t.TempDir()
	chdir(t, destRoot)

	badPath := filepath.Join(t.TempDir(), "does-not-exist")
	if err := executeCloneCmd(model.Repository{Name: "missing", SSHUrl: badPath}, ""); err == nil {
		t.Fatal("expected an error cloning a nonexistent repository")
	}
}

type repoNode struct {
	name   string
	sshURL string
}

// searchRepositoriesGraphQLBody builds a /graphql response body matching the
// shape of model.SearchRepositoriesQuery (an inline "... on Repository"
// fragment, which is flattened onto the "node" object in the response).
func searchRepositoriesGraphQLBody(nodes []repoNode) map[string]interface{} {
	edges := make([]map[string]interface{}, 0, len(nodes))
	for _, n := range nodes {
		edges = append(edges, map[string]interface{}{
			"node": map[string]interface{}{
				"name":             n.name,
				"nameWithOwner":    "testorg/" + n.name,
				"url":              "https://github.com/testorg/" + n.name,
				"sshUrl":           n.sshURL,
				"description":      "",
				"isPrivate":        false,
				"isArchived":       false,
				"isDisabled":       false,
				"defaultBranchRef": map[string]interface{}{"name": "main"},
				"primaryLanguage":  map[string]interface{}{"name": "Go"},
				"pullRequests":     map[string]interface{}{"totalCount": 0},
			},
		})
	}
	return map[string]interface{}{
		"data": map[string]interface{}{
			"search": map[string]interface{}{
				"repositoryCount": len(nodes),
				"pageInfo":        map[string]interface{}{"endCursor": "", "hasNextPage": false},
				"edges":           edges,
			},
		},
	}
}

func TestCloneRepositories_NoReposSelected(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	// The mock server's built-in default /graphql response doesn't match the
	// shape of model.SearchRepositoriesQuery, so it must be overridden even
	// though we expect zero results here. ctx.Config has no organization
	// configured, so GetReposForOrg(all=false) filters the (empty) search
	// result down to no repositories regardless — exercising the "no
	// repositories to clone" early return without needing any real git repos.
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       searchRepositoriesGraphQLBody(nil),
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	if err := CloneRepositories(ctx, "testorg", nil, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCloneRepositories_GetReposForOrgError(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"errors": []map[string]interface{}{{"message": "boom"}}},
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	if err := CloneRepositories(ctx, "testorg", nil, ""); err == nil {
		t.Fatal("expected an error propagated from repo.GetReposForOrg")
	}
}

func TestCloneRepositories_EmptySelection_ClonesAll(t *testing.T) {
	requireGit(t)
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()

	src1 := newLocalGitRepo(t, "repo1")
	src2 := newLocalGitRepo(t, "repo2")
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       searchRepositoriesGraphQLBody([]repoNode{{name: "repo1", sshURL: src1}, {name: "repo2", sshURL: src2}}),
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true
	ctx.Config.AddOrganization("testorg")

	destRoot := t.TempDir()
	chdir(t, destRoot)

	if err := CloneRepositories(ctx, "testorg", nil, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, name := range []string{"repo1", "repo2"} {
		if _, err := os.Stat(filepath.Join(destRoot, name, "README.md")); err != nil {
			t.Errorf("expected %s to have been cloned: %v", name, err)
		}
	}
}

func TestCloneRepositories_ExplicitRepos_ClonesOnlySelected(t *testing.T) {
	requireGit(t)
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()

	src1 := newLocalGitRepo(t, "repo1")
	src2 := newLocalGitRepo(t, "repo2")
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       searchRepositoriesGraphQLBody([]repoNode{{name: "repo1", sshURL: src1}, {name: "repo2", sshURL: src2}}),
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true
	ctx.Config.AddOrganization("testorg")

	destRoot := t.TempDir()
	chdir(t, destRoot)

	if err := CloneRepositories(ctx, "testorg", []string{"repo1"}, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(destRoot, "repo1", "README.md")); err != nil {
		t.Errorf("expected repo1 to have been cloned: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destRoot, "repo2")); err == nil {
		t.Error("did not expect repo2 to have been cloned (it was not in the explicit selection)")
	}
}

func TestCloneRepositories_PartialFailure(t *testing.T) {
	requireGit(t)
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()

	src1 := newLocalGitRepo(t, "repo1")
	badPath := filepath.Join(t.TempDir(), "does-not-exist")
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       searchRepositoriesGraphQLBody([]repoNode{{name: "repo1", sshURL: src1}, {name: "repo2", sshURL: badPath}}),
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true
	ctx.Config.AddOrganization("testorg")

	destRoot := t.TempDir()
	chdir(t, destRoot)

	err := CloneRepositories(ctx, "testorg", []string{"repo1", "repo2"}, "")
	if err == nil {
		t.Fatal("expected an aggregated error mentioning the failed repository")
	}

	if _, statErr := os.Stat(filepath.Join(destRoot, "repo1", "README.md")); statErr != nil {
		t.Errorf("expected repo1 to still have been cloned despite repo2 failing: %v", statErr)
	}
}
