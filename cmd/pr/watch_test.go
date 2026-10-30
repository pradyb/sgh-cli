// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package pr

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
	"github.com/pradyb/sgh-cli/pkg/workflow"
)

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

// withNoLookupWait makes the run lookup poll 3 times without really waiting.
func withNoLookupWait(t *testing.T) {
	t.Helper()
	i, g, s := workflow.RunLookupInterval, workflow.RunLookupGrace, workflow.RunLookupSleep
	workflow.RunLookupInterval, workflow.RunLookupGrace, workflow.RunLookupSleep = time.Second, 3*time.Second, func(time.Duration) {}
	t.Cleanup(func() { workflow.RunLookupInterval, workflow.RunLookupGrace, workflow.RunLookupSleep = i, g, s })
}

const mergePath = "/repos/acme/repo1/pulls/1/merge"

// mergeServer: PR 1 merges as commit "mergesha"; the push to the base branch started run 555 (done)
// for that commit, plus a run for somebody else's push that must not be followed.
func mergeServer(t *testing.T, runs ...map[string]interface{}) *testutils.MockGitHubServer {
	t.Helper()
	m := testutils.NewMockGitHubServer()
	t.Cleanup(m.Close)
	m.SetResponse(mergePath, testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"merged": true, "sha": "mergesha", "message": "Pull Request successfully merged"},
	})
	if runs == nil {
		runs = []map[string]interface{}{
			{"id": 555, "name": "CI", "head_sha": "mergesha", "event": "push", "head_branch": "main"},
			{"id": 999, "name": "CI", "head_sha": "someone-elses", "event": "push", "head_branch": "main"},
		}
	}
	m.SetResponse("/repos/acme/repo1/actions/runs", testutils.MockResponse{StatusCode: http.StatusOK, Body: map[string]interface{}{"total_count": len(runs), "workflow_runs": runs}})
	m.SetResponse("/repos/acme/repo1/actions/runs/555", testutils.MockResponse{StatusCode: http.StatusOK, Body: map[string]interface{}{"id": 555, "status": "completed", "conclusion": "success"}})
	m.SetResponse("/repos/acme/repo1/actions/runs/555/jobs", testutils.MockResponse{StatusCode: http.StatusOK, Body: map[string]interface{}{"total_count": 0, "jobs": []map[string]interface{}{}}})
	return m
}

func mergeArgs(extra ...string) []string {
	return append([]string{"merge", "--org", "acme", "-r", "repo1", "--pr", "1"}, extra...)
}

func TestMergeCommand_WatchJSON_FollowsTheRunForTheMergeCommitOnly(t *testing.T) {
	withNoLookupWait(t)
	m := mergeServer(t)
	ctx := servicetest.NewMockContext(t, m)
	ctx.JSON = true

	out := captureStdout(t, func() {
		if err := execCmd(MergeCommand(ctx), mergeArgs("--watch")...); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{`"kind":"merged"`, `"kind":"run_found","repository":"repo1","run_id":555,"sha":"mergesha"`, `"kind":"run_done","conclusion":"success"`}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d: %q", len(lines), len(want), out)
	}
	for i, l := range lines {
		if !json.Valid([]byte(l)) || !strings.Contains(l, want[i]) {
			t.Errorf("line %d = %q, want valid JSON containing %s", i, l, want[i])
		}
	}
	if !strings.Contains(lines[0], `"sha":"mergesha"`) {
		t.Errorf("merged line should carry the merge commit: %s", lines[0])
	}
	for _, r := range m.GetRequests() {
		if strings.Contains(r.Path, "/runs/999") {
			t.Errorf("followed someone else's run: %s", r.Path)
		}
	}
	if ctx.HasError {
		t.Error("expected HasError to stay false")
	}
}

func TestMergeCommand_Watch_Human(t *testing.T) {
	withNoLookupWait(t)
	ctx := servicetest.NewMockContext(t, mergeServer(t))
	ctx.Silent = true

	out := captureStdout(t, func() {
		if err := execCmd(MergeCommand(ctx), mergeArgs("--watch")...); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Pull Request successfully merged") || !strings.Contains(out, "Watching repo1 run 555 (CI)") || ctx.HasError {
		t.Errorf("HasError=%v, output = %s", ctx.HasError, out)
	}
}

func TestMergeCommand_Watch_NoRunExitsCleanly(t *testing.T) {
	withNoLookupWait(t)
	m := mergeServer(t, map[string]interface{}{"id": 999, "head_sha": "someone-elses", "event": "push", "head_branch": "main"})
	ctx := servicetest.NewMockContext(t, m)
	ctx.JSON = true

	out := captureStdout(t, func() {
		if err := execCmd(MergeCommand(ctx), mergeArgs("--watch")...); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, `"kind":"watch_skipped"`) || strings.Contains(out, "run_found") || ctx.HasError {
		t.Errorf("HasError=%v, want merged then watch_skipped and a clean exit: %s", ctx.HasError, out)
	}
}

func TestMergeCommand_Watch_FailedMergeDoesNotLookForRuns(t *testing.T) {
	withNoLookupWait(t)
	m := mergeServer(t)
	m.SetResponse(mergePath, testutils.MockResponse{StatusCode: http.StatusMethodNotAllowed, Body: map[string]interface{}{"message": "Pull Request is not mergeable"}})
	ctx := servicetest.NewMockContext(t, m)
	ctx.JSON = true

	out := captureStdout(t, func() {
		if err := execCmd(MergeCommand(ctx), mergeArgs("--watch")...); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !ctx.HasError || !strings.Contains(out, `"kind":"merge_error"`) || strings.Contains(out, "run_found") {
		t.Errorf("HasError=%v, want a merge_error line and no watching: %s", ctx.HasError, out)
	}
	for _, r := range m.GetRequests() {
		if strings.Contains(r.Path, "/actions/") {
			t.Errorf("looked for runs after a failed merge: %s", r.Path)
		}
	}
}

func TestMergeCommand_WithoutWatchDoesNotLookForRuns(t *testing.T) {
	m := mergeServer(t)
	ctx := servicetest.NewMockContext(t, m)
	ctx.Silent = true

	captureStdout(t, func() {
		if err := execCmd(MergeCommand(ctx), mergeArgs()...); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, r := range m.GetRequests() {
		if strings.Contains(r.Path, "/actions/") {
			t.Errorf("unexpected run lookup without --watch: %s", r.Path)
		}
	}
}

func TestMergeCommand_InvalidWatchFlagsSendNothing(t *testing.T) {
	for name, extra := range map[string][]string{
		"interval too low":       {"--watch", "--interval", "2"},
		"timeout without watch":  {"--timeout", "5m"},
		"interval without watch": {"--interval", "20"},
	} {
		t.Run(name, func(t *testing.T) {
			m := mergeServer(t)
			ctx := servicetest.NewMockContext(t, m)

			if err := execCmd(MergeCommand(ctx), mergeArgs(extra...)...); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !ctx.HasError || len(m.GetRequests()) != 0 {
				t.Errorf("HasError=%v requests=%d, want an error and nothing merged", ctx.HasError, len(m.GetRequests()))
			}
		})
	}
}
