// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package workflow

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/pradyb/sgh-cli/internal/model"
	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
	"github.com/pradyb/sgh-cli/pkg/workflow"
)

// newTestRoot builds a minimal fake parent command that only defines the
// persistent flags the workflow subcommands actually read. It intentionally
// has no PersistentPreRun/PersistentPostRun (unlike the real cmd/root.go),
// so it never calls os.Exit on an error path during tests.
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

func workflowRunsBody() map[string]interface{} {
	return map[string]interface{}{
		"total_count": 2,
		"workflow_runs": []map[string]interface{}{
			{"id": 1, "name": "Build", "status": "completed", "conclusion": "success", "head_branch": "main"},
			{"id": 2, "name": "Deploy", "status": "in_progress", "conclusion": "", "head_branch": "main"},
		},
	}
}

func TestNewWorkflowCommand_Structure(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)

	cmd := NewWorkflowCommand(ctx)
	if cmd.Use != "workflow <command>" {
		t.Errorf("Use = %q", cmd.Use)
	}

	want := map[string]bool{"list": false, "view": false, "rerun": false, "cancel": false, "dispatch": false, "approve": false}
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

func TestListCommand_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/actions/runs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       workflowRunsBody(),
	})
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	if err := execCmd(ListCommand(ctx), "list", "--org", "acme", "-r", "repo1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListCommand_QuickFilters(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/actions/runs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       workflowRunsBody(),
	})
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	tests := [][]string{
		{"list", "--org", "acme", "-r", "repo1", "--running"},
		{"list", "--org", "acme", "-r", "repo1", "--queued"},
		{"list", "--org", "acme", "-r", "repo1", "--failed"},
		{"list", "--org", "acme", "-r", "repo1", "--status", "success"},
		{"list", "--org", "acme", "-r", "repo1", "--sort", "created"},
		{"list", "--org", "acme", "-r", "repo1", "--branch", "main", "--last", "5"},
		{"list", "--org", "acme", "-r", "repo1", "--workflow", "Build"},
		{"list", "--org", "acme", "-e", "repo2"},
	}
	for _, args := range tests {
		if err := execCmd(ListCommand(ctx), args...); err != nil {
			t.Errorf("args %v: unexpected error: %v", args, err)
		}
	}
}

func TestListCommand_JSONOutput(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/actions/runs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       workflowRunsBody(),
	})
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true
	ctx.JSON = true
	ctx.Limit = 1

	if err := execCmd(ListCommand(ctx), "list", "--org", "acme", "-r", "repo1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListCommand_MutuallyExclusiveFlags(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)

	err := execCmd(ListCommand(ctx), "list", "--org", "acme", "--running", "--failed")
	if err == nil {
		t.Fatal("expected an error for mutually exclusive flags")
	}
}

func TestViewCommand_ExplicitRun(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/actions/runs/123", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"id": 123, "name": "Build", "status": "completed", "conclusion": "success"},
	})
	mockServer.SetResponse("/repos/acme/repo1/actions/runs/123/jobs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"total_count": 1,
			"jobs":        []map[string]interface{}{{"id": 1, "run_id": 123, "name": "build-job", "status": "completed"}},
		},
	})
	ctx := servicetest.NewMockContext(t, mockServer)

	if err := execCmd(ViewCommand(ctx), "view", "--org", "acme", "-r", "repo1", "--run", "123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViewCommand_LatestRun(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/actions/runs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"total_count":   1,
			"workflow_runs": []map[string]interface{}{{"id": 555, "name": "Build"}},
		},
	})
	mockServer.SetResponse("/repos/acme/repo1/actions/runs/555", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"id": 555, "name": "Build", "status": "completed", "conclusion": "success"},
	})
	mockServer.SetResponse("/repos/acme/repo1/actions/runs/555/jobs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"total_count": 0, "jobs": []map[string]interface{}{}},
	})
	ctx := servicetest.NewMockContext(t, mockServer)

	if err := execCmd(ViewCommand(ctx), "view", "--org", "acme", "-r", "repo1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViewCommand_LatestRunError(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/actions/runs", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})
	ctx := servicetest.NewMockContext(t, mockServer)

	// GetLatestRunID fails; the command should just log and return, not crash.
	if err := execCmd(ViewCommand(ctx), "view", "--org", "acme", "-r", "repo1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViewCommand_WatchFlagOnCompletedRun(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/actions/runs/123", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"id": 123, "name": "Build", "status": "completed", "conclusion": "success"},
	})
	mockServer.SetResponse("/repos/acme/repo1/actions/runs/123/jobs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"total_count": 0, "jobs": []map[string]interface{}{}},
	})
	ctx := servicetest.NewMockContext(t, mockServer)

	// Since the run is already completed, --watch should take the
	// non-watch print path and never enter the bubbletea program.
	if err := execCmd(ViewCommand(ctx), "view", "--org", "acme", "-r", "repo1", "--run", "123", "--watch"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViewCommand_MissingRepository(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)

	if err := execCmd(ViewCommand(ctx), "view", "--org", "acme"); err == nil {
		t.Fatal("expected an error for missing required --repository flag")
	}
}

func TestRerunCommand_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/actions/runs/123/rerun", testutils.MockResponse{
		StatusCode: http.StatusCreated,
	})
	ctx := servicetest.NewMockContext(t, mockServer)

	if err := execCmd(rerunCommand(ctx), "rerun", "--org", "acme", "-r", "repo1", "--run", "123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRerunCommand_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/actions/runs/123/rerun", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})
	ctx := servicetest.NewMockContext(t, mockServer)

	if err := execCmd(rerunCommand(ctx), "rerun", "--org", "acme", "-r", "repo1", "--run", "123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRerunCommand_DryRun(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.DryRun = true

	if err := execCmd(rerunCommand(ctx), "rerun", "--org", "acme", "-r", "repo1", "--run", "123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests in dry-run mode, got %d", len(mockServer.GetRequests()))
	}
}

func TestRerunCommand_MissingRequiredFlags(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)

	if err := execCmd(rerunCommand(ctx), "rerun", "--org", "acme"); err == nil {
		t.Fatal("expected an error for missing required --repository/--run flags")
	}
}

func TestCancelCommand_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/actions/runs/123/cancel", testutils.MockResponse{
		StatusCode: http.StatusAccepted,
	})
	ctx := servicetest.NewMockContext(t, mockServer)

	if err := execCmd(cancelCommand(ctx), "cancel", "--org", "acme", "-r", "repo1", "--run", "123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCancelCommand_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/actions/runs/123/cancel", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})
	ctx := servicetest.NewMockContext(t, mockServer)

	if err := execCmd(cancelCommand(ctx), "cancel", "--org", "acme", "-r", "repo1", "--run", "123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCancelCommand_DryRun(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.DryRun = true

	if err := execCmd(cancelCommand(ctx), "cancel", "--org", "acme", "-r", "repo1", "--run", "123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests in dry-run mode, got %d", len(mockServer.GetRequests()))
	}
}

func TestDispatchCommand_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/actions/workflows/build.yml/dispatches", testutils.MockResponse{
		StatusCode: http.StatusNoContent,
	})
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	err := execCmd(dispatchCommand(ctx), "dispatch", "--org", "acme", "-r", "repo1",
		"--workflow", "build.yml", "--ref", "main", "--input", "env=prod", "--input", "dry_run=false")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDispatchCommand_MultiRepoMixedResults(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/actions/workflows/build.yml/dispatches", testutils.MockResponse{
		StatusCode: http.StatusNoContent,
	})
	mockServer.SetResponse("/repos/acme/repo2/actions/workflows/build.yml/dispatches", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	err := execCmd(dispatchCommand(ctx), "dispatch", "--org", "acme", "-r", "repo1", "-r", "repo2",
		"--workflow", "build.yml", "--ref", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDispatchCommand_DryRun(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.DryRun = true

	err := execCmd(dispatchCommand(ctx), "dispatch", "--org", "acme", "-r", "repo1",
		"--workflow", "build.yml", "--ref", "main", "--input", "env=prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests in dry-run mode, got %d", len(mockServer.GetRequests()))
	}
}

func TestDispatchCommand_MissingRequiredFlags(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)

	if err := execCmd(dispatchCommand(ctx), "dispatch", "--org", "acme"); err == nil {
		t.Fatal("expected an error for missing required --workflow/--ref flags")
	}
}

func TestDispatchCommand_ExportedWrapper(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/actions/workflows/build.yml/dispatches", testutils.MockResponse{
		StatusCode: http.StatusNoContent,
	})
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	err := execCmd(DispatchCommand(ctx), "dispatch", "--org", "acme", "-r", "repo1", "--workflow", "build.yml", "--ref", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRepoCompletionFn(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)

	root := newTestRoot()
	if err := root.PersistentFlags().Set("org", "acme"); err != nil {
		t.Fatalf("failed to set org flag: %v", err)
	}

	fn := repoCompletionFn(ctx)
	names, directive := fn(root, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
	}
	// No repositories configured in the isolated test home, so this should
	// resolve to an empty (but non-nil-panic) slice rather than erroring.
	if names == nil {
		t.Log("names is nil, which is fine for an unconfigured org")
	}
}

// --- watch model unit tests ---

func completedDetail() model.WorkflowRunDetail {
	return model.WorkflowRunDetail{Run: model.WorkflowRun{ID: 123, Name: "Build", Status: "completed", Conclusion: "success"}}
}

func inProgressDetail() model.WorkflowRunDetail {
	return model.WorkflowRunDetail{Run: model.WorkflowRun{ID: 123, Name: "Build", Status: "in_progress"}}
}

func TestNewWatchModel(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)
	req := workflow.WorkflowRunRequest{OrgName: "acme", RepoName: "repo1", RunID: 123}

	m := newWatchModel(ctx, req, 10*time.Second, inProgressDetail())
	if m.loading {
		t.Error("expected loading to start false")
	}
	if m.detail.Run.ID != 123 {
		t.Errorf("detail.Run.ID = %d, want 123", m.detail.Run.ID)
	}
}

func TestWatchModel_Init(t *testing.T) {
	m := newWatchModel(nil, workflow.WorkflowRunRequest{}, time.Millisecond, inProgressDetail())
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected a non-nil tea.Cmd from Init")
	}
	msg := cmd()
	if _, ok := msg.(watchTickMsg); !ok {
		t.Errorf("Init() cmd produced %T, want watchTickMsg", msg)
	}
}

func TestWatchModel_Update_DataMsgInProgress(t *testing.T) {
	m := newWatchModel(nil, workflow.WorkflowRunRequest{}, time.Millisecond, completedDetail())
	next, cmd := m.Update(watchDataMsg{detail: inProgressDetail()})
	nm := next.(watchModel)
	if nm.loading {
		t.Error("expected loading to be false after data msg")
	}
	if nm.done {
		t.Error("expected done to stay false while run is in progress")
	}
	if cmd == nil {
		t.Fatal("expected a re-tick command while in progress")
	}
	if _, ok := cmd().(watchTickMsg); !ok {
		t.Error("expected the re-tick command to produce watchTickMsg")
	}
}

func TestWatchModel_Update_DataMsgCompleted(t *testing.T) {
	m := newWatchModel(nil, workflow.WorkflowRunRequest{}, time.Millisecond, inProgressDetail())
	next, cmd := m.Update(watchDataMsg{detail: completedDetail()})
	nm := next.(watchModel)
	if !nm.done {
		t.Error("expected done to be true once the run completes")
	}
	if cmd == nil {
		t.Fatal("expected a tea.Quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", cmd())
	}
}

func TestWatchModel_Update_Tick(t *testing.T) {
	m := newWatchModel(nil, workflow.WorkflowRunRequest{}, time.Millisecond, inProgressDetail())
	next, cmd := m.Update(watchTickMsg{})
	nm := next.(watchModel)
	if !nm.loading {
		t.Error("expected loading to become true on tick")
	}
	if cmd == nil {
		t.Fatal("expected fetchDetail command on tick")
	}
}

func TestWatchModel_Update_KeyQuit(t *testing.T) {
	for _, key := range []string{"q", "ctrl+c"} {
		m := newWatchModel(nil, workflow.WorkflowRunRequest{}, time.Millisecond, inProgressDetail())
		next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		if key == "ctrl+c" {
			next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		}
		nm := next.(watchModel)
		if !nm.done {
			t.Errorf("key %q: expected done to be true", key)
		}
		if cmd == nil || cmd() == nil {
			t.Errorf("key %q: expected a quit command", key)
		}
	}
}

func TestWatchModel_Update_KeyRefresh(t *testing.T) {
	m := newWatchModel(nil, workflow.WorkflowRunRequest{}, time.Millisecond, inProgressDetail())
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	nm := next.(watchModel)
	if !nm.loading {
		t.Error("expected loading to become true on refresh key")
	}
	if cmd == nil {
		t.Fatal("expected fetchDetail command on refresh")
	}
}

func TestWatchModel_Update_UnhandledMsg(t *testing.T) {
	m := newWatchModel(nil, workflow.WorkflowRunRequest{}, time.Millisecond, inProgressDetail())
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if _, ok := next.(watchModel); !ok {
		t.Fatalf("expected watchModel, got %T", next)
	}
	if cmd != nil {
		t.Error("expected a nil command for an unhandled message")
	}
}

func TestWatchModel_View_Loading(t *testing.T) {
	m := watchModel{loading: true, detail: model.WorkflowRunDetail{}}
	out := m.View()
	if out == "" {
		t.Fatal("expected non-empty loading view")
	}
}

func TestWatchModel_View_InProgressShowsHint(t *testing.T) {
	m := newWatchModel(nil, workflow.WorkflowRunRequest{}, 10*time.Second, inProgressDetail())
	out := m.View()
	if out == "" {
		t.Fatal("expected non-empty view")
	}
}

func TestWatchModel_View_Done(t *testing.T) {
	m := newWatchModel(nil, workflow.WorkflowRunRequest{}, 10*time.Second, completedDetail())
	m.done = true
	out := m.View()
	if out == "" {
		t.Fatal("expected non-empty view")
	}
}

func TestWatchModel_FetchDetail(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/actions/runs/123", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"id": 123, "name": "Build", "status": "completed", "conclusion": "success"},
	})
	mockServer.SetResponse("/repos/acme/repo1/actions/runs/123/jobs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"total_count": 0, "jobs": []map[string]interface{}{}},
	})
	ctx := servicetest.NewMockContext(t, mockServer)
	req := workflow.WorkflowRunRequest{OrgName: "acme", RepoName: "repo1", RunID: 123}

	m := newWatchModel(ctx, req, time.Millisecond, model.WorkflowRunDetail{})
	msg := m.fetchDetail()
	dataMsg, ok := msg.(watchDataMsg)
	if !ok {
		t.Fatalf("fetchDetail() = %T, want watchDataMsg", msg)
	}
	if dataMsg.detail.Run.ID != 123 {
		t.Errorf("detail.Run.ID = %d, want 123", dataMsg.detail.Run.ID)
	}
}

const approvePendingPath = "/repos/acme/repo1/actions/runs/123/pending_deployments"

func approveServer(t *testing.T, canApprove bool) *testutils.MockGitHubServer {
	t.Helper()
	mockServer := testutils.NewMockGitHubServer()
	t.Cleanup(mockServer.Close)
	mockServer.SetResponse(approvePendingPath, testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: []map[string]interface{}{{
			"environment":              map[string]interface{}{"id": 9, "name": "approval-1"},
			"current_user_can_approve": canApprove,
		}},
	})
	return mockServer
}

func postCount(mockServer *testutils.MockGitHubServer) int {
	n := 0
	for _, r := range mockServer.GetRequests() {
		if r.Method == http.MethodPost && r.Path == approvePendingPath {
			n++
		}
	}
	return n
}

// execCmdWithStdin is execCmd with a scripted stdin, for confirmation prompts.
func execCmdWithStdin(cmd *cobra.Command, stdin string, args ...string) error {
	root := newTestRoot()
	root.AddCommand(cmd)
	root.SetArgs(args)
	root.SetIn(strings.NewReader(stdin))
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	return root.Execute()
}

func TestApproveCommand_YesSkipsPrompt(t *testing.T) {
	mockServer := approveServer(t, true)
	ctx := servicetest.NewMockContext(t, mockServer)

	if err := execCmd(approveCommand(ctx), "approve", "--org", "acme", "-r", "repo1", "--run", "123", "--yes"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if postCount(mockServer) != 1 || ctx.HasError {
		t.Errorf("posts=%d HasError=%v, want 1 post and no error", postCount(mockServer), ctx.HasError)
	}
}

func TestApproveCommand_PromptAccepted(t *testing.T) {
	mockServer := approveServer(t, true)
	ctx := servicetest.NewMockContext(t, mockServer)

	if err := execCmdWithStdin(approveCommand(ctx), "yes\n", "approve", "--org", "acme", "-r", "repo1", "--run", "123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if postCount(mockServer) != 1 {
		t.Errorf("posts = %d, want 1", postCount(mockServer))
	}
}

func TestApproveCommand_PromptDeclinedOrEOF(t *testing.T) {
	for name, stdin := range map[string]string{"no": "no\n", "eof": ""} {
		t.Run(name, func(t *testing.T) {
			mockServer := approveServer(t, true)
			ctx := servicetest.NewMockContext(t, mockServer)

			if err := execCmdWithStdin(approveCommand(ctx), stdin, "approve", "--org", "acme", "-r", "repo1", "--run", "123"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if postCount(mockServer) != 0 {
				t.Errorf("posts = %d, want 0 without confirmation", postCount(mockServer))
			}
		})
	}
}

func TestApproveCommand_Reject(t *testing.T) {
	mockServer := approveServer(t, true)
	ctx := servicetest.NewMockContext(t, mockServer)

	if err := execCmd(approveCommand(ctx), "approve", "--org", "acme", "-r", "repo1", "--run", "123", "--yes", "--reject", "--comment", "not now"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range mockServer.GetRequests() {
		if r.Method == http.MethodPost && !strings.Contains(r.Body, `"state":"rejected"`) {
			t.Errorf("body = %s, want rejected state", r.Body)
		}
	}
}

func TestApproveCommand_NotAReviewerSetsError(t *testing.T) {
	mockServer := approveServer(t, false)
	ctx := servicetest.NewMockContext(t, mockServer)

	if err := execCmd(approveCommand(ctx), "approve", "--org", "acme", "-r", "repo1", "--run", "123", "--yes"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if postCount(mockServer) != 0 || !ctx.HasError {
		t.Errorf("posts=%d HasError=%v, want 0 posts and HasError", postCount(mockServer), ctx.HasError)
	}
}

func TestApproveCommand_DryRun(t *testing.T) {
	mockServer := approveServer(t, true)
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.DryRun = true

	if err := execCmd(approveCommand(ctx), "approve", "--org", "acme", "-r", "repo1", "--run", "123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if postCount(mockServer) != 0 {
		t.Errorf("posts = %d, want 0 in dry-run", postCount(mockServer))
	}
}

func TestApproveCommand_JSONRequiresYes(t *testing.T) {
	mockServer := approveServer(t, true)
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.JSON = true

	if err := execCmd(approveCommand(ctx), "approve", "--org", "acme", "-r", "repo1", "--run", "123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if postCount(mockServer) != 0 || !ctx.HasError {
		t.Errorf("posts=%d HasError=%v, want 0 posts and HasError", postCount(mockServer), ctx.HasError)
	}
}

func TestApproveCommand_RunNeedsSingleRepo(t *testing.T) {
	mockServer := approveServer(t, true)
	ctx := servicetest.NewMockContext(t, mockServer)

	if err := execCmd(approveCommand(ctx), "approve", "--org", "acme", "-r", "repo1", "-r", "repo2", "--run", "123", "--yes"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.HasError || len(mockServer.GetRequests()) != 0 {
		t.Errorf("HasError=%v requests=%d, want error and no requests", ctx.HasError, len(mockServer.GetRequests()))
	}
}

func TestApproveCommand_MissingRepository(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)

	if err := execCmd(approveCommand(ctx), "approve", "--org", "acme"); err == nil {
		t.Fatal("expected an error for missing required --repository flag")
	}
}

func TestParseInputPairs(t *testing.T) {
	got, err := parseInputPairs([]string{"env=prod", "empty=", "url=a=b=c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{"env": "prod", "empty": "", "url": "a=b=c"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if gv, ok := got[k]; !ok || gv != v {
			t.Errorf("input %q = %q (present=%v), want %q", k, gv, ok, v)
		}
	}

	if got, err := parseInputPairs(nil); err != nil || len(got) != 0 {
		t.Errorf("nil pairs: got %v, %v; want empty map, nil", got, err)
	}

	for _, bad := range []string{"target", "dry_run", "env:prod", "=x", "="} {
		if _, err := parseInputPairs([]string{"ok=1", bad}); err == nil {
			t.Errorf("parseInputPairs(%q) expected an error", bad)
		}
	}
}

// A malformed --input must abort before any API call, so a multi-repo dispatch never runs partially
// and the workflow never runs with the intended input silently missing.
func TestDispatchCommand_InvalidInputSendsNothing(t *testing.T) {
	for _, bad := range []string{"target", "=x"} {
		t.Run(bad, func(t *testing.T) {
			mockServer := testutils.NewMockGitHubServer()
			defer mockServer.Close()
			ctx := servicetest.NewMockContext(t, mockServer)
			ctx.Silent = true

			err := execCmd(dispatchCommand(ctx), "dispatch", "--org", "acme", "-r", "repo1", "-r", "repo2",
				"--workflow", "build.yml", "--ref", "main", "--input", "env=prod", "--input", bad)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(mockServer.GetRequests()) != 0 {
				t.Errorf("expected no network requests, got %d", len(mockServer.GetRequests()))
			}
			if !ctx.HasError {
				t.Error("expected ctx.HasError so the process exits non-zero")
			}
		})
	}
}

func TestDispatchCommand_InvalidInputAbortsDryRun(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.DryRun = true

	if err := execCmd(dispatchCommand(ctx), "dispatch", "--org", "acme", "-r", "repo1",
		"--workflow", "build.yml", "--ref", "main", "--input", "target"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.HasError {
		t.Error("dry-run must also reject a malformed --input")
	}
}

// captureStdout redirects os.Stdout while fn runs and returns everything written to it.
// ui.PrintJSON writes directly to os.Stdout (not cmd.OutOrStdout()), so this is the only way
// to assert on what a --json run actually prints.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	original := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	func() {
		defer func() {
			_ = w.Close()
			os.Stdout = original
		}()
		fn()
	}()
	out := <-done
	_ = r.Close()
	return out
}

func TestViewCommand_JSONOutput(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/actions/runs/123", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"id": 123, "name": "Build", "status": "completed", "conclusion": "success"},
	})
	mockServer.SetResponse("/repos/acme/repo1/actions/runs/123/jobs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"total_count": 0, "jobs": []map[string]interface{}{}},
	})
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true
	ctx.JSON = true

	out := captureStdout(t, func() {
		if err := execCmd(ViewCommand(ctx), "view", "--org", "acme", "-r", "repo1", "--run", "123"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var detail model.WorkflowRunDetail
	if err := json.Unmarshal([]byte(out), &detail); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %s", err, out)
	}
	if detail.Run.ID != 123 || detail.Run.Name != "Build" {
		t.Errorf("decoded run = %+v", detail.Run)
	}
	if ctx.HasError {
		t.Error("HasError should be false for a successful lookup")
	}
}

func TestViewCommand_JSONOutput_LatestRunNoticeOnStderr(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/actions/runs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"total_count":   1,
			"workflow_runs": []map[string]interface{}{{"id": 555, "name": "Build"}},
		},
	})
	mockServer.SetResponse("/repos/acme/repo1/actions/runs/555", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"id": 555, "name": "Build", "status": "completed"},
	})
	mockServer.SetResponse("/repos/acme/repo1/actions/runs/555/jobs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"total_count": 0, "jobs": []map[string]interface{}{}},
	})
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true
	ctx.JSON = true

	out := captureStdout(t, func() {
		if err := execCmd(ViewCommand(ctx), "view", "--org", "acme", "-r", "repo1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if strings.Contains(out, "Using latest workflow run") {
		t.Errorf("stdout must not contain the informational notice, got %q", out)
	}
	var detail model.WorkflowRunDetail
	if err := json.Unmarshal([]byte(out), &detail); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %s", err, out)
	}
}

func TestViewCommand_JSONOutput_ErrorSetsHasError(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/actions/runs/123", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true
	ctx.JSON = true

	captureStdout(t, func() {
		if err := execCmd(ViewCommand(ctx), "view", "--org", "acme", "-r", "repo1", "--run", "123"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !ctx.HasError {
		t.Error("expected HasError to be set when the run lookup fails")
	}
}

// withNoViewWatchSleep replaces viewWatchSleep with a no-op for one test, so a
// `view --watch --json` test needing several poll iterations doesn't really wait between them.
func withNoViewWatchSleep(t *testing.T) {
	t.Helper()
	orig := viewWatchSleep
	viewWatchSleep = func(time.Duration) {}
	t.Cleanup(func() { viewWatchSleep = orig })
}

func TestViewCommand_WatchJSON_StreamsNDJSONUntilRunCompletes(t *testing.T) {
	withNoViewWatchSleep(t)
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponseSequence("/repos/acme/repo1/actions/runs/123", []testutils.MockResponse{
		{StatusCode: http.StatusOK, Body: map[string]interface{}{"id": 123, "status": "in_progress"}},
		{StatusCode: http.StatusOK, Body: map[string]interface{}{"id": 123, "status": "completed", "conclusion": "success"}},
	})
	mockServer.SetResponseSequence("/repos/acme/repo1/actions/runs/123/jobs", []testutils.MockResponse{
		{StatusCode: http.StatusOK, Body: map[string]interface{}{
			"total_count": 1,
			"jobs":        []map[string]interface{}{{"id": 1, "run_id": 123, "name": "build-job", "status": "in_progress"}},
		}},
		{StatusCode: http.StatusOK, Body: map[string]interface{}{
			"total_count": 1,
			"jobs":        []map[string]interface{}{{"id": 1, "run_id": 123, "name": "build-job", "status": "completed", "conclusion": "success"}},
		}},
	})
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.JSON = true

	out := captureStdout(t, func() {
		if err := execCmd(ViewCommand(ctx), "view", "--org", "acme", "-r", "repo1", "--run", "123", "--watch"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (job_started, job_completed, run_done): %q", len(lines), out)
	}
	wantKinds := []string{"job_started", "job_completed", "run_done"}
	for i, line := range lines {
		var e workflow.ViewWatchEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("line %d not valid JSON: %v (%q)", i, err, line)
		}
		if e.Kind != wantKinds[i] {
			t.Errorf("line %d kind = %q, want %q", i, e.Kind, wantKinds[i])
		}
	}
	if ctx.HasError {
		t.Error("expected HasError to stay false on a clean run")
	}
}

func TestViewCommand_WatchJSON_RunLookupErrorSetsHasError(t *testing.T) {
	withNoViewWatchSleep(t)
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/actions/runs/123", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.JSON = true

	captureStdout(t, func() {
		if err := execCmd(ViewCommand(ctx), "view", "--org", "acme", "-r", "repo1", "--run", "123", "--watch"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !ctx.HasError {
		t.Error("expected HasError when the initial run lookup fails")
	}
}

// --- workflow approve --watch ---

const approveRunPath = "/repos/acme/repo1/actions/runs/123"

// withStdinTerminal forces isStdinTerminal for the duration of one test.
func withStdinTerminal(t *testing.T, terminal bool) {
	t.Helper()
	orig := isStdinTerminal
	isStdinTerminal = func() bool { return terminal }
	t.Cleanup(func() { isStdinTerminal = orig })
}

// withNoWatchSleep replaces watchSleep with a no-op for one test, so a --watch test
// needing several poll iterations doesn't really wait between them.
func withNoWatchSleep(t *testing.T) {
	t.Helper()
	orig := watchSleep
	watchSleep = func(time.Duration) {}
	t.Cleanup(func() { watchSleep = orig })
}

func TestApproveCommand_Watch_RequiresSingleRepo(t *testing.T) {
	mockServer := approveServer(t, true)
	ctx := servicetest.NewMockContext(t, mockServer)

	err := execCmd(approveCommand(ctx), "approve", "--org", "acme", "-r", "repo1", "-r", "repo2", "--watch", "--yes", "-e", "approval-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.HasError || len(mockServer.GetRequests()) != 0 {
		t.Errorf("HasError=%v requests=%d, want an error and no requests", ctx.HasError, len(mockServer.GetRequests()))
	}
}

func TestApproveCommand_Watch_YesRequiresEnvironment(t *testing.T) {
	mockServer := approveServer(t, true)
	ctx := servicetest.NewMockContext(t, mockServer)

	err := execCmd(approveCommand(ctx), "approve", "--org", "acme", "-r", "repo1", "--run", "123", "--watch", "--yes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.HasError || len(mockServer.GetRequests()) != 0 {
		t.Errorf("HasError=%v requests=%d, want an error and no requests", ctx.HasError, len(mockServer.GetRequests()))
	}
}

func TestApproveCommand_Watch_MinimumInterval(t *testing.T) {
	mockServer := approveServer(t, true)
	ctx := servicetest.NewMockContext(t, mockServer)

	err := execCmd(approveCommand(ctx), "approve", "--org", "acme", "-r", "repo1", "--run", "123", "--watch", "--yes", "-e", "approval-1", "--interval", "2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.HasError || len(mockServer.GetRequests()) != 0 {
		t.Errorf("HasError=%v requests=%d, want an error and no requests", ctx.HasError, len(mockServer.GetRequests()))
	}
}

func TestApproveCommand_Watch_NonTTYWithoutYes(t *testing.T) {
	withStdinTerminal(t, false)
	mockServer := approveServer(t, true)
	ctx := servicetest.NewMockContext(t, mockServer)

	err := execCmd(approveCommand(ctx), "approve", "--org", "acme", "-r", "repo1", "--run", "123", "--watch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.HasError || len(mockServer.GetRequests()) != 0 {
		t.Errorf("HasError=%v requests=%d, want an error and no requests", ctx.HasError, len(mockServer.GetRequests()))
	}
}

func TestApproveCommand_Watch_YesEndToEnd(t *testing.T) {
	withNoWatchSleep(t)
	mockServer := approveServer(t, true)
	mockServer.SetResponseSequence(approveRunPath, []testutils.MockResponse{
		{StatusCode: http.StatusOK, Body: map[string]interface{}{"id": 123, "status": "waiting"}},
		{StatusCode: http.StatusOK, Body: map[string]interface{}{"id": 123, "status": "completed", "conclusion": "success"}},
	})
	ctx := servicetest.NewMockContext(t, mockServer)

	out := captureStdout(t, func() {
		err := execCmd(approveCommand(ctx), "approve", "--org", "acme", "-r", "repo1", "--run", "123", "--watch", "--yes", "-e", "approval-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if ctx.HasError {
		t.Errorf("unexpected HasError; output: %s", out)
	}
	if !strings.Contains(out, "New gate: approval-1") || !strings.Contains(out, "Run completed: success") {
		t.Errorf("output missing expected lines, got: %s", out)
	}
	if postCount(mockServer) != 1 {
		t.Errorf("posts = %d, want 1", postCount(mockServer))
	}
}

func TestApproveCommand_Watch_InteractivePromptAccepted(t *testing.T) {
	withStdinTerminal(t, true)
	withNoWatchSleep(t)
	mockServer := approveServer(t, true)
	mockServer.SetResponseSequence(approveRunPath, []testutils.MockResponse{
		{StatusCode: http.StatusOK, Body: map[string]interface{}{"id": 123, "status": "waiting"}},
		{StatusCode: http.StatusOK, Body: map[string]interface{}{"id": 123, "status": "completed", "conclusion": "success"}},
	})
	ctx := servicetest.NewMockContext(t, mockServer)

	err := execCmdWithStdin(approveCommand(ctx), "yes\n", "approve", "--org", "acme", "-r", "repo1", "--run", "123", "--watch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if postCount(mockServer) != 1 {
		t.Errorf("posts = %d, want 1", postCount(mockServer))
	}
}

func TestApproveCommand_Watch_InteractivePromptDeclinedThenTimesOut(t *testing.T) {
	withStdinTerminal(t, true)
	withNoWatchSleep(t) // no real delay between polls; --timeout itself is still real wall-clock time
	mockServer := approveServer(t, true)
	mockServer.SetResponse(approveRunPath, testutils.MockResponse{
		StatusCode: http.StatusOK, Body: map[string]interface{}{"id": 123, "status": "waiting"},
	})
	ctx := servicetest.NewMockContext(t, mockServer)

	err := execCmdWithStdin(approveCommand(ctx), "no\n", "approve", "--org", "acme", "-r", "repo1", "--run", "123", "--watch", "--timeout", "50ms")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.HasError {
		t.Error("expected HasError after the watch times out")
	}
	if postCount(mockServer) != 0 {
		t.Errorf("posts = %d, want 0 — the gate was declined, never approved", postCount(mockServer))
	}
}

func TestApproveCommand_Watch_JSONOutput(t *testing.T) {
	withNoWatchSleep(t)
	mockServer := approveServer(t, true)
	mockServer.SetResponseSequence(approveRunPath, []testutils.MockResponse{
		{StatusCode: http.StatusOK, Body: map[string]interface{}{"id": 123, "status": "waiting"}},
		{StatusCode: http.StatusOK, Body: map[string]interface{}{"id": 123, "status": "completed", "conclusion": "success"}},
	})
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.JSON = true

	out := captureStdout(t, func() {
		err := execCmd(approveCommand(ctx), "approve", "--org", "acme", "-r", "repo1", "--run", "123", "--watch", "--yes", "-e", "approval-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 NDJSON lines (watch_started, gate_decided, run_done), got %d: %s", len(lines), out)
	}
	for _, line := range lines {
		var e map[string]interface{}
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Errorf("line not valid JSON: %s (%v)", line, err)
		}
	}
	if lines[0] != `{"kind":"watch_started","run_id":123}` ||
		!strings.Contains(lines[1], `"gate_decided"`) || !strings.Contains(lines[2], `"run_done"`) {
		t.Errorf("lines = %v, want watch_started then gate_decided then run_done", lines)
	}
}

func TestApproveCommand_Watch_ShowsProgressAndResolvedRunID(t *testing.T) {
	withStdinTerminal(t, true)
	withNoWatchSleep(t)
	mockServer := approveServer(t, true)
	mockServer.SetResponse("/repos/acme/repo1/actions/runs", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"total_count":   1,
			"workflow_runs": []map[string]interface{}{{"id": 123, "status": "waiting"}},
		},
	})
	mockServer.SetResponseSequence(approveRunPath, []testutils.MockResponse{
		{StatusCode: http.StatusOK, Body: map[string]interface{}{"id": 123, "status": "waiting"}},
		{StatusCode: http.StatusOK, Body: map[string]interface{}{"id": 123, "status": "completed", "conclusion": "success"}},
	})
	mockServer.SetResponseSequence(approveRunPath+"/jobs", []testutils.MockResponse{
		{StatusCode: http.StatusOK, Body: map[string]interface{}{"total_count": 1, "jobs": []map[string]interface{}{
			{"id": 1, "name": "build", "status": "in_progress", "steps": []map[string]interface{}{
				{"number": 1, "name": "compile", "status": "in_progress"},
			}},
		}}},
		{StatusCode: http.StatusOK, Body: map[string]interface{}{"total_count": 1, "jobs": []map[string]interface{}{
			{"id": 1, "name": "build", "status": "completed", "conclusion": "success", "steps": []map[string]interface{}{
				{"number": 1, "name": "compile", "status": "completed", "conclusion": "success"},
			}},
		}}},
	})
	ctx := servicetest.NewMockContext(t, mockServer)

	var stderr bytes.Buffer
	out := captureStdout(t, func() {
		root := newTestRoot()
		root.AddCommand(approveCommand(ctx))
		root.SetArgs([]string{"approve", "--org", "acme", "-r", "repo1", "--watch"}) // no --run: resolved
		root.SetIn(strings.NewReader("yes\n"))
		root.SetOut(io.Discard)
		root.SetErr(&stderr)
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"Watching repo1 run 123", "build\n", "build / compile\n", "build (success)", "build / compile (success)", "New gate: approval-1", "Run completed: success"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q, got:\n%s", want, out)
		}
	}
	if !strings.Contains(stderr.String(), "(run 123)") {
		t.Errorf("prompt should name the resolved run, got stderr:\n%s", stderr.String())
	}
}
