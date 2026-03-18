package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/rc"
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
	jobs.opt.JobExpireInterval = fs.Duration(time.Millisecond)
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
	ctx := context.Background()
	wait := make(chan struct{})
	jobs := newJobs()
	jobs.opt.JobExpireInterval = fs.Duration(time.Millisecond)
	assert.Equal(t, false, jobs.expireRunning)
	var gotJobID int64
	var gotJob *Job
	job, out, err := jobs.NewJob(ctx, func(ctx context.Context, in rc.Params) (rc.Params, error) {
		defer close(wait)
		var ok bool
		gotJobID, ok = GetJobID(ctx)
		assert.True(t, ok)
		gotJob, ok = GetJob(ctx)
		assert.True(t, ok)
		return in, nil
	}, rc.Params{"_async": true})
	require.NoError(t, err)
	assert.Equal(t, 2, len(out), "check output has jobid and executeId")
	<-wait
	assert.Equal(t, job.ID, gotJobID, "check can get JobID from ctx")
	assert.Equal(t, job, gotJob, "check can get Job from ctx")
	assert.Equal(t, 1, len(jobs.jobs))
	jobs.Expire()
	assert.Equal(t, 1, len(jobs.jobs))
	jobs.mu.Lock()
	job.mu.Lock()
	job.EndTime = time.Now().Add(-time.Duration(rc.Opt.JobExpireDuration) - 60*time.Second)
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
	ctx := context.Background()
	jobs := newJobs()
	job1, _, err := jobs.NewJob(ctx, noopFn, rc.Params{"_async": true})
	require.NoError(t, err)
	job2, _, err := jobs.NewJob(ctx, noopFn, rc.Params{"_async": true})
	require.NoError(t, err)
	wantIDs := []int64{job1.ID, job2.ID}
	gotIDs := jobs.IDs()
	require.Equal(t, 2, len(gotIDs))
	if gotIDs[0] != wantIDs[0] {
		gotIDs[0], gotIDs[1] = gotIDs[1], gotIDs[0]
	}
	assert.Equal(t, wantIDs, gotIDs)
}

func TestJobsExecuteIDs(t *testing.T) {
	ctx := context.Background()
	jobs := newJobs()
	job1, _, err := jobs.NewJob(ctx, noopFn, rc.Params{"_async": true})
	require.NoError(t, err)
	job2, _, err := jobs.NewJob(ctx, noopFn, rc.Params{"_async": true})
	require.NoError(t, err)
	assert.Equal(t, executeID, job1.ExecuteID, "execute ID should match global executeID")
	assert.Equal(t, executeID, job2.ExecuteID, "execute ID should match global executeID")
	assert.True(t, job1.ExecuteID == job2.ExecuteID, "just to be sure, all the jobs share the same executeID")
}

func TestJobsGet(t *testing.T) {
	ctx := context.Background()
	jobs := newJobs()
	job, _, err := jobs.NewJob(ctx, noopFn, rc.Params{"_async": true})
	require.NoError(t, err)
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
	<-ctx.Done()
	return nil, ctx.Err()
}

var ctxParmFn = func(paramCtx context.Context, returnError bool) func(ctx context.Context, in rc.Params) (rc.Params, error) {
	return func(ctx context.Context, in rc.Params) (rc.Params, error) {
		<-paramCtx.Done()
		if returnError {
			return nil, ctx.Err()
		}
		return rc.Params{}, nil
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
	ctx := context.Background()
	jobs := newJobs()
	job, _, err := jobs.NewJob(ctx, longFn, rc.Params{"_async": true})
	require.NoError(t, err)
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

	job, _, err = jobs.NewJob(ctx, longFn, rc.Params{"_async": true})
	require.NoError(t, err)
	sleepJob()
	job.finish(nil, nil)

	assert.Equal(t, false, job.EndTime.IsZero())
	assert.Equal(t, rc.Params{}, job.Output)
	assert.True(t, job.Duration >= floatSleepTime)
	assert.Equal(t, "", job.Error)
	assert.Equal(t, true, job.Success)
	assert.Equal(t, true, job.Finished)

	job, _, err = jobs.NewJob(ctx, longFn, rc.Params{"_async": true})
	require.NoError(t, err)
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
	ctx := context.Background()
	wait := make(chan struct{})
	boom := func(ctx context.Context, in rc.Params) (rc.Params, error) {
		sleepJob()
		defer close(wait)
		panic("boom")
	}

	jobs := newJobs()
	job, _, err := jobs.NewJob(ctx, boom, rc.Params{"_async": true})
	require.NoError(t, err)
	<-wait
	runtime.Gosched() // yield to make sure job is updated

	// Wait a short time for the panic to propagate
	for i := range uint(10) {
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
	ctx := context.Background()
	jobID.Store(0)
	jobs := newJobs()
	job, out, err := jobs.NewJob(ctx, noopFn, rc.Params{"_async": true})
	require.NoError(t, err)
	assert.Equal(t, int64(1), job.ID)
	assert.Equal(t, executeID, job.ExecuteID)
	assert.Equal(t, rc.Params{"jobid": int64(1), "executeId": executeID}, out)
	assert.Equal(t, job, jobs.Get(1))
	assert.NotEmpty(t, job.Stop)
}

func TestStartJob(t *testing.T) {
	ctx := context.Background()
	jobID.Store(0)
	job, out, err := NewJob(ctx, longFn, rc.Params{"_async": true})
	assert.NoError(t, err)
	assert.Equal(t, rc.Params{"jobid": int64(1), "executeId": executeID}, out)
	assert.Equal(t, int64(1), job.ID)
	assert.Equal(t, executeID, job.ExecuteID)
}

func TestExecuteJob(t *testing.T) {
	jobID.Store(0)
	job, out, err := NewJob(context.Background(), shortFn, rc.Params{})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), job.ID)
	assert.Equal(t, rc.Params{}, out)
}

func TestExecuteJobWithConfig(t *testing.T) {
	ctx := context.Background()
	jobID.Store(0)
	called := false
	jobFn := func(ctx context.Context, in rc.Params) (rc.Params, error) {
		ci := fs.GetConfig(ctx)
		assert.Equal(t, 42*fs.Mebi, ci.BufferSize)
		called = true
		return nil, nil
	}
	_, _, err := NewJob(context.Background(), jobFn, rc.Params{
		"_config": rc.Params{
			"BufferSize": "42M",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, true, called)
	// Retest with string parameter
	jobID.Store(0)
	called = false
	_, _, err = NewJob(ctx, jobFn, rc.Params{
		"_config": `{"BufferSize": "42M"}`,
	})
	require.NoError(t, err)
	assert.Equal(t, true, called)
	// Check that wasn't the default
	ci := fs.GetConfig(ctx)
	assert.NotEqual(t, 42*fs.Mebi, ci.BufferSize)
}

func TestExecuteJobWithFilter(t *testing.T) {
	ctx := context.Background()
	called := false
	jobID.Store(0)
	jobFn := func(ctx context.Context, in rc.Params) (rc.Params, error) {
		fi := filter.GetConfig(ctx)
		assert.Equal(t, fs.SizeSuffix(1024), fi.Opt.MaxSize)
		assert.Equal(t, []string{"a", "b", "c"}, fi.Opt.IncludeRule)
		called = true
		return nil, nil
	}
	_, _, err := NewJob(ctx, jobFn, rc.Params{
		"_filter": rc.Params{
			"IncludeRule": []string{"a", "b", "c"},
			"MaxSize":     "1k",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, true, called)
}

func TestExecuteJobWithGroup(t *testing.T) {
	ctx := context.Background()
	jobID.Store(0)
	called := false
	jobFn := func(ctx context.Context, in rc.Params) (rc.Params, error) {
		called = true
		group, found := accounting.StatsGroupFromContext(ctx)
		assert.Equal(t, true, found)
		assert.Equal(t, "myparty", group)
		return nil, nil
	}
	_, _, err := NewJob(ctx, jobFn, rc.Params{
		"_group": "myparty",
	})
	require.NoError(t, err)
	assert.Equal(t, true, called)
}

func TestExecuteJobErrorPropagation(t *testing.T) {
	ctx := context.Background()
	jobID.Store(0)

	testErr := errors.New("test error")
	errorFn := func(ctx context.Context, in rc.Params) (out rc.Params, err error) {
		return nil, testErr
	}
	_, _, err := NewJob(ctx, errorFn, rc.Params{})
	assert.Equal(t, testErr, err)
}

func TestRcJobStatus(t *testing.T) {
	ctx := context.Background()
	jobID.Store(0)
	_, _, err := NewJob(ctx, longFn, rc.Params{"_async": true})
	assert.NoError(t, err)

	call := rc.Calls.Get("job/status")
	assert.NotNil(t, call)
	in := rc.Params{"jobid": 1}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, float64(1), out["id"])
	assert.Equal(t, executeID, out["executeId"])
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
	ctx := context.Background()
	jobID.Store(0)
	_, _, err := NewJob(ctx, longFn, rc.Params{"_async": true})
	assert.NoError(t, err)

	call := rc.Calls.Get("job/list")
	assert.NotNil(t, call)
	in := rc.Params{}
	out1, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, out1)
	assert.Equal(t, executeID, out1["executeId"], "should have executeId")
	assert.Equal(t, []int64{1}, out1["jobids"], "should have job listed")
	assert.Equal(t, []int64{1}, out1["runningIds"], "should have running job")
	assert.Equal(t, []int64{}, out1["finishedIds"], "should not have finished job")

	_, _, err = NewJob(ctx, longFn, rc.Params{"_async": true})
	assert.NoError(t, err)

	call = rc.Calls.Get("job/list")
	assert.NotNil(t, call)
	in = rc.Params{}
	out2, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, out2)
	assert.Equal(t, 2, len(out2["jobids"].([]int64)), "should have all jobs listed")

	assert.Equal(t, out1["executeId"], out2["executeId"], "executeId should be the same")
}

func TestRcAsyncJobStop(t *testing.T) {
	ctx := context.Background()
	jobID.Store(0)
	_, _, err := NewJob(ctx, ctxFn, rc.Params{"_async": true})
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
		jobID.Store(0)
		job, out, err := NewJob(ctx, ctxFn, rc.Params{})
		assert.Error(t, err)
		assert.Equal(t, int64(1), job.ID)
		assert.Equal(t, rc.Params{}, out)
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

func TestRcJobStopGroup(t *testing.T) {
	ctx := context.Background()
	jobID.Store(0)
	_, _, err := NewJob(ctx, ctxFn, rc.Params{
		"_async": true,
		"_group": "myparty",
	})
	require.NoError(t, err)
	_, _, err = NewJob(ctx, ctxFn, rc.Params{
		"_async": true,
		"_group": "myparty",
	})
	require.NoError(t, err)

	call := rc.Calls.Get("job/stopgroup")
	assert.NotNil(t, call)
	in := rc.Params{"group": "myparty"}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.Empty(t, out)

	in = rc.Params{}
	_, err = call.Fn(context.Background(), in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Didn't find key")

	time.Sleep(10 * time.Millisecond)

	call = rc.Calls.Get("job/status")
	assert.NotNil(t, call)
	for i := 1; i <= 2; i++ {
		in = rc.Params{"jobid": i}
		out, err = call.Fn(context.Background(), in)
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, "myparty", out["group"])
		assert.Equal(t, "context canceled", out["error"])
		assert.Equal(t, true, out["finished"])
		assert.Equal(t, false, out["success"])
	}
}

func TestOnFinish(t *testing.T) {
	jobID.Store(0)
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	job, _, err := NewJob(ctx, ctxParmFn(ctx, false), rc.Params{"_async": true})
	assert.NoError(t, err)

	stop, err := OnFinish(job.ID, func() { close(done) })
	defer stop()
	assert.NoError(t, err)

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for OnFinish to fire")
	}
}

func TestOnFinishAlreadyFinished(t *testing.T) {
	jobID.Store(0)
	done := make(chan struct{})
	ctx := t.Context()
	job, _, err := NewJob(ctx, shortFn, rc.Params{})
	assert.NoError(t, err)

	stop, err := OnFinish(job.ID, func() { close(done) })
	defer stop()
	assert.NoError(t, err)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for OnFinish to fire")
	}
}

func TestOnFinishDataRace(t *testing.T) {
	jobID.Store(0)
	job, _, err := NewJob(context.Background(), ctxFn, rc.Params{"_async": true})
	assert.NoError(t, err)
	var expect, got uint64
	finished := make(chan struct{})
	stop, stopped := make(chan struct{}), make(chan struct{})
	go func() {
	Loop:
		for {
			select {
			case <-stop:
				break Loop
			default:
				_, err := OnFinish(job.ID, func() {
					finished <- struct{}{}
				})
				assert.NoError(t, err)
				expect += 1
			}
		}
		close(stopped)
	}()

	time.Sleep(10 * time.Millisecond)
	job.Stop()

	// Wait for the first OnFinish to fire
	<-finished
	got += 1

	// Stop the OnFinish producer
	close(stop)
	<-stopped

	timeout := time.After(5 * time.Second)
	for {
		if got == expect {
			break
		}
		select {
		case <-finished:
			got += 1
		case <-timeout:
			t.Fatal("Timeout waiting for all OnFinish calls to fire")
		}
	}
}

// Register some test rc calls
func init() {
	rc.Add(rc.Call{
		Path:         "test/needs_request",
		NeedsRequest: true,
	})
	rc.Add(rc.Call{
		Path:          "test/needs_response",
		NeedsResponse: true,
	})

}

func TestNewJobFromParams(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		in   rc.Params
		want rc.Params
	}{{
		in: rc.Params{
			"_path": "rc/noop",
			"a":     "potato",
		},
		want: rc.Params{
			"a": "potato",
		},
	}, {
		in: rc.Params{
			"_path": "rc/noop",
			"b":     "sausage",
		},
		want: rc.Params{
			"b": "sausage",
		},
	}, {
		in: rc.Params{
			"_path": "rc/error",
			"e":     "sausage",
		},
		want: rc.Params{
			"error": "arbitrary error on input map[e:sausage]",
			"input": rc.Params{
				"e": "sausage",
			},
			"path":   "rc/error",
			"status": 500,
		},
	}, {
		in: rc.Params{
			"_path": "bad/path",
			"param": "sausage",
		},
		want: rc.Params{
			"error": "couldn't find path \"bad/path\"",
			"input": rc.Params{
				"param": "sausage",
			},
			"path":   "bad/path",
			"status": 404,
		},
	}, {
		in: rc.Params{
			"_path": "test/needs_request",
		},
		want: rc.Params{
			"error":  "can't run path \"test/needs_request\" as it needs the request",
			"input":  rc.Params{},
			"path":   "test/needs_request",
			"status": 400,
		},
	}, {
		in: rc.Params{
			"_path": "test/needs_response",
		},
		want: rc.Params{
			"error":  "can't run path \"test/needs_response\" as it needs the response",
			"input":  rc.Params{},
			"path":   "test/needs_response",
			"status": 400,
		},
	}, {
		in: rc.Params{
			"nopath": "BOOM",
		},
		want: rc.Params{
			"error": "Didn't find key \"_path\" in input",
			"input": rc.Params{
				"nopath": "BOOM",
			},
			"path":   "",
			"status": 400,
		},
	}} {
		got := NewJobFromParams(ctx, test.in)
		assert.Equal(t, test.want, got)
	}
}

func TestNewJobFromBytes(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		in   string
		want string
	}{{
		in: `{
			"_path": "rc/noop",
			"a":     "potato"
}`,
		want: `{
	"a": "potato"
}
`,
	}, {
		in: `{
				"_path": "rc/error",
				"e":     "sausage"
			}`,
		want: `{
	"error": "arbitrary error on input map[e:sausage]",
	"input": {
		"e": "sausage"
	},
	"path": "rc/error",
	"status": 500
}
`,
	}, {
		in: `parse error`,
		want: `{
	"error": "invalid character 'p' looking for beginning of value",
	"input": null,
	"path": "unknown",
	"status": 400
}
`,
	}, {
		in: `"just a string"`,
		want: `{
	"error": "json: cannot unmarshal string into Go value of type rc.Params",
	"input": null,
	"path": "unknown",
	"status": 400
}
`,
	}} {
		got := NewJobFromBytes(ctx, []byte(test.in))
		assert.Equal(t, test.want, string(got))
	}
}

func TestJobsBatch(t *testing.T) {
	ctx := context.Background()

	call := rc.Calls.Get("job/batch")
	assert.NotNil(t, call)

	inJSON := `{
  "inputs": [
    {
      "_path": "rc/noop",
      "a": "potato"
    },
    "bad string",
    {
      "_path": "rc/noop",
      "b": "sausage"
    },
    {
      "_path": "rc/error",
      "e": "sausage"
    },
    {
      "_path": "bad/path",
      "param": "sausage"
    },
    {
      "_path": "test/needs_request"
    },
    {
      "_path": "test/needs_response"
    },
    {
      "nopath": "BOOM"
    }
  ]
}
`
	var in rc.Params
	require.NoError(t, json.Unmarshal([]byte(inJSON), &in))

	wantJSON := `{
  "results": [
    {
      "a": "potato"
    },
    {
      "error": "\"inputs\" items must be objects not string",
      "input": null,
      "path": "unknown",
      "status": 400
    },
    {
      "b": "sausage"
    },
    {
      "error": "arbitrary error on input map[e:sausage]",
      "input": {
        "e": "sausage"
      },
      "path": "rc/error",
      "status": 500
    },
    {
      "error": "couldn't find path \"bad/path\"",
      "input": {
        "param": "sausage"
      },
      "path": "bad/path",
      "status": 404
    },
    {
      "error": "can't run path \"test/needs_request\" as it needs the request",
      "input": {},
      "path": "test/needs_request",
      "status": 400
    },
    {
      "error": "can't run path \"test/needs_response\" as it needs the response",
      "input": {},
      "path": "test/needs_response",
      "status": 400
    },
    {
      "error": "Didn't find key \"_path\" in input",
      "input": {
        "nopath": "BOOM"
      },
      "path": "",
      "status": 400
    }
  ]
}
`

	var want rc.Params
	require.NoError(t, json.Unmarshal([]byte(wantJSON), &want))

	out, err := call.Fn(ctx, in)
	require.NoError(t, err)

	var got rc.Params
	require.NoError(t, rc.Reshape(&got, out))

	assert.Equal(t, want, got)
}

func TestJobsBatchConcurrent(t *testing.T) {
	ctx := context.Background()
	for concurrency := range 10 {
		in := rc.Params{}
		var inputs []any
		var results []rc.Params
		for i := range 100 {
			in := map[string]any{
				"_path": "rc/noop",
				"i":     i,
			}
			inputs = append(inputs, in)
			results = append(results, rc.Params{
				"i": i,
			})
		}
		in["inputs"] = inputs
		want := rc.Params{
			"results": results,
		}

		if concurrency > 0 {
			in["concurrency"] = concurrency
		}
		call := rc.Calls.Get("job/batch")
		assert.NotNil(t, call)

		got, err := call.Fn(ctx, in)
		require.NoError(t, err)

		assert.Equal(t, want, got)
	}

}
