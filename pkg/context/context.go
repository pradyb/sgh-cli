// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package context

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"

	"github.com/pradyb/sgh-cli/internal/client"
	"github.com/pradyb/sgh-cli/internal/config"
	"github.com/pradyb/sgh-cli/pkg/logger"
)

type Context struct {
	Config        *config.Config
	HttpClient    *client.HttpClient
	GraphqlClient *client.GraphqlClient

	Verbose     bool
	LogResponse bool
	Compact     bool
	JSON        bool
	DryRun      bool
	Silent      bool
	NoColor     bool
	Limit       int
	HasError    bool
}

func Init() (*Context, error) {
	// Validate GitHub token — prefer SGH_TOKEN, fall back to GITHUB_TOKEN
	token := os.Getenv("SGH_TOKEN")
	if token == "" {
		if fallback := os.Getenv("GITHUB_TOKEN"); fallback != "" {
			fmt.Fprintln(os.Stderr, "Warning: GITHUB_TOKEN is deprecated, please use SGH_TOKEN instead")
			token = fallback
		}
	}
	if token == "" {
		return nil, fmt.Errorf("SGH_TOKEN environment variable is required. Please set your GitHub Personal Access Token")
	}

	// Enhanced token validation
	if err := validateGitHubToken(token); err != nil {
		return nil, fmt.Errorf("invalid SGH_TOKEN: %w", err)
	}

	config, err := config.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize config: %w", err)
	}

	// Get timeout from environment or use default
	timeoutStr := os.Getenv("SGH_TIMEOUT")
	httpTimeout := 30 * time.Second
	if timeoutStr != "" {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			httpTimeout = timeout
		} else {
			logger.Flog.Warn().Str("timeout", timeoutStr).Msg("Invalid SGH_TIMEOUT, using default")
		}
	}

	// Create HTTP client with timeout and rate limiting
	httpClient := client.NewHttpClient(httpTimeout, token)
	if httpClient == nil {
		return nil, fmt.Errorf("failed to create HTTP client")
	}

	// Configure client based on environment variables
	if os.Getenv("SGH_VERBOSE") == "true" {
		httpClient.Verbose = true
	}
	if os.Getenv("SGH_LOG_RESPONSE") == "true" {
		httpClient.LogResponse = true
	}

	// Create OAuth2 client with custom transport
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{
		Timeout:   httpTimeout,
		Transport: httpClient.Client.Transport, // Use the same transport with rate limiting
	})
	oauthClient := oauth2.NewClient(ctx, src)

	// Create GraphQL client
	gqlClient := githubv4.NewClient(oauthClient)
	graphqlClient := &client.GraphqlClient{
		Client:         gqlClient,
		RateLimiter:    httpClient.RateLimiter,
		RetryConfig:    httpClient.RetryConfig,
		CircuitBreaker: httpClient.CircuitBreaker,
	}

	return &Context{
		Config:        config,
		HttpClient:    httpClient,
		GraphqlClient: graphqlClient,
	}, nil
}

// ValidateGitHubToken performs comprehensive token validation.
func ValidateGitHubToken(token string) error { return validateGitHubToken(token) }

// validateGitHubToken performs comprehensive token validation
func validateGitHubToken(token string) error {
	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}

	// Check minimum length (GitHub tokens are typically 40+ characters)
	if len(token) < 20 {
		return fmt.Errorf("token appears to be invalid (too short, expected at least 20 characters)")
	}

	// Check for common invalid patterns
	if strings.Contains(token, " ") {
		return fmt.Errorf("token contains spaces, which is invalid")
	}

	// Check for common test tokens
	if strings.HasPrefix(token, "ghp_test_") || strings.HasPrefix(token, "test_") {
		return fmt.Errorf("token appears to be a test token")
	}

	// Validate GitHub token format (starts with ghp_, gho_, ghu_, ghs_, ghr_, or github_pat_)
	validPrefixes := []string{"ghp_", "gho_", "ghu_", "ghs_", "ghr_", "github_pat_"}
	hasValidPrefix := false
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(token, prefix) {
			hasValidPrefix = true
			break
		}
	}

	if !hasValidPrefix {
		return fmt.Errorf("token format appears invalid (should start with ghp_, gho_, ghu_, ghs_, ghr_, or github_pat_)")
	}

	return nil
}

// SwitchToken rebuilds the HTTP and GraphQL clients using the given token.
// Called when a per-org token is configured, overriding the global SGH_TOKEN.
func (c *Context) SwitchToken(token string) {
	httpClient := client.NewHttpClient(c.HttpClient.Client.Timeout, token)
	if httpClient == nil {
		return
	}
	httpClient.Verbose = c.HttpClient.Verbose
	httpClient.LogResponse = c.HttpClient.LogResponse

	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	octx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{
		Timeout:   c.HttpClient.Client.Timeout,
		Transport: httpClient.Client.Transport,
	})
	gqlClient := githubv4.NewClient(oauth2.NewClient(octx, src))

	c.HttpClient = httpClient
	c.GraphqlClient = &client.GraphqlClient{
		Client:         gqlClient,
		RateLimiter:    httpClient.RateLimiter,
		RetryConfig:    httpClient.RetryConfig,
		CircuitBreaker: httpClient.CircuitBreaker,
	}
}

func (c *Context) SetVerbose(verbose bool) {
	c.Verbose = verbose
	c.HttpClient.Verbose = verbose
	c.GraphqlClient.Verbose = verbose
}

func (c *Context) SetLogResponse(logResponse bool) {
	c.HttpClient.LogResponse = logResponse
}

func (c *Context) SetWorkerCount(noOfWorkers int) {
	c.Config.NoOfWorkers = noOfWorkers
}
