package tag

import (
	"strings"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/repo"
	logger "github.com/prady-lab/sgh-cli/utils"
)

func CreateNewTags(ctx *context.Context, orgName, tagName, refBranchName string, repos []string, message string) []model.CommonResponse {
	repoNames := make([]string, 0)
	if len(repos) == 0 {
		logger.Glog.Info().Msgf("Creating new tag for all configured repositories in %s", orgName)
		orgRepoNames, err := repo.GetSelectedRepoNames(ctx, orgName)
		if err != nil {
			logger.Glog.Error().Err(err).Msgf("Error in getting the repositories for the organization %s", orgName)
			return nil
		}
		repoNames = append(repoNames, orgRepoNames...)
	} else {
		actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, repos)
		logger.Glog.Info().Str("repos", strings.Join(actualRepoNames, ",")).Msgf("Creating new tag for selected repositories in %s", orgName)
		repoNames = append(repoNames, actualRepoNames...)
	}
	return createNewTags(ctx, orgName, tagName, refBranchName, repoNames, message)
}

func createNewTags(ctx *context.Context, orgName, tagName, refBranchName string, repoNames []string, message string) []model.CommonResponse {
	/*jobQueue := async.NewASyncJobQueue(len(repoNames))
	for i, repoName := range repoNames {
		jobQueue.AddJob(async.ASyncJob{Id: i, JobData: repoName})
	}
	jobQueue.Close()

	responses := make([]model.CommonResponse, 0)

	bar := ui.NewProgressBar(len(repoNames), fmt.Sprintf("Creating Tags for org %s...", orgName))

	jobQueue.Start(func(job async.ASyncJob) (interface{}, error) {
		repoName := job.JobData.(string)
		return service.CreateNewTag(ctx, orgName, repoName, tagName, refBranchName, message)
	}, func(result async.ASyncJobResult) {
		bar.Add(1)
		responses = append(responses, model.CommonResponse{OrgName: orgName, RepositoryName: result.JobData.(string), ItemName: tagName, ItemType: "TAG", SuccessMessage: result.Result.(model.NewItemResponse).Object.SHA})
	}, func(err async.ASyncJobError) {
		bar.Add(1)
		var ge *apperrors.GitHubError
		if err.Error != nil && errors.As(err.Error, &ge) {
			responses = append(responses, model.CommonResponse{OrgName: orgName, RepositoryName: err.JobData.(string), ItemName: tagName, ItemType: "TAG", ErrorMessage: ge.Message})
		} else {
			responses = append(responses, model.CommonResponse{OrgName: orgName, RepositoryName: err.JobData.(string), ItemName: tagName, ItemType: "TAG", ErrorMessage: err.Error.Error()})
		}
	})

	return responses*/
	return nil
}

func DeleteTags(ctx *context.Context, orgName, tagName string, repos []string) []model.CommonResponse {
	repoNames := make([]string, 0)
	if len(repos) == 0 {
		logger.Glog.Info().Msgf("Deleting tag for all configured repositories in %s", orgName)
		orgRepoNames, err := repo.GetSelectedRepoNames(ctx, orgName)
		if err != nil {
			logger.Glog.Error().Err(err).Msgf("Error in getting the repositories for the organization %s", orgName)
			return nil
		}
		repoNames = append(repoNames, orgRepoNames...)
	} else {
		actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, repos)
		logger.Glog.Info().Str("repos", strings.Join(actualRepoNames, ",")).Msgf("Deleting tag for selected repositories in %s", orgName)
		repoNames = append(repoNames, actualRepoNames...)
	}
	return deleteTags(ctx, orgName, tagName, repoNames)
}

func deleteTags(ctx *context.Context, orgName, tagName string, repoNames []string) []model.CommonResponse {
	/*jobQueue := async.NewASyncJobQueue(len(repoNames))
	for i, repoName := range repoNames {
		jobQueue.AddJob(async.ASyncJob{Id: i, JobData: repoName})
	}
	jobQueue.Close()

	responses := make([]model.CommonResponse, 0)

	bar := ui.NewProgressBar(len(repoNames), fmt.Sprintf("Deleting tags for org %s...", orgName))

	jobQueue.Start(func(job async.ASyncJob) (interface{}, error) {
		repoName := job.JobData.(string)
		return service.DeleteTag(ctx, orgName, repoName, tagName)
	}, func(result async.ASyncJobResult) {
		bar.Add(1)
		responses = append(responses, model.CommonResponse{OrgName: orgName, RepositoryName: result.JobData.(string), ItemName: tagName, ItemType: "TAG", SuccessMessage: "Tag deleted"})
	}, func(err async.ASyncJobError) {
		bar.Add(1)
		var ge *apperrors.GitHubError
		if err.Error != nil && errors.As(err.Error, &ge) {
			responses = append(responses, model.CommonResponse{OrgName: orgName, RepositoryName: err.JobData.(string), ItemName: tagName, ItemType: "TAG", ErrorMessage: ge.Message})
		} else {
			responses = append(responses, model.CommonResponse{OrgName: orgName, RepositoryName: err.JobData.(string), ItemName: tagName, ItemType: "TAG", ErrorMessage: err.Error.Error()})
		}
	})
	return responses*/
	return nil
}
