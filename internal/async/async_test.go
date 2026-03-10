// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package async

import (
	"sync"
	"testing"
	"time"
)

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

	go queue.Start(jobHandler, resultHandler, errorHandler, 2)

	for i := 1; i <= 5; i++ {
		queue.AddJob(AsyncJob[int]{ID: i, JobData: i})
	}

	queue.Close()

	// Wait a bit for processing to complete
	time.Sleep(100 * time.Millisecond)

	if len(results) != 5 {
		t.Errorf("Expected 5 results, got %d", len(results))
	}

	if len(errors) != 0 {
		t.Errorf("Expected 0 errors, got %d", len(errors))
	}

	// Check that results are doubled
	for i, result := range results {
		expected := (i + 1) * 2
		if result != expected {
			t.Errorf("Expected result %d, got %d", expected, result)
		}
	}
}

func TestAsyncJobQueueMultipleWorkers(t *testing.T) {
	queue := NewAsyncJobQueue[int, int](10)
	var results []int
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
		// No errors expected
	}

	go queue.Start(jobHandler, resultHandler, errorHandler, 3)

	for i := 1; i <= 10; i++ {
		queue.AddJob(AsyncJob[int]{ID: i, JobData: i})
	}

	queue.Close()

	// Wait for processing to complete
	time.Sleep(200 * time.Millisecond)

	if len(results) != 10 {
		t.Errorf("Expected 10 results, got %d", len(results))
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

	go queue.Start(jobHandler, resultHandler, errorHandler, 2)

	for i := 1; i <= 6; i++ {
		queue.AddJob(AsyncJob[int]{ID: i, JobData: i})
	}

	queue.Close()

	// Wait for processing to complete
	time.Sleep(100 * time.Millisecond)

	// Should have 3 results (odd numbers) and 3 errors (even numbers)
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	if len(errors) != 3 {
		t.Errorf("Expected 3 errors, got %d", len(errors))
	}

	// Check that only odd numbers produced results
	for _, result := range results {
		if result%4 != 2 {
			t.Errorf("Expected result to be odd number * 2, got %d", result)
		}
	}
}

func TestAsyncJobQueueEmptyQueue(t *testing.T) {
	queue := NewAsyncJobQueue[int, int](0)
	jobHandler := func(job AsyncJob[int]) (int, error) { return job.JobData, nil }
	resultHandler := func(result AsyncJobResult[int, int]) {
		// Should not be called
	}
	errorHandler := func(err AsyncJobError[int]) {
		// Should not be called
	}

	go queue.Start(jobHandler, resultHandler, errorHandler, 2)
	queue.Close()

	// Should complete without issues
	time.Sleep(50 * time.Millisecond)
}

type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}
