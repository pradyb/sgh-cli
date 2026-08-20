// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package postrelease

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

func TestProcessPostRelease_BranchOnly_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/git/ref/heads/main", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"object": map[string]interface{}{"sha": "refsha123", "type": "commit"}},
	})
	// POST .../git/refs is served by the mock server's built-in default handler.

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := ProcessPostRelease(ctx, PostReleaseRequest{
		OrgName:    "testorg",
		RepoNames:  []string{"repo1"},
		Ref:        "main",
		BranchName: "hotfix-1",
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	got := responses[0]
	if got.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", got.ErrorMessage)
	}
	if got.BranchName != "hotfix-1" {
		t.Errorf("BranchName = %q, want %q", got.BranchName, "hotfix-1")
	}
	if got.BranchSHA == "" {
		t.Error("expected BranchSHA to be populated")
	}
	if got.TagName != "" {
		t.Errorf("TagName = %q, want empty", got.TagName)
	}
}

func TestProcessPostRelease_TagOnly_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/git/ref/heads/main", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"object": map[string]interface{}{"sha": "refsha123", "type": "commit"}},
	})
	mockServer.SetResponse("/repos/testorg/repo1/git/tags", testutils.MockResponse{
		StatusCode: http.StatusCreated,
		Body:       map[string]interface{}{"sha": "tagcommitsha"},
	})
	// POST .../git/refs (final tag ref creation) is served by the mock server's built-in default handler.

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := ProcessPostRelease(ctx, PostReleaseRequest{
		OrgName:   "testorg",
		RepoNames: []string{"repo1"},
		Ref:       "main",
		TagName:   "v1.2.3",
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	got := responses[0]
	if got.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", got.ErrorMessage)
	}
	if got.TagName != "v1.2.3" {
		t.Errorf("TagName = %q, want %q", got.TagName, "v1.2.3")
	}
	if got.TagSHA == "" {
		t.Error("expected TagSHA to be populated")
	}
	if got.TagURL == "" {
		t.Error("expected TagURL to be populated")
	}
	if got.BranchName != "" {
		t.Errorf("BranchName = %q, want empty", got.BranchName)
	}
}

func TestProcessPostRelease_BranchAndTag_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/git/ref/heads/main", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"object": map[string]interface{}{"sha": "refsha123", "type": "commit"}},
	})
	mockServer.SetResponse("/repos/testorg/repo1/git/tags", testutils.MockResponse{
		StatusCode: http.StatusCreated,
		Body:       map[string]interface{}{"sha": "tagcommitsha"},
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := ProcessPostRelease(ctx, PostReleaseRequest{
		OrgName:    "testorg",
		RepoNames:  []string{"repo1"},
		Ref:        "main",
		BranchName: "hotfix-1",
		TagName:    "v1.2.3",
		Message:    "Release message",
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	got := responses[0]
	if got.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", got.ErrorMessage)
	}
	if got.BranchName != "hotfix-1" || got.BranchSHA == "" {
		t.Errorf("branch fields not populated: %+v", got)
	}
	if got.TagName != "v1.2.3" || got.TagSHA == "" || got.TagURL == "" {
		t.Errorf("tag fields not populated: %+v", got)
	}
}

func TestProcessPostRelease_BranchError(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/git/ref/heads/main", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := ProcessPostRelease(ctx, PostReleaseRequest{
		OrgName:    "testorg",
		RepoNames:  []string{"repo1"},
		Ref:        "main",
		BranchName: "hotfix-1",
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
	if responses[0].RepositoryName != "repo1" {
		t.Errorf("RepositoryName = %q, want %q", responses[0].RepositoryName, "repo1")
	}
}

func TestProcessPostRelease_TagError(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/git/ref/heads/main", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"object": map[string]interface{}{"sha": "refsha123", "type": "commit"}},
	})
	mockServer.SetResponse("/repos/testorg/repo1/git/tags", testutils.MockResponse{
		StatusCode: http.StatusUnprocessableEntity,
		Body:       map[string]interface{}{"message": "tag already exists"},
	})

	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := ProcessPostRelease(ctx, PostReleaseRequest{
		OrgName:   "testorg",
		RepoNames: []string{"repo1"},
		Ref:       "main",
		TagName:   "v1.2.3",
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
}

// TestProcessPostRelease_MessageDefaultsToTagName verifies that when Message
// is left empty, the tag annotation message sent to the API falls back to
// TagName (pure request-building logic). testutils.MockGitHubServer doesn't
// record request bodies, so this uses a bespoke httptest server that does.
func TestProcessPostRelease_MessageDefaultsToTagName(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)
	ctx.Silent = true

	var capturedTagBody []byte
	captureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/testorg/repo1/git/ref/heads/main":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"object":{"sha":"refsha123","type":"commit"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/testorg/repo1/git/tags":
			capturedTagBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"sha":"tagcommitsha"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/testorg/repo1/git/refs":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"ref":"refs/tags/v9.9.9","url":"https://x/tags/v9.9.9","object":{"sha":"tagrefsha"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"not found"}`))
		}
	}))
	defer captureServer.Close()

	restore := service.SetGitHubBaseURLForTesting(captureServer.URL)
	defer restore()

	responses := ProcessPostRelease(ctx, PostReleaseRequest{
		OrgName:   "testorg",
		RepoNames: []string{"repo1"},
		Ref:       "main",
		TagName:   "v9.9.9",
		// Message intentionally left empty.
	})
	if len(responses) != 1 || responses[0].ErrorMessage != "" {
		t.Fatalf("unexpected responses: %+v", responses)
	}

	if capturedTagBody == nil {
		t.Fatal("expected a captured request body for the git/tags call")
	}
	var payload struct {
		Message string `json:"message"`
		Tag     string `json:"tag"`
	}
	if err := json.Unmarshal(capturedTagBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal captured tag request body: %v", err)
	}
	if payload.Message != "v9.9.9" {
		t.Errorf("Message = %q, want %q (defaulted from TagName)", payload.Message, "v9.9.9")
	}
}
