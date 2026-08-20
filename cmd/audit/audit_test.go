// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package audit

import (
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig })

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close pipe writer: %v", err)
	}
	os.Stdout = orig

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read pipe: %v", err)
	}
	return string(out)
}

// newTestRoot builds a minimal parent command that only defines the
// persistent flags the audit command actually reads (org, compact). It has
// no PersistentPreRun/PersistentPostRun, unlike the real cmd/root.go, so it
// is safe to drive error paths through it without risking an os.Exit(1)
// from root's PersistentPostRun.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().StringP("org", "o", "", "")
	root.PersistentFlags().BoolP("compact", "C", false, "")
	return root
}

func auditLogBody() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"action":     "team.create",
			"actor":      "jane-doe",
			"actor_ip":   "127.0.0.1",
			"created_at": int64(1700000000000),
			"org":        "testorg",
			"user":       "jane-doe",
		},
	}
}

func TestNewAuditCommand_Metadata(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)

	cmd := NewAuditCommand(ctx)

	if cmd.Use != "audit <command>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "audit <command>")
	}
	found := false
	for _, alias := range cmd.Aliases {
		if alias == "al" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'al' alias, got %v", cmd.Aliases)
	}

	sub, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatalf("expected a 'list' subcommand: %v", err)
	}
	for _, flagName := range []string{"phrase", "include", "count"} {
		if sub.Flags().Lookup(flagName) == nil {
			t.Errorf("expected --%s flag on list command", flagName)
		}
	}
}

func TestAuditListCommand_TableOutput_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/orgs/testorg/audit-log", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       auditLogBody(),
	})
	ctx := servicetest.NewMockContext(t, mockServer)

	root := newTestRoot()
	root.AddCommand(NewAuditCommand(ctx))
	root.SetArgs([]string{"audit", "list", "--org", "testorg", "--phrase", "team.create", "--include", "git", "--count", "10"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "team.create") {
		t.Errorf("expected output to mention team.create, got %q", out)
	}

	requests := mockServer.GetRequests()
	found := false
	for _, req := range requests {
		if req.Path == "/orgs/testorg/audit-log" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a request to /orgs/testorg/audit-log, got %+v", requests)
	}
}

func TestAuditListCommand_JSONOutput_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/orgs/testorg/audit-log", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       auditLogBody(),
	})
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.JSON = true

	root := newTestRoot()
	root.AddCommand(NewAuditCommand(ctx))
	root.SetArgs([]string{"audit", "list", "--org", "testorg"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, `"action": "team.create"`) {
		t.Errorf("expected JSON output to contain team.create action, got %q", out)
	}
}

func TestAuditListCommand_CompactOutput_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/orgs/testorg/audit-log", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       auditLogBody(),
	})
	ctx := servicetest.NewMockContext(t, mockServer)

	root := newTestRoot()
	root.AddCommand(NewAuditCommand(ctx))
	root.SetArgs([]string{"audit", "list", "--org", "testorg", "--compact"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "team.create") {
		t.Errorf("expected compact output to mention team.create, got %q", out)
	}
}

// TestAuditListCommand_ErrorExits verifies that when pkg/audit reports an
// ErrorMessage, the command prints an error and calls os.Exit(1) — a
// production behavior that would kill the whole test binary if invoked
// in-process, so it is exercised via the standard Go re-exec-the-test-binary
// technique: the outer test spawns itself as a subprocess with an env var
// set, the subprocess actually hits the os.Exit(1) branch, and the outer
// test only asserts on the subprocess's exit code and stderr.
func TestAuditListCommand_ErrorExits(t *testing.T) {
	if os.Getenv("SGH_TEST_AUDIT_EXIT_CHILD") == "1" {
		mockServer := testutils.NewMockGitHubServer()
		defer mockServer.Close()
		mockServer.SetResponse("/orgs/testorg/audit-log", testutils.MockResponse{
			StatusCode: http.StatusForbidden,
			Body:       map[string]interface{}{"message": "not allowed"},
		})
		ctx := servicetest.NewMockContext(t, mockServer)

		root := newTestRoot()
		root.AddCommand(NewAuditCommand(ctx))
		root.SetArgs([]string{"audit", "list", "--org", "testorg"})
		root.SetOut(io.Discard)
		_ = root.Execute()
		// os.Exit(1) inside the Run closure should terminate the process
		// before execution ever reaches this point.
		t.Fatal("expected process to have exited by now")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestAuditListCommand_ErrorExits")
	cmd.Env = append(os.Environ(), "SGH_TEST_AUDIT_EXIT_CHILD=1")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected the child process to exit with an error, got %v (stderr: %s)", err, stderr.String())
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("exit code = %d, want 1 (stderr: %s)", exitErr.ExitCode(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "Error:") {
		t.Errorf("expected stderr to contain 'Error:', got %q", stderr.String())
	}
}
