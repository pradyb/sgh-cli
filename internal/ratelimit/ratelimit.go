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
type RateLimiter struct {
	mu        sync.RWMutex
	limits    map[string]*RateLimitInfo
	waitQueue chan struct{}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limits:    make(map[string]*RateLimitInfo),
		waitQueue: make(chan struct{}, 100), // Buffer for 100 concurrent requests
	}
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
		resource = "core" // Default to core API
	}

	limit, _ := strconv.Atoi(resp.Header.Get("X-RateLimit-Limit"))
	remaining, _ := strconv.Atoi(resp.Header.Get("X-RateLimit-Remaining"))
	resetTime, _ := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64)

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
		Msg("Updated rate limit info")
}

// WaitIfNeeded waits if rate limit is exceeded
func (rl *RateLimiter) WaitIfNeeded(ctx context.Context, resource string) error {
	rl.mu.RLock()
	info, exists := rl.limits[resource]
	rl.mu.RUnlock()

	if !exists {
		return nil // No rate limit info available, proceed
	}

	// If we have remaining requests or reset time has passed, proceed
	if info.Remaining > 0 || time.Now().After(info.ResetTime) {
		return nil
	}

	// Calculate wait time
	waitTime := time.Until(info.ResetTime)
	if waitTime <= 0 {
		return nil // Reset time has passed
	}

	logger.Flog.Warn().
		Str("resource", resource).
		Int("remaining", info.Remaining).
		Dur("waitTime", waitTime).
		Msg("Rate limit exceeded, waiting for reset")

	// Wait for rate limit reset with context cancellation support
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(waitTime):
		logger.Flog.Info().
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
