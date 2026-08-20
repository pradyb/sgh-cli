// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package workflow

import (
	"net/http"
	"testing"

	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

func TestDispatchWorkflow_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/actions/workflows/build.yml/dispatches", testutils.MockResponse{
		StatusCode: http.StatusNoContent,
	})

	ctx := service.NewMockContext(t, mockServer)
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

	ctx := service.NewMockContext(t, mockServer)
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

	ctx := service.NewMockContext(t, mockServer)
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

	ctx := service.NewMockContext(t, mockServer)
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

	ctx := service.NewMockContext(t, mockServer)
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

	ctx := service.NewMockContext(t, mockServer)

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

	ctx := service.NewMockContext(t, mockServer)

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

	ctx := service.NewMockContext(t, mockServer)

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

	ctx := service.NewMockContext(t, mockServer)

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

	ctx := service.NewMockContext(t, mockServer)

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

	ctx := service.NewMockContext(t, mockServer)

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

	ctx := service.NewMockContext(t, mockServer)

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

	ctx := service.NewMockContext(t, mockServer)

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

	ctx := service.NewMockContext(t, mockServer)

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

	ctx := service.NewMockContext(t, mockServer)

	run := CancelWorkflowRun(ctx, WorkflowRunRequest{OrgName: "testorg", RepoName: "repo1", RunID: 123})

	if run.ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
	if run.RepositoryName != "repo1" || run.ID != 123 {
		t.Errorf("unexpected run: %+v", run)
	}
}
