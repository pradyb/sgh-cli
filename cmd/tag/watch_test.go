// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package tag

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
	"github.com/pradyb/sgh-cli/pkg/context"
	"github.com/pradyb/sgh-cli/pkg/workflow"
)

// withNoLookupWait makes the run lookup poll 3 times without really waiting.
func withNoLookupWait(t *testing.T) {
	t.Helper()
	i, g, s := workflow.RunLookupInterval, workflow.RunLookupGrace, workflow.RunLookupSleep
	workflow.RunLookupInterval, workflow.RunLookupGrace, workflow.RunLookupSleep = time.Second, 3*time.Second, func(time.Duration) {}
	t.Cleanup(func() { workflow.RunLookupInterval, workflow.RunLookupGrace, workflow.RunLookupSleep = i, g, s })
}

// tagServer: v1.0.0 is created on main's head "base-sha-1". Main's own push run (7) is for the same
// commit and must not be mistaken for the tag's run (8).
func tagServer(t *testing.T, runs ...map[string]interface{}) *testutils.MockGitHubServer {
	t.Helper()
	m := testutils.NewMockGitHubServer()
	t.Cleanup(m.Close)
	m.SetResponse("/repos/acme/repo1/git/ref/heads/main", testutils.MockResponse{
		StatusCode: http.StatusOK, Body: map[string]interface{}{"object": map[string]interface{}{"sha": "base-sha-1"}},
	})
	m.SetResponse("/repos/acme/repo1/git/tags", testutils.MockResponse{StatusCode: http.StatusCreated, Body: map[string]interface{}{"sha": "tag-object-sha"}})
	m.SetResponse("/repos/acme/repo1/git/refs", testutils.MockResponse{
		StatusCode: http.StatusCreated,
		Body:       map[string]interface{}{"ref": "refs/tags/v1.0.0", "object": map[string]interface{}{"sha": "tag-object-sha", "type": "tag"}},
	})
	if runs == nil {
		runs = []map[string]interface{}{
			{"id": 7, "name": "CI", "head_sha": "base-sha-1", "event": "push", "head_branch": "main"},
			{"id": 8, "name": "Release", "head_sha": "base-sha-1", "event": "push", "head_branch": "v1.0.0"},
		}
	}
	m.SetResponse("/repos/acme/repo1/actions/runs", testutils.MockResponse{StatusCode: http.StatusOK, Body: map[string]interface{}{"total_count": len(runs), "workflow_runs": runs}})
	m.SetResponse("/repos/acme/repo1/actions/runs/8", testutils.MockResponse{StatusCode: http.StatusOK, Body: map[string]interface{}{"id": 8, "status": "completed", "conclusion": "success"}})
	m.SetResponse("/repos/acme/repo1/actions/runs/8/jobs", testutils.MockResponse{StatusCode: http.StatusOK, Body: map[string]interface{}{"total_count": 0, "jobs": []map[string]interface{}{}}})
	return m
}

// withServer is a context talking to m (silent: no progress bar or summary banner on stdout).
func withServer(t *testing.T, m *testutils.MockGitHubServer) *context.Context {
	t.Helper()
	ctx := servicetest.NewMockContext(t, m)
	ctx.Silent = true
	return ctx
}

func tagArgs(extra ...string) []string {
	return append([]string{"create", "--org", "acme", "-r", "repo1", "--tag", "v1.0.0", "--head", "main", "-m", "release"}, extra...)
}

func TestCreateCommand_WatchJSON_FollowsTheTagsRunNotTheBranchPush(t *testing.T) {
	withNoLookupWait(t)
	m := tagServer(t)
	ctx := withServer(t, m)
	ctx.JSON = true

	out := captureStdout(t, func() {
		if err := execCmd(CreateCommand(ctx), tagArgs("--watch")...); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{`"kind":"tag_created","repository":"repo1","sha":"base-sha-1"`, `"kind":"run_found","repository":"repo1","run_id":8`, `"kind":"run_done","conclusion":"success"`}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d: %q", len(lines), len(want), out)
	}
	for i, l := range lines {
		if !json.Valid([]byte(l)) || !strings.Contains(l, want[i]) {
			t.Errorf("line %d = %q, want valid JSON containing %s", i, l, want[i])
		}
	}
	for _, r := range m.GetRequests() {
		if strings.Contains(r.Path, "/runs/7") {
			t.Errorf("followed the branch push's run: %s", r.Path)
		}
	}
	if ctx.HasError {
		t.Error("expected HasError to stay false")
	}
}

func TestCreateCommand_Watch_Human(t *testing.T) {
	withNoLookupWait(t)
	ctx := withServer(t, tagServer(t))

	out := captureStdout(t, func() {
		if err := execCmd(CreateCommand(ctx), tagArgs("--watch")...); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Watching repo1 run 8 (Release)") || ctx.HasError {
		t.Errorf("HasError=%v, output = %s", ctx.HasError, out)
	}
}

func TestCreateCommand_Watch_OnlyTheBranchPushRunExistsSoNothingIsFollowed(t *testing.T) {
	withNoLookupWait(t)
	m := tagServer(t, map[string]interface{}{"id": 7, "head_sha": "base-sha-1", "event": "push", "head_branch": "main"})
	ctx := withServer(t, m)
	ctx.JSON = true

	out := captureStdout(t, func() {
		if err := execCmd(CreateCommand(ctx), tagArgs("--watch")...); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, `"kind":"watch_skipped"`) || strings.Contains(out, "run_found") || ctx.HasError {
		t.Errorf("HasError=%v, want a watch_skipped line and a clean exit: %s", ctx.HasError, out)
	}
}

func TestCreateCommand_Watch_FailedTagIsReportedAndNotWatched(t *testing.T) {
	withNoLookupWait(t)
	m := tagServer(t)
	m.SetResponse("/repos/acme/repo1/git/ref/heads/main", testutils.MockResponse{StatusCode: http.StatusNotFound, Body: map[string]interface{}{"message": "Not Found"}})
	ctx := withServer(t, m)
	ctx.JSON = true

	out := captureStdout(t, func() {
		if err := execCmd(CreateCommand(ctx), tagArgs("--watch")...); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !ctx.HasError || !strings.Contains(out, `"kind":"tag_error"`) || strings.Contains(out, "run_found") {
		t.Errorf("HasError=%v, want a tag_error line and no watching: %s", ctx.HasError, out)
	}
	for _, r := range m.GetRequests() {
		if strings.Contains(r.Path, "/actions/") {
			t.Errorf("looked for runs after a failed tag: %s", r.Path)
		}
	}
}

func TestCreateCommand_WithoutWatchDoesNotLookForRuns(t *testing.T) {
	m := tagServer(t)
	ctx := withServer(t, m)

	captureStdout(t, func() {
		if err := execCmd(CreateCommand(ctx), tagArgs()...); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, r := range m.GetRequests() {
		if strings.Contains(r.Path, "/actions/") {
			t.Errorf("unexpected run lookup without --watch: %s", r.Path)
		}
	}
}

func TestCreateCommand_InvalidWatchFlagsSendNothing(t *testing.T) {
	for name, extra := range map[string][]string{
		"interval too low":       {"--watch", "--interval", "2"},
		"timeout without watch":  {"--timeout", "5m"},
		"interval without watch": {"--interval", "20"},
	} {
		t.Run(name, func(t *testing.T) {
			m := tagServer(t)
			ctx := withServer(t, m)

			if err := execCmd(CreateCommand(ctx), tagArgs(extra...)...); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !ctx.HasError || len(m.GetRequests()) != 0 {
				t.Errorf("HasError=%v requests=%d, want an error and no tag created", ctx.HasError, len(m.GetRequests()))
			}
		})
	}
}
