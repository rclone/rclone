package jobs

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/rc/rcflags"
	"github.com/rclone/rclone/fstest/testy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJobs(t *testing.T) {
	jobs := newJobs()
	assert.Equal(t, 0, len(jobs.jobs))
}

func TestJobsKickExpire(t *testing.T) {
	testy.SkipUnreliable(t)
	jobs := newJobs()
	jobs.opt.JobExpireInterval = time.Millisecond
	assert.Equal(t, false, jobs.expireRunning)
	jobs.kickExpire()
	jobs.mu.Lock()
	assert.Equal(t, true, jobs.expireRunning)
	jobs.mu.Unlock()
	time.Sleep(10 * time.Millisecond)
	jobs.mu.Lock()
	assert.Equal(t, false, jobs.expireRunning)
	jobs.mu.Unlock()
}

func TestJobsExpire(t *testing.T) {
	testy.SkipUnreliable(t)
	wait := make(chan struct{})
	jobs := newJobs()
	jobs.opt.JobExpireInterval = time.Millisecond
	assert.Equal(t, false, jobs.expireRunning)
	job := jobs.NewAsyncJob(func(ctx context.Context, in rc.Params) (rc.Params, error) {
		defer close(wait)
		return in, nil
	}, rc.Params{})
	<-wait
	assert.Equal(t, 1, len(jobs.jobs))
	jobs.Expire()
	assert.Equal(t, 1, len(jobs.jobs))
	jobs.mu.Lock()
	job.mu.Lock()
	job.EndTime = time.Now().Add(-rcflags.Opt.JobExpireDuration - 60*time.Second)
	assert.Equal(t, true, jobs.expireRunning)
	job.mu.Unlock()
	jobs.mu.Unlock()
	time.Sleep(250 * time.Millisecond)
	jobs.mu.Lock()
	assert.Equal(t, false, jobs.expireRunning)
	assert.Equal(t, 0, len(jobs.jobs))
	jobs.mu.Unlock()
}

var noopFn = func(ctx context.Context, in rc.Params) (rc.Params, error) {
	return nil, nil
}

func TestJobsIDs(t *testing.T) {
	jobs := newJobs()
	job1 := jobs.NewAsyncJob(noopFn, rc.Params{})
	job2 := jobs.NewAsyncJob(noopFn, rc.Params{})
	wantIDs := []int64{job1.ID, job2.ID}
	gotIDs := jobs.IDs()
	require.Equal(t, 2, len(gotIDs))
	if gotIDs[0] != wantIDs[0] {
		gotIDs[0], gotIDs[1] = gotIDs[1], gotIDs[0]
	}
	assert.Equal(t, wantIDs, gotIDs)
}

func TestJobsGet(t *testing.T) {
	jobs := newJobs()
	job := jobs.NewAsyncJob(noopFn, rc.Params{})
	assert.Equal(t, job, jobs.Get(job.ID))
	assert.Nil(t, jobs.Get(123123123123))
}

var longFn = func(ctx context.Context, in rc.Params) (rc.Params, error) {
	time.Sleep(1 * time.Hour)
	return nil, nil
}

var shortFn = func(ctx context.Context, in rc.Params) (rc.Params, error) {
	time.Sleep(time.Millisecond)
	return nil, nil
}

var ctxFn = func(ctx context.Context, in rc.Params) (rc.Params, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

const (
	sleepTime      = 100 * time.Millisecond
	floatSleepTime = float64(sleepTime) / 1e9 / 2
)

// sleep for some time so job.Duration is non-0
func sleepJob() {
	time.Sleep(sleepTime)
}

func TestJobFinish(t *testing.T) {
	jobs := newJobs()
	job := jobs.NewAsyncJob(longFn, rc.Params{})
	sleepJob()

	assert.Equal(t, true, job.EndTime.IsZero())
	assert.Equal(t, rc.Params(nil), job.Output)
	assert.Equal(t, 0.0, job.Duration)
	assert.Equal(t, "", job.Error)
	assert.Equal(t, false, job.Success)
	assert.Equal(t, false, job.Finished)

	wantOut := rc.Params{"a": 1}
	job.finish(wantOut, nil)

	assert.Equal(t, false, job.EndTime.IsZero())
	assert.Equal(t, wantOut, job.Output)
	assert.True(t, job.Duration >= floatSleepTime)
	assert.Equal(t, "", job.Error)
	assert.Equal(t, true, job.Success)
	assert.Equal(t, true, job.Finished)

	job = jobs.NewAsyncJob(longFn, rc.Params{})
	sleepJob()
	job.finish(nil, nil)

	assert.Equal(t, false, job.EndTime.IsZero())
	assert.Equal(t, rc.Params{}, job.Output)
	assert.True(t, job.Duration >= floatSleepTime)
	assert.Equal(t, "", job.Error)
	assert.Equal(t, true, job.Success)
	assert.Equal(t, true, job.Finished)

	job = jobs.NewAsyncJob(longFn, rc.Params{})
	sleepJob()
	job.finish(wantOut, errors.New("potato"))

	assert.Equal(t, false, job.EndTime.IsZero())
	assert.Equal(t, wantOut, job.Output)
	assert.True(t, job.Duration >= floatSleepTime)
	assert.Equal(t, "potato", job.Error)
	assert.Equal(t, false, job.Success)
	assert.Equal(t, true, job.Finished)
}

// We've tested the functionality of run() already as it is
// part of NewJob, now just test the panic catching
func TestJobRunPanic(t *testing.T) {
	wait := make(chan struct{})
	boom := func(ctx context.Context, in rc.Params) (rc.Params, error) {
		sleepJob()
		defer close(wait)
		panic("boom")
	}

	jobs := newJobs()
	job := jobs.NewAsyncJob(boom, rc.Params{})
	<-wait
	runtime.Gosched() // yield to make sure job is updated

	// Wait a short time for the panic to propagate
	for i := uint(0); i < 10; i++ {
		job.mu.Lock()
		e := job.Error
		job.mu.Unlock()
		if e != "" {
			break
		}
		time.Sleep(time.Millisecond << i)
	}

	job.mu.Lock()
	assert.Equal(t, false, job.EndTime.IsZero())
	assert.Equal(t, rc.Params{}, job.Output)
	assert.True(t, job.Duration >= floatSleepTime)
	assert.Contains(t, job.Error, "panic received: boom")
	assert.Equal(t, false, job.Success)
	assert.Equal(t, true, job.Finished)
	job.mu.Unlock()
}

func TestJobsNewJob(t *testing.T) {
	jobID = 0
	jobs := newJobs()
	job := jobs.NewAsyncJob(noopFn, rc.Params{})
	assert.Equal(t, int64(1), job.ID)
	assert.Equal(t, job, jobs.Get(1))
	assert.NotEmpty(t, job.Stop)
}

func TestStartJob(t *testing.T) {
	jobID = 0
	out, err := StartAsyncJob(longFn, rc.Params{})
	assert.NoError(t, err)
	assert.Equal(t, rc.Params{"jobid": int64(1)}, out)
}

func TestExecuteJob(t *testing.T) {
	jobID = 0
	_, id, err := ExecuteJob(context.Background(), shortFn, rc.Params{})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), id)
}

func TestExecuteJobErrorPropagation(t *testing.T) {
	jobID = 0

	testErr := errors.New("test error")
	errorFn := func(ctx context.Context, in rc.Params) (out rc.Params, err error) {
		return nil, testErr
	}
	_, _, err := ExecuteJob(context.Background(), errorFn, rc.Params{})
	assert.Equal(t, testErr, err)
}

func TestRcJobStatus(t *testing.T) {
	jobID = 0
	_, err := StartAsyncJob(longFn, rc.Params{})
	assert.NoError(t, err)

	call := rc.Calls.Get("job/status")
	assert.NotNil(t, call)
	in := rc.Params{"jobid": 1}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, float64(1), out["id"])
	assert.Equal(t, "", out["error"])
	assert.Equal(t, false, out["finished"])
	assert.Equal(t, false, out["success"])

	in = rc.Params{"jobid": 123123123}
	_, err = call.Fn(context.Background(), in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "job not found")

	in = rc.Params{"jobidx": 123123123}
	_, err = call.Fn(context.Background(), in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Didn't find key")
}

func TestRcJobList(t *testing.T) {
	jobID = 0
	_, err := StartAsyncJob(longFn, rc.Params{})
	assert.NoError(t, err)

	call := rc.Calls.Get("job/list")
	assert.NotNil(t, call)
	in := rc.Params{}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, rc.Params{"jobids": []int64{1}}, out)
}

func TestRcAsyncJobStop(t *testing.T) {
	jobID = 0
	_, err := StartAsyncJob(ctxFn, rc.Params{})
	assert.NoError(t, err)

	call := rc.Calls.Get("job/stop")
	assert.NotNil(t, call)
	in := rc.Params{"jobid": 1}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.Empty(t, out)

	in = rc.Params{"jobid": 123123123}
	_, err = call.Fn(context.Background(), in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "job not found")

	in = rc.Params{"jobidx": 123123123}
	_, err = call.Fn(context.Background(), in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Didn't find key")

	time.Sleep(10 * time.Millisecond)

	call = rc.Calls.Get("job/status")
	assert.NotNil(t, call)
	in = rc.Params{"jobid": 1}
	out, err = call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, float64(1), out["id"])
	assert.Equal(t, "context canceled", out["error"])
	assert.Equal(t, true, out["finished"])
	assert.Equal(t, false, out["success"])
}

func TestRcSyncJobStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		jobID = 0
		_, id, err := ExecuteJob(ctx, ctxFn, rc.Params{})
		assert.Error(t, err)
		assert.Equal(t, int64(1), id)
	}()

	time.Sleep(10 * time.Millisecond)

	call := rc.Calls.Get("job/stop")
	assert.NotNil(t, call)
	in := rc.Params{"jobid": 1}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.Empty(t, out)

	in = rc.Params{"jobid": 123123123}
	_, err = call.Fn(context.Background(), in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "job not found")

	in = rc.Params{"jobidx": 123123123}
	_, err = call.Fn(context.Background(), in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Didn't find key")

	cancel()
	time.Sleep(10 * time.Millisecond)

	call = rc.Calls.Get("job/status")
	assert.NotNil(t, call)
	in = rc.Params{"jobid": 1}
	out, err = call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, float64(1), out["id"])
	assert.Equal(t, "context canceled", out["error"])
	assert.Equal(t, true, out["finished"])
	assert.Equal(t, false, out["success"])
}
