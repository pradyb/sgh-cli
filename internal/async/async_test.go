// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package async

import (
	"slices"
	"sync"
	"testing"
	"time"
)

// Start blocks until its workers have drained the job channel and both consumer
// goroutines have finished, so it returns only once every result and error
// handler has been invoked. Waiting on it is exact; the tests below run it in a
// goroutine and wait on `done` rather than sleeping for a guessed duration.
//
// Results arrive in worker-completion order, which is not the order jobs were
// submitted. Assertions therefore compare sorted values.

func TestAsyncJobQueueBasicProcessing(t *testing.T) {
	queue := NewAsyncJobQueue[int, int](3)
	var results []int
	var errors []error
	var mu sync.Mutex

	jobHandler := func(job AsyncJob[int]) (int, error) {
		return job.JobData * 2, nil
	}

	resultHandler := func(result AsyncJobResult[int, int]) {
		mu.Lock()
		results = append(results, result.Result)
		mu.Unlock()
	}

	errorHandler := func(err AsyncJobError[int]) {
		mu.Lock()
		errors = append(errors, err.Error)
		mu.Unlock()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		queue.Start(jobHandler, resultHandler, errorHandler, 2)
	}()

	for i := 1; i <= 5; i++ {
		queue.AddJob(AsyncJob[int]{ID: i, JobData: i})
	}

	queue.Close()
	<-done

	if len(errors) != 0 {
		t.Errorf("Expected 0 errors, got %d: %v", len(errors), errors)
	}

	// Every job should have been doubled, in whatever order the workers finished.
	slices.Sort(results)
	want := []int{2, 4, 6, 8, 10}
	if !slices.Equal(results, want) {
		t.Errorf("Expected results %v, got %v", want, results)
	}
}

func TestAsyncJobQueueMultipleWorkers(t *testing.T) {
	queue := NewAsyncJobQueue[int, int](10)
	var results []int
	var errors []error
	var mu sync.Mutex

	jobHandler := func(job AsyncJob[int]) (int, error) {
		time.Sleep(10 * time.Millisecond) // Simulate work
		return job.JobData * 2, nil
	}

	resultHandler := func(result AsyncJobResult[int, int]) {
		mu.Lock()
		results = append(results, result.Result)
		mu.Unlock()
	}

	errorHandler := func(err AsyncJobError[int]) {
		mu.Lock()
		errors = append(errors, err.Error)
		mu.Unlock()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		queue.Start(jobHandler, resultHandler, errorHandler, 3)
	}()

	for i := 1; i <= 10; i++ {
		queue.AddJob(AsyncJob[int]{ID: i, JobData: i})
	}

	queue.Close()
	<-done

	if len(errors) != 0 {
		t.Errorf("Expected 0 errors, got %d: %v", len(errors), errors)
	}

	// No job may be dropped or processed twice when spread across 3 workers.
	slices.Sort(results)
	want := []int{2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	if !slices.Equal(results, want) {
		t.Errorf("Expected results %v, got %v", want, results)
	}
}

func TestAsyncJobQueueErrorHandling(t *testing.T) {
	queue := NewAsyncJobQueue[int, int](5)
	var results []int
	var errors []error
	var mu sync.Mutex

	jobHandler := func(job AsyncJob[int]) (int, error) {
		if job.JobData%2 == 0 {
			return 0, &testError{message: "even number error"}
		}
		return job.JobData * 2, nil
	}

	resultHandler := func(result AsyncJobResult[int, int]) {
		mu.Lock()
		results = append(results, result.Result)
		mu.Unlock()
	}

	errorHandler := func(err AsyncJobError[int]) {
		mu.Lock()
		errors = append(errors, err.Error)
		mu.Unlock()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		queue.Start(jobHandler, resultHandler, errorHandler, 2)
	}()

	for i := 1; i <= 6; i++ {
		queue.AddJob(AsyncJob[int]{ID: i, JobData: i})
	}

	queue.Close()
	<-done

	// Odd jobs produce results, even jobs produce errors.
	slices.Sort(results)
	want := []int{2, 6, 10}
	if !slices.Equal(results, want) {
		t.Errorf("Expected results %v, got %v", want, results)
	}

	if len(errors) != 3 {
		t.Errorf("Expected 3 errors, got %d: %v", len(errors), errors)
	}
}

func TestAsyncJobQueueEmptyQueue(t *testing.T) {
	queue := NewAsyncJobQueue[int, int](0)
	var resultCount, errorCount int

	jobHandler := func(job AsyncJob[int]) (int, error) { return job.JobData, nil }
	resultHandler := func(result AsyncJobResult[int, int]) { resultCount++ }
	errorHandler := func(err AsyncJobError[int]) { errorCount++ }

	done := make(chan struct{})
	go func() {
		defer close(done)
		queue.Start(jobHandler, resultHandler, errorHandler, 2)
	}()

	queue.Close()

	// Closing an empty queue must let Start return rather than hang.
	<-done

	if resultCount != 0 || errorCount != 0 {
		t.Errorf("Expected no handler calls, got %d results and %d errors", resultCount, errorCount)
	}
}

type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}
