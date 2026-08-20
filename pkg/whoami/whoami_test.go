// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package whoami

import (
	"testing"

	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

func TestGetCurrentUser(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()

	ctx := service.NewMockContext(t, mockServer)

	user := GetCurrentUser(ctx)

	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.Login != "testuser" {
		t.Errorf("Login = %q, want %q", user.Login, "testuser")
	}
	if ctx.HasError {
		t.Error("expected HasError to remain false on success")
	}
}

func TestGetCurrentUser_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/user", testutils.MockResponse{
		StatusCode: 500,
		Body:       map[string]interface{}{"message": "boom"},
	})

	ctx := service.NewMockContext(t, mockServer)

	user := GetCurrentUser(ctx)

	if user != nil {
		t.Errorf("expected nil user on error, got %+v", user)
	}
	if !ctx.HasError {
		t.Error("expected HasError to be set on failure")
	}
}
