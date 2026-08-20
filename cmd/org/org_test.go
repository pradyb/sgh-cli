// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package org

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig })

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close pipe writer: %v", err)
	}
	os.Stdout = orig

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read pipe: %v", err)
	}
	return string(out)
}

func orgsGraphQLBody() map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"viewer": map[string]interface{}{
				"organizations": map[string]interface{}{
					"pageInfo": map[string]interface{}{
						"startCursor":     "",
						"hasPreviousPage": false,
						"endCursor":       "",
						"hasNextPage":     false,
					},
					"nodes": []map[string]interface{}{
						{
							"login":                           "testorg",
							"name":                            "Test Org",
							"description":                     "A test organization",
							"email":                           "org@example.com",
							"websiteUrl":                      "https://example.com",
							"location":                        "Earth",
							"twitterUsername":                 "testorg",
							"createdAt":                       "2020-01-01T00:00:00Z",
							"updatedAt":                       "2023-01-01T00:00:00Z",
							"url":                             "https://github.com/testorg",
							"avatarUrl":                       "https://avatars.githubusercontent.com/u/1",
							"isVerified":                      true,
							"requiresTwoFactorAuthentication": false,
							"membersWithRole":                 map[string]interface{}{"totalCount": 5},
							"teams":                           map[string]interface{}{"totalCount": 2},
							"repositories":                    map[string]interface{}{"totalCount": 3, "totalDiskUsage": 100},
							"publicRepositories":              map[string]interface{}{"totalCount": 7},
						},
					},
				},
			},
		},
	}
}

func TestNewOrgCommand_Metadata(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := service.NewMockContext(t, mockServer)

	cmd := NewOrgCommand(ctx)

	if cmd.Use != "org <command>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "org <command>")
	}
	sub, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatalf("expected a 'list' subcommand: %v", err)
	}
	if sub.Use != "list" {
		t.Errorf("subcommand Use = %q, want %q", sub.Use, "list")
	}
	found := false
	for _, alias := range sub.Aliases {
		if alias == "ls" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'ls' alias on list command, got %v", sub.Aliases)
	}
}

func TestOrgListCommand_TableOutput_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       orgsGraphQLBody(),
	})
	ctx := service.NewMockContext(t, mockServer)

	cmd := ListCommand(ctx)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "testorg") {
		t.Errorf("expected output to mention testorg, got %q", out)
	}
	if ctx.HasError {
		t.Error("expected HasError to remain false on success")
	}
}

func TestOrgListCommand_JSONOutput_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       orgsGraphQLBody(),
	})
	ctx := service.NewMockContext(t, mockServer)
	ctx.JSON = true

	cmd := ListCommand(ctx)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, `"Login": "testorg"`) {
		t.Errorf("expected JSON output to contain testorg login, got %q", out)
	}
}

func TestOrgListCommand_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "forbidden"},
	})
	ctx := service.NewMockContext(t, mockServer)

	cmd := ListCommand(ctx)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	_ = captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !ctx.HasError {
		t.Error("expected HasError to be set on failure")
	}
}
