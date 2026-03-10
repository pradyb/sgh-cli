package issue

import (
	"fmt"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/context"
)

type IssueListRequest struct {
	OrgName          string
	RepoNames        []string
	ExcludeRepoNames []string
	State            string
	Labels           string
	Assignee         string
	Creator          string
	LastCount        int
}

type IssueViewRequest struct {
	OrgName     string
	RepoName    string
	IssueNumber int
}

type IssueUpdateRequest struct {
	OrgName     string
	RepoName    string
	IssueNumber int
	State       string // "open" or "closed"
}

type IssueUpdateResponse struct {
	RepositoryName string
	IssueNumber    int
	ErrorMessage   string
}

func UpdateIssue(ctx *context.Context, req IssueUpdateRequest) IssueUpdateResponse {
	err := service.UpdateIssue(ctx, req.OrgName, req.RepoName, req.IssueNumber, req.State)
	if err != nil {
		return IssueUpdateResponse{
			RepositoryName: req.RepoName,
			IssueNumber:    req.IssueNumber,
			ErrorMessage:   fmt.Sprintf("failed to update issue: %v", err),
		}
	}
	return IssueUpdateResponse{RepositoryName: req.RepoName, IssueNumber: req.IssueNumber}
}

func ListIssues(ctx *context.Context, req IssueListRequest) []model.IssueResponse {
	responses := make([]model.IssueResponse, 0)

	processor.ProcessRepositoriesOperation(ctx, req.OrgName, req.RepoNames, req.ExcludeRepoNames, processor.OperationListIssues,
		func(ctx *context.Context, orgName, repoName string) ([]model.IssueResponse, error) {
			issues, err := service.ListIssues(ctx, orgName, repoName, req.State, req.Labels, req.Assignee, req.Creator, req.LastCount)
			if err != nil {
				return nil, err
			}
			// GitHub Issues API returns PRs too; filter them out
			filtered := make([]model.IssueResponse, 0, len(issues))
			for _, i := range issues {
				if i.IsIssue() {
					filtered = append(filtered, i)
				}
			}
			return filtered, nil
		},
		func(repoName string, result processor.RepoOperationResult[[]model.IssueResponse]) {
			responses = append(responses, result.Result...)
		},
		func(repoName string, err error) {
			responses = append(responses, model.IssueResponse{
				RepositoryName: repoName,
				ErrorMessage:   fmt.Sprintf("failed to list issues: %v", err),
			})
		})

	return responses
}

func GetIssue(ctx *context.Context, req IssueViewRequest) model.IssueResponse {
	actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(req.OrgName, []string{req.RepoName})
	if len(actualRepoNames) == 0 {
		return model.IssueResponse{ErrorMessage: fmt.Sprintf("repository not found: %s", req.RepoName)}
	}
	repoName := actualRepoNames[0]

	issue, err := service.GetIssue(ctx, req.OrgName, repoName, req.IssueNumber)
	if err != nil {
		return model.IssueResponse{
			RepositoryName: repoName,
			ErrorMessage:   fmt.Sprintf("failed to get issue: %v", err),
		}
	}
	return issue
}

func GetIssueComments(ctx *context.Context, orgName, repoName string, issueNumber int) []model.IssueComment {
	comments, err := service.ListIssueComments(ctx, orgName, repoName, issueNumber)
	if err != nil {
		return nil
	}
	return comments
}
