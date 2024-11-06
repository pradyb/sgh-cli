package commit

import (
	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/context"
)

func ListCommits(ctx *context.Context, orgName string, repoNames []string, branchName string, noOfDays int) []model.CommitResponse {
	responses := make([]model.CommitResponse, 0)

	processor.ProcessRepositoriesOperation(ctx, orgName, repoNames, []string{}, processor.OperationListPullRequest,
		func(ctx *context.Context, orgName, repoName string) ([]model.CommitResponse, error) {
			return service.ListCommits(ctx, orgName, repoName, branchName, noOfDays)
		},
		func(repoName string, result processor.RepoOperationResult[[]model.CommitResponse]) {
			responses = append(responses, result.Result...)
		},
		func(repoName string, err error) {
			responses = append(responses, model.CommitResponse{RepositoryName: repoName, ErrorMessage: err.Error()})
		})
	return responses
}

func GetCommitInfo(ctx *context.Context, orgName string, repoName string, commitSha string) model.CommitResponse {
	response, err := service.GetCommitInfo(ctx, orgName, repoName, commitSha)
	if err != nil {
		return model.CommitResponse{RepositoryName: repoName, ErrorMessage: err.Error()}
	}
	return response
}

func GetCommitCheckRuns(ctx *context.Context, orgName string, repoName string, commitSha string) model.CheckRunResponse {
	checkRuns, err := service.GetCommitCheckRuns(ctx, orgName, repoName, commitSha)
	if err != nil {
		return model.CheckRunResponse{RepositoryName: repoName, ErrorMessage: err.Error()}
	}
	return checkRuns
}
