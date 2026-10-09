// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package workflow

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pradyb/sgh-cli/internal/model"
	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

func job(id int, name, status, conclusion string, steps ...model.WorkflowStep) model.WorkflowJob {
	return model.WorkflowJob{ID: id, Name: name, Status: status, Conclusion: conclusion, Steps: steps}
}

func step(number int, name, status, conclusion string) model.WorkflowStep {
	return model.WorkflowStep{Number: number, Name: name, Status: status, Conclusion: conclusion}
}

// --- viewWatchState.diff ---

func TestViewWatchState_Diff_JobStartedThenCompleted(t *testing.T) {
	s := newViewWatchState()

	events := s.diff(model.WorkflowRunDetail{Jobs: []model.WorkflowJob{job(1, "build", "queued", "")}})
	if len(events) != 0 {
		t.Fatalf("queued job produced events: %+v, want none", events)
	}

	events = s.diff(model.WorkflowRunDetail{Jobs: []model.WorkflowJob{job(1, "build", "in_progress", "")}})
	if len(events) != 1 || events[0].Kind != "job_started" || events[0].Job != "build" {
		t.Fatalf("events = %+v, want one job_started for build", events)
	}

	events = s.diff(model.WorkflowRunDetail{Jobs: []model.WorkflowJob{job(1, "build", "completed", "success")}})
	if len(events) != 1 || events[0].Kind != "job_completed" || events[0].Conclusion != "success" {
		t.Fatalf("events = %+v, want one job_completed with conclusion success", events)
	}

	// Re-polling the same completed state must not re-report anything.
	events = s.diff(model.WorkflowRunDetail{Jobs: []model.WorkflowJob{job(1, "build", "completed", "success")}})
	if len(events) != 0 {
		t.Fatalf("re-polling completed job produced events: %+v, want none", events)
	}
}

func TestViewWatchState_Diff_JobCompletesWithoutObservedStart(t *testing.T) {
	s := newViewWatchState()

	// Never seen in_progress: goes straight from unseen to completed between polls.
	events := s.diff(model.WorkflowRunDetail{Jobs: []model.WorkflowJob{job(1, "build", "completed", "success")}})
	if len(events) != 1 || events[0].Kind != "job_completed" {
		t.Fatalf("events = %+v, want only job_completed (no synthetic job_started)", events)
	}
}

func TestViewWatchState_Diff_StepStartedAndCompleted(t *testing.T) {
	s := newViewWatchState()

	events := s.diff(model.WorkflowRunDetail{Jobs: []model.WorkflowJob{
		job(1, "build", "in_progress", "", step(1, "checkout", "in_progress", "")),
	}})
	var kinds []string
	for _, e := range events {
		kinds = append(kinds, e.Kind)
	}
	if len(events) != 2 || kinds[0] != "job_started" || kinds[1] != "step_started" {
		t.Fatalf("kinds = %v, want [job_started step_started]", kinds)
	}
	if events[1].Job != "build" || events[1].Step != "checkout" {
		t.Fatalf("step event = %+v, want Job=build Step=checkout", events[1])
	}

	events = s.diff(model.WorkflowRunDetail{Jobs: []model.WorkflowJob{
		job(1, "build", "in_progress", "", step(1, "checkout", "completed", "success")),
	}})
	if len(events) != 1 || events[0].Kind != "step_completed" || events[0].Conclusion != "success" {
		t.Fatalf("events = %+v, want one step_completed with conclusion success", events)
	}

	// A second step at the same job, keyed independently by step number.
	events = s.diff(model.WorkflowRunDetail{Jobs: []model.WorkflowJob{
		job(1, "build", "in_progress", "",
			step(1, "checkout", "completed", "success"),
			step(2, "test", "in_progress", ""),
		),
	}})
	if len(events) != 1 || events[0].Kind != "step_started" || events[0].Step != "test" {
		t.Fatalf("events = %+v, want one step_started for test", events)
	}
}

func TestViewWatchState_Diff_JobCompletedAfterItsStepsInOnePoll(t *testing.T) {
	s := newViewWatchState()

	// The whole job finished between polls: its steps must read before the job's completion.
	events := s.diff(model.WorkflowRunDetail{Jobs: []model.WorkflowJob{
		job(1, "build", "completed", "success",
			step(1, "checkout", "completed", "success"),
			step(2, "test", "completed", "success"),
		),
	}})
	var got []string
	for _, e := range events {
		got = append(got, e.Kind+":"+e.Step)
	}
	if strings.Join(got, ",") != "step_completed:checkout,step_completed:test,job_completed:" {
		t.Fatalf("events = %v, want both step_completed before job_completed", got)
	}
}

// --- WatchViewRun ---

const viewRunPath = "/repos/testorg/repo1/actions/runs/42"

func runDetailResponse(status, conclusion string) testutils.MockResponse {
	return testutils.MockResponse{StatusCode: http.StatusOK, Body: map[string]interface{}{
		"id": 42, "status": status, "conclusion": conclusion,
	}}
}

func jobsResponse(jobs ...map[string]interface{}) testutils.MockResponse {
	return testutils.MockResponse{StatusCode: http.StatusOK, Body: map[string]interface{}{
		"total_count": len(jobs), "jobs": jobs,
	}}
}

func jobBody(id int, name, status, conclusion string) map[string]interface{} {
	return map[string]interface{}{"id": id, "run_id": 42, "name": name, "status": status, "conclusion": conclusion}
}

func TestWatchViewRun_PollsUntilRunCompletesReportingEachTransitionOnce(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)

	mockServer.SetResponseSequence(viewRunPath, []testutils.MockResponse{
		runDetailResponse("in_progress", ""),
		runDetailResponse("completed", "success"),
	})
	mockServer.SetResponseSequence(viewRunPath+"/jobs", []testutils.MockResponse{
		jobsResponse(jobBody(1, "build", "in_progress", "")),
		jobsResponse(jobBody(1, "build", "completed", "success")),
	})

	initial := GetWorkflowRunDetail(ctx, WorkflowRunRequest{OrgName: "testorg", RepoName: "repo1", RunID: 42})

	var events []ViewWatchEvent
	final := WatchViewRun(context.Background(), ctx, WorkflowRunRequest{OrgName: "testorg", RepoName: "repo1", RunID: 42}, initial, WatchViewOptions{
		Interval: time.Millisecond,
		Sleep:    func(time.Duration) {},
		Notify:   func(e ViewWatchEvent) { events = append(events, e) },
	})

	if final.Run.Status != "completed" {
		t.Fatalf("final.Run.Status = %q, want completed", final.Run.Status)
	}
	var kinds []string
	for _, e := range events {
		kinds = append(kinds, e.Kind)
	}
	want := []string{"job_started", "job_completed", "run_done"}
	if len(kinds) != len(want) {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
	for i, k := range want {
		if kinds[i] != k {
			t.Fatalf("kinds = %v, want %v", kinds, want)
		}
	}
}

func TestWatchViewRun_AlreadyCompletedReportsOnlyJobAndRunDone(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)

	mockServer.SetResponse(viewRunPath, runDetailResponse("completed", "success"))
	mockServer.SetResponse(viewRunPath+"/jobs", jobsResponse(jobBody(1, "build", "completed", "success")))

	initial := GetWorkflowRunDetail(ctx, WorkflowRunRequest{OrgName: "testorg", RepoName: "repo1", RunID: 42})

	var events []ViewWatchEvent
	WatchViewRun(context.Background(), ctx, WorkflowRunRequest{OrgName: "testorg", RepoName: "repo1", RunID: 42}, initial, WatchViewOptions{
		Interval: time.Millisecond,
		Sleep:    func(time.Duration) { t.Fatal("should not sleep: run is already completed") },
		Notify:   func(e ViewWatchEvent) { events = append(events, e) },
	})

	if len(events) != 2 || events[0].Kind != "job_completed" || events[1].Kind != "run_done" {
		t.Fatalf("events = %+v, want [job_completed run_done]", events)
	}
	if events[1].Conclusion != "success" {
		t.Errorf("run_done.Conclusion = %q, want success", events[1].Conclusion)
	}
}

func TestWatchViewRun_ContextCancelledStopsCleanly(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)

	mockServer.SetResponse(viewRunPath, runDetailResponse("in_progress", ""))
	mockServer.SetResponse(viewRunPath+"/jobs", jobsResponse())

	initial := GetWorkflowRunDetail(ctx, WorkflowRunRequest{OrgName: "testorg", RepoName: "repo1", RunID: 42})

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	var events []ViewWatchEvent
	final := WatchViewRun(cancelledCtx, ctx, WorkflowRunRequest{OrgName: "testorg", RepoName: "repo1", RunID: 42}, initial, WatchViewOptions{
		Interval: time.Millisecond,
		Sleep:    func(time.Duration) { t.Fatal("should not sleep: context already cancelled") },
		Notify:   func(e ViewWatchEvent) { events = append(events, e) },
	})

	if final.Run.Status != "in_progress" {
		t.Errorf("final.Run.Status = %q, want in_progress (unchanged, since watch stopped)", final.Run.Status)
	}
	if len(events) != 0 {
		t.Errorf("events = %+v, want none (context already cancelled, run never observed as done)", events)
	}
}
