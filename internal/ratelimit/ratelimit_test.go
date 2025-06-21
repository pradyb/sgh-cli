package ratelimit

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/prady-lab/sgh-cli/pkg/apperrors"
)

func makeResponse(headers map[string]string) *http.Response {
	r := &http.Response{Header: make(http.Header)}
	for k, v := range headers {
		r.Header.Set(k, v)
	}

	// Add a proper Request field to avoid nil pointer dereference
	r.Request = &http.Request{
		URL: &url.URL{Path: "/api/test"},
	}

	return r
}

func TestUpdateFromResponseAndGetStatus(t *testing.T) {
	rl := NewRateLimiter()
	reset := time.Now().Add(1 * time.Hour).Unix()
	rl.UpdateFromResponse(makeResponse(map[string]string{
		"X-RateLimit-Limit":     "100",
		"X-RateLimit-Remaining": "42",
		"X-RateLimit-Reset":     strconv.FormatInt(reset, 10),
		"X-RateLimit-Resource":  "core",
	}))
	status := rl.GetStatus()
	info, ok := status["core"]
	if !ok {
		t.Fatal("core resource not found in status")
	}
	if info.Limit != 100 || info.Remaining != 42 || info.Resource != "core" {
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
		"X-RateLimit-Limit":     "100",
		"X-RateLimit-Remaining": "50",
		"X-RateLimit-Reset":     strconv.FormatInt(reset, 10),
		"X-RateLimit-Resource":  "core",
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := rl.WaitIfNeeded(ctx, "core"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWaitIfNeeded_Wait(t *testing.T) {
	rl := NewRateLimiter()
	// Set reset time to be in the future (use 3 seconds to avoid precision issues)
	reset := time.Now().Add(3 * time.Second).Unix()
	response := makeResponse(map[string]string{
		"X-RateLimit-Limit":     "100",
		"X-RateLimit-Remaining": "0", // Set to 0 to definitely trigger wait
		"X-RateLimit-Reset":     strconv.FormatInt(reset, 10),
		"X-RateLimit-Resource":  "core",
	})
	rl.UpdateFromResponse(response)

	// Debug: check the state after update
	status := rl.GetStatus()
	if info, ok := status["core"]; ok {
		t.Logf("Rate limit state: limit=%d, remaining=%d, reset=%v", info.Limit, info.Remaining, info.ResetTime)
		t.Logf("Current time: %v", time.Now())
		t.Logf("Wait time would be: %v", time.Until(info.ResetTime))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	err := rl.WaitIfNeeded(ctx, "core")
	elapsed := time.Since(start)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// The function should wait for the reset time when remaining is 0
	if elapsed < 2*time.Second {
		t.Errorf("should have waited at least 2s, elapsed: %v", elapsed)
	}
}

func TestWaitIfNeeded_NoWaitWhenLowButNotZero(t *testing.T) {
	rl := NewRateLimiter()
	reset := time.Now().Add(1 * time.Hour).Unix()
	rl.UpdateFromResponse(makeResponse(map[string]string{
		"X-RateLimit-Limit":     "100",
		"X-RateLimit-Remaining": "3", // Low but not zero
		"X-RateLimit-Reset":     strconv.FormatInt(reset, 10),
		"X-RateLimit-Resource":  "core",
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := rl.WaitIfNeeded(ctx, "core"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)
	// Should not wait when remaining > 0
	if elapsed > 50*time.Millisecond {
		t.Errorf("should not have waited, elapsed: %v", elapsed)
	}
}

func TestWaitIfNeeded_ContextCancel(t *testing.T) {
	rl := NewRateLimiter()
	reset := time.Now().Add(1 * time.Second).Unix()
	rl.UpdateFromResponse(makeResponse(map[string]string{
		"X-RateLimit-Limit":     "100",
		"X-RateLimit-Remaining": "0",
		"X-RateLimit-Reset":     strconv.FormatInt(reset, 10),
		"X-RateLimit-Resource":  "core",
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := rl.WaitIfNeeded(ctx, "core")
	if err == nil {
		t.Error("expected context deadline exceeded error")
	}
}

func TestIsRateLimited(t *testing.T) {
	rl := NewRateLimiter()
	reset := time.Now().Add(1 * time.Second).Unix()
	rl.UpdateFromResponse(makeResponse(map[string]string{
		"X-RateLimit-Limit":     "100",
		"X-RateLimit-Remaining": "0",
		"X-RateLimit-Reset":     strconv.FormatInt(reset, 10),
		"X-RateLimit-Resource":  "core",
	}))
	if !rl.IsRateLimited("core") {
		t.Error("should be rate limited")
	}
}

func TestGetRemainingRequests(t *testing.T) {
	rl := NewRateLimiter()
	reset := time.Now().Add(1 * time.Second).Unix()
	rl.UpdateFromResponse(makeResponse(map[string]string{
		"X-RateLimit-Limit":     "100",
		"X-RateLimit-Remaining": "7",
		"X-RateLimit-Reset":     strconv.FormatInt(reset, 10),
		"X-RateLimit-Resource":  "core",
	}))
	rem, ok := rl.GetRemainingRequests("core")
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
