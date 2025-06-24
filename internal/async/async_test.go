package async

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestASyncJobQueueBasicProcessing(t *testing.T) {
	queue := NewASyncJobQueue[int, int](3)
	var processed []int
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(3) // Expect 3 results

	jobHandler := func(job ASyncJob[int]) (int, error) {
		return job.JobData * 2, nil
	}
	resultHandler := func(result ASyncJobResult[int, int]) {
		mu.Lock()
		processed = append(processed, result.Result)
		mu.Unlock()
		wg.Done()
	}
	errorHandler := func(err ASyncJobError[int]) {
		t.Errorf("unexpected error: %v", err.Error)
	}

	for i := 1; i <= 3; i++ {
		queue.AddJob(ASyncJob[int]{Id: i, JobData: i})
	}
	queue.Close()
	queue.Start(jobHandler, resultHandler, errorHandler, 1)

	// Wait for all results to be processed
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(processed) != 3 {
		t.Errorf("expected 3 jobs processed, got %d", len(processed))
	}

	// Check if all expected results are present (order might vary due to concurrency)
	expected := map[int]bool{2: false, 4: false, 6: false}
	for _, v := range processed {
		if _, ok := expected[v]; ok {
			expected[v] = true
		} else {
			t.Errorf("unexpected result: %d", v)
		}
	}

	for val, found := range expected {
		if !found {
			t.Errorf("expected result %d not found", val)
		}
	}
}

func TestASyncJobQueueMultipleWorkers(t *testing.T) {
	queue := NewASyncJobQueue[int, int](10)
	var sum atomic.Int32
	var wg sync.WaitGroup
	wg.Add(10) // Expect 10 results

	jobHandler := func(job ASyncJob[int]) (int, error) {
		return job.JobData, nil
	}
	resultHandler := func(result ASyncJobResult[int, int]) {
		sum.Add(int32(result.Result))
		wg.Done()
	}
	errorHandler := func(err ASyncJobError[int]) {
		t.Errorf("unexpected error: %v", err.Error)
	}

	for i := 1; i <= 10; i++ {
		queue.AddJob(ASyncJob[int]{Id: i, JobData: i})
	}
	queue.Close()
	queue.Start(jobHandler, resultHandler, errorHandler, 4)

	// Wait for all results to be processed
	wg.Wait()

	if sum.Load() != 55 {
		t.Errorf("expected sum 55, got %d", sum.Load())
	}
}

func TestASyncJobQueueErrorHandling(t *testing.T) {
	queue := NewASyncJobQueue[int, int](5)
	var errorCount atomic.Int32
	var successCount atomic.Int32
	var wg sync.WaitGroup
	wg.Add(5) // Expect 5 total results (3 successes + 2 errors)

	jobHandler := func(job ASyncJob[int]) (int, error) {
		if job.JobData%2 == 0 {
			return 0, errors.New("even number error")
		}
		return job.JobData, nil
	}
	resultHandler := func(result ASyncJobResult[int, int]) {
		// Only odd numbers should succeed
		if result.Result%2 == 0 {
			t.Errorf("unexpected even result: %d", result.Result)
		}
		successCount.Add(1)
		wg.Done()
	}
	errorHandler := func(err ASyncJobError[int]) {
		errorCount.Add(1)
		wg.Done()
	}

	for i := 1; i <= 5; i++ {
		queue.AddJob(ASyncJob[int]{Id: i, JobData: i})
	}
	queue.Close()
	queue.Start(jobHandler, resultHandler, errorHandler, 2)

	// Wait for all results to be processed
	wg.Wait()

	if errorCount.Load() != 2 {
		t.Errorf("expected 2 errors, got %d", errorCount.Load())
	}
	if successCount.Load() != 3 {
		t.Errorf("expected 3 successes, got %d", successCount.Load())
	}
}

func TestASyncJobQueueEmptyQueue(t *testing.T) {
	queue := NewASyncJobQueue[int, int](0)
	jobHandler := func(job ASyncJob[int]) (int, error) { return job.JobData, nil }
	resultHandler := func(result ASyncJobResult[int, int]) {
		// No jobs will be processed - testing empty queue behavior
	}
	errorHandler := func(err ASyncJobError[int]) {
		// We don't expect any errors in this test - testing empty queue behavior
	}
	queue.Close()
	queue.Start(jobHandler, resultHandler, errorHandler, 1)
	// Should not panic or deadlock when processing empty queue
}
