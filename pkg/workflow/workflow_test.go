// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package workflow

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
	"github.com/pradyb/sgh-cli/pkg/context"
)

func TestDispatchWorkflow_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/actions/workflows/build.yml/dispatches", testutils.MockResponse{
		StatusCode: http.StatusNoContent,
	})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	results := DispatchWorkflow(ctx, WorkflowDispatchRequest{
		OrgName:    "testorg",
		RepoNames:  []string{"repo1"},
		WorkflowID: "build.yml",
		Ref:        "main",
		Inputs:     map[string]string{"env": "prod"},
	})

	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	got := results[0]
	if got.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", got.ErrorMessage)
	}
	if got.RepositoryName != "repo1" || got.WorkflowID != "build.yml" || got.Ref != "main" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestDispatchWorkflow_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/actions/workflows/build.yml/dispatches", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	results := DispatchWorkflow(ctx, WorkflowDispatchRequest{
		OrgName:    "testorg",
		RepoNames:  []string{"repo1"},
		WorkflowID: "build.yml",
		Ref:        "main",
	})

	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
}

func workflowRunsBody() map[string]interface{} {
	return map[string]interface{}{
		"total_count": 2,
		"workflow_runs": []map[string]interface{}{
			{"id": 1, "name": "Build", "status": "completed", "conclusion": "success", "head_branch": "main"},
			{"id": 2, "name": "Deploy", "status": "completed", "conclusion": "failure", "head_branch": "main"},
		},
	}
}

func TestListWorkflowRuns_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       workflowRunsBody(),
	})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	runs := ListWorkflowRuns(ctx, WorkflowListRequest{OrgName: "testorg", RepoNames: []string{"repo1"}, Count: 10})

	if len(runs) != 2 {
		t.Fatalf("len(runs) = %d, want 2", len(runs))
	}
	for _, r := range runs {
		if r.RepositoryName != "repo1" {
			t.Errorf("RepositoryName = %q, want %q", r.RepositoryName, "repo1")
		}
	}
}

func TestListWorkflowRuns_FilterByWorkflowName(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       workflowRunsBody(),
	})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	tests := []struct {
		name         string
		workflowName string
		wantNames    []string
	}{
		{name: "no filter returns all", workflowName: "", wantNames: []string{"Build", "Deploy"}},
		{name: "case-insensitive substring match", workflowName: "build", wantNames: []string{"Build"}},
		{name: "no match returns none", workflowName: "release", wantNames: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runs := ListWorkflowRuns(ctx, WorkflowListRequest{
				OrgName: "testorg", RepoNames: []string{"repo1"}, Count: 10, WorkflowName: tt.workflowName,
			})
			var gotNames []string
			for _, r := range runs {
				gotNames = append(gotNames, r.Name)
			}
			if len(gotNames) != len(tt.wantNames) {
				t.Fatalf("names = %v, want %v", gotNames, tt.wantNames)
			}
			for i := range gotNames {
				if gotNames[i] != tt.wantNames[i] {
					t.Errorf("names = %v, want %v", gotNames, tt.wantNames)
				}
			}
		})
	}
}

func TestListWorkflowRuns_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	runs := ListWorkflowRuns(ctx, WorkflowListRequest{OrgName: "testorg", RepoNames: []string{"repo1"}, Count: 10})

	if len(runs) != 1 {
		t.Fatalf("len(runs) = %d, want 1", len(runs))
	}
	if runs[0].ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
}

func TestRerunWorkflowRun_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs/123/rerun", testutils.MockResponse{
		StatusCode: http.StatusCreated,
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	run := RerunWorkflowRun(ctx, WorkflowRunRequest{OrgName: "testorg", RepoName: "repo1", RunID: 123})

	if run.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", run.ErrorMessage)
	}
	if run.Status != "rerun_requested" || run.ID != 123 {
		t.Errorf("unexpected run: %+v", run)
	}
}

func TestRerunWorkflowRun_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs/123/rerun", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	run := RerunWorkflowRun(ctx, WorkflowRunRequest{OrgName: "testorg", RepoName: "repo1", RunID: 123})

	if run.ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
	if run.RepositoryName != "repo1" || run.ID != 123 {
		t.Errorf("unexpected run: %+v", run)
	}
}

func TestGetLatestRunID_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"total_count":   1,
			"workflow_runs": []map[string]interface{}{{"id": 555, "name": "Build"}},
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	id, err := GetLatestRunID(ctx, "testorg", "repo1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 555 {
		t.Errorf("id = %d, want 555", id)
	}
}

func TestGetLatestRunID_NoRuns(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"total_count": 0, "workflow_runs": []map[string]interface{}{}},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	_, err := GetLatestRunID(ctx, "testorg", "repo1")
	if err == nil {
		t.Fatal("expected an error when no workflow runs exist")
	}
}

func TestGetLatestRunID_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	_, err := GetLatestRunID(ctx, "testorg", "repo1")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestGetWorkflowRunDetail_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs/123", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"id": 123, "name": "Build", "status": "completed", "conclusion": "success"},
	})
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs/123/jobs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"total_count": 1,
			"jobs":        []map[string]interface{}{{"id": 1, "run_id": 123, "name": "build-job", "status": "completed"}},
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	detail := GetWorkflowRunDetail(ctx, WorkflowRunRequest{OrgName: "testorg", RepoName: "repo1", RunID: 123})

	if detail.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", detail.ErrorMessage)
	}
	if detail.Run.RepositoryName != "repo1" || detail.Run.ID != 123 {
		t.Errorf("unexpected run: %+v", detail.Run)
	}
	if len(detail.Jobs) != 1 || detail.Jobs[0].Name != "build-job" {
		t.Errorf("unexpected jobs: %+v", detail.Jobs)
	}
}

func TestGetWorkflowRunDetail_RunError(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs/123", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	detail := GetWorkflowRunDetail(ctx, WorkflowRunRequest{OrgName: "testorg", RepoName: "repo1", RunID: 123})

	if detail.ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
	if detail.Run.RepositoryName != "repo1" || detail.Run.ID != 123 {
		t.Errorf("unexpected run: %+v", detail.Run)
	}
}

func TestGetWorkflowRunDetail_JobsError(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs/123", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"id": 123, "name": "Build", "status": "completed"},
	})
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs/123/jobs", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	detail := GetWorkflowRunDetail(ctx, WorkflowRunRequest{OrgName: "testorg", RepoName: "repo1", RunID: 123})

	if detail.ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
	if detail.Run.ID != 123 {
		t.Errorf("Run.ID = %d, want 123 (run should still be populated)", detail.Run.ID)
	}
}

func TestCancelWorkflowRun_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs/123/cancel", testutils.MockResponse{
		StatusCode: http.StatusAccepted,
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	run := CancelWorkflowRun(ctx, WorkflowRunRequest{OrgName: "testorg", RepoName: "repo1", RunID: 123})

	if run.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", run.ErrorMessage)
	}
	if run.Status != "cancel_requested" || run.ID != 123 {
		t.Errorf("unexpected run: %+v", run)
	}
}

func TestCancelWorkflowRun_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs/123/cancel", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	run := CancelWorkflowRun(ctx, WorkflowRunRequest{OrgName: "testorg", RepoName: "repo1", RunID: 123})

	if run.ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
	if run.RepositoryName != "repo1" || run.ID != 123 {
		t.Errorf("unexpected run: %+v", run)
	}
}

func gate(id int, name string, canApprove bool) map[string]interface{} {
	return map[string]interface{}{
		"environment":              map[string]interface{}{"id": id, "name": name},
		"current_user_can_approve": canApprove,
	}
}

const pendingPath = "/repos/testorg/repo1/actions/runs/42/pending_deployments"

// lastReview returns the recorded POST to the pending_deployments endpoint, if any.
func lastReview(mockServer *testutils.MockGitHubServer) (testutils.MockRequest, bool) {
	reqs := mockServer.GetRequests()
	for i := len(reqs) - 1; i >= 0; i-- {
		if reqs[i].Method == http.MethodPost && reqs[i].Path == pendingPath {
			return reqs[i], true
		}
	}
	return testutils.MockRequest{}, false
}

func newApproveCtx(t *testing.T, gates ...map[string]interface{}) (*testutils.MockGitHubServer, *context.Context) {
	t.Helper()
	mockServer := testutils.NewMockGitHubServer()
	t.Cleanup(mockServer.Close)
	mockServer.SetResponse(pendingPath, testutils.MockResponse{StatusCode: http.StatusOK, Body: gates})
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true
	return mockServer, ctx
}

func TestApproveWorkflowRun_ApprovesAllReviewableGates(t *testing.T) {
	mockServer, ctx := newApproveCtx(t, gate(1, "approval-1", true), gate(2, "other", false))

	res := ApproveWorkflowRun(ctx, ApproveRequest{OrgName: "testorg", RepoName: "repo1", RunID: 42})

	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}
	if !reflect.DeepEqual(res.Decided, []string{"approval-1"}) || !reflect.DeepEqual(res.Skipped, []string{"other"}) {
		t.Errorf("Decided=%v Skipped=%v", res.Decided, res.Skipped)
	}
	req, ok := lastReview(mockServer)
	if !ok {
		t.Fatal("expected a POST review request")
	}
	if want := `{"environment_ids":[1],"state":"approved","comment":"Approved via sgh"}`; !jsonEqual(req.Body, want) {
		t.Errorf("body = %s, want %s", req.Body, want)
	}
}

func TestApproveWorkflowRun_RejectWithComment(t *testing.T) {
	mockServer, ctx := newApproveCtx(t, gate(1, "approval-1", true))

	res := ApproveWorkflowRun(ctx, ApproveRequest{OrgName: "testorg", RepoName: "repo1", RunID: 42, Reject: true, Comment: "not now"})

	if res.Error != "" || res.State != "rejected" {
		t.Fatalf("res = %+v", res)
	}
	req, _ := lastReview(mockServer)
	if !jsonEqual(req.Body, `{"environment_ids":[1],"state":"rejected","comment":"not now"}`) {
		t.Errorf("body = %s", req.Body)
	}
}

func TestApproveWorkflowRun_EnvironmentFilter(t *testing.T) {
	mockServer, ctx := newApproveCtx(t, gate(1, "approval-1", true), gate(2, "approval-2", true))

	res := ApproveWorkflowRun(ctx, ApproveRequest{OrgName: "testorg", RepoName: "repo1", RunID: 42, Environments: []string{"approval-2"}})

	if !reflect.DeepEqual(res.Decided, []string{"approval-2"}) {
		t.Fatalf("Decided = %v", res.Decided)
	}
	req, _ := lastReview(mockServer)
	if !jsonEqual(req.Body, `{"environment_ids":[2],"state":"approved","comment":"Approved via sgh"}`) {
		t.Errorf("body = %s", req.Body)
	}
}

func TestApproveWorkflowRun_ResolvesLatestWaitingRun(t *testing.T) {
	mockServer, ctx := newApproveCtx(t, gate(1, "approval-1", true))
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"total_count":   1,
			"workflow_runs": []map[string]interface{}{{"id": 42, "status": "waiting"}},
		},
	})

	res := ApproveWorkflowRun(ctx, ApproveRequest{OrgName: "testorg", RepoName: "repo1"})

	if res.Error != "" || res.RunID != 42 {
		t.Fatalf("res = %+v", res)
	}
	if _, ok := lastReview(mockServer); !ok {
		t.Error("expected a POST review request")
	}
}

func TestApproveWorkflowRun_NoWaitingRun(t *testing.T) {
	mockServer, ctx := newApproveCtx(t)
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"total_count": 0, "workflow_runs": []map[string]interface{}{}},
	})

	res := ApproveWorkflowRun(ctx, ApproveRequest{OrgName: "testorg", RepoName: "repo1"})

	if !strings.Contains(res.Error, "no waiting workflow runs") {
		t.Errorf("Error = %q", res.Error)
	}
}

func TestApproveWorkflowRun_NotAReviewer(t *testing.T) {
	mockServer, ctx := newApproveCtx(t, gate(1, "approval-1", false))

	res := ApproveWorkflowRun(ctx, ApproveRequest{OrgName: "testorg", RepoName: "repo1", RunID: 42})

	if !strings.Contains(res.Error, "not a required reviewer") || !reflect.DeepEqual(res.Skipped, []string{"approval-1"}) {
		t.Errorf("res = %+v", res)
	}
	if _, ok := lastReview(mockServer); ok {
		t.Error("must not POST when nothing is reviewable")
	}
}

func TestApproveWorkflowRun_NoPendingGates(t *testing.T) {
	_, ctx := newApproveCtx(t)

	res := ApproveWorkflowRun(ctx, ApproveRequest{OrgName: "testorg", RepoName: "repo1", RunID: 42})

	if !strings.Contains(res.Error, "no matching pending deployments") {
		t.Errorf("Error = %q", res.Error)
	}
}

func TestApproveWorkflowRun_DryRunSendsNothing(t *testing.T) {
	mockServer, ctx := newApproveCtx(t, gate(1, "approval-1", true))
	ctx.DryRun = true

	res := ApproveWorkflowRun(ctx, ApproveRequest{OrgName: "testorg", RepoName: "repo1", RunID: 42})

	if !res.DryRun || !reflect.DeepEqual(res.Decided, []string{"approval-1"}) {
		t.Errorf("res = %+v", res)
	}
	if _, ok := lastReview(mockServer); ok {
		t.Error("dry-run must not POST")
	}
}

func TestApproveWorkflowRun_ConfirmDeclined(t *testing.T) {
	mockServer, ctx := newApproveCtx(t, gate(1, "approval-1", true))

	res := ApproveWorkflowRun(ctx, ApproveRequest{
		OrgName: "testorg", RepoName: "repo1", RunID: 42,
		Confirm: func(ApproveResult) bool { return false },
	})

	if !res.Aborted || len(res.Decided) != 0 {
		t.Errorf("res = %+v", res)
	}
	if _, ok := lastReview(mockServer); ok {
		t.Error("declined confirmation must not POST")
	}
}

func jsonEqual(a, b string) bool {
	var x, y interface{}
	if json.Unmarshal([]byte(a), &x) != nil || json.Unmarshal([]byte(b), &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}

func detailMock(t *testing.T) *testutils.MockGitHubServer {
	t.Helper()
	mockServer := testutils.NewMockGitHubServer()
	t.Cleanup(mockServer.Close)
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs/123", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"id": 123, "name": "Build", "status": "completed"},
	})
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs/123/jobs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"total_count": 0, "jobs": []map[string]interface{}{}},
	})
	return mockServer
}

func TestGetWorkflowRunDetail_IncludesApprovalsChronologically(t *testing.T) {
	mockServer := detailMock(t)
	// The API returns newest first.
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs/123/approvals", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: []map[string]interface{}{
			{"state": "rejected", "comment": "second", "environments": []map[string]interface{}{{"id": 2, "name": "approval-2"}}},
			{"state": "approved", "comment": "first", "environments": []map[string]interface{}{{"id": 1, "name": "approval-1"}}},
		},
	})
	ctx := servicetest.NewMockContext(t, mockServer)

	detail := GetWorkflowRunDetail(ctx, WorkflowRunRequest{OrgName: "testorg", RepoName: "repo1", RunID: 123})

	if detail.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", detail.ErrorMessage)
	}
	if len(detail.Approvals) != 2 || detail.Approvals[0].Comment != "first" || detail.Approvals[1].Comment != "second" {
		t.Errorf("approvals = %+v, want [first, second]", detail.Approvals)
	}
}

func TestGetWorkflowRunDetail_ApprovalsErrorIsNotFatal(t *testing.T) {
	mockServer := detailMock(t)
	mockServer.SetResponse("/repos/testorg/repo1/actions/runs/123/approvals", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "Forbidden"},
	})
	ctx := servicetest.NewMockContext(t, mockServer)

	detail := GetWorkflowRunDetail(ctx, WorkflowRunRequest{OrgName: "testorg", RepoName: "repo1", RunID: 123})

	if detail.ErrorMessage != "" || len(detail.Approvals) != 0 {
		t.Errorf("want run without approvals and no error, got %+v", detail)
	}
}
