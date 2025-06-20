package context

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"

	"github.com/prady-lab/sgh-cli/internal/client"
	"github.com/prady-lab/sgh-cli/internal/config"
)

type Context struct {
	Config        *config.Config
	HttpClient    *client.HttpClient
	GraphqlClient *client.GraphqlClient

	Verbose     bool
	LogResponse bool
}

func Init() (*Context, error) {
	// Validate GitHub token
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable is required. Please set your GitHub Personal Access Token")
	}

	// Basic token format validation
	if len(token) < 20 {
		return nil, fmt.Errorf("GITHUB_TOKEN appears to be invalid (too short). Please check your GitHub Personal Access Token")
	}

	config, err := config.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize config: %w", err)
	}

	// Create HTTP client with timeout and rate limiting
	httpTimeout := 30 * time.Second
	httpClient := client.NewHttpClient(httpTimeout)
	if httpClient == nil {
		return nil, fmt.Errorf("failed to create HTTP client")
	}

	// Create OAuth2 client
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
