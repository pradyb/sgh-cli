package pr

import (
	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/context"
)

func CreateNewPullRequest(ctx *context.Context, orgName string, repoNames []string, baseRef, headRef, title, body string) []model.PullRequestResponse {
	responses := make([]model.PullRequestResponse, 0)

	processor.ProcessRepositoriesOperation(ctx, orgName, repoNames, processor.OperationCreatePullRequest,
		func(ctx *context.Context, orgName, repoName string) (model.PullRequestResponse, error) {
			prResponse, err := service.CreateNewPullRequest(ctx, orgName, repoName, title, body, baseRef, headRef)
			if err != nil {
				return model.PullRequestResponse{}, err
			}
			assignees := ctx.Config.PullRequestAssignees(orgName)
			if len(assignees) > 0 {
				service.AddIssueAssignees(ctx, orgName, repoName, prResponse.PRNumber, assignees)
				service.AddReviewers(ctx, orgName, repoName, prResponse.PRNumber, assignees)
			}
			return service.GetPullRequestInfo(ctx, orgName, repoName, prResponse.PRNumber)
		},
		func(repoName string, result processor.RepoOperationResult[model.PullRequestResponse]) {
			responses = append(responses, result.Result)
		},
		func(repoName string, err error) {
			responses = append(responses, model.PullRequestResponse{ErrorMessage: err.Error()})
		})
	return responses
}

func ListPullRequests(ctx *context.Context, orgName string, repoNames []string, baseRef, headRef string, all bool) []model.PullRequestResponse {
	responses := make([]model.PullRequestResponse, 0)

	processor.ProcessRepositoriesOperation(ctx, orgName, repoNames, processor.OperationListPullRequest,
		func(ctx *context.Context, orgName, repoName string) ([]model.PullRequestResponse, error) {
			return service.ListPullRequests(ctx, orgName, repoName, baseRef, headRef, all)
		},
		func(repoName string, result processor.RepoOperationResult[[]model.PullRequestResponse]) {
			responses = append(responses, result.Result...)
		},
		func(repoName string, err error) {
			responses = append(responses, model.PullRequestResponse{ErrorMessage: err.Error()})
		})
	return responses
}

func UpdatePullRequest(ctx *context.Context, orgName string, repoName string, prNumber int, state string) model.PullRequestResponse {
	response, err := service.UpdatePullRequest(ctx, orgName, repoName, prNumber, state)
	if err != nil {
		return model.PullRequestResponse{ErrorMessage: err.Error()}
	}
	return response
}

func MergePullRequest(ctx *context.Context, orgName string, repoName string, prNumber int, title, body string) model.MergeResponse {
	response, err := service.MergePullRequest(ctx, orgName, repoName, prNumber, title, body)
	if err != nil {
		return model.MergeResponse{ErrorMessage: err.Error()}
	}
	return response
}
