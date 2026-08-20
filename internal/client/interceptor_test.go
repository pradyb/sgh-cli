// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pradyb/sgh-cli/internal/ratelimit"
)

func TestInterceptorRoundTrip_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "5000")
		w.Header().Set("X-RateLimit-Remaining", "4999")
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Hour).Unix()))
		w.Header().Set("X-RateLimit-Resource", "core")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rl := ratelimit.NewRateLimiter()
	interceptor := &Interceptor{OriginalTransport: http.DefaultTransport, RateLimiter: rl}

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := interceptor.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip returned error: %v", err)
	}
	resp.Body.Close()

	if got := interceptor.requestCount.Load(); got != 1 {
		t.Errorf("requestCount = %d, want 1", got)
	}

	status := rl.GetStatus()
	info, ok := status["core"]
	if !ok {
		t.Fatal("expected rate limiter to have been updated with 'core' resource info")
	}
	if info.Remaining != 4999 {
		t.Errorf("Remaining = %d, want 4999", info.Remaining)
	}
}

// A nil RateLimiter must not cause RoundTrip to panic.
func TestInterceptorRoundTrip_NilRateLimiter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	interceptor := &Interceptor{OriginalTransport: http.DefaultTransport}

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := interceptor.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip returned error: %v", err)
	}
	resp.Body.Close()

	if got := interceptor.requestCount.Load(); got != 1 {
		t.Errorf("requestCount = %d, want 1", got)
	}
}

// Closing the server before issuing the request makes the underlying
// transport's RoundTrip fail (connection refused), exercising the
// network-error path in Interceptor.RoundTrip.
func TestInterceptorRoundTrip_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	interceptor := &Interceptor{OriginalTransport: http.DefaultTransport, RateLimiter: ratelimit.NewRateLimiter()}

	req, err := http.NewRequest(http.MethodGet, addr, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	_, err = interceptor.RoundTrip(req)
	if err == nil {
		t.Fatal("expected network error, got nil")
	}

	if got := interceptor.requestCount.Load(); got != 1 {
		t.Errorf("requestCount = %d, want 1", got)
	}
}

func TestLogRateDetails(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
	}{
		{
			name: "full rate limit headers",
			headers: map[string]string{
				"X-RateLimit-Limit":     "5000",
				"X-RateLimit-Remaining": "4999",
				"X-RateLimit-Used":      "1",
				"X-RateLimit-Reset":     fmt.Sprintf("%d", time.Now().Add(time.Hour).Unix()),
				"X-RateLimit-Resource":  "core",
			},
		},
		{
			name:    "no rate limit headers",
			headers: map[string]string{},
		},
		{
			name: "unparseable reset value",
			headers: map[string]string{
				"X-RateLimit-Reset": "not-a-number",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{Header: http.Header{}}
			for k, v := range tt.headers {
				resp.Header.Set(k, v)
			}
			// Must not panic; this function is log-only.
			logRateDetails(resp)
		})
	}
}
