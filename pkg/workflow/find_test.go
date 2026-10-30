// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package workflow

import (
	stdctx "context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

const runsPath = "/repos/testorg/repo1/actions/runs"

// withFastLookup makes lookups poll without really waiting (3 attempts) for one test.
func withFastLookup(t *testing.T) *int {
	t.Helper()
	origI, origG, origS := RunLookupInterval, RunLookupGrace, RunLookupSleep
	sleeps := 0
	RunLookupInterval, RunLookupGrace = time.Second, 3*time.Second
	RunLookupSleep = func(time.Duration) { sleeps++ }
	t.Cleanup(func() { RunLookupInterval, RunLookupGrace, RunLookupSleep = origI, origG, origS })
	return &sleeps
}

func run(id int, sha, event, branch string) map[string]interface{} {
	return map[string]interface{}{"id": id, "head_sha": sha, "event": event, "head_branch": branch, "name": "CI"}
}

func runsBody(runs ...map[string]interface{}) testutils.MockResponse {
	return testutils.MockResponse{StatusCode: http.StatusOK, Body: map[string]interface{}{"total_count": len(runs), "workflow_runs": runs}}
}

func TestFindRuns_MatchesOnlyTheCommitEventAndBranchAndSortsByID(t *testing.T) {
	withFastLookup(t)
	m := testutils.NewMockGitHubServer()
	defer m.Close()
	m.SetResponse(runsPath, runsBody(
		run(30, "abc", "push", "v1.0.0"),
		run(10, "abc", "push", "v1.0.0"),
		run(20, "abc", "push", "main"),         // same commit, earlier branch push (not this tag)
		run(40, "abc", "pull_request", "main"), // wrong event
		run(50, "other", "push", "v1.0.0"),     // someone else's commit
	))
	ctx := servicetest.NewMockContext(t, m)

	got, err := FindRuns(stdctx.Background(), ctx, FindRunsRequest{OrgName: "testorg", RepoName: "repo1", HeadSHA: "abc", Event: "push", Branch: "v1.0.0"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0].ID != 10 || got[1].ID != 30 {
		t.Errorf("runs = %+v, want only runs 10 and 30, in ID order", got)
	}
	var query string
	for _, r := range m.GetRequests() {
		if r.Path == runsPath {
			query = r.Query
		}
	}
	if !strings.Contains(query, "head_sha=abc") || !strings.Contains(query, "event=push") {
		t.Errorf("request %q should filter by head_sha and event server-side too", query)
	}
}

func TestFindRuns_WaitsForTheRunToAppear(t *testing.T) {
	sleeps := withFastLookup(t)
	m := testutils.NewMockGitHubServer()
	defer m.Close()
	m.SetResponseSequence(runsPath, []testutils.MockResponse{
		runsBody(),
		runsBody(),
		runsBody(run(7, "abc", "push", "main")),
	})
	ctx := servicetest.NewMockContext(t, m)

	got, err := FindRuns(stdctx.Background(), ctx, FindRunsRequest{OrgName: "testorg", RepoName: "repo1", HeadSHA: "abc", Event: "push"})

	if err != nil || len(got) != 1 || got[0].ID != 7 {
		t.Fatalf("got %+v, err %v, want run 7 after two empty polls", got, err)
	}
	if *sleeps < 2 {
		t.Errorf("sleeps = %d, want it to have waited between polls", *sleeps)
	}
}

func TestFindRuns_NothingWithinGraceIsNotAnError(t *testing.T) {
	withFastLookup(t)
	m := testutils.NewMockGitHubServer()
	defer m.Close()
	m.SetResponse(runsPath, runsBody(run(1, "someone-elses", "push", "main")))
	ctx := servicetest.NewMockContext(t, m)

	got, err := FindRuns(stdctx.Background(), ctx, FindRunsRequest{OrgName: "testorg", RepoName: "repo1", HeadSHA: "abc", Event: "push"})

	if err != nil || len(got) != 0 {
		t.Errorf("got %+v, err %v, want no runs and no error", got, err)
	}
	var polls int
	for _, r := range m.GetRequests() {
		if r.Path == runsPath {
			polls++
		}
	}
	if polls != 3 {
		t.Errorf("polls = %d, want 3 (grace/interval)", polls)
	}
}

func TestFindRuns_APIErrorIsReturned(t *testing.T) {
	withFastLookup(t)
	m := testutils.NewMockGitHubServer()
	defer m.Close()
	m.SetResponse(runsPath, testutils.MockResponse{StatusCode: http.StatusForbidden, Body: map[string]interface{}{"message": "no"}})
	ctx := servicetest.NewMockContext(t, m)

	if _, err := FindRuns(stdctx.Background(), ctx, FindRunsRequest{OrgName: "testorg", RepoName: "repo1", HeadSHA: "abc"}); err == nil {
		t.Error("expected the listing error to be returned")
	}
}

func TestFindRuns_StopsWhenCancelled(t *testing.T) {
	withFastLookup(t)
	m := testutils.NewMockGitHubServer()
	defer m.Close()
	m.SetResponse(runsPath, runsBody())
	ctx := servicetest.NewMockContext(t, m)
	cancelled, cancel := stdctx.WithCancel(stdctx.Background())
	cancel()

	_, err := FindRuns(cancelled, ctx, FindRunsRequest{OrgName: "testorg", RepoName: "repo1", HeadSHA: "abc"})

	if err == nil {
		t.Error("expected the cancellation to be returned")
	}
}

const rerunRunPath = "/repos/testorg/repo1/actions/runs/5"

func runDetail(status string, attempt int) testutils.MockResponse {
	return testutils.MockResponse{StatusCode: http.StatusOK, Body: map[string]interface{}{"id": 5, "status": status, "run_attempt": attempt}}
}

func TestWaitForRerun(t *testing.T) {
	req := WorkflowRunRequest{OrgName: "testorg", RepoName: "repo1", RunID: 5}
	jobs := testutils.MockResponse{StatusCode: http.StatusOK, Body: map[string]interface{}{"total_count": 0, "jobs": []map[string]interface{}{}}}

	t.Run("old completed state is skipped until the new attempt shows", func(t *testing.T) {
		withFastLookup(t)
		m := testutils.NewMockGitHubServer()
		defer m.Close()
		m.SetResponseSequence(rerunRunPath, []testutils.MockResponse{runDetail("completed", 1), runDetail("completed", 1), runDetail("queued", 2)})
		m.SetResponse(rerunRunPath+"/jobs", jobs)
		ctx := servicetest.NewMockContext(t, m)

		detail, ok := WaitForRerun(stdctx.Background(), ctx, req, 1)

		if !ok || detail.Run.RunAttempt != 2 || detail.Run.Status != "queued" {
			t.Errorf("got ok=%v %+v, want the queued attempt 2", ok, detail.Run)
		}
	})

	t.Run("a new attempt that already finished still counts", func(t *testing.T) {
		withFastLookup(t)
		m := testutils.NewMockGitHubServer()
		defer m.Close()
		m.SetResponse(rerunRunPath, runDetail("completed", 2))
		m.SetResponse(rerunRunPath+"/jobs", jobs)
		ctx := servicetest.NewMockContext(t, m)

		if _, ok := WaitForRerun(stdctx.Background(), ctx, req, 1); !ok {
			t.Error("attempt 2 > 1 should count as started even though it is completed")
		}
	})

	t.Run("never starting gives up", func(t *testing.T) {
		withFastLookup(t)
		m := testutils.NewMockGitHubServer()
		defer m.Close()
		m.SetResponse(rerunRunPath, runDetail("completed", 1))
		m.SetResponse(rerunRunPath+"/jobs", jobs)
		ctx := servicetest.NewMockContext(t, m)

		if _, ok := WaitForRerun(stdctx.Background(), ctx, req, 1); ok {
			t.Error("expected false when no new attempt appears")
		}
	})

	t.Run("an unfetchable run is handed back to be reported", func(t *testing.T) {
		withFastLookup(t)
		m := testutils.NewMockGitHubServer()
		defer m.Close()
		m.SetResponse(rerunRunPath, testutils.MockResponse{StatusCode: http.StatusNotFound, Body: map[string]interface{}{"message": "Not Found"}})
		ctx := servicetest.NewMockContext(t, m)

		detail, ok := WaitForRerun(stdctx.Background(), ctx, req, 1)

		if !ok || detail.ErrorMessage == "" {
			t.Errorf("got ok=%v err=%q, want true with the error set", ok, detail.ErrorMessage)
		}
	})
}
