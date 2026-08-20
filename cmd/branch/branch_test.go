// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package branch

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/internal/testutils"
	"github.com/pradyb/sgh-cli/pkg/context"
)

// newTestRoot builds a minimal fake parent command that only defines the
// persistent flag the branch subcommands actually read ("org" via
// cmd.Flags().GetString). It intentionally has no
// PersistentPreRun/PersistentPostRun (unlike the real cmd/root.go), so it
// never calls os.Exit on an error path during tests. ctx.DryRun / ctx.JSON /
// ctx.Compact / ctx.Limit are read directly off the *context.Context, not
// through flags, so they are set on ctx in the tests below instead.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().StringP("org", "o", "", "")
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

// captureStdout redirects os.Stdout to a pipe for the duration of fn and
// returns everything written to it. The pipe is drained concurrently on a
// background goroutine — on Windows the anonymous pipe buffer is only a few
// KB, and a table-heavy Run (e.g. a non-compact multi-repo listing) can
// easily exceed that, so draining only after fn() returns would deadlock.
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

func newMockedContext(t *testing.T) (*context.Context, *testutils.MockGitHubServer) {
	t.Helper()
	mockServer := testutils.NewMockGitHubServer()
	t.Cleanup(mockServer.Close)
	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true
	return ctx, mockServer
}

func TestNewBranchCommand_Structure(t *testing.T) {
	ctx, _ := newMockedContext(t)
	cmd := NewBranchCommand(ctx)

	if cmd.Use != "branch <command>" {
		t.Errorf("Use = %q", cmd.Use)
	}
	want := map[string]bool{"list": false, "create": false, "rename": false, "delete": false}
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

func TestRepoCompletionFn(t *testing.T) {
	ctx, _ := newMockedContext(t)
	ctx.Config.AddOrganization("acme")
	ctx.Config.AddRepository("acme", "repo1")

	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().StringP("org", "o", "acme", "")
	child := &cobra.Command{Use: "list"}
	root.AddCommand(child)

	fn := repoCompletionFn(ctx)
	got, directive := fn(child, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", directive)
	}
	found := false
	for _, name := range got {
		if name == "repo1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected repo1 in completions, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// list
// ---------------------------------------------------------------------------

// graphqlBranchSearchBody builds a /graphql response body matching the shape
// of model.SearchBranchesQuery. ListBranches routes single-repo (and
// org-wide) list requests through GraphQL rather than REST — only 2+
// explicit repo names trigger the REST fan-out path.
func graphqlBranchSearchBody() map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"search": map[string]interface{}{
				"repositoryCount": 1,
				"pageInfo":        map[string]interface{}{"endCursor": "", "hasNextPage": false},
				"edges": []map[string]interface{}{
					{
						"node": map[string]interface{}{
							"name": "repo1",
							"refs": map[string]interface{}{
								"totalCount": 2,
								"edges": []map[string]interface{}{
									{"node": map[string]interface{}{
										"name":                 "main",
										"target":               map[string]interface{}{"oid": "sha-main"},
										"branchProtectionRule": map[string]interface{}{"isAdminEnforced": true, "pattern": ""},
									}},
									{"node": map[string]interface{}{
										"name":                 "develop",
										"target":               map[string]interface{}{"oid": "sha-dev"},
										"branchProtectionRule": map[string]interface{}{"isAdminEnforced": false, "pattern": ""},
									}},
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestListCommand_GraphQL_Success(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       graphqlBranchSearchBody(),
	})

	out := captureStdout(t, func() {
		err := execCmd(ListCommand(ctx), "list", "--org", "acme", "-r", "repo1", "--filter", "dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "develop") {
		t.Errorf("expected output to contain the matching branch, got: %s", out)
	}
}

func TestListCommand_JSONOutput(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.JSON = true
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       graphqlBranchSearchBody(),
	})

	out := captureStdout(t, func() {
		err := execCmd(ListCommand(ctx), "list", "--org", "acme", "-r", "repo1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "[") {
		t.Errorf("expected JSON array output, got: %s", out)
	}
}

func TestListCommand_REST_MultiRepoWithSort(t *testing.T) {
	ctx, _ := newMockedContext(t)

	out := captureStdout(t, func() {
		err := execCmd(ListCommand(ctx), "list", "--org", "acme", "-r", "repo1", "-r", "repo2", "-e", "repo3", "--sort", "name")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "develop") {
		t.Errorf("expected output to contain a branch name, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// create
// ---------------------------------------------------------------------------

func TestCreateCommand_FromRef_Success(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/acme/repo1/git/ref/heads/main", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"object": map[string]interface{}{"sha": "base-sha-1"}},
	})

	captureStdout(t, func() {
		err := execCmd(CreateCommand(ctx), "create", "--org", "acme", "-r", "repo1",
			"--new", "feature-x", "--ref", "main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestCreateCommand_FromCommit_Success(t *testing.T) {
	ctx, _ := newMockedContext(t)

	captureStdout(t, func() {
		err := execCmd(CreateCommand(ctx), "create", "--org", "acme", "-r", "repo1",
			"--new", "feature-x", "--commit", "abc123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestCreateCommand_CommitWithMultipleRepos_Error(t *testing.T) {
	ctx, mockServer := newMockedContext(t)

	err := execCmd(CreateCommand(ctx), "create", "--org", "acme", "-r", "repo1", "-r", "repo2",
		"--new", "feature-x", "--commit", "abc123")
	if err == nil {
		t.Fatal("expected an error when --commit is combined with more than one repository")
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests, got %d", len(mockServer.GetRequests()))
	}
}

func TestCreateCommand_RefAndCommitMutuallyExclusive(t *testing.T) {
	ctx, _ := newMockedContext(t)

	err := execCmd(CreateCommand(ctx), "create", "--org", "acme", "-r", "repo1",
		"--new", "feature-x", "--ref", "main", "--commit", "abc123")
	if err == nil {
		t.Fatal("expected an error when --ref and --commit are both set")
	}
}

func TestCreateCommand_MissingRefOrCommit(t *testing.T) {
	ctx, _ := newMockedContext(t)

	err := execCmd(CreateCommand(ctx), "create", "--org", "acme", "-r", "repo1", "--new", "feature-x")
	if err == nil {
		t.Fatal("expected an error when neither --ref nor --commit is set")
	}
}

func TestCreateCommand_MissingOrg(t *testing.T) {
	ctx, mockServer := newMockedContext(t)

	captureStdout(t, func() {
		err := execCmd(CreateCommand(ctx), "create", "--new", "feature-x", "--ref", "main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests when org is missing, got %d", len(mockServer.GetRequests()))
	}
}

func TestCreateCommand_DryRun_FromRef(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.DryRun = true

	out := captureStdout(t, func() {
		err := execCmd(CreateCommand(ctx), "create", "--org", "acme", "-r", "repo1",
			"--new", "feature-x", "--ref", "main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "DRY RUN") {
		t.Errorf("expected dry-run banner in output, got: %s", out)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests in dry-run mode, got %d", len(mockServer.GetRequests()))
	}
}

func TestCreateCommand_DryRun_FromCommit(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.DryRun = true

	out := captureStdout(t, func() {
		err := execCmd(CreateCommand(ctx), "create", "--org", "acme", "-r", "repo1",
			"--new", "feature-x", "--commit", "abc123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "DRY RUN") {
		t.Errorf("expected dry-run banner in output, got: %s", out)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests in dry-run mode, got %d", len(mockServer.GetRequests()))
	}
}

func TestCreateCommand_MissingRequiredFlags(t *testing.T) {
	ctx, _ := newMockedContext(t)

	if err := execCmd(CreateCommand(ctx), "create", "--org", "acme", "--ref", "main"); err == nil {
		t.Fatal("expected an error for missing required --new flag")
	}
}

// ---------------------------------------------------------------------------
// rename
// ---------------------------------------------------------------------------

func TestRenameCommand_Success(t *testing.T) {
	ctx, _ := newMockedContext(t)

	captureStdout(t, func() {
		err := execCmd(RenameCommand(ctx), "rename", "--org", "acme", "-r", "repo1",
			"--old", "old-name", "--new", "new-name")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestRenameCommand_Error(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/acme/repo1/branches/old-name/rename", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "branch not found"},
	})

	captureStdout(t, func() {
		err := execCmd(RenameCommand(ctx), "rename", "--org", "acme", "-r", "repo1",
			"--old", "old-name", "--new", "new-name")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestRenameCommand_DryRun(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.DryRun = true

	out := captureStdout(t, func() {
		err := execCmd(RenameCommand(ctx), "rename", "--org", "acme", "-r", "repo1",
			"--old", "old-name", "--new", "new-name")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "DRY RUN") {
		t.Errorf("expected dry-run banner in output, got: %s", out)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests in dry-run mode, got %d", len(mockServer.GetRequests()))
	}
}

func TestRenameCommand_MissingRequiredFlags(t *testing.T) {
	ctx, _ := newMockedContext(t)

	if err := execCmd(RenameCommand(ctx), "rename", "--org", "acme", "-r", "repo1"); err == nil {
		t.Fatal("expected an error for missing required --old/--new flags")
	}
}

// ---------------------------------------------------------------------------
// delete
// ---------------------------------------------------------------------------

func TestDeleteCommand_Success(t *testing.T) {
	ctx, _ := newMockedContext(t)

	captureStdout(t, func() {
		err := execCmd(DeleteCommand(ctx), "delete", "--org", "acme", "-r", "repo1", "--branch", "old-feature")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestDeleteCommand_Error(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/acme/repo1/git/refs/heads/old-feature", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	captureStdout(t, func() {
		err := execCmd(DeleteCommand(ctx), "delete", "--org", "acme", "-r", "repo1", "--branch", "old-feature")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestDeleteCommand_MissingOrg(t *testing.T) {
	ctx, mockServer := newMockedContext(t)

	captureStdout(t, func() {
		err := execCmd(DeleteCommand(ctx), "delete", "--branch", "old-feature")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests when org is missing, got %d", len(mockServer.GetRequests()))
	}
}

func TestDeleteCommand_DryRun(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.DryRun = true

	out := captureStdout(t, func() {
		err := execCmd(DeleteCommand(ctx), "delete", "--org", "acme", "-r", "repo1", "--branch", "old-feature")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "DRY RUN") {
		t.Errorf("expected dry-run banner in output, got: %s", out)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests in dry-run mode, got %d", len(mockServer.GetRequests()))
	}
}

func TestDeleteCommand_MissingRequiredFlags(t *testing.T) {
	ctx, _ := newMockedContext(t)

	if err := execCmd(DeleteCommand(ctx), "delete", "--org", "acme"); err == nil {
		t.Fatal("expected an error for missing required --branch flag")
	}
}
