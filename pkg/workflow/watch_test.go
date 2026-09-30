// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package workflow

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/pradyb/sgh-cli/internal/testutils"
)

const runPath = "/repos/testorg/repo1/actions/runs/42"

func runStatus(status, conclusion string) testutils.MockResponse {
	return testutils.MockResponse{StatusCode: http.StatusOK, Body: map[string]interface{}{
		"id": 42, "status": status, "conclusion": conclusion,
	}}
}

func pendingBody(gates ...map[string]interface{}) testutils.MockResponse {
	return testutils.MockResponse{StatusCode: http.StatusOK, Body: gates}
}

// noSleep is passed as WatchApproveRequest.Sleep so tests never actually wait.
func noSleep(time.Duration) {}

func TestWatchApprovals_ApprovesGateThenRunCompletes(t *testing.T) {
	mockServer, ctx := newApproveCtx(t, gate(1, "approval-1", true))
	mockServer.SetResponseSequence(runPath, []testutils.MockResponse{
		runStatus("waiting", ""),
		runStatus("completed", "success"),
	})

	var events []WatchEvent
	res, err := WatchApprovals(context.Background(), ctx, WatchApproveRequest{
		OrgName: "testorg", RepoName: "repo1", RunID: 42,
		Yes: true, Environments: []string{"approval-1"},
		Sleep:  noSleep,
		Notify: func(e WatchEvent) { events = append(events, e) },
	})

	if err != nil {
		t.Fatalf("WatchApprovals(): %v", err)
	}
	if res.Conclusion != "success" || len(res.Decided) != 1 || res.Decided[0] != "approval-1" {
		t.Errorf("res = %+v", res)
	}
	if len(events) != 2 || events[0].Kind != "gate_decided" || events[1].Kind != "run_done" {
		t.Fatalf("events = %+v", events)
	}
	req, ok := lastReview(mockServer)
	if !ok {
		t.Fatal("expected a POST review request")
	}
	if !jsonEqual(req.Body, `{"environment_ids":[1],"state":"approved","comment":"Approved via sgh"}`) {
		t.Errorf("body = %s", req.Body)
	}
}

func TestWatchApprovals_TwoSequentialGates(t *testing.T) {
	mockServer, ctx := newApproveCtx(t)
	mockServer.SetResponseSequence(runPath, []testutils.MockResponse{
		runStatus("waiting", ""),
		runStatus("waiting", ""),
		runStatus("completed", "success"),
	})
	mockServer.SetResponseSequence(pendingPath, []testutils.MockResponse{
		pendingBody(gate(1, "approval-1", true)),
		pendingBody(gate(2, "approval-2", true)), // approval-1 has advanced past; only approval-2 remains
	})

	var decided []string
	res, err := WatchApprovals(context.Background(), ctx, WatchApproveRequest{
		OrgName: "testorg", RepoName: "repo1", RunID: 42,
		Yes: true, Environments: []string{"approval-1", "approval-2"},
		Sleep: noSleep,
		Notify: func(e WatchEvent) {
			if e.Kind == "gate_decided" {
				decided = append(decided, e.Environment)
			}
		},
	})

	if err != nil {
		t.Fatalf("WatchApprovals(): %v", err)
	}
	if len(decided) != 2 || decided[0] != "approval-1" || decided[1] != "approval-2" {
		t.Errorf("decided in order = %v, want [approval-1 approval-2]", decided)
	}
	if res.Conclusion != "success" {
		t.Errorf("Conclusion = %q, want success", res.Conclusion)
	}
}

func TestWatchApprovals_DeclineThenAnotherGateContinues(t *testing.T) {
	mockServer, ctx := newApproveCtx(t)
	mockServer.SetResponseSequence(runPath, []testutils.MockResponse{
		runStatus("waiting", ""),
		runStatus("waiting", ""),
		runStatus("completed", "success"),
	})
	mockServer.SetResponse(pendingPath, testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       []map[string]interface{}{gate(1, "approval-1", true), gate(2, "approval-2", true)},
	})

	calls := 0
	res, err := WatchApprovals(context.Background(), ctx, WatchApproveRequest{
		OrgName: "testorg", RepoName: "repo1", RunID: 42,
		Sleep: noSleep,
		Confirm: func(g WatchGate) bool {
			calls++
			return g.Environment != "approval-1" // decline approval-1, approve approval-2
		},
	})

	if err != nil {
		t.Fatalf("WatchApprovals(): %v", err)
	}
	if calls != 2 {
		t.Errorf("Confirm called %d times, want 2 (once per gate, never repeated)", calls)
	}
	if len(res.Decided) != 1 || res.Decided[0] != "approval-2" {
		t.Errorf("Decided = %v, want [approval-2]", res.Decided)
	}
	req, _ := lastReview(mockServer)
	if !jsonEqual(req.Body, `{"environment_ids":[2],"state":"approved","comment":"Approved via sgh"}`) {
		t.Errorf("only approval-2 should have been submitted, body = %s", req.Body)
	}
}

func TestWatchApprovals_YesWithoutEnvironmentsRefused(t *testing.T) {
	mockServer, ctx := newApproveCtx(t)

	_, err := WatchApprovals(context.Background(), ctx, WatchApproveRequest{
		OrgName: "testorg", RepoName: "repo1", RunID: 42, Yes: true,
	})

	if err == nil {
		t.Fatal("expected an error")
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Error("must not make any network call before this validation")
	}
}

func TestWatchApprovals_NoConfirmAndNotYesRefused(t *testing.T) {
	mockServer, ctx := newApproveCtx(t)

	_, err := WatchApprovals(context.Background(), ctx, WatchApproveRequest{
		OrgName: "testorg", RepoName: "repo1", RunID: 42,
	})

	if err == nil {
		t.Fatal("expected an error")
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Error("must not make any network call before this validation")
	}
}

func TestWatchApprovals_YesWithEnvironmentsDecidesOnlyAllowlisted(t *testing.T) {
	mockServer, ctx := newApproveCtx(t)
	mockServer.SetResponseSequence(runPath, []testutils.MockResponse{
		runStatus("waiting", ""),
		runStatus("completed", "success"),
	})
	mockServer.SetResponse(pendingPath, testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       []map[string]interface{}{gate(1, "approval-1", true), gate(2, "production", true)},
	})

	var skippedReasons []string
	res, err := WatchApprovals(context.Background(), ctx, WatchApproveRequest{
		OrgName: "testorg", RepoName: "repo1", RunID: 42,
		Yes: true, Environments: []string{"approval-1"},
		Sleep: noSleep,
		Notify: func(e WatchEvent) {
			if e.Kind == "gate_skipped" {
				skippedReasons = append(skippedReasons, e.Environment+": "+e.Reason)
			}
		},
	})

	if err != nil {
		t.Fatalf("WatchApprovals(): %v", err)
	}
	if len(res.Decided) != 1 || res.Decided[0] != "approval-1" {
		t.Errorf("Decided = %v, want only approval-1", res.Decided)
	}
	if len(skippedReasons) != 1 || skippedReasons[0] != "production: not in --environment, left pending" {
		t.Errorf("skipped = %v", skippedReasons)
	}
	req, _ := lastReview(mockServer)
	if !jsonEqual(req.Body, `{"environment_ids":[1],"state":"approved","comment":"Approved via sgh"}`) {
		t.Errorf("only approval-1 should have been submitted, body = %s", req.Body)
	}
}

func TestWatchApprovals_NotAReviewerSkippedAndReported(t *testing.T) {
	mockServer, ctx := newApproveCtx(t, gate(1, "approval-1", false))
	mockServer.SetResponseSequence(runPath, []testutils.MockResponse{
		runStatus("waiting", ""),
		runStatus("completed", "success"),
	})

	var events []WatchEvent
	res, err := WatchApprovals(context.Background(), ctx, WatchApproveRequest{
		OrgName: "testorg", RepoName: "repo1", RunID: 42,
		Yes: true, Environments: []string{"approval-1"},
		Sleep:  noSleep,
		Notify: func(e WatchEvent) { events = append(events, e) },
	})

	if err != nil {
		t.Fatalf("WatchApprovals(): %v", err)
	}
	if len(res.Decided) != 0 {
		t.Errorf("Decided = %v, want none", res.Decided)
	}
	if len(events) < 1 || events[0].Kind != "gate_skipped" || events[0].Reason != "not a required reviewer" {
		t.Errorf("events = %+v", events)
	}
	if len(mockServer.GetRequests()) == 0 {
		t.Fatal("expected at least one request")
	}
	for _, r := range mockServer.GetRequests() {
		if r.Method == http.MethodPost && r.Path == pendingPath {
			t.Error("must not submit a review for a gate the user cannot approve")
		}
	}
}

func TestWatchApprovals_ConcurrentDecisionErrorReportedAndContinues(t *testing.T) {
	mockServer, ctx := newApproveCtx(t)
	mockServer.SetResponseSequence(runPath, []testutils.MockResponse{
		runStatus("waiting", ""),
		runStatus("completed", "success"),
	})
	// The mock server keys responses by path only (not method), so a poll's GET (list)
	// then POST (decide) to the same pending_deployments path consume this queue in
	// order — exactly the sequence one poll produces. A 422 simulates someone else
	// having already decided the gate first.
	mockServer.SetResponseSequence(pendingPath, []testutils.MockResponse{
		{StatusCode: http.StatusOK, Body: []map[string]interface{}{gate(1, "approval-1", true)}},                  // GET
		{StatusCode: http.StatusUnprocessableEntity, Body: map[string]interface{}{"message": "already reviewed"}}, // POST
	})

	var events []WatchEvent
	res, err := WatchApprovals(context.Background(), ctx, WatchApproveRequest{
		OrgName: "testorg", RepoName: "repo1", RunID: 42,
		Yes: true, Environments: []string{"approval-1"},
		Sleep:  noSleep,
		Notify: func(e WatchEvent) { events = append(events, e) },
	})

	if err != nil {
		t.Fatalf("WatchApprovals() should not stop on a non-auth submit error: %v", err)
	}
	if !res.HadFailures {
		t.Error("expected HadFailures = true")
	}
	found := false
	for _, e := range events {
		if e.Kind == "gate_error" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a gate_error event, got %+v", events)
	}
}

func TestWatchApprovals_AuthErrorStopsWatch(t *testing.T) {
	mockServer, ctx := newApproveCtx(t)
	mockServer.SetResponse(runPath, runStatus("waiting", ""))
	mockServer.SetResponseSequence(pendingPath, []testutils.MockResponse{
		{StatusCode: http.StatusOK, Body: []map[string]interface{}{gate(1, "approval-1", true)}}, // GET
		{StatusCode: http.StatusForbidden, Body: map[string]interface{}{"message": "Forbidden"}}, // POST
	})

	_, err := WatchApprovals(context.Background(), ctx, WatchApproveRequest{
		OrgName: "testorg", RepoName: "repo1", RunID: 42,
		Yes: true, Environments: []string{"approval-1"},
		Sleep: noSleep,
	})

	if err == nil {
		t.Fatal("expected an auth error to stop the watch")
	}
}

func TestWatchApprovals_ContextCancelledStopsCleanly(t *testing.T) {
	mockServer, ctx := newApproveCtx(t)
	mockServer.SetResponse(runPath, runStatus("waiting", ""))

	watchCtx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before the first poll

	res, err := WatchApprovals(watchCtx, ctx, WatchApproveRequest{
		OrgName: "testorg", RepoName: "repo1", RunID: 42,
		Yes: true, Environments: []string{"approval-1"},
		Sleep: noSleep,
	})

	if err != nil {
		t.Fatalf("a cancelled context should stop cleanly, got: %v", err)
	}
	if len(res.Decided) != 0 {
		t.Errorf("Decided = %v, want none", res.Decided)
	}
}

func TestWatchApprovals_Timeout(t *testing.T) {
	mockServer, ctx := newApproveCtx(t)
	mockServer.SetResponse(runPath, runStatus("waiting", ""))

	_, err := WatchApprovals(context.Background(), ctx, WatchApproveRequest{
		OrgName: "testorg", RepoName: "repo1", RunID: 42,
		Yes: true, Environments: []string{"approval-1"},
		Timeout: 1 * time.Nanosecond, // elapsed by the time the first poll finishes
		Sleep:   noSleep,
	})

	if !errors.Is(err, ErrWatchTimeout) {
		t.Errorf("err = %v, want ErrWatchTimeout", err)
	}
}

func TestWatchApprovals_ResolvesLatestWaitingRun(t *testing.T) {
	mockServer, ctx := newApproveCtx(t)
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"total_count":   1,
			"workflow_runs": []map[string]interface{}{{"id": 42, "status": "waiting"}},
		},
	})
	mockServer.SetResponse(runPath, runStatus("completed", "success"))

	res, err := WatchApprovals(context.Background(), ctx, WatchApproveRequest{
		OrgName: "testorg", RepoName: "repo1",
		Yes: true, Environments: []string{"approval-1"},
		Sleep: noSleep,
	})

	if err != nil {
		t.Fatalf("WatchApprovals(): %v", err)
	}
	if res.RunID != 42 {
		t.Errorf("RunID = %d, want 42", res.RunID)
	}
}

func TestWatchApprovals_ReporterCalledForRunDone(t *testing.T) {
	mockServer, ctx := newApproveCtx(t)
	mockServer.SetResponse(runPath, runStatus("completed", "failure"))

	var gotConclusion string
	res, err := WatchApprovals(context.Background(), ctx, WatchApproveRequest{
		OrgName: "testorg", RepoName: "repo1", RunID: 42,
		Yes: true, Environments: []string{"approval-1"},
		Sleep: noSleep,
		Notify: func(e WatchEvent) {
			if e.Kind == "run_done" {
				gotConclusion = e.Conclusion
			}
		},
	})

	if err != nil {
		t.Fatalf("WatchApprovals(): %v", err)
	}
	if gotConclusion != "failure" || res.Conclusion != "failure" {
		t.Errorf("conclusion = %q / %q, want failure/failure", gotConclusion, res.Conclusion)
	}
}
