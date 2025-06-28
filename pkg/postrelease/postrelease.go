// Package postrelease provides functions for automating post-release workflows such as merging branches,
// creating tags, and updating protected branch settings across multiple repositories in a GitHub organization.
package postrelease

import (
	"fmt"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/pr"
	pb "github.com/prady-lab/sgh-cli/pkg/protectedbranch"
	"github.com/prady-lab/sgh-cli/pkg/tag"
)

// PostReleaseRequest contains the parameters for a post-release workflow across repositories.
type PostReleaseRequest struct {
	OrgName      string
	RepoNames    []string
	ExcludeRepos []string
	BaseRef      string
	HeadRef      string
	Title        string
	Body         string
	CreateTag    bool
	TagName      string
}

// PostProcessingRequest contains parameters for post-processing operations.
type PostProcessingRequest struct {
	OrgName  string
	RepoName string
	BaseRef  string
}

// ProcessPostRelease executes the post-release workflow for the given request, performing branch merges,
// tag creation, and protected branch updates as needed. Returns a slice of PostReleaseResponse results.
func ProcessPostRelease(ctx *context.Context, request PostReleaseRequest) []model.PostReleaseResponse {
	responses := make([]model.PostReleaseResponse, 0)
	processor.ProcessRepositoriesOperation(ctx, request.OrgName, request.RepoNames, request.ExcludeRepos, processor.OperationPostRelease,
		func(ctx *context.Context, orgName, repoName string) (model.PostReleaseResponse, error) {
			// lock the Head branch and add the required status checks
			pb.UpdateProtectedBranchForRepo(ctx, repoName, pb.ProtectedBranchRequest{OrgName: orgName, BranchName: request.HeadRef, Lock: true, RemoveStatus: false}, model.ProtectedBranch{})
			// unlock the Base branch and remove the required status checks
			pb.UpdateProtectedBranchForRepo(ctx, repoName, pb.ProtectedBranchRequest{OrgName: orgName, BranchName: request.BaseRef, Lock: false, RemoveStatus: true}, model.ProtectedBranch{})

			prResponse, err := pr.CreateNewPullRequestForRepo(ctx, pr.PullRequestRequest{OrgName: orgName, RepoName: repoName, BaseRef: request.BaseRef, HeadRef: request.HeadRef, Title: request.Title, Body: request.Body}, false)
			if err != nil {
				postProcessing(ctx, PostProcessingRequest{OrgName: request.OrgName, RepoName: repoName, BaseRef: request.BaseRef})
				return model.PostReleaseResponse{RepositoryName: repoName}, fmt.Errorf("failed to create pull request: %w", err)
			}
			// merge to main/develop
			mrResponse := pr.MergePullRequest(ctx, pr.PRMergeRequest{OrgName: orgName, RepoName: repoName, PRNumber: prResponse.PRNumber, Title: request.Title, Body: request.Body})
			if mrResponse.ErrorMessage != "" {
				postProcessing(ctx, PostProcessingRequest{OrgName: request.OrgName, RepoName: repoName, BaseRef: request.BaseRef})
				return model.PostReleaseResponse{RepositoryName: repoName}, fmt.Errorf("failed to merge pull request: %s", mrResponse.ErrorMessage)
			}

			// create tag
			if request.CreateTag {
				tagReq := tag.TagCreateSingleRequest{
					OrgName:          orgName,
					RepoName:         repoName,
					ExcludeRepoNames: []string{},
					TagName:          request.TagName,
					RefBranchName:    request.BaseRef,
					Message:          request.Title,
				}
				tagResponse, err := tag.CreateNewTag(ctx, tagReq)
				if err != nil {
					postProcessing(ctx, PostProcessingRequest{OrgName: request.OrgName, RepoName: repoName, BaseRef: request.BaseRef})
					return model.PostReleaseResponse{RepositoryName: repoName}, fmt.Errorf("failed to create tag: %w", err)
				}
				return model.PostReleaseResponse{RepositoryName: repoName, PRNumber: prResponse.PRNumber, PRHtmlUrl: prResponse.HTMLUrl, TagHtmlUrl: tagResponse.Url, TagCommitSHA: tagResponse.Object.SHA}, nil
			}

			postProcessing(ctx, PostProcessingRequest{OrgName: request.OrgName, RepoName: repoName, BaseRef: request.BaseRef})

			return model.PostReleaseResponse{RepositoryName: repoName, PRNumber: prResponse.PRNumber, PRHtmlUrl: prResponse.HTMLUrl}, nil
		}, func(repoName string, result processor.RepoOperationResult[model.PostReleaseResponse]) {
			responses = append(responses, result.Result)
		},
		func(repoName string, err error) {
			responses = append(responses, model.PostReleaseResponse{RepositoryName: repoName, ErrorMessage: fmt.Sprintf("failed to process post-release: %v", err)})
		})
	return responses
}

func postProcessing(ctx *context.Context, req PostProcessingRequest) {
	// lock the Base branch and add the required status checks
	pb.UpdateProtectedBranchForRepo(ctx, req.RepoName, pb.ProtectedBranchRequest{OrgName: req.OrgName, BranchName: req.BaseRef, Lock: true, RemoveStatus: false}, model.ProtectedBranch{})
}
