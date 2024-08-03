package pr

import (
	"strings"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/repo"
	logger "github.com/prady-lab/sgh-cli/utils"
)

func CreateNewPR(ctx *context.Context, title, body, orgName, targetBranch, sourceBranch string, repos []string) []model.PRResponse {
	repoNames := make([]string, 0)
	if len(repos) == 0 {
		logger.Glog.Info().Msgf("Creating new pull request for all configured repositories in %s", orgName)
		orgRepoNames, err := repo.GetSelectedRepoNames(ctx, orgName)
		if err != nil {
			logger.Glog.Error().Err(err).Msgf("Error in getting the repositories for the organization %s", orgName)
			return nil
		}
		repoNames = append(repoNames, orgRepoNames...)
	} else {
		actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, repos)
		logger.Glog.Info().Str("repos", strings.Join(actualRepoNames, ",")).Msgf("Creating new pull request for selected repositories in %s", orgName)
		repoNames = append(repoNames, actualRepoNames...)
	}
	return createNewPRs(ctx, orgName, title, body, targetBranch, sourceBranch, repoNames)
}

func createNewPRs(ctx *context.Context, orgName, title, body, targetBranch, sourceBranch string, repoNames []string) []model.PRResponse {
	/*jobQueue := async.NewASyncJobQueue(len(repoNames))
	for i, repoName := range repoNames {
		jobQueue.AddJob(async.ASyncJob{Id: i, JobData: repoName})
	}
	jobQueue.Close()

	responses := make([]model.PRResponse, 0)

	bar := ui.NewProgressBar(len(repoNames), fmt.Sprintf("Creating pull requests for org %s...", orgName))

	jobQueue.Start(func(job async.ASyncJob) (interface{}, error) {
		repoName := job.JobData.(string)
		prResponse, err := service.CreateNewPullRequest(ctx, orgName, repoName, title, body, targetBranch, sourceBranch)
		if err != nil {
			return nil, err
		}
		assignees := ctx.Config.PullRequestAssignees(orgName)
		if len(assignees) > 0 {
			service.AddIssueAssignees(ctx, orgName, repoName, prResponse.PRNumber, assignees)
		}
		return service.GetPullRequestInfo(ctx, orgName, repoName, prResponse.PRNumber)
	}, func(result async.ASyncJobResult) {
		bar.Add(1)
		responses = append(responses, result.Result.(model.PRResponse))
	}, func(err async.ASyncJobError) {
		bar.Add(1)
		var ge *apperrors.GitHubError
		if err.Error != nil && errors.As(err.Error, &ge) {
			responses = append(responses, model.PRResponse{ErrorMessage: ge.Message})
		} else {
			responses = append(responses, model.PRResponse{ErrorMessage: err.Error.Error()})
		}
	})
	return responses*/
	return nil
}

func ListPRs(ctx *context.Context, orgName string, repos []string) []model.PRResponse {
	repoNames := make([]string, 0)
	if len(repos) == 0 {
		logger.Glog.Info().Msgf("Listing pull request for all configured repositories in %s", orgName)
		orgRepoNames, err := repo.GetSelectedRepoNames(ctx, orgName)
		if err != nil {
			logger.Glog.Error().Err(err).Msgf("Error in getting the repositories for the organization %s", orgName)
			return nil
		}
		repoNames = append(repoNames, orgRepoNames...)
	} else {
		actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, repos)
		logger.Glog.Info().Str("repos", strings.Join(actualRepoNames, ",")).Msgf("Listing pull requests for selected repositories in %s", orgName)
		repoNames = append(repoNames, actualRepoNames...)
	}
	return nil
}
