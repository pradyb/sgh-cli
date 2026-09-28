// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package workflow

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pradyb/sgh-cli/internal/model"
	"github.com/pradyb/sgh-cli/internal/processor"
	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/pkg/context"
	"github.com/pradyb/sgh-cli/pkg/logger"
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
	return latestRunID(ctx, orgName, repoName, "")
}

func latestRunID(ctx *context.Context, orgName, repoName, status string) (int, error) {
	runs, err := service.ListWorkflowRuns(ctx, orgName, repoName, "", status, 1)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch workflow runs: %w", err)
	}
	if len(runs) == 0 {
		if status != "" {
			return 0, fmt.Errorf("no %s workflow runs found for %s/%s", status, orgName, repoName)
		}
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

type ApproveRequest struct {
	OrgName  string
	RepoName string
	// RunID 0 resolves the latest run waiting on a gate.
	RunID int
	// Environments limits the decision to these gates; empty means every gate the user can review.
	Environments []string
	Reject       bool
	Comment      string
	// Confirm, when set, is asked before anything is sent; returning false aborts.
	Confirm func(ApproveResult) bool
}

type ApproveResult struct {
	Repository string   `json:"repository"`
	RunID      int      `json:"run_id"`
	State      string   `json:"state"`
	Decided    []string `json:"decided,omitempty"`
	Skipped    []string `json:"skipped,omitempty"`
	DryRun     bool     `json:"dry_run,omitempty"`
	Aborted    bool     `json:"aborted,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// ApproveWorkflowRun approves (or rejects) the pending environment gates of a workflow run.
// Gates the current user is not a required reviewer of are reported as Skipped, not failures.
func ApproveWorkflowRun(ctx *context.Context, req ApproveRequest) ApproveResult {
	state, defaultComment := "approved", "Approved via sgh"
	if req.Reject {
		state, defaultComment = "rejected", "Rejected via sgh"
	}
	comment := req.Comment
	if comment == "" {
		comment = defaultComment
	}
	res := ApproveResult{Repository: req.RepoName, RunID: req.RunID, State: state}
	fail := func(format string, a ...any) ApproveResult {
		res.Error = fmt.Sprintf(format, a...)
		logger.Glog.Error().Str("repo", req.RepoName).Int("runID", res.RunID).Msg(res.Error)
		return res
	}

	if res.RunID == 0 {
		id, err := latestRunID(ctx, req.OrgName, req.RepoName, "waiting")
		if err != nil {
			return fail("%v", err)
		}
		res.RunID = id
	}

	gates, err := service.ListPendingDeployments(ctx, req.OrgName, req.RepoName, res.RunID)
	if err != nil {
		return fail("failed to list pending deployments: %v", err)
	}

	var ids []int
	for _, g := range gates {
		name := g.Environment.Name
		if len(req.Environments) > 0 && !slices.Contains(req.Environments, name) {
			continue
		}
		if !g.CurrentUserCanApprove {
			res.Skipped = append(res.Skipped, name)
			continue
		}
		ids = append(ids, g.Environment.ID)
		res.Decided = append(res.Decided, name)
	}
	if len(ids) == 0 {
		if len(res.Skipped) > 0 {
			return fail("you are not a required reviewer for: %s", strings.Join(res.Skipped, ", "))
		}
		return fail("no matching pending deployments for run %d", res.RunID)
	}

	if ctx.DryRun {
		res.DryRun = true
		return res
	}
	if req.Confirm != nil && !req.Confirm(res) {
		res.Aborted = true
		res.Decided = nil
		return res
	}

	if err := service.ReviewPendingDeployments(ctx, req.OrgName, req.RepoName, res.RunID, ids, state, comment); err != nil {
		res.Decided = nil
		return fail("failed to submit review: %v", err)
	}
	return res
}
