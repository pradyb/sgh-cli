// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

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
	var consumeWg sync.WaitGroup
	consumeWg.Add(2)

	go func() {
		defer consumeWg.Done()
		for result := range q.Results {
			resultHandler(result)
		}
	}()

	go func() {
		defer consumeWg.Done()
		for err := range q.Errors {
			errorHandler(err)
		}
	}()

	for range make([]int, noOfWorkers) {
		q.wg.Add(1)
		go q.worker(jobHandler)
	}

	q.wg.Wait()

	close(q.Results)
	close(q.Errors)

	consumeWg.Wait()
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
