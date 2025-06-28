package processor

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/prady-lab/sgh-cli/internal/async"
	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/prady-lab/sgh-cli/pkg/repo"
	"github.com/prady-lab/sgh-cli/pkg/ui"
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
	OperationReviewPullRequest
	OperationMergePullRequest
	OperationListProtectedBranch
	OperationUpdateProtectedBranch
	OperationDeleteProtectedBranch
	OperationPostRelease
	OperationListCommits
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
	OperationReviewPullRequest: {
		"message": "Reviewing Pull Request",
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
	OperationPostRelease: {
		"message": "Post Release",
	},
	OperationListCommits: {
		"message": "Listing Commits",
	},
}

type OperationResultType interface {
	bool | model.RefResponse | model.PullRequestResponse | []model.PullRequestResponse | model.ProtectedBranch | []model.ProtectedBranch | model.PostReleaseResponse | []model.PostReleaseResponse | model.CommitResponse | []model.CommitResponse | model.ReviewPullRequestResponse | []model.ReviewPullRequestResponse | model.MergeResponse | []model.MergeResponse
}

type RepoOperationResult[R OperationResultType] struct {
	OperationType string
	Result        R
}

type (
	RepoOperationHandler[R OperationResultType]       func(ctx *context.Context, orgName, repoName string) (R, error)
	RepoOperationResultHandler[R OperationResultType] func(repoName string, result RepoOperationResult[R])
	RepoOperationErrorHandler                         func(repoName string, err error)
)

type BranchOperationData struct {
	NewBranchName string
	RefBranchName string
}

// String returns a string representation of the operation enum
func (o OperationEnum) String() string {
	switch o {
	case OperationCreateBranch:
		return "CreateBranch"
	case OperationDeleteBranch:
		return "DeleteBranch"
	case OperationCreateTag:
		return "CreateTag"
	case OperationDeleteTag:
		return "DeleteTag"
	case OperationCreatePullRequest:
		return "CreatePullRequest"
	case OperationListPullRequest:
		return "ListPullRequest"
	case OperationUpdatePullRequest:
		return "UpdatePullRequest"
	case OperationReviewPullRequest:
		return "ReviewPullRequest"
	case OperationMergePullRequest:
		return "MergePullRequest"
	case OperationListProtectedBranch:
		return "ListProtectedBranch"
	case OperationUpdateProtectedBranch:
		return "UpdateProtectedBranch"
	case OperationDeleteProtectedBranch:
		return "DeleteProtectedBranch"
	case OperationPostRelease:
		return "PostRelease"
	case OperationListCommits:
		return "ListCommits"
	default:
		return fmt.Sprintf("UnknownOperation(%d)", o)
	}
}

func ProcessRepositoriesOperation[R OperationResultType](ctx *context.Context, orgName string, repos, excludeRepos []string, operation OperationEnum, operationHandler RepoOperationHandler[R], resultHandler RepoOperationResultHandler[R], errorHandler RepoOperationErrorHandler) error {
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

	// Exclude repositories
	filteredRepoNames := make([]string, 0)
	actualExcludeRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, excludeRepos)
	for _, repoName := range repoNames {
		if !slices.Contains(actualExcludeRepoNames, repoName) {
			filteredRepoNames = append(filteredRepoNames, repoName)
		}
	}

	if len(filteredRepoNames) == 0 {
		logger.Glog.Info().Msgf("No repositories to process")
		return nil
	}

	return process(ctx, orgName, filteredRepoNames, operation, operationHandler, resultHandler, errorHandler)
}

func process[R OperationResultType](ctx *context.Context, orgName string, repoNames []string, operation OperationEnum, operationHandler RepoOperationHandler[R], resultHandler RepoOperationResultHandler[R], errorHandler RepoOperationErrorHandler) error {
	startTime := time.Now()
	jobQueue := async.NewAsyncJobQueue[any, any](len(repoNames))
	message := RepoOperationConfig[operation]["message"]

	bar := ui.NewProgressBar(len(repoNames), fmt.Sprintf("%s for org %s...", message, orgName))

	// Pre-allocate slices for better performance
	successCount := 0
	errorCount := 0
	var mu sync.Mutex

	for i, repoName := range repoNames {
		jobQueue.AddJob(async.AsyncJob[any]{ID: i, JobData: repoName})
	}
	jobQueue.Close()

	noOfWorkers := ctx.Config.NoOfWorkers
	if noOfWorkers == 0 {
		noOfWorkers = 1
	}

	// Log performance metrics
	logger.Glog.Info().
		Str("org", orgName).
		Int("totalRepos", len(repoNames)).
		Int("workers", noOfWorkers).
		Str("operation", operation.String()).
		Msg("Starting repository operation")

	jobQueue.Start(
		func(job async.AsyncJob[any]) (any, error) {
			jobStartTime := time.Now()
			response, err := operationHandler(ctx, orgName, job.JobData.(string))
			jobDuration := time.Since(jobStartTime)

			// Update counters
			mu.Lock()
			if err != nil {
				errorCount++
			} else {
				successCount++
			}
			mu.Unlock()

			// Log performance for individual jobs
			logger.Flog.Debug().
				Str("repo", job.JobData.(string)).
				Dur("duration", jobDuration).
				Bool("success", err == nil).
				Msg("Repository operation completed")

			bar.Describe(fmt.Sprintf("Processed %s", job.JobData.(string)))
			bar.Add(1)
			return response, err
		}, func(result async.AsyncJobResult[any, any]) {
			resultHandler(result.JobData.(string), RepoOperationResult[R]{Result: result.Result.(R)})
		}, func(err async.AsyncJobError[any]) {
			errorHandler(err.JobData.(string), err.Error)
		}, noOfWorkers)

	totalDuration := time.Since(startTime)

	fmt.Println()
	// Log final performance metrics
	logger.Glog.Info().
		Str("org", orgName).
		Int("totalRepos", len(repoNames)).
		Int("successCount", successCount).
		Int("errorCount", errorCount).
		Dur("totalDuration", totalDuration).
		Float64("avgDurationPerRepo", float64(totalDuration.Milliseconds())/float64(len(repoNames))).
		Str("operation", operation.String()).
		Msg("Repository operation completed")

	return nil
}
