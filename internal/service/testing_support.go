// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package service

import (
	"net/http"
	"os"
	"runtime"
	"testing"

	"github.com/shurcooL/githubv4"

	"github.com/pradyb/sgh-cli/internal/testutils"
	appcontext "github.com/pradyb/sgh-cli/pkg/context"
)

// TestToken is a syntactically valid GitHub token used to satisfy
// appcontext.Init's token validation in tests. It is a well-known dummy
// value, not a credential; the literal is split so it doesn't read as one
// contiguous token to secret scanners (this file can't be named _test.go,
// since it must be importable by other packages' tests).
const TestToken = "ghp_" + "1234567890abcdef1234567890abcdef123456"

// SetGitHubBaseURLForTesting overrides the GitHub REST API base URL, allowing
// callers (typically tests in this module) to point requests at a mock
// server. It returns a restore function that puts the original URL back.
func SetGitHubBaseURLForTesting(url string) (restore func()) {
	original := githubBaseURL
	githubBaseURL = url
	return func() { githubBaseURL = original }
}

// NewMockContext builds an *appcontext.Context whose REST and GraphQL
// clients both point at the given mock server, so that code exercising this
// package can be tested end-to-end without hitting the real GitHub API.
//
// It sets SGH_TOKEN and the REST base URL for the duration of the test and
// restores both via t.Cleanup. It also points the OS home directory at a
// fresh temp dir (via HOME/USERPROFILE) so ctx.Config starts out empty and
// deterministic instead of loading the real user's sgh.json — important
// because several pkg/* functions (e.g. anything going through
// internal/processor.ProcessRepositoriesOperation) consult ctx.Config for
// repo selection and include/exclude patterns.
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

	t.Cleanup(SetGitHubBaseURLForTesting(mockServer.URL()))

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
