// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package pr

import (
	"io"
	"net/http"
	"testing"

	"github.com/spf13/cobra"

	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

// newTestRoot builds a minimal fake parent command that only defines the
// persistent flags the pr subcommands actually read. It intentionally has
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

func TestNewPRCommand_Structure(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)

	cmd := NewPRCommand(ctx)
	want := map[string]bool{
		"create": false, "list": false, "view": false, "review": false,
		"update": false, "merge": false, "close": false, "reopen": false,
	}
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

func TestCreateCommand_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/pulls", testutils.MockResponse{
		StatusCode: http.StatusCreated,
		Body: map[string]interface{}{
			"number":   42,
			"title":    "Add feature",
			"html_url": "https://github.com/acme/repo1/pull/42",
			"state":    "open",
		},
	})
	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	err := execCmd(CreateCommand(ctx), "create", "--org", "acme", "-r", "repo1",
		"--title", "Add feature", "--base", "main", "--head", "feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateCommand_MultiRepoMixedResults(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/pulls", testutils.MockResponse{
		StatusCode: http.StatusCreated,
		Body:       map[string]interface{}{"number": 1, "title": "PR in repo1"},
	})
	mockServer.SetResponse("/repos/acme/repo2/pulls", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})
	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	err := execCmd(CreateCommand(ctx), "create", "--org", "acme", "-r", "repo1", "-r", "repo2",
		"--title", "combined", "--base", "main", "--head", "feature", "--label", "bug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateCommand_DryRun(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)
	ctx.DryRun = true

	err := execCmd(CreateCommand(ctx), "create", "--org", "acme", "-r", "repo1",
		"--title", "Add feature", "--base", "main", "--head", "feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests in dry-run mode, got %d", len(mockServer.GetRequests()))
	}
}

func TestCreateCommand_MissingRequiredFlags(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)

	if err := execCmd(CreateCommand(ctx), "create", "--org", "acme"); err == nil {
		t.Fatal("expected an error for missing required --title/--base/--head flags")
	}
}

func TestListCommand_RESTMultiRepo(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/pulls", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: []map[string]interface{}{
			{"number": 1, "title": "Alice's PR", "user": map[string]interface{}{"login": "alice"}},
		},
	})
	mockServer.SetResponse("/repos/acme/repo2/pulls", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})
	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	err := execCmd(ListCommand(ctx), "list", "--org", "acme", "-r", "repo1", "-r", "repo2",
		"--state", "all", "--author", "alice", "--sort", "author")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListCommand_GraphQLSingleRepo(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"data": map[string]interface{}{
				"search": map[string]interface{}{
					"issueCount": 1,
					"pageInfo":   map[string]interface{}{"endCursor": "", "hasNextPage": false},
					"edges": []map[string]interface{}{
						{
							"node": map[string]interface{}{
								"number":  42,
								"title":   "GraphQL PR",
								"url":     "https://github.com/acme/repo1/pull/42",
								"baseRef": map[string]interface{}{"name": "main", "repository": map[string]interface{}{"name": "repo1"}},
								"headRef": map[string]interface{}{"name": "feature", "repository": map[string]interface{}{"name": "repo1"}},
								"state":   "OPEN",
								"author":  map[string]interface{}{"login": "jdoe", "name": "J Doe"},
							},
						},
					},
				},
			},
		},
	})
	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	err := execCmd(ListCommand(ctx), "list", "--org", "acme", "-r", "repo1", "--base", "main", "--head", "feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListCommand_JSONOutput(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"data": map[string]interface{}{
				"search": map[string]interface{}{
					"issueCount": 0,
					"pageInfo":   map[string]interface{}{"endCursor": "", "hasNextPage": false},
					"edges":      []map[string]interface{}{},
				},
			},
		},
	})
	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true
	ctx.JSON = true

	if err := execCmd(ListCommand(ctx), "list", "--org", "acme"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListCommand_DeprecatedAllAliases(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"data": map[string]interface{}{
				"search": map[string]interface{}{
					"issueCount": 0,
					"pageInfo":   map[string]interface{}{"endCursor": "", "hasNextPage": false},
					"edges":      []map[string]interface{}{},
				},
			},
		},
	})
	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	if err := execCmd(ListCommand(ctx), "list", "--org", "acme", "--all"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := execCmd(ListCommand(ctx), "list", "--org", "acme", "--all-status"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViewCommand_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"data": map[string]interface{}{
				"organization": map[string]interface{}{
					"repository": map[string]interface{}{
						"pullRequest": map[string]interface{}{
							"number": 100,
							"title":  "Add feature",
							"url":    "https://github.com/acme/repo1/pull/100",
							"state":  "OPEN",
							"author": map[string]interface{}{"login": "author1", "name": "Author One"},
						},
					},
				},
			},
		},
	})
	ctx := service.NewMockContext(t, mockServer)

	if err := execCmd(ViewCommand(ctx), "view", "--org", "acme", "-r", "repo1", "--pr", "100"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViewCommand_JSONOutput(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "forbidden"},
	})
	ctx := service.NewMockContext(t, mockServer)
	ctx.JSON = true

	if err := execCmd(ViewCommand(ctx), "view", "--org", "acme", "-r", "repo1", "--pr", "100"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViewCommand_MissingRequiredFlags(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)

	if err := execCmd(ViewCommand(ctx), "view", "--org", "acme"); err == nil {
		t.Fatal("expected an error for missing required --repository/--pr flags")
	}
}

func TestReviewCommand_Approve(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/pulls/7/reviews", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"id": 99, "state": "APPROVED", "user": map[string]interface{}{"login": "reviewer1"},
		},
	})
	ctx := service.NewMockContext(t, mockServer)

	err := execCmd(ReviewCommand(ctx), "review", "--org", "acme", "-r", "repo1", "--pr", "7", "--approve")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReviewCommand_RequestChangesWithBody(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/pulls/7/reviews", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"id": 99, "state": "CHANGES_REQUESTED", "user": map[string]interface{}{"login": "reviewer1"},
		},
	})
	ctx := service.NewMockContext(t, mockServer)
	ctx.JSON = true

	err := execCmd(ReviewCommand(ctx), "review", "--org", "acme", "-r", "repo1", "--pr", "7",
		"--request-changes", "--body", "please fix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReviewCommand_CommentMissingBody(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)

	// --comment without --body should be rejected before any network call.
	err := execCmd(ReviewCommand(ctx), "review", "--org", "acme", "-r", "repo1", "--pr", "7", "--comment")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests when body validation fails, got %d", len(mockServer.GetRequests()))
	}
}

func TestReviewCommand_NoEventFlag(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)

	// No --approve/--comment/--request-changes and no legacy --event: invalid.
	err := execCmd(ReviewCommand(ctx), "review", "--org", "acme", "-r", "repo1", "--pr", "7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests for invalid event, got %d", len(mockServer.GetRequests()))
	}
}

func TestReviewCommand_LegacyEventFlag(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/pulls/7/reviews", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"id": 99, "state": "COMMENTED", "user": map[string]interface{}{"login": "reviewer1"}},
	})
	ctx := service.NewMockContext(t, mockServer)

	err := execCmd(ReviewCommand(ctx), "review", "--org", "acme", "-r", "repo1", "--pr", "7",
		"--event", "comment", "--body", "legacy path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReviewCommand_DryRun(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)
	ctx.DryRun = true

	err := execCmd(ReviewCommand(ctx), "review", "--org", "acme", "-r", "repo1", "--pr", "7", "--approve")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests in dry-run mode, got %d", len(mockServer.GetRequests()))
	}
}

func TestReviewCommand_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/pulls/7/reviews", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})
	ctx := service.NewMockContext(t, mockServer)

	err := execCmd(ReviewCommand(ctx), "review", "--org", "acme", "-r", "repo1", "--pr", "7", "--approve")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReviewCommand_MissingRequiredFlags(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)

	if err := execCmd(ReviewCommand(ctx), "review", "--org", "acme"); err == nil {
		t.Fatal("expected an error for missing required --repository/--pr flags")
	}
}

func TestUpdateCommand_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/pulls/7", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"number": 7, "title": "Updated PR", "state": "closed"},
	})
	ctx := service.NewMockContext(t, mockServer)

	err := execCmd(UpdateCommand(ctx), "update", "--org", "acme", "-r", "repo1", "--pr", "7", "--state", "closed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateCommand_InvalidState(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)

	err := execCmd(UpdateCommand(ctx), "update", "--org", "acme", "-r", "repo1", "--pr", "7", "--state", "bogus")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests for invalid state, got %d", len(mockServer.GetRequests()))
	}
}

func TestUpdateCommand_DryRun(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)
	ctx.DryRun = true

	err := execCmd(UpdateCommand(ctx), "update", "--org", "acme", "-r", "repo1", "--pr", "7", "--state", "open")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests in dry-run mode, got %d", len(mockServer.GetRequests()))
	}
}

func TestUpdateCommand_MissingRequiredFlags(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)

	if err := execCmd(UpdateCommand(ctx), "update", "--org", "acme"); err == nil {
		t.Fatal("expected an error for missing required flags")
	}
}

func TestMergeCommand_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/pulls/7/merge", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"merged": true, "message": "merged", "sha": "abc123"},
	})
	ctx := service.NewMockContext(t, mockServer)

	err := execCmd(MergeCommand(ctx), "merge", "--org", "acme", "-r", "repo1", "--pr", "7",
		"--title", "Post release merge", "--body", "merging release branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMergeCommand_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/pulls/7/merge", testutils.MockResponse{
		StatusCode: http.StatusMethodNotAllowed,
		Body:       map[string]interface{}{"message": "Pull Request is not mergeable"},
	})
	ctx := service.NewMockContext(t, mockServer)

	err := execCmd(MergeCommand(ctx), "merge", "--org", "acme", "-r", "repo1", "--pr", "7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMergeCommand_DryRun(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)
	ctx.DryRun = true

	err := execCmd(MergeCommand(ctx), "merge", "--org", "acme", "-r", "repo1", "--pr", "7", "--title", "merge it")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests in dry-run mode, got %d", len(mockServer.GetRequests()))
	}
}

func TestMergeCommand_MissingRequiredFlags(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)

	if err := execCmd(MergeCommand(ctx), "merge", "--org", "acme"); err == nil {
		t.Fatal("expected an error for missing required --repository/--pr flags")
	}
}

func TestCloseCommand_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/pulls/42", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"number": 42, "title": "Closed PR", "state": "closed"},
	})
	ctx := service.NewMockContext(t, mockServer)

	if err := execCmd(CloseCommand(ctx), "close", "--org", "acme", "-r", "repo1", "--pr", "42"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCloseCommand_DryRun(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)
	ctx.DryRun = true

	err := execCmd(CloseCommand(ctx), "close", "--org", "acme", "-r", "repo1", "--pr", "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests in dry-run mode, got %d", len(mockServer.GetRequests()))
	}
}

func TestCloseCommand_MissingRequiredFlags(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)

	if err := execCmd(CloseCommand(ctx), "close", "--org", "acme"); err == nil {
		t.Fatal("expected an error for missing required --repository/--pr flags")
	}
}

func TestReopenCommand_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/repo1/pulls/42", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"number": 42, "title": "Reopened PR", "state": "open"},
	})
	ctx := service.NewMockContext(t, mockServer)

	if err := execCmd(ReopenCommand(ctx), "reopen", "--org", "acme", "-r", "repo1", "--pr", "42"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReopenCommand_DryRun(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)
	ctx.DryRun = true

	err := execCmd(ReopenCommand(ctx), "reopen", "--org", "acme", "-r", "repo1", "--pr", "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockServer.GetRequests()) != 0 {
		t.Errorf("expected no network requests in dry-run mode, got %d", len(mockServer.GetRequests()))
	}
}

func TestReopenCommand_MissingRequiredFlags(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)

	if err := execCmd(ReopenCommand(ctx), "reopen", "--org", "acme"); err == nil {
		t.Fatal("expected an error for missing required --repository/--pr flags")
	}
}
