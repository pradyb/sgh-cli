// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// captureStdout redirects os.Stdout for the duration of fn and returns what
// was written. The pipe is drained concurrently, not after fn() returns:
// the anonymous pipe's buffer is small, and a write larger than that buffer
// would block forever waiting for a reader that only starts once fn() has
// already returned — a deadlock.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w

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

func TestShortcutDefs_AllGroupsKnown(t *testing.T) {
	ctx := newTestContext()
	known := make(map[string]bool, len(shortcutGroups))
	for _, g := range shortcutGroups {
		known[g] = true
	}
	for _, s := range shortcutDefs(ctx) {
		if !known[s.group] {
			t.Errorf("shortcut %q has group %q which is not listed in shortcutGroups", s.name, s.group)
		}
		if s.builder == nil {
			t.Errorf("shortcut %q has a nil builder", s.name)
		}
	}
}

func TestShortcutDefs_UniqueNames(t *testing.T) {
	ctx := newTestContext()
	seen := make(map[string]bool)
	for _, s := range shortcutDefs(ctx) {
		if seen[s.name] {
			t.Errorf("duplicate shortcut name %q", s.name)
		}
		seen[s.name] = true
	}
}

func TestShortcutDefs_BuildersProduceValidCommands(t *testing.T) {
	ctx := newTestContext()
	for _, s := range shortcutDefs(ctx) {
		t.Run(s.name, func(t *testing.T) {
			cmd := s.builder(ctx)
			if cmd == nil {
				t.Fatalf("builder for %q returned nil command", s.name)
			}
		})
	}
}

func TestRegisterShortcuts(t *testing.T) {
	ctx := newTestContext()
	root := &cobra.Command{Use: "sgh"}
	registerShortcuts(root, ctx)

	defs := shortcutDefs(ctx)
	if len(root.Commands()) != len(defs) {
		t.Fatalf("registered %d commands, want %d", len(root.Commands()), len(defs))
	}

	for _, s := range defs {
		found, _, err := root.Find([]string{s.name})
		if err != nil || found == nil || found == root {
			t.Errorf("expected shortcut %q to be registered", s.name)
			continue
		}
		if found.Use != s.name {
			t.Errorf("shortcut %q: Use = %q, want %q", s.name, found.Use, s.name)
		}
		if found.GroupID != "shortcuts" {
			t.Errorf("shortcut %q: GroupID = %q, want %q", s.name, found.GroupID, "shortcuts")
		}
		wantShort := "→ " + s.expands
		if found.Short != wantShort {
			t.Errorf("shortcut %q: Short = %q, want %q", s.name, found.Short, wantShort)
		}
	}
}

func TestNewShortcutsHelpCommand(t *testing.T) {
	ctx := newTestContext()
	cmd := newShortcutsHelpCommand(ctx)

	if cmd.Use != "shortcuts" {
		t.Errorf("Use = %q, want %q", cmd.Use, "shortcuts")
	}

	out := captureStdout(t, func() { cmd.Run(cmd, nil) })

	if !strings.Contains(out, "Available Shortcuts") {
		t.Errorf("output missing header, got: %s", out)
	}
	// Spot-check a couple of shortcuts from different groups render.
	for _, want := range []string{"rpl", "repo list", "prl", "pr list", "Usage: sgh <shortcut>"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q, got: %s", want, out)
		}
	}
}

func TestNewShortcutsHelpCommand_AllGroupsRender(t *testing.T) {
	ctx := newTestContext()
	cmd := newShortcutsHelpCommand(ctx)

	out := captureStdout(t, func() { cmd.Run(cmd, nil) })

	grouped := make(map[string]bool)
	for _, s := range shortcutDefs(ctx) {
		grouped[s.group] = true
	}
	for _, g := range shortcutGroups {
		if !grouped[g] {
			continue // group with no shortcuts is skipped by the render loop
		}
		if !strings.Contains(out, g) {
			t.Errorf("expected group %q to render in output", g)
		}
	}
}

// Ensures the whoami shortcut's inline closure builder (the one shortcut
// whose builder isn't a bare function reference) is exercised directly.
func TestShortcutDefs_WhoAmIBuilder(t *testing.T) {
	ctx := newTestContext()
	var found *shortcut
	for i, s := range shortcutDefs(ctx) {
		if s.name == "wai" {
			found = &shortcutDefs(ctx)[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected a 'wai' shortcut to be defined")
	}
	cmd := found.builder(ctx)
	if cmd == nil {
		t.Fatal("whoami shortcut builder returned nil")
	}
}
