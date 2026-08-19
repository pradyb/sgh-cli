// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/pradyb/sgh-cli/internal/circuitbreaker"
	"github.com/pradyb/sgh-cli/internal/ratelimit"
	"github.com/pradyb/sgh-cli/internal/retry"
	"github.com/pradyb/sgh-cli/pkg/apperrors"
	"github.com/pradyb/sgh-cli/pkg/logger"
	"github.com/shurcooL/githubv4"
)

type HttpClient struct {
	Client         http.Client
	Token          string
	Verbose        bool
	LogResponse    bool
	RateLimiter    *ratelimit.RateLimiter
	RetryConfig    *retry.RetryConfig
	CircuitBreaker *circuitbreaker.CircuitBreaker
}

type Interceptor struct {
	OriginalTransport http.RoundTripper
	RateLimiter       *ratelimit.RateLimiter
	requestCount      atomic.Int64
}

func NewHttpClient(timeout time.Duration, token string) *HttpClient {
	rateLimiter := ratelimit.NewRateLimiter()
	circuitBreaker := circuitbreaker.New(circuitbreaker.DefaultConfig())

	// Create a custom transport with connection pooling and timeouts
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableCompression:  false,
		ForceAttemptHTTP2:   true,
	}

	interceptor := &Interceptor{
		OriginalTransport: transport,
		RateLimiter:       rateLimiter,
	}

	return &HttpClient{
		Client: http.Client{
			Timeout:   timeout,
			Transport: interceptor,
		},
		Token:          token,
		RateLimiter:    rateLimiter,
		RetryConfig:    retry.DefaultRetryConfig(),
		CircuitBreaker: circuitBreaker,
	}
}

func (i *Interceptor) RoundTrip(r *http.Request) (*http.Response, error) {
	i.requestCount.Add(1)
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
	rateResetStr := resp.Header.Get("X-RateLimit-Reset")
	rateResource := resp.Header.Get("X-RateLimit-Resource")

	var rateReset string
	if rateResetStr != "" {
		if rateResetInt, err := strconv.ParseInt(rateResetStr, 10, 64); err != nil {
			logger.Flog.Warn().
				Str("rateResetStr", rateResetStr).
				Err(err).
				Msg("Failed to parse rate limit reset time")
			rateReset = "invalid"
		} else {
			rateReset = time.Unix(rateResetInt, 0).String()
		}
	} else {
		rateReset = "not provided"
	}

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
	req.Header.Add("Authorization", fmt.Sprintf("token %s", c.Token))
	req.Header.Add("Content-Type", "application/json")
}

func (c *HttpClient) logRequestIfVerbose(req *http.Request) {
	if c.Verbose {
		// Clone the request to redact the Authorization header before logging
		clone := req.Clone(req.Context())
		if clone.Header.Get("Authorization") != "" {
			clone.Header.Set("Authorization", "token [REDACTED]")
		}
		reqDump, err := httputil.DumpRequestOut(clone, true)
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
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		msg := extractGitHubMessage(body, res.StatusCode)
		logger.Flog.Error().
			Str("url", req.URL.String()).
			Str("method", req.Method).
			Int("statusCode", res.StatusCode).
			Str("error", msg).
			Msg("API error response")
		return nil, &apperrors.GitHubError{
			StatusCode: res.StatusCode,
			Message:    msg,
			URL:        req.URL.String(),
		}
	}

	return res, nil
}

func (c *HttpClient) handleError(response *http.Response, err error) (*http.Response, error) {
	if githubErr, ok := err.(*apperrors.GitHubError); ok {
		// Enhance GitHub error with additional context
		if githubErr.StatusCode == 401 {
			return response, fmt.Errorf("authentication failed: %w (check your SGH_TOKEN)", githubErr)
		} else if githubErr.StatusCode == 403 {
			return response, fmt.Errorf("permission denied: %w (check your token permissions)", githubErr)
		} else if githubErr.StatusCode == 404 {
			return response, fmt.Errorf("resource not found: %w", githubErr)
		} else if githubErr.StatusCode >= 500 {
			return response, fmt.Errorf("GitHub API server error: %w", githubErr)
		}
		return response, githubErr
	}

	// Handle network errors
	if netErr, ok := err.(*net.OpError); ok {
		return response, fmt.Errorf("network error: %w (check your internet connection)", netErr)
	}

	// Handle timeout errors
	if timeoutErr, ok := err.(interface{ Timeout() bool }); ok && timeoutErr.Timeout() {
		return response, fmt.Errorf("request timeout: %w (try increasing SGH_TIMEOUT environment variable)", err)
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

// APICallCount returns the total number of HTTP requests made.
func (c *HttpClient) APICallCount() int64 {
	if t, ok := c.Client.Transport.(*Interceptor); ok {
		return t.requestCount.Load()
	}
	return 0
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
				return err
			}

			logger.Flog.Debug().
				Int("timeTakenInMs", int(elapsed)).
				Msg("GraphQL query completed successfully")
			return nil
		})
		return queryErr
	})
	if err != nil {
		logger.Flog.Error().Err(err).Msg("GraphQL query execution failed")
		return err
	}
	return queryErr
}

// extractGitHubMessage parses a GitHub API JSON error body and returns
// a clean, human-readable message. Falls back to the raw body on parse failure.
func extractGitHubMessage(body []byte, statusCode int) string {
	var payload struct {
		Message string `json:"message"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && payload.Message != "" {
		msg := payload.Message
		if len(payload.Errors) > 0 && payload.Errors[0].Message != "" {
			msg += " — " + payload.Errors[0].Message
		}
		return msg
	}
	text := string(body)
	if len(text) > 200 {
		text = text[:200] + "..."
	}
	return text
}
