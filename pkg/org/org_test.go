// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"
	"testing"

	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

func TestListOrgs(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
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
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	orgs := ListOrgs(ctx)

	if ctx.HasError {
		t.Fatal("expected HasError to remain false")
	}
	if len(orgs) != 1 {
		t.Fatalf("len(orgs) = %d, want 1", len(orgs))
	}
	got := orgs[0]
	if got.Login != "testorg" {
		t.Errorf("Login = %q, want %q", got.Login, "testorg")
	}
	if got.MembersCount != 5 {
		t.Errorf("MembersCount = %d, want 5", got.MembersCount)
	}
	if got.PrivateReposCount != 3 || got.PublicReposCount != 7 || got.ReposCount != 10 {
		t.Errorf("repo counts = %+v, want private=3 public=7 total=10", got)
	}
}

func TestListOrgs_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusInternalServerError,
		Body:       map[string]interface{}{"message": "boom"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	orgs := ListOrgs(ctx)

	if orgs != nil {
		t.Errorf("expected nil orgs on error, got %v", orgs)
	}
	if !ctx.HasError {
		t.Error("expected HasError to be set on failure")
	}
}
