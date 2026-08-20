// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package protectedbranch

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
	"github.com/pradyb/sgh-cli/pkg/context"
)

// newTestRoot builds a minimal parent command that only defines the
// persistent flags the protected-branch command reads (org, json, dry-run),
// with no PersistentPreRun/PersistentPostRun — unlike the real root command,
// which os.Exit(1)s on flag validation issues or ctx.HasError, making it
// unsafe to drive error-path tests through.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().StringP("org", "o", "", "")
	root.PersistentFlags().BoolP("json", "J", false, "")
	root.PersistentFlags().Bool("dry-run", false, "")
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
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true
	return ctx, mockServer
}

// emptySearchGraphQLBody is a /graphql response shaped like
// model.SearchProtectedBranchesQuery with no results, so ListProtectedBranches's
// internal lookup cleanly returns no results (matching the unconfigured
// default GraphQL body would otherwise decode-error against this query shape).
var emptySearchGraphQLBody = map[string]interface{}{
	"data": map[string]interface{}{
		"search": map[string]interface{}{
			"repositoryCount": 0,
			"pageInfo": map[string]interface{}{
				"endCursor":   "",
				"hasNextPage": false,
			},
			"edges": []map[string]interface{}{},
		},
	},
}

// ---------------------------------------------------------------------------
// list
// ---------------------------------------------------------------------------

func TestListCommand_Success(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       emptySearchGraphQLBody,
	})

	root := newTestRoot()
	root.AddCommand(NewProtectedBranchCommand(ctx))
	root.SetArgs([]string{"protected-branch", "list", "--org", "acme", "-r", "repo1", "--branch", "main"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestListCommand_JSONOutput(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.JSON = true
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       emptySearchGraphQLBody,
	})

	root := newTestRoot()
	root.AddCommand(NewProtectedBranchCommand(ctx))
	root.SetArgs([]string{"pb", "list", "--org", "acme", "-r", "repo1"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "[") {
		t.Errorf("expected JSON array output, got: %s", out)
	}
}

func TestListCommand_ExcludeRepos(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       emptySearchGraphQLBody,
	})

	root := newTestRoot()
	root.AddCommand(NewProtectedBranchCommand(ctx))
	root.SetArgs([]string{"protected-branch", "list", "--org", "acme", "-r", "repo1", "-e", "repo2"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// update
// ---------------------------------------------------------------------------

func TestUpdateCommand_Success(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       emptySearchGraphQLBody,
	})
	// PUT .../protection is served by the mock server's built-in default handler.

	root := newTestRoot()
	root.AddCommand(NewProtectedBranchCommand(ctx))
	root.SetArgs([]string{
		"protected-branch", "update", "--org", "acme", "-r", "repo1", "--branch", "main",
		"--lock", "--remove-status-checks",
		"--add-bypass-user", "john-doe", "--add-push-user", "jane-doe",
	})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var sawUpdate bool
	for _, req := range mockServer.GetRequests() {
		if req.Method == http.MethodPut && req.Path == "/repos/acme/repo1/branches/main/protection" {
			sawUpdate = true
		}
	}
	if !sawUpdate {
		t.Error("expected a PUT request to the branch protection endpoint")
	}
	if ctx.HasError {
		t.Error("did not expect ctx.HasError to be set on a successful update")
	}
}

func TestUpdateCommand_Alias_RemoveUsers(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       emptySearchGraphQLBody,
	})

	root := newTestRoot()
	root.AddCommand(NewProtectedBranchCommand(ctx))
	root.SetArgs([]string{
		"pb", "edit", "--org", "acme", "-r", "repo1", "--branch", "main",
		"--remove-bypass-user", "john-doe", "--remove-push-user", "jane-doe",
	})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestUpdateCommand_Error(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       emptySearchGraphQLBody,
	})
	mockServer.SetResponse("/repos/acme/repo1/branches/main/protection", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	root := newTestRoot()
	root.AddCommand(NewProtectedBranchCommand(ctx))
	root.SetArgs([]string{"protected-branch", "update", "--org", "acme", "-r", "repo1", "--branch", "main"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !ctx.HasError {
		t.Error("expected ctx.HasError to be set after a failed update")
	}
}

func TestUpdateCommand_DryRun(t *testing.T) {
	ctx, _ := newMockedContext(t)
	ctx.DryRun = true

	root := newTestRoot()
	root.AddCommand(NewProtectedBranchCommand(ctx))
	root.SetArgs([]string{
		"protected-branch", "update", "--org", "acme", "-r", "repo1", "--branch", "main",
		"--lock", "--remove-status-checks",
		"--add-bypass-user", "john-doe", "--remove-bypass-user", "old-admin",
		"--add-push-user", "jane-doe", "--remove-push-user", "old-deploy",
	})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "DRY RUN") {
		t.Errorf("expected dry-run banner in output, got: %s", out)
	}
	if !strings.Contains(out, "john-doe") {
		t.Errorf("expected dry-run details to mention bypass user, got: %s", out)
	}
}

func TestUpdateCommand_MissingBranch_Error(t *testing.T) {
	ctx, _ := newMockedContext(t)

	root := newTestRoot()
	root.AddCommand(NewProtectedBranchCommand(ctx))
	root.SetArgs([]string{"protected-branch", "update", "--org", "acme", "-r", "repo1"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	if err := root.Execute(); err == nil {
		t.Fatal("expected an error when --branch is missing")
	}
}

// ---------------------------------------------------------------------------
// delete
// ---------------------------------------------------------------------------

func TestDeleteCommand_Success(t *testing.T) {
	ctx, _ := newMockedContext(t)

	root := newTestRoot()
	root.AddCommand(NewProtectedBranchCommand(ctx))
	root.SetArgs([]string{"protected-branch", "delete", "--org", "acme", "-r", "repo1", "--branch", "main"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestDeleteCommand_Alias_Error(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/acme/repo1/branches/main/protection", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "not found"},
	})

	root := newTestRoot()
	root.AddCommand(NewProtectedBranchCommand(ctx))
	root.SetArgs([]string{"pb", "rm", "--org", "acme", "-r", "repo1", "--branch", "main"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestDeleteCommand_DryRun(t *testing.T) {
	ctx, _ := newMockedContext(t)
	ctx.DryRun = true

	root := newTestRoot()
	root.AddCommand(NewProtectedBranchCommand(ctx))
	root.SetArgs([]string{"protected-branch", "delete", "--org", "acme", "-r", "repo1", "--branch", "main"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "DRY RUN") {
		t.Errorf("expected dry-run banner in output, got: %s", out)
	}
}

func TestDeleteCommand_MissingBranch_Error(t *testing.T) {
	ctx, _ := newMockedContext(t)

	root := newTestRoot()
	root.AddCommand(NewProtectedBranchCommand(ctx))
	root.SetArgs([]string{"protected-branch", "delete", "--org", "acme", "-r", "repo1"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	if err := root.Execute(); err == nil {
		t.Fatal("expected an error when --branch is missing")
	}
}

func TestNewProtectedBranchCommand_HasSubcommands(t *testing.T) {
	ctx, _ := newMockedContext(t)
	cmd := NewProtectedBranchCommand(ctx)

	want := map[string]bool{"list": false, "update": false, "delete": false}
	for _, c := range cmd.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("expected subcommand %q to be registered", name)
		}
	}
}
