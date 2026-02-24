package proton

import "context"

type job[In, Out any] struct {
	ctx context.Context
	req In

	res chan Out
	err chan error

	done chan struct{}
}

func newJob[In, Out any](ctx context.Context, req In) *job[In, Out] {
	return &job[In, Out]{
		ctx:  ctx,
		req:  req,
		res:  make(chan Out),
		err:  make(chan error),
		done: make(chan struct{}),
	}
}

func (job *job[In, Out]) result() (Out, error) {
	return <-job.res, <-job.err
}

func (job *job[In, Out]) postSuccess(res Out) {
	close(job.err)
	job.res <- res
}

func (job *job[In, Out]) postFailure(err error) {
	close(job.res)
	job.err <- err
}

func (job *job[In, Out]) waitDone() {
	<-job.done
}
