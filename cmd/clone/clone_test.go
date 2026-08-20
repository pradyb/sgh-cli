// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package clone

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

// newTestRoot builds a minimal fake parent command that only defines the
// persistent flag the clone command reads ("org" via cmd.Flags().GetString).
// It intentionally has no PersistentPreRun/PersistentPostRun (unlike the real
// cmd/root.go), so it never calls os.Exit on an error path during tests.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().StringP("org", "o", "", "")
	return root
}

func execCmd(cmd *cobra.Command, args ...string) error {
	root := newTestRoot()
	root.AddCommand(cmd)
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	return root.Execute()
}

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
// directory named exactly `name`, with a "main" branch (a committed
// README.md) and a "feature" branch (an additional feature.txt commit).
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

// searchRepositoriesGraphQLBody builds a /graphql response body matching the
// shape of model.SearchRepositoriesQuery (an inline "... on Repository"
// fragment, which is flattened onto the "node" object in the response).
func searchRepositoriesGraphQLBody(names, urls []string) map[string]interface{} {
	edges := make([]map[string]interface{}, 0, len(names))
	for i, n := range names {
		edges = append(edges, map[string]interface{}{
			"node": map[string]interface{}{
				"name":             n,
				"nameWithOwner":    "acme/" + n,
				"url":              "https://github.com/acme/" + n,
				"sshUrl":           urls[i],
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
				"repositoryCount": len(names),
				"pageInfo":        map[string]interface{}{"endCursor": "", "hasNextPage": false},
				"edges":           edges,
			},
		},
	}
}

func TestNewCloneCommand_Structure(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)

	cmd := NewCloneCommand(ctx)
	if cmd.Use != "clone" {
		t.Errorf("Use = %q, want clone", cmd.Use)
	}
	if cmd.Flags().Lookup("repository") == nil {
		t.Error("expected --repository flag to be registered")
	}
	if cmd.Flags().Lookup("branch") == nil {
		t.Error("expected --branch flag to be registered")
	}
}

func TestCloneCommand_Success(t *testing.T) {
	requireGit(t)
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()

	src := newLocalGitRepo(t, "repo1")
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       searchRepositoriesGraphQLBody([]string{"repo1"}, []string{src}),
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true
	ctx.Config.AddOrganization("acme")

	destRoot := t.TempDir()
	chdir(t, destRoot)

	if err := execCmd(NewCloneCommand(ctx), "clone", "--org", "acme", "-r", "repo1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(destRoot, "repo1", "README.md")); err != nil {
		t.Errorf("expected repo1 to have been cloned: %v", err)
	}
}

func TestCloneCommand_WithBranchFlag(t *testing.T) {
	requireGit(t)
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()

	src := newLocalGitRepo(t, "repo1")
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       searchRepositoriesGraphQLBody([]string{"repo1"}, []string{src}),
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true
	ctx.Config.AddOrganization("acme")

	destRoot := t.TempDir()
	chdir(t, destRoot)

	if err := execCmd(NewCloneCommand(ctx), "clone", "--org", "acme", "-r", "repo1", "--branch", "feature"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(destRoot, "repo1", "feature.txt")); err != nil {
		t.Errorf("expected feature.txt from the 'feature' branch to exist: %v", err)
	}
}

// TestCloneCommand_ErrorPropagation exercises the Run's error-logging branch:
// the clone command only logs the error returned by clone.CloneRepositories
// (it does not surface it as a cobra error), so execCmd should still report
// a clean (nil) exit even though the underlying GraphQL search failed.
func TestCloneCommand_ErrorPropagation(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"errors": []map[string]interface{}{{"message": "boom"}}},
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	destRoot := t.TempDir()
	chdir(t, destRoot)

	if err := execCmd(NewCloneCommand(ctx), "clone", "--org", "acme"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
