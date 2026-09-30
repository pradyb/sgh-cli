// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package service

import (
	"net/http"
	"os"
	"runtime"
	"testing"

	"github.com/shurcooL/githubv4"

	internalconfig "github.com/pradyb/sgh-cli/internal/config"
	"github.com/pradyb/sgh-cli/internal/testutils"
	appcontext "github.com/pradyb/sgh-cli/pkg/context"
	"github.com/pradyb/sgh-cli/pkg/keyring"
)

// TestMain isolates HOME so every appcontext.Init() call in this package reads and
// writes a fresh, empty config, never the developer's real ~/.config/sgh/sgh.json —
// important now that Init() can migrate a plaintext token and save the config as a side
// effect. Most tests here already isolate HOME per-test too (see NewMockContext below),
// but not all (internal/service/integration_test.go's createTestContext does not); this
// backstops those. Also forces an in-memory fake keyring, so no test touches the real
// OS keyring.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "sgh-service-test-home")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(home)
	homeVar := "HOME"
	if runtime.GOOS == "windows" {
		homeVar = "USERPROFILE"
	}
	os.Setenv(homeVar, home)
	internalconfig.TokenKeyring = keyring.NewFake()
	os.Exit(m.Run())
}

// TestToken mirrors servicetest.TestToken. See NewMockContext below for why
// this package cannot simply use servicetest's copy.
const TestToken = "ghp_" + "1234567890abcdef1234567890abcdef1234"

// NewMockContext is a local copy of servicetest.NewMockContext.
//
// Tests in this package cannot import internal/service/servicetest, because
// servicetest imports internal/service and these tests are in package service
// itself — that is an import cycle. Every other package's tests should use
// servicetest.NewMockContext rather than copying this again.
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

	ctx.GraphqlClient.Client = githubv4.NewEnterpriseClient(
		mockServer.URL()+"/graphql",
		&http.Client{Transport: ctx.HttpClient.Client.Transport},
	)

	return ctx
}
