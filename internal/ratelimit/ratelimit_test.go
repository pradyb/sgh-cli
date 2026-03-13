// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package ratelimit

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/prady-lab/sgh-cli/pkg/apperrors"
)

const (
	headerRateLimitLimit     = "X-RateLimit-Limit"
	headerRateLimitRemaining = "X-RateLimit-Remaining"
	headerRateLimitReset     = "X-RateLimit-Reset"
	headerRateLimitResource  = "X-RateLimit-Resource"
	coreResource             = "core"
)

func makeResponse(headers map[string]string) *http.Response {
	r := &http.Response{Header: make(http.Header)}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

func TestUpdateFromResponseAndGetStatus(t *testing.T) {
	rl := NewRateLimiter()
	reset := time.Now().Add(1 * time.Hour).Unix()
	rl.UpdateFromResponse(makeResponse(map[string]string{
		headerRateLimitLimit:     "100",
		headerRateLimitRemaining: "42",
		headerRateLimitReset:     strconv.FormatInt(reset, 10),
		headerRateLimitResource:  coreResource,
	}))
	status := rl.GetStatus()
	info, ok := status[coreResource]
	if !ok {
		t.Fatal("core resource not found in status")
	}
	if info.Limit != 100 || info.Remaining != 42 || info.Resource != coreResource {
		t.Errorf("unexpected rate limit info: %+v", info)
	}
	if info.ResetTime.Unix() != reset {
		t.Errorf("unexpected reset time: got %v want %v", info.ResetTime.Unix(), reset)
	}
}

func TestWaitIfNeeded_NoWait(t *testing.T) {
	rl := NewRateLimiter()
	reset := time.Now().Add(1 * time.Second).Unix()
	rl.UpdateFromResponse(makeResponse(map[string]string{
		headerRateLimitLimit:     "100",
		headerRateLimitRemaining: "50",
		headerRateLimitReset:     strconv.FormatInt(reset, 10),
		headerRateLimitResource:  coreResource,
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := rl.WaitIfNeeded(ctx, coreResource); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWaitIfNeeded_Wait(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing-sensitive test in short mode")
	}
	rl := NewRateLimiter()
	// Set reset time 4s in the future so there is enough headroom for slow CI runners.
	reset := time.Now().Add(4 * time.Second).Unix()
	response := makeResponse(map[string]string{
		headerRateLimitLimit:     "100",
		headerRateLimitRemaining: "0", // Set to 0 to definitely trigger wait
		headerRateLimitReset:     strconv.FormatInt(reset, 10),
		headerRateLimitResource:  coreResource,
	})
	rl.UpdateFromResponse(response)

	// Debug: check the state after update
	status := rl.GetStatus()
	if info, ok := status[coreResource]; ok {
		t.Logf("Rate limit state: limit=%d, remaining=%d, reset=%v", info.Limit, info.Remaining, info.ResetTime)
		t.Logf("Current time: %v", time.Now())
		t.Logf("Wait time would be: %v", time.Until(info.ResetTime))

		// Ensure we actually need to wait
		if time.Until(info.ResetTime) <= 0 {
			t.Skip("Reset time already passed, cannot test wait behavior")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	start := time.Now()
	err := rl.WaitIfNeeded(ctx, coreResource)
	elapsed := time.Since(start)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// Should wait at least 1s (generous lower bound for slow runners)
	if elapsed < 1000*time.Millisecond {
		t.Errorf("should have waited at least 1s, elapsed: %v", elapsed)
	}
	// But not more than 5s
	if elapsed > 5000*time.Millisecond {
		t.Errorf("waited too long, elapsed: %v", elapsed)
	}
}

func TestWaitIfNeeded_ContextCancel(t *testing.T) {
	rl := NewRateLimiter()
	// Set reset time to be well in the future to ensure we need to wait
	reset := time.Now().Add(5 * time.Second).Unix()
	rl.UpdateFromResponse(makeResponse(map[string]string{
		headerRateLimitLimit:     "100",
		headerRateLimitRemaining: "0",
		headerRateLimitReset:     strconv.FormatInt(reset, 10),
		headerRateLimitResource:  coreResource,
	}))

	// Use a longer timeout to ensure we're testing cancellation, not timeout
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := rl.WaitIfNeeded(ctx, coreResource)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected context deadline exceeded error")
	}

	// Should have been cancelled around 200ms, not waited the full 5 seconds
	if elapsed > 300*time.Millisecond {
		t.Errorf("context cancellation took too long: %v", elapsed)
	}
}

func TestIsRateLimited(t *testing.T) {
	rl := NewRateLimiter()
	// Set reset time to be in the future to ensure rate limiting is active
	reset := time.Now().Add(30 * time.Second).Unix()
	rl.UpdateFromResponse(makeResponse(map[string]string{
		headerRateLimitLimit:     "100",
		headerRateLimitRemaining: "0",
		headerRateLimitReset:     strconv.FormatInt(reset, 10),
		headerRateLimitResource:  coreResource,
	}))

	if !rl.IsRateLimited(coreResource) {
		t.Error("should be rate limited when remaining=0 and reset time is in future")
	}

	// Test case where we're not rate limited
	rl.UpdateFromResponse(makeResponse(map[string]string{
		headerRateLimitLimit:     "100",
		headerRateLimitRemaining: "50",
		headerRateLimitReset:     strconv.FormatInt(reset, 10),
		headerRateLimitResource:  coreResource,
	}))

	if rl.IsRateLimited(coreResource) {
		t.Error("should not be rate limited when remaining > 0")
	}
}

func TestGetRemainingRequests(t *testing.T) {
	rl := NewRateLimiter()
	reset := time.Now().Add(1 * time.Second).Unix()
	rl.UpdateFromResponse(makeResponse(map[string]string{
		headerRateLimitLimit:     "100",
		headerRateLimitRemaining: "7",
		headerRateLimitReset:     strconv.FormatInt(reset, 10),
		headerRateLimitResource:  coreResource,
	}))
	rem, ok := rl.GetRemainingRequests(coreResource)
	if !ok || rem != 7 {
		t.Errorf("expected 7 remaining, got %d", rem)
	}
}

func TestHandleRateLimitError(t *testing.T) {
	rl := NewRateLimiter()
	err := &apperrors.GitHubError{StatusCode: 403, Message: "API rate limit exceeded"}
	dur, ok := rl.HandleRateLimitError(err)
	if !ok || dur <= 0 {
		t.Errorf("expected backoff duration, got %v, %v", dur, ok)
	}
}

func TestWaitIfNeeded_SmallRemaining(t *testing.T) {
	rl := NewRateLimiter()
	reset := time.Now().Add(2 * time.Second).Unix()

	// Test with small but positive remaining requests
	rl.UpdateFromResponse(makeResponse(map[string]string{
		headerRateLimitLimit:     "100",
		headerRateLimitRemaining: "1", // Should not wait when remaining > 0
		headerRateLimitReset:     strconv.FormatInt(reset, 10),
		headerRateLimitResource:  coreResource,
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := rl.WaitIfNeeded(ctx, coreResource)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Should not wait when remaining > 0
	if elapsed > 100*time.Millisecond {
		t.Errorf("should not have waited when remaining > 0, elapsed: %v", elapsed)
	}
}

// TestWaitIfNeeded_EdgeCases tests additional edge cases for rate limiting
func TestWaitIfNeeded_EdgeCases(t *testing.T) {
	t.Run("expired reset time", func(t *testing.T) {
		rl := NewRateLimiter()
		// Set reset time in the past
		reset := time.Now().Add(-1 * time.Second).Unix()
		rl.UpdateFromResponse(makeResponse(map[string]string{
			headerRateLimitLimit:     "100",
			headerRateLimitRemaining: "0",
			headerRateLimitReset:     strconv.FormatInt(reset, 10),
			headerRateLimitResource:  coreResource,
		}))

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		start := time.Now()
		err := rl.WaitIfNeeded(ctx, coreResource)
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Should not wait when reset time has passed
		if elapsed > 50*time.Millisecond {
			t.Errorf("should not have waited when reset time has passed, elapsed: %v", elapsed)
		}
	})

	t.Run("unknown resource", func(t *testing.T) {
		rl := NewRateLimiter()
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := rl.WaitIfNeeded(ctx, "unknown-resource")
		if err != nil {
			t.Errorf("unexpected error for unknown resource: %v", err)
		}
	})
}
