package tag

import (
	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/context"
)

func CreateNewTags(ctx *context.Context, orgName string, repoNames []string, tagName, refBranchName string, message string) []model.RefUIResponse {
	responses := make([]model.RefUIResponse, 0)

	processor.ProcessRepositoriesOperation(ctx, orgName, repoNames, processor.OperationCreateTag,
		func(ctx *context.Context, orgName, repoName string) (model.RefResponse, error) {
			return service.CreateNewTag(ctx, orgName, repoName, tagName, refBranchName, message)
		},
		func(repoName string, result processor.RepoOperationResult[model.RefResponse]) {
			responses = append(responses, model.CreateNewCommonResponse(repoName, tagName, "CREATE_TAG", result.Result.Object.SHA, ""))
		},
		func(repoName string, err error) {
			responses = append(responses, model.CreateNewCommonResponse(repoName, tagName, "CREATE_TAG", "", err.Error()))
		})
	return responses
}

func CreateNewTag(ctx *context.Context, orgName, repoName, tagName, refBranchName, message string) (model.RefResponse, error) {
	return service.CreateNewTag(ctx, orgName, repoName, tagName, refBranchName, message)
}

func DeleteTags(ctx *context.Context, orgName string, repoNames []string, tagName string) []model.RefUIResponse {
	responses := make([]model.RefUIResponse, 0)

	processor.ProcessRepositoriesOperation(ctx, orgName, repoNames, processor.OperationDeleteTag,
		func(ctx *context.Context, orgName, repoName string) (bool, error) {
			return service.DeleteTag(ctx, orgName, repoName, tagName)
		},
		func(repoName string, result processor.RepoOperationResult[bool]) {
			responses = append(responses, model.CreateNewCommonResponse(repoName, tagName, "DELETE_TAG", "Branch deleted", ""))
		},
		func(repoName string, err error) {
			responses = append(responses, model.CreateNewCommonResponse(repoName, tagName, "DELETE_TAG", "", err.Error()))
		})
	return responses
}
