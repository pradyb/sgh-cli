// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package whoami

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/pradyb/sgh-cli/internal/service"
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

func TestNewWhoAmICommand_Metadata(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)

	cmd := NewWhoAmICommand(ctx)

	if cmd.Use != "whoami" {
		t.Errorf("Use = %q, want %q", cmd.Use, "whoami")
	}
	found := false
	for _, alias := range cmd.Aliases {
		if alias == "me" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'me' alias, got %v", cmd.Aliases)
	}
}

func TestWhoAmICommand_TableOutput_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)

	cmd := NewWhoAmICommand(ctx)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "testuser") {
		t.Errorf("expected output to mention testuser, got %q", out)
	}
	if ctx.HasError {
		t.Error("expected HasError to remain false on success")
	}
}

func TestWhoAmICommand_JSONOutput_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)
	ctx.JSON = true

	cmd := NewWhoAmICommand(ctx)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, `"login": "testuser"`) && !strings.Contains(out, `"login":"testuser"`) {
		t.Errorf("expected JSON output to contain testuser login, got %q", out)
	}
}

func TestWhoAmICommand_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/user", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "forbidden"},
	})
	ctx := service.NewMockContext(t, mockServer)

	cmd := NewWhoAmICommand(ctx)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !ctx.HasError {
		t.Error("expected HasError to be set on failure")
	}
	if !strings.Contains(out, "Could not fetch user info") {
		t.Errorf("expected nil-user fallback message, got %q", out)
	}
}

func TestWhoAmICommand_JSONOutput_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/user", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "forbidden"},
	})
	ctx := service.NewMockContext(t, mockServer)
	ctx.JSON = true

	cmd := NewWhoAmICommand(ctx)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !ctx.HasError {
		t.Error("expected HasError to be set on failure")
	}
	if !strings.Contains(out, "null") {
		t.Errorf("expected JSON output of nil user to render null, got %q", out)
	}
}
