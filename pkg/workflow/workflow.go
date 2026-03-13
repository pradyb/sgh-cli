// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package workflow

import (
	"fmt"
	"strings"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
)

type WorkflowListRequest struct {
	OrgName          string
	RepoNames        []string
	ExcludeRepoNames []string
	Branch           string
	Status           string
	Count            int
	WorkflowName     string
}

type WorkflowRunRequest struct {
	OrgName  string
	RepoName string
	RunID    int
}

type WorkflowDispatchRequest struct {
	OrgName          string
	RepoNames        []string
	ExcludeRepoNames []string
	WorkflowID       string
	Ref              string
	Inputs           map[string]string
}

type WorkflowDispatchResult struct {
	RepositoryName string
	WorkflowID     string
	Ref            string
	ErrorMessage   string
}

func DispatchWorkflow(ctx *context.Context, req WorkflowDispatchRequest) []WorkflowDispatchResult {
	results := make([]WorkflowDispatchResult, 0)

	processor.ProcessRepositoriesOperation(ctx, req.OrgName, req.RepoNames, req.ExcludeRepoNames, processor.OperationListWorkflowRuns,
		func(ctx *context.Context, orgName, repoName string) (bool, error) {
			return true, service.DispatchWorkflow(ctx, orgName, repoName, req.WorkflowID, req.Ref, req.Inputs)
		},
		func(repoName string, _ processor.RepoOperationResult[bool]) {
			results = append(results, WorkflowDispatchResult{
				RepositoryName: repoName,
				WorkflowID:     req.WorkflowID,
				Ref:            req.Ref,
			})
		},
		func(repoName string, err error) {
			results = append(results, WorkflowDispatchResult{
				RepositoryName: repoName,
				WorkflowID:     req.WorkflowID,
				Ref:            req.Ref,
				ErrorMessage:   fmt.Sprintf("failed to dispatch workflow: %v", err),
			})
		})
	return results
}

func ListWorkflowRuns(ctx *context.Context, req WorkflowListRequest) []model.WorkflowRun {
	responses := make([]model.WorkflowRun, 0)

	processor.ProcessRepositoriesOperation(ctx, req.OrgName, req.RepoNames, req.ExcludeRepoNames, processor.OperationListWorkflowRuns,
		func(ctx *context.Context, orgName, repoName string) ([]model.WorkflowRun, error) {
			runs, err := service.ListWorkflowRuns(ctx, orgName, repoName, req.Branch, req.Status, req.Count)
			if err != nil {
				return nil, err
			}
			for i := range runs {
				runs[i].RepositoryName = repoName
			}
			return runs, nil
		},
		func(repoName string, result processor.RepoOperationResult[[]model.WorkflowRun]) {
			for _, run := range result.Result {
				if req.WorkflowName != "" && !strings.Contains(strings.ToLower(run.Name), strings.ToLower(req.WorkflowName)) {
					continue
				}
				responses = append(responses, run)
			}
		},
		func(repoName string, err error) {
			responses = append(responses, model.WorkflowRun{
				RepositoryName: repoName,
				ErrorMessage:   fmt.Sprintf("failed to list workflow runs: %v", err),
			})
		})

	return responses
}

func RerunWorkflowRun(ctx *context.Context, req WorkflowRunRequest) model.WorkflowRun {
	repoName := req.RepoName

	_, err := service.RerunWorkflowRun(ctx, req.OrgName, repoName, req.RunID)
	if err != nil {
		logger.Glog.Error().Err(err).Str("repo", repoName).Int("runID", req.RunID).Msg("Failed to rerun workflow")
		return model.WorkflowRun{
			RepositoryName: repoName,
			ID:             req.RunID,
			ErrorMessage:   fmt.Sprintf("failed to rerun workflow: %v", err),
		}
	}
	return model.WorkflowRun{
		RepositoryName: repoName,
		ID:             req.RunID,
		Status:         "rerun_requested",
	}
}

func GetLatestRunID(ctx *context.Context, orgName, repoName string) (int, error) {
	runs, err := service.ListWorkflowRuns(ctx, orgName, repoName, "", "", 1)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch workflow runs: %w", err)
	}
	if len(runs) == 0 {
		return 0, fmt.Errorf("no workflow runs found for %s/%s", orgName, repoName)
	}
	return runs[0].ID, nil
}

func GetWorkflowRunDetail(ctx *context.Context, req WorkflowRunRequest) model.WorkflowRunDetail {
	repoName := req.RepoName

	run, err := service.GetWorkflowRun(ctx, req.OrgName, repoName, req.RunID)
	if err != nil {
		logger.Glog.Error().Err(err).Str("repo", repoName).Int("runID", req.RunID).Msg("Failed to get workflow run")
		return model.WorkflowRunDetail{
			Run:          model.WorkflowRun{RepositoryName: repoName, ID: req.RunID},
			ErrorMessage: fmt.Sprintf("failed to get workflow run: %v", err),
		}
	}
	run.RepositoryName = repoName

	jobs, err := service.GetWorkflowRunJobs(ctx, req.OrgName, repoName, req.RunID)
	if err != nil {
		logger.Glog.Error().Err(err).Str("repo", repoName).Int("runID", req.RunID).Msg("Failed to get workflow jobs")
		return model.WorkflowRunDetail{
			Run:          run,
			ErrorMessage: fmt.Sprintf("failed to get workflow jobs: %v", err),
		}
	}

	return model.WorkflowRunDetail{
		Run:  run,
		Jobs: jobs,
	}
}

func CancelWorkflowRun(ctx *context.Context, req WorkflowRunRequest) model.WorkflowRun {
	repoName := req.RepoName

	_, err := service.CancelWorkflowRun(ctx, req.OrgName, repoName, req.RunID)
	if err != nil {
		logger.Glog.Error().Err(err).Str("repo", repoName).Int("runID", req.RunID).Msg("Failed to cancel workflow")
		return model.WorkflowRun{
			RepositoryName: repoName,
			ID:             req.RunID,
			ErrorMessage:   fmt.Sprintf("failed to cancel workflow: %v", err),
		}
	}
	return model.WorkflowRun{
		RepositoryName: repoName,
		ID:             req.RunID,
		Status:         "cancel_requested",
	}
}
