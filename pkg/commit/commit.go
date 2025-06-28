package commit

import (
	"fmt"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/context"
)

// CommitListRequest contains parameters for listing commits.
type CommitListRequest struct {
	OrgName      string
	RepoNames    []string
	ExcludeRepos []string
	BranchName   string
	NoOfDays     int
}

// CommitInfoRequest contains parameters for getting commit information.
type CommitInfoRequest struct {
	OrgName   string
	RepoName  string
	CommitSHA string
}

// CommitCheckRunsRequest contains parameters for getting commit check runs.
type CommitCheckRunsRequest struct {
	OrgName   string
	RepoName  string
	CommitSHA string
}

func ListCommits(ctx *context.Context, req CommitListRequest) []model.CommitResponse {
	responses := make([]model.CommitResponse, 0)

	processor.ProcessRepositoriesOperation(ctx, req.OrgName, req.RepoNames, req.ExcludeRepos, processor.OperationListPullRequest,
		func(ctx *context.Context, orgName, repoName string) ([]model.CommitResponse, error) {
			return service.ListCommits(ctx, orgName, repoName, req.BranchName, req.NoOfDays)
		},
		func(repoName string, result processor.RepoOperationResult[[]model.CommitResponse]) {
			responses = append(responses, result.Result...)
		},
		func(repoName string, err error) {
			responses = append(responses, model.CommitResponse{RepositoryName: repoName, ErrorMessage: fmt.Sprintf("failed to list commits: %v", err)})
		})
	return responses
}

func GetCommitInfo(ctx *context.Context, req CommitInfoRequest) model.CommitResponse {
	response, err := service.GetCommitInfo(ctx, req.OrgName, req.RepoName, req.CommitSHA)
	if err != nil {
		return model.CommitResponse{RepositoryName: req.RepoName, ErrorMessage: fmt.Sprintf("failed to get commit info: %v", err)}
	}
	return response
}

func GetCommitCheckRuns(ctx *context.Context, req CommitCheckRunsRequest) model.CheckRunResponse {
	checkRuns, err := service.GetCommitCheckRuns(ctx, req.OrgName, req.RepoName, req.CommitSHA)
	if err != nil {
		return model.CheckRunResponse{RepositoryName: req.RepoName, ErrorMessage: fmt.Sprintf("failed to get commit check runs: %v", err)}
	}
	return checkRuns
}
