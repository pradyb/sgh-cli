// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package audit

import (
	"net/http"
	"testing"

	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

func TestListAuditLog(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/orgs/testorg/audit-log", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: []map[string]interface{}{
			{
				"action":     "team.create",
				"actor":      "jane-doe",
				"actor_ip":   "127.0.0.1",
				"created_at": int64(1700000000000),
				"org":        "testorg",
				"user":       "jane-doe",
			},
		},
	})

	ctx := service.NewMockContext(t, mockServer)

	resp := ListAuditLog(ctx, AuditListRequest{OrgName: "testorg", Count: 10})

	if resp.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", resp.ErrorMessage)
	}
	if resp.OrgName != "testorg" {
		t.Errorf("OrgName = %q, want %q", resp.OrgName, "testorg")
	}
	if len(resp.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(resp.Entries))
	}
	if resp.Entries[0].Action != "team.create" {
		t.Errorf("Action = %q, want %q", resp.Entries[0].Action, "team.create")
	}
}

func TestListAuditLog_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/orgs/testorg/audit-log", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := service.NewMockContext(t, mockServer)

	resp := ListAuditLog(ctx, AuditListRequest{OrgName: "testorg"})

	if resp.ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be set")
	}
	if resp.OrgName != "testorg" {
		t.Errorf("OrgName = %q, want %q", resp.OrgName, "testorg")
	}
	if resp.Entries != nil {
		t.Errorf("expected nil Entries on error, got %v", resp.Entries)
	}
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		name string
		ms   int64
		want string
	}{
		{name: "zero returns empty", ms: 0, want: ""},
		{name: "known epoch millis", ms: 1700000000000, want: "2023-11-14 22:13:20 UTC"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatTimestamp(tt.ms); got != tt.want {
				t.Errorf("FormatTimestamp(%d) = %q, want %q", tt.ms, got, tt.want)
			}
		})
	}
}
