// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package pr

import (
	"net/http"
	"testing"

	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

func TestListPullRequests_GraphQL_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"data": map[string]interface{}{
				"search": map[string]interface{}{
					"issueCount": 1,
					"pageInfo":   map[string]interface{}{"endCursor": "", "hasNextPage": false},
					"edges": []map[string]interface{}{
						{
							"node": map[string]interface{}{
								"number":           42,
								"title":            "GraphQL PR",
								"url":              "https://github.com/testorg/test-repo/pull/42",
								"body":             "description",
								"baseRef":          map[string]interface{}{"name": "main", "repository": map[string]interface{}{"name": "test-repo"}},
								"headRef":          map[string]interface{}{"name": "feature", "repository": map[string]interface{}{"name": "test-repo"}},
								"state":            "OPEN",
								"mergeStateStatus": "CLEAN",
								"author":           map[string]interface{}{"login": "jdoe", "name": "J Doe"},
								"reviewRequests": map[string]interface{}{
									"totalCount": 1,
									"edges": []map[string]interface{}{
										{"node": map[string]interface{}{"requestedReviewer": map[string]interface{}{"__typename": "User", "login": "reviewer1", "name": "Reviewer One"}}},
									},
								},
								"assignees": map[string]interface{}{
									"totalCount": 1,
									"edges": []map[string]interface{}{
										{"node": map[string]interface{}{"login": "assignee1", "name": "Assignee One"}},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	ctx := service.NewMockContext(t, mockServer)

	responses := ListPullRequests(ctx, PRRequest{
		OrgName:   "testorg",
		LastCount: 10,
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	pr := responses[0]
	if pr.PRNumber != 42 || pr.TitleName != "GraphQL PR" {
		t.Errorf("unexpected PR: %+v", pr)
	}
	if pr.Base.Ref != "main" || pr.Base.Repo.Name != "test-repo" {
		t.Errorf("unexpected base branch: %+v", pr.Base)
	}
	if pr.Head.Ref != "feature" || pr.Head.Repo.Name != "test-repo" {
		t.Errorf("unexpected head branch: %+v", pr.Head)
	}
	if pr.Author.Login != "jdoe" || pr.Author.Name != "J Doe" {
		t.Errorf("unexpected author: %+v", pr.Author)
	}
	if len(pr.Assignees) != 1 || pr.Assignees[0].Login != "assignee1" {
		t.Errorf("unexpected assignees: %+v", pr.Assignees)
	}
	if len(pr.Reviewers) != 1 || pr.Reviewers[0].Type != "User" || pr.Reviewers[0].User.Login != "reviewer1" {
		t.Errorf("unexpected reviewers: %+v", pr.Reviewers)
	}
}

func TestListPullRequests_GraphQL_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "rate limited"},
	})

	ctx := service.NewMockContext(t, mockServer)

	responses := ListPullRequests(ctx, PRRequest{OrgName: "testorg"})

	if len(responses) != 0 {
		t.Errorf("expected no responses on error, got %+v", responses)
	}
}

func TestGetPRDetailsGraphQL_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"data": map[string]interface{}{
				"organization": map[string]interface{}{
					"repository": map[string]interface{}{
						"pullRequest": map[string]interface{}{
							"number":           100,
							"title":            "Add feature",
							"body":             "desc",
							"url":              "https://github.com/testorg/test-repo/pull/100",
							"baseRef":          map[string]interface{}{"name": "main", "repository": map[string]interface{}{"name": "test-repo"}},
							"headRef":          map[string]interface{}{"name": "feature", "repository": map[string]interface{}{"name": "test-repo"}},
							"headRefOid":       "abc123",
							"reviewDecision":   "APPROVED",
							"state":            "OPEN",
							"mergeable":        "MERGEABLE",
							"mergeStateStatus": "CLEAN",
							"createdAt":        "2024-01-01T00:00:00Z",
							"updatedAt":        "2024-01-02T00:00:00Z",
							"mergedAt":         "",
							"mergedBy":         map[string]interface{}{"login": "merger1", "name": "Merger One"},
							"author":           map[string]interface{}{"login": "author1", "name": "Author One"},
							"reviewRequests": map[string]interface{}{
								"totalCount": 2,
								"edges": []map[string]interface{}{
									{"node": map[string]interface{}{"requestedReviewer": map[string]interface{}{"__typename": "User", "login": "reviewer1", "name": "Reviewer One"}}},
									{"node": map[string]interface{}{"requestedReviewer": map[string]interface{}{"__typename": "Team", "name": "core-team"}}},
								},
							},
							"assignees": map[string]interface{}{
								"totalCount": 1,
								"edges": []map[string]interface{}{
									{"node": map[string]interface{}{"login": "assignee1", "name": "Assignee One"}},
								},
							},
							"labels": map[string]interface{}{
								"totalCount": 2,
								"edges": []map[string]interface{}{
									{"node": map[string]interface{}{"name": "bug", "color": "ff0000"}},
									{"node": map[string]interface{}{"name": "priority", "color": "00ff00"}},
								},
							},
							"totalCommentsCount": 3,
							"comments":           map[string]interface{}{"totalCount": 3},
							"commits": map[string]interface{}{
								"totalCount": 1,
								"edges": []map[string]interface{}{
									{
										"node": map[string]interface{}{
											"commit": map[string]interface{}{
												"checkSuites": map[string]interface{}{
													"totalCount": 1,
													"edges": []map[string]interface{}{
														{
															"node": map[string]interface{}{
																"conclusion": "SUCCESS",
																"checkRuns": map[string]interface{}{
																	"totalCount": 1,
																	"edges": []map[string]interface{}{
																		{
																			"node": map[string]interface{}{
																				"status":      "COMPLETED",
																				"conclusion":  "SUCCESS",
																				"startedAt":   "2024-01-01T00:00:00Z",
																				"completedAt": "2024-01-01T00:05:00Z",
																				"detailsUrl":  "https://ci.example.com/1",
																				"name":        "build",
																				"text":        "all good",
																				"summary":     "build summary",
																				"title":       "build title",
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
							"additions":    10,
							"deletions":    5,
							"changedFiles": 2,
							"files": map[string]interface{}{
								"totalCount": 1,
								"edges": []map[string]interface{}{
									{"node": map[string]interface{}{"path": "main.go", "additions": 10, "deletions": 5, "changeType": "MODIFIED"}},
								},
							},
							"reviews": map[string]interface{}{
								"totalCount": 1,
								"edges": []map[string]interface{}{
									{
										"node": map[string]interface{}{
											"author":      map[string]interface{}{"login": "reviewer1", "name": "Reviewer One"},
											"state":       "APPROVED",
											"body":        "lgtm",
											"createdAt":   "2024-01-01T01:00:00Z",
											"submittedAt": "2024-01-01T01:05:00Z",
											"commit":      map[string]interface{}{"oid": "def456", "id": "node-id"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	ctx := service.NewMockContext(t, mockServer)

	prResp, filesResp, checkResp, reviews := GetPRDetailsGraphQL(ctx, PRDetailsRequest{
		OrgName:  "testorg",
		RepoName: "test-repo",
		PRNumber: 100,
	})

	if prResp.ErrorMessage != "" {
		t.Fatalf("unexpected error: %s", prResp.ErrorMessage)
	}
	if prResp.PRNumber != 100 || prResp.TitleName != "Add feature" {
		t.Errorf("unexpected PR: %+v", prResp)
	}
	if prResp.Head.Sha != "abc123" {
		t.Errorf("Head.Sha = %q, want %q", prResp.Head.Sha, "abc123")
	}
	if prResp.MergedBy.Login != "merger1" {
		t.Errorf("MergedBy.Login = %q, want %q", prResp.MergedBy.Login, "merger1")
	}
	if prResp.ReviewComments != 3 || prResp.Comments != 3 || prResp.Commits != 1 {
		t.Errorf("unexpected counts: %+v", prResp)
	}
	if prResp.Additions != 10 || prResp.Deletions != 5 || prResp.ChangedFiles != 2 {
		t.Errorf("unexpected diff stats: %+v", prResp)
	}
	if len(prResp.Labels) != 2 || prResp.Labels[0] != "bug" || prResp.Labels[1] != "priority" {
		t.Errorf("unexpected labels: %+v", prResp.Labels)
	}
	if len(prResp.Assignees) != 1 || prResp.Assignees[0].Login != "assignee1" {
		t.Errorf("unexpected assignees: %+v", prResp.Assignees)
	}
	if len(prResp.Reviewers) != 2 {
		t.Fatalf("len(Reviewers) = %d, want 2", len(prResp.Reviewers))
	}
	if prResp.Reviewers[0].Type != "User" || prResp.Reviewers[0].User.Login != "reviewer1" {
		t.Errorf("unexpected user reviewer: %+v", prResp.Reviewers[0])
	}
	if prResp.Reviewers[1].Type != "Team" || prResp.Reviewers[1].Team.Name != "core-team" {
		t.Errorf("unexpected team reviewer: %+v", prResp.Reviewers[1])
	}

	if len(filesResp.Files) != 1 || filesResp.Files[0].Filename != "main.go" {
		t.Errorf("unexpected files: %+v", filesResp.Files)
	}

	if checkResp.OverallConclusion != "SUCCESS" || len(checkResp.CheckRuns) != 1 {
		t.Errorf("unexpected check runs: %+v", checkResp)
	}
	if checkResp.CheckRuns[0].Name != "build" || checkResp.CheckRuns[0].Output.Summary != "build summary" {
		t.Errorf("unexpected check run detail: %+v", checkResp.CheckRuns[0])
	}

	if len(reviews) != 1 || reviews[0].User.Login != "reviewer1" || reviews[0].CommitID != "def456" {
		t.Errorf("unexpected reviews: %+v", reviews)
	}
}

func TestGetPRDetailsGraphQL_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "forbidden"},
	})

	ctx := service.NewMockContext(t, mockServer)

	prResp, filesResp, checkResp, reviews := GetPRDetailsGraphQL(ctx, PRDetailsRequest{
		OrgName:  "testorg",
		RepoName: "test-repo",
		PRNumber: 100,
	})

	if prResp.ErrorMessage == "" {
		t.Error("expected ErrorMessage to be set")
	}
	if len(filesResp.Files) != 0 {
		t.Errorf("expected no files on error, got %+v", filesResp)
	}
	if checkResp.OverallConclusion != "" {
		t.Errorf("expected empty check run response on error, got %+v", checkResp)
	}
	if reviews != nil {
		t.Errorf("expected nil reviews on error, got %+v", reviews)
	}
}
