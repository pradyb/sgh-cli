// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package workflow

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/pkg/apperrors"
	appcontext "github.com/pradyb/sgh-cli/pkg/context"
)

// ErrWatchTimeout is returned by WatchApprovals when Timeout elapses before the run
// completes.
var ErrWatchTimeout = errors.New("timed out waiting for the run to complete")

// WatchApproveRequest configures WatchApprovals.
type WatchApproveRequest struct {
	OrgName  string
	RepoName string
	// RunID 0 resolves the latest run in "waiting" status.
	RunID int
	// Environments limits which gates are decided. Required (non-empty) when Yes is
	// true, so an unattended watch never approves a gate that didn't exist yet when it
	// started.
	Environments []string
	Reject       bool
	Comment      string
	// Interval between polls. Callers should enforce their own sane minimum; this
	// package does not impose one.
	Interval time.Duration
	// Timeout, if positive, ends the watch (returning ErrWatchTimeout) once elapsed.
	// Zero means no timeout.
	Timeout time.Duration
	Yes     bool
	// Confirm is asked once per newly-seen gate, only when !Yes. Returning false skips
	// that gate for the rest of this watch (it is never re-asked).
	Confirm func(WatchGate) bool
	// Notify reports one thing that happened, in the order it happened, so the caller
	// can print it (or emit it as NDJSON). Optional; nil means "don't report".
	Notify func(WatchEvent)
	// Sleep defaults to time.Sleep; tests override it so polling doesn't really wait.
	Sleep func(time.Duration)
}

// WatchGate is what a caller's Confirm function is asked to decide about.
type WatchGate struct {
	Environment string
	Reviewers   []string
}

// WatchEvent is one thing that happened during a watch, in order.
type WatchEvent struct {
	// Kind is one of: "gate_decided", "gate_skipped", "gate_error", "run_done".
	Kind string `json:"kind"`
	// Environment is set for gate_decided / gate_skipped / gate_error.
	Environment string `json:"environment,omitempty"`
	// State is "approved" or "rejected", set for gate_decided.
	State string `json:"state,omitempty"`
	// Reason is set for gate_skipped ("not a required reviewer" / "declined") and
	// gate_error (the submit failure).
	Reason string `json:"reason,omitempty"`
	// Conclusion is the run's conclusion, set for run_done.
	Conclusion string `json:"conclusion,omitempty"`
}

// WatchApprovals stays attached to one run, deciding each newly-appearing environment
// gate as it shows up (after confirmation, unless Yes), until the run completes, ctx is
// cancelled (e.g. Ctrl-C — returns nil, deciding nothing further), or Timeout elapses.
// The returned error is nil on a clean stop; non-nil only for a hard failure (the run or
// gate list couldn't be fetched, an auth error, or Timeout). A gate-level submit failure
// is reported via Notify (Kind: "gate_error") and does not stop the watch — check
// WatchResult's HadFailures for whether the caller should exit non-zero.
func WatchApprovals(ctx context.Context, appCtx *appcontext.Context, req WatchApproveRequest) (WatchResult, error) {
	result := WatchResult{Repository: req.RepoName, RunID: req.RunID}

	// Enforced here, not just by the CLI layer, so no caller can bypass it: unattended
	// auto-approval must never decide a gate that didn't exist when the watch started.
	if req.Yes && len(req.Environments) == 0 {
		return result, errors.New("Yes requires a non-empty Environments allowlist, so an unattended watch never approves a gate that didn't exist when it started")
	}
	if req.Confirm == nil && !req.Yes {
		return result, errors.New("Confirm must be set when Yes is false")
	}

	sleep := req.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	notify := req.Notify
	if notify == nil {
		notify = func(WatchEvent) {}
	}

	if result.RunID == 0 {
		id, err := latestRunID(appCtx, req.OrgName, req.RepoName, "waiting")
		if err != nil {
			return result, err
		}
		result.RunID = id
	}

	var deadline time.Time
	if req.Timeout > 0 {
		deadline = time.Now().Add(req.Timeout)
	}

	state, defaultComment := "approved", "Approved via sgh"
	if req.Reject {
		state, defaultComment = "rejected", "Rejected via sgh"
	}
	comment := req.Comment
	if comment == "" {
		comment = defaultComment
	}

	seen := make(map[int]bool)     // environment ID -> already handled (decided or declined) this run
	reported := make(map[int]bool) // environment ID -> already reported as outside the allowlist
	for {
		if err := ctx.Err(); err != nil {
			return result, nil // Ctrl-C or similar: stop cleanly, decide nothing further
		}
		if !deadline.IsZero() && !time.Now().Before(deadline) {
			return result, ErrWatchTimeout
		}

		run, err := service.GetWorkflowRun(appCtx, req.OrgName, req.RepoName, result.RunID)
		if err != nil {
			return result, fmt.Errorf("failed to get workflow run: %w", err)
		}
		if run.Status == "completed" {
			notify(WatchEvent{Kind: "run_done", Conclusion: run.Conclusion})
			result.Conclusion = run.Conclusion
			return result, nil
		}

		gates, err := service.ListPendingDeployments(appCtx, req.OrgName, req.RepoName, result.RunID)
		if err != nil {
			return result, fmt.Errorf("failed to list pending deployments: %w", err)
		}

		for _, g := range gates {
			if seen[g.Environment.ID] {
				continue
			}
			name := g.Environment.Name

			if len(req.Environments) > 0 && !slices.Contains(req.Environments, name) {
				// Outside the allowlist: leave it genuinely pending (not "seen", so it's
				// picked up automatically if a later invocation adds it to the
				// allowlist), but only report it once per watch, not on every poll.
				if !reported[g.Environment.ID] {
					reported[g.Environment.ID] = true
					notify(WatchEvent{Kind: "gate_skipped", Environment: name, Reason: "not in --environment, left pending"})
				}
				continue
			}
			if !g.CurrentUserCanApprove {
				seen[g.Environment.ID] = true
				notify(WatchEvent{Kind: "gate_skipped", Environment: name, Reason: "not a required reviewer"})
				continue
			}

			decide := req.Yes
			if !decide {
				// req.Confirm is guaranteed non-nil here: validated up front above.
				reviewers := make([]string, len(g.Reviewers))
				for i, r := range g.Reviewers {
					reviewers[i] = r.DisplayName()
				}
				decide = req.Confirm(WatchGate{Environment: name, Reviewers: reviewers})
			}
			if !decide {
				seen[g.Environment.ID] = true
				notify(WatchEvent{Kind: "gate_skipped", Environment: name, Reason: "declined"})
				continue
			}

			if err := service.ReviewPendingDeployments(appCtx, req.OrgName, req.RepoName, result.RunID, []int{g.Environment.ID}, state, comment); err != nil {
				var ghErr *apperrors.GitHubError
				if errors.As(err, &ghErr) && (ghErr.StatusCode == http.StatusUnauthorized || ghErr.StatusCode == http.StatusForbidden) {
					return result, err
				}
				// Any other failure (commonly: someone else already decided this gate
				// first) is reported and the gate is marked handled so it's never
				// retried in a loop; watching continues.
				seen[g.Environment.ID] = true
				result.HadFailures = true
				notify(WatchEvent{Kind: "gate_error", Environment: name, Reason: err.Error()})
				continue
			}
			seen[g.Environment.ID] = true
			result.Decided = append(result.Decided, name)
			notify(WatchEvent{Kind: "gate_decided", Environment: name, State: state})
		}

		sleep(req.Interval)
	}
}

// WatchResult summarizes one watch session, for the caller's final report and exit code.
type WatchResult struct {
	Repository  string
	RunID       int
	Decided     []string
	HadFailures bool
	// Conclusion is the run's final conclusion, set once WatchApprovals returns with a
	// nil error because the run completed (as opposed to a timeout or Ctrl-C).
	Conclusion string
}
