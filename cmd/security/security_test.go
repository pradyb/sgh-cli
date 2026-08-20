// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package security

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

// newTestRoot builds a minimal fake parent command that only defines the
// persistent flags the security subcommands actually read. It intentionally
// has no PersistentPreRun/PersistentPostRun (unlike the real cmd/root.go), so
// it never calls os.Exit on an error path during tests.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().StringP("org", "o", "", "")
	root.PersistentFlags().BoolP("verbose", "v", false, "")
	root.PersistentFlags().BoolP("log-response", "L", false, "")
	root.PersistentFlags().IntP("workers", "w", 5, "")
	root.PersistentFlags().StringP("output", "O", "table", "")
	root.PersistentFlags().BoolP("compact", "C", false, "")
	root.PersistentFlags().BoolP("json", "J", false, "")
	root.PersistentFlags().Bool("dry-run", false, "")
	root.PersistentFlags().Bool("no-color", false, "")
	root.PersistentFlags().Int("limit", 0, "")
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

func newMockedContext(t *testing.T) (*context.Context, *testutils.MockGitHubServer) {
	t.Helper()
	mockServer := testutils.NewMockGitHubServer()
	t.Cleanup(mockServer.Close)
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true
	return ctx, mockServer
}

func alertsBody() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"number":                   1,
			"secret_type":              "github_pat",
			"secret_type_display_name": "GitHub Personal Access Token",
			"state":                    "open",
			"created_at":               "2024-01-01T00:00:00Z",
			"updated_at":               "2024-01-01T00:00:00Z",
			"html_url":                 "https://github.com/acme/repo1/security/secret-scanning/1",
			"location":                 map[string]interface{}{"path": "config.yml", "start_line": 3, "end_line": 3},
		},
		{
			"number":                   2,
			"secret_type":              "aws_access_key_id",
			"secret_type_display_name": "AWS Access Key ID",
			"state":                    "resolved",
			"resolution":               "false_positive",
			"created_at":               "2024-02-01T00:00:00Z",
			"updated_at":               "2024-02-02T00:00:00Z",
			"html_url":                 "https://github.com/acme/repo1/security/secret-scanning/2",
			"location":                 map[string]interface{}{"path": "secrets.env", "start_line": 1, "end_line": 1},
		},
	}
}

func TestNewSecurityCommand_Structure(t *testing.T) {
	ctx, _ := newMockedContext(t)
	cmd := NewSecurityCommand(ctx)
	want := map[string]bool{"list": false, "view": false, "update": false}
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

func TestListAlertsCommand_Success(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/acme/repo1/secret-scanning/alerts", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       alertsBody(),
	})

	out := captureStdout(t, func() {
		err := execCmd(ListAlertsCommand(ctx), "list", "--org", "acme", "-r", "repo1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "github_pat") && !strings.Contains(out, "GitHub Personal Access Token") {
		t.Errorf("expected output to mention the secret type, got: %s", out)
	}
}

func TestListAlertsCommand_StateFilter(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/acme/repo1/secret-scanning/alerts", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       alertsBody(),
	})

	err := execCmd(ListAlertsCommand(ctx), "list", "--org", "acme", "-r", "repo1", "--state", "open", "--sort", "state")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListAlertsCommand_InvalidState(t *testing.T) {
	ctx, mockServer := newMockedContext(t)

	err := execCmd(ListAlertsCommand(ctx), "list", "--org", "acme", "-r", "repo1", "--state", "bogus")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests for invalid state, got %d", len(mockServer.GetRequests()))
	}
}

func TestListAlertsCommand_JSONOutput(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.JSON = true
	mockServer.SetResponse("/repos/acme/repo1/secret-scanning/alerts", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       alertsBody(),
	})

	out := captureStdout(t, func() {
		err := execCmd(ListAlertsCommand(ctx), "list", "--org", "acme", "-r", "repo1", "--secret-type", "github_pat")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "[") {
		t.Errorf("expected JSON array output, got: %s", out)
	}
}

func TestListAlertsCommand_LimitTruncates(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.Limit = 1
	mockServer.SetResponse("/repos/acme/repo1/secret-scanning/alerts", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       alertsBody(),
	})

	err := execCmd(ListAlertsCommand(ctx), "list", "--org", "acme", "-r", "repo1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListAlertsCommand_Error(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/acme/repo1/secret-scanning/alerts", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	err := execCmd(ListAlertsCommand(ctx), "list", "--org", "acme", "-r", "repo1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// view
// ---------------------------------------------------------------------------

func TestViewAlertCommand_Success(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/acme/repo1/secret-scanning/alerts/1", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"number":      1,
			"secret_type": "github_pat",
			"state":       "open",
			"html_url":    "https://github.com/acme/repo1/security/secret-scanning/1",
		},
	})

	err := execCmd(ViewAlertCommand(ctx), "view", "--org", "acme", "-r", "repo1", "--alert", "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViewAlertCommand_JSONOutput(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.JSON = true
	mockServer.SetResponse("/repos/acme/repo1/secret-scanning/alerts/1", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"number": 1, "secret_type": "github_pat", "state": "open"},
	})

	out := captureStdout(t, func() {
		err := execCmd(ViewAlertCommand(ctx), "view", "--org", "acme", "-r", "repo1", "--alert", "1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "github_pat") {
		t.Errorf("expected JSON output to mention secret type, got: %s", out)
	}
}

func TestViewAlertCommand_Error(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/acme/repo1/secret-scanning/alerts/99", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	err := execCmd(ViewAlertCommand(ctx), "view", "--org", "acme", "-r", "repo1", "--alert", "99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViewAlertCommand_MissingRequiredFlags(t *testing.T) {
	ctx, _ := newMockedContext(t)

	if err := execCmd(ViewAlertCommand(ctx), "view", "--org", "acme"); err == nil {
		t.Fatal("expected an error for missing required --repository/--alert flags")
	}
}

// ---------------------------------------------------------------------------
// update
// ---------------------------------------------------------------------------

func TestUpdateAlertCommand_Success(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/acme/repo1/secret-scanning/alerts/1", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"number":             1,
			"state":              "resolved",
			"resolution":         "false_positive",
			"resolution_comment": "not a real secret",
		},
	})

	out := captureStdout(t, func() {
		err := execCmd(UpdateAlertCommand(ctx), "update", "--org", "acme", "-r", "repo1", "--alert", "1",
			"--state", "resolved", "--resolution", "false_positive", "--comment", "not a real secret")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Successfully updated alert") {
		t.Errorf("expected success message, got: %s", out)
	}
}

func TestUpdateAlertCommand_ReopenState(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/acme/repo1/secret-scanning/alerts/1", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"number": 1, "state": "open"},
	})

	err := execCmd(UpdateAlertCommand(ctx), "update", "--org", "acme", "-r", "repo1", "--alert", "1", "--state", "open")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateAlertCommand_InvalidState(t *testing.T) {
	ctx, mockServer := newMockedContext(t)

	err := execCmd(UpdateAlertCommand(ctx), "update", "--org", "acme", "-r", "repo1", "--alert", "1", "--state", "bogus")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests for invalid state, got %d", len(mockServer.GetRequests()))
	}
}

func TestUpdateAlertCommand_ResolvedMissingResolution(t *testing.T) {
	ctx, mockServer := newMockedContext(t)

	err := execCmd(UpdateAlertCommand(ctx), "update", "--org", "acme", "-r", "repo1", "--alert", "1", "--state", "resolved")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests when resolution is missing, got %d", len(mockServer.GetRequests()))
	}
}

func TestUpdateAlertCommand_InvalidResolution(t *testing.T) {
	ctx, mockServer := newMockedContext(t)

	err := execCmd(UpdateAlertCommand(ctx), "update", "--org", "acme", "-r", "repo1", "--alert", "1",
		"--state", "resolved", "--resolution", "bogus")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests for invalid resolution, got %d", len(mockServer.GetRequests()))
	}
}

func TestUpdateAlertCommand_DryRun(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.DryRun = true

	out := captureStdout(t, func() {
		err := execCmd(UpdateAlertCommand(ctx), "update", "--org", "acme", "-r", "repo1", "--alert", "1",
			"--state", "resolved", "--resolution", "revoked", "--comment", "rotated")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "DRY RUN") {
		t.Errorf("expected dry-run banner, got: %s", out)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests in dry-run mode, got %d", len(mockServer.GetRequests()))
	}
}

func TestUpdateAlertCommand_ServerError(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/acme/repo1/secret-scanning/alerts/1", testutils.MockResponse{
		StatusCode: http.StatusUnprocessableEntity,
		Body:       map[string]interface{}{"message": "invalid resolution"},
	})

	err := execCmd(UpdateAlertCommand(ctx), "update", "--org", "acme", "-r", "repo1", "--alert", "1",
		"--state", "resolved", "--resolution", "wont_fix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateAlertCommand_JSONOutput(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.JSON = true
	mockServer.SetResponse("/repos/acme/repo1/secret-scanning/alerts/1", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"number": 1, "state": "resolved", "resolution": "used_in_tests"},
	})

	out := captureStdout(t, func() {
		err := execCmd(UpdateAlertCommand(ctx), "update", "--org", "acme", "-r", "repo1", "--alert", "1",
			"--state", "resolved", "--resolution", "used_in_tests")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "used_in_tests") {
		t.Errorf("expected JSON output to mention resolution, got: %s", out)
	}
}

func TestUpdateAlertCommand_MissingRequiredFlags(t *testing.T) {
	ctx, _ := newMockedContext(t)

	if err := execCmd(UpdateAlertCommand(ctx), "update", "--org", "acme"); err == nil {
		t.Fatal("expected an error for missing required flags")
	}
}
