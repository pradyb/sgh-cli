// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package testutils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// MockGitHubServer provides a mock GitHub API server for testing
type MockGitHubServer struct {
	server    *httptest.Server
	mux       *http.ServeMux
	rateLimit *RateLimitInfo
	mu        sync.RWMutex
	requests  []MockRequest
	responses map[string]MockResponse
}

// RateLimitInfo represents GitHub API rate limit information
type RateLimitInfo struct {
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	Reset     time.Time `json:"reset"`
	Used      int       `json:"used"`
}

// MockRequest represents a request made to the mock server
type MockRequest struct {
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Headers   map[string]string `json:"headers"`
	Body      string            `json:"body"`
	Timestamp time.Time         `json:"timestamp"`
}

// MockResponse represents a response from the mock server
type MockResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       interface{}       `json:"body"`
	Delay      time.Duration     `json:"delay"`
}

// NewMockGitHubServer creates a new mock GitHub API server
func NewMockGitHubServer() *MockGitHubServer {
	mock := &MockGitHubServer{
		mux:       http.NewServeMux(),
		rateLimit: &RateLimitInfo{Limit: 5000, Remaining: 4999, Reset: time.Now().Add(time.Hour), Used: 1},
		requests:  make([]MockRequest, 0),
		responses: make(map[string]MockResponse),
	}

	// Set up default routes
	mock.setupDefaultRoutes()

	mock.server = httptest.NewServer(mock.mux)
	return mock
}

// setupDefaultRoutes sets up default API routes
func (m *MockGitHubServer) setupDefaultRoutes() {
	// Rate limit endpoint
	m.mux.HandleFunc("/rate_limit", m.handleRateLimit)

	// User endpoint
	m.mux.HandleFunc("/user", m.handleUser)

	// Organizations endpoint
	m.mux.HandleFunc("/orgs/", m.handleOrganizations)

	// Repositories endpoint
	m.mux.HandleFunc("/repos/", m.handleRepositories)

	// GraphQL endpoint
	m.mux.HandleFunc("/graphql", m.handleGraphQL)

	// Search endpoint
	m.mux.HandleFunc("/search/", m.handleSearch)

	// Default handler for unmatched routes
	m.mux.HandleFunc("/", m.handleDefault)
}

// URL returns the base URL of the mock server
func (m *MockGitHubServer) URL() string {
	return m.server.URL
}

// Close shuts down the mock server
func (m *MockGitHubServer) Close() {
	m.server.Close()
}

// GetRequests returns all requests made to the server
func (m *MockGitHubServer) GetRequests() []MockRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	requests := make([]MockRequest, len(m.requests))
	copy(requests, m.requests)
	return requests
}

// ClearRequests clears the request history
func (m *MockGitHubServer) ClearRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = make([]MockRequest, 0)
}

// SetResponse sets a custom response for a specific path
func (m *MockGitHubServer) SetResponse(path string, response MockResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[path] = response
}

// SetRateLimit sets the rate limit information
func (m *MockGitHubServer) SetRateLimit(limit, remaining int, reset time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rateLimit = &RateLimitInfo{
		Limit:     limit,
		Remaining: remaining,
		Reset:     reset,
		Used:      limit - remaining,
	}
}

// recordRequest records a request for later inspection
func (m *MockGitHubServer) recordRequest(r *http.Request, body string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	headers := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	m.requests = append(m.requests, MockRequest{
		Method:    r.Method,
		Path:      r.URL.Path,
		Headers:   headers,
		Body:      body,
		Timestamp: time.Now(),
	})
}

// writeJSONResponse writes a JSON response with standard headers
func (m *MockGitHubServer) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", m.rateLimit.Limit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", m.rateLimit.Remaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", m.rateLimit.Reset.Unix()))
	w.Header().Set("X-RateLimit-Used", fmt.Sprintf("%d", m.rateLimit.Used))
	w.Header().Set("X-GitHub-Request-Id", "mock-request-id")
	w.Header().Set("X-GitHub-Media-Type", "github.v3+json")

	w.WriteHeader(statusCode)

	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// handleRateLimit handles the /rate_limit endpoint
func (m *MockGitHubServer) handleRateLimit(w http.ResponseWriter, r *http.Request) {
	m.recordRequest(r, "")

	response := map[string]interface{}{
		"resources": map[string]interface{}{
			"core": map[string]interface{}{
				"limit":     m.rateLimit.Limit,
				"remaining": m.rateLimit.Remaining,
				"reset":     m.rateLimit.Reset.Unix(),
				"used":      m.rateLimit.Used,
			},
			"search": map[string]interface{}{
				"limit":     30,
				"remaining": 30,
				"reset":     time.Now().Add(time.Minute).Unix(),
				"used":      0,
			},
			"graphql": map[string]interface{}{
				"limit":     5000,
				"remaining": 5000,
				"reset":     time.Now().Add(time.Hour).Unix(),
				"used":      0,
			},
		},
	}

	m.writeJSONResponse(w, http.StatusOK, response)
}

// handleUser handles the /user endpoint
func (m *MockGitHubServer) handleUser(w http.ResponseWriter, r *http.Request) {
	m.recordRequest(r, "")

	user := map[string]interface{}{
		"login":        "testuser",
		"id":           12345,
		"node_id":      "MDQ6VXNlcjEyMzQ1",
		"avatar_url":   "https://avatars.githubusercontent.com/u/12345?v=4",
		"url":          "https://api.github.com/users/testuser",
		"html_url":     "https://github.com/testuser",
		"type":         "User",
		"site_admin":   false,
		"name":         "Test User",
		"email":        "test@example.com",
		"public_repos": 10,
		"public_gists": 0,
		"followers":    5,
		"following":    10,
		"created_at":   "2020-01-01T00:00:00Z",
		"updated_at":   "2023-01-01T00:00:00Z",
	}

	m.writeJSONResponse(w, http.StatusOK, user)
}

// handleOrganizations handles organization-related endpoints
func (m *MockGitHubServer) handleOrganizations(w http.ResponseWriter, r *http.Request) {
	m.recordRequest(r, "")

	// Check if we have a custom response for this path
	m.mu.RLock()
	if response, exists := m.responses[r.URL.Path]; exists {
		m.mu.RUnlock()
		if response.Delay > 0 {
			time.Sleep(response.Delay)
		}
		for key, value := range response.Headers {
			w.Header().Set(key, value)
		}
		w.WriteHeader(response.StatusCode)
		if response.Body != nil {
			json.NewEncoder(w).Encode(response.Body)
		}
		return
	}
	m.mu.RUnlock()

	path := r.URL.Path
	if strings.Contains(path, "/repos") {
		repos := []map[string]interface{}{
			{
				"id":        123456,
				"node_id":   "R_kgDOG123456",
				"name":      "test-repo-1",
				"full_name": "testorg/test-repo-1",
				"private":   false,
				"owner": map[string]interface{}{
					"login": "testorg",
					"id":    123,
				},
				"html_url":          "https://github.com/testorg/test-repo-1",
				"description":       "Test repository 1",
				"fork":              false,
				"url":               "https://api.github.com/repos/testorg/test-repo-1",
				"created_at":        "2023-01-01T00:00:00Z",
				"updated_at":        "2023-01-01T00:00:00Z",
				"pushed_at":         "2023-01-01T00:00:00Z",
				"git_url":           "git://github.com/testorg/test-repo-1.git",
				"ssh_url":           "git@github.com:testorg/test-repo-1.git",
				"clone_url":         "https://github.com/testorg/test-repo-1.git",
				"svn_url":           "https://svn.github.com/testorg/test-repo-1",
				"homepage":          nil,
				"size":              100,
				"stargazers_count":  5,
				"watchers_count":    5,
				"language":          "Go",
				"has_issues":        true,
				"has_projects":      true,
				"has_downloads":     true,
				"has_wiki":          true,
				"has_pages":         false,
				"has_discussions":   false,
				"forks_count":       2,
				"mirror_url":        nil,
				"archived":          false,
				"disabled":          false,
				"open_issues_count": 3,
				"license": map[string]interface{}{
					"key":     "mit",
					"name":    "MIT License",
					"url":     "https://api.github.com/licenses/mit",
					"spdx_id": "MIT",
					"node_id": "MDc6TGljZW5zZTEz",
				},
				"allow_forking":               true,
				"is_template":                 false,
				"web_commit_signoff_required": false,
				"topics":                      []string{"go", "cli", "github"},
				"visibility":                  "public",
				"forks":                       2,
				"open_issues":                 3,
				"watchers":                    5,
				"default_branch":              "main",
			},
			{
				"id":        123457,
				"node_id":   "R_kgDOG123457",
				"name":      "test-repo-2",
				"full_name": "testorg/test-repo-2",
				"private":   true,
				"owner": map[string]interface{}{
					"login": "testorg",
					"id":    123,
				},
				"html_url":                    "https://github.com/testorg/test-repo-2",
				"description":                 "Test repository 2",
				"fork":                        false,
				"url":                         "https://api.github.com/repos/testorg/test-repo-2",
				"created_at":                  "2023-01-01T00:00:00Z",
				"updated_at":                  "2023-01-01T00:00:00Z",
				"pushed_at":                   "2023-01-01T00:00:00Z",
				"git_url":                     "git://github.com/testorg/test-repo-2.git",
				"ssh_url":                     "git@github.com:testorg/test-repo-2.git",
				"clone_url":                   "https://github.com/testorg/test-repo-2.git",
				"svn_url":                     "https://svn.github.com/testorg/test-repo-2",
				"homepage":                    nil,
				"size":                        50,
				"stargazers_count":            2,
				"watchers_count":              2,
				"language":                    "JavaScript",
				"has_issues":                  true,
				"has_projects":                true,
				"has_downloads":               true,
				"has_wiki":                    true,
				"has_pages":                   false,
				"has_discussions":             false,
				"forks_count":                 1,
				"mirror_url":                  nil,
				"archived":                    false,
				"disabled":                    false,
				"open_issues_count":           1,
				"license":                     nil,
				"allow_forking":               true,
				"is_template":                 false,
				"web_commit_signoff_required": false,
				"topics":                      []string{"javascript", "web"},
				"visibility":                  "private",
				"forks":                       1,
				"open_issues":                 1,
				"watchers":                    2,
				"default_branch":              "main",
			},
		}

		m.writeJSONResponse(w, http.StatusOK, repos)
		return
	}

	// Default organization response
	org := map[string]interface{}{
		"login":                     "testorg",
		"id":                        123,
		"node_id":                   "MDEyOk9yZ2FuaXphdGlvbjEyMw==",
		"url":                       "https://api.github.com/orgs/testorg",
		"repos_url":                 "https://api.github.com/orgs/testorg/repos",
		"events_url":                "https://api.github.com/orgs/testorg/events",
		"hooks_url":                 "https://api.github.com/orgs/testorg/hooks",
		"issues_url":                "https://api.github.com/orgs/testorg/issues",
		"members_url":               "https://api.github.com/orgs/testorg/members{/member}",
		"public_members_url":        "https://api.github.com/orgs/testorg/public_members{/member}",
		"avatar_url":                "https://avatars.githubusercontent.com/u/123?v=4",
		"description":               "Test organization",
		"name":                      "Test Organization",
		"company":                   nil,
		"blog":                      "",
		"location":                  nil,
		"email":                     "test@testorg.com",
		"twitter_username":          nil,
		"is_verified":               false,
		"has_organization_projects": true,
		"has_repository_projects":   true,
		"public_repos":              10,
		"public_gists":              0,
		"followers":                 5,
		"following":                 0,
		"html_url":                  "https://github.com/testorg",
		"created_at":                "2020-01-01T00:00:00Z",
		"updated_at":                "2023-01-01T00:00:00Z",
		"type":                      "Organization",
	}

	m.writeJSONResponse(w, http.StatusOK, org)
}

// handleRepositories handles repository-related endpoints
func (m *MockGitHubServer) handleRepositories(w http.ResponseWriter, r *http.Request) {
	m.recordRequest(r, "")

	path := r.URL.Path

	// Check if we have a custom response for this path
	m.mu.RLock()
	if response, exists := m.responses[r.URL.Path]; exists {
		m.mu.RUnlock()
		if response.Delay > 0 {
			time.Sleep(response.Delay)
		}
		for key, value := range response.Headers {
			w.Header().Set(key, value)
		}
		m.writeJSONResponse(w, response.StatusCode, response.Body)
		return
	}
	m.mu.RUnlock()

	// Handle list branches
	if r.Method == "GET" && strings.Contains(path, "/branches") && !strings.Contains(path, "/protection") {
		branches := []map[string]interface{}{
			{
				"name":      "main",
				"protected": true,
				"commit": map[string]interface{}{
					"sha": "abc123",
					"url": "https://api.github.com/repos/testorg/test-repo/commits/abc123",
				},
			},
			{
				"name":      "develop",
				"protected": false,
				"commit": map[string]interface{}{
					"sha": "def456",
					"url": "https://api.github.com/repos/testorg/test-repo/commits/def456",
				},
			},
		}
		m.writeJSONResponse(w, http.StatusOK, branches)
		return
	}

	// Handle branch creation
	if r.Method == "POST" && strings.Contains(path, "/git/refs") {
		response := map[string]interface{}{
			"ref":     "refs/heads/test-branch",
			"node_id": "MDM6UmVmMTIzNDU2Nzg5OjEyMzQ1Njc4OQ==",
			"url":     "https://api.github.com/repos/testorg/test-repo/git/refs/heads/test-branch",
			"object": map[string]interface{}{
				"sha":  "1234567890abcdef1234567890abcdef12345678",
				"type": "commit",
				"url":  "https://api.github.com/repos/testorg/test-repo/git/commits/1234567890abcdef1234567890abcdef12345678",
			},
		}
		m.writeJSONResponse(w, http.StatusCreated, response)
		return
	}

	// Handle protected branch operations
	if r.Method == "PUT" && strings.Contains(path, "/protection") {
		response := map[string]interface{}{
			"url":                              "https://api.github.com/repos/testorg/test-repo/branches/main/protection",
			"enforce_admins":                   map[string]interface{}{"url": "https://api.github.com/repos/testorg/test-repo/branches/main/protection/enforce_admins", "enabled": true},
			"required_pull_request_reviews":    map[string]interface{}{"url": "https://api.github.com/repos/testorg/test-repo/branches/main/protection/required_pull_request_reviews", "dismiss_stale_reviews": true, "require_code_owner_reviews": false, "require_last_push_approval": true, "required_approving_review_count": 1},
			"required_signatures":              map[string]interface{}{"url": "https://api.github.com/repos/testorg/test-repo/branches/main/protection/required_signatures", "enabled": false},
			"required_linear_history":          map[string]interface{}{"enabled": false},
			"allow_force_pushes":               map[string]interface{}{"enabled": false},
			"allow_deletions":                  map[string]interface{}{"enabled": false},
			"block_creations":                  map[string]interface{}{"enabled": false},
			"required_conversation_resolution": map[string]interface{}{"enabled": true},
			"lock_branch":                      map[string]interface{}{"enabled": false},
			"allow_fork_syncing":               map[string]interface{}{"enabled": false},
		}
		m.writeJSONResponse(w, http.StatusOK, response)
		return
	}

	// Default repository response
	repo := map[string]interface{}{
		"id":        123456,
		"node_id":   "R_kgDOG123456",
		"name":      "test-repo",
		"full_name": "testorg/test-repo",
		"private":   false,
		"owner": map[string]interface{}{
			"login": "testorg",
			"id":    123,
		},
		"html_url":          "https://github.com/testorg/test-repo",
		"description":       "Test repository",
		"fork":              false,
		"url":               "https://api.github.com/repos/testorg/test-repo",
		"created_at":        "2023-01-01T00:00:00Z",
		"updated_at":        "2023-01-01T00:00:00Z",
		"pushed_at":         "2023-01-01T00:00:00Z",
		"git_url":           "git://github.com/testorg/test-repo.git",
		"ssh_url":           "git@github.com:testorg/test-repo.git",
		"clone_url":         "https://github.com/testorg/test-repo.git",
		"svn_url":           "https://svn.github.com/testorg/test-repo",
		"homepage":          nil,
		"size":              100,
		"stargazers_count":  5,
		"watchers_count":    5,
		"language":          "Go",
		"has_issues":        true,
		"has_projects":      true,
		"has_downloads":     true,
		"has_wiki":          true,
		"has_pages":         false,
		"has_discussions":   false,
		"forks_count":       2,
		"mirror_url":        nil,
		"archived":          false,
		"disabled":          false,
		"open_issues_count": 3,
		"license": map[string]interface{}{
			"key":     "mit",
			"name":    "MIT License",
			"url":     "https://api.github.com/licenses/mit",
			"spdx_id": "MIT",
			"node_id": "MDc6TGljZW5zZTEz",
		},
		"allow_forking":               true,
		"is_template":                 false,
		"web_commit_signoff_required": false,
		"topics":                      []string{"go", "cli", "github"},
		"visibility":                  "public",
		"forks":                       2,
		"open_issues":                 3,
		"watchers":                    5,
		"default_branch":              "main",
	}

	m.writeJSONResponse(w, http.StatusOK, repo)
}

// handleGraphQL handles GraphQL API requests
func (m *MockGitHubServer) handleGraphQL(w http.ResponseWriter, r *http.Request) {
	m.recordRequest(r, "")

	// Parse the GraphQL query from request body
	var request struct {
		Query     string                 `json:"query"`
		Variables map[string]interface{} `json:"variables"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		m.writeJSONResponse(w, http.StatusBadRequest, map[string]interface{}{
			"errors": []map[string]interface{}{
				{"message": "Invalid JSON"},
			},
		})
		return
	}

	// Mock different GraphQL responses based on query
	response := m.mockGraphQLResponse(request.Query, request.Variables)
	m.writeJSONResponse(w, http.StatusOK, response)
}

// handleSearch handles search API requests
func (m *MockGitHubServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	m.recordRequest(r, "")

	path := r.URL.Path

	if strings.Contains(path, "/repositories") {
		repos := []map[string]interface{}{
			{
				"id":        123456,
				"node_id":   "R_kgDOG123456",
				"name":      "search-result-repo",
				"full_name": "testorg/search-result-repo",
				"private":   false,
				"owner": map[string]interface{}{
					"login": "testorg",
					"id":    123,
				},
				"html_url":          "https://github.com/testorg/search-result-repo",
				"description":       "Repository found in search",
				"fork":              false,
				"url":               "https://api.github.com/repos/testorg/search-result-repo",
				"created_at":        "2023-01-01T00:00:00Z",
				"updated_at":        "2023-01-01T00:00:00Z",
				"pushed_at":         "2023-01-01T00:00:00Z",
				"git_url":           "git://github.com/testorg/search-result-repo.git",
				"ssh_url":           "git@github.com:testorg/search-result-repo.git",
				"clone_url":         "https://github.com/testorg/search-result-repo.git",
				"svn_url":           "https://svn.github.com/testorg/search-result-repo",
				"homepage":          nil,
				"size":              100,
				"stargazers_count":  5,
				"watchers_count":    5,
				"language":          "Go",
				"has_issues":        true,
				"has_projects":      true,
				"has_downloads":     true,
				"has_wiki":          true,
				"has_pages":         false,
				"has_discussions":   false,
				"forks_count":       2,
				"mirror_url":        nil,
				"archived":          false,
				"disabled":          false,
				"open_issues_count": 3,
				"license": map[string]interface{}{
					"key":     "mit",
					"name":    "MIT License",
					"url":     "https://api.github.com/licenses/mit",
					"spdx_id": "MIT",
					"node_id": "MDc6TGljZW5zZTEz",
				},
				"allow_forking":               true,
				"is_template":                 false,
				"web_commit_signoff_required": false,
				"topics":                      []string{"go", "search"},
				"visibility":                  "public",
				"forks":                       2,
				"open_issues":                 3,
				"watchers":                    5,
				"default_branch":              "main",
			},
		}

		searchResponse := map[string]interface{}{
			"total_count":        1,
			"incomplete_results": false,
			"items":              repos,
		}

		m.writeJSONResponse(w, http.StatusOK, searchResponse)
		return
	}

	// Default search response
	searchResponse := map[string]interface{}{
		"total_count":        0,
		"incomplete_results": false,
		"items":              []interface{}{},
	}

	m.writeJSONResponse(w, http.StatusOK, searchResponse)
}

// handleDefault handles unmatched routes
func (m *MockGitHubServer) handleDefault(w http.ResponseWriter, r *http.Request) {
	m.recordRequest(r, "")

	// Check if we have a custom response for this path
	m.mu.RLock()
	if response, exists := m.responses[r.URL.Path]; exists {
		m.mu.RUnlock()

		// Apply delay if specified
		if response.Delay > 0 {
			time.Sleep(response.Delay)
		}

		// Set custom headers
		for key, value := range response.Headers {
			w.Header().Set(key, value)
		}

		w.WriteHeader(response.StatusCode)

		if response.Body != nil {
			json.NewEncoder(w).Encode(response.Body)
		}
		return
	}
	m.mu.RUnlock()

	// Default 404 response
	m.writeJSONResponse(w, http.StatusNotFound, map[string]interface{}{
		"message":           "Not Found",
		"documentation_url": "https://docs.github.com/rest",
	})
}

// mockGraphQLResponse generates mock GraphQL responses
func (m *MockGitHubServer) mockGraphQLResponse(query string, variables map[string]interface{}) map[string]interface{} {
	// This is a simplified GraphQL response generator
	// In a real implementation, you'd parse the query and generate appropriate responses

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"organization": map[string]interface{}{
				"repositories": map[string]interface{}{
					"nodes": []map[string]interface{}{
						{
							"name": "test-repo-1",
							"defaultBranchRef": map[string]interface{}{
								"name": "main",
							},
						},
						{
							"name": "test-repo-2",
							"defaultBranchRef": map[string]interface{}{
								"name": "main",
							},
						},
					},
					"totalCount": 2,
				},
			},
		},
	}

	return response
}
