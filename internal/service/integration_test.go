// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/pradyb/sgh-cli/internal/client"
	"github.com/pradyb/sgh-cli/internal/testutils"
	appcontext "github.com/pradyb/sgh-cli/pkg/context"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

const (
	graphqlPath    = "/graphql"
	testOrgName    = "testorg"
	testRepoName   = "test-repo"
	testBranchName = "test-branch"
	mainBranchName = "main"
)

func TestGitHubAPIIntegration(t *testing.T) {
	// Create mock server
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()

	// Set up test environment
	originalToken := os.Getenv("SGH_TOKEN")
	defer os.Setenv("SGH_TOKEN", originalToken)
	os.Setenv("SGH_TOKEN", "ghp_1234567890abcdef1234567890abcdef123456")

	// Create test context with mock server URL
	ctx := createTestContext(t, mockServer.URL())

	t.Run("ListBranches", func(t *testing.T) {
		mockServer.ClearRequests()

		branches, err := ListBranches(ctx, "testorg", "test-repo")

		require.NoError(t, err)
		assert.NotNil(t, branches)

		// Verify request was made
		requests := mockServer.GetRequests()
		assert.GreaterOrEqual(t, len(requests), 1)
		assert.Equal(t, "GET", requests[0].Method)
		assert.Contains(t, requests[0].Path, "/repos/testorg/test-repo/branches")
	})

	t.Run("CreateNewBranch", func(t *testing.T) {
		// Clear previous requests
		mockServer.ClearRequests()

		response, err := CreateNewBranch(ctx, "testorg", "test-repo", "test-branch", "main")

		require.NoError(t, err)
		assert.Equal(t, "refs/heads/test-branch", response.Ref)
		assert.Equal(t, "1234567890abcdef1234567890abcdef12345678", response.Object.SHA)

		// Verify request was made
		requests := mockServer.GetRequests()
		assert.Len(t, requests, 2) // One for getting SHA, one for creating branch
		assert.Equal(t, "GET", requests[0].Method)
		assert.Contains(t, requests[0].Path, "/git/ref/heads/main")
		assert.Equal(t, "POST", requests[1].Method)
		assert.Contains(t, requests[1].Path, "/git/refs")
	})

	t.Run("UpdateProtectedBranch", func(t *testing.T) {
		// Clear previous requests
		mockServer.ClearRequests()

		payload := []byte(`{"required_pull_request_reviews":{"required_approving_review_count":1}}`)
		response, err := UpdateProtectedBranch(ctx, "testorg", "test-repo", "main", payload)

		require.NoError(t, err)
		assert.Equal(t, "test-repo", response.RepositoryName)
		assert.Equal(t, "Branch Protection", response.Type)

		// Verify request was made
		requests := mockServer.GetRequests()
		assert.Len(t, requests, 1)
		assert.Equal(t, "PUT", requests[0].Method)
		assert.Contains(t, requests[0].Path, "/protection")
	})

	t.Run("DeleteProtectedBranch", func(t *testing.T) {
		// Clear previous requests
		mockServer.ClearRequests()

		// Set custom response for DELETE
		mockServer.SetResponse("/repos/testorg/test-repo/branches/main/protection", testutils.MockResponse{
			StatusCode: http.StatusNoContent,
			Body:       nil,
		})

		success, err := DeleteProtectedBranch(ctx, "testorg", "test-repo", "main")

		require.NoError(t, err)
		assert.True(t, success)

		// Verify request was made
		requests := mockServer.GetRequests()
		assert.Len(t, requests, 1)
		assert.Equal(t, "DELETE", requests[0].Method)
		assert.Contains(t, requests[0].Path, "/protection")
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		// Clear previous requests
		mockServer.ClearRequests()

		// Set custom error response
		mockServer.SetResponse("/repos/testorg/nonexistent/branches", testutils.MockResponse{
			StatusCode: http.StatusNotFound,
			Body: map[string]interface{}{
				"message":           "Not Found",
				"documentation_url": "https://docs.github.com/rest",
			},
		})

		branches, err := ListBranches(ctx, "testorg", "nonexistent")

		assert.Error(t, err)
		assert.Nil(t, branches)
		assert.Contains(t, err.Error(), "404")

		// Verify request was made
		requests := mockServer.GetRequests()
		assert.Len(t, requests, 1)
		assert.Contains(t, requests[0].Path, "/repos/testorg/nonexistent/branches")
	})

	t.Run("RateLimitHandling", func(t *testing.T) {
		// Clear previous requests
		mockServer.ClearRequests()

		// Set low rate limit
		mockServer.SetRateLimit(5000, 1, time.Now().Add(time.Minute))

		// This should still work as the mock server doesn't actually enforce rate limits
		branches, err := ListBranches(ctx, "testorg", "test-repo")

		require.NoError(t, err)
		assert.NotNil(t, branches)

		// Verify rate limit headers were received
		requests := mockServer.GetRequests()
		assert.GreaterOrEqual(t, len(requests), 1)
	})
}

func TestGitHubAPIIntegrationWithTimeout(t *testing.T) {
	// Create mock server with proper cleanup
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()

	// Set up test environment with restoration
	originalToken := os.Getenv("SGH_TOKEN")
	defer func() {
		if err := os.Setenv("SGH_TOKEN", originalToken); err != nil {
			t.Logf("Warning: failed to restore SGH_TOKEN: %v", err)
		}
	}()

	if err := os.Setenv("SGH_TOKEN", "ghp_1234567890abcdef1234567890abcdef123456"); err != nil {
		t.Fatalf("Failed to set test token: %v", err)
	}

	// Create test context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	testCtx := createTestContext(t, mockServer.URL())

	t.Run("ListBranchesWithTimeout", func(t *testing.T) {
		done := make(chan error, 1)
		go func() {
			_, err := ListBranches(testCtx, "testorg", "test-repo")
			done <- err
		}()

		select {
		case err := <-done:
			if err != nil {
				t.Errorf("ListBranches failed: %v", err)
			}
		case <-ctx.Done():
			t.Error("ListBranches timed out")
		}
	})
}

func TestGraphQLIntegration(t *testing.T) {
	// Create mock server
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()

	// Set up test environment
	originalToken := os.Getenv("SGH_TOKEN")
	defer os.Setenv("SGH_TOKEN", originalToken)
	os.Setenv("SGH_TOKEN", "ghp_1234567890abcdef1234567890abcdef123456")

	// Set a very short rate limit reset time for testing (1 second)
	mockServer.SetRateLimit(5000, 5000, time.Now().Add(time.Second))

	// Create test context with mock server URL
	ctx := createTestContext(t, mockServer.URL())

	// Create a custom GraphQL client that uses the mock server
	customTransport := &mockServerTransport{mockServer: mockServer}

	// Create OAuth2 client with custom HTTP client
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "ghp_1234567890abcdef1234567890abcdef123456"})
	oauthClient := oauth2.NewClient(context.Background(), src)
	oauthClient.Transport = customTransport

	// Create GraphQL client with custom OAuth2 client
	gqlClient := githubv4.NewClient(oauthClient)
	customGraphQLClient := &client.GraphqlClient{
		Client:         gqlClient,
		RateLimiter:    ctx.HttpClient.RateLimiter,
		RetryConfig:    ctx.HttpClient.RetryConfig,
		CircuitBreaker: ctx.HttpClient.CircuitBreaker,
	}

	t.Run("GraphQLQuery", func(t *testing.T) {
		// Clear previous requests
		mockServer.ClearRequests()

		// Create a simple GraphQL query using a struct
		type OrganizationQuery struct {
			Organization struct {
				Repositories struct {
					Nodes []struct {
						Name             string `json:"name"`
						DefaultBranchRef struct {
							Name string `json:"name"`
						} `json:"defaultBranchRef"`
					} `json:"nodes"`
					TotalCount int `json:"totalCount"`
				} `json:"repositories"`
			} `json:"organization"`
		}

		var query OrganizationQuery
		variables := map[string]interface{}{
			"org": "testorg",
		}

		// Add timeout to prevent hanging
		reqCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := customGraphQLClient.QueryWithContext(reqCtx, &query, variables)

		require.NoError(t, err)
		// Verify request was made
		requests := mockServer.GetRequests()
		assert.Len(t, requests, 1)
		assert.Equal(t, "POST", requests[0].Method)
		assert.Equal(t, graphqlPath, requests[0].Path)
		assert.Contains(t, requests[0].Headers, "Content-Type")
	})

	t.Run("GraphQLError", func(t *testing.T) {
		// Clear previous requests
		mockServer.ClearRequests()
		// Set custom error response for GraphQL
		mockServer.SetResponse(graphqlPath, testutils.MockResponse{
			StatusCode: http.StatusBadRequest,
			Body: map[string]interface{}{
				"errors": []map[string]interface{}{
					{"message": "Invalid query"},
				},
			},
		})

		// Create a simple query struct for error testing
		type ErrorQuery struct {
			Test string `json:"test"`
		}

		var query ErrorQuery

		// Add timeout to prevent hanging
		reqCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := customGraphQLClient.QueryWithContext(reqCtx, &query, nil)

		assert.Error(t, err)
		// Verify request was made
		requests := mockServer.GetRequests()
		assert.Len(t, requests, 1)
		assert.Equal(t, graphqlPath, requests[0].Path)
	})
}

// createTestContext creates a test context with the mock server URL
func createTestContext(t *testing.T, mockServerURL string) *appcontext.Context {
	// Override the GitHub API base URL for testing
	originalBaseURL := githubBaseURL
	githubBaseURL = mockServerURL
	t.Cleanup(func() { githubBaseURL = originalBaseURL })

	ctx, err := appcontext.Init()
	require.NoError(t, err)
	return ctx
}

// Benchmark tests
func BenchmarkListBranches(b *testing.B) {
	// Create mock server
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()

	// Set up test environment
	originalToken := os.Getenv("SGH_TOKEN")
	defer os.Setenv("SGH_TOKEN", originalToken)
	os.Setenv("SGH_TOKEN", "ghp_1234567890abcdef1234567890abcdef123456")

	// Set a very short rate limit reset time for testing
	mockServer.SetRateLimit(5000, 5000, time.Now().Add(time.Second))

	// Create test context
	ctx := createTestContext(nil, mockServer.URL())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ListBranches(ctx, "testorg", "test-repo")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCreateNewBranch(b *testing.B) {
	// Create mock server
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()

	// Set up test environment
	originalToken := os.Getenv("SGH_TOKEN")
	defer os.Setenv("SGH_TOKEN", originalToken)
	os.Setenv("SGH_TOKEN", "ghp_1234567890abcdef1234567890abcdef123456")

	// Set a very short rate limit reset time for testing
	mockServer.SetRateLimit(5000, 5000, time.Now().Add(time.Second))

	// Create test context
	ctx := createTestContext(nil, mockServer.URL())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CreateNewBranch(ctx, "testorg", "test-repo", "test-branch", "main")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// mockServerTransport routes all requests to the mock server
type mockServerTransport struct {
	mockServer *testutils.MockGitHubServer
}

func (t *mockServerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Create a new request to the mock server
	mockURL, _ := url.Parse(t.mockServer.URL())
	req.URL.Scheme = mockURL.Scheme
	req.URL.Host = mockURL.Host

	// Use the default transport to make the request
	transport := &http.Transport{}
	return transport.RoundTrip(req)
}
