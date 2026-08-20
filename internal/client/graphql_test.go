// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pradyb/sgh-cli/internal/circuitbreaker"
	"github.com/pradyb/sgh-cli/internal/ratelimit"
	"github.com/pradyb/sgh-cli/internal/retry"
	"github.com/shurcooL/githubv4"
)

type viewerLoginQuery struct {
	Viewer struct {
		Login githubv4.String
	}
}

// fastRetryConfig avoids the default 1s+ backoff delays for tests that
// intentionally hit an error path.
func fastRetryConfig() *retry.RetryConfig {
	return &retry.RetryConfig{
		MaxAttempts:   1,
		InitialDelay:  time.Millisecond,
		MaxDelay:      time.Millisecond,
		BackoffFactor: 1,
	}
}

func newGraphqlClient(t *testing.T, serverURL string) *GraphqlClient {
	t.Helper()
	return &GraphqlClient{
		Client:         githubv4.NewEnterpriseClient(serverURL, http.DefaultClient),
		RateLimiter:    ratelimit.NewRateLimiter(),
		RetryConfig:    fastRetryConfig(),
		CircuitBreaker: circuitbreaker.New(circuitbreaker.DefaultConfig()),
	}
}

func TestGraphqlClient_QuerySuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"viewer": map[string]interface{}{"login": "testuser"},
			},
		})
	}))
	defer srv.Close()

	gc := newGraphqlClient(t, srv.URL)
	gc.Verbose = true // also exercises the verbose logging branch

	var q viewerLoginQuery
	if err := gc.Query(&q, nil); err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if string(q.Viewer.Login) != "testuser" {
		t.Errorf("Viewer.Login = %q, want %q", q.Viewer.Login, "testuser")
	}
}

func TestGraphqlClient_QueryError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data":   nil,
			"errors": []map[string]interface{}{{"message": "something went wrong"}},
		})
	}))
	defer srv.Close()

	gc := newGraphqlClient(t, srv.URL)

	var q viewerLoginQuery
	err := gc.QueryWithContext(context.Background(), &q, nil)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

// A nil RateLimiter must not cause QueryWithContext to panic; the "graphql"
// resource wait is skipped entirely in that case.
func TestGraphqlClient_NilRateLimiter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"viewer": map[string]interface{}{"login": "testuser"},
			},
		})
	}))
	defer srv.Close()

	gc := &GraphqlClient{
		Client:         githubv4.NewEnterpriseClient(srv.URL, http.DefaultClient),
		RateLimiter:    nil,
		RetryConfig:    fastRetryConfig(),
		CircuitBreaker: circuitbreaker.New(circuitbreaker.DefaultConfig()),
	}

	var q viewerLoginQuery
	if err := gc.Query(&q, map[string]interface{}{"foo": "bar"}); err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
}
