package protectedbranch

import (
	"encoding/json"
	"strings"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/context"
	logger "github.com/prady-lab/sgh-cli/utils"
	"github.com/shurcooL/githubv4"
)

func ListProtectedBranches(ctx *context.Context, orgName string, repoNames []string, branchName string) []model.ProtectedBranch {
	responses := make([]model.ProtectedBranch, 0)

	if len(repoNames) <= 1 {
		// Invoke via GraphQL
		logger.Glog.Info().Msgf("Invoking GraphQL to list protected branches for %s", orgName)
		queryString := getQueryString(ctx, orgName, repoNames)
		variables := map[string]interface{}{
			"queryString": githubv4.String(queryString),
			"branchName":  githubv4.String(branchName),
			"repoCursor":  (*githubv4.String)(nil),
		}

		var selectedBranchName string
		for {
			var searchProtectedBranchesQuery model.SearchProtectedBranchesQuery
			err := service.Query(ctx, &searchProtectedBranchesQuery, variables)
			if err != nil {
				logger.Glog.Error().Err(err).Msg("Error in listing protected branches")
				return responses
			}
			for _, edge := range searchProtectedBranchesQuery.Search.Edges {
				if edge.Node.Repository.Refs.TotalCount != 0 {
					node := edge.Node.Repository.Refs.Edges[0].Node
					selectedBranchName = node.Name
					if edge.Node.Repository.Refs.TotalCount > 1 {
						for _, edge := range edge.Node.Repository.Refs.Edges {
							if edge.Node.Name == branchName {
								node = edge.Node
								selectedBranchName = node.Name
							}
						}
					}
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
					responses = append(responses, model.ProtectedBranch{
						RepositoryName:                 edge.Node.Repository.Name,
						LockBranch:                     model.BoolData{Enabled: node.BranchProtectionRule.LockBranch},
						EnforceAdmins:                  model.BoolData{Enabled: node.BranchProtectionRule.IsAdminEnforced},
						RequiredConversationResolution: model.BoolData{Enabled: node.BranchProtectionRule.RequiresConversationResolution},
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
					})
				}
			}
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
				return service.ListProtectedBranches(ctx, orgName, repoName, branchName)
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

func getQueryString(ctx *context.Context, orgName string, repoNames []string) string {
	var queryString string
	queryString = "org:" + orgName
	if len(repoNames) == 0 {
		if ctx.Config.IsOrganizationPresent(orgName) {
			includes := ctx.Config.IncludePatterns(orgName)
			if (len(includes)) == 1 {
				queryString = queryString + " " + strings.ReplaceAll(includes[0], "*", "") + " in:name"
			}
		}
	} else {
		actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, repoNames)
		logger.Glog.Info().Str("repos", strings.Join(actualRepoNames, ",")).Msgf("Listing Pull Requests for selected repositories in %s", orgName)
		queryString = queryString + " " + actualRepoNames[0] + " in:name"
	}
	return queryString
}

func UpdateProtectedBranch(ctx *context.Context, orgName string, repoNames []string, branchName string, lock, removeStatus bool) []model.ProtectedBranch {
	responses := make([]model.ProtectedBranch, 0)

	payload := `
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
	processor.ProcessRepositoriesOperation(ctx, orgName, repoNames, processor.OperationUpdateProtectedBranch,
		func(ctx *context.Context, orgName, repoName string) (model.ProtectedBranch, error) {
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
		},
		func(repoName string, result processor.RepoOperationResult[model.ProtectedBranch]) {
			responses = append(responses, result.Result)
		},
		func(repoName string, err error) {
			responses = append(responses, model.ProtectedBranch{RepositoryName: repoName, ErrorMessage: err.Error()})
		})
	return responses
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
