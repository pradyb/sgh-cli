// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package postrelease

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
// persistent flags the post-release command reads (org, dry-run), with no
// PersistentPreRun/PersistentPostRun — unlike the real root command, which
// os.Exit(1)s on flag validation issues or ctx.HasError, making it unsafe to
// drive error-path tests through.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().StringP("org", "o", "", "")
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

func TestPostReleaseCommand_BranchAndTag_Success(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/testorg/repo1/git/ref/heads/main", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"object": map[string]interface{}{"sha": "refsha123", "type": "commit"}},
	})
	mockServer.SetResponse("/repos/testorg/repo1/git/tags", testutils.MockResponse{
		StatusCode: http.StatusCreated,
		Body:       map[string]interface{}{"sha": "tagcommitsha"},
	})

	root := newTestRoot()
	root.AddCommand(NewPostReleaseCommand(ctx))
	root.SetArgs([]string{
		"post-release", "--org", "testorg", "-r", "repo1",
		"--ref", "main", "--branch", "hotfix-1", "--tag", "v1.2.3", "--message", "Hotfix",
	})
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

func TestPostReleaseCommand_BranchOnly_Success(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/testorg/repo1/git/ref/heads/main", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"object": map[string]interface{}{"sha": "refsha123", "type": "commit"}},
	})

	root := newTestRoot()
	root.AddCommand(NewPostReleaseCommand(ctx))
	root.SetArgs([]string{
		"post-release", "--org", "testorg", "-r", "repo1",
		"--ref", "main", "--branch", "hotfix-1",
	})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestPostReleaseCommand_Error(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/testorg/repo1/git/ref/heads/main", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	root := newTestRoot()
	root.AddCommand(NewPostReleaseCommand(ctx))
	root.SetArgs([]string{
		"post-release", "--org", "testorg", "-r", "repo1",
		"--ref", "main", "--branch", "hotfix-1",
	})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "repo1") {
		t.Errorf("expected output to mention repo1 even on error, got: %s", out)
	}
}

func TestPostReleaseCommand_DryRun(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.DryRun = true

	root := newTestRoot()
	root.AddCommand(NewPostReleaseCommand(ctx))
	root.SetArgs([]string{
		"post-release", "--org", "testorg", "-r", "repo1",
		"--ref", "main", "--branch", "hotfix-1", "--tag", "v1.2.3", "--message", "Hotfix",
	})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Dry Run") && !strings.Contains(out, "DRY RUN") && !strings.Contains(out, "dry") {
		t.Errorf("expected dry-run banner in output, got: %s", out)
	}

	// Dry-run must not have made any repository-mutating calls.
	for _, req := range mockServer.GetRequests() {
		if req.Method == http.MethodPost || req.Method == http.MethodPut {
			t.Errorf("dry-run should not issue mutating requests, got %s %s", req.Method, req.Path)
		}
	}
}

func TestPostReleaseCommand_DryRun_ExcludeRepos(t *testing.T) {
	ctx, _ := newMockedContext(t)
	ctx.DryRun = true

	root := newTestRoot()
	root.AddCommand(NewPostReleaseCommand(ctx))
	root.SetArgs([]string{
		"post-release", "--org", "testorg", "-r", "repo1", "-r", "repo2", "-e", "repo2",
		"--ref", "main", "--tag", "v1.2.3",
	})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if strings.Contains(out, "repo2") {
		t.Errorf("expected repo2 to be excluded from dry-run output, got: %s", out)
	}
}

func TestPostReleaseCommand_RequiredFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing --ref",
			args: []string{"post-release", "--org", "testorg", "--branch", "hotfix-1"},
		},
		{
			name: "missing both --branch and --tag",
			args: []string{"post-release", "--org", "testorg", "--ref", "main"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := newMockedContext(t)

			root := newTestRoot()
			root.AddCommand(NewPostReleaseCommand(ctx))
			root.SetArgs(tt.args)
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)

			if err := root.Execute(); err == nil {
				t.Fatal("expected an error for missing required flags")
			}
		})
	}
}
