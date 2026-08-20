// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package security

import (
	"net/http"
	"testing"

	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

func alertsBody() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"number":                   1,
			"secret_type":              "github_pat",
			"secret_type_display_name": "GitHub Personal Access Token",
			"state":                    "open",
			"created_at":               "2024-01-01T00:00:00Z",
			"updated_at":               "2024-01-01T00:00:00Z",
			"html_url":                 "https://github.com/testorg/repo1/security/secret-scanning/1",
			"location":                 map[string]interface{}{"path": "config.yml", "start_line": 3, "end_line": 3},
		},
		{
			"number":                   2,
			"secret_type":              "aws_access_key_id",
			"secret_type_display_name": "AWS Access Key ID",
			"state":                    "resolved",
			"resolution":               "false_positive",
			"created_at":               "2024-02-01T00:00:00Z",
			"updated_at":               "2024-02-02T00:00:00Z",
			"html_url":                 "https://github.com/testorg/repo1/security/secret-scanning/2",
			"location":                 map[string]interface{}{"path": "secrets.env", "start_line": 1, "end_line": 1},
		},
	}
}

func TestListSecretScanningAlerts_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/secret-scanning/alerts", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       alertsBody(),
	})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	alerts := ListSecretScanningAlerts(ctx, AlertListRequest{OrgName: "testorg", RepoNames: []string{"repo1"}})

	if len(alerts) != 2 {
		t.Fatalf("len(alerts) = %d, want 2", len(alerts))
	}
	for _, a := range alerts {
		if a.RepositoryName != "repo1" {
			t.Errorf("RepositoryName = %q, want %q", a.RepositoryName, "repo1")
		}
	}
}

func TestListSecretScanningAlerts_FilterBySecretType(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/secret-scanning/alerts", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       alertsBody(),
	})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	alerts := ListSecretScanningAlerts(ctx, AlertListRequest{
		OrgName:    "testorg",
		RepoNames:  []string{"repo1"},
		SecretType: "AWS_Access_Key_ID", // exercise case-insensitive match
	})

	if len(alerts) != 1 {
		t.Fatalf("len(alerts) = %d, want 1", len(alerts))
	}
	if alerts[0].Number != 2 {
		t.Errorf("Number = %d, want 2", alerts[0].Number)
	}
}

func TestListSecretScanningAlerts_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/secret-scanning/alerts", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	alerts := ListSecretScanningAlerts(ctx, AlertListRequest{OrgName: "testorg", RepoNames: []string{"repo1"}})

	if len(alerts) != 1 {
		t.Fatalf("len(alerts) = %d, want 1", len(alerts))
	}
	if alerts[0].ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
	if alerts[0].RepositoryName != "repo1" {
		t.Errorf("RepositoryName = %q, want %q", alerts[0].RepositoryName, "repo1")
	}
}

func TestGetSecretScanningAlert_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/secret-scanning/alerts/1", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"number":      1,
			"secret_type": "github_pat",
			"state":       "open",
			"html_url":    "https://github.com/testorg/repo1/security/secret-scanning/1",
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	alert := GetSecretScanningAlert(ctx, AlertViewRequest{OrgName: "testorg", RepoName: "repo1", AlertNumber: 1})

	if alert.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", alert.ErrorMessage)
	}
	if alert.Number != 1 {
		t.Errorf("Number = %d, want 1", alert.Number)
	}
	if alert.RepositoryName != "repo1" {
		t.Errorf("RepositoryName = %q, want %q", alert.RepositoryName, "repo1")
	}
}

func TestGetSecretScanningAlert_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/secret-scanning/alerts/99", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	alert := GetSecretScanningAlert(ctx, AlertViewRequest{OrgName: "testorg", RepoName: "repo1", AlertNumber: 99})

	if alert.ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
	if alert.RepositoryName != "repo1" {
		t.Errorf("RepositoryName = %q, want %q", alert.RepositoryName, "repo1")
	}
}

func TestUpdateSecretScanningAlert_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/secret-scanning/alerts/1", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"number":             1,
			"state":              "resolved",
			"resolution":         "false_positive",
			"resolution_comment": "not a real secret",
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	alert := UpdateSecretScanningAlert(ctx, AlertUpdateRequest{
		OrgName: "testorg", RepoName: "repo1", AlertNumber: 1,
		State: "resolved", Resolution: "false_positive", ResolutionComment: "not a real secret",
	})

	if alert.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", alert.ErrorMessage)
	}
	if alert.State != "resolved" || alert.Resolution != "false_positive" {
		t.Errorf("unexpected alert state: %+v", alert)
	}
	if alert.RepositoryName != "repo1" {
		t.Errorf("RepositoryName = %q, want %q", alert.RepositoryName, "repo1")
	}
}

func TestUpdateSecretScanningAlert_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/repo1/secret-scanning/alerts/1", testutils.MockResponse{
		StatusCode: http.StatusUnprocessableEntity,
		Body:       map[string]interface{}{"message": "invalid resolution"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	alert := UpdateSecretScanningAlert(ctx, AlertUpdateRequest{
		OrgName: "testorg", RepoName: "repo1", AlertNumber: 1, State: "resolved", Resolution: "wont_fix",
	})

	if alert.ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
	if alert.RepositoryName != "repo1" {
		t.Errorf("RepositoryName = %q, want %q", alert.RepositoryName, "repo1")
	}
}
