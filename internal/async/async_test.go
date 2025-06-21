package async

import (
	"errors"
	"sync/atomic"
	"testing"
)

func TestASyncJobQueue_BasicProcessing(t *testing.T) {
	queue := NewASyncJobQueue[int, int](3)
	var processed []int
	var mu atomic.Int32

	jobHandler := func(job ASyncJob[int]) (int, error) {
		return job.JobData * 2, nil
	}
	resultHandler := func(result ASyncJobResult[int, int]) {
		processed = append(processed, result.Result)
		mu.Add(1)
	}
	errorHandler := func(err ASyncJobError[int]) {
		t.Errorf("unexpected error: %v", err.Error)
	}

	for i := 1; i <= 3; i++ {
		queue.AddJob(ASyncJob[int]{Id: i, JobData: i})
	}
	queue.Close()
	queue.Start(jobHandler, resultHandler, errorHandler, 1)

	if mu.Load() != 3 {
		t.Errorf("expected 3 jobs processed, got %d", mu.Load())
	}
	for i, v := range processed {
		if v != (i+1)*2 {
			t.Errorf("expected result %d, got %d", (i+1)*2, v)
		}
	}
}

func TestASyncJobQueue_MultipleWorkers(t *testing.T) {
	queue := NewASyncJobQueue[int, int](10)
	var sum atomic.Int32

	jobHandler := func(job ASyncJob[int]) (int, error) {
		return job.JobData, nil
	}
	resultHandler := func(result ASyncJobResult[int, int]) {
		sum.Add(int32(result.Result))
	}
	errorHandler := func(err ASyncJobError[int]) {
		t.Errorf("unexpected error: %v", err.Error)
	}

	for i := 1; i <= 10; i++ {
		queue.AddJob(ASyncJob[int]{Id: i, JobData: i})
	}
	queue.Close()
	queue.Start(jobHandler, resultHandler, errorHandler, 4)

	if sum.Load() != 55 {
		t.Errorf("expected sum 55, got %d", sum.Load())
	}
}

func TestASyncJobQueue_ErrorHandling(t *testing.T) {
	queue := NewASyncJobQueue[int, int](5)
	var errorCount atomic.Int32

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
	}
	errorHandler := func(err ASyncJobError[int]) {
		errorCount.Add(1)
	}

	for i := 1; i <= 5; i++ {
		queue.AddJob(ASyncJob[int]{Id: i, JobData: i})
	}
	queue.Close()
	queue.Start(jobHandler, resultHandler, errorHandler, 2)

	if errorCount.Load() != 2 {
		t.Errorf("expected 2 errors, got %d", errorCount.Load())
	}
}

func TestASyncJobQueue_EmptyQueue(t *testing.T) {
	queue := NewASyncJobQueue[int, int](0)
	jobHandler := func(job ASyncJob[int]) (int, error) { return job.JobData, nil }
	resultHandler := func(result ASyncJobResult[int, int]) {}
	errorHandler := func(err ASyncJobError[int]) {}
	queue.Close()
	queue.Start(jobHandler, resultHandler, errorHandler, 1)
	// Should not panic or deadlock
}
