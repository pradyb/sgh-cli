package branch

import (
	"fmt"
	"regexp"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/context"
)

// BranchListRequest contains parameters for listing branches.
type BranchListRequest struct {
	OrgName          string
	RepoNames        []string
	ExcludeRepoNames []string
	Filter           string
}

// BranchCreateFromCommitRequest contains parameters for creating a branch from a commit.
type BranchCreateFromCommitRequest struct {
	OrgName       string
	RepoName      string
	NewBranchName string
	CommitSHA     string
}

// BranchCreateRequest contains parameters for creating branches.
type BranchCreateRequest struct {
	OrgName          string
	RepoNames        []string
	ExcludeRepoNames []string
	NewBranchName    string
	RefBranchName    string
}

// BranchDeleteRequest contains parameters for deleting branches.
type BranchDeleteRequest struct {
	OrgName          string
	RepoNames        []string
	ExcludeRepoNames []string
	BranchName       string
}

func CreateNewBranchFromCommit(ctx *context.Context, req BranchCreateFromCommitRequest) []model.RefUIResponse {
	actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(req.OrgName, []string{req.RepoName})

	response, err := service.CreateNewBranchFromCommit(ctx, req.OrgName, actualRepoNames[0], req.NewBranchName, req.CommitSHA)
	if err != nil {
		return []model.RefUIResponse{model.CreateNewCommonResponse(actualRepoNames[0], req.NewBranchName, "CREATE_BRANCH_BY_COMMIT_ID", "", fmt.Sprintf("failed to create branch from commit: %v", err))}
	}
	return []model.RefUIResponse{model.CreateNewCommonResponse(actualRepoNames[0], req.NewBranchName, "CREATE_BRANCH_BY_COMMIT_ID", "", response.Object.SHA)}
}

func CreateNewBranches(ctx *context.Context, req BranchCreateRequest) []model.RefUIResponse {
	responses := make([]model.RefUIResponse, 0)

	processor.ProcessRepositoriesOperation(ctx, req.OrgName, req.RepoNames, req.ExcludeRepoNames, processor.OperationCreateBranch,
		func(ctx *context.Context, orgName, repoName string) (model.RefResponse, error) {
			return service.CreateNewBranch(ctx, orgName, repoName, req.NewBranchName, req.RefBranchName)
		},
		func(repoName string, result processor.RepoOperationResult[model.RefResponse]) {
			responses = append(responses, model.CreateNewCommonResponse(repoName, req.NewBranchName, "CREATE_BRANCH", result.Result.Object.SHA, ""))
		},
		func(repoName string, err error) {
			responses = append(responses, model.CreateNewCommonResponse(repoName, req.NewBranchName, "CREATE_BRANCH", "", fmt.Sprintf("failed to create branch: %v", err)))
		})
	return responses
}

func DeleteBranches(ctx *context.Context, req BranchDeleteRequest) []model.RefUIResponse {
	responses := make([]model.RefUIResponse, 0)

	processor.ProcessRepositoriesOperation(ctx, req.OrgName, req.RepoNames, req.ExcludeRepoNames, processor.OperationDeleteBranch,
		func(ctx *context.Context, orgName, repoName string) (bool, error) {
			return service.DeleteBranch(ctx, orgName, repoName, req.BranchName)
		},
		func(repoName string, result processor.RepoOperationResult[bool]) {
			responses = append(responses, model.CreateNewCommonResponse(repoName, req.BranchName, "DELETE_BRANCH", "Branch deleted", ""))
		},
		func(repoName string, err error) {
			responses = append(responses, model.CreateNewCommonResponse(repoName, req.BranchName, "DELETE_BRANCH", "", fmt.Sprintf("failed to delete branch: %v", err)))
		})
	return responses
}

func ListBranches(ctx *context.Context, req BranchListRequest) []model.BranchResponse {
	responses := make([]model.BranchResponse, 0)

	var filterRegex *regexp.Regexp
	if req.Filter != "" {
		filterRegex, _ = regexp.Compile("(?i)" + req.Filter)
	}

	processor.ProcessRepositoriesOperation(ctx, req.OrgName, req.RepoNames, req.ExcludeRepoNames, processor.OperationListBranches,
		func(ctx *context.Context, orgName, repoName string) ([]model.BranchResponse, error) {
			branches, err := service.ListBranches(ctx, orgName, repoName)
			if err != nil {
				return nil, err
			}
			for i := range branches {
				branches[i].RepositoryName = repoName
			}
			return branches, nil
		},
		func(repoName string, result processor.RepoOperationResult[[]model.BranchResponse]) {
			for _, b := range result.Result {
				if filterRegex == nil || filterRegex.MatchString(b.Name) {
					responses = append(responses, b)
				}
			}
		},
		func(repoName string, err error) {
			responses = append(responses, model.BranchResponse{
				RepositoryName: repoName,
				Name:           fmt.Sprintf("error: %v", err),
			})
		})

	return responses
}
