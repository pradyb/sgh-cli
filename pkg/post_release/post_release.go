package postrelease

import (
	"errors"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/pr"
	pb "github.com/prady-lab/sgh-cli/pkg/protected_branch"
	"github.com/prady-lab/sgh-cli/pkg/tag"
)

type PostReleaseRequest struct {
	OrgName   string
	RepoNames []string
	BaseRef   string
	HeadRef   string
	Title     string
	Body      string
	CreateTag bool
	TagName   string
}

func ProcessPostRelease(ctx *context.Context, request PostReleaseRequest) []model.PostReleaseResponse {
	responses := make([]model.PostReleaseResponse, 0)
	processor.ProcessRepositoriesOperation(ctx, request.OrgName, request.RepoNames, []string{}, processor.OperationPostRelease,
		func(ctx *context.Context, orgName, repoName string) (model.PostReleaseResponse, error) {
			// lock the Head branch and add the required status checks
			pb.UpdateProtectedBranchForRepo(ctx, repoName, pb.ProtectedBranchRequest{OrgName: orgName, BranchName: request.HeadRef, Lock: true, RemoveStatus: false, AddUsers: nil, RemoveUsers: nil}, model.ProtectedBranch{})
			// unlock the Base branch and remove the required status checks
			pb.UpdateProtectedBranchForRepo(ctx, repoName, pb.ProtectedBranchRequest{OrgName: orgName, BranchName: request.BaseRef, Lock: false, RemoveStatus: true, AddUsers: nil, RemoveUsers: nil}, model.ProtectedBranch{})

			prResponse, err := pr.CreateNewPullRequestForRepo(ctx, pr.PullRequestRequest{OrgName: orgName, RepoName: repoName, BaseRef: request.BaseRef, HeadRef: request.HeadRef, Title: request.Title, Body: request.Body}, false)
			if err != nil {
				postProcessing(ctx, request.OrgName, repoName, request.BaseRef)
				return model.PostReleaseResponse{RepositoryName: repoName}, err
			}
			// merge to main/develop
			mrResponse := pr.MergePullRequest(ctx, orgName, repoName, prResponse.PRNumber, request.Title, request.Body)
			if mrResponse.ErrorMessage != "" {
				postProcessing(ctx, request.OrgName, repoName, request.BaseRef)
				return model.PostReleaseResponse{RepositoryName: repoName}, errors.New(mrResponse.ErrorMessage)
			}

			// create tag
			if request.CreateTag {
				tagResponse, err := tag.CreateNewTag(ctx, orgName, repoName, []string{}, request.TagName, request.BaseRef, request.Title)
				if err != nil {
					postProcessing(ctx, request.OrgName, repoName, request.BaseRef)
					return model.PostReleaseResponse{RepositoryName: repoName}, err
				}
				return model.PostReleaseResponse{RepositoryName: repoName, PRNumber: prResponse.PRNumber, PRHtmlUrl: prResponse.HTMLUrl, TagHtmlUrl: tagResponse.Url, TagCommitSHA: tagResponse.Object.SHA}, nil
			}

			postProcessing(ctx, request.OrgName, repoName, request.BaseRef)

			return model.PostReleaseResponse{RepositoryName: repoName, PRNumber: prResponse.PRNumber, PRHtmlUrl: prResponse.HTMLUrl}, nil
		}, func(repoName string, result processor.RepoOperationResult[model.PostReleaseResponse]) {
			responses = append(responses, result.Result)
		},
		func(repoName string, err error) {
			responses = append(responses, model.PostReleaseResponse{RepositoryName: repoName, ErrorMessage: err.Error()})
		})
	return responses
}

func postProcessing(ctx *context.Context, orgName, repoName, baseRef string) {
	// lock the Base branch and add the required status checks
	pb.UpdateProtectedBranchForRepo(ctx, repoName, pb.ProtectedBranchRequest{OrgName: orgName, BranchName: baseRef, Lock: true, RemoveStatus: false, AddUsers: nil, RemoveUsers: nil}, model.ProtectedBranch{})
}
