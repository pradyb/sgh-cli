package async

import (
	"sync"
)

type ASyncJob[T any] struct {
	Id      int
	JobType string
	JobData T
}

type ASyncJobResult[T, R any] struct {
	Id      int
	JobData T
	Result  R
}

type ASyncJobError[T any] struct {
	Id      int
	JobData T
	Error   error
}

type ASyncJobHandler[T, R any] func(job ASyncJob[T]) (R, error)

type ASyncJobResultHandler[T, R any] func(result ASyncJobResult[T, R])

type ASyncJobErrorHandler[T any] func(err ASyncJobError[T])

type ASyncJobQueue[T, R any] struct {
	Jobs    chan ASyncJob[T]
	Results chan ASyncJobResult[T, R]
	Errors  chan ASyncJobError[T]
	wg      sync.WaitGroup
}

func NewASyncJobQueue[T, R any](jobQueueSize int) *ASyncJobQueue[T, R] {
	return &ASyncJobQueue[T, R]{
		Jobs:    make(chan ASyncJob[T], jobQueueSize),
		Results: make(chan ASyncJobResult[T, R], jobQueueSize),
		Errors:  make(chan ASyncJobError[T], jobQueueSize),
	}
}

func (q *ASyncJobQueue[T, R]) Start(jobHandler ASyncJobHandler[T, R], resultHandler ASyncJobResultHandler[T, R], errorHandler ASyncJobErrorHandler[T], noOfWorkers int) {
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

func (q *ASyncJobQueue[T, R]) worker(jobHandler ASyncJobHandler[T, R]) {
	defer q.wg.Done()
	for job := range q.Jobs {
		result, err := jobHandler(job)
		if err != nil {
			q.Errors <- ASyncJobError[T]{Id: job.Id, Error: err, JobData: job.JobData}
		} else {
			q.Results <- ASyncJobResult[T, R]{Id: job.Id, Result: result, JobData: job.JobData}
		}
	}
}

func (q *ASyncJobQueue[T, R]) AddJob(job ASyncJob[T]) {
	q.Jobs <- job
}

func (q *ASyncJobQueue[T, R]) Close() {
	close(q.Jobs)
}
