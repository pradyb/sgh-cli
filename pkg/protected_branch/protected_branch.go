package protectedbranch

import (
	"encoding/json"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/context"
)

func ListProtectedBranches(ctx *context.Context, orgName string, repoNames []string, branchName string) []model.ProtectedBranch {
	responses := make([]model.ProtectedBranch, 0)

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
	return responses
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
				return model.ProtectedBranch{}, err
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
				return model.ProtectedBranch{}, err
			}
			return service.UpdateProtectedBranch(ctx, orgName, repoName, branchName, jsonBody)
		},
		func(repoName string, result processor.RepoOperationResult[model.ProtectedBranch]) {
			responses = append(responses, result.Result)
		},
		func(repoName string, err error) {
			responses = append(responses, model.ProtectedBranch{ErrorMessage: err.Error()})
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
