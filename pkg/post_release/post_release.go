package postrelease

import (
	"errors"

	"github.com/prady-lab/sgh-cli/pkg/context"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/pkg/pr"
	"github.com/prady-lab/sgh-cli/pkg/tag"
)

func ProcessPostRelease(ctx *context.Context, orgName string, repoNames []string, baseRef, headRef, title, body string, createTag bool, tagName string) []model.PostReleaseResponse {

	responses := make([]model.PostReleaseResponse, 0)

	processor.ProcessRepositoriesOperation(ctx, orgName, repoNames, processor.OperationPostRelease,
		func(ctx *context.Context, orgName, repoName string) (model.PostReleaseResponse, error) {

			prResponse, err := pr.CreateNewPullRequestForRepo(ctx, orgName, repoName, baseRef, headRef, title, body)
			if err != nil {
				return model.PostReleaseResponse{RepositoryName: repoName}, err
			}
			// merge to main/develop
			mrResponse := pr.MergePullRequest(ctx, orgName, repoName, prResponse.PRNumber, title, body)
			if mrResponse.ErrorMessage != "" {
				return model.PostReleaseResponse{RepositoryName: repoName}, errors.New(mrResponse.ErrorMessage)
			}

			// create tag
			if createTag {
				tagResponse, err := tag.CreateNewTag(ctx, orgName, repoName, tagName, baseRef, title)
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
	return responses
}
