// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

// Package servicetest provides helpers for testing code that talks to the
// GitHub API through internal/service.
//
// It lives in its own package rather than in internal/service because it
// imports "testing" and "net/http/httptest". Anything internal/service
// imports is linked into the sgh binary, since every pkg/* command package
// depends on it; keeping these helpers here means the test-only machinery
// never reaches a user's build. The files here are ordinary (non-_test.go)
// sources so that other packages' tests can import them, but nothing in the
// production import graph references this package.
package servicetest

import (
	"net/http"
	"os"
	"runtime"
	"testing"

	"github.com/shurcooL/githubv4"

	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/internal/testutils"
	appcontext "github.com/pradyb/sgh-cli/pkg/context"
)

// TestToken is a syntactically valid GitHub personal access token used to
// satisfy appcontext.Init's token validation. It is a well-known dummy value
// with the shape of a classic PAT — the "ghp_" prefix followed by 36
// characters — and is not a credential. The literal is split so it does not
// read as one contiguous token to secret scanners.
const TestToken = "ghp_" + "1234567890abcdef1234567890abcdef1234"

// SetGitHubBaseURL points internal/service at the given REST API base URL and
// returns a function that restores the previous value.
func SetGitHubBaseURL(url string) (restore func()) {
	return service.SetGitHubBaseURLForTesting(url)
}

// NewMockContext builds an *appcontext.Context whose REST and GraphQL clients
// both point at the given mock server, so that code exercising the GitHub API
// can be tested end-to-end without reaching the real one.
//
// It sets SGH_TOKEN and the REST base URL for the duration of the test and
// restores both via t.Cleanup. It also points the OS home directory at a fresh
// temp dir (via HOME/USERPROFILE) so ctx.Config starts out empty and
// deterministic instead of loading the real user's sgh.json — important
// because several pkg/* functions (e.g. anything going through
// internal/processor.ProcessRepositoriesOperation) consult ctx.Config for repo
// selection and include/exclude patterns.
func NewMockContext(t *testing.T, mockServer *testutils.MockGitHubServer) *appcontext.Context {
	t.Helper()

	originalToken := os.Getenv("SGH_TOKEN")
	if err := os.Setenv("SGH_TOKEN", TestToken); err != nil {
		t.Fatalf("failed to set SGH_TOKEN: %v", err)
	}
	t.Cleanup(func() { os.Setenv("SGH_TOKEN", originalToken) })

	homeVar := "HOME"
	if runtime.GOOS == "windows" {
		homeVar = "USERPROFILE"
	}
	originalHome := os.Getenv(homeVar)
	if err := os.Setenv(homeVar, t.TempDir()); err != nil {
		t.Fatalf("failed to isolate home dir: %v", err)
	}
	t.Cleanup(func() { os.Setenv(homeVar, originalHome) })

	t.Cleanup(SetGitHubBaseURL(mockServer.URL()))

	ctx, err := appcontext.Init()
	if err != nil {
		t.Fatalf("failed to init context: %v", err)
	}

	// Redirect the GraphQL client at the mock server's /graphql endpoint,
	// reusing the same authenticated transport as the REST client.
	ctx.GraphqlClient.Client = githubv4.NewEnterpriseClient(
		mockServer.URL()+"/graphql",
		&http.Client{Transport: ctx.HttpClient.Client.Transport},
	)

	return ctx
}
