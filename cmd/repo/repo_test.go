// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
	"github.com/pradyb/sgh-cli/pkg/context"
)

// newTestRoot builds a minimal fake parent command that only defines the
// persistent flags the repo subcommands actually read. It intentionally has
// no PersistentPreRun/PersistentPostRun (unlike the real cmd/root.go), so it
// never calls os.Exit on an error path during tests.
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

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w

	// Drain concurrently: the anonymous pipe's buffer is small, and a
	// table-heavy Run can easily exceed it, so draining only after fn()
	// returns would deadlock.
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
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true
	return ctx, mockServer
}

// graphqlRepoSearchBody returns a two-repo GraphQL search response shaped like
// model.SearchRepositoriesQuery.
func graphqlRepoSearchBody() map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"search": map[string]interface{}{
				"repositoryCount": 2,
				"pageInfo":        map[string]interface{}{"endCursor": "", "hasNextPage": false},
				"edges": []map[string]interface{}{
					{"node": map[string]interface{}{
						"name":             "repo-a",
						"nameWithOwner":    "acme/repo-a",
						"url":              "https://github.com/acme/repo-a",
						"sshUrl":           "git@github.com:acme/repo-a.git",
						"description":      "desc a",
						"isPrivate":        false,
						"isArchived":       false,
						"isDisabled":       false,
						"defaultBranchRef": map[string]interface{}{"name": "main"},
						"primaryLanguage":  map[string]interface{}{"name": "Go"},
						"pullRequests":     map[string]interface{}{"totalCount": 1},
					}},
					{"node": map[string]interface{}{
						"name":             "repo-b",
						"nameWithOwner":    "acme/repo-b",
						"url":              "https://github.com/acme/repo-b",
						"sshUrl":           "git@github.com:acme/repo-b.git",
						"description":      "desc b",
						"isPrivate":        true,
						"isArchived":       false,
						"isDisabled":       false,
						"defaultBranchRef": map[string]interface{}{"name": "main"},
						"primaryLanguage":  map[string]interface{}{"name": "Python"},
						"pullRequests":     map[string]interface{}{"totalCount": 0},
					}},
				},
			},
		},
	}
}

// emptyGraphqlRepoSearchBody returns a zero-result GraphQL search response
// shaped to match model.SearchRepositoriesQuery.
func emptyGraphqlRepoSearchBody() map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"search": map[string]interface{}{
				"repositoryCount": 0,
				"pageInfo":        map[string]interface{}{"endCursor": "", "hasNextPage": false},
				"edges":           []map[string]interface{}{},
			},
		},
	}
}

func TestNewRepoCommand_Structure(t *testing.T) {
	ctx, _ := newMockedContext(t)
	cmd := NewRepoCommand(ctx)
	want := map[string]bool{"list": false, "search": false, "archive": false, "visibility": false}
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

func TestListCommand_All_Success(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/graphql", testutils.MockResponse{StatusCode: http.StatusOK, Body: graphqlRepoSearchBody()})

	out := captureStdout(t, func() {
		if err := execCmd(ListCommand(ctx), "list", "--org", "acme", "--all"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "repo-a") {
		t.Errorf("expected output to contain repo-a, got: %s", out)
	}
}

func TestListCommand_NotConfigured_NoData(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/graphql", testutils.MockResponse{StatusCode: http.StatusOK, Body: emptyGraphqlRepoSearchBody()})

	out := captureStdout(t, func() {
		if err := execCmd(ListCommand(ctx), "list", "--org", "acme"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "No repositories found") {
		t.Errorf("expected no-data message, got: %s", out)
	}
}

func TestListCommand_JSONOutput(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.JSON = true
	mockServer.SetResponse("/graphql", testutils.MockResponse{StatusCode: http.StatusOK, Body: graphqlRepoSearchBody()})

	out := captureStdout(t, func() {
		if err := execCmd(ListCommand(ctx), "list", "--org", "acme", "--all"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "[") {
		t.Errorf("expected JSON array output, got: %s", out)
	}
}

func TestListCommand_LimitTruncates(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.Limit = 1
	mockServer.SetResponse("/graphql", testutils.MockResponse{StatusCode: http.StatusOK, Body: graphqlRepoSearchBody()})

	out := captureStdout(t, func() {
		if err := execCmd(ListCommand(ctx), "list", "--org", "acme", "--all"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if strings.Contains(out, "repo-b") {
		t.Errorf("expected repo-b to be truncated by --limit, got: %s", out)
	}
}

func TestListCommand_MissingOrg(t *testing.T) {
	ctx, _ := newMockedContext(t)

	if err := execCmd(ListCommand(ctx), "list"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListCommand_PositionalArgFallback(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/graphql", testutils.MockResponse{StatusCode: http.StatusOK, Body: graphqlRepoSearchBody()})

	if err := execCmd(ListCommand(ctx), "list", "acme", "--all"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListCommand_Error(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	if err := execCmd(ListCommand(ctx), "list", "--org", "acme", "--all"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// search
// ---------------------------------------------------------------------------

func TestSearchCommand_Success(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/graphql", testutils.MockResponse{StatusCode: http.StatusOK, Body: graphqlRepoSearchBody()})

	out := captureStdout(t, func() {
		err := execCmd(SearchCommand(ctx), "search", "--org", "acme", "--query", "api", "--language", "go", "--topic", "cli")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "repo-a") {
		t.Errorf("expected output to contain repo-a, got: %s", out)
	}
}

func TestSearchCommand_NoResults(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/graphql", testutils.MockResponse{StatusCode: http.StatusOK, Body: emptyGraphqlRepoSearchBody()})

	out := captureStdout(t, func() {
		if err := execCmd(SearchCommand(ctx), "search", "--org", "acme", "--query", "nothing"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "No repositories matched") {
		t.Errorf("expected no-match message, got: %s", out)
	}
}

func TestSearchCommand_MissingOrg(t *testing.T) {
	ctx, _ := newMockedContext(t)

	if err := execCmd(SearchCommand(ctx), "search", "--query", "api"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSearchCommand_NoFilters(t *testing.T) {
	ctx, _ := newMockedContext(t)

	if err := execCmd(SearchCommand(ctx), "search", "--org", "acme"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSearchCommand_Error(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	if err := execCmd(SearchCommand(ctx), "search", "--org", "acme", "--query", "api"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// archive
// ---------------------------------------------------------------------------

func TestArchiveCommand_Success(t *testing.T) {
	ctx, _ := newMockedContext(t)

	if err := execCmd(ArchiveCommand(ctx), "archive", "--org", "acme", "-r", "repo1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestArchiveCommand_Unarchive(t *testing.T) {
	ctx, _ := newMockedContext(t)

	if err := execCmd(ArchiveCommand(ctx), "archive", "--org", "acme", "-r", "repo1", "--unarchive"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestArchiveCommand_ExcludeFlag(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/graphql", testutils.MockResponse{StatusCode: http.StatusOK, Body: graphqlRepoSearchBody()})
	ctx.Config.AddOrganization("acme")

	if err := execCmd(ArchiveCommand(ctx), "archive", "--org", "acme", "-e", "repo-a"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestArchiveCommand_Error(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/acme/repo1", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	if err := execCmd(ArchiveCommand(ctx), "archive", "--org", "acme", "-r", "repo1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestArchiveCommand_DryRun(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.DryRun = true

	out := captureStdout(t, func() {
		if err := execCmd(ArchiveCommand(ctx), "archive", "--org", "acme", "-r", "repo1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "DRY RUN") {
		t.Errorf("expected dry-run banner, got: %s", out)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests in dry-run mode, got %d", len(mockServer.GetRequests()))
	}
}

func TestArchiveCommand_DryRun_ResolveError(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.DryRun = true
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	if err := execCmd(ArchiveCommand(ctx), "archive", "--org", "acme"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// visibility
// ---------------------------------------------------------------------------

func TestVisibilityCommand_Success(t *testing.T) {
	ctx, _ := newMockedContext(t)

	err := execCmd(VisibilityCommand(ctx), "visibility", "--org", "acme", "-r", "repo1", "--visibility", "private")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVisibilityCommand_InvalidValue(t *testing.T) {
	ctx, mockServer := newMockedContext(t)

	err := execCmd(VisibilityCommand(ctx), "visibility", "--org", "acme", "-r", "repo1", "--visibility", "bogus")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests for invalid visibility, got %d", len(mockServer.GetRequests()))
	}
}

func TestVisibilityCommand_MissingRequiredFlag(t *testing.T) {
	ctx, _ := newMockedContext(t)

	if err := execCmd(VisibilityCommand(ctx), "visibility", "--org", "acme", "-r", "repo1"); err == nil {
		t.Fatal("expected an error for missing required --visibility flag")
	}
}

func TestVisibilityCommand_DryRun(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	ctx.DryRun = true

	out := captureStdout(t, func() {
		err := execCmd(VisibilityCommand(ctx), "visibility", "--org", "acme", "-r", "repo1", "--visibility", "public")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "DRY RUN") {
		t.Errorf("expected dry-run banner, got: %s", out)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests in dry-run mode, got %d", len(mockServer.GetRequests()))
	}
}

func TestVisibilityCommand_Error(t *testing.T) {
	ctx, mockServer := newMockedContext(t)
	mockServer.SetResponse("/repos/acme/repo1", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "not found"},
	})

	err := execCmd(VisibilityCommand(ctx), "visibility", "--org", "acme", "-r", "repo1", "--visibility", "private")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
