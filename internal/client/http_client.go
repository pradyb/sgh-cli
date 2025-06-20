package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"time"

	"github.com/prady-lab/sgh-cli/internal/circuitbreaker"
	"github.com/prady-lab/sgh-cli/internal/ratelimit"
	"github.com/prady-lab/sgh-cli/internal/retry"
	"github.com/prady-lab/sgh-cli/pkg/apperrors"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/shurcooL/githubv4"
)

type HttpClient struct {
	Client         http.Client
	Verbose        bool
	LogResponse    bool
	RateLimiter    *ratelimit.RateLimiter
	RetryConfig    *retry.RetryConfig
	CircuitBreaker *circuitbreaker.CircuitBreaker
}

type Interceptor struct {
	OriginalTransport http.RoundTripper
	RateLimiter       *ratelimit.RateLimiter
}

func NewHttpClient(timeout time.Duration) *HttpClient {
	rateLimiter := ratelimit.NewRateLimiter()
	circuitBreaker := circuitbreaker.New(circuitbreaker.DefaultConfig())
	transport := &Interceptor{
		OriginalTransport: http.DefaultTransport,
		RateLimiter:       rateLimiter,
	}

	return &HttpClient{
		Client: http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		RateLimiter:    rateLimiter,
		RetryConfig:    retry.DefaultRetryConfig(),
		CircuitBreaker: circuitBreaker,
	}
}

func (i *Interceptor) RoundTrip(r *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := i.OriginalTransport.RoundTrip(r)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		logger.Flog.Error().Err(err).
			Str("url", r.URL.String()).
			Str("method", r.Method).
			Int("timeTakenInMs", int(elapsed)).
			Msg("HTTP request failed")
		return resp, err
	}

	// Update rate limit information
	if i.RateLimiter != nil {
		i.RateLimiter.UpdateFromResponse(resp)
	}

	logger.Flog.Info().
		Str("url", r.URL.String()).
		Str("method", r.Method).
		Int("statusCode", resp.StatusCode).
		Int("timeTakenInMs", int(elapsed)).
		Msg("API request completed")

	logRateDetails(resp)
	return resp, err
}

func logRateDetails(resp *http.Response) {
	rateLimit := resp.Header.Get("X-RateLimit-Limit")
	rateRemaining := resp.Header.Get("X-RateLimit-Remaining")
	rateUsed := resp.Header.Get("X-RateLimit-Used")
	rateResetInt, _ := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64)
	rateReset := time.Unix(rateResetInt, 0).String()
	rateResource := resp.Header.Get("X-RateLimit-Resource")

	logger.Flog.Info().
		Str("rateLimit", rateLimit).
		Str("rateRemaining", rateRemaining).
		Str("rateUsed", rateUsed).
		Str("rateResource", rateResource).
		Str("rateReset", rateReset).
		Msgf("Rate limit details")
}

func (c *HttpClient) Send(req *http.Request) (*http.Response, error) {
	return c.SendWithContext(context.Background(), req)
}

func (c *HttpClient) SendWithContext(ctx context.Context, req *http.Request) (*http.Response, error) {
	c.prepareRequest(req)
	c.logRequestIfVerbose(req)

	resource := c.determineResource(req)
	if err := c.waitForRateLimit(ctx, resource); err != nil {
		return nil, err
	}

	response, err := c.executeRequestWithRetry(ctx, req)
	if err != nil {
		return c.handleError(response, err)
	}

	c.logResponseIfEnabled(response)
	return response, nil
}

func (c *HttpClient) prepareRequest(req *http.Request) {
	req.Header.Add("Authorization", fmt.Sprintf("token %s", os.Getenv("GITHUB_TOKEN")))
	req.Header.Add("Content-Type", "application/json")
}

func (c *HttpClient) logRequestIfVerbose(req *http.Request) {
	if c.Verbose {
		reqDump, err := httputil.DumpRequestOut(req, true)
		if err != nil {
			logger.Glog.Error().Err(err).Msg("Error in print the request")
		}
		fmt.Printf("REQUEST:\n%s", string(reqDump))
	}
}

func (c *HttpClient) determineResource(req *http.Request) string {
	resource := "core" // Default to core API
	if req.URL.Path != "" {
		switch req.URL.Path {
		case "/graphql":
			resource = "graphql"
		case "/search":
			resource = "search"
		}
	}
	return resource
}

func (c *HttpClient) waitForRateLimit(ctx context.Context, resource string) error {
	if c.RateLimiter != nil {
		if err := c.RateLimiter.WaitIfNeeded(ctx, resource); err != nil {
			return fmt.Errorf("rate limit wait failed: %w", err)
		}
	}
	return nil
}

func (c *HttpClient) executeRequestWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	var response *http.Response
	err := c.CircuitBreaker.Execute(func() error {
		var retryErr error
		response, retryErr = retry.DoHTTP(ctx, c.RetryConfig, func() (*http.Response, error) {
			return c.doSingleRequest(ctx, req)
		})
		return retryErr
	})
	return response, err
}

func (c *HttpClient) doSingleRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	reqClone := req.Clone(ctx)
	res, err := c.Client.Do(reqClone)
	if err != nil {
		return nil, err
	}

	if res.StatusCode >= 400 {
		body, _ := httputil.DumpResponse(res, true)
		return res, &apperrors.GitHubError{
			StatusCode: res.StatusCode,
			Message:    string(body),
			URL:        req.URL.String(),
		}
	}

	return res, nil
}

func (c *HttpClient) handleError(response *http.Response, err error) (*http.Response, error) {
	if _, ok := err.(*apperrors.GitHubError); ok {
		return response, err
	}
	return response, fmt.Errorf("HTTP request failed: %w", err)
}

func (c *HttpClient) logResponseIfEnabled(response *http.Response) {
	if c.LogResponse && response != nil {
		respDump, err := httputil.DumpResponse(response, true)
		if err != nil {
			logger.Glog.Error().Err(err).Msg("Error in print the response")
		} else {
			fmt.Printf("RESPONSE:\n%s", string(respDump))
		}
	}
}

// SetRetryConfig allows customizing retry behavior
func (c *HttpClient) SetRetryConfig(config *retry.RetryConfig) {
	c.RetryConfig = config
}

// GetRateLimitStatus returns current rate limit status
func (c *HttpClient) GetRateLimitStatus() map[string]ratelimit.RateLimitInfo {
	if c.RateLimiter == nil {
		return nil
	}
	return c.RateLimiter.GetStatus()
}

type GraphqlClient struct {
	Client         *githubv4.Client
	Verbose        bool
	RateLimiter    *ratelimit.RateLimiter
	RetryConfig    *retry.RetryConfig
	CircuitBreaker *circuitbreaker.CircuitBreaker
}

func (c *GraphqlClient) Query(query interface{}, variables map[string]interface{}) error {
	return c.QueryWithContext(context.Background(), query, variables)
}

func (c *GraphqlClient) QueryWithContext(ctx context.Context, query interface{}, variables map[string]interface{}) error {
	if c.Verbose {
		logger.Flog.Info().Msgf("Executing GraphQL query with variables %v", variables)
	}

	// Wait for rate limit if needed (GraphQL uses the "graphql" resource)
	if c.RateLimiter != nil {
		if err := c.RateLimiter.WaitIfNeeded(ctx, "graphql"); err != nil {
			return fmt.Errorf("GraphQL rate limit wait failed: %w", err)
		}
	}
	// Execute GraphQL query with circuit breaker and retry logic
	var queryErr error
	err := c.CircuitBreaker.Execute(func() error {
		queryErr = retry.Do(ctx, c.RetryConfig, func() error {
			start := time.Now()
			err := c.Client.Query(ctx, query, variables)
			elapsed := time.Since(start).Milliseconds()

			if err != nil {
				logger.Flog.Error().Err(err).
					Int("timeTakenInMs", int(elapsed)).
					Msg("GraphQL query failed")

				// Return error as-is - the retry package will determine if it's retryable
				return err
			}

			logger.Flog.Info().
				Int("timeTakenInMs", int(elapsed)).
				Msg("GraphQL query completed successfully")
			return nil
		})
		return queryErr
	})
	if err != nil {
		logger.Glog.Error().Err(err).Msg("GraphQL query execution failed")
		return err
	}
	return queryErr
}
