package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/prady-lab/sgh-cli/pkg/apperrors"
	"github.com/prady-lab/sgh-cli/pkg/logger"
)

// RetryConfig contains configuration for retry behavior
type RetryConfig struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	Jitter          bool
	RetryableErrors []int // HTTP status codes that should be retried
}

// DefaultRetryConfig returns a sensible default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
		RetryableErrors: []int{
			http.StatusTooManyRequests,     // 429
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout,      // 504
		},
	}
}

// AggressiveRetryConfig returns a more aggressive retry configuration for critical operations
func AggressiveRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:   5,
		InitialDelay:  500 * time.Millisecond,
		MaxDelay:      60 * time.Second,
		BackoffFactor: 1.5,
		Jitter:        true,
		RetryableErrors: []int{
			http.StatusTooManyRequests,
			http.StatusInternalServerError,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout,
			http.StatusRequestTimeout, // 408
		},
	}
}

// RetryableFunc is a function that can be retried
type RetryableFunc func() error

// HTTPRetryableFunc is a function that returns an HTTP response and can be retried
type HTTPRetryableFunc func() (*http.Response, error)

// Do executes a function with retry logic
func Do(ctx context.Context, config *RetryConfig, fn RetryableFunc) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := fn()
		if err == nil {
			if attempt > 1 {
				logger.Flog.Info().
					Int("attempt", attempt).
					Int("totalAttempts", config.MaxAttempts).
					Msg("Operation succeeded after retry")
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err, config) {
			logger.Flog.Debug().
				Err(err).
				Int("attempt", attempt).
				Msg("Error is not retryable, giving up")
			return err
		}

		// Don't wait after the last attempt
		if attempt == config.MaxAttempts {
			break
		}

		// Calculate delay
		delay := calculateDelay(attempt, config)

		logger.Flog.Warn().
			Err(err).
			Int("attempt", attempt).
			Int("maxAttempts", config.MaxAttempts).
			Dur("delay", delay).
			Msg("Operation failed, retrying")

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	logger.Flog.Error().
		Err(lastErr).
		Int("maxAttempts", config.MaxAttempts).
		Msg("All retry attempts failed")

	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxAttempts, lastErr)
}

// DoHTTP executes an HTTP function with retry logic and returns the response
func DoHTTP(ctx context.Context, config *RetryConfig, fn HTTPRetryableFunc) (*http.Response, error) {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastResp *http.Response
	var lastErr error

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		resp, err := fn()
		if err == nil && resp != nil && isSuccessfulResponse(resp) {
			if attempt > 1 {
				logger.Flog.Info().
					Int("attempt", attempt).
					Int("statusCode", resp.StatusCode).
					Msg("HTTP request succeeded after retry")
			}
			return resp, nil
		}

		lastResp = resp
		lastErr = err

		// Check if error/response is retryable
		if !isRetryableHTTPError(resp, err, config) {
			logger.Flog.Debug().
				Err(err).
				Int("attempt", attempt).
				Int("statusCode", getStatusCode(resp)).
				Msg("HTTP error is not retryable, giving up")
			return resp, err
		}

		// Close response body if present to prevent resource leaks
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}

		// Don't wait after the last attempt
		if attempt == config.MaxAttempts {
			break
		}

		// Calculate delay, considering Retry-After header
		delay := calculateHTTPDelay(attempt, resp, config)

		logger.Flog.Warn().
			Err(err).
			Int("attempt", attempt).
			Int("maxAttempts", config.MaxAttempts).
			Int("statusCode", getStatusCode(resp)).
			Dur("delay", delay).
			Msg("HTTP request failed, retrying")

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	logger.Flog.Error().
		Err(lastErr).
		Int("maxAttempts", config.MaxAttempts).
		Int("statusCode", getStatusCode(lastResp)).
		Msg("All HTTP retry attempts failed")

	return lastResp, fmt.Errorf("HTTP operation failed after %d attempts: %w", config.MaxAttempts, lastErr)
}

// isRetryableError determines if an error should be retried
func isRetryableError(err error, config *RetryConfig) bool {
	if err == nil {
		return false
	}

	// Check for GitHub API errors
	if githubErr, ok := err.(*apperrors.GitHubError); ok {
		return isRetryableStatusCode(githubErr.StatusCode, config)
	}

	// Check for network errors
	if isNetworkError(err) {
		return true
	}

	// Check for URL errors (connection issues)
	if _, ok := err.(*url.Error); ok {
		return true
	}

	return false
}

// isRetryableHTTPError determines if an HTTP response/error should be retried
func isRetryableHTTPError(resp *http.Response, err error, config *RetryConfig) bool {
	// If there's an error, check if it's retryable
	if err != nil {
		return isRetryableError(err, config)
	}

	// If there's a response, check the status code
	if resp != nil {
		return isRetryableStatusCode(resp.StatusCode, config)
	}

	return false
}

// isRetryableStatusCode checks if an HTTP status code should be retried
func isRetryableStatusCode(statusCode int, config *RetryConfig) bool {
	for _, code := range config.RetryableErrors {
		if statusCode == code {
			return true
		}
	}
	return false
}

// isNetworkError checks if an error is a network-related error
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// Check for net.Error interface
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout() || netErr.Temporary()
	}

	// Check for specific network error types
	switch err.(type) {
	case *net.DNSError, *net.OpError:
		return true
	}

	return false
}

// isSuccessfulResponse checks if an HTTP response indicates success
func isSuccessfulResponse(resp *http.Response) bool {
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

// getStatusCode safely extracts status code from response
func getStatusCode(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}

// calculateDelay calculates the delay for the next retry attempt
func calculateDelay(attempt int, config *RetryConfig) time.Duration {
	// Exponential backoff
	delay := float64(config.InitialDelay) * math.Pow(config.BackoffFactor, float64(attempt-1))

	// Apply jitter to prevent thundering herd
	if config.Jitter {
		jitter := delay * 0.1 * (rand.Float64()*2 - 1) // ±10% jitter
		delay += jitter
	}

	// Ensure delay is within bounds
	if delay < 0 {
		delay = float64(config.InitialDelay)
	}
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}

	return time.Duration(delay)
}

// calculateHTTPDelay calculates delay considering HTTP Retry-After header
func calculateHTTPDelay(attempt int, resp *http.Response, config *RetryConfig) time.Duration {
	// Check for Retry-After header
	if resp != nil {
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			if seconds, err := time.ParseDuration(retryAfter + "s"); err == nil {
				// Don't exceed max delay
				if seconds > config.MaxDelay {
					return config.MaxDelay
				}
				return seconds
			}
		}
	}

	// Fall back to regular exponential backoff
	return calculateDelay(attempt, config)
}
