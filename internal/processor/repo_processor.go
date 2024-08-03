package processor

import (
	"fmt"
	"strings"

	"github.com/prady-lab/sgh-cli/internal/async"
	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/repo"
	"github.com/prady-lab/sgh-cli/pkg/ui"
	logger "github.com/prady-lab/sgh-cli/utils"
)

type OperationDataType interface {
	BranchOperationData
}
type OperationResultType interface {
	model.NewItemResponse | bool
}

type RepoOperationData[T OperationDataType] struct {
	OperationType  string
	Message        string
	AdditionalData T
}
type RepoOperationResult[R OperationResultType] struct {
	OperationType string
	Result        R
}

type RepoOperationHandler[T OperationDataType, R OperationResultType] func(ctx *context.Context, orgName, repoName string, additionalData RepoOperationData[T]) (R, error)
type RepoOperationResultHandler[T OperationDataType, R OperationResultType] func(repoName string, additionalData RepoOperationData[T], result RepoOperationResult[R])
type RepoOperationErrorHandler[T OperationDataType] func(repoName string, additionalData RepoOperationData[T], err error)

type BranchOperationData struct {
	NewBranchName string
	RefBranchName string
}

func ProcessRepositoriesOperation[T OperationDataType, R OperationResultType](ctx *context.Context, orgName string, repos []string, additionalData RepoOperationData[T], operationHandler RepoOperationHandler[T, R], resultHandler RepoOperationResultHandler[T, R], errorHandler RepoOperationErrorHandler[T]) error {
	repoNames := make([]string, 0)
	if len(repos) == 0 {
		logger.Glog.Info().Msgf("%s for all configured repositories in %s", additionalData.Message, orgName)
		orgRepoNames, err := repo.GetSelectedRepoNames(ctx, orgName)
		if err != nil {
			logger.Glog.Error().Err(err).Msgf("Error in getting the Repos for the organization %s", orgName)
			return err
		}
		repoNames = append(repoNames, orgRepoNames...)
	} else {
		actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, repos)
		logger.Glog.Info().Str("repos", strings.Join(actualRepoNames, ",")).Msgf("%s for selected repositories in %s", additionalData.Message, orgName)
		repoNames = append(repoNames, actualRepoNames...)
	}
	return process(ctx, orgName, repoNames, additionalData, operationHandler, resultHandler, errorHandler)
}

func process[T OperationDataType, R OperationResultType](ctx *context.Context, orgName string, repoNames []string, additionalData RepoOperationData[T], operationHandler RepoOperationHandler[T, R], resultHandler RepoOperationResultHandler[T, R], errorHandler RepoOperationErrorHandler[T]) error {
	jobQueue := async.NewASyncJobQueue[any, any](len(repoNames))
	for i, repoName := range repoNames {
		jobQueue.AddJob(async.ASyncJob[any]{Id: i, JobData: repoName})
	}
	jobQueue.Close()

	bar := ui.NewProgressBar(len(repoNames), fmt.Sprintf("%s for org %s...", additionalData.Message, orgName))

	jobQueue.Start(func(job async.ASyncJob[any]) (interface{}, error) {
		return operationHandler(ctx, orgName, job.JobData.(string), additionalData)
	}, func(result async.ASyncJobResult[any, any]) {
		bar.Add(1)
		resultHandler(result.JobData.(string), additionalData, RepoOperationResult[R]{OperationType: additionalData.OperationType, Result: result.Result.(R)})
	}, func(err async.ASyncJobError[any]) {
		bar.Add(1)
		errorHandler(err.JobData.(string), additionalData, err.Error)
	})
	return nil
}
