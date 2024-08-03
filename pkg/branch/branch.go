package branch

import (
	"errors"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/apperrors"
	"github.com/prady-lab/sgh-cli/pkg/context"
)

func CreateNewBranchFromCommit(ctx *context.Context, orgName, newBranchName, commitSHA string, repoName string) []model.CommonResponse {
	actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, []string{repoName})
	response, err := service.CreateNewBranchFromCommit(ctx, orgName, actualRepoNames[0], newBranchName, commitSHA)
	if err != nil {
		var ge *apperrors.GitHubError
		if errors.As(err, &ge) {
			return []model.CommonResponse{{OrgName: orgName, RepositoryName: actualRepoNames[0], ItemName: newBranchName, ItemType: "BRANCH", ErrorMessage: ge.Message}}
		}
		return []model.CommonResponse{{OrgName: orgName, RepositoryName: actualRepoNames[0], ItemName: newBranchName, ItemType: "BRANCH", ErrorMessage: err.Error()}}
	}
	return []model.CommonResponse{{OrgName: orgName, RepositoryName: actualRepoNames[0], ItemName: newBranchName, ItemType: "BRANCH", SuccessMessage: response.Object.SHA}}
}

func CreateNewBranches(ctx *context.Context, orgName, newBranchName, refBranchName string, repoNames []string) []model.CommonResponse {
	branchResponses := make([]model.CommonResponse, 0)

	requestData := processor.RepoOperationData[processor.BranchOperationData]{OperationType: "Creating Branch", Message: "Creating Branches"}

	processor.ProcessRepositoriesOperation(ctx, orgName, repoNames, requestData,
		func(ctx *context.Context, orgName, repoName string, additionalData processor.RepoOperationData[processor.BranchOperationData]) (model.NewItemResponse, error) {
			return service.CreateNewBranch(ctx, orgName, repoName, newBranchName, refBranchName)
		},
		func(repoName string, additionalData processor.RepoOperationData[processor.BranchOperationData], result processor.RepoOperationResult[model.NewItemResponse]) {
			branchResponses = append(branchResponses, model.CommonResponse{OrgName: orgName, RepositoryName: repoName, ItemName: newBranchName, ItemType: "BRANCH", SuccessMessage: result.Result.Object.SHA})
		},
		func(repoName string, additionalData processor.RepoOperationData[processor.BranchOperationData], err error) {
			var ge *apperrors.GitHubError
			if errors.As(err, &ge) {
				branchResponses = append(branchResponses, model.CommonResponse{OrgName: orgName, RepositoryName: repoName, ItemName: newBranchName, ItemType: "BRANCH", ErrorMessage: ge.Message})
			} else {
				branchResponses = append(branchResponses, model.CommonResponse{OrgName: orgName, RepositoryName: repoName, ItemName: newBranchName, ItemType: "BRANCH", ErrorMessage: err.Error()})
			}
		})
	return branchResponses
}

func DeleteBranches(ctx *context.Context, orgName, branchName string, repoNames []string) []model.CommonResponse {
	branchResponses := make([]model.CommonResponse, 0)
	requestData := processor.RepoOperationData[processor.BranchOperationData]{OperationType: "Deleting Branch", Message: "Deleting Branches"}

	processor.ProcessRepositoriesOperation(ctx, orgName, repoNames, requestData,
		func(ctx *context.Context, orgName, repoName string, additionalData processor.RepoOperationData[processor.BranchOperationData]) (bool, error) {
			return service.DeleteBranch(ctx, orgName, repoName, branchName)
		},
		func(repoName string, additionalData processor.RepoOperationData[processor.BranchOperationData], result processor.RepoOperationResult[bool]) {
			branchResponses = append(branchResponses, model.CommonResponse{OrgName: orgName, RepositoryName: repoName, ItemName: branchName, ItemType: "BRANCH", SuccessMessage: "Branch deleted"})
		},
		func(repoName string, additionalData processor.RepoOperationData[processor.BranchOperationData], err error) {
			var ge *apperrors.GitHubError
			if errors.As(err, &ge) {
				branchResponses = append(branchResponses, model.CommonResponse{OrgName: orgName, RepositoryName: repoName, ItemName: branchName, ItemType: "BRANCH", ErrorMessage: ge.Message})
			} else {
				branchResponses = append(branchResponses, model.CommonResponse{OrgName: orgName, RepositoryName: repoName, ItemName: branchName, ItemType: "BRANCH", ErrorMessage: err.Error()})
			}
		})
	return branchResponses
}
