package pr

import (
	"sync"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/commit"
	"github.com/prady-lab/sgh-cli/pkg/context"
)

func CreateNewPullRequest(ctx *context.Context, orgName string, repoNames []string, baseRef, headRef, title, body string) []model.PullRequestResponse {
	responses := make([]model.PullRequestResponse, 0)

	processor.ProcessRepositoriesOperation(ctx, orgName, repoNames, processor.OperationCreatePullRequest,
		func(ctx *context.Context, orgName, repoName string) (model.PullRequestResponse, error) {
			return CreateNewPullRequestForRepo(ctx, orgName, repoName, baseRef, headRef, title, body)
		},
		func(repoName string, result processor.RepoOperationResult[model.PullRequestResponse]) {
			responses = append(responses, result.Result)
		},
		func(repoName string, err error) {
			responses = append(responses, model.PullRequestResponse{ErrorMessage: err.Error()})
		})
	return responses
}

func CreateNewPullRequestForRepo(ctx *context.Context, orgName string, repoName, baseRef, headRef, title, body string) (model.PullRequestResponse, error) {
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

func ReviewPullRequest(ctx *context.Context, orgName string, repoName string, prNumber int, event, body string) model.ReviewPullRequestResponse {
	response, err := service.ReviewPullRequest(ctx, orgName, repoName, prNumber, event, body)
	if err != nil {
		return model.ReviewPullRequestResponse{ErrorMessage: err.Error()}
	}
	return response
}

func ListPullRequestReviews(ctx *context.Context, orgName string, repoName string, prNumber int) []model.ReviewPullRequestResponse {
	response, err := service.ListPullRequestReviews(ctx, orgName, repoName, prNumber)
	if err != nil {
		return []model.ReviewPullRequestResponse{{ErrorMessage: err.Error()}}
	}
	return response
}

func GetPullRequestFiles(ctx *context.Context, orgName string, repoName string, prNumber int) model.PullRequestFilesResponse {
	response, err := service.GetPullRequestFiles(ctx, orgName, repoName, prNumber)
	if err != nil {
		return model.PullRequestFilesResponse{RepositoryName: repoName, PRNumber: prNumber, ErrorMessage: err.Error()}
	}
	return model.PullRequestFilesResponse{RepositoryName: repoName, PRNumber: prNumber, Files: response}
}

func GetPullRequestInfo(ctx *context.Context, orgName string, repoName string, prNumber int) model.PullRequestResponse {
	response, err := service.GetPullRequestInfo(ctx, orgName, repoName, prNumber)
	if err != nil {
		return model.PullRequestResponse{ErrorMessage: err.Error()}
	}
	return response
}

func UpdatePullRequest(ctx *context.Context, orgName string, repoName string, prNumber int, state string) model.PullRequestResponse {
	response, err := service.UpdatePullRequest(ctx, orgName, repoName, prNumber, state)
	if err != nil {
		return model.PullRequestResponse{ErrorMessage: err.Error()}
	}
	return response
}

func MergePullRequest(ctx *context.Context, orgName string, repoName string, prNumber int, title, body string) model.MergeResponse {
	actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, []string{repoName})
	if len(actualRepoNames) == 0 {
		return model.MergeResponse{RepositoryName: actualRepoNames[0], ErrorMessage: "Repository not found"}
	}
	response, err := service.MergePullRequest(ctx, orgName, actualRepoNames[0], prNumber, title, body)
	if err != nil {
		return model.MergeResponse{RepositoryName: actualRepoNames[0], ErrorMessage: err.Error()}
	}
	response.RepositoryName = actualRepoNames[0]
	return response
}

func GetPRDetails(ctx *context.Context, orgName string, repoName string, prNumber int, lastSha string) (model.PullRequestResponse, model.PullRequestFilesResponse, model.CheckRunResponse, []model.ReviewPullRequestResponse) {

	var wg sync.WaitGroup
	var pullRequestResponse model.PullRequestResponse
	var pullRequestFilesResponse model.PullRequestFilesResponse
	var checkRunResponse model.CheckRunResponse
	var prReviews []model.ReviewPullRequestResponse

	wg.Add(4)
	go func() {
		defer wg.Done()
		pullRequestResponse = GetPullRequestInfo(ctx, orgName, repoName, prNumber)
	}()
	go func() {
		defer wg.Done()
		pullRequestFilesResponse = GetPullRequestFiles(ctx, orgName, repoName, prNumber)
	}()
	go func() {
		defer wg.Done()
		checkRunResponse = commit.GetCommitCheckRuns(ctx, orgName, repoName, lastSha)
	}()
	go func() {
		defer wg.Done()
		prReviews = ListPullRequestReviews(ctx, orgName, repoName, prNumber)
	}()

	wg.Wait()
	return pullRequestResponse, pullRequestFilesResponse, checkRunResponse, prReviews
}
