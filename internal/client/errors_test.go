// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pradyb/sgh-cli/pkg/apperrors"
)

func TestExtractGitHubMessage(t *testing.T) {
	longText := strings.Repeat("x", 250)

	tests := []struct {
		name       string
		body       string
		statusCode int
		want       string
	}{
		{
			name:       "valid JSON with message only",
			body:       `{"message":"Bad credentials"}`,
			statusCode: http.StatusUnauthorized,
			want:       "Bad credentials",
		},
		{
			name:       "valid JSON with message and nested errors",
			body:       `{"message":"Validation Failed","errors":[{"message":"name already exists"}]}`,
			statusCode: http.StatusUnprocessableEntity,
			want:       "Validation Failed — name already exists",
		},
		{
			name:       "invalid JSON short body",
			body:       "plain text error",
			statusCode: http.StatusInternalServerError,
			want:       "plain text error",
		},
		{
			name:       "invalid JSON long body gets truncated",
			body:       longText,
			statusCode: http.StatusInternalServerError,
			want:       longText[:200] + "...",
		},
		{
			name:       "JSON without message field falls back to raw body",
			body:       `{"foo":"bar"}`,
			statusCode: http.StatusBadRequest,
			want:       `{"foo":"bar"}`,
		},
		{
			name:       "empty body",
			body:       "",
			statusCode: http.StatusNotFound,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractGitHubMessage([]byte(tt.body), tt.statusCode)
			if got != tt.want {
				t.Errorf("extractGitHubMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDoSingleRequest_ErrorWraps(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantMsg    string
	}{
		{
			name:       "JSON body with message",
			statusCode: http.StatusForbidden,
			body:       `{"message":"API rate limit exceeded"}`,
			wantMsg:    "API rate limit exceeded",
		},
		{
			name:       "non-JSON body under truncation threshold",
			statusCode: http.StatusNotFound,
			body:       "short plain text",
			wantMsg:    "short plain text",
		},
		{
			name:       "non-JSON body over truncation threshold",
			statusCode: http.StatusInternalServerError,
			body:       strings.Repeat("y", 300),
			wantMsg:    strings.Repeat("y", 200) + "...",
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

			_, err = c.doSingleRequest(context.Background(), req)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			var ghErr *apperrors.GitHubError
			if !errors.As(err, &ghErr) {
				t.Fatalf("errors.As failed to unwrap *apperrors.GitHubError from %v", err)
			}
			if ghErr.StatusCode != tt.statusCode {
				t.Errorf("StatusCode = %d, want %d", ghErr.StatusCode, tt.statusCode)
			}
			if ghErr.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", ghErr.Message, tt.wantMsg)
			}
			if ghErr.URL != req.URL.String() {
				t.Errorf("URL = %q, want %q", ghErr.URL, req.URL.String())
			}
		})
	}
}

func TestHandleError_NonGitHubTimeoutError(t *testing.T) {
	c := &HttpClient{}
	timeoutErr := &timeoutOnlyError{}

	_, err := c.handleError(nil, timeoutErr)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "request timeout") {
		t.Errorf("error = %q, want substring %q", err.Error(), "request timeout")
	}
}

func TestHandleError_GenericError(t *testing.T) {
	c := &HttpClient{}
	genericErr := errors.New("boom")

	_, err := c.handleError(nil, genericErr)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP request failed") {
		t.Errorf("error = %q, want substring %q", err.Error(), "HTTP request failed")
	}
}

// timeoutOnlyError implements the minimal interface{ Timeout() bool } that
// handleError checks for, without being a *net.OpError or GitHubError.
type timeoutOnlyError struct{}

func (e *timeoutOnlyError) Error() string   { return "i/o timeout" }
func (e *timeoutOnlyError) Timeout() bool   { return true }
func (e *timeoutOnlyError) Temporary() bool { return true }
