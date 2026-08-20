// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package client

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/pradyb/sgh-cli/pkg/apperrors"
)

func newTestHttpClient(t *testing.T) *HttpClient {
	t.Helper()
	return NewHttpClient(5*time.Second, testToken)
}

func TestSend_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := newTestHttpClient(t)
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/user", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := c.Send(req)
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
}

func TestSend_ErrorBranches(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		body           string
		wantSubstring  string
		wantStatusCode int
	}{
		{
			name:           "401 unauthorized",
			statusCode:     http.StatusUnauthorized,
			body:           `{"message":"Bad credentials"}`,
			wantSubstring:  "authentication failed",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "403 forbidden",
			statusCode:     http.StatusForbidden,
			body:           `{"message":"Forbidden"}`,
			wantSubstring:  "permission denied",
			wantStatusCode: http.StatusForbidden,
		},
		{
			name:           "404 not found",
			statusCode:     http.StatusNotFound,
			body:           `{"message":"Not Found"}`,
			wantSubstring:  "resource not found",
			wantStatusCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c := newTestHttpClient(t)
			req, err := http.NewRequest(http.MethodGet, srv.URL+"/repos/foo/bar", nil)
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}

			_, err = c.Send(req)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantSubstring) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantSubstring)
			}

			var ghErr *apperrors.GitHubError
			if !errors.As(err, &ghErr) {
				t.Fatalf("errors.As failed to unwrap *apperrors.GitHubError from %v", err)
			}
			if ghErr.StatusCode != tt.wantStatusCode {
				t.Errorf("GitHubError.StatusCode = %d, want %d", ghErr.StatusCode, tt.wantStatusCode)
			}
		})
	}
}

// This deliberately exercises the retry path: 500 is in the default
// RetryConfig's RetryableErrors list, so the client will retry (with
// exponential backoff) before finally giving up. Expect this test to take
// a few seconds.
func TestSend_ServerError_RetriesThenFails(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"Internal Server Error"}`))
	}))
	defer srv.Close()

	c := newTestHttpClient(t)
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/repos/foo/bar", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	_, err = c.Send(req)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "GitHub API server error") {
		t.Errorf("error = %q, want substring %q", err.Error(), "GitHub API server error")
	}

	var ghErr *apperrors.GitHubError
	if !errors.As(err, &ghErr) {
		t.Fatalf("errors.As failed to unwrap *apperrors.GitHubError from %v", err)
	}
	if ghErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("GitHubError.StatusCode = %d, want 500", ghErr.StatusCode)
	}

	if attempts != c.RetryConfig.MaxAttempts {
		t.Errorf("server received %d attempts, want %d (RetryConfig.MaxAttempts)", attempts, c.RetryConfig.MaxAttempts)
	}
}

func TestPrepareRequestSetsHeaders(t *testing.T) {
	var gotAuth, gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestHttpClient(t)
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/user", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := c.Send(req)
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	resp.Body.Close()

	wantAuth := "token " + testToken
	if gotAuth != wantAuth {
		t.Errorf("Authorization header = %q, want %q", gotAuth, wantAuth)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type header = %q, want application/json", gotContentType)
	}
}

func TestDetermineResource(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "graphql exact path", path: "/graphql", want: "graphql"},
		{name: "search exact path", path: "/search", want: "search"},
		{name: "search with subpath falls back to core", path: "/search/repositories", want: "core"},
		{name: "repos path", path: "/repos/foo/bar", want: "core"},
		{name: "empty path", path: "", want: "core"},
		{name: "root path", path: "/", want: "core"},
	}

	c := &HttpClient{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{URL: &url.URL{Path: tt.path}}
			if got := c.determineResource(req); got != tt.want {
				t.Errorf("determineResource(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestLogRequestIfVerbose_NoPanic(t *testing.T) {
	c := &HttpClient{Verbose: true, Token: testToken}
	req, err := http.NewRequest(http.MethodGet, "https://example.com/foo", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Authorization", "token "+testToken)

	// Must not panic; output is log-only and redaction is a side detail.
	c.logRequestIfVerbose(req)
}

func TestLogRequestIfVerbose_Disabled_NoOutput(t *testing.T) {
	c := &HttpClient{Verbose: false}
	req, err := http.NewRequest(http.MethodGet, "https://example.com/foo", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	// Must not panic, and should be a no-op since Verbose is false.
	c.logRequestIfVerbose(req)
}

func TestLogResponseIfEnabled_NoPanic(t *testing.T) {
	c := &HttpClient{LogResponse: true}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     http.Header{},
		Body:       http.NoBody,
	}
	c.logResponseIfEnabled(resp)
}

func TestLogResponseIfEnabled_NilResponse_NoPanic(t *testing.T) {
	c := &HttpClient{LogResponse: true}
	c.logResponseIfEnabled(nil)
}

func TestLogResponseIfEnabled_Disabled_NoOutput(t *testing.T) {
	c := &HttpClient{LogResponse: false}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       http.NoBody,
	}
	c.logResponseIfEnabled(resp)
}
