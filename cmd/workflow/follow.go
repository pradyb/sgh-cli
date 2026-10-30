// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package workflow

import (
	stdctx "context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/pradyb/sgh-cli/pkg/context"
	"github.com/pradyb/sgh-cli/pkg/workflow"
)

// This file is how other commands (pr merge, tag create, ...) hand the CI run they triggered to
// the same watchers as `workflow view --watch`.

// FollowOptions are the --watch flags shared by every command that can follow a run.
type FollowOptions struct {
	Interval int           // polling interval in seconds (minimum 5)
	Timeout  time.Duration // per run; 0 = none
}

// followEvent is the NDJSON line the --watch entry points print around the runs they follow.
// Repositories are followed one after another, so every event up to the next run_done belongs to
// the repository named by the preceding line.
type followEvent struct {
	// Kind is the action's outcome (dispatched, rerun_requested, merged, tag_created, cancel_done,
	// or its *_error), or what happened looking for the run: run_found, watch_skipped, watch_error.
	Kind       string `json:"kind"`
	Repository string `json:"repository"`
	RunID      int    `json:"run_id,omitempty"`
	SHA        string `json:"sha,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Conclusion string `json:"conclusion,omitempty"` // cancel_done
}

func emitFollowEvent(e followEvent) {
	b, _ := json.Marshal(e)
	fmt.Println(string(b))
}

// EmitFollowEvent prints one NDJSON line for a command's own outcome when --watch --json is
// replacing its usual table. sha and reason may be empty.
func EmitFollowEvent(kind, repo string, runID int, sha, reason string) {
	emitFollowEvent(followEvent{Kind: kind, Repository: repo, RunID: runID, SHA: sha, Reason: reason})
}

// ValidateFollowInterval rejects a --interval below the minimum, reporting it on stderr and
// setting HasError, so a caller can refuse before it changes anything.
func ValidateFollowInterval(cmd *cobra.Command, ctx *context.Context, interval int) bool {
	if interval < 5 {
		fmt.Fprintln(cmd.ErrOrStderr(), "  ✗ --interval must be at least 5 seconds when using --watch")
		ctx.HasError = true
		return false
	}
	return true
}

// watchContext is the command's context, ended after timeout when that is positive.
func watchContext(cmd *cobra.Command, timeout time.Duration) (stdctx.Context, stdctx.CancelFunc) {
	parent := cmd.Context()
	if parent == nil {
		parent = stdctx.Background()
	}
	if timeout > 0 {
		return stdctx.WithTimeout(parent, timeout)
	}
	return parent, func() {}
}

// FollowRerun follows a run that was just asked to rerun (same run ID, new attempt) once its new
// attempt is visible. prevAttempt is the run_attempt read before the rerun was requested.
func FollowRerun(cmd *cobra.Command, ctx *context.Context, req workflow.WorkflowRunRequest, prevAttempt int, o FollowOptions) {
	wctx, cancel := watchContext(cmd, 0)
	defer cancel()
	detail, started := workflow.WaitForRerun(wctx, ctx, req, prevAttempt)
	if !started {
		reason := fmt.Sprintf("the rerun of run %d did not start within %s", req.RunID, workflow.RunLookupGrace)
		reportSkipped(cmd, ctx, req.RepoName, reason)
		return
	}
	if ctx.JSON {
		emitFollowEvent(followEvent{Kind: "run_found", Repository: req.RepoName, RunID: req.RunID})
	} else {
		fmt.Printf("\n  Watching %s run %d (attempt %d)\n", req.RepoName, req.RunID, detail.Run.RunAttempt)
	}
	if d := followRun(cmd, ctx, req, detail, o.Interval, o.Timeout); d.ErrorMessage != "" {
		ctx.HasError = true
	}
}

// FollowCommitRuns finds the runs a push (a merge commit, a new tag) started and follows each in
// turn. Finding nothing within the grace period is not an error: the repository may have no
// workflow for that event.
func FollowCommitRuns(cmd *cobra.Command, ctx *context.Context, find workflow.FindRunsRequest, o FollowOptions) {
	if !ctx.JSON {
		fmt.Printf("\n  Looking for the %s run(s) triggered by %.7s on %s...\n", find.Event, find.HeadSHA, find.RepoName)
	}
	wctx, cancel := watchContext(cmd, 0)
	defer cancel()
	runs, err := workflow.FindRuns(wctx, ctx, find)
	switch {
	case err != nil:
		ctx.HasError = true
		if ctx.JSON {
			emitFollowEvent(followEvent{Kind: "watch_error", Repository: find.RepoName, SHA: find.HeadSHA, Reason: err.Error()})
		} else {
			fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ %s: could not look up the triggered run: %v\n", find.RepoName, err)
		}
		return
	case len(runs) == 0:
		reason := fmt.Sprintf("no %s run found for %.7s within %s (the workflow may not trigger on this event)", find.Event, find.HeadSHA, workflow.RunLookupGrace)
		reportSkipped(cmd, ctx, find.RepoName, reason)
		return
	}
	for _, r := range runs {
		req := workflow.WorkflowRunRequest{OrgName: find.OrgName, RepoName: find.RepoName, RunID: r.ID}
		if ctx.JSON {
			emitFollowEvent(followEvent{Kind: "run_found", Repository: find.RepoName, RunID: r.ID, SHA: find.HeadSHA})
		} else {
			fmt.Printf("\n  Watching %s run %d (%s)\n", find.RepoName, r.ID, r.Name)
		}
		if d := followRun(cmd, ctx, req, workflow.GetWorkflowRunDetail(ctx, req), o.Interval, o.Timeout); d.ErrorMessage != "" {
			ctx.HasError = true
		}
	}
}

func reportSkipped(cmd *cobra.Command, ctx *context.Context, repo, reason string) {
	if ctx.JSON {
		emitFollowEvent(followEvent{Kind: "watch_skipped", Repository: repo, Reason: reason})
		return
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "  ! %s: not watching, %s\n", repo, reason)
}
