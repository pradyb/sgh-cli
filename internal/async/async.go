package async

import (
	"sync"
)

type AsyncJob[T any] struct {
	ID      int
	JobType string
	JobData T
}

type AsyncJobResult[T, R any] struct {
	ID      int
	JobData T
	Result  R
}

type AsyncJobError[T any] struct {
	ID      int
	JobData T
	Error   error
}

type AsyncJobHandler[T, R any] func(job AsyncJob[T]) (R, error)

type AsyncJobResultHandler[T, R any] func(result AsyncJobResult[T, R])

type AsyncJobErrorHandler[T any] func(err AsyncJobError[T])

type AsyncJobQueue[T, R any] struct {
	Jobs    chan AsyncJob[T]
	Results chan AsyncJobResult[T, R]
	Errors  chan AsyncJobError[T]
	wg      sync.WaitGroup
}

func NewAsyncJobQueue[T, R any](jobQueueSize int) *AsyncJobQueue[T, R] {
	return &AsyncJobQueue[T, R]{
		Jobs:    make(chan AsyncJob[T], jobQueueSize),
		Results: make(chan AsyncJobResult[T, R], jobQueueSize),
		Errors:  make(chan AsyncJobError[T], jobQueueSize),
	}
}

func (q *AsyncJobQueue[T, R]) Start(jobHandler AsyncJobHandler[T, R], resultHandler AsyncJobResultHandler[T, R], errorHandler AsyncJobErrorHandler[T], noOfWorkers int) {
	// Start goroutines to handle results and errors concurrently
	go func() {
		for result := range q.Results {
			resultHandler(result)
		}
	}()

	go func() {
		for err := range q.Errors {
			errorHandler(err)
		}
	}()

	// Start workers
	for range make([]int, noOfWorkers) {
		q.wg.Add(1)
		go q.worker(jobHandler)
	}

	// Wait for all workers to complete
	q.wg.Wait()

	// Close channels to signal completion
	close(q.Results)
	close(q.Errors)
}

func (q *AsyncJobQueue[T, R]) worker(jobHandler AsyncJobHandler[T, R]) {
	defer q.wg.Done()
	for job := range q.Jobs {
		result, err := jobHandler(job)
		if err != nil {
			q.Errors <- AsyncJobError[T]{ID: job.ID, Error: err, JobData: job.JobData}
		} else {
			q.Results <- AsyncJobResult[T, R]{ID: job.ID, Result: result, JobData: job.JobData}
		}
	}
}

func (q *AsyncJobQueue[T, R]) AddJob(job AsyncJob[T]) {
	q.Jobs <- job
}

func (q *AsyncJobQueue[T, R]) Close() {
	close(q.Jobs)
}
