package proton

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ProtonMail/gluon/async"
)

// ErrJobCancelled indicates the job was cancelled.
var ErrJobCancelled = errors.New("job cancelled by surrounding context")

// Pool is a worker pool that handles input of type In and returns results of type Out.
type Pool[In comparable, Out any] struct {
	queue        *async.QueuedChannel[*job[In, Out]]
	wg           sync.WaitGroup
	panicHandler async.PanicHandler
}

// doneFunc must be called to free up pool resources.
type doneFunc func()

// New returns a new pool.
func NewPool[In comparable, Out any](size int, panicHandler async.PanicHandler, work func(context.Context, In) (Out, error)) *Pool[In, Out] {
	pool := &Pool[In, Out]{
		queue: async.NewQueuedChannel[*job[In, Out]](0, 0, panicHandler, "gpa-pool"),
	}

	for i := 0; i < size; i++ {
		pool.wg.Add(1)

		go func() {
			defer async.HandlePanic(pool.panicHandler)

			defer pool.wg.Done()

			for job := range pool.queue.GetChannel() {
				select {
				case <-job.ctx.Done():
					job.postFailure(ErrJobCancelled)

				default:
					res, err := work(job.ctx, job.req)
					if err != nil {
						job.postFailure(err)
					} else {
						job.postSuccess(res)
					}

					job.waitDone()
				}
			}
		}()
	}

	return pool
}

// Process submits jobs to the pool. The callback provides access to the result, or an error if one occurred.
func (pool *Pool[In, Out]) Process(ctx context.Context, reqs []In, fn func(int, In, Out, error) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wg      sync.WaitGroup
		errList []error
		lock    sync.Mutex
	)

	for i, req := range reqs {
		req := req

		wg.Add(1)

		go func(index int) {
			defer async.HandlePanic(pool.panicHandler)

			defer wg.Done()

			job, done, err := pool.newJob(ctx, req)
			if err != nil {
				lock.Lock()
				defer lock.Unlock()

				// Cancel ongoing jobs.
				cancel()

				// Collect the error.
				errList = append(errList, err)

				return
			}

			defer done()

			res, err := job.result()

			if err := fn(index, req, res, err); err != nil {
				lock.Lock()
				defer lock.Unlock()

				// Cancel ongoing jobs.
				cancel()

				// Collect the error.
				errList = append(errList, err)
			}
		}(i)
	}

	wg.Wait()

	// TODO: Join the errors somehow?
	if len(errList) > 0 {
		return errList[0]
	}

	return nil
}

// ProcessAll submits jobs to the pool. All results are returned once available.
func (pool *Pool[In, Out]) ProcessAll(ctx context.Context, reqs []In) ([]Out, error) {
	data := make([]Out, len(reqs))

	if err := pool.Process(ctx, reqs, func(index int, req In, res Out, err error) error {
		if err != nil {
			return err
		}

		data[index] = res

		return nil
	}); err != nil {
		return nil, err
	}

	return data, nil
}

// ProcessOne submits one job to the pool and returns the result.
func (pool *Pool[In, Out]) ProcessOne(ctx context.Context, req In) (Out, error) {
	job, done, err := pool.newJob(ctx, req)
	if err != nil {
		var o Out
		return o, err
	}

	defer done()

	return job.result()
}

func (pool *Pool[In, Out]) Done() {
	pool.queue.Close()
	pool.wg.Wait()
}

// newJob submits a job to the pool. It returns a job handle and a DoneFunc.
// The job handle allows the job result to be obtained. The DoneFunc is used to mark the job as done,
// which frees up the worker in the pool for reuse.
func (pool *Pool[In, Out]) newJob(ctx context.Context, req In) (*job[In, Out], doneFunc, error) {
	job := newJob[In, Out](ctx, req)

	if !pool.queue.Enqueue(job) {
		return nil, nil, fmt.Errorf("pool closed")
	}

	return job, func() { close(job.done) }, nil
}
