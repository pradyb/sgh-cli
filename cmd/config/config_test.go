// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package config

import (
	"bytes"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	internalconfig "github.com/pradyb/sgh-cli/internal/config"
	"github.com/pradyb/sgh-cli/pkg/context"
)

// isolateHome points the OS home directory lookup at a fresh temp dir so
// Save() never touches the real user's sgh config.
func isolateHome(t *testing.T) {
	t.Helper()
	tempDir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", tempDir)
	} else {
		t.Setenv("HOME", tempDir)
	}
}

func newTestContext(t *testing.T) *context.Context {
	t.Helper()
	isolateHome(t)
	return &context.Context{Config: &internalconfig.Config{}}
}

// newTestRoot builds a minimal fake parent command that only defines the
// persistent flags the config subcommands actually read. It intentionally
// has no PersistentPreRun/PersistentPostRun (unlike the real cmd/root.go), so
// it never calls os.Exit on an error path during tests.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().StringP("org", "o", "", "")
	root.PersistentFlags().BoolP("json", "J", false, "")
	root.PersistentFlags().Bool("dry-run", false, "")
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

// captureOutput captures anything written to os.Stdout or os.Stderr during fn
// (some error paths, e.g. ui.PrintCLIError, write to stderr instead of stdout).
// Both pipes are drained concurrently for the same reason captureStdout's is:
// the anonymous pipe's buffer is small enough that a table-heavy Run can fill
// it before fn() returns, which would deadlock a post-hoc read.
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = wOut, wErr

	outCh := make(chan string, 1)
	errCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, rOut)
		outCh <- buf.String()
	}()
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, rErr)
		errCh <- buf.String()
	}()

	fn()

	os.Stdout, os.Stderr = origOut, origErr
	wOut.Close()
	wErr.Close()
	return <-outCh + <-errCh
}

func TestNewConfigCommand_Structure(t *testing.T) {
	ctx := newTestContext(t)
	cmd := NewConfigCommand(ctx)
	want := map[string]bool{"list": false, "validate": false, "add": false, "remove": false, "set": false, "reset": false}
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

// ---------------------------------------------------------------------------
// list
// ---------------------------------------------------------------------------

func TestListCommand_NoOrgs(t *testing.T) {
	ctx := newTestContext(t)

	out := captureStdout(t, func() {
		if err := execCmd(listCommand(ctx), "list"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "No organizations configured") {
		t.Errorf("expected no-data message, got: %s", out)
	}
}

func TestListCommand_WithOrgs(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")
	ctx.Config.AddRepositoryPattern("acme", true, false, "^api-")

	out := captureStdout(t, func() {
		if err := execCmd(listCommand(ctx), "list"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "acme") {
		t.Errorf("expected output to contain org name, got: %s", out)
	}
}

func TestListCommand_JSONOutput(t *testing.T) {
	ctx := newTestContext(t)
	ctx.JSON = true
	ctx.Config.AddOrganization("acme")

	out := captureStdout(t, func() {
		if err := execCmd(listCommand(ctx), "list"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "acme") {
		t.Errorf("expected JSON output to contain org name, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// validate
// ---------------------------------------------------------------------------

func TestValidateCommand_Valid(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")
	ctx.Config.AddRepositoryPattern("acme", true, false, "^api-")

	out := captureStdout(t, func() {
		if err := execCmd(validateCommand(ctx), "validate"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Configuration is valid") {
		t.Errorf("expected valid message, got: %s", out)
	}
	if ctx.HasError {
		t.Error("did not expect ctx.HasError to be set for a valid config")
	}
}

func TestValidateCommand_NoPatterns(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")

	out := captureStdout(t, func() {
		if err := execCmd(validateCommand(ctx), "validate"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "all repos selected") {
		t.Errorf("expected 'all repos selected' hint, got: %s", out)
	}
}

func TestValidateCommand_InvalidEmail(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")
	// SetTaggerEmail does not validate format itself — only `config validate` does.
	ctx.Config.SetTaggerEmail("acme", "not-an-email")

	out := captureStdout(t, func() {
		if err := execCmd(validateCommand(ctx), "validate"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Configuration has errors") {
		t.Errorf("expected error message, got: %s", out)
	}
	if !ctx.HasError {
		t.Error("expected ctx.HasError to be set for an invalid config")
	}
}

// ---------------------------------------------------------------------------
// add
// ---------------------------------------------------------------------------

func TestAddCommand_Org(t *testing.T) {
	ctx := newTestContext(t)

	if err := execCmd(addCommand(ctx), "add", "org", "acme"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.Config.IsOrganizationPresent("acme") {
		t.Error("expected organization to be added")
	}
}

func TestAddCommand_Repo(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")

	if err := execCmd(addCommand(ctx), "add", "repo", "widgets", "--org", "acme"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.Config.IsRepositoryPresent("acme", "widgets") {
		t.Error("expected repository to be added")
	}
}

func TestAddCommand_Repo_MissingOrg(t *testing.T) {
	ctx := newTestContext(t)

	if err := execCmd(addCommand(ctx), "add", "repo", "widgets"); err == nil {
		t.Fatal("expected an error when --org is missing for repo key")
	}
}

func TestAddCommand_PatternInclude(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")

	err := execCmd(addCommand(ctx), "add", "pattern", "^api-", "--org", "acme", "--include")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.Config.IsRepositoryPatternPresent("acme", "^api-", true) {
		t.Error("expected include pattern to be added")
	}
}

func TestAddCommand_PatternExclude(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")

	err := execCmd(addCommand(ctx), "add", "pattern", "service-legacy$", "--org", "acme", "--exclude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.Config.IsRepositoryPatternPresent("acme", "service-legacy$", false) {
		t.Error("expected exclude pattern to be added")
	}
}

func TestAddCommand_PatternMissingIncludeExclude(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")

	out := captureOutput(t, func() {
		err := execCmd(addCommand(ctx), "add", "pattern", "^api-", "--org", "acme")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Must specify --include") {
		t.Errorf("expected include/exclude required message, got: %s", out)
	}
}

func TestAddCommand_PatternInvalid(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")

	out := captureOutput(t, func() {
		err := execCmd(addCommand(ctx), "add", "pattern", ".*", "--org", "acme", "--include")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Invalid pattern") {
		t.Errorf("expected invalid pattern message, got: %s", out)
	}
	if ctx.Config.IsRepositoryPatternPresent("acme", ".*", true) {
		t.Error("did not expect catch-all pattern to be added")
	}
}

func TestAddCommand_PRAssignee(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")

	err := execCmd(addCommand(ctx), "add", "pr-assignee", "jane-doe", "--org", "acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.Config.IsPullRequestAssigneePresent("acme", "jane-doe") {
		t.Error("expected assignee to be added")
	}
}

func TestAddCommand_UnknownKey(t *testing.T) {
	ctx := newTestContext(t)

	out := captureOutput(t, func() {
		if err := execCmd(addCommand(ctx), "add", "bogus", "value"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Unknown key") {
		t.Errorf("expected unknown key message, got: %s", out)
	}
}

func TestAddCommand_WrongArgCount(t *testing.T) {
	ctx := newTestContext(t)

	if err := execCmd(addCommand(ctx), "add", "org"); err == nil {
		t.Fatal("expected an error for wrong argument count")
	}
}

// ---------------------------------------------------------------------------
// remove
// ---------------------------------------------------------------------------

func TestRemoveCommand_Org(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")

	if err := execCmd(removeCommand(ctx), "remove", "org", "acme"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Config.IsOrganizationPresent("acme") {
		t.Error("expected organization to be removed")
	}
}

func TestRemoveCommand_Repo(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")
	ctx.Config.AddRepository("acme", "widgets")

	err := execCmd(removeCommand(ctx), "remove", "repo", "widgets", "--org", "acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Config.IsRepositoryPresent("acme", "widgets") {
		t.Error("expected repository to be removed")
	}
}

func TestRemoveCommand_Pattern(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")
	ctx.Config.AddRepositoryPattern("acme", true, false, "^api-")

	err := execCmd(removeCommand(ctx), "remove", "pattern", "^api-", "--org", "acme", "--include")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Config.IsRepositoryPatternPresent("acme", "^api-", true) {
		t.Error("expected pattern to be removed")
	}
}

func TestRemoveCommand_Pattern_MissingIncludeExclude(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")

	if err := execCmd(removeCommand(ctx), "remove", "pattern", "^api-", "--org", "acme"); err == nil {
		t.Fatal("expected an error when neither --include nor --exclude is set")
	}
}

func TestRemoveCommand_PRAssignee(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")
	ctx.Config.AddPullRequestAssignee("acme", "jane-doe")

	err := execCmd(removeCommand(ctx), "remove", "pr-assignee", "jane-doe", "--org", "acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Config.IsPullRequestAssigneePresent("acme", "jane-doe") {
		t.Error("expected assignee to be removed")
	}
}

func TestRemoveCommand_UnknownKey(t *testing.T) {
	ctx := newTestContext(t)

	out := captureOutput(t, func() {
		if err := execCmd(removeCommand(ctx), "remove", "bogus", "value"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Unknown key") {
		t.Errorf("expected unknown key message, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// set
// ---------------------------------------------------------------------------

func TestSetCommand_Token(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")

	err := execCmd(setCommand(ctx), "set", "token", "ghp_1234567890abcdef1234567890abcdef1234", "--org", "acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := ctx.Config.TokenForOwner("acme"); got == "" {
		t.Error("expected token to be set")
	}
}

func TestSetCommand_Token_Invalid(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")

	out := captureOutput(t, func() {
		err := execCmd(setCommand(ctx), "set", "token", "not-a-token", "--org", "acme")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Invalid token") {
		t.Errorf("expected invalid token message, got: %s", out)
	}
	if got := ctx.Config.TokenForOwner("acme"); got != "" {
		t.Error("did not expect token to be set")
	}
}

func TestSetCommand_OwnerType(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")

	err := execCmd(setCommand(ctx), "set", "owner-type", "User", "--org", "acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := ctx.Config.OwnerTypeFor("acme"); got != "User" {
		t.Errorf("OwnerTypeFor() = %q, want User", got)
	}
}

func TestSetCommand_OwnerType_Invalid(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")

	if err := execCmd(setCommand(ctx), "set", "owner-type", "bogus", "--org", "acme"); err == nil {
		t.Fatal("expected an error for invalid owner-type value")
	}
}

func TestSetCommand_TaggerName(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")

	err := execCmd(setCommand(ctx), "set", "tagger-name", "Jane Doe", "--org", "acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := ctx.Config.TaggerName("acme"); got != "Jane Doe" {
		t.Errorf("TaggerName() = %q, want Jane Doe", got)
	}
}

func TestSetCommand_TaggerEmail(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")

	err := execCmd(setCommand(ctx), "set", "tagger-email", "jane@example.com", "--org", "acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := ctx.Config.TaggerEmail("acme"); got != "jane@example.com" {
		t.Errorf("TaggerEmail() = %q, want jane@example.com", got)
	}
}

func TestSetCommand_MissingOrg(t *testing.T) {
	ctx := newTestContext(t)

	if err := execCmd(setCommand(ctx), "set", "tagger-name", "Jane Doe"); err == nil {
		t.Fatal("expected an error when --org is missing")
	}
}

func TestSetCommand_UnknownKey(t *testing.T) {
	ctx := newTestContext(t)

	out := captureOutput(t, func() {
		if err := execCmd(setCommand(ctx), "set", "bogus", "value"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Unknown key") {
		t.Errorf("expected unknown key message, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// reset
// ---------------------------------------------------------------------------

func TestResetCommand_All(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")
	ctx.Config.AddOrganization("beta")

	out := captureStdout(t, func() {
		if err := execCmd(resetCommand(ctx), "reset", "--yes"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "reset successfully") {
		t.Errorf("expected reset success message, got: %s", out)
	}
	if len(ctx.Config.OrganizationNames()) != 0 {
		t.Error("expected all organizations to be removed")
	}
}

func TestResetCommand_SingleOrg(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")
	ctx.Config.AddOrganization("beta")

	out := captureStdout(t, func() {
		err := execCmd(resetCommand(ctx), "reset", "--org", "acme", "--yes")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "removed from config") {
		t.Errorf("expected removal message, got: %s", out)
	}
	if ctx.Config.IsOrganizationPresent("acme") {
		t.Error("expected acme to be removed")
	}
	if !ctx.Config.IsOrganizationPresent("beta") {
		t.Error("expected beta to remain")
	}
}

func TestResetCommand_SingleOrg_NotFound(t *testing.T) {
	ctx := newTestContext(t)

	out := captureOutput(t, func() {
		err := execCmd(resetCommand(ctx), "reset", "--org", "nope", "--yes")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "not found in config") {
		t.Errorf("expected not-found message, got: %s", out)
	}
}

// withStdin redirects os.Stdin to a pipe pre-loaded with input, for the
// resetCommand confirmation prompts (fmt.Scanln), and restores it after fn.
func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	w.Close()

	orig := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = orig }()

	fn()
}

func TestResetCommand_All_ConfirmedInteractively(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")

	out := captureStdout(t, func() {
		withStdin(t, "yes\n", func() {
			if err := execCmd(resetCommand(ctx), "reset"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	})
	if !strings.Contains(out, "reset successfully") {
		t.Errorf("expected reset success message, got: %s", out)
	}
	if len(ctx.Config.OrganizationNames()) != 0 {
		t.Error("expected all organizations to be removed")
	}
}

func TestResetCommand_All_AbortedInteractively(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")

	out := captureStdout(t, func() {
		withStdin(t, "no\n", func() {
			if err := execCmd(resetCommand(ctx), "reset"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	})
	if !strings.Contains(out, "Aborted") {
		t.Errorf("expected abort message, got: %s", out)
	}
	if !ctx.Config.IsOrganizationPresent("acme") {
		t.Error("expected acme to remain after an aborted reset")
	}
}

func TestResetCommand_SingleOrg_ConfirmedInteractively(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")

	out := captureStdout(t, func() {
		withStdin(t, "acme\n", func() {
			if err := execCmd(resetCommand(ctx), "reset", "--org", "acme"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	})
	if !strings.Contains(out, "removed from config") {
		t.Errorf("expected removal message, got: %s", out)
	}
	if ctx.Config.IsOrganizationPresent("acme") {
		t.Error("expected acme to be removed")
	}
}

func TestResetCommand_SingleOrg_AbortedInteractively(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")

	out := captureStdout(t, func() {
		withStdin(t, "wrong-name\n", func() {
			if err := execCmd(resetCommand(ctx), "reset", "--org", "acme"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	})
	if !strings.Contains(out, "Aborted") {
		t.Errorf("expected abort message, got: %s", out)
	}
	if !ctx.Config.IsOrganizationPresent("acme") {
		t.Error("expected acme to remain after an aborted reset")
	}
}

func TestResetCommand_ForceAlias(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Config.AddOrganization("acme")

	err := execCmd(resetCommand(ctx), "reset", "--force")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ctx.Config.OrganizationNames()) != 0 {
		t.Error("expected all organizations to be removed via --force alias")
	}
}

// ---------------------------------------------------------------------------
// helper functions
// ---------------------------------------------------------------------------

func TestMaskToken(t *testing.T) {
	tests := []struct {
		name string
		tok  string
		want string
	}{
		{"short token", "abc123", "***"},
		{"exactly eight chars", "12345678", "***"},
		{"long token", "ghp_1234567890abcdef", "ghp_***cdef"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maskToken(tt.tok); got != tt.want {
				t.Errorf("maskToken(%q) = %q, want %q", tt.tok, got, tt.want)
			}
		})
	}
}
