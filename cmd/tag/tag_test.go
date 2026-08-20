// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package tag

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
// persistent flag the tag subcommands actually read ("org" via
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

func TestNewTagCommand_Structure(t *testing.T) {
	ctx, _ := newMockedContext(t)
	cmd := NewTagCommand(ctx)

	if cmd.Use != "tag <command>" {
		t.Errorf("Use = %q", cmd.Use)
	}
	want := map[string]bool{"list": false, "create": false, "delete": false}
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

// ---------------------------------------------------------------------------
// list
// ---------------------------------------------------------------------------

// graphqlTagSearchBody builds a /graphql response body matching the shape of
// model.SearchTagsQuery. ListTags routes single-repo (and org-wide) list
// requests through GraphQL rather than REST — only 2+ explicit repo names
// trigger the REST fan-out path.
func graphqlTagSearchBody() map[string]interface{} {
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
										"name":   "v1.0.0",
										"target": map[string]interface{}{"oid": "sha-v1"},
									}},
									{"node": map[string]interface{}{
										"name":   "v2.0.0",
										"target": map[string]interface{}{"oid": "sha-v2"},
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
		Body:       graphqlTagSearchBody(),
	})

	out := captureStdout(t, func() {
		err := execCmd(ListCommand(ctx), "list", "--org", "acme", "-r", "repo1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "v1.0.0") {
		t.Errorf("expected output to contain the tag name, got: %s", out)
	}
}

func TestListCommand_JSONOutput(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.JSON = true
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       graphqlTagSearchBody(),
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

func TestListCommand_ExcludeReposAndFilter(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       graphqlTagSearchBody(),
	})

	out := captureStdout(t, func() {
		err := execCmd(ListCommand(ctx), "list", "--org", "acme", "-r", "repo1", "-e", "repo2", "-f", "v2", "--sort", "tag")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if strings.Contains(out, "v1.0.0") {
		t.Errorf("expected filtered-out tag not to appear, got: %s", out)
	}
	if !strings.Contains(out, "v2.0.0") {
		t.Errorf("expected matching tag to appear, got: %s", out)
	}
}

func TestListCommand_REST_MultiRepo(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	tagsBody := []map[string]interface{}{
		{"name": "v1.0.0", "commit": map[string]interface{}{"sha": "sha-v1"}},
	}
	mockServer.SetResponse("/repos/acme/repo1/tags", testutils.MockResponse{StatusCode: http.StatusOK, Body: tagsBody})
	mockServer.SetResponse("/repos/acme/repo2/tags", testutils.MockResponse{StatusCode: http.StatusOK, Body: tagsBody})

	out := captureStdout(t, func() {
		err := execCmd(ListCommand(ctx), "list", "--org", "acme", "-r", "repo1", "-r", "repo2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "v1.0.0") {
		t.Errorf("expected output to contain the tag name, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// create
// ---------------------------------------------------------------------------

func TestCreateCommand_Success(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/acme/repo1/git/ref/heads/main", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"object": map[string]interface{}{"sha": "base-sha-1"}},
	})
	mockServer.SetResponse("/repos/acme/repo1/git/tags", testutils.MockResponse{
		StatusCode: http.StatusCreated,
		Body:       map[string]interface{}{"sha": "tag-object-sha"},
	})

	captureStdout(t, func() {
		err := execCmd(CreateCommand(ctx), "create", "--org", "acme", "-r", "repo1",
			"--tag", "v1.0.0", "--head", "main", "-m", "release")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestCreateCommand_MissingOrg(t *testing.T) {
	ctx, mockServer := newMockedContext(t)

	captureStdout(t, func() {
		err := execCmd(CreateCommand(ctx), "create", "--tag", "v1.0.0", "--head", "main", "-m", "release")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests when org is missing, got %d", len(mockServer.GetRequests()))
	}
}

func TestCreateCommand_DryRun(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.DryRun = true

	out := captureStdout(t, func() {
		err := execCmd(CreateCommand(ctx), "create", "--org", "acme", "-r", "repo1",
			"--tag", "v1.0.0", "--head", "main", "-m", "release")
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

	if err := execCmd(CreateCommand(ctx), "create", "--org", "acme"); err == nil {
		t.Fatal("expected an error for missing required --tag/--head/--message flags")
	}
}

// ---------------------------------------------------------------------------
// delete
// ---------------------------------------------------------------------------

func TestDeleteCommand_Success(t *testing.T) {
	ctx, _ := newMockedContext(t)

	captureStdout(t, func() {
		err := execCmd(DeleteCommand(ctx), "delete", "--org", "acme", "-r", "repo1", "--tag", "v0.9.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestDeleteCommand_Error(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/acme/repo1/git/refs/tags/v0.9.0", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	captureStdout(t, func() {
		err := execCmd(DeleteCommand(ctx), "delete", "--org", "acme", "-r", "repo1", "--tag", "v0.9.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestDeleteCommand_MissingOrg(t *testing.T) {
	ctx, mockServer := newMockedContext(t)

	captureStdout(t, func() {
		err := execCmd(DeleteCommand(ctx), "delete", "--tag", "v0.9.0")
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
		err := execCmd(DeleteCommand(ctx), "delete", "--org", "acme", "-r", "repo1", "--tag", "v0.9.0")
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
		t.Fatal("expected an error for missing required --tag flag")
	}
}
