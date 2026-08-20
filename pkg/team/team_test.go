// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package team

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shurcooL/githubv4"

	"github.com/pradyb/sgh-cli/internal/model"
	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

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

func TestGetTeamAndMembers_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       teamGraphQLBody(),
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	teams, err := GetTeamAndMembers(ctx, TeamMembersRequest{OrgName: "testorg", NoOfMembers: 10})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(teams) != 1 {
		t.Fatalf("len(teams) = %d, want 1", len(teams))
	}
	got := teams[0]
	if got.Name != "team-a" {
		t.Errorf("Name = %q, want team-a", got.Name)
	}
	if got.TotalMembers != 2 {
		t.Errorf("TotalMembers = %d, want 2", got.TotalMembers)
	}
	if got.RepositoriesCount != 5 {
		t.Errorf("RepositoriesCount = %d, want 5", got.RepositoriesCount)
	}
	if len(got.Members) != 2 {
		t.Fatalf("len(Members) = %d, want 2", len(got.Members))
	}
	if got.Members[0].Login != "alice" || got.Members[1].Login != "bob" {
		t.Errorf("Members = %+v", got.Members)
	}
	wantPeopleURL := "https://github.com/orgs/testorg/people/alice"
	if got.Members[0].PeopleUrl != wantPeopleURL {
		t.Errorf("PeopleUrl = %q, want %q", got.Members[0].PeopleUrl, wantPeopleURL)
	}
}

func TestGetTeamAndMembers_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	teams, err := GetTeamAndMembers(ctx, TeamMembersRequest{OrgName: "testorg", NoOfMembers: 10})

	if err == nil {
		t.Fatal("expected an error")
	}
	if teams != nil {
		t.Errorf("expected nil teams on error, got %v", teams)
	}
}

func TestGetMembers(t *testing.T) {
	members := Members{
		TotalCount: 2,
		Edges: []struct {
			Node model.UserFragment
		}{
			{Node: model.UserFragment{Login: "alice", Name: "Alice", WebsiteUrl: "https://alice.example"}},
			{Node: model.UserFragment{Login: "bob", Name: ""}},
		},
	}

	got := getMembers(members, "https://github.com/orgs/testorg/people/")

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Login != "alice" || got[0].Name != "Alice" || got[0].Url != "https://alice.example" {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[0].PeopleUrl != "https://github.com/orgs/testorg/people/alice" {
		t.Errorf("PeopleUrl = %q", got[0].PeopleUrl)
	}
	if got[1].Login != "bob" || got[1].Name != "" {
		t.Errorf("got[1] = %+v", got[1])
	}
}

func TestGetMembers_Empty(t *testing.T) {
	got := getMembers(Members{}, "https://example.com/")
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}

// decodeGraphQLVariables reads the variables sent in a GraphQL POST body.
func decodeGraphQLVariables(r *http.Request) map[string]interface{} {
	var req struct {
		Query     string                 `json:"query"`
		Variables map[string]interface{} `json:"variables"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	return req.Variables
}

func teamNode(name string, membersHasNextPage bool, membersEndCursor string, members ...map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"node": map[string]interface{}{
			"name": name,
			"url":  "https://github.com/orgs/testorg/teams/" + name,
			"members": map[string]interface{}{
				"totalCount": len(members),
				"edges":      members,
				"pageInfo":   map[string]interface{}{"startCursor": "", "hasPreviousPage": false, "endCursor": membersEndCursor, "hasNextPage": membersHasNextPage},
			},
			"repositories": map[string]interface{}{"totalCount": 1},
		},
	}
}

func memberNode(login string) map[string]interface{} {
	return map[string]interface{}{"node": map[string]interface{}{"login": login, "name": "", "bio": "", "websiteUrl": ""}}
}

// TestGetTeamAndMembers_TeamPagination exercises the outer team-page pagination
// loop: the first response reports another page of teams, the second does not.
func TestGetTeamAndMembers_TeamPagination(t *testing.T) {
	graphqlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := decodeGraphQLVariables(r)
		teamCursor, _ := vars["teamCursor"].(string)

		w.Header().Set("Content-Type", "application/json")
		var teamsPage map[string]interface{}
		if teamCursor == "" {
			teamsPage = map[string]interface{}{
				"pageInfo": map[string]interface{}{"startCursor": "", "hasPreviousPage": false, "endCursor": "team-cursor-1", "hasNextPage": true},
				"edges":    []map[string]interface{}{teamNode("team-a", false, "")},
			}
		} else {
			teamsPage = map[string]interface{}{
				"pageInfo": map[string]interface{}{"startCursor": "", "hasPreviousPage": false, "endCursor": "", "hasNextPage": false},
				"edges":    []map[string]interface{}{teamNode("team-b", false, "")},
			}
		}
		body := map[string]interface{}{
			"data": map[string]interface{}{
				"organization": map[string]interface{}{
					"name":  "testorg",
					"url":   "https://github.com/testorg",
					"teams": teamsPage,
				},
			},
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer graphqlServer.Close()

	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.GraphqlClient.Client = githubv4.NewEnterpriseClient(graphqlServer.URL+"/graphql", &http.Client{Transport: ctx.HttpClient.Client.Transport})

	teams, err := GetTeamAndMembers(ctx, TeamMembersRequest{OrgName: "testorg", NoOfMembers: 10})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(teams) != 2 {
		t.Fatalf("len(teams) = %d, want 2, got %+v", len(teams), teams)
	}
	if teams[0].Name != "team-a" || teams[1].Name != "team-b" {
		t.Errorf("teams = %+v, want team-a then team-b", teams)
	}
}

// TestGetTeamAndMembers_MemberPagination exercises the member-cursor pagination
// loop within a single team: the first response reports another page of
// members for the same team, the second does not.
func TestGetTeamAndMembers_MemberPagination(t *testing.T) {
	graphqlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := decodeGraphQLVariables(r)
		memberCursor, _ := vars["memberCursor"].(string)

		w.Header().Set("Content-Type", "application/json")
		var teamsPage map[string]interface{}
		if memberCursor == "" {
			teamsPage = map[string]interface{}{
				"pageInfo": map[string]interface{}{"startCursor": "", "hasPreviousPage": false, "endCursor": "", "hasNextPage": false},
				"edges":    []map[string]interface{}{teamNode("team-a", true, "member-cursor-1", memberNode("alice"))},
			}
		} else {
			teamsPage = map[string]interface{}{
				"pageInfo": map[string]interface{}{"startCursor": "", "hasPreviousPage": false, "endCursor": "", "hasNextPage": false},
				"edges":    []map[string]interface{}{teamNode("team-a", false, "", memberNode("bob"))},
			}
		}
		body := map[string]interface{}{
			"data": map[string]interface{}{
				"organization": map[string]interface{}{
					"name":  "testorg",
					"url":   "https://github.com/testorg",
					"teams": teamsPage,
				},
			},
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer graphqlServer.Close()

	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.GraphqlClient.Client = githubv4.NewEnterpriseClient(graphqlServer.URL+"/graphql", &http.Client{Transport: ctx.HttpClient.Client.Transport})

	teams, err := GetTeamAndMembers(ctx, TeamMembersRequest{OrgName: "testorg", NoOfMembers: 10, AllMembers: true})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(teams) != 1 {
		t.Fatalf("len(teams) = %d, want 1, got %+v", len(teams), teams)
	}
	if teams[0].Name != "team-a" {
		t.Errorf("Name = %q, want team-a", teams[0].Name)
	}
	if len(teams[0].Members) != 2 {
		t.Fatalf("len(Members) = %d, want 2 (accumulated across pages), got %+v", len(teams[0].Members), teams[0].Members)
	}
	if teams[0].Members[0].Login != "alice" || teams[0].Members[1].Login != "bob" {
		t.Errorf("Members = %+v, want alice then bob", teams[0].Members)
	}
}
