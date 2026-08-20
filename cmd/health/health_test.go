// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package health

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
	"github.com/pradyb/sgh-cli/pkg/context"
)

// newTestRoot builds a minimal fake parent command with no
// PersistentPreRun/PersistentPostRun (unlike the real cmd/root.go, which
// os.Exit(1)s on flag validation issues), so it never kills the test
// process on an error path.
func newTestRoot() *cobra.Command {
	return &cobra.Command{Use: "root"}
}

func execCmd(cmd *cobra.Command, args ...string) error {
	root := newTestRoot()
	root.AddCommand(cmd)
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	return root.Execute()
}

// captureStdout redirects os.Stdout to a pipe for the duration of fn and
// returns everything written to it. The pipe is drained concurrently on a
// background goroutine — on Windows the anonymous pipe buffer is only a few
// KB, and a large enough Run could deadlock if draining only started after
// fn() returns.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
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

func newMockedContext(t *testing.T) *context.Context {
	t.Helper()
	mockServer := testutils.NewMockGitHubServer()
	t.Cleanup(mockServer.Close)
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true
	return ctx
}

// hasInternet reports whether the sandbox can reach the real GitHub API.
// checkGitHubAPIConnectivity, checkAuthentication, and
// checkNetworkConnectivity all hit hardcoded real hosts (api.github.com,
// www.google.com) rather than going through ctx's mockable base URL, so
// tests that exercise those paths are skipped when there's no route out.
func hasInternet() bool {
	conn, err := net.DialTimeout("tcp", "api.github.com:443", 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func requireInternet(t *testing.T) {
	t.Helper()
	if !hasInternet() {
		t.Skip("no route to api.github.com; skipping network-dependent health check test")
	}
}

// ---------------------------------------------------------------------------
// command structure
// ---------------------------------------------------------------------------

func TestNewHealthCommand_Structure(t *testing.T) {
	ctx := newMockedContext(t)
	cmd := NewHealthCommand(ctx)

	if cmd.Use != "health" {
		t.Errorf("Use = %q, want health", cmd.Use)
	}
	if cmd.Flags().Lookup("json") == nil {
		t.Error("expected --json flag to be registered")
	}
}

// ---------------------------------------------------------------------------
// pure / deterministic sub-checks (no network involved)
// ---------------------------------------------------------------------------

func TestCheckAuthentication_NoToken(t *testing.T) {
	t.Setenv("SGH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")

	err := checkAuthentication(&context.Context{})
	if err == nil {
		t.Fatal("expected an error when no token is configured")
	}
	if !strings.Contains(err.Error(), "SGH_TOKEN") {
		t.Errorf("error = %q, want it to mention SGH_TOKEN", err.Error())
	}
}

func TestCheckConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		ctx     *context.Context
		wantErr bool
	}{
		{
			name:    "nil config",
			ctx:     &context.Context{Config: nil},
			wantErr: true,
		},
		{
			name:    "empty organizations",
			ctx:     newMockedContext(t),
			wantErr: false,
		},
		{
			name: "with organizations",
			ctx: func() *context.Context {
				c := newMockedContext(t)
				c.Config.AddOrganization("acme")
				return c
			}(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkConfiguration(tt.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkConfiguration() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckRateLimitStatus_NoInfo(t *testing.T) {
	ctx := newMockedContext(t)

	if err := checkRateLimitStatus(ctx); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// network-dependent checks and the top-level Run paths
// ---------------------------------------------------------------------------

func TestRunHealthCheckJSON_Structure(t *testing.T) {
	requireInternet(t)
	ctx := newMockedContext(t)

	out := captureStdout(t, func() {
		runHealthCheckJSON(ctx)
	})

	var report healthReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("failed to unmarshal JSON report: %v\noutput: %s", err, out)
	}
	if report.Total != 5 {
		t.Errorf("Total = %d, want 5", report.Total)
	}
	if len(report.Checks) != 5 {
		t.Errorf("len(Checks) = %d, want 5", len(report.Checks))
	}
	if report.Healthy != (report.Passed == report.Total) {
		t.Errorf("Healthy = %v, inconsistent with Passed=%d/Total=%d", report.Healthy, report.Passed, report.Total)
	}

	// The mock context's SGH_TOKEN is a syntactically valid but unrecognized
	// token, so the real GitHub API should reject it — a deterministic way to
	// exercise the "fail" branch of runHealthCheckJSON without depending on
	// unpredictable rate-limit or configuration state.
	var sawAuthFail bool
	for _, c := range report.Checks {
		if c.Name == "Authentication" && c.Status == "fail" {
			sawAuthFail = true
		}
	}
	if !sawAuthFail {
		t.Errorf("expected Authentication check to fail with a fake token, report: %+v", report)
	}
}

func TestRunHealthCheck_TextOutput(t *testing.T) {
	requireInternet(t)
	ctx := newMockedContext(t)

	out := captureStdout(t, func() {
		runHealthCheck(ctx)
	})

	if !strings.Contains(out, "sgh-cli health check") {
		t.Errorf("expected header in output, got: %s", out)
	}
	if !strings.Contains(out, "checks passed") {
		t.Errorf("expected summary line in output, got: %s", out)
	}
}

func TestNewHealthCommand_JSONFlag_Execution(t *testing.T) {
	requireInternet(t)
	ctx := newMockedContext(t)

	out := captureStdout(t, func() {
		if err := execCmd(NewHealthCommand(ctx), "health", "--json"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "\"checks\"") {
		t.Errorf("expected JSON report output, got: %s", out)
	}
}

func TestNewHealthCommand_DefaultExecution(t *testing.T) {
	requireInternet(t)
	ctx := newMockedContext(t)

	out := captureStdout(t, func() {
		if err := execCmd(NewHealthCommand(ctx), "health"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "sgh-cli health check") {
		t.Errorf("expected text report output, got: %s", out)
	}
}
