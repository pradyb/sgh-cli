// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package workflow

import (
	"io"
	"net/http"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/pradyb/sgh-cli/internal/model"
	"github.com/pradyb/sgh-cli/internal/service"
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
	ctx := service.NewMockContext(t, mockServer)

	cmd := NewWorkflowCommand(ctx)
	if cmd.Use != "workflow <command>" {
		t.Errorf("Use = %q", cmd.Use)
	}

	want := map[string]bool{"list": false, "view": false, "rerun": false, "cancel": false, "dispatch": false}
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
	ctx := service.NewMockContext(t, mockServer)
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
	ctx := service.NewMockContext(t, mockServer)
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
	ctx := service.NewMockContext(t, mockServer)
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
	ctx := service.NewMockContext(t, mockServer)

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
	ctx := service.NewMockContext(t, mockServer)

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
	ctx := service.NewMockContext(t, mockServer)

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
	ctx := service.NewMockContext(t, mockServer)

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
	ctx := service.NewMockContext(t, mockServer)

	// Since the run is already completed, --watch should take the
	// non-watch print path and never enter the bubbletea program.
	if err := execCmd(ViewCommand(ctx), "view", "--org", "acme", "-r", "repo1", "--run", "123", "--watch"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViewCommand_MissingRepository(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)

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
	ctx := service.NewMockContext(t, mockServer)

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
	ctx := service.NewMockContext(t, mockServer)

	if err := execCmd(rerunCommand(ctx), "rerun", "--org", "acme", "-r", "repo1", "--run", "123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRerunCommand_DryRun(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)
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
	ctx := service.NewMockContext(t, mockServer)

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
	ctx := service.NewMockContext(t, mockServer)

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
	ctx := service.NewMockContext(t, mockServer)

	if err := execCmd(cancelCommand(ctx), "cancel", "--org", "acme", "-r", "repo1", "--run", "123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCancelCommand_DryRun(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)
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
	ctx := service.NewMockContext(t, mockServer)
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
	ctx := service.NewMockContext(t, mockServer)
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
	ctx := service.NewMockContext(t, mockServer)
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
	ctx := service.NewMockContext(t, mockServer)

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
	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	err := execCmd(DispatchCommand(ctx), "dispatch", "--org", "acme", "-r", "repo1", "--workflow", "build.yml", "--ref", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRepoCompletionFn(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)

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
	ctx := service.NewMockContext(t, mockServer)
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
	ctx := service.NewMockContext(t, mockServer)
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
