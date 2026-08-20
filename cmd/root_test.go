// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	internalconfig "github.com/pradyb/sgh-cli/internal/config"
	"github.com/pradyb/sgh-cli/pkg/context"
)

func newTestContext() *context.Context {
	return &context.Context{Config: &internalconfig.Config{}}
}

func TestValidateOrganizationName(t *testing.T) {
	tests := []struct {
		name    string
		org     string
		wantErr bool
	}{
		{"empty", "", true},
		{"valid simple", "acme", false},
		{"valid with hyphen", "acme-corp", false},
		{"valid with underscore", "acme_corp", false},
		{"invalid leading hyphen", "-acme", true},
		{"invalid space", "acme corp", true},
		{"invalid slash", "acme/corp", true},
		{"too long", strings.Repeat("a", 40), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOrganizationName(tt.org)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateOrganizationName(%q) error = %v, wantErr %v", tt.org, err, tt.wantErr)
			}
		})
	}
}

func TestValidateWorkerCount(t *testing.T) {
	tests := []struct {
		name    string
		workers int
		wantErr bool
	}{
		{"zero", 0, true},
		{"negative", -1, true},
		{"one", 1, false},
		{"typical", 5, false},
		{"max", 50, false},
		{"over max", 51, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWorkerCount(tt.workers)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateWorkerCount(%d) error = %v, wantErr %v", tt.workers, err, tt.wantErr)
			}
		})
	}
}

// captureStderr redirects os.Stderr for the duration of fn and returns what
// was written. The pipe is drained concurrently, not after fn() returns:
// the anonymous pipe's buffer is small, and a write larger than that buffer
// would block forever waiting for a reader that only starts once fn() has
// already returned — a deadlock.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w

	outCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outCh <- buf.String()
	}()

	fn()

	os.Stderr = orig
	w.Close()
	return <-outCh
}

func TestPrintCLIError(t *testing.T) {
	out := captureStderr(t, func() { printCLIError("something failed", "try again") })

	if !strings.Contains(out, "something failed") {
		t.Errorf("output missing message: %q", out)
	}
	if !strings.Contains(out, "try again") {
		t.Errorf("output missing hint: %q", out)
	}
}

func TestPrintCLIError_NoHint(t *testing.T) {
	out := captureStderr(t, func() { printCLIError("boom", "") })

	if !strings.Contains(out, "boom") {
		t.Errorf("output missing message: %q", out)
	}
}

func newFlagsCommand() *cobra.Command {
	c := &cobra.Command{Use: "test"}
	c.Flags().BoolP("verbose", "v", false, "")
	c.Flags().BoolP("log-response", "L", false, "")
	c.Flags().IntP("workers", "w", 5, "")
	c.Flags().Bool("dry-run", false, "")
	c.Flags().Bool("no-color", false, "")
	c.Flags().Int("limit", 0, "")
	c.Flags().StringP("org", "o", "", "")
	c.Flags().StringP("output", "O", "table", "")
	c.Flags().BoolP("compact", "C", false, "")
	c.Flags().BoolP("json", "J", false, "")
	return c
}

func TestSetupContext_Defaults(t *testing.T) {
	ctx := newTestContext()
	ctx.HttpClient = nil // setupContext doesn't touch HttpClient directly except via SwitchToken

	c := newFlagsCommand()
	// SetVerbose/SetLogResponse touch ctx.HttpClient/GraphqlClient, so those must be non-nil.
	realCtx, err := context.Init()
	if err == nil {
		ctx = realCtx
	} else {
		t.Skip("context.Init unavailable without SGH_TOKEN in this environment")
	}

	setupContext(c, ctx)

	if ctx.Verbose {
		t.Error("expected Verbose to remain false by default")
	}
	if ctx.DryRun {
		t.Error("expected DryRun to remain false by default")
	}
	if ctx.Compact || ctx.JSON {
		t.Error("expected neither Compact nor JSON by default")
	}
}

func TestSetupContext_OutputModes(t *testing.T) {
	t.Setenv("SGH_TOKEN", "ghp_1234567890abcdef1234567890abcdef123456")
	ctx, err := context.Init()
	if err != nil {
		t.Fatalf("context.Init: %v", err)
	}

	tests := []struct {
		name        string
		args        []string
		wantCompact bool
		wantJSON    bool
	}{
		{"compact flag wins", []string{"--compact"}, true, false},
		{"json flag wins", []string{"--json"}, false, true},
		{"output=compact string", []string{"--output", "compact"}, true, false},
		{"output=json string", []string{"--output", "json"}, false, true},
		{"output=table is neither", []string{"--output", "table"}, false, false},
		{"compact takes priority over json flag", []string{"--compact", "--json"}, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx.Compact = false
			ctx.JSON = false
			c := newFlagsCommand()
			if err := c.ParseFlags(tt.args); err != nil {
				t.Fatalf("ParseFlags: %v", err)
			}
			setupContext(c, ctx)
			if ctx.Compact != tt.wantCompact {
				t.Errorf("Compact = %v, want %v", ctx.Compact, tt.wantCompact)
			}
			if ctx.JSON != tt.wantJSON {
				t.Errorf("JSON = %v, want %v", ctx.JSON, tt.wantJSON)
			}
		})
	}
}

func TestSetupContext_DryRunAndLimit(t *testing.T) {
	t.Setenv("SGH_TOKEN", "ghp_1234567890abcdef1234567890abcdef123456")
	ctx, err := context.Init()
	if err != nil {
		t.Fatalf("context.Init: %v", err)
	}

	c := newFlagsCommand()
	if err := c.ParseFlags([]string{"--dry-run", "--limit", "42"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	setupContext(c, ctx)

	if !ctx.DryRun {
		t.Error("expected DryRun to be true")
	}
	if ctx.Limit != 42 {
		t.Errorf("Limit = %d, want 42", ctx.Limit)
	}
}

func TestSetupContext_NoColor(t *testing.T) {
	t.Setenv("SGH_TOKEN", "ghp_1234567890abcdef1234567890abcdef123456")
	origNoColor := os.Getenv("NO_COLOR")
	defer os.Setenv("NO_COLOR", origNoColor)
	os.Unsetenv("NO_COLOR")

	ctx, err := context.Init()
	if err != nil {
		t.Fatalf("context.Init: %v", err)
	}

	c := newFlagsCommand()
	if err := c.ParseFlags([]string{"--no-color"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	setupContext(c, ctx)

	if !ctx.NoColor {
		t.Error("expected NoColor to be true")
	}
	if os.Getenv("NO_COLOR") == "" {
		t.Error("expected NO_COLOR env var to be set")
	}
}

func TestSetupContext_WorkerCount(t *testing.T) {
	t.Setenv("SGH_TOKEN", "ghp_1234567890abcdef1234567890abcdef123456")
	ctx, err := context.Init()
	if err != nil {
		t.Fatalf("context.Init: %v", err)
	}

	c := newFlagsCommand()
	if err := c.ParseFlags([]string{"--workers", "12"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	setupContext(c, ctx)

	if ctx.Config.NoOfWorkers != 12 {
		t.Errorf("NoOfWorkers = %d, want 12", ctx.Config.NoOfWorkers)
	}
}

// TestValidateCommandRequirements_ExemptCommands verifies that commands whose
// ancestor chain includes a skip-listed name (or that have no parent / no
// Run+RunE, e.g. group commands) never require --org and therefore never
// call os.Exit. This exercises validateCommandRequirements's non-exiting
// branches directly, in-process.
func TestValidateCommandRequirements_ExemptCommands(t *testing.T) {
	root := &cobra.Command{Use: "sgh"}
	root.Flags().IntP("workers", "w", 5, "")
	root.Flags().StringP("org", "o", "", "")

	// "repo" is in the skip-org list, so its "list" child is exempt.
	repoGroup := &cobra.Command{Use: "repo"}
	repoList := &cobra.Command{Use: "list", Run: func(cmd *cobra.Command, args []string) {}}
	repoList.Flags().IntP("workers", "w", 5, "")
	repoList.Flags().StringP("org", "o", "", "")
	repoGroup.AddCommand(repoList)
	root.AddCommand(repoGroup)

	// A group command with no Run/RunE is exempt regardless of name.
	groupOnly := &cobra.Command{Use: "grouponly"}
	groupOnly.Flags().IntP("workers", "w", 5, "")
	groupOnly.Flags().StringP("org", "o", "", "")
	root.AddCommand(groupOnly)

	tests := []*cobra.Command{root, repoList, groupOnly}
	for _, c := range tests {
		t.Run(c.Name(), func(t *testing.T) {
			// Must not panic or exit; org flag is empty throughout.
			validateCommandRequirements(c, nil)
		})
	}
}

func TestValidateCommandRequirements_ValidOrgPasses(t *testing.T) {
	root := &cobra.Command{Use: "sgh"}
	leaf := &cobra.Command{Use: "list", Run: func(cmd *cobra.Command, args []string) {}}
	leaf.Flags().StringP("org", "o", "", "")
	leaf.Flags().IntP("workers", "w", 5, "")
	root.AddCommand(leaf)
	if err := leaf.Flags().Set("org", "acme-corp"); err != nil {
		t.Fatalf("Set org: %v", err)
	}

	// Must not exit: valid org, valid workers.
	validateCommandRequirements(leaf, nil)
}

func TestValidateCommandRequirements_PositionalOrgArg(t *testing.T) {
	root := &cobra.Command{Use: "sgh"}
	leaf := &cobra.Command{Use: "list", Run: func(cmd *cobra.Command, args []string) {}}
	leaf.Flags().StringP("org", "o", "", "")
	leaf.Flags().IntP("workers", "w", 5, "")
	root.AddCommand(leaf)

	// No --org flag set, but a positional arg is accepted as the org.
	validateCommandRequirements(leaf, []string{"acme-corp"})
}

// The following two tests cover validateCommandRequirements's os.Exit(1)
// branches (missing org, invalid worker count) via the standard Go
// subprocess-recursion technique: re-exec this test binary with an env var
// selecting a small helper that calls the exit path directly, then assert
// on the child's exit code from the parent test.
func TestValidateCommandRequirements_MissingOrgExits(t *testing.T) {
	if os.Getenv("SGH_TEST_MISSING_ORG_SUBPROCESS") == "1" {
		leaf := &cobra.Command{Use: "list", Run: func(cmd *cobra.Command, args []string) {}}
		leaf.Flags().StringP("org", "o", "", "")
		leaf.Flags().IntP("workers", "w", 5, "")
		root := &cobra.Command{Use: "sgh"}
		root.AddCommand(leaf)
		validateCommandRequirements(leaf, nil)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestValidateCommandRequirements_MissingOrgExits")
	cmd.Env = append(os.Environ(), "SGH_TEST_MISSING_ORG_SUBPROCESS=1")
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected subprocess to exit with an error, got %v", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("exit code = %d, want 1", exitErr.ExitCode())
	}
}

func TestValidateCommandRequirements_InvalidWorkersExits(t *testing.T) {
	if os.Getenv("SGH_TEST_INVALID_WORKERS_SUBPROCESS") == "1" {
		leaf := &cobra.Command{Use: "list", Run: func(cmd *cobra.Command, args []string) {}}
		leaf.Flags().StringP("org", "o", "acme", "")
		leaf.Flags().IntP("workers", "w", 500, "")
		root := &cobra.Command{Use: "sgh"}
		root.AddCommand(leaf)
		validateCommandRequirements(leaf, nil)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestValidateCommandRequirements_InvalidWorkersExits")
	cmd.Env = append(os.Environ(), "SGH_TEST_INVALID_WORKERS_SUBPROCESS=1")
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected subprocess to exit with an error, got %v", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("exit code = %d, want 1", exitErr.ExitCode())
	}
}

func TestLogCommandExecution(t *testing.T) {
	c := newFlagsCommand()
	if err := c.ParseFlags([]string{"--verbose", "--workers", "3"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	// Must not panic; output goes to the file logger, nothing to assert on directly.
	logCommandExecution(c)
}

func TestNewRootCommand_Wiring(t *testing.T) {
	t.Setenv("SGH_TOKEN", "ghp_1234567890abcdef1234567890abcdef123456")
	t.Setenv("SGH_ORG", "")
	t.Setenv("SGH_WORKERS", "")
	ctx, err := context.Init()
	if err != nil {
		t.Fatalf("context.Init: %v", err)
	}

	root := NewRootCommand(ctx)

	wantNames := []string{
		"repo", "clone", "commit", "issue",
		"branch", "tag", "pr", "protected-branch",
		"workflow", "post-release",
		"team", "security", "audit", "org",
		"config", "health", "whoami", "version", "tui",
		"shortcuts",
	}
	for _, name := range wantNames {
		found, _, err := root.Find([]string{name})
		if err != nil || found == nil || found == root {
			t.Errorf("expected subcommand %q to be registered", name)
		}
	}

	// A representative shortcut should also be registered directly on root.
	if _, _, err := root.Find([]string{"rpl"}); err != nil {
		t.Errorf("expected shortcut 'rpl' to be registered: %v", err)
	}

	if flag := root.PersistentFlags().Lookup("org"); flag == nil {
		t.Error("expected --org persistent flag")
	}
	if flag := root.PersistentFlags().Lookup("workers"); flag == nil || flag.DefValue != "5" {
		t.Errorf("expected --workers default 5, got %+v", flag)
	}
	if root.PersistentFlags().Lookup("output") == nil {
		t.Error("expected --output persistent flag")
	}
}

func TestNewRootCommand_EnvDefaults(t *testing.T) {
	t.Setenv("SGH_TOKEN", "ghp_1234567890abcdef1234567890abcdef123456")
	t.Setenv("SGH_ORG", "my-default-org")
	t.Setenv("SGH_WORKERS", "17")
	ctx, err := context.Init()
	if err != nil {
		t.Fatalf("context.Init: %v", err)
	}

	root := NewRootCommand(ctx)

	if flag := root.PersistentFlags().Lookup("org"); flag == nil || flag.DefValue != "my-default-org" {
		t.Errorf("expected --org default 'my-default-org', got %+v", flag)
	}
	if flag := root.PersistentFlags().Lookup("workers"); flag == nil || flag.DefValue != "17" {
		t.Errorf("expected --workers default 17, got %+v", flag)
	}
}

func TestNewRootCommand_InvalidWorkersEnvFallsBack(t *testing.T) {
	t.Setenv("SGH_TOKEN", "ghp_1234567890abcdef1234567890abcdef123456")
	t.Setenv("SGH_WORKERS", "not-a-number")
	ctx, err := context.Init()
	if err != nil {
		t.Fatalf("context.Init: %v", err)
	}

	root := NewRootCommand(ctx)

	if flag := root.PersistentFlags().Lookup("workers"); flag == nil || flag.DefValue != "5" {
		t.Errorf("expected --workers default to fall back to 5, got %+v", flag)
	}
}

func TestNewRootCommand_HelpRun(t *testing.T) {
	t.Setenv("SGH_TOKEN", "ghp_1234567890abcdef1234567890abcdef123456")
	ctx, err := context.Init()
	if err != nil {
		t.Fatalf("context.Init: %v", err)
	}

	root := NewRootCommand(ctx)
	root.SetArgs([]string{})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	if err := root.Execute(); err != nil {
		t.Errorf("Execute() with no args returned error: %v", err)
	}
}
