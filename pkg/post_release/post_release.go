package postrelease

import (
	"errors"

	"github.com/prady-lab/sgh-cli/pkg/context"
	logger "github.com/prady-lab/sgh-cli/utils"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
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

	// lock the Head branch and add the required status checks
	logger.Glog.Info().Msgf("locking the Head branch %s and adding the required status checks", request.HeadRef)
	pb.UpdateProtectedBranch(ctx, request.OrgName, request.RepoNames, request.HeadRef, true, false)

	logger.Glog.Info().Msgf("unlocking the Base branch %s and removing the required status checks", request.BaseRef)
	// unlock the Base branch and remove the required status checks
	pb.UpdateProtectedBranch(ctx, request.OrgName, request.RepoNames, request.BaseRef, false, true)

	processor.ProcessRepositoriesOperation(ctx, request.OrgName, request.RepoNames, processor.OperationPostRelease,
		func(ctx *context.Context, orgName, repoName string) (model.PostReleaseResponse, error) {
			prResponse, err := pr.CreateNewPullRequestForRepo(ctx, orgName, repoName, request.BaseRef, request.HeadRef, request.Title, request.Body, false)
			if err != nil {
				return model.PostReleaseResponse{RepositoryName: repoName}, err
			}
			// merge to main/develop
			mrResponse := pr.MergePullRequest(ctx, orgName, repoName, prResponse.PRNumber, request.Title, request.Body)
			if mrResponse.ErrorMessage != "" {
				return model.PostReleaseResponse{RepositoryName: repoName}, errors.New(mrResponse.ErrorMessage)
			}

			// create tag
			if request.CreateTag {
				tagResponse, err := tag.CreateNewTag(ctx, orgName, repoName, request.TagName, request.BaseRef, request.Title)
				if err != nil {
					return model.PostReleaseResponse{RepositoryName: repoName}, err
				}
				return model.PostReleaseResponse{RepositoryName: repoName, PRNumber: prResponse.PRNumber, PRHtmlUrl: prResponse.HTMLUrl, TagHtmlUrl: tagResponse.Url, TagCommitSHA: tagResponse.Object.SHA}, nil
			}

			return model.PostReleaseResponse{RepositoryName: repoName, PRNumber: prResponse.PRNumber, PRHtmlUrl: prResponse.HTMLUrl}, nil
		}, func(repoName string, result processor.RepoOperationResult[model.PostReleaseResponse]) {
			responses = append(responses, result.Result)
		},
		func(repoName string, err error) {
			responses = append(responses, model.PostReleaseResponse{RepositoryName: repoName, ErrorMessage: err.Error()})
		})

	// lock the Base branch and add the required status checks
	logger.Glog.Info().Msgf("locking the Base branch %s and adding the required status checks", request.BaseRef)
	pb.UpdateProtectedBranch(ctx, request.OrgName, request.RepoNames, request.BaseRef, true, false)
	return responses
}
