// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pradyb/sgh-cli/internal/circuitbreaker"
	"github.com/pradyb/sgh-cli/internal/ratelimit"
	"github.com/pradyb/sgh-cli/internal/retry"
)

func TestNewHttpClientConstructsDependencies(t *testing.T) {
	c := NewHttpClient(10*time.Second, testToken)

	if c.RateLimiter == nil {
		t.Error("RateLimiter is nil")
	}
	if c.RetryConfig == nil {
		t.Error("RetryConfig is nil")
	}
	if c.CircuitBreaker == nil {
		t.Error("CircuitBreaker is nil")
	}
	if c.Token != testToken {
		t.Errorf("Token = %q, want %q", c.Token, testToken)
	}

	if _, ok := c.Client.Transport.(*Interceptor); !ok {
		t.Fatalf("Transport = %T, want *Interceptor", c.Client.Transport)
	}
}

func TestGetRateLimitStatus_NilRateLimiter(t *testing.T) {
	c := &HttpClient{RateLimiter: nil}
	if got := c.GetRateLimitStatus(); got != nil {
		t.Errorf("GetRateLimitStatus() = %v, want nil", got)
	}
}

func TestGetRateLimitStatus_WithData(t *testing.T) {
	rl := ratelimit.NewRateLimiter()
	c := &HttpClient{RateLimiter: rl}

	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("X-RateLimit-Resource", "core")
	resp.Header.Set("X-RateLimit-Limit", "5000")
	resp.Header.Set("X-RateLimit-Remaining", "4321")
	rl.UpdateFromResponse(resp)

	status := c.GetRateLimitStatus()
	info, ok := status["core"]
	if !ok {
		t.Fatal("expected 'core' entry in rate limit status")
	}
	if info.Remaining != 4321 {
		t.Errorf("Remaining = %d, want 4321", info.Remaining)
	}
}

func TestSetRetryConfig(t *testing.T) {
	c := NewHttpClient(5*time.Second, testToken)

	custom := &retry.RetryConfig{
		MaxAttempts:  7,
		InitialDelay: 42 * time.Millisecond,
	}
	c.SetRetryConfig(custom)

	if c.RetryConfig != custom {
		t.Errorf("RetryConfig = %v, want the exact custom pointer %v", c.RetryConfig, custom)
	}
	if c.RetryConfig.MaxAttempts != 7 {
		t.Errorf("MaxAttempts = %d, want 7", c.RetryConfig.MaxAttempts)
	}
}

func TestAPICallCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestHttpClient(t)
	if got := c.APICallCount(); got != 0 {
		t.Fatalf("APICallCount() before any request = %d, want 0", got)
	}

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/user", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := c.Send(req)
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	resp.Body.Close()

	if got := c.APICallCount(); got != 1 {
		t.Errorf("APICallCount() after one request = %d, want 1", got)
	}
}

func TestAPICallCount_NonInterceptorTransport(t *testing.T) {
	c := &HttpClient{Client: http.Client{Transport: http.DefaultTransport}}
	if got := c.APICallCount(); got != 0 {
		t.Errorf("APICallCount() = %d, want 0 for a non-Interceptor transport", got)
	}
}

// Sanity check that CircuitBreaker.DefaultConfig wiring in NewHttpClient
// produces a breaker that starts closed.
func TestNewHttpClientCircuitBreakerStartsClosed(t *testing.T) {
	c := NewHttpClient(5*time.Second, testToken)
	if c.CircuitBreaker.GetState() != circuitbreaker.StateClosed {
		t.Errorf("CircuitBreaker.GetState() = %v, want StateClosed", c.CircuitBreaker.GetState())
	}
}
