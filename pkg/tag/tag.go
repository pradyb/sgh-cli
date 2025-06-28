package tag

import (
	"fmt"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/context"
)

// TagCreateRequest contains parameters for creating tags.
type TagCreateRequest struct {
	OrgName          string
	RepoNames        []string
	ExcludeRepoNames []string
	TagName          string
	RefBranchName    string
	Message          string
}

// TagCreateSingleRequest contains parameters for creating a single tag.
type TagCreateSingleRequest struct {
	OrgName          string
	RepoName         string
	ExcludeRepoNames []string
	TagName          string
	RefBranchName    string
	Message          string
}

// TagDeleteRequest contains parameters for deleting tags.
type TagDeleteRequest struct {
	OrgName          string
	RepoNames        []string
	ExcludeRepoNames []string
	TagName          string
}

func CreateNewTags(ctx *context.Context, req TagCreateRequest) []model.RefUIResponse {
	responses := make([]model.RefUIResponse, 0)

	processor.ProcessRepositoriesOperation(ctx, req.OrgName, req.RepoNames, req.ExcludeRepoNames, processor.OperationCreateTag,
		func(ctx *context.Context, orgName, repoName string) (model.RefResponse, error) {
			return service.CreateNewTag(ctx, orgName, repoName, req.TagName, req.RefBranchName, req.Message)
		},
		func(repoName string, result processor.RepoOperationResult[model.RefResponse]) {
			responses = append(responses, model.CreateNewCommonResponse(repoName, req.TagName, "CREATE_TAG", result.Result.Object.SHA, ""))
		},
		func(repoName string, err error) {
			responses = append(responses, model.CreateNewCommonResponse(repoName, req.TagName, "CREATE_TAG", "", fmt.Sprintf("failed to create tag: %v", err)))
		})
	return responses
}

func CreateNewTag(ctx *context.Context, req TagCreateSingleRequest) (model.RefResponse, error) {
	return service.CreateNewTag(ctx, req.OrgName, req.RepoName, req.TagName, req.RefBranchName, req.Message)
}

func DeleteTags(ctx *context.Context, req TagDeleteRequest) []model.RefUIResponse {
	responses := make([]model.RefUIResponse, 0)

	processor.ProcessRepositoriesOperation(ctx, req.OrgName, req.RepoNames, req.ExcludeRepoNames, processor.OperationDeleteTag,
		func(ctx *context.Context, orgName, repoName string) (bool, error) {
			return service.DeleteTag(ctx, orgName, repoName, req.TagName)
		},
		func(repoName string, result processor.RepoOperationResult[bool]) {
			responses = append(responses, model.CreateNewCommonResponse(repoName, req.TagName, "DELETE_TAG", "Tag deleted", ""))
		},
		func(repoName string, err error) {
			responses = append(responses, model.CreateNewCommonResponse(repoName, req.TagName, "DELETE_TAG", "", fmt.Sprintf("failed to delete tag: %v", err)))
		})
	return responses
}
