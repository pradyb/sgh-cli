package branch

import (
	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/context"
)

func CreateNewBranchFromCommit(ctx *context.Context, orgName, repoName, newBranchName, commitSHA string) []model.RefUIResponse {
	actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, []string{repoName})

	response, err := service.CreateNewBranchFromCommit(ctx, orgName, actualRepoNames[0], newBranchName, commitSHA)
	if err != nil {
		return []model.RefUIResponse{model.CreateNewCommonResponse(actualRepoNames[0], newBranchName, "CREATE_BRANCH_BY_COMMIT_ID", "", err.Error())}
	}
	return []model.RefUIResponse{model.CreateNewCommonResponse(actualRepoNames[0], newBranchName, "CREATE_BRANCH_BY_COMMIT_ID", "", response.Object.SHA)}
}

func CreateNewBranches(ctx *context.Context, orgName string, repoNames []string, newBranchName, refBranchName string) []model.RefUIResponse {
	responses := make([]model.RefUIResponse, 0)

	processor.ProcessRepositoriesOperation(ctx, orgName, repoNames, processor.OperationCreateBranch,
		func(ctx *context.Context, orgName, repoName string) (model.RefResponse, error) {
			return service.CreateNewBranch(ctx, orgName, repoName, newBranchName, refBranchName)
		},
		func(repoName string, result processor.RepoOperationResult[model.RefResponse]) {
			responses = append(responses, model.CreateNewCommonResponse(repoName, newBranchName, "CREATE_BRANCH", result.Result.Object.SHA, ""))
		},
		func(repoName string, err error) {
			responses = append(responses, model.CreateNewCommonResponse(repoName, newBranchName, "CREATE_BRANCH", "", err.Error()))
		})
	return responses
}

func DeleteBranches(ctx *context.Context, orgName string, repoNames []string, branchName string) []model.RefUIResponse {
	responses := make([]model.RefUIResponse, 0)

	processor.ProcessRepositoriesOperation(ctx, orgName, repoNames, processor.OperationDeleteBranch,
		func(ctx *context.Context, orgName, repoName string) (bool, error) {
			return service.DeleteBranch(ctx, orgName, repoName, branchName)
		},
		func(repoName string, result processor.RepoOperationResult[bool]) {
			responses = append(responses, model.CreateNewCommonResponse(repoName, branchName, "DELETE_BRANCH", "Branch deleted", ""))
		},
		func(repoName string, err error) {
			responses = append(responses, model.CreateNewCommonResponse(repoName, branchName, "DELETE_BRANCH", "", err.Error()))
		})
	return responses
}
