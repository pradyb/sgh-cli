// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package commit

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/internal/testutils"
	"github.com/pradyb/sgh-cli/pkg/context"
)

// newTestRoot builds a minimal parent command that only defines the
// persistent flags the commit command reads (org, json, limit), with no
// PersistentPreRun/PersistentPostRun — unlike the real root command, which
// os.Exit(1)s on flag validation issues or ctx.HasError, making it unsafe to
// drive error-path tests through.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().StringP("org", "o", "", "")
	root.PersistentFlags().BoolP("json", "J", false, "")
	root.PersistentFlags().Int("limit", 0, "")
	return root
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w

	// Drain concurrently: the anonymous pipe's buffer is small, and a
	// table-heavy Run can easily exceed it, so draining only after fn()
	// returns would deadlock.
	outCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outCh <- buf.String()
	}()

	fn()

	os.Stdout = orig
	w.Close()
	return <-outCh
}

func newMockedContext(t *testing.T) (*context.Context, *testutils.MockGitHubServer) {
	t.Helper()
	mockServer := testutils.NewMockGitHubServer()
	t.Cleanup(mockServer.Close)
	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true
	return ctx, mockServer
}

func commitsBody() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"sha":        "abc123",
			"node_id":    "MDY6Q29tbWl0",
			"html_url":   "https://github.com/testorg/repo1/commit/abc123",
			"commit_url": "https://api.github.com/repos/testorg/repo1/commits/abc123",
			"author":     map[string]interface{}{"login": "jane-doe"},
			"commit": map[string]interface{}{
				"message": "Fix the bug",
				"author":  map[string]interface{}{"name": "Jane Doe", "email": "jane@example.com", "date": "2024-01-01T00:00:00Z"},
			},
		},
	}
}

func TestListCommand_Summary_Success(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/testorg/repo1/commits", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       commitsBody(),
	})

	root := newTestRoot()
	root.AddCommand(NewCommitCommand(ctx))
	root.SetArgs([]string{"commit", "list", "--org", "testorg", "-r", "repo1"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "repo1") {
		t.Errorf("expected output to mention repo1, got: %s", out)
	}
}

func TestListCommand_Details_Success(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/testorg/repo1/commits", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       commitsBody(),
	})

	root := newTestRoot()
	root.AddCommand(NewCommitCommand(ctx))
	root.SetArgs([]string{"commit", "list", "--org", "testorg", "-r", "repo1", "--details", "--include-merge-commits", "--sort", "repo", "--compact"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "abc123") && !strings.Contains(out, "repo1") {
		t.Errorf("expected detailed output to mention the commit, got: %s", out)
	}
}

func TestListCommand_JSONOutput(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.JSON = true
	mockServer.SetResponse("/repos/testorg/repo1/commits", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       commitsBody(),
	})

	root := newTestRoot()
	root.AddCommand(NewCommitCommand(ctx))
	root.SetArgs([]string{"commit", "list", "--org", "testorg", "-r", "repo1"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, `"abc123"`) {
		t.Errorf("expected JSON output to contain the commit sha, got: %s", out)
	}
}

func TestListCommand_Limit(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.JSON = true
	ctx.Limit = 1
	mockServer.SetResponse("/repos/testorg/repo1/commits", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       commitsBody(),
	})

	root := newTestRoot()
	root.AddCommand(NewCommitCommand(ctx))
	root.SetArgs([]string{"commit", "list", "--org", "testorg", "-r", "repo1"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestListCommand_Error(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/testorg/repo1/commits", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	root := newTestRoot()
	root.AddCommand(NewCommitCommand(ctx))
	root.SetArgs([]string{"commit", "list", "--org", "testorg", "-r", "repo1"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "repo1") {
		t.Errorf("expected error output to still mention repo1, got: %s", out)
	}
}

func TestListCommand_SinceUntilFlags(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/testorg/repo1/commits", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       commitsBody(),
	})

	root := newTestRoot()
	root.AddCommand(NewCommitCommand(ctx))
	root.SetArgs([]string{
		"commit", "list", "--org", "testorg", "-r", "repo1",
		"--since", "2024-06-01", "--until", "2024-06-30",
	})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestListCommand_Alias(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/testorg/repo1/commits", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       commitsBody(),
	})

	root := newTestRoot()
	root.AddCommand(NewCommitCommand(ctx))
	root.SetArgs([]string{"ci", "ls", "--org", "testorg", "-r", "repo1"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestRepoCompletionFn(t *testing.T) {
	ctx, _ := newMockedContext(t)

	root := newTestRoot()
	commitCmd := NewCommitCommand(ctx)
	root.AddCommand(commitCmd)
	root.SetArgs([]string{"commit", "--org", "testorg"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	_ = root.Execute()

	fn := repoCompletionFn(ctx)
	var listCmd *cobra.Command
	for _, c := range commitCmd.Commands() {
		if c.Name() == "list" {
			listCmd = c
		}
	}
	if listCmd == nil {
		t.Fatal("expected a list subcommand")
	}

	names, directive := fn(listCmd, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
	}
	// ctx.Config starts empty in the mocked context, so no repo names are
	// expected — this just exercises the completion function without panicking.
	if names == nil {
		names = []string{}
	}
}
