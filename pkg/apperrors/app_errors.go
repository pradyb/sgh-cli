// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package apperrors

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// GitHubError represents errors from GitHub API
type GitHubError struct {
	StatusCode int
	Message    string
	URL        string
	RetryAfter time.Duration
}

func (e *GitHubError) Error() string {
	return fmt.Sprintf("GitHub API error (%d): %s", e.StatusCode, e.Message)
}

func (e *GitHubError) IsRateLimit() bool {
	return e.StatusCode == http.StatusForbidden &&
		(strings.Contains(e.Message, "rate limit") || strings.Contains(e.Message, "abuse detection"))
}

func (e *GitHubError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

func (e *GitHubError) IsUnauthorized() bool {
	return e.StatusCode == http.StatusUnauthorized
}

func (e *GitHubError) IsServerError() bool {
	return e.StatusCode >= 500 && e.StatusCode < 600
}

func (e *GitHubError) IsClientError() bool {
	return e.StatusCode >= 400 && e.StatusCode < 500
}

func (e *GitHubError) ShouldRetry() bool {
	return e.IsRateLimit() || e.IsServerError() || e.StatusCode == http.StatusRequestTimeout
}

// ConfigError represents configuration-related errors
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("configuration error for %s: %s", e.Field, e.Message)
}

// ValidationError represents input validation errors
type ValidationError struct {
	Field   string
	Value   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s' with value '%s': %s",
		e.Field, e.Value, e.Message)
}

// NetworkError represents network-related errors
type NetworkError struct {
	Operation string
	URL       string
	Message   string
	Retryable bool
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error during %s to %s: %s", e.Operation, e.URL, e.Message)
}

// TimeoutError represents timeout errors
type TimeoutError struct {
	Operation string
	Duration  time.Duration
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("timeout after %v during %s", e.Duration, e.Operation)
}

// RateLimitError represents rate limiting errors
type RateLimitError struct {
	Resource   string
	Limit      int
	Remaining  int
	ResetTime  time.Time
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded for %s (limit: %d, remaining: %d, reset: %s)",
		e.Resource, e.Limit, e.Remaining, e.ResetTime.Format(time.RFC3339))
}

// CircuitBreakerError represents circuit breaker errors
type CircuitBreakerError struct {
	State   string
	Message string
}

func (e *CircuitBreakerError) Error() string {
	return fmt.Sprintf("circuit breaker is %s: %s", e.State, e.Message)
}

// BatchOperationError represents errors that occur during batch operations
type BatchOperationError struct {
	TotalOperations  int
	FailedOperations int
	Errors           []error
}

func (e *BatchOperationError) Error() string {
	return fmt.Sprintf("batch operation failed: %d/%d operations failed",
		e.FailedOperations, e.TotalOperations)
}

func (e *BatchOperationError) AddError(err error) {
	e.Errors = append(e.Errors, err)
	e.FailedOperations++
}

// OperationError represents errors that occur during operations
type OperationError struct {
	Err       error
	Operation string
	Context   map[string]interface{}
	Timestamp time.Time
}

func (e *OperationError) Error() string {
	return fmt.Sprintf("operation '%s' failed at %s: %v",
		e.Operation, e.Timestamp.Format(time.RFC3339), e.Err)
}

func (e *OperationError) Unwrap() error {
	return e.Err
}

// WrapError wraps an error with additional context
func WrapError(err error, operation string, context map[string]interface{}) error {
	if err == nil {
		return nil
	}
	return &OperationError{
		Err:       err,
		Operation: operation,
		Context:   context,
		Timestamp: time.Now(),
	}
}

// IsRetryable checks if an error should trigger a retry
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for GitHub-specific errors
	if githubErr, ok := err.(*GitHubError); ok {
		return githubErr.ShouldRetry()
	}

	// Check for network errors
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout() || netErr.Temporary()
	}

	// Check for context errors
	if err == context.DeadlineExceeded || err == context.Canceled {
		return false
	}

	return false
}

// GetErrorCategory categorizes errors for metrics and logging
func GetErrorCategory(err error) string {
	if err == nil {
		return "none"
	}

	switch {
	case IsRetryable(err):
		return "retryable"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	default:
		if githubErr, ok := err.(*GitHubError); ok {
			if githubErr.IsRateLimit() {
				return "rate_limit"
			}
			if githubErr.IsUnauthorized() {
				return "unauthorized"
			}
			if githubErr.IsNotFound() {
				return "not_found"
			}
		}
		return "unknown"
	}
}

// IsRetryableError checks if an error should be retried
func IsRetryableError(err error) bool {
	switch e := err.(type) {
	case *GitHubError:
		return e.ShouldRetry()
	case *NetworkError:
		return e.Retryable
	case *TimeoutError:
		return true
	case *RateLimitError:
		return true
	default:
		return false
	}
}

// IsPermanentError checks if an error is permanent and should not be retried
func IsPermanentError(err error) bool {
	switch e := err.(type) {
	case *GitHubError:
		return e.IsNotFound() || e.IsUnauthorized() || (e.StatusCode >= 400 && e.StatusCode < 500 && !e.IsRateLimit())
	case *ValidationError:
		return true
	case *ConfigError:
		return true
	default:
		return false
	}
}
