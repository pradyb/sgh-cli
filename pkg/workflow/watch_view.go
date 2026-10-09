// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/pradyb/sgh-cli/internal/model"
	appcontext "github.com/pradyb/sgh-cli/pkg/context"
)

// ViewWatchEvent is one job/step/run status transition reported during `view --watch --json`.
type ViewWatchEvent struct {
	// Kind is one of: "job_started", "job_completed", "step_started", "step_completed", "run_done".
	Kind string `json:"kind"`
	// Job is set for job_started/job_completed/step_started/step_completed.
	Job string `json:"job,omitempty"`
	// Step is set for step_started/step_completed.
	Step string `json:"step,omitempty"`
	// Conclusion is set for job_completed, step_completed, and run_done.
	Conclusion string `json:"conclusion,omitempty"`
}

// WatchViewOptions configures WatchViewRun.
type WatchViewOptions struct {
	Interval time.Duration
	// Notify reports one event, in the order it happened. Optional; nil means "don't report".
	Notify func(ViewWatchEvent)
	// Sleep defaults to time.Sleep; tests override it so polling doesn't really wait.
	Sleep func(time.Duration)
}

// WatchViewRun polls a run's detail, starting from an already-fetched initial detail, reporting
// each job/step status transition via Notify as it's first observed, until the run completes or
// ctx is cancelled (e.g. Ctrl-C — stops immediately, reporting nothing further). It never
// re-reports a transition that was already reported on an earlier poll. Returns the last detail
// fetched.
func WatchViewRun(ctx context.Context, appCtx *appcontext.Context, req WorkflowRunRequest, initial model.WorkflowRunDetail, opt WatchViewOptions) model.WorkflowRunDetail {
	sleep := opt.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	notify := opt.Notify
	if notify == nil {
		notify = func(ViewWatchEvent) {}
	}

	state := newViewWatchState()
	detail := initial
	for {
		for _, e := range state.diff(detail) {
			notify(e)
		}
		if !detail.IsInProgress() {
			notify(ViewWatchEvent{Kind: "run_done", Conclusion: detail.Run.Conclusion})
			return detail
		}
		if ctx.Err() != nil {
			return detail // Ctrl-C or similar: stop cleanly, report nothing further
		}
		sleep(opt.Interval)
		if ctx.Err() != nil {
			return detail
		}
		detail = GetWorkflowRunDetail(appCtx, req)
	}
}

// viewWatchProgress tracks which of "started"/"completed" have already been reported for one
// job or step, so a poll that finds nothing new produces no events (regardless of how the raw
// status flickers between polls, e.g. queued <-> in_progress before ever reaching completed).
type viewWatchProgress struct {
	started   bool
	completed bool
}

// viewWatchState tracks reported progress per job and step.
type viewWatchState struct {
	jobs  map[int]*viewWatchProgress    // job ID -> progress
	steps map[string]*viewWatchProgress // "jobID:stepNumber" -> progress
}

func newViewWatchState() *viewWatchState {
	return &viewWatchState{jobs: make(map[int]*viewWatchProgress), steps: make(map[string]*viewWatchProgress)}
}

// diff returns the newly-observed job/step transitions in detail (a job/step is "started" the
// first time it's seen in_progress, "completed" the first time it's seen completed — a job that
// jumps straight to completed without ever being observed in_progress reports only completed),
// updating the tracked state in place.
func (s *viewWatchState) diff(detail model.WorkflowRunDetail) []ViewWatchEvent {
	var events []ViewWatchEvent
	for _, j := range detail.Jobs {
		p, ok := s.jobs[j.ID]
		if !ok {
			p = &viewWatchProgress{}
			s.jobs[j.ID] = p
		}
		if j.Status == "in_progress" && !p.started {
			p.started = true
			events = append(events, ViewWatchEvent{Kind: "job_started", Job: j.Name})
		}

		for _, st := range j.Steps {
			key := fmt.Sprintf("%d:%d", j.ID, st.Number)
			sp, ok := s.steps[key]
			if !ok {
				sp = &viewWatchProgress{}
				s.steps[key] = sp
			}
			if st.Status == "in_progress" && !sp.started {
				sp.started = true
				events = append(events, ViewWatchEvent{Kind: "step_started", Job: j.Name, Step: st.Name})
			}
			if st.Status == "completed" && !sp.completed {
				sp.completed = true
				events = append(events, ViewWatchEvent{Kind: "step_completed", Job: j.Name, Step: st.Name, Conclusion: st.Conclusion})
			}
		}

		// After its steps, so a job that finished within one poll reads in order.
		if j.Status == "completed" && !p.completed {
			p.completed = true
			events = append(events, ViewWatchEvent{Kind: "job_completed", Job: j.Name, Conclusion: j.Conclusion})
		}
	}
	return events
}
