// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package issue

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/spf13/cobra"

	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
	"github.com/pradyb/sgh-cli/pkg/context"
)

// newTestRoot builds a minimal fake parent command that only defines the
// persistent flags the issue subcommands read via cmd.Flags().Get*, without
// any PersistentPreRun/PersistentPostRun (unlike the real cmd/root.go, whose
// PersistentPostRun calls os.Exit(1) on error — unusable in tests).
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

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it. Needed because pkg/ui writes directly to
// os.Stdout/os.Stderr rather than through cmd.OutOrStdout().
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
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

	os.Stdout = old
	w.Close()
	return <-outCh
}

func newMockCtx(t *testing.T) (*context.Context, *testutils.MockGitHubServer) {
	t.Helper()
	mockServer := testutils.NewMockGitHubServer()
	t.Cleanup(mockServer.Close)
	ctx := servicetest.NewMockContext(t, mockServer)
	return ctx, mockServer
}

func execRoot(ctx *context.Context, args ...string) (*cobra.Command, *bytes.Buffer, *bytes.Buffer, error) {
	root := newTestRoot()
	root.AddCommand(NewIssueCommand(ctx))
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	err := root.Execute()
	return root, &stdout, &stderr, err
}

// ---- repoCompletionFn ----

func TestRepoCompletionFn(t *testing.T) {
	tests := []struct {
		name  string
		repos []string
	}{
		{name: "no configured repos"},
		{name: "some configured repos", repos: []string{"repo1", "repo2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := newMockCtx(t)
			ctx.Config.AddOrganization("testorg")
			for _, r := range tt.repos {
				ctx.Config.AddRepository("testorg", r)
			}

			root := newTestRoot()
			root.PersistentFlags().Set("org", "testorg")

			fn := repoCompletionFn(ctx)
			got, directive := fn(root, nil, "")

			if directive != cobra.ShellCompDirectiveNoFileComp {
				t.Errorf("directive = %v, want NoFileComp", directive)
			}
			if len(got) != len(tt.repos) {
				t.Errorf("got = %v, want %v", got, tt.repos)
			}
		})
	}
}

// ---- issue list ----

func TestIssueListCommand_GraphQL_Success(t *testing.T) {
	ctx, mockServer := newMockCtx(t)
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"data": map[string]interface{}{
				"search": map[string]interface{}{
					"issueCount": 1,
					"pageInfo":   map[string]interface{}{"endCursor": "", "hasNextPage": false},
					"edges": []map[string]interface{}{
						{"node": map[string]interface{}{
							"number": 42, "title": "Bug", "url": "https://github.com/testorg/repo1/issues/42",
							"body": "desc", "state": "OPEN",
							"createdAt": "2024-01-01T00:00:00Z", "updatedAt": "2024-01-02T00:00:00Z",
							"author": map[string]interface{}{"login": "jane", "name": "Jane Doe"},
							"repository": map[string]interface{}{
								"name": "repo1", "nameWithOwner": "testorg/repo1",
								"url": "https://github.com/testorg/repo1", "sshUrl": "git@github.com:testorg/repo1.git",
							},
							"assignees": map[string]interface{}{"totalCount": 0, "edges": []map[string]interface{}{}},
							"labels":    map[string]interface{}{"totalCount": 0, "edges": []map[string]interface{}{}},
							"comments":  map[string]interface{}{"totalCount": 0},
						}},
					},
				},
			},
		},
	})

	_, _, _, err := execRoot(ctx, "issue", "list", "--org", "testorg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sawGraphQL bool
	for _, r := range mockServer.GetRequests() {
		if r.Path == "/graphql" {
			sawGraphQL = true
		}
	}
	if !sawGraphQL {
		t.Error("expected a /graphql request")
	}
}

func TestIssueListCommand_JSON(t *testing.T) {
	ctx, mockServer := newMockCtx(t)
	ctx.JSON = true
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"data": map[string]interface{}{
				"search": map[string]interface{}{
					"issueCount": 1,
					"pageInfo":   map[string]interface{}{"endCursor": "", "hasNextPage": false},
					"edges": []map[string]interface{}{
						{"node": map[string]interface{}{
							"number": 7, "title": "JSON issue", "url": "https://github.com/testorg/repo1/issues/7",
							"body": "", "state": "OPEN",
							"createdAt": "2024-01-01T00:00:00Z", "updatedAt": "2024-01-02T00:00:00Z",
							"author":     map[string]interface{}{"login": "jane", "name": "Jane Doe"},
							"repository": map[string]interface{}{"name": "repo1", "nameWithOwner": "testorg/repo1", "url": "", "sshUrl": ""},
							"assignees":  map[string]interface{}{"totalCount": 0, "edges": []map[string]interface{}{}},
							"labels":     map[string]interface{}{"totalCount": 0, "edges": []map[string]interface{}{}},
							"comments":   map[string]interface{}{"totalCount": 0},
						}},
					},
				},
			},
		},
	})

	out := captureStdout(t, func() {
		if _, _, _, err := execRoot(ctx, "issue", "list", "--org", "testorg"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !bytes.Contains([]byte(out), []byte("JSON issue")) {
		t.Errorf("expected JSON output to contain issue title, got: %s", out)
	}
}

func TestIssueListCommand_MultiRepo_REST_Success(t *testing.T) {
	ctx, mockServer := newMockCtx(t)
	ctx.Silent = true
	mockServer.SetResponse("/repos/testorg/repo1/issues", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       []map[string]interface{}{{"number": 1, "title": "one", "state": "open"}},
	})
	mockServer.SetResponse("/repos/testorg/repo2/issues", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       []map[string]interface{}{{"number": 2, "title": "two", "state": "open"}},
	})

	_, _, _, err := execRoot(ctx, "issue", "list", "--org", "testorg", "-r", "repo1", "-r", "repo2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.HasError {
		t.Error("expected HasError to remain false on success")
	}
}

func TestIssueListCommand_MultiRepo_PartialError(t *testing.T) {
	ctx, mockServer := newMockCtx(t)
	ctx.Silent = true
	mockServer.SetResponse("/repos/testorg/repo1/issues", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       []map[string]interface{}{{"number": 1, "title": "one", "state": "open"}},
	})
	mockServer.SetResponse("/repos/testorg/repo2/issues", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	_, _, _, err := execRoot(ctx, "issue", "list", "--org", "testorg", "-r", "repo1", "-r", "repo2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.HasError {
		t.Error("expected HasError to be set after a partial failure")
	}
}

func TestIssueListCommand_CreatorAlias(t *testing.T) {
	ctx, mockServer := newMockCtx(t)
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"data": map[string]interface{}{"search": map[string]interface{}{"issueCount": 0, "pageInfo": map[string]interface{}{"endCursor": "", "hasNextPage": false}, "edges": []map[string]interface{}{}}}},
	})

	_, _, _, err := execRoot(ctx, "issue", "list", "--org", "testorg", "--creator", "jane-doe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body string
	for _, r := range mockServer.GetRequests() {
		if r.Path == "/graphql" {
			body = r.Body
		}
	}
	if !bytes.Contains([]byte(body), []byte("author:jane-doe")) {
		t.Errorf("expected the query to include author:jane-doe from the deprecated --creator alias, got: %s", body)
	}
}

// ---- issue view ----

func TestIssueViewCommand_Success(t *testing.T) {
	ctx, mockServer := newMockCtx(t)
	mockServer.SetResponse("/repos/testorg/repo1/issues/42", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"number": 42, "title": "Investigate", "state": "open"},
	})
	mockServer.SetResponse("/repos/testorg/repo1/issues/42/comments", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       []map[string]interface{}{{"id": 1, "body": "a comment", "user": map[string]interface{}{"login": "jane"}}},
	})

	_, _, _, err := execRoot(ctx, "issue", "view", "--org", "testorg", "-r", "repo1", "--issue", "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sawComments bool
	for _, r := range mockServer.GetRequests() {
		if r.Path == "/repos/testorg/repo1/issues/42/comments" {
			sawComments = true
		}
	}
	if !sawComments {
		t.Error("expected the comments endpoint to have been called on success")
	}
}

func TestIssueViewCommand_ErrorSkipsComments(t *testing.T) {
	ctx, mockServer := newMockCtx(t)
	mockServer.SetResponse("/repos/testorg/repo1/issues/99", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	_, _, _, err := execRoot(ctx, "issue", "view", "--org", "testorg", "-r", "repo1", "--issue", "99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, r := range mockServer.GetRequests() {
		if r.Path == "/repos/testorg/repo1/issues/99/comments" {
			t.Error("did not expect the comments endpoint to be called when the issue lookup failed")
		}
	}
}

func TestIssueViewCommand_MissingRequiredFlags(t *testing.T) {
	ctx, _ := newMockCtx(t)

	_, _, _, err := execRoot(ctx, "issue", "view", "--org", "testorg")
	if err == nil {
		t.Fatal("expected an error when required flags are missing")
	}
}

func TestIssueViewCommand_JSON(t *testing.T) {
	ctx, mockServer := newMockCtx(t)
	ctx.JSON = true
	mockServer.SetResponse("/repos/testorg/repo1/issues/5", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"number": 5, "title": "JSON view", "state": "open"},
	})

	out := captureStdout(t, func() {
		if _, _, _, err := execRoot(ctx, "issue", "view", "--org", "testorg", "-r", "repo1", "--issue", "5"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !bytes.Contains([]byte(out), []byte("JSON view")) {
		t.Errorf("expected JSON output to contain the issue title, got: %s", out)
	}

	// The comments endpoint should not be hit for JSON output.
	for _, r := range mockServer.GetRequests() {
		if r.Path == "/repos/testorg/repo1/issues/5/comments" {
			t.Error("did not expect the comments endpoint to be called for --json output")
		}
	}
}

// ---- issue create ----

func TestIssueCreateCommand_Success(t *testing.T) {
	ctx, mockServer := newMockCtx(t)
	mockServer.SetResponse("/repos/testorg/repo1/issues", testutils.MockResponse{
		StatusCode: http.StatusCreated,
		Body: map[string]interface{}{
			"number": 3, "title": "New bug", "state": "open",
			"html_url": "https://github.com/testorg/repo1/issues/3",
		},
	})

	out := captureStdout(t, func() {
		_, _, _, err := execRoot(ctx, "issue", "create", "--org", "testorg", "-r", "repo1",
			"--title", "New bug", "--label", "bug, high-priority, ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !bytes.Contains([]byte(out), []byte("Issue #3 created in repo1")) {
		t.Errorf("expected success message in output, got: %s", out)
	}
	if !bytes.Contains([]byte(out), []byte("https://github.com/testorg/repo1/issues/3")) {
		t.Errorf("expected HTML URL in output, got: %s", out)
	}

	// Verify the label list was split/trimmed correctly.
	var body string
	for _, r := range mockServer.GetRequests() {
		if r.Method == http.MethodPost && r.Path == "/repos/testorg/repo1/issues" {
			body = r.Body
		}
	}
	var decoded struct {
		Labels []string `json:"labels"`
	}
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}
	want := []string{"bug", "high-priority"}
	if len(decoded.Labels) != len(want) || decoded.Labels[0] != want[0] || decoded.Labels[1] != want[1] {
		t.Errorf("Labels = %v, want %v", decoded.Labels, want)
	}
}

func TestIssueCreateCommand_Error(t *testing.T) {
	ctx, mockServer := newMockCtx(t)
	mockServer.SetResponse("/repos/testorg/repo1/issues", testutils.MockResponse{
		StatusCode: http.StatusUnprocessableEntity,
		Body:       map[string]interface{}{"message": "validation failed"},
	})

	_, _, stderr, err := execRoot(ctx, "issue", "create", "--org", "testorg", "-r", "repo1", "--title", "Bad")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("failed to create issue")) {
		t.Errorf("expected error message on stderr, got: %s", stderr.String())
	}
}

func TestIssueCreateCommand_MissingRequiredFlags(t *testing.T) {
	ctx, _ := newMockCtx(t)

	_, _, _, err := execRoot(ctx, "issue", "create", "--org", "testorg")
	if err == nil {
		t.Fatal("expected an error when required flags are missing")
	}
}

func TestIssueCreateCommand_JSON(t *testing.T) {
	ctx, mockServer := newMockCtx(t)
	ctx.JSON = true
	mockServer.SetResponse("/repos/testorg/repo1/issues", testutils.MockResponse{
		StatusCode: http.StatusCreated,
		Body:       map[string]interface{}{"number": 9, "title": "JSON create", "state": "open"},
	})

	out := captureStdout(t, func() {
		_, _, _, err := execRoot(ctx, "issue", "create", "--org", "testorg", "-r", "repo1", "--title", "JSON create")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !bytes.Contains([]byte(out), []byte("JSON create")) {
		t.Errorf("expected JSON output to contain the issue title, got: %s", out)
	}
}
