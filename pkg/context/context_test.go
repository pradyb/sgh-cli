// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package context

import (
	"strings"
	"testing"
	"time"
)

func TestValidateGitHubToken(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantErr string
	}{
		{name: "empty token", token: "", wantErr: "cannot be empty"},
		{name: "too short", token: "ghp_abc", wantErr: "too short"},
		{name: "contains spaces", token: "ghp_ abcdefghijklmnopqrstuvwxyz012345", wantErr: "spaces"},
		{name: "test token prefix", token: "ghp_test_abcdefghijklmnopqrstuvwxyz012345", wantErr: "test token"},
		{name: "test_ prefix", token: "test_abcdefghijklmnopqrstuvwxyz0123456789", wantErr: "test token"},
		{name: "invalid prefix", token: "xyz_abcdefghijklmnopqrstuvwxyz0123456789", wantErr: "format appears invalid"},
		{name: "valid ghp_ token", token: "ghp_1234567890abcdef1234567890abcdef123456", wantErr: ""},
		{name: "valid github_pat_ token", token: "github_pat_1234567890abcdef1234567890abcdef123456", wantErr: ""},
		{name: "valid gho_ token", token: "gho_1234567890abcdef1234567890abcdef123456", wantErr: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGitHubToken(tt.token)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("ValidateGitHubToken(%q) = %v, want nil", tt.token, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateGitHubToken(%q) = nil, want error containing %q", tt.token, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ValidateGitHubToken(%q) error = %q, want to contain %q", tt.token, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestInit_MissingToken(t *testing.T) {
	t.Setenv("SGH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")

	ctx, err := Init()

	if err == nil {
		t.Fatal("expected an error when no token is set")
	}
	if ctx != nil {
		t.Errorf("expected nil context, got %+v", ctx)
	}
	if !strings.Contains(err.Error(), "SGH_TOKEN") {
		t.Errorf("error = %q, want it to mention SGH_TOKEN", err.Error())
	}
}

func TestInit_InvalidToken(t *testing.T) {
	t.Setenv("SGH_TOKEN", "not-a-valid-token")

	ctx, err := Init()

	if err == nil {
		t.Fatal("expected an error for an invalid token")
	}
	if ctx != nil {
		t.Errorf("expected nil context, got %+v", ctx)
	}
	if !strings.Contains(err.Error(), "invalid SGH_TOKEN") {
		t.Errorf("error = %q, want it to mention invalid SGH_TOKEN", err.Error())
	}
}

func TestInit_FallsBackToGitHubToken(t *testing.T) {
	t.Setenv("SGH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "ghp_1234567890abcdef1234567890abcdef123456")

	ctx, err := Init()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx == nil {
		t.Fatal("expected a non-nil context")
	}
	if ctx.HttpClient == nil || ctx.GraphqlClient == nil {
		t.Error("expected HttpClient and GraphqlClient to be initialized")
	}
}

func TestInit_CustomTimeout(t *testing.T) {
	t.Setenv("SGH_TOKEN", "ghp_1234567890abcdef1234567890abcdef123456")
	t.Setenv("SGH_TIMEOUT", "5s")

	ctx, err := Init()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.HttpClient.Client.Timeout != 5*time.Second {
		t.Errorf("timeout = %v, want %v", ctx.HttpClient.Client.Timeout, 5*time.Second)
	}
}

func TestInit_InvalidTimeoutFallsBackToDefault(t *testing.T) {
	t.Setenv("SGH_TOKEN", "ghp_1234567890abcdef1234567890abcdef123456")
	t.Setenv("SGH_TIMEOUT", "not-a-duration")

	ctx, err := Init()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.HttpClient.Client.Timeout != 30*time.Second {
		t.Errorf("timeout = %v, want default 30s", ctx.HttpClient.Client.Timeout)
	}
}

func TestSetVerbose(t *testing.T) {
	t.Setenv("SGH_TOKEN", "ghp_1234567890abcdef1234567890abcdef123456")
	ctx, err := Init()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.SetVerbose(true)
	if !ctx.Verbose || !ctx.HttpClient.Verbose || !ctx.GraphqlClient.Verbose {
		t.Error("expected Verbose to propagate to HttpClient and GraphqlClient")
	}

	ctx.SetVerbose(false)
	if ctx.Verbose || ctx.HttpClient.Verbose || ctx.GraphqlClient.Verbose {
		t.Error("expected Verbose to be cleared everywhere")
	}
}

func TestSetLogResponse(t *testing.T) {
	t.Setenv("SGH_TOKEN", "ghp_1234567890abcdef1234567890abcdef123456")
	ctx, err := Init()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.SetLogResponse(true)
	if !ctx.HttpClient.LogResponse {
		t.Error("expected LogResponse to be true")
	}
}

func TestSetWorkerCount(t *testing.T) {
	t.Setenv("SGH_TOKEN", "ghp_1234567890abcdef1234567890abcdef123456")
	ctx, err := Init()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.SetWorkerCount(7)
	if ctx.Config.NoOfWorkers != 7 {
		t.Errorf("NoOfWorkers = %d, want 7", ctx.Config.NoOfWorkers)
	}
}

func TestSwitchToken(t *testing.T) {
	t.Setenv("SGH_TOKEN", "ghp_1234567890abcdef1234567890abcdef123456")
	ctx, err := Init()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	originalHTTPClient := ctx.HttpClient
	ctx.SwitchToken("ghp_abcdef1234567890abcdef1234567890abcdef")

	if ctx.HttpClient == originalHTTPClient {
		t.Error("expected SwitchToken to rebuild the HTTP client")
	}
	if ctx.HttpClient.Token != "ghp_abcdef1234567890abcdef1234567890abcdef" {
		t.Errorf("Token = %q, want the new token", ctx.HttpClient.Token)
	}
}
