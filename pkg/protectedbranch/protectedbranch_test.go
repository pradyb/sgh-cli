// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package protectedbranch

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	internalconfig "github.com/pradyb/sgh-cli/internal/config"
	"github.com/pradyb/sgh-cli/internal/model"
	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
	"github.com/pradyb/sgh-cli/pkg/context"
)

// setProtectedBranchConfig registers orgName in ctx.Config (if not already
// present) and overwrites its ProtectedBranch settings, forcing a rebuild of
// the config's internal lookup so subsequent reads see the change.
func setProtectedBranchConfig(t *testing.T, ctx *context.Context, orgName string, pb internalconfig.ProtectedBranch) {
	t.Helper()
	ctx.Config.AddOrganization(orgName)
	for i := range ctx.Config.Organizations {
		if strings.EqualFold(ctx.Config.Organizations[i].Name, orgName) {
			ctx.Config.Organizations[i].ProtectedBranch = pb
		}
	}
	// SetOwnerType always rebuilds the config's internal org lookup as a
	// side effect, which is otherwise unexported and unreachable from here.
	ctx.Config.SetOwnerType(orgName, "Organization")
}

// newProtectedBranchTestContext builds a *context.Context backed only by an
// in-memory config (no HTTP/GraphQL clients), suitable for exercising pure
// payload-building logic that only reads ctx.Config.
func newProtectedBranchTestContext(t *testing.T, orgName string, pb internalconfig.ProtectedBranch) *context.Context {
	t.Helper()
	ctx := &context.Context{Config: &internalconfig.Config{}}
	setProtectedBranchConfig(t, ctx, orgName, pb)
	return ctx
}

func mustUnmarshalSearchQuery(t *testing.T, raw string) model.SearchProtectedBranchesQuery {
	t.Helper()
	var q model.SearchProtectedBranchesQuery
	if err := json.Unmarshal([]byte(raw), &q); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}
	return q
}

// emptySearchGraphQLBody is a /graphql response shaped like
// model.SearchProtectedBranchesQuery with no results. The mock server's
// unconfigured default GraphQL body doesn't match that query shape at all
// (it has no "search" field), which the githubv4 decoder treats as an
// error; tests that don't care about pre-existing protected-branch state
// use this instead so ListProtectedBranches's internal lookup cleanly
// returns no results.
var emptySearchGraphQLBody = map[string]interface{}{
	"data": map[string]interface{}{
		"search": map[string]interface{}{
			"repositoryCount": 0,
			"pageInfo": map[string]interface{}{
				"endCursor":   "",
				"hasNextPage": false,
			},
			"edges": []map[string]interface{}{},
		},
	},
}

// ---------------------------------------------------------------------------
// Pure logic: RemoveItem
// ---------------------------------------------------------------------------

func TestRemoveItem(t *testing.T) {
	tests := []struct {
		name string
		list []string
		item string
		want []string
	}{
		{name: "removes matching item", list: []string{"a", "b", "c"}, item: "b", want: []string{"a", "c"}},
		{name: "removes all duplicate matches", list: []string{"a", "b", "b", "c"}, item: "b", want: []string{"a", "c"}},
		{name: "no match leaves list unchanged", list: []string{"a", "b"}, item: "z", want: []string{"a", "b"}},
		{name: "empty list", list: nil, item: "a", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RemoveItem(tt.list, tt.item)
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("RemoveItem(%v, %q) = %v, want %v", tt.list, tt.item, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Pure logic: getQueryString
// ---------------------------------------------------------------------------

func TestGetQueryString(t *testing.T) {
	t.Run("org not present, no repo name", func(t *testing.T) {
		ctx := &context.Context{Config: &internalconfig.Config{}}
		if got := getQueryString(ctx, "acme", ""); got != "org:acme" {
			t.Errorf("got = %q, want %q", got, "org:acme")
		}
	})

	t.Run("org present, no include patterns", func(t *testing.T) {
		ctx := &context.Context{Config: &internalconfig.Config{}}
		ctx.Config.AddOrganization("acme")
		if got := getQueryString(ctx, "acme", ""); got != "org:acme" {
			t.Errorf("got = %q, want %q", got, "org:acme")
		}
	})

	t.Run("org present, single include pattern", func(t *testing.T) {
		ctx := &context.Context{Config: &internalconfig.Config{}}
		ctx.Config.AddOrganization("acme")
		ctx.Config.AddRepositoryPattern("acme", true, false, "widget-*")
		want := "org:acme widget- in:name"
		if got := getQueryString(ctx, "acme", ""); got != want {
			t.Errorf("got = %q, want %q", got, want)
		}
	})

	t.Run("org present, multiple include patterns", func(t *testing.T) {
		ctx := &context.Context{Config: &internalconfig.Config{}}
		ctx.Config.AddOrganization("acme")
		ctx.Config.AddRepositoryPattern("acme", true, false, "widget-*")
		ctx.Config.AddRepositoryPattern("acme", true, false, "gadget-*")
		if got := getQueryString(ctx, "acme", ""); got != "org:acme" {
			t.Errorf("got = %q, want %q", got, "org:acme")
		}
	})

	t.Run("repo name given takes precedence", func(t *testing.T) {
		ctx := &context.Context{Config: &internalconfig.Config{}}
		want := "org:acme myrepo in:name"
		if got := getQueryString(ctx, "acme", "myrepo"); got != want {
			t.Errorf("got = %q, want %q", got, want)
		}
	})
}

// ---------------------------------------------------------------------------
// Pure logic: getSelectedBranchRef
// ---------------------------------------------------------------------------

func TestGetSelectedBranchRef(t *testing.T) {
	const multiRefFixture = `{"Name":"repo","Refs":{"TotalCount":2,"Edges":[{"Node":{"Name":"main"}},{"Node":{"Name":"develop"}}]}}`
	var multiRef model.ProtectedBranchRepoFragment
	if err := json.Unmarshal([]byte(multiRefFixture), &multiRef); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}

	if got := getSelectedBranchRef(multiRef, "develop"); got.Name != "develop" {
		t.Errorf("Name = %q, want %q", got.Name, "develop")
	}
	if got := getSelectedBranchRef(multiRef, "missing"); got.Name != "main" {
		t.Errorf("fallback Name = %q, want %q (first ref)", got.Name, "main")
	}

	const singleRefFixture = `{"Name":"repo","Refs":{"TotalCount":1,"Edges":[{"Node":{"Name":"only-branch"}}]}}`
	var singleRef model.ProtectedBranchRepoFragment
	if err := json.Unmarshal([]byte(singleRefFixture), &singleRef); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}

	if got := getSelectedBranchRef(singleRef, "irrelevant"); got.Name != "only-branch" {
		t.Errorf("Name = %q, want %q (single ref is always selected)", got.Name, "only-branch")
	}
}

// ---------------------------------------------------------------------------
// Pure logic: transformToProtectedBranch and its helpers
// ---------------------------------------------------------------------------

const repoWithProtectionFixture = `{
  "Search": {
    "Edges": [
      {
        "Node": {
          "Repository": {
            "Name": "repo-with-protection",
            "Refs": {
              "TotalCount": 2,
              "Edges": [
                {
                  "Node": {
                    "Name": "main",
                    "Rules": {"TotalCount": 0, "Edges": []},
                    "BranchProtectionRule": {
                      "Pattern": "main",
                      "LockBranch": true,
                      "IsAdminEnforced": true,
                      "RequiresConversationResolution": true,
                      "DismissesStaleReviews": true,
                      "RequiresCodeOwnerReviews": false,
                      "RequireLastPushApproval": true,
                      "RequiredApprovingReviewCount": 2,
                      "RequiredStatusChecks": [{"Context": "ci/test"}],
                      "PushAllowances": {"Edges": [{"Node": {"Actor": {"User": {"Login": "push-user", "Name": "Push User"}}}}]},
                      "BypassPullRequestAllowances": {"Edges": [{"Node": {"Actor": {"User": {"Login": "admin1", "Name": "Admin One"}}}}]}
                    }
                  }
                },
                {
                  "Node": {
                    "Name": "develop",
                    "Rules": {
                      "TotalCount": 1,
                      "Edges": [
                        {
                          "Node": {
                            "Type": "REQUIRED_STATUS_CHECKS",
                            "Parameters": {"RequiredStatusChecksParam": {"RequiredStatusChecks": [{"Context": "ci/lint"}]}},
                            "RepositoryRuleset": {
                              "Name": "ruleset-1",
                              "BypassActors": {"Edges": [{"Node": {"Actor": {"Team": {"Name": "team-a"}}}}]}
                            }
                          }
                        }
                      ]
                    }
                  }
                }
              ]
            }
          }
        }
      }
    ]
  }
}`

func TestTransformToProtectedBranch_AllBranches(t *testing.T) {
	q := mustUnmarshalSearchQuery(t, repoWithProtectionFixture)

	responses, selected := transformToProtectedBranch(q, "")

	if len(responses) != 2 {
		t.Fatalf("len(responses) = %d, want 2: %+v", len(responses), responses)
	}
	if responses[0].Type != "Branch Protection" || responses[0].Name != "main" {
		t.Errorf("responses[0] = %+v", responses[0])
	}
	if responses[1].Type != "Repository Rule" || responses[1].Name != "develop" {
		t.Errorf("responses[1] = %+v", responses[1])
	}
	if selected != "" {
		t.Errorf("selected = %q, want empty (only set when branchName is given)", selected)
	}
}

func TestTransformToProtectedBranch_SpecificBranch(t *testing.T) {
	q := mustUnmarshalSearchQuery(t, repoWithProtectionFixture)

	responses, selected := transformToProtectedBranch(q, "main")

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1: %+v", len(responses), responses)
	}
	if selected != "main" {
		t.Errorf("selected = %q, want %q", selected, "main")
	}
	got := responses[0]
	if got.RepositoryName != "repo-with-protection" || got.Type != "Branch Protection" || got.Name != "main" {
		t.Fatalf("got = %+v", got)
	}
	if !got.LockBranch || !got.EnforceAdmins || !got.RequiredConversationResolution {
		t.Errorf("bool flags not propagated: %+v", got)
	}
	if got.RequiredPullRequestReviews.RequiredApprovingReviewCount != 2 {
		t.Errorf("RequiredApprovingReviewCount = %d, want 2", got.RequiredPullRequestReviews.RequiredApprovingReviewCount)
	}
	if !got.RequiredPullRequestReviews.DismissStaleReviews || !got.RequiredPullRequestReviews.RequireLastPushApproval {
		t.Errorf("RequiredPullRequestReviews = %+v", got.RequiredPullRequestReviews)
	}
	if strings.Join(got.RequiredStatusChecks.Contexts, ",") != "ci/test" {
		t.Errorf("Contexts = %v, want [ci/test]", got.RequiredStatusChecks.Contexts)
	}
	if len(got.Restrictions.Users) != 1 || got.Restrictions.Users[0].Login != "push-user" {
		t.Errorf("Restrictions.Users = %+v", got.Restrictions.Users)
	}
	if len(got.RequiredPullRequestReviews.BypassPullRequestAllowances.Users) != 1 ||
		got.RequiredPullRequestReviews.BypassPullRequestAllowances.Users[0].Login != "admin1" {
		t.Errorf("BypassPullRequestAllowances.Users = %+v", got.RequiredPullRequestReviews.BypassPullRequestAllowances.Users)
	}
}

func TestTransformToProtectedBranch_SpecificBranch_Ruleset(t *testing.T) {
	q := mustUnmarshalSearchQuery(t, repoWithProtectionFixture)

	responses, selected := transformToProtectedBranch(q, "develop")

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1: %+v", len(responses), responses)
	}
	if selected != "develop" {
		t.Errorf("selected = %q, want %q", selected, "develop")
	}
	got := responses[0]
	if got.Type != "Repository Rule" || got.Name != "develop" {
		t.Fatalf("got = %+v", got)
	}
	if strings.Join(got.RepositoryRulesetNames, ",") != "ruleset-1" {
		t.Errorf("RepositoryRulesetNames = %v", got.RepositoryRulesetNames)
	}
	if strings.Join(got.RequiredStatusChecks.Contexts, ",") != "ci/lint" {
		t.Errorf("Contexts = %v, want [ci/lint]", got.RequiredStatusChecks.Contexts)
	}
}

func TestTransformToProtectedBranch_FallbackWhenBranchNotFound(t *testing.T) {
	q := mustUnmarshalSearchQuery(t, repoWithProtectionFixture)

	responses, selected := transformToProtectedBranch(q, "missing-branch")

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1: %+v", len(responses), responses)
	}
	if responses[0].Name != "main" {
		t.Errorf("Name = %q, want %q (falls back to first ref)", responses[0].Name, "main")
	}
	if selected != "main" {
		t.Errorf("selected = %q, want %q", selected, "main")
	}
}

func TestTransformToProtectedBranch_NoProtection_ReturnsNA(t *testing.T) {
	const fixture = `{
      "Search": {
        "Edges": [
          {"Node": {"Repository": {"Name": "repo-plain", "Refs": {"TotalCount": 1, "Edges": [{"Node": {"Name": "feature"}}]}}}}
        ]
      }
    }`
	q := mustUnmarshalSearchQuery(t, fixture)

	responses, _ := transformToProtectedBranch(q, "feature")

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1: %+v", len(responses), responses)
	}
	if responses[0].Type != "NA" || responses[0].RepositoryName != "repo-plain" {
		t.Errorf("responses[0] = %+v", responses[0])
	}
}

func TestTransformToProtectedBranch_SkipsReposWithNoRefs(t *testing.T) {
	const fixture = `{
      "Search": {
        "Edges": [
          {"Node": {"Repository": {"Name": "repo-empty", "Refs": {"TotalCount": 0, "Edges": []}}}}
        ]
      }
    }`
	q := mustUnmarshalSearchQuery(t, fixture)

	if responses, _ := transformToProtectedBranch(q, ""); len(responses) != 0 {
		t.Errorf("expected no responses for a repo with no refs, got %+v", responses)
	}
	if responses, _ := transformToProtectedBranch(q, "main"); len(responses) != 0 {
		t.Errorf("expected no responses for a repo with no refs, got %+v", responses)
	}
}

// ---------------------------------------------------------------------------
// Pure logic: transformRuleSetToProtectedBranch and rule-parameter helpers
// ---------------------------------------------------------------------------

const ruleSetFixture = `{
  "Name": "develop",
  "Rules": {
    "TotalCount": 2,
    "Edges": [
      {
        "Node": {
          "Type": "REQUIRED_STATUS_CHECKS",
          "Parameters": {"RequiredStatusChecksParam": {"RequiredStatusChecks": [{"Context":"ci/lint"},{"Context":"ci/test"}]}},
          "RepositoryRuleset": {
            "Name": "ruleset-1",
            "BypassActors": {"Edges": [{"Node": {"Actor": {"Team": {"Name": "team-a"}}}}]}
          }
        }
      },
      {
        "Node": {
          "Type": "PULL_REQUEST",
          "Parameters": {"PullRequestParam": {
            "RequiredApprovingReviewCount": 3,
            "DismissStaleReviewsOnPush": true,
            "RequireCodeOwnerReview": true,
            "RequireLastPushApproval": false,
            "RequiredReviewThreadResolution": true
          }},
          "RepositoryRuleset": {
            "Name": "ruleset-1",
            "BypassActors": {"Edges": [{"Node": {"Actor": {"Team": {"Name": "team-a"}}}}]}
          }
        }
      }
    ]
  }
}`

func TestTransformRuleSetToProtectedBranch(t *testing.T) {
	var node model.ProtectedBranchRefFragment
	if err := json.Unmarshal([]byte(ruleSetFixture), &node); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}

	got := transformRuleSetToProtectedBranch(node, "repo1")

	if got.RepositoryName != "repo1" || got.Type != "Repository Rule" || got.Name != "develop" {
		t.Fatalf("got = %+v", got)
	}
	if strings.Join(got.RepositoryRulesetNames, ",") != "ruleset-1" {
		t.Errorf("RepositoryRulesetNames = %v", got.RepositoryRulesetNames)
	}
	if strings.Join(got.RequiredStatusChecks.Contexts, ",") != "ci/lint,ci/test" {
		t.Errorf("Contexts = %v", got.RequiredStatusChecks.Contexts)
	}
	want := model.RequiredPullRequestReviews{
		DismissStaleReviews:            true,
		RequireCodeOwnerReviews:        true,
		RequireLastPushApproval:        false,
		RequiredApprovingReviewCount:   3,
		RequiredReviewThreadResolution: true,
	}
	if got.RequiredPullRequestReviews.DismissStaleReviews != want.DismissStaleReviews ||
		got.RequiredPullRequestReviews.RequireCodeOwnerReviews != want.RequireCodeOwnerReviews ||
		got.RequiredPullRequestReviews.RequireLastPushApproval != want.RequireLastPushApproval ||
		got.RequiredPullRequestReviews.RequiredApprovingReviewCount != want.RequiredApprovingReviewCount ||
		got.RequiredPullRequestReviews.RequiredReviewThreadResolution != want.RequiredReviewThreadResolution {
		t.Errorf("RequiredPullRequestReviews = %+v, want %+v", got.RequiredPullRequestReviews, want)
	}

	// Same ruleset name + same bypassing team appears in both rule edges;
	// the result should be deduplicated to a single bypass entry.
	if len(got.RequiredPullRequestReviews.BypassPullRequestAllowances.Users) != 1 {
		t.Fatalf("BypassPullRequestAllowances.Users = %+v, want 1 deduped entry", got.RequiredPullRequestReviews.BypassPullRequestAllowances.Users)
	}
	bypass := got.RequiredPullRequestReviews.BypassPullRequestAllowances.Users[0]
	if bypass.Login != "ruleset-1" || bypass.Name != "team-a" {
		t.Errorf("bypass user = %+v, want {Login:ruleset-1 Name:team-a}", bypass)
	}
}

func TestGetRuleCheckContexts(t *testing.T) {
	node := model.RuleParamStatusChecksParam{
		RequiredStatusChecks: []struct{ Context string }{{Context: "a"}, {Context: "b"}},
	}
	if got := getRuleCheckContexts(node); strings.Join(got, ",") != "a,b" {
		t.Errorf("got = %v, want [a b]", got)
	}
	if got := getRuleCheckContexts(model.RuleParamStatusChecksParam{}); len(got) != 0 {
		t.Errorf("got = %v, want empty", got)
	}
}

func TestGetRulePullRequestParams(t *testing.T) {
	node := model.RulePullRequestParam{
		RequiredApprovingReviewCount:   2,
		DismissStaleReviewsOnPush:      true,
		RequireCodeOwnerReview:         true,
		RequireLastPushApproval:        true,
		RequiredReviewThreadResolution: true,
	}
	want := model.RequiredPullRequestReviews{
		DismissStaleReviews:            true,
		RequireCodeOwnerReviews:        true,
		RequireLastPushApproval:        true,
		RequiredApprovingReviewCount:   2,
		RequiredReviewThreadResolution: true,
	}
	if got := getRulePullRequestParams(node); !reflect.DeepEqual(got, want) {
		t.Errorf("got = %+v, want %+v", got, want)
	}
}

func TestGetRuleParamerters(t *testing.T) {
	t.Run("required status checks populates contexts only", func(t *testing.T) {
		pb := &model.ProtectedBranch{}
		params := model.RuleParameters{RequiredStatusChecksParam: model.RuleParamStatusChecksParam{
			RequiredStatusChecks: []struct{ Context string }{{Context: "a"}},
		}}

		got := getRuleParamerters("REQUIRED_STATUS_CHECKS", params, nil, pb)

		if strings.Join(got, ",") != "a" {
			t.Errorf("got = %v, want [a]", got)
		}
		if pb.RequiredPullRequestReviews.RequiredApprovingReviewCount != 0 {
			t.Errorf("pb should be untouched for a status-check rule, got %+v", pb.RequiredPullRequestReviews)
		}
	})

	t.Run("pull request populates pb only, leaves contexts untouched", func(t *testing.T) {
		pb := &model.ProtectedBranch{}
		params := model.RuleParameters{PullRequestParam: model.RulePullRequestParam{RequiredApprovingReviewCount: 4}}
		existing := []string{"unchanged"}

		got := getRuleParamerters("PULL_REQUEST", params, existing, pb)

		if strings.Join(got, ",") != "unchanged" {
			t.Errorf("checkContexts should pass through unchanged, got %v", got)
		}
		if pb.RequiredPullRequestReviews.RequiredApprovingReviewCount != 4 {
			t.Errorf("RequiredApprovingReviewCount = %d, want 4", pb.RequiredPullRequestReviews.RequiredApprovingReviewCount)
		}
	})

	t.Run("unknown rule type is a no-op", func(t *testing.T) {
		pb := &model.ProtectedBranch{}
		existing := []string{"unchanged"}

		got := getRuleParamerters("SOMETHING_ELSE", model.RuleParameters{}, existing, pb)

		if strings.Join(got, ",") != "unchanged" {
			t.Errorf("got = %v, want [unchanged]", got)
		}
		if pb.RequiredPullRequestReviews.RequiredApprovingReviewCount != 0 {
			t.Errorf("pb should be untouched, got %+v", pb.RequiredPullRequestReviews)
		}
	})
}

// ---------------------------------------------------------------------------
// Pure logic: createRequestPayload (and its addStatusChecks / addUsersToBypassPullRequestReview / addRestrictions helpers)
// ---------------------------------------------------------------------------

func TestCreateRequestPayload(t *testing.T) {
	tests := []struct {
		name                     string
		orgProtectedBranch       internalconfig.ProtectedBranch
		request                  ProtectedBranchRequest
		existing                 model.ProtectedBranch
		repoName                 string
		wantChecks               []model.CheckRequest
		wantLockBranch           bool
		wantApprovingReviewCount int
		wantBypassUsers          []string
		wantRestrictionUsers     []string
	}{
		{
			name:       "new branch uses default Build check",
			request:    ProtectedBranchRequest{OrgName: "acme"},
			repoName:   "myrepo",
			wantChecks: []model.CheckRequest{{Context: "Build"}},
		},
		{
			name: "new branch uses configured checks, users and lock",
			orgProtectedBranch: internalconfig.ProtectedBranch{
				StatusChecks:             []internalconfig.StatusCheck{{Context: "ci/build", AppID: 123}, {Context: "ci/lint"}},
				BypassPullRequestUsers:   []string{"admin1"},
				AllowedRestrictionsUsers: []string{"deploy-bot"},
				ApprovingReviewCount:     2,
			},
			request:                  ProtectedBranchRequest{OrgName: "acme", Lock: true},
			repoName:                 "myrepo",
			wantChecks:               []model.CheckRequest{{Context: "ci/build", AppID: 123}, {Context: "ci/lint"}},
			wantLockBranch:           true,
			wantApprovingReviewCount: 2,
			wantBypassUsers:          []string{"admin1"},
			wantRestrictionUsers:     []string{"deploy-bot"},
		},
		{
			name: "RemoveStatus skips status checks",
			orgProtectedBranch: internalconfig.ProtectedBranch{
				StatusChecks: []internalconfig.StatusCheck{{Context: "ci/build"}},
			},
			request:    ProtectedBranchRequest{OrgName: "acme", RemoveStatus: true},
			repoName:   "myrepo",
			wantChecks: []model.CheckRequest{},
		},
		{
			name: "repo in ignore list skips status checks",
			orgProtectedBranch: internalconfig.ProtectedBranch{
				StatusChecks:                []internalconfig.StatusCheck{{Context: "ci/build"}},
				IgnoreBuildStatusCheckRepos: []string{"myrepo"},
			},
			request:    ProtectedBranchRequest{OrgName: "acme"},
			repoName:   "myrepo",
			wantChecks: []model.CheckRequest{},
		},
		{
			name: "existing branch merges add/remove and ignores config defaults",
			orgProtectedBranch: internalconfig.ProtectedBranch{
				BypassPullRequestUsers:   []string{"configured-user"},
				AllowedRestrictionsUsers: []string{"configured-deploy"},
			},
			request: ProtectedBranchRequest{
				OrgName:           "acme",
				AddBypassUsers:    []string{"new-admin"},
				RemoveBypassUsers: []string{"existing-admin"},
				AddPushUsers:      []string{"new-deploy"},
				RemovePushUsers:   []string{"existing-deploy"},
			},
			existing: model.ProtectedBranch{
				Name: "main",
				RequiredPullRequestReviews: model.RequiredPullRequestReviews{
					BypassPullRequestAllowances: model.UserTeam{Users: []model.User{{Login: "existing-admin"}}},
				},
				Restrictions: model.Restriction{Users: []model.User{{Login: "existing-deploy"}}},
			},
			repoName:             "myrepo",
			wantChecks:           []model.CheckRequest{{Context: "Build"}},
			wantBypassUsers:      []string{"new-admin"},
			wantRestrictionUsers: []string{"new-deploy"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newProtectedBranchTestContext(t, "acme", tt.orgProtectedBranch)

			got, err := createRequestPayload(ctx, tt.repoName, tt.request, tt.existing)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got.RequiredStatusChecks.Checks, tt.wantChecks) {
				t.Errorf("Checks = %+v, want %+v", got.RequiredStatusChecks.Checks, tt.wantChecks)
			}
			if got.LockBranch != tt.wantLockBranch {
				t.Errorf("LockBranch = %v, want %v", got.LockBranch, tt.wantLockBranch)
			}
			if got.RequiredPullRequestReviews.RequiredApprovingReviewCount != tt.wantApprovingReviewCount {
				t.Errorf("RequiredApprovingReviewCount = %d, want %d", got.RequiredPullRequestReviews.RequiredApprovingReviewCount, tt.wantApprovingReviewCount)
			}
			if strings.Join(got.RequiredPullRequestReviews.BypassPullRequestAllowances.Users, ",") != strings.Join(tt.wantBypassUsers, ",") {
				t.Errorf("BypassPullRequestAllowances.Users = %v, want %v", got.RequiredPullRequestReviews.BypassPullRequestAllowances.Users, tt.wantBypassUsers)
			}
			if strings.Join(got.Restrictions.Users, ",") != strings.Join(tt.wantRestrictionUsers, ",") {
				t.Errorf("Restrictions.Users = %v, want %v", got.Restrictions.Users, tt.wantRestrictionUsers)
			}

			// Baseline template defaults should always survive.
			if !got.EnforceAdmins || !got.RequiredConversationResolution || !got.RequiredStatusChecks.Strict {
				t.Errorf("expected template defaults to survive, got %+v", got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Service-backed: UpdateProtectedBranchForRepo
// ---------------------------------------------------------------------------

func TestUpdateProtectedBranchForRepo_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/myrepo/branches/main/protection", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"required_status_checks": map[string]interface{}{
				"strict":   true,
				"contexts": []string{"ci/test"},
			},
			"required_pull_request_reviews": map[string]interface{}{
				"dismiss_stale_reviews":           true,
				"require_code_owner_reviews":      false,
				"require_last_push_approval":      true,
				"required_approving_review_count": 2,
				"bypass_pull_request_allowances": map[string]interface{}{
					"users": []map[string]interface{}{{"login": "bypass-user"}},
				},
			},
			"restrictions": map[string]interface{}{
				"users": []map[string]interface{}{{"login": "restricted-user", "name": "Restricted User"}},
			},
		},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	resp, err := UpdateProtectedBranchForRepo(ctx, "myrepo", ProtectedBranchRequest{OrgName: "acme", BranchName: "main"}, model.ProtectedBranch{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.RepositoryName != "myrepo" || resp.Type != "Branch Protection" {
		t.Errorf("resp = %+v", resp)
	}
	if strings.Join(resp.RequiredStatusChecks.Contexts, ",") != "ci/test" {
		t.Errorf("Contexts = %v, want [ci/test]", resp.RequiredStatusChecks.Contexts)
	}
	if resp.RequiredPullRequestReviews.RequiredApprovingReviewCount != 2 {
		t.Errorf("RequiredApprovingReviewCount = %d, want 2", resp.RequiredPullRequestReviews.RequiredApprovingReviewCount)
	}
	if len(resp.RequiredPullRequestReviews.BypassPullRequestAllowances.Users) != 1 || resp.RequiredPullRequestReviews.BypassPullRequestAllowances.Users[0].Login != "bypass-user" {
		t.Errorf("BypassPullRequestAllowances.Users = %+v", resp.RequiredPullRequestReviews.BypassPullRequestAllowances.Users)
	}
	if len(resp.Restrictions.Users) != 1 || resp.Restrictions.Users[0].Login != "restricted-user" {
		t.Errorf("Restrictions.Users = %+v", resp.Restrictions.Users)
	}
}

func TestUpdateProtectedBranchForRepo_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/myrepo/branches/main/protection", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)

	resp, err := UpdateProtectedBranchForRepo(ctx, "myrepo", ProtectedBranchRequest{OrgName: "acme", BranchName: "main"}, model.ProtectedBranch{})
	if err == nil {
		t.Fatal("expected an error")
	}
	if resp.RepositoryName != "myrepo" {
		t.Errorf("RepositoryName = %q, want %q", resp.RepositoryName, "myrepo")
	}
}

// ---------------------------------------------------------------------------
// Service-backed: UpdateProtectedBranch (top-level, via processor)
// ---------------------------------------------------------------------------

func TestUpdateProtectedBranch_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	// The default PUT .../protection response from the mock server is used.
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       emptySearchGraphQLBody,
	})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := UpdateProtectedBranch(ctx, ProtectedBranchRequest{OrgName: "acme", RepoNames: []string{"myrepo"}, BranchName: "main"}, nil)

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1: %+v", len(responses), responses)
	}
	if responses[0].RepositoryName != "myrepo" || responses[0].ErrorMessage != "" {
		t.Errorf("responses[0] = %+v", responses[0])
	}
	if responses[0].Type != "Branch Protection" {
		t.Errorf("Type = %q, want %q", responses[0].Type, "Branch Protection")
	}
}

func TestUpdateProtectedBranch_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       emptySearchGraphQLBody,
	})
	mockServer.SetResponse("/repos/acme/myrepo/branches/main/protection", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := UpdateProtectedBranch(ctx, ProtectedBranchRequest{OrgName: "acme", RepoNames: []string{"myrepo"}, BranchName: "main"}, nil)

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1: %+v", len(responses), responses)
	}
	if responses[0].ErrorMessage == "" {
		t.Error("expected ErrorMessage to be set")
	}
	if !ctx.HasError {
		t.Error("expected ctx.HasError to be set")
	}
}

// TestUpdateProtectedBranch_ExistingBranch exercises the branch where the
// repo already has a protected branch: UpdateProtectedBranch first looks it
// up via ListProtectedBranches and threads it through to
// UpdateProtectedBranchForRepo as existingProtectedBranch, so that
// createRequestPayload preserves (rather than replaces) previously
// configured bypass/restriction users.
func TestUpdateProtectedBranch_ExistingBranch(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       branchProtectionGraphQLBody("myrepo", "main"),
	})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := UpdateProtectedBranch(ctx, ProtectedBranchRequest{OrgName: "acme", RepoNames: []string{"myrepo"}, BranchName: "main"}, nil)

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1: %+v", len(responses), responses)
	}
	if responses[0].RepositoryName != "myrepo" || responses[0].ErrorMessage != "" {
		t.Errorf("responses[0] = %+v", responses[0])
	}
}

// ---------------------------------------------------------------------------
// Service-backed: DeleteProtectedBranch
// ---------------------------------------------------------------------------

func TestDeleteProtectedBranch_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := DeleteProtectedBranch(ctx, "acme", []string{"myrepo"}, nil, "main")

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1: %+v", len(responses), responses)
	}
	if !responses[0].IsSuccess() {
		t.Errorf("expected success, got ErrorMessage=%q", responses[0].ErrorMessage)
	}
	if responses[0].RepositoryName != "myrepo" || responses[0].Ref != "main" || responses[0].Type != "DELETE_PROTECTED_BRANCH" {
		t.Errorf("responses[0] = %+v", responses[0])
	}
	if responses[0].SuccessMessage != "Protected Branch deleted" {
		t.Errorf("SuccessMessage = %q", responses[0].SuccessMessage)
	}
}

func TestDeleteProtectedBranch_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/acme/myrepo/branches/main/protection", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "not found"},
	})

	ctx := servicetest.NewMockContext(t, mockServer)
	ctx.Silent = true

	responses := DeleteProtectedBranch(ctx, "acme", []string{"myrepo"}, nil, "main")

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1: %+v", len(responses), responses)
	}
	if responses[0].IsSuccess() {
		t.Error("expected failure")
	}
	if !strings.Contains(responses[0].ErrorMessage, "failed to delete protected branch") {
		t.Errorf("ErrorMessage = %q", responses[0].ErrorMessage)
	}
}

// ---------------------------------------------------------------------------
// Service-backed: ListProtectedBranches
// ---------------------------------------------------------------------------

// branchProtectionGraphQLBody builds a GraphQL response body shaped exactly
// like model.SearchProtectedBranchesQuery expects to decode a single
// repository with one branch-protection-rule-protected branch. Inline
// fragments ("... on Repository", "... on User") are flattened directly into
// their parent object, matching githubv4's decoding behaviour.
func branchProtectionGraphQLBody(repoName, branchName string) map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"search": map[string]interface{}{
				"repositoryCount": 1,
				"pageInfo": map[string]interface{}{
					"endCursor":   "",
					"hasNextPage": false,
				},
				"edges": []map[string]interface{}{
					{
						"node": map[string]interface{}{
							"name": repoName,
							"refs": map[string]interface{}{
								"totalCount": 1,
								"edges": []map[string]interface{}{
									{
										"node": map[string]interface{}{
											"name": branchName,
											"rules": map[string]interface{}{
												"totalCount": 0,
												"edges":      []map[string]interface{}{},
											},
											"branchProtectionRule": map[string]interface{}{
												"pattern":                        branchName,
												"isAdminEnforced":                true,
												"requiresConversationResolution": true,
												"dismissesStaleReviews":          true,
												"requireLastPushApproval":        true,
												"requiredApprovingReviewCount":   2,
												"requiredStatusChecks": []map[string]interface{}{
													{"context": "ci/test"},
												},
												"pushAllowances": map[string]interface{}{
													"totalCount": 0,
													"edges":      []map[string]interface{}{},
												},
												"bypassPullRequestAllowances": map[string]interface{}{
													"totalCount": 1,
													"edges": []map[string]interface{}{
														{
															"node": map[string]interface{}{
																"actor": map[string]interface{}{
																	"login": "admin1",
																	"name":  "Admin One",
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
			},
		},
	}
}

func TestListProtectedBranches_GraphQL(t *testing.T) {
	t.Run("zero repos requested", func(t *testing.T) {
		mockServer := testutils.NewMockGitHubServer()
		defer mockServer.Close()
		mockServer.SetResponse("/graphql", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       branchProtectionGraphQLBody("myrepo", "main"),
		})
		ctx := servicetest.NewMockContext(t, mockServer)

		result := ListProtectedBranches(ctx, "acme", nil, nil, "main")

		if len(result) != 1 {
			t.Fatalf("len(result) = %d, want 1: %+v", len(result), result)
		}
		if result[0].RepositoryName != "myrepo" || result[0].Type != "Branch Protection" {
			t.Errorf("result[0] = %+v", result[0])
		}
		if result[0].RequiredPullRequestReviews.RequiredApprovingReviewCount != 2 {
			t.Errorf("RequiredApprovingReviewCount = %d, want 2", result[0].RequiredPullRequestReviews.RequiredApprovingReviewCount)
		}
	})

	t.Run("single repo requested", func(t *testing.T) {
		mockServer := testutils.NewMockGitHubServer()
		defer mockServer.Close()
		mockServer.SetResponse("/graphql", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       branchProtectionGraphQLBody("myrepo", "main"),
		})
		ctx := servicetest.NewMockContext(t, mockServer)

		result := ListProtectedBranches(ctx, "acme", []string{"myrepo"}, nil, "main")

		if len(result) != 1 {
			t.Fatalf("len(result) = %d, want 1: %+v", len(result), result)
		}
	})

	t.Run("query error returns no results", func(t *testing.T) {
		mockServer := testutils.NewMockGitHubServer()
		defer mockServer.Close()
		mockServer.SetResponse("/graphql", testutils.MockResponse{
			StatusCode: http.StatusForbidden,
			Body:       map[string]interface{}{"message": "forbidden"},
		})
		ctx := servicetest.NewMockContext(t, mockServer)

		result := ListProtectedBranches(ctx, "acme", nil, nil, "main")

		if len(result) != 0 {
			t.Errorf("expected empty result on query error, got %+v", result)
		}
	})
}

func TestListProtectedBranches_MultipleRepos(t *testing.T) {
	t.Run("success merges per-repo results", func(t *testing.T) {
		mockServer := testutils.NewMockGitHubServer()
		defer mockServer.Close()
		mockServer.SetResponse("/graphql", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       branchProtectionGraphQLBody("repo1", "main"),
		})
		ctx := servicetest.NewMockContext(t, mockServer)

		result := ListProtectedBranches(ctx, "acme", []string{"repo1", "repo2"}, nil, "main")

		if len(result) != 2 {
			t.Fatalf("len(result) = %d, want 2: %+v", len(result), result)
		}
	})

	t.Run("per-repo query error surfaces an error entry per repo", func(t *testing.T) {
		mockServer := testutils.NewMockGitHubServer()
		defer mockServer.Close()
		mockServer.SetResponse("/graphql", testutils.MockResponse{
			StatusCode: http.StatusForbidden,
			Body:       map[string]interface{}{"message": "forbidden"},
		})
		ctx := servicetest.NewMockContext(t, mockServer)

		result := ListProtectedBranches(ctx, "acme", []string{"repo1", "repo2"}, nil, "main")

		if len(result) != 2 {
			t.Fatalf("len(result) = %d, want 2: %+v", len(result), result)
		}
		for _, r := range result {
			if !strings.Contains(r.ErrorMessage, "failed to get protected branch details") {
				t.Errorf("expected error message on %+v", r)
			}
		}
	})

	t.Run("no protected branches found returns NA per repo", func(t *testing.T) {
		mockServer := testutils.NewMockGitHubServer()
		defer mockServer.Close()
		mockServer.SetResponse("/graphql", testutils.MockResponse{
			StatusCode: http.StatusOK,
			Body:       emptySearchGraphQLBody,
		})
		ctx := servicetest.NewMockContext(t, mockServer)

		result := ListProtectedBranches(ctx, "acme", []string{"repo1", "repo2"}, nil, "main")

		if len(result) != 2 {
			t.Fatalf("len(result) = %d, want 2: %+v", len(result), result)
		}
		for _, r := range result {
			if r.Type != "NA" || r.ErrorMessage != "" {
				t.Errorf("expected NA entry with no error, got %+v", r)
			}
		}
	})
}
