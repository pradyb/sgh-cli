// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package workflow

import (
	"context"
	"sort"
	"time"

	"github.com/pradyb/sgh-cli/internal/model"
	"github.com/pradyb/sgh-cli/internal/service"
	appcontext "github.com/pradyb/sgh-cli/pkg/context"
)

// How long, and how often, to look for the run an action (a merge, a tag, a rerun) is about to
// start. Variables so tests can shorten them; the wait is counted in polls, so a no-op
// RunLookupSleep makes a lookup that finds nothing return immediately.
var (
	RunLookupInterval = 3 * time.Second
	RunLookupGrace    = 60 * time.Second
	RunLookupSleep    = time.Sleep
)

func lookupAttempts() int {
	if n := int(RunLookupGrace / RunLookupInterval); n > 1 {
		return n
	}
	return 1
}

// FindRunsRequest identifies the runs a push triggered.
type FindRunsRequest struct {
	OrgName  string
	RepoName string
	// HeadSHA is the commit pushed (a merge commit, or the commit a new tag points at).
	HeadSHA string
	// Event is the triggering event, e.g. "push".
	Event string
	// Branch, when set, must equal the run's head_branch. For a tag push that is the tag name,
	// which stops a tag on an already-pushed commit from matching that commit's branch-push run.
	Branch string
}

// FindRuns waits for the runs started for req. It matches by commit rather than "latest run",
// since the latest run on a busy branch may belong to someone else's push. The first match
// is followed by one more look, because workflows triggered by the same push start a moment
// apart. It returns no runs and no error if none appears within RunLookupGrace (the workflow
// may simply not trigger on this event), and stops early if ctx is cancelled.
func FindRuns(ctx context.Context, appCtx *appcontext.Context, req FindRunsRequest) ([]model.WorkflowRun, error) {
	attempts := lookupAttempts()
	for i := 0; i < attempts; i++ {
		matched, err := matchingRuns(appCtx, req)
		if err != nil {
			return nil, err
		}
		if len(matched) > 0 {
			RunLookupSleep(RunLookupInterval)
			if again, err := matchingRuns(appCtx, req); err == nil && len(again) >= len(matched) {
				matched = again
			}
			sort.Slice(matched, func(a, b int) bool { return matched[a].ID < matched[b].ID })
			return matched, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if i < attempts-1 {
			RunLookupSleep(RunLookupInterval)
		}
	}
	return nil, nil
}

func matchingRuns(appCtx *appcontext.Context, req FindRunsRequest) ([]model.WorkflowRun, error) {
	runs, err := service.ListWorkflowRunsForCommit(appCtx, req.OrgName, req.RepoName, req.HeadSHA, req.Event)
	if err != nil {
		return nil, err
	}
	var matched []model.WorkflowRun
	for _, r := range runs {
		// The API filters on these too; checking again keeps a stray run from being followed.
		if r.HeadSha != req.HeadSHA || (req.Event != "" && r.Event != req.Event) || (req.Branch != "" && r.HeadBranch != req.Branch) {
			continue
		}
		matched = append(matched, r)
	}
	return matched, nil
}

// WaitForRerun waits until a requested rerun is visible on the run: a rerun keeps the run ID, and
// for a moment GitHub still reports the previous attempt's "completed" state, which would end a
// watch immediately. prevAttempt is the run_attempt read before the rerun was requested. It
// returns false if the new attempt hasn't appeared within RunLookupGrace. A run that can't be
// fetched is returned as-is (true) so the caller reports its error.
func WaitForRerun(ctx context.Context, appCtx *appcontext.Context, req WorkflowRunRequest, prevAttempt int) (model.WorkflowRunDetail, bool) {
	attempts := lookupAttempts()
	var detail model.WorkflowRunDetail
	for i := 0; i < attempts; i++ {
		detail = GetWorkflowRunDetail(appCtx, req)
		if detail.ErrorMessage != "" || detail.Run.RunAttempt > prevAttempt || detail.Run.Status != "completed" {
			return detail, true
		}
		if ctx.Err() != nil {
			return detail, false
		}
		if i < attempts-1 {
			RunLookupSleep(RunLookupInterval)
		}
	}
	return detail, false
}
