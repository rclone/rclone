// Manage background jobs that the rc is running

package jobs

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/rc"
)

// Job describes a asynchronous task started via the rc package
type Job struct {
	observers sync.Map

	mu        sync.Mutex
	ID        int64     `json:"id"`
	Group     string    `json:"group"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	Error     string    `json:"error"`
	Finished  bool      `json:"finished"`
	Success   bool      `json:"success"`
	Duration  float64   `json:"duration"`
	Output    rc.Params `json:"output"`
	Stop      func()    `json:"-"`

	// realErr is the Error before printing it as a string, it's used to return
	// the real error to the upper application layers while still printing the
	// string error message.
	realErr error
}

// WaitForFinish will block until provided context is canceled or job is
// finished.
func (j *Job) WaitForFinish(ctx context.Context) {
	j.mu.Lock()
	if j.Finished {
		return
	}
	j.mu.Unlock()

	waitChan := make(chan struct{})
	j.observers.Store(waitChan, struct{}{})

	select {
	case <-ctx.Done():
		return
	case <-waitChan:
		return
	}
}

func (j *Job) notifyFinished() {
	j.observers.Range(func(key, value interface{}) bool {
		if key == nil {
			return false
		}

		close(key.(chan struct{}))
		return true
	})
}

// RcParams marshals job stats into rc.Params.
func (j *Job) RcParams() (rc.Params, error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	out := make(rc.Params)
	err := rc.Reshape(&out, j)
	return out, err
}

// mark the job as finished
func (j *Job) finish(out rc.Params, err error) {
	j.mu.Lock()
	j.EndTime = time.Now()
	if out == nil {
		out = make(rc.Params)
	}
	j.Output = out
	j.Duration = j.EndTime.Sub(j.StartTime).Seconds()
	if err != nil {
		j.realErr = err
		j.Error = err.Error()
		j.Success = false
	} else {
		j.realErr = nil
		j.Error = ""
		j.Success = true
	}
	j.Finished = true
	j.mu.Unlock()
	j.notifyFinished()
	running.kickExpire() // make sure this job gets expired
}

// run the job until completion writing the return status
func (j *Job) run(ctx context.Context, fn rc.Func, in rc.Params) {
	defer func() {
		if r := recover(); r != nil {
			j.finish(nil, errors.Errorf("panic received: %v \n%s", r, string(debug.Stack())))
		}
	}()
	j.finish(fn(ctx, in))
}

// Jobs describes a collection of running tasks
type Jobs struct {
	mu            sync.RWMutex
	jobs          map[int64]*Job
	opt           *rc.Options
	expireRunning bool
}

var (
	running = newJobs()
	jobID   = int64(0)
)

// newJobs makes a new Jobs structure
func newJobs() *Jobs {
	return &Jobs{
		jobs: map[int64]*Job{},
		opt:  &rc.DefaultOpt,
	}
}

// SetOpt sets the options when they are known
func SetOpt(opt *rc.Options) {
	running.opt = opt
}

// SetInitialJobID allows for setting jobID before starting any jobs.
func SetInitialJobID(id int64) {
	if !atomic.CompareAndSwapInt64(&jobID, 0, id) {
		panic("Setting jobID is only possible before starting any jobs")
	}
}

// kickExpire makes sure Expire is running
func (jobs *Jobs) kickExpire() {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	if !jobs.expireRunning {
		time.AfterFunc(jobs.opt.JobExpireInterval, jobs.Expire)
		jobs.expireRunning = true
	}
}

// Expire expires any jobs that haven't been collected
func (jobs *Jobs) Expire() {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	now := time.Now()
	for ID, job := range jobs.jobs {
		job.mu.Lock()
		if job.Finished && now.Sub(job.EndTime) > jobs.opt.JobExpireDuration {
			delete(jobs.jobs, ID)
		}
		job.mu.Unlock()
	}
	if len(jobs.jobs) != 0 {
		time.AfterFunc(jobs.opt.JobExpireInterval, jobs.Expire)
		jobs.expireRunning = true
	} else {
		jobs.expireRunning = false
	}
}

// IDs returns the IDs of the running jobs
func (jobs *Jobs) IDs() (IDs []int64) {
	jobs.mu.RLock()
	defer jobs.mu.RUnlock()
	IDs = []int64{}
	for ID := range jobs.jobs {
		IDs = append(IDs, ID)
	}
	return IDs
}

// Get a job with a given ID or nil if it doesn't exist
func (jobs *Jobs) Get(ID int64) *Job {
	jobs.mu.RLock()
	defer jobs.mu.RUnlock()
	return jobs.jobs[ID]
}

func getGroup(in rc.Params) string {
	// Check to see if the group is set
	group, err := in.GetString("_group")
	if rc.NotErrParamNotFound(err) {
		fs.Errorf(nil, "Can't get _group param %+v", err)
	}
	delete(in, "_group")
	return group
}

// NewAsyncJob start a new asynchronous Job off
func (jobs *Jobs) NewAsyncJob(fn rc.Func, in rc.Params) *Job {
	id := atomic.AddInt64(&jobID, 1)

	group := getGroup(in)
	if group == "" {
		group = fmt.Sprintf("job/%d", id)
	}
	ctx := accounting.WithStatsGroup(context.Background(), group)
	ctx, cancel := context.WithCancel(ctx)
	stop := func() {
		cancel()
		// Wait for cancel to propagate before returning.
		<-ctx.Done()
	}
	job := &Job{
		ID:        id,
		Group:     group,
		StartTime: time.Now(),
		Stop:      stop,
	}
	jobs.mu.Lock()
	jobs.jobs[job.ID] = job
	jobs.mu.Unlock()
	go job.run(ctx, fn, in)
	return job
}

// NewSyncJob start a new synchronous Job off
func (jobs *Jobs) NewSyncJob(ctx context.Context, in rc.Params) (*Job, context.Context) {
	id := atomic.AddInt64(&jobID, 1)
	group := getGroup(in)
	if group == "" {
		group = fmt.Sprintf("job/%d", id)
	}
	ctxG := accounting.WithStatsGroup(ctx, fmt.Sprintf("job/%d", id))
	ctx, cancel := context.WithCancel(ctxG)
	stop := func() {
		cancel()
		// Wait for cancel to propagate before returning.
		<-ctx.Done()
	}
	job := &Job{
		ID:        id,
		Group:     group,
		StartTime: time.Now(),
		Stop:      stop,
	}
	jobs.mu.Lock()
	jobs.jobs[job.ID] = job
	jobs.mu.Unlock()
	return job, ctx
}

// StartAsyncJob starts a new job asynchronously and returns a Param suitable
// for output.
func StartAsyncJob(fn rc.Func, in rc.Params) (rc.Params, error) {
	job := running.NewAsyncJob(fn, in)
	out := make(rc.Params)
	out["jobid"] = job.ID
	return out, nil
}

// ExecuteJob executes new job synchronously and returns a Param suitable for
// output.
func ExecuteJob(ctx context.Context, fn rc.Func, in rc.Params) (rc.Params, int64, error) {
	job, ctx := running.NewSyncJob(ctx, in)
	job.run(ctx, fn, in)
	return job.Output, job.ID, job.realErr
}

func init() {
	rc.Add(rc.Call{
		Path:  "job/status",
		Fn:    rcJobStatus,
		Title: "Reads the status of the job ID",
		Help: `Parameters

- jobid - id of the job (integer)
- long_poll - optional millisecond duration to wait for job to complete before returning 

Results

- finished - boolean
- duration - time in seconds that the job ran for
- endTime - time the job finished (eg "2018-10-26T18:50:20.528746884+01:00")
- error - error from the job or empty string for no error
- finished - boolean whether the job has finished or not
- id - as passed in above
- startTime - time the job started (eg "2018-10-26T18:50:20.528336039+01:00")
- success - boolean - true for success false otherwise
- output - output of the job as would have been returned if called synchronously
- progress - output of the progress related to the underlying job
`,
	})
}

// Returns the status of a job
func rcJobStatus(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	jobID, err := in.GetInt64("jobid")
	if err != nil {
		return nil, err
	}
	job := running.Get(jobID)
	if job == nil {
		return nil, errors.New("job not found")
	}
	longPoll, err := in.GetInt64("long_poll")
	if err != nil && !rc.IsErrParamNotFound(err) {
		return nil, err
	}

	if longPoll > 0 {
		ctxWait, cancel := context.WithTimeout(ctx, time.Duration(longPoll)*time.Millisecond)
		job.WaitForFinish(ctxWait)
		cancel()
	}
	out, err = job.RcParams()
	if err != nil {
		return nil, errors.Wrap(err, "reshape failed in job status")
	}
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:  "job/list",
		Fn:    rcJobList,
		Title: "Lists the IDs of the running jobs",
		Help: `Parameters - None

Results

- jobids - array of integer job ids
`,
	})
}

// Returns list of job ids.
func rcJobList(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	out = make(rc.Params)
	out["jobids"] = running.IDs()
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:  "job/stop",
		Fn:    rcJobStop,
		Title: "Stop the running job",
		Help: `Parameters

- jobid - id of the job (integer)
`,
	})
}

// Stops the running job.
func rcJobStop(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	jobID, err := in.GetInt64("jobid")
	if err != nil {
		return nil, err
	}
	job := running.Get(jobID)
	if job == nil {
		return nil, errors.New("job not found")
	}
	job.mu.Lock()
	defer job.mu.Unlock()
	out = make(rc.Params)
	job.Stop()
	return out, nil
}
