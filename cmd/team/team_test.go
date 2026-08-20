// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package team

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
	"github.com/pradyb/sgh-cli/pkg/context"
)

// newTestRoot builds a minimal parent command that only defines the
// persistent flags the team command reads (org, json, limit), with no
// PersistentPreRun/PersistentPostRun — unlike the real root command, which
// os.Exit(1)s on flag validation issues or ctx.HasError, making it unsafe to
// drive error-path tests through.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().StringP("org", "o", "", "")
	root.PersistentFlags().BoolP("json", "J", false, "")
	root.PersistentFlags().Int("limit", 0, "")
	return root
}

func teamGraphQLBody() map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"organization": map[string]interface{}{
				"name": "testorg",
				"url":  "https://github.com/testorg",
				"teams": map[string]interface{}{
					"pageInfo": map[string]interface{}{"startCursor": "", "hasPreviousPage": false, "endCursor": "", "hasNextPage": false},
					"edges": []map[string]interface{}{
						{
							"node": map[string]interface{}{
								"name": "team-a",
								"url":  "https://github.com/orgs/testorg/teams/team-a",
								"members": map[string]interface{}{
									"totalCount": 2,
									"edges": []map[string]interface{}{
										{"node": map[string]interface{}{"login": "alice", "name": "Alice", "bio": "", "websiteUrl": ""}},
										{"node": map[string]interface{}{"login": "bob", "name": "Bob", "bio": "", "websiteUrl": ""}},
									},
									"pageInfo": map[string]interface{}{"startCursor": "", "hasPreviousPage": false, "endCursor": "", "hasNextPage": false},
								},
								"repositories": map[string]interface{}{"totalCount": 5},
							},
						},
					},
				},
			},
		},
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w

	// Drain concurrently: the anonymous pipe's buffer is small, and a
	// table-heavy Run can easily exceed it, so draining only after fn()
	// returns would deadlock.
	outCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outCh <- buf.String()
	}()

	fn()

	os.Stdout = orig
	w.Close()
	return <-outCh
}

func newMockedContext(t *testing.T) (*context.Context, *testutils.MockGitHubServer) {
	t.Helper()
	mockServer := testutils.NewMockGitHubServer()
	t.Cleanup(mockServer.Close)
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       teamGraphQLBody(),
	})
	ctx := servicetest.NewMockContext(t, mockServer)
	return ctx, mockServer
}

func TestListCommand_Success_TableOutput(t *testing.T) {
	ctx, mockServer := newMockedContext(t)

	root := newTestRoot()
	root.AddCommand(NewTeamCommand(ctx))
	root.SetArgs([]string{"team", "list", "--org", "testorg"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "team-a") {
		t.Errorf("expected output to mention team-a, got: %s", out)
	}

	requests := mockServer.GetRequests()
	if len(requests) == 0 {
		t.Fatal("expected at least one request to the mock server")
	}
}

func TestListCommand_Success_JSONOutput(t *testing.T) {
	ctx, _ := newMockedContext(t)
	ctx.JSON = true

	root := newTestRoot()
	root.AddCommand(NewTeamCommand(ctx))
	root.SetArgs([]string{"team", "list", "--org", "testorg"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, `"team-a"`) && !strings.Contains(out, "team-a") {
		t.Errorf("expected JSON output to mention team-a, got: %s", out)
	}
	if !strings.Contains(out, "{") {
		t.Errorf("expected JSON-shaped output, got: %s", out)
	}
}

func TestListCommand_Limit(t *testing.T) {
	ctx, _ := newMockedContext(t)
	ctx.JSON = true
	ctx.Limit = 0 // only one team returned by the fixture anyway; exercise the limit branch with 1

	root := newTestRoot()
	root.AddCommand(NewTeamCommand(ctx))
	root.SetArgs([]string{"team", "list", "--org", "testorg"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	ctx.Limit = 1
	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "team-a") {
		t.Errorf("expected team-a in limited output, got: %s", out)
	}
}

func TestListCommand_GraphQLError_DoesNotPanic(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})
	ctx := servicetest.NewMockContext(t, mockServer)

	root := newTestRoot()
	root.AddCommand(NewTeamCommand(ctx))
	root.SetArgs([]string{"team", "list", "--org", "testorg"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	// Run's error path only logs and returns; Execute itself should not
	// surface an error (there's no RunE here).
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requests := mockServer.GetRequests()
	if len(requests) == 0 {
		t.Fatal("expected the graphql call to have been attempted")
	}
}

func TestListCommand_ArgsValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "all without team name",
			args:    []string{"team", "list", "--org", "testorg", "--all"},
			wantErr: "team name is required when --all is used",
		},
		{
			name:    "too many members",
			args:    []string{"team", "list", "--org", "testorg", "--members", "101"},
			wantErr: "maximum number of members to list is 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := newMockedContext(t)

			root := newTestRoot()
			root.AddCommand(NewTeamCommand(ctx))
			root.SetArgs(tt.args)
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)

			err := root.Execute()
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestListCommand_DeprecatedAllMembersAlias(t *testing.T) {
	ctx, _ := newMockedContext(t)

	root := newTestRoot()
	root.AddCommand(NewTeamCommand(ctx))
	root.SetArgs([]string{"team", "list", "--org", "testorg", "--team", "team-a", "--all-members"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !allMembers {
		t.Error("expected the deprecated --all-members flag to set allMembers = true")
	}
	if teamName != "team-a" {
		t.Errorf("teamName = %q, want team-a", teamName)
	}
}

func TestListCommand_AllWithTeam_Success(t *testing.T) {
	ctx, _ := newMockedContext(t)

	root := newTestRoot()
	root.AddCommand(NewTeamCommand(ctx))
	root.SetArgs([]string{"team", "list", "--org", "testorg", "--team", "team-a", "--all"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !allMembers {
		t.Error("expected allMembers = true")
	}
	if noOfMembers != 50 {
		t.Errorf("noOfMembers = %d, want default 50", noOfMembers)
	}
}

func TestNewTeamCommand_HasListSubcommand(t *testing.T) {
	ctx, _ := newMockedContext(t)
	cmd := NewTeamCommand(ctx)

	found := false
	for _, c := range cmd.Commands() {
		if c.Name() == "list" {
			found = true
		}
	}
	if !found {
		t.Error("expected team command to have a list subcommand")
	}
}
