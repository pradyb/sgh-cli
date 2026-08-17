// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package issue

import (
	"fmt"

	"github.com/shurcooL/githubv4"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
)

type IssueListRequest struct {
	OrgName          string
	RepoNames        []string
	ExcludeRepoNames []string
	State            string
	Labels           string
	Assignee         string
	Author           string
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

type IssueCreateRequest struct {
	OrgName  string
	RepoName string
	Title    string
	Body     string
	Assignee string
	Labels   []string
}

type IssueUpdateResponse struct {
	RepositoryName string
	IssueNumber    int
	ErrorMessage   string
}

func CreateIssue(ctx *context.Context, req IssueCreateRequest) model.IssueResponse {
	actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(req.OrgName, []string{req.RepoName})
	repoName := req.RepoName
	if len(actualRepoNames) > 0 {
		repoName = actualRepoNames[0]
	}
	issue, err := service.CreateIssue(ctx, req.OrgName, repoName, req.Title, req.Body, req.Assignee, req.Labels)
	if err != nil {
		return model.IssueResponse{
			RepositoryName: repoName,
			ErrorMessage:   fmt.Sprintf("failed to create issue: %v", err),
		}
	}
	return issue
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

	if len(req.RepoNames) <= 1 {
		// Invoke via GraphQL
		logger.Flog.Info().Msgf("Invoking GraphQL to list issues for %s", req.OrgName)

		queryString := buildIssueSearchQuery(ctx, req)
		variables := map[string]any{
			"queryString": githubv4.String(queryString),
			"issueCursor": (*githubv4.String)(nil),
			"lastCount":   githubv4.Int(req.LastCount),
		}

		var issueQuery model.SearchIssuesQuery
		err := service.Query(ctx, &issueQuery, variables)
		if err != nil {
			logger.Glog.Error().Err(err).Msg("Error in listing issues via GraphQL")
			return responses
		}

		for _, edge := range issueQuery.Search.Edges {
			issue := edge.Node.Issue
			labels := make([]model.IssueLabel, 0)
			for _, labelEdge := range issue.Labels.Edges {
				labels = append(labels, model.IssueLabel{Name: labelEdge.Node.Name})
			}

			issueResponse := model.IssueResponse{
				Number:         issue.Number,
				Title:          issue.Title,
				Body:           issue.Body,
				State:          issue.State,
				HTMLUrl:        issue.Url,
				CreatedAt:      issue.CreatedAt,
				UpdatedAt:      issue.UpdatedAt,
				RepositoryName: issue.Repository.Name,
				Author: model.User{
					Login: issue.Author.User.Login,
					Name:  issue.Author.User.Name,
				},
				Labels:   labels,
				Comments: issue.Comments.TotalCount,
			}

			issueResponse.Assignees = populateAssignees(issue.Assignees)
			responses = append(responses, issueResponse)
		}
		return responses
	} else {
		processor.ProcessRepositoriesOperation(ctx, req.OrgName, req.RepoNames, req.ExcludeRepoNames, processor.OperationListIssues,
			func(ctx *context.Context, orgName, repoName string) ([]model.IssueResponse, error) {
				// The REST endpoint calls the issue author "creator".
				issues, err := service.ListIssues(ctx, orgName, repoName, req.State, req.Labels, req.Assignee, req.Author, req.LastCount)
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
}

func buildIssueSearchQuery(ctx *context.Context, req IssueListRequest) string {
	scope := "org:" + req.OrgName
	if len(req.RepoNames) == 1 {
		actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(req.OrgName, req.RepoNames)
		if len(actualRepoNames) > 0 {
			scope = "repo:" + req.OrgName + "/" + actualRepoNames[0]
		}
	}

	queryString := scope + " is:issue"

	if req.State != "" && req.State != "all" {
		queryString += " state:" + req.State
	}

	if req.Labels != "" {
		queryString += " label:" + req.Labels
	}

	if req.Assignee != "" {
		queryString += " assignee:" + req.Assignee
	}

	if req.Author != "" {
		queryString += " author:" + req.Author
	}

	return queryString
}

func populateAssignees(assignees model.AssigneesFragment) []model.User {
	users := make([]model.User, 0)
	for _, edge := range assignees.Edges {
		users = append(users, model.User{
			Login: edge.Node.Login,
			Name:  edge.Node.Name,
		})
	}
	return users
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
