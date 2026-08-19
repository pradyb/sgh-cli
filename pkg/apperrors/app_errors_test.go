// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package apperrors

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestGitHubErrorClassification(t *testing.T) {
	tests := []struct {
		name             string
		err              *GitHubError
		wantRateLimit    bool
		wantNotFound     bool
		wantUnauthorized bool
		wantServerError  bool
		wantClientError  bool
		wantShouldRetry  bool
	}{
		{
			name:            "rate limit",
			err:             &GitHubError{StatusCode: http.StatusForbidden, Message: "API rate limit exceeded"},
			wantRateLimit:   true,
			wantClientError: true,
			wantShouldRetry: true,
		},
		{
			name:            "abuse detection is also a rate limit",
			err:             &GitHubError{StatusCode: http.StatusForbidden, Message: "You have triggered an abuse detection mechanism"},
			wantRateLimit:   true,
			wantClientError: true,
			wantShouldRetry: true,
		},
		{
			// A 403 without rate-limit wording is a genuine permission denial
			// and must not be retried, or the tool hammers a forbidden endpoint.
			name:            "plain 403 is not a rate limit",
			err:             &GitHubError{StatusCode: http.StatusForbidden, Message: "Resource not accessible by personal access token"},
			wantClientError: true,
		},
		{
			name:            "not found",
			err:             &GitHubError{StatusCode: http.StatusNotFound, Message: "Not Found"},
			wantNotFound:    true,
			wantClientError: true,
		},
		{
			name:             "unauthorized",
			err:              &GitHubError{StatusCode: http.StatusUnauthorized, Message: "Bad credentials"},
			wantUnauthorized: true,
			wantClientError:  true,
		},
		{
			name:            "server error",
			err:             &GitHubError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error"},
			wantServerError: true,
			wantShouldRetry: true,
		},
		{
			name:            "bad gateway",
			err:             &GitHubError{StatusCode: http.StatusBadGateway, Message: "Bad Gateway"},
			wantServerError: true,
			wantShouldRetry: true,
		},
		{
			name:            "request timeout is retryable",
			err:             &GitHubError{StatusCode: http.StatusRequestTimeout, Message: "Request Timeout"},
			wantClientError: true,
			wantShouldRetry: true,
		},
		{
			name: "success code is neither",
			err:  &GitHubError{StatusCode: http.StatusOK, Message: "OK"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.IsRateLimit(); got != tt.wantRateLimit {
				t.Errorf("IsRateLimit() = %v, want %v", got, tt.wantRateLimit)
			}
			if got := tt.err.IsNotFound(); got != tt.wantNotFound {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.wantNotFound)
			}
			if got := tt.err.IsUnauthorized(); got != tt.wantUnauthorized {
				t.Errorf("IsUnauthorized() = %v, want %v", got, tt.wantUnauthorized)
			}
			if got := tt.err.IsServerError(); got != tt.wantServerError {
				t.Errorf("IsServerError() = %v, want %v", got, tt.wantServerError)
			}
			if got := tt.err.IsClientError(); got != tt.wantClientError {
				t.Errorf("IsClientError() = %v, want %v", got, tt.wantClientError)
			}
			if got := tt.err.ShouldRetry(); got != tt.wantShouldRetry {
				t.Errorf("ShouldRetry() = %v, want %v", got, tt.wantShouldRetry)
			}
		})
	}
}

func TestErrorMessages(t *testing.T) {
	reset := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		err      error
		contains []string
	}{
		{
			"github",
			&GitHubError{StatusCode: 404, Message: "Not Found"},
			[]string{"404", "Not Found"},
		},
		{
			"config",
			&ConfigError{Field: "token", Message: "is required"},
			[]string{"token", "is required"},
		},
		{
			"validation",
			&ValidationError{Field: "org", Value: "bad org", Message: "invalid"},
			[]string{"org", "bad org", "invalid"},
		},
		{
			"network",
			&NetworkError{Operation: "GET", URL: "https://api.github.com", Message: "connection refused"},
			[]string{"GET", "api.github.com", "connection refused"},
		},
		{
			"timeout",
			&TimeoutError{Operation: "list repos", Duration: 30 * time.Second},
			[]string{"30s", "list repos"},
		},
		{
			"rate limit",
			&RateLimitError{Resource: "core", Limit: 5000, Remaining: 0, ResetTime: reset},
			[]string{"core", "5000", "2026-08-19"},
		},
		{
			"circuit breaker",
			&CircuitBreakerError{State: "open", Message: "too many failures"},
			[]string{"open", "too many failures"},
		},
		{
			"batch",
			&BatchOperationError{TotalOperations: 10, FailedOperations: 3},
			[]string{"3", "10"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.Error()
			for _, want := range tt.contains {
				if !strings.Contains(msg, want) {
					t.Errorf("Error() = %q, missing %q", msg, want)
				}
			}
		})
	}
}

func TestBatchOperationErrorAddError(t *testing.T) {
	batch := &BatchOperationError{TotalOperations: 3}

	if batch.FailedOperations != 0 {
		t.Fatalf("FailedOperations = %d, want 0 before any AddError", batch.FailedOperations)
	}

	batch.AddError(errors.New("first"))
	batch.AddError(errors.New("second"))

	if batch.FailedOperations != 2 {
		t.Errorf("FailedOperations = %d, want 2", batch.FailedOperations)
	}
	if len(batch.Errors) != 2 {
		t.Errorf("len(Errors) = %d, want 2", len(batch.Errors))
	}
	if !strings.Contains(batch.Error(), "2/3") {
		t.Errorf("Error() = %q, want it to report 2/3", batch.Error())
	}
}

func TestOperationErrorUnwrap(t *testing.T) {
	sentinel := errors.New("underlying failure")
	wrapped := &OperationError{
		Err:       sentinel,
		Operation: "create branch",
		Timestamp: time.Now(),
	}

	if !errors.Is(wrapped, sentinel) {
		t.Error("errors.Is should find the wrapped error")
	}
	if got := wrapped.Unwrap(); got != sentinel {
		t.Errorf("Unwrap() = %v, want %v", got, sentinel)
	}
}

func TestWrapError(t *testing.T) {
	t.Run("nil stays nil", func(t *testing.T) {
		if got := WrapError(nil, "op", nil); got != nil {
			t.Errorf("WrapError(nil) = %v, want nil", got)
		}
	})

	t.Run("preserves the chain and context", func(t *testing.T) {
		sentinel := errors.New("boom")
		ctxData := map[string]any{"org": "my-org", "repo": "api"}

		err := WrapError(sentinel, "delete branch", ctxData)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !errors.Is(err, sentinel) {
			t.Error("wrapped error should still match the original")
		}

		var opErr *OperationError
		if !errors.As(err, &opErr) {
			t.Fatalf("expected *OperationError, got %T", err)
		}
		if opErr.Operation != "delete branch" {
			t.Errorf("Operation = %q, want %q", opErr.Operation, "delete branch")
		}
		if opErr.Context["org"] != "my-org" {
			t.Errorf("Context[org] = %v, want my-org", opErr.Context["org"])
		}
		if opErr.Timestamp.IsZero() {
			t.Error("Timestamp should be set")
		}
	})
}

// fakeNetError lets us exercise the net.Error branch without real sockets.
type fakeNetError struct{ timeout, temporary bool }

func (e fakeNetError) Error() string   { return "fake net error" }
func (e fakeNetError) Timeout() bool   { return e.timeout }
func (e fakeNetError) Temporary() bool { return e.temporary }

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"github server error", &GitHubError{StatusCode: 500}, true},
		{"github rate limit", &GitHubError{StatusCode: 403, Message: "rate limit"}, true},
		{"github not found", &GitHubError{StatusCode: 404}, false},
		{"net timeout", fakeNetError{timeout: true}, true},
		{"net temporary", fakeNetError{temporary: true}, true},
		{"net neither", fakeNetError{}, false},
		{"deadline exceeded", context.DeadlineExceeded, false},
		{"canceled", context.Canceled, false},
		{"plain error", errors.New("nope"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestGetErrorCategory(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, "none"},
		{"retryable server error", &GitHubError{StatusCode: 503}, "retryable"},
		{"rate limit is retryable first", &GitHubError{StatusCode: 403, Message: "rate limit"}, "retryable"},
		{"deadline", context.DeadlineExceeded, "timeout"},
		{"cancelled", context.Canceled, "cancelled"},
		{"unauthorized", &GitHubError{StatusCode: 401}, "unauthorized"},
		{"not found", &GitHubError{StatusCode: 404}, "not_found"},
		{"other github", &GitHubError{StatusCode: 422, Message: "Unprocessable"}, "unknown"},
		{"plain error", errors.New("nope"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetErrorCategory(tt.err); got != tt.want {
				t.Errorf("GetErrorCategory(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"github retryable", &GitHubError{StatusCode: 500}, true},
		{"github permanent", &GitHubError{StatusCode: 404}, false},
		{"network flagged retryable", &NetworkError{Retryable: true}, true},
		{"network flagged permanent", &NetworkError{Retryable: false}, false},
		{"timeout always", &TimeoutError{Operation: "x"}, true},
		{"rate limit always", &RateLimitError{Resource: "core"}, true},
		{"validation never", &ValidationError{Field: "org"}, false},
		{"plain error", errors.New("nope"), false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryableError(tt.err); got != tt.want {
				t.Errorf("IsRetryableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsPermanentError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"not found", &GitHubError{StatusCode: 404}, true},
		{"unauthorized", &GitHubError{StatusCode: 401}, true},
		{"unprocessable", &GitHubError{StatusCode: 422}, true},
		// A rate limit is a 4xx but is explicitly not permanent — it clears.
		{"rate limit is not permanent", &GitHubError{StatusCode: 403, Message: "rate limit"}, false},
		{"server error is not permanent", &GitHubError{StatusCode: 500}, false},
		{"validation", &ValidationError{Field: "org"}, true},
		{"config", &ConfigError{Field: "token"}, true},
		{"network", &NetworkError{}, false},
		{"plain error", errors.New("nope"), false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPermanentError(tt.err); got != tt.want {
				t.Errorf("IsPermanentError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// An error is never both retryable and permanent; the retry loop consults both
// and would behave incoherently if they overlapped.
func TestRetryableAndPermanentAreMutuallyExclusive(t *testing.T) {
	candidates := []error{
		&GitHubError{StatusCode: 404},
		&GitHubError{StatusCode: 401},
		&GitHubError{StatusCode: 403, Message: "rate limit"},
		&GitHubError{StatusCode: 422},
		&GitHubError{StatusCode: 500},
		&GitHubError{StatusCode: 503},
		&NetworkError{Retryable: true},
		&NetworkError{Retryable: false},
		&TimeoutError{},
		&RateLimitError{},
		&ValidationError{},
		&ConfigError{},
	}

	for _, err := range candidates {
		if IsRetryableError(err) && IsPermanentError(err) {
			t.Errorf("%T (%v) is classified as both retryable and permanent", err, err)
		}
	}
}

// Compile-time proof that every error type in this package satisfies error.
var _ = []error{
	&GitHubError{}, &ConfigError{}, &ValidationError{}, &NetworkError{},
	&TimeoutError{}, &RateLimitError{}, &CircuitBreakerError{},
	&BatchOperationError{}, &OperationError{},
}

var _ net.Error = fakeNetError{}
