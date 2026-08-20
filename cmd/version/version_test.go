// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package version

import (
	"io"
	"os"
	"strings"
	"testing"
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

func TestNewVersionCommand_Metadata(t *testing.T) {
	cmd := NewVersionCommand()

	if cmd.Use != "version" {
		t.Errorf("Use = %q, want %q", cmd.Use, "version")
	}
	if cmd.Run == nil {
		t.Fatal("expected Run to be set")
	}
	shortFlag := cmd.Flags().Lookup("short")
	if shortFlag == nil {
		t.Fatal("expected a --short flag to be registered")
	}
	if shortFlag.Shorthand != "s" {
		t.Errorf("short flag shorthand = %q, want %q", shortFlag.Shorthand, "s")
	}
}

func TestVersionCommand_Short(t *testing.T) {
	cmd := NewVersionCommand()
	cmd.SetArgs([]string{"--short"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	got := strings.TrimSpace(out)
	if got != Version {
		t.Errorf("output = %q, want %q", got, Version)
	}
}

func TestVersionCommand_Full(t *testing.T) {
	cmd := NewVersionCommand()
	cmd.SetArgs([]string{})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"sgh-cli", "Version", Version, "Commit SHA", CommitSHA, "Build Date", BuildDate, "Go Version", "Platform"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q does not contain %q", out, want)
		}
	}
}

func TestDisplayVersion(t *testing.T) {
	out := captureStdout(t, displayVersion)

	if !strings.Contains(out, "sgh-cli") {
		t.Errorf("expected displayVersion output to mention sgh-cli, got %q", out)
	}
}
