package protectedbranch

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/shurcooL/githubv4"
)

func ListProtectedBranches(ctx *context.Context, orgName string, repoNames []string, branchName string) []model.ProtectedBranch {
	responses := make([]model.ProtectedBranch, 0)

	if len(repoNames) <= 1 {
		// Invoke via GraphQL
		logger.Glog.Info().Msgf("Invoking GraphQL to list protected branches for %s", orgName)
		repoName := ""
		if len(repoNames) == 1 {
			actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, repoNames)
			logger.Glog.Info().Str("repos", strings.Join(actualRepoNames, ",")).Msgf("%s for selected repositories in %s", "Listing Protected Branch", orgName)
			repoName = actualRepoNames[0]
		}
		queryString := getQueryString(ctx, orgName, repoName)
		variables := map[string]interface{}{
			"queryString": githubv4.String(queryString),
			"branchName":  githubv4.String(branchName),
			"repoCursor":  (*githubv4.String)(nil),
		}

		var selectedBranchName string
		var branches []model.ProtectedBranch
		for {
			var searchProtectedBranchesQuery model.SearchProtectedBranchesQuery
			err := service.Query(ctx, &searchProtectedBranchesQuery, variables)
			if err != nil {
				logger.Glog.Error().Err(err).Msg("Error in listing protected branches")
				return responses
			}

			branches, selectedBranchName = transformToProtectedBranch(searchProtectedBranchesQuery, branchName)
			responses = append(responses, branches...)

			variables["repoCursor"] = githubv4.String(searchProtectedBranchesQuery.Search.PageInfo.EndCursor)
			logger.Flog.Info().Msgf("Next page details %t %s", searchProtectedBranchesQuery.Search.PageInfo.HasNextPage, searchProtectedBranchesQuery.Search.PageInfo.EndCursor)

			if !searchProtectedBranchesQuery.Search.PageInfo.HasNextPage {
				break
			}
		}
		if selectedBranchName != branchName {
			logger.Glog.Warn().Msgf("selecting %s branch", selectedBranchName)
		}

	} else {
		processor.ProcessRepositoriesOperation(ctx, orgName, repoNames, processor.OperationListProtectedBranch,
			func(ctx *context.Context, orgName, repoName string) (model.ProtectedBranch, error) {
				return getProtectedBranchDetails(ctx, orgName, repoName, branchName, githubv4.String("")), nil
			},
			func(repoName string, result processor.RepoOperationResult[model.ProtectedBranch]) {
				responses = append(responses, result.Result)
			},
			func(repoName string, err error) {
				responses = append(responses, model.ProtectedBranch{RepositoryName: repoName, ErrorMessage: err.Error()})
			})
	}
	return responses
}

func getQueryString(ctx *context.Context, orgName string, repoName string) string {
	var queryString string
	queryString = "org:" + orgName
	if repoName == "" {
		if ctx.Config.IsOrganizationPresent(orgName) {
			includes := ctx.Config.IncludePatterns(orgName)
			if (len(includes)) == 1 {
				queryString = queryString + " " + strings.ReplaceAll(includes[0], "*", "") + " in:name"
			}
		}
	} else {
		queryString = queryString + " " + repoName + " in:name"
	}
	return queryString
}

func getProtectedBranchDetails(ctx *context.Context, orgName, repoName, branchName string, repoCursor githubv4.String) model.ProtectedBranch {
	queryString := getQueryString(ctx, orgName, repoName)
	variables := map[string]interface{}{
		"queryString": githubv4.String(queryString),
		"branchName":  githubv4.String(branchName),
		"repoCursor":  repoCursor,
	}

	var searchProtectedBranchesQuery model.SearchProtectedBranchesQuery
	err := service.Query(ctx, &searchProtectedBranchesQuery, variables)
	if err != nil {
		logger.Glog.Error().Err(err).Msg("Error in listing protected branches")
		return model.ProtectedBranch{RepositoryName: repoName, ErrorMessage: err.Error()}
	}
	branches, _ := transformToProtectedBranch(searchProtectedBranchesQuery, branchName)
	if len(branches) > 0 {
		return branches[0]
	}
	return model.ProtectedBranch{RepositoryName: repoName, ErrorMessage: "No protected branch found"}
}

func transformToProtectedBranch(searchProtectedBranchesQuery model.SearchProtectedBranchesQuery, branchName string) ([]model.ProtectedBranch, string) {
	var selectedBranchName string
	responses := make([]model.ProtectedBranch, 0)
	for _, edge := range searchProtectedBranchesQuery.Search.Edges {
		if edge.Node.Repository.Refs.TotalCount != 0 {
			node := getSelectedBranchRef(edge.Node.Repository, branchName)
			selectedBranchName = node.Name

			if node.BranchProtectionRule.Pattern != "" {
				responses = append(responses, transformBranchProtectionRuleToProtectedBranch(node, edge.Node.Repository.Name))
			} else {
				responses = append(responses, transformRuleSetToProtectedBranch(node, edge.Node.Repository.Name))
			}
		}
	}
	return responses, selectedBranchName
}

func getSelectedBranchRef(repoFragment model.ProtectedBranchRepoFragment, branchName string) model.ProtectedBranchRefFragment {
	node := repoFragment.Refs.Edges[0].Node
	if repoFragment.Refs.TotalCount > 1 {
		for _, edge := range repoFragment.Refs.Edges {
			if edge.Node.Name == branchName {
				node = edge.Node
				break
			}
		}
	}
	return node
}

func transformBranchProtectionRuleToProtectedBranch(node model.ProtectedBranchRefFragment, repoName string) model.ProtectedBranch {
	checkContexts := make([]string, 0)
	restrictionsUsers := make([]model.User, 0)
	bypassPullRequestAllowances := make([]model.User, 0)

	for _, check := range node.BranchProtectionRule.RequiredStatusChecks {
		checkContexts = append(checkContexts, check.Context)
	}
	for _, edge := range node.BranchProtectionRule.PushAllowances.Edges {
		restrictionsUsers = append(restrictionsUsers, model.User{Login: edge.Node.Actor.User.Login, Name: edge.Node.Actor.User.Name})
	}
	for _, edge := range node.BranchProtectionRule.BypassPullRequestAllowances.Edges {
		bypassPullRequestAllowances = append(bypassPullRequestAllowances, model.User{Login: edge.Node.Actor.User.Login, Name: edge.Node.Actor.User.Name})
	}

	return model.ProtectedBranch{
		RepositoryName:                 repoName,
		Type:                           "Branch Protection",
		LockBranch:                     node.BranchProtectionRule.LockBranch,
		EnforceAdmins:                  node.BranchProtectionRule.IsAdminEnforced,
		RequiredConversationResolution: node.BranchProtectionRule.RequiresConversationResolution,
		RequiredPullRequestReviews: model.RequiredPullRequestReviews{
			DismissStaleReviews:          node.BranchProtectionRule.DismissesStaleReviews,
			RequireCodeOwnerReviews:      node.BranchProtectionRule.RequiresCodeOwnerReviews,
			RequireLastPushApproval:      node.BranchProtectionRule.RequireLastPushApproval,
			RequiredApprovingReviewCount: node.BranchProtectionRule.RequiredApprovingReviewCount,
			BypassPullRequestAllowances:  model.UserTeam{Users: bypassPullRequestAllowances},
		},
		RequiredStatusChecks: model.RequiredStatusChecks{Contexts: checkContexts},
		Restrictions: model.Restriction{
			Users: restrictionsUsers,
		},
	}
}

func transformRuleSetToProtectedBranch(node model.ProtectedBranchRefFragment, repoName string) model.ProtectedBranch {
	bypassPullRequestAllowances := make([]model.User, 0)
	checkContexts := make([]string, 0)
	rulesetNames := make([]string, 0)

	pb := model.ProtectedBranch{
		RepositoryName: repoName,
		Type:           "Repository Rule",
	}

	if len(node.Rules.Edges) > 0 {
		repositoryRulesetNames := map[string][]string{}
		for _, edge := range node.Rules.Edges {
			for _, actorEdge := range edge.Node.RepositoryRuleset.BypassActors.Edges {
				if !slices.Contains(repositoryRulesetNames[edge.Node.RepositoryRuleset.Name], actorEdge.Node.Actor.Team.Name) {
					repositoryRulesetNames[edge.Node.RepositoryRuleset.Name] = append(repositoryRulesetNames[edge.Node.RepositoryRuleset.Name], actorEdge.Node.Actor.Team.Name)
				}
			}
			checkContexts = getRuleParamerters(edge.Node.Type, edge.Node.Parameters, checkContexts, &pb)
		}

		for k, v := range repositoryRulesetNames {
			rulesetNames = append(rulesetNames, k)
			bypassPullRequestAllowances = append(bypassPullRequestAllowances, model.User{Login: k, Name: strings.Join(v, ",")})
		}

	}
	pb.RepositoryRulesetNames = rulesetNames
	pb.RequiredStatusChecks = model.RequiredStatusChecks{Contexts: checkContexts}
	pb.RequiredPullRequestReviews.BypassPullRequestAllowances = model.UserTeam{Users: bypassPullRequestAllowances}
	return pb
}

func getRuleParamerters(paramType string, parameters model.RuleParameters, checkContexts []string, pb *model.ProtectedBranch) []string {
	if paramType == "REQUIRED_STATUS_CHECKS" {
		checkContexts = getRuleCheckContexts(parameters.RequiredStatusChecksParam)
	} else if paramType == "PULL_REQUEST" {
		pb.RequiredPullRequestReviews = getRulePullRequestParams(parameters.PullRequestParam)
	}
	return checkContexts
}

func getRuleCheckContexts(node model.RuleParamStatusChecksParam) []string {
	checkContexts := make([]string, 0)
	for _, context := range node.RequiredStatusChecks {
		checkContexts = append(checkContexts, context.Context)
	}
	return checkContexts
}

func getRulePullRequestParams(node model.RulePullRequestParam) model.RequiredPullRequestReviews {
	return model.RequiredPullRequestReviews{
		DismissStaleReviews:            node.DismissStaleReviewsOnPush,
		RequireCodeOwnerReviews:        node.RequireCodeOwnerReview,
		RequireLastPushApproval:        node.RequireLastPushApproval,
		RequiredApprovingReviewCount:   node.RequiredApprovingReviewCount,
		RequiredReviewThreadResolution: node.RequiredReviewThreadResolution,
	}
}

const payload = `
	{
        "required_status_checks": {
            "strict": true,
            "checks": [
                
            ]
        },
        "required_pull_request_reviews": {
            "dismiss_stale_reviews": true,
            "require_code_owner_reviews": false,
            "require_last_push_approval": true,
            "required_approving_review_count": 1,
            "bypass_pull_request_allowances": {
                "users": [
                   
                ],
                "teams": []
            }
        },
        "required_signatures": false,
        "enforce_admins": true,
        "required_linear_history": false,
        "allow_force_pushes": false,
        "allow_deletions": false,
        "required_conversation_resolution": true,
        "lock_branch": false,
        "allow_fork_syncing": false,
        "restrictions": {
            "users": [
              
            ],
            "teams": [],
            "apps": []
        },
        "block_creations": false
    }
	`

func UpdateProtectedBranch(ctx *context.Context, orgName string, repoNames []string, branchName string, lock, removeStatus bool) []model.ProtectedBranch {
	responses := make([]model.ProtectedBranch, 0)

	processor.ProcessRepositoriesOperation(ctx, orgName, repoNames, processor.OperationUpdateProtectedBranch,
		func(ctx *context.Context, orgName, repoName string) (model.ProtectedBranch, error) {
			return UpdateProtectedBranchForRepo(ctx, orgName, repoName, branchName, lock, removeStatus)
		},
		func(repoName string, result processor.RepoOperationResult[model.ProtectedBranch]) {
			responses = append(responses, result.Result)
		},
		func(repoName string, err error) {
			responses = append(responses, model.ProtectedBranch{RepositoryName: repoName, ErrorMessage: err.Error()})
		})
	return responses
}

func UpdateProtectedBranchForRepo(ctx *context.Context, orgName string, repoName string, branchName string, lock, removeStatus bool) (model.ProtectedBranch, error) {
	var requestPayload model.ProtectedBranchRequest
	if err := json.Unmarshal([]byte(payload), &requestPayload); err != nil {
		return model.ProtectedBranch{RepositoryName: repoName}, err
	}
	if !removeStatus && !ctx.Config.IsRepoPresentInIgnoreForStatusCheck(orgName, repoName) {
		requestPayload.RequiredStatusChecks.Checks = append(requestPayload.RequiredStatusChecks.Checks, model.CheckRequest{Context: "Build", AppID: 15368})
	}

	if lock {
		requestPayload.LockBranch = true
	}

	requestPayload.RequiredPullRequestReviews.BypassPullRequestAllowances.Users = append(requestPayload.RequiredPullRequestReviews.BypassPullRequestAllowances.Users, ctx.Config.ProtectedBranchDetail(orgName).BypassPullRequestUsers...)

	requestPayload.Restrictions.Users = append(requestPayload.Restrictions.Users, ctx.Config.ProtectedBranchDetail(orgName).AllowedRestrictionsUsers...)
	requestPayload.RequiredPullRequestReviews.RequiredApprovingReviewCount = ctx.Config.ProtectedBranchDetail(orgName).ApprovingReviewCount

	jsonBody, err := json.Marshal(requestPayload)
	if err != nil {
		return model.ProtectedBranch{RepositoryName: repoName}, err
	}
	return service.UpdateProtectedBranch(ctx, orgName, repoName, branchName, jsonBody)
}

func DeleteProtectedBranch(ctx *context.Context, orgName string, repoNames []string, branchName string) []model.RefUIResponse {
	responses := make([]model.RefUIResponse, 0)

	processor.ProcessRepositoriesOperation(ctx, orgName, repoNames, processor.OperationDeleteProtectedBranch,
		func(ctx *context.Context, orgName, repoName string) (bool, error) {
			return service.DeleteProtectedBranch(ctx, orgName, repoName, branchName)
		},
		func(repoName string, result processor.RepoOperationResult[bool]) {
			responses = append(responses, model.CreateNewCommonResponse(repoName, branchName, "DELETE_PROTECTED_BRANCH", "Protected Branch deleted", ""))
		},
		func(repoName string, err error) {
			responses = append(responses, model.CreateNewCommonResponse(repoName, branchName, "DELETE_PROTECTED_BRANCH", "", err.Error()))
		})
	return responses
}
