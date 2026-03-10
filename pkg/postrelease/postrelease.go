// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

// Package postrelease provides functions for automating post-release workflows
// such as creating hotfix branches and release tags across multiple repositories
// in a GitHub organization.
package postrelease

import (
	"fmt"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/context"
)

// PostReleaseRequest contains the parameters for a post-release workflow across repositories.
type PostReleaseRequest struct {
	OrgName      string
	RepoNames    []string
	ExcludeRepos []string
	// Ref is the source branch to create the hotfix branch and/or tag from.
	Ref        string
	BranchName string // optional: hotfix branch to create
	TagName    string // optional: release tag to create
	Message    string // optional: tag annotation message (defaults to TagName)
}

// ProcessPostRelease executes the post-release workflow for the given request,
// creating a hotfix branch and/or a release tag per repository.
// Returns a slice of PostReleaseResponse results.
func ProcessPostRelease(ctx *context.Context, request PostReleaseRequest) []model.PostReleaseResponse {
	responses := make([]model.PostReleaseResponse, 0)

	processor.ProcessRepositoriesOperation(ctx, request.OrgName, request.RepoNames, request.ExcludeRepos, processor.OperationPostRelease,
		func(ctx *context.Context, orgName, repoName string) (model.PostReleaseResponse, error) {
			result := model.PostReleaseResponse{RepositoryName: repoName}

			if request.BranchName != "" {
				branchResp, err := service.CreateNewBranch(ctx, orgName, repoName, request.BranchName, request.Ref)
				if err != nil {
					return result, fmt.Errorf("failed to create branch %q: %w", request.BranchName, err)
				}
				result.BranchName = request.BranchName
				result.BranchSHA = branchResp.Object.SHA
			}

			if request.TagName != "" {
				msg := request.Message
				if msg == "" {
					msg = request.TagName
				}
				tagResp, err := service.CreateNewTag(ctx, orgName, repoName, request.TagName, request.Ref, msg)
				if err != nil {
					return result, fmt.Errorf("failed to create tag %q: %w", request.TagName, err)
				}
				result.TagName = request.TagName
				result.TagURL = tagResp.Url
				result.TagSHA = tagResp.Object.SHA
			}

			return result, nil
		},
		func(repoName string, result processor.RepoOperationResult[model.PostReleaseResponse]) {
			responses = append(responses, result.Result)
		},
		func(repoName string, err error) {
			responses = append(responses, model.PostReleaseResponse{RepositoryName: repoName, ErrorMessage: err.Error()})
		})

	return responses
}
