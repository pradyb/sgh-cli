package ratelimit

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prady-lab/sgh-cli/pkg/apperrors"
	"github.com/prady-lab/sgh-cli/pkg/logger"
)

// RateLimitInfo contains GitHub API rate limit information
type RateLimitInfo struct {
	Limit     int
	Remaining int
	ResetTime time.Time
	Resource  string
}

// RateLimiter manages GitHub API rate limiting
// bufferConfig allows custom buffer sizes per resource (for testability)
type RateLimiter struct {
	mu           sync.RWMutex
	limits       map[string]*RateLimitInfo
	waitQueue    chan struct{}
	bufferConfig map[string]int // resource -> buffer size
}

// NewRateLimiter creates a new rate limiter with optional buffer config
func NewRateLimiterWithBuffer(bufferConfig map[string]int) *RateLimiter {
	return &RateLimiter{
		limits:       make(map[string]*RateLimitInfo),
		waitQueue:    make(chan struct{}, 100),
		bufferConfig: bufferConfig,
	}
}

// NewRateLimiter creates a new rate limiter with default buffer config
func NewRateLimiter() *RateLimiter {
	return NewRateLimiterWithBuffer(nil)
}

// UpdateFromResponse updates rate limit info from HTTP response headers
func (rl *RateLimiter) UpdateFromResponse(resp *http.Response) {
	if resp == nil {
		return
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	resource := resp.Header.Get("X-RateLimit-Resource")
	if resource == "" {
		// Try to determine resource from URL path
		if resp.Request != nil && resp.Request.URL != nil {
			switch resp.Request.URL.Path {
			case "/graphql":
				resource = "graphql"
			case "/search":
				resource = "search"
			default:
				resource = "core" // Default to core API
			}
		} else {
			resource = "core" // Default to core API
		}
	}

	limit, _ := strconv.Atoi(resp.Header.Get("X-RateLimit-Limit"))
	remaining, _ := strconv.Atoi(resp.Header.Get("X-RateLimit-Remaining"))
	resetTime, _ := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64)

	// Validate the parsed values
	if limit <= 0 {
		logger.Flog.Warn().
			Str("resource", resource).
			Str("header", resp.Header.Get("X-RateLimit-Limit")).
			Msg("Invalid rate limit header, using default")
		limit = 5000 // Default limit
	}

	if remaining < 0 {
		logger.Flog.Warn().
			Str("resource", resource).
			Str("header", resp.Header.Get("X-RateLimit-Remaining")).
			Msg("Invalid remaining requests header, using 0")
		remaining = 0
	}

	if resetTime <= 0 {
		logger.Flog.Warn().
			Str("resource", resource).
			Str("header", resp.Header.Get("X-RateLimit-Reset")).
			Msg("Invalid reset time header, using current time + 1 hour")
		resetTime = time.Now().Add(time.Hour).Unix()
	}

	info := &RateLimitInfo{
		Limit:     limit,
		Remaining: remaining,
		ResetTime: time.Unix(resetTime, 0),
		Resource:  resource,
	}

	rl.limits[resource] = info

	logger.Flog.Debug().
		Str("resource", resource).
		Int("limit", limit).
		Int("remaining", remaining).
		Time("reset", info.ResetTime).
		Str("url", resp.Request.URL.String()).
		Msg("Updated rate limit info")
}

// WaitIfNeeded waits if rate limit is exceeded
func (rl *RateLimiter) WaitIfNeeded(ctx context.Context, resource string) error {
	rl.mu.RLock()
	info, exists := rl.limits[resource]
	bufferConfig := rl.bufferConfig
	rl.mu.RUnlock()

	if !exists {
		logger.Flog.Debug().
			Str("resource", resource).
			Msg("No rate limit info available, proceeding")
		return nil // No rate limit info available, proceed
	}

	// Use buffer from config if set, else use default
	bufferSize := 5 // Default for core
	if bufferConfig != nil {
		if buf, ok := bufferConfig[resource]; ok {
			bufferSize = buf
		}
	}
	if resource == "graphql" && (bufferConfig == nil || bufferSize == 5) {
		bufferSize = 2
	} else if resource == "search" && (bufferConfig == nil || bufferSize == 5) {
		bufferSize = 3
	}

	// If we have enough remaining requests, proceed
	if info.Remaining > bufferSize {
		logger.Flog.Debug().
			Str("resource", resource).
			Int("remaining", info.Remaining).
			Int("buffer", bufferSize).
			Msg("Sufficient rate limit remaining, proceeding")
		return nil
	}

	// Calculate wait time
	waitTime := time.Until(info.ResetTime)
	if waitTime <= 0 {
		logger.Flog.Debug().
			Str("resource", resource).
			Msg("Rate limit reset time has passed, proceeding")
		return nil // Reset time has passed
	}

	// Only wait if we're actually at zero or very low
	if info.Remaining > 0 {
		logger.Flog.Debug().
			Str("resource", resource).
			Int("remaining", info.Remaining).
			Int("buffer", bufferSize).
			Msg("Rate limit low but not exhausted, proceeding with caution")
		return nil
	}

	logger.Glog.Warn().
		Str("resource", resource).
		Int("remaining", info.Remaining).
		Dur("waitTime", waitTime).
		Msg("Rate limit exceeded, waiting for reset")

	// Wait for rate limit reset with context cancellation support
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(waitTime):
		logger.Glog.Info().
			Str("resource", resource).
			Msg("Rate limit reset, resuming requests")
		return nil
	}
}

// GetRemainingRequests returns the number of remaining requests for a resource
func (rl *RateLimiter) GetRemainingRequests(resource string) (int, bool) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	info, exists := rl.limits[resource]
	if !exists {
		return 0, false
	}

	return info.Remaining, true
}

// IsRateLimited checks if we're currently rate limited
func (rl *RateLimiter) IsRateLimited(resource string) bool {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	info, exists := rl.limits[resource]
	if !exists {
		return false
	}

	return info.Remaining <= 0 && time.Now().Before(info.ResetTime)
}

// GetStatus returns current rate limit status for all resources
func (rl *RateLimiter) GetStatus() map[string]RateLimitInfo {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	status := make(map[string]RateLimitInfo)
	for resource, info := range rl.limits {
		status[resource] = *info
	}

	return status
}

// HandleRateLimitError processes rate limit errors and calculates wait time
func (rl *RateLimiter) HandleRateLimitError(err error) (time.Duration, bool) {
	if githubErr, ok := err.(*apperrors.GitHubError); ok && githubErr.IsRateLimit() {
		// Extract retry-after header if available
		// For now, use a default backoff strategy
		return calculateBackoffDuration(1), true
	}
	return 0, false
}

// calculateBackoffDuration calculates exponential backoff duration
func calculateBackoffDuration(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}

	// Exponential backoff: 2^attempt seconds, max 5 minutes
	duration := time.Duration(1<<uint(attempt-1)) * time.Second
	maxDuration := 5 * time.Minute

	if duration > maxDuration {
		duration = maxDuration
	}

	return duration
}

// RefreshRateLimit manually refreshes rate limit information for a resource
func (rl *RateLimiter) RefreshRateLimit(resource string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Clear the rate limit info for the resource to force a fresh check
	delete(rl.limits, resource)

	logger.Flog.Debug().
		Str("resource", resource).
		Msg("Rate limit information cleared, will refresh on next request")
}

// ForceUpdateRateLimit manually updates rate limit info (useful for testing or manual override)
func (rl *RateLimiter) ForceUpdateRateLimit(resource string, limit, remaining int, resetTime time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	info := &RateLimitInfo{
		Limit:     limit,
		Remaining: remaining,
		ResetTime: resetTime,
		Resource:  resource,
	}

	rl.limits[resource] = info

	logger.Flog.Info().
		Str("resource", resource).
		Int("limit", limit).
		Int("remaining", remaining).
		Time("reset", resetTime).
		Msg("Rate limit information manually updated")
}

// GetRateLimitInfo returns detailed rate limit information for a resource
func (rl *RateLimiter) GetRateLimitInfo(resource string) (*RateLimitInfo, bool) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	info, exists := rl.limits[resource]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid race conditions
	infoCopy := *info
	return &infoCopy, true
}
