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

type OperationEnum int

const (
	OperationCreateBranch OperationEnum = iota
	OperationDeleteBranch
	OperationCreateTag
	OperationDeleteTag
	OperationCreatePullRequest
	OperationListPullRequest
	OperationUpdatePullRequest
	OperationMergePullRequest
	OperationListProtectedBranch
	OperationUpdateProtectedBranch
	OperationDeleteProtectedBranch
)

var RepoOperationConfig = map[OperationEnum]map[string]string{
	OperationCreateBranch: {
		"message": "Creating Branch",
	},
	OperationDeleteBranch: {
		"message": "Deleting Branch",
	},
	OperationCreateTag: {
		"message": "Creating Tag",
	},
	OperationDeleteTag: {
		"message": "Deleting Tag",
	},
	OperationCreatePullRequest: {
		"message": "Creating Pull Request",
	},
	OperationListPullRequest: {
		"message": "Listing Pull Requests",
	},
	OperationUpdatePullRequest: {
		"message": "Updating Pull Request",
	},
	OperationMergePullRequest: {
		"message": "Merging Pull Request",
	},
	OperationListProtectedBranch: {
		"message": "Listing Protected Branch",
	},
	OperationUpdateProtectedBranch: {
		"message": "Updating Protected Branch",
	},
	OperationDeleteProtectedBranch: {
		"message": "Deleting Protected Branch",
	},
}

type OperationResultType interface {
	bool | model.RefResponse | model.PullRequestResponse | []model.PullRequestResponse | model.ProtectedBranch | []model.ProtectedBranch
}

type RepoOperationResult[R OperationResultType] struct {
	OperationType string
	Result        R
}

type RepoOperationHandler[R OperationResultType] func(ctx *context.Context, orgName, repoName string) (R, error)
type RepoOperationResultHandler[R OperationResultType] func(repoName string, result RepoOperationResult[R])
type RepoOperationErrorHandler func(repoName string, err error)

type BranchOperationData struct {
	NewBranchName string
	RefBranchName string
}

func ProcessRepositoriesOperation[R OperationResultType](ctx *context.Context, orgName string, repos []string, operation OperationEnum, operationHandler RepoOperationHandler[R], resultHandler RepoOperationResultHandler[R], errorHandler RepoOperationErrorHandler) error {
	repoNames := make([]string, 0)
	message := RepoOperationConfig[operation]["message"]

	if len(repos) == 0 {
		logger.Glog.Info().Msgf("%s for all configured repositories in %s", message, orgName)
		orgRepoNames, err := repo.GetSelectedRepoNames(ctx, orgName)
		if err != nil {
			logger.Glog.Error().Err(err).Msgf("Error in getting the Repos for the organization %s", orgName)
			return err
		}
		repoNames = append(repoNames, orgRepoNames...)
	} else {
		actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, repos)
		logger.Glog.Info().Str("repos", strings.Join(actualRepoNames, ",")).Msgf("%s for selected repositories in %s", message, orgName)
		repoNames = append(repoNames, actualRepoNames...)
	}
	return process(ctx, orgName, repoNames, operation, operationHandler, resultHandler, errorHandler)
}

func process[R OperationResultType](ctx *context.Context, orgName string, repoNames []string, operation OperationEnum, operationHandler RepoOperationHandler[R], resultHandler RepoOperationResultHandler[R], errorHandler RepoOperationErrorHandler) error {
	jobQueue := async.NewASyncJobQueue[any, any](len(repoNames))
	message := RepoOperationConfig[operation]["message"]

	bar := ui.NewProgressBar(len(repoNames), fmt.Sprintf("%s for org %s...", message, orgName))

	for i, repoName := range repoNames {
		jobQueue.AddJob(async.ASyncJob[any]{Id: i, JobData: repoName})
	}
	jobQueue.Close()

	jobQueue.Start(
		func(job async.ASyncJob[any]) (interface{}, error) {
			response, err := operationHandler(ctx, orgName, job.JobData.(string))
			bar.Describe(fmt.Sprintf("Processed %s", job.JobData.(string)))
			bar.Add(1)
			return response, err
		}, func(result async.ASyncJobResult[any, any]) {
			resultHandler(result.JobData.(string), RepoOperationResult[R]{Result: result.Result.(R)})
		}, func(err async.ASyncJobError[any]) {
			errorHandler(err.JobData.(string), err.Error)
		})
	return nil
}
