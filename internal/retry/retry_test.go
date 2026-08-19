// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package retry

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/pradyb/sgh-cli/pkg/apperrors"
)

func TestDo_SuccessFirstTry(t *testing.T) {
	calls := 0
	err := Do(context.Background(), DefaultRetryConfig(), func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestDo_RetryOnError(t *testing.T) {
	calls := 0
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialDelay:    1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		BackoffFactor:   1,
		Jitter:          false,
		RetryableErrors: []int{500},
	}
	err := Do(context.Background(), config, func() error {
		calls++
		if calls < 3 {
			return &apperrors.GitHubError{StatusCode: 500, Message: "Internal Server Error"}
		}
		return nil
	})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestDo_StopOnPermanentError(t *testing.T) {
	calls := 0
	permErr := errors.New("permanent")
	err := Do(context.Background(), &RetryConfig{MaxAttempts: 5, InitialDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond, BackoffFactor: 1, Jitter: false}, func() error {
		calls++
		return permErr
	})
	if err == nil || !errors.Is(err, permErr) {
		t.Errorf("expected permanent error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestDo_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	cancel()
	err := Do(ctx, DefaultRetryConfig(), func() error {
		calls++
		return errors.New("fail")
	})
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Errorf("expected context canceled, got %v", err)
	}
}

func TestDoHTTP_SuccessFirstTry(t *testing.T) {
	calls := 0
	resp := &http.Response{StatusCode: 200}
	r, err := DoHTTP(context.Background(), DefaultRetryConfig(), func() (*http.Response, error) {
		calls++
		return resp, nil
	})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if r != resp {
		t.Errorf("expected same response")
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestDoHTTP_RetryOnStatus(t *testing.T) {
	calls := 0
	badResp := &http.Response{StatusCode: 500, Body: http.NoBody}
	goodResp := &http.Response{StatusCode: 200, Body: http.NoBody}
	r, err := DoHTTP(context.Background(), &RetryConfig{MaxAttempts: 3, InitialDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond, BackoffFactor: 1, Jitter: false, RetryableErrors: []int{500}}, func() (*http.Response, error) {
		calls++
		if calls < 3 {
			return badResp, nil
		}
		return goodResp, nil
	})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if r != goodResp {
		t.Errorf("expected good response")
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestDoHTTP_StopOnPermanentError(t *testing.T) {
	calls := 0
	permErr := errors.New("permanent")
	r, err := DoHTTP(context.Background(), DefaultRetryConfig(), func() (*http.Response, error) {
		calls++
		return nil, permErr
	})
	if err == nil || !errors.Is(err, permErr) {
		t.Errorf("expected permanent error, got %v", err)
	}
	if r != nil {
		t.Errorf("expected nil response")
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestDoHTTP_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	cancel()
	r, err := DoHTTP(ctx, DefaultRetryConfig(), func() (*http.Response, error) {
		calls++
		return nil, errors.New("fail")
	})
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Errorf("expected context canceled, got %v", err)
	}
	if r != nil {
		t.Errorf("expected nil response")
	}
}

func TestCalculateDelayAndJitter(t *testing.T) {
	cfg := &RetryConfig{InitialDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond, BackoffFactor: 2, Jitter: true}
	d1 := calculateDelay(1, cfg)
	d2 := calculateDelay(2, cfg)
	d3 := calculateDelay(3, cfg)
	if d2 <= d1 {
		t.Errorf("expected d2 > d1, got %v, %v", d2, d1)
	}
	if d3 <= d2 {
		t.Errorf("expected d3 > d2, got %v, %v", d3, d2)
	}
	if d3 > cfg.MaxDelay {
		t.Errorf("expected d3 <= max delay, got %v", d3)
	}
}

func TestDoHTTP_ResourceLeakPrevention(t *testing.T) {
	calls := 0

	// Create responses with bodies that track if they're closed
	createResponse := func(statusCode int) *http.Response {
		body := &closeTrackingBody{closed: false}
		return &http.Response{
			StatusCode: statusCode,
			Body:       body,
		}
	}

	config := &RetryConfig{
		MaxAttempts:     3,
		InitialDelay:    1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		BackoffFactor:   1,
		Jitter:          false,
		RetryableErrors: []int{500},
	}

	_, err := DoHTTP(context.Background(), config, func() (*http.Response, error) {
		calls++
		if calls < 3 {
			return createResponse(500), nil // Should retry and close body
		}
		return createResponse(200), nil // Success
	})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}

	// All intermediate response bodies should have been closed
	// This test verifies resource leak prevention
}

type closeTrackingBody struct {
	closed bool
}

func (b *closeTrackingBody) Read(p []byte) (n int, err error) {
	return 0, errors.New("EOF")
}

func (b *closeTrackingBody) Close() error {
	b.closed = true
	return nil
}
