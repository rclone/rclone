// Package jobs manages background jobs that the rc is running.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/rc"
)

// Fill in these to avoid circular dependencies
func init() {
	cache.JobOnFinish = OnFinish
	cache.JobGetJobID = GetJobID
}

// Job describes an asynchronous task started via the rc package
type Job struct {
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
	listeners []*func()

	// realErr is the Error before printing it as a string, it's used to return
	// the real error to the upper application layers while still printing the
	// string error message.
	realErr error
}

// mark the job as finished
func (job *Job) finish(out rc.Params, err error) {
	job.mu.Lock()
	job.EndTime = time.Now()
	if out == nil {
		out = make(rc.Params)
	}
	job.Output = out
	job.Duration = job.EndTime.Sub(job.StartTime).Seconds()
	if err != nil {
		job.realErr = err
		job.Error = err.Error()
		job.Success = false
	} else {
		job.realErr = nil
		job.Error = ""
		job.Success = true
	}
	job.Finished = true

	// Notify listeners that the job is finished
	for i := range job.listeners {
		go (*job.listeners[i])()
	}

	job.mu.Unlock()
	running.kickExpire() // make sure this job gets expired
}

func (job *Job) addListener(fn *func()) {
	job.mu.Lock()
	defer job.mu.Unlock()
	job.listeners = append(job.listeners, fn)
}

func (job *Job) removeListener(fn *func()) {
	job.mu.Lock()
	defer job.mu.Unlock()
	for i, ln := range job.listeners {
		if ln == fn {
			job.listeners = append(job.listeners[:i], job.listeners[i+1:]...)
			return
		}
	}
}

// OnFinish adds listener to job that will be triggered when job is finished.
// It returns a function to cancel listening.
func (job *Job) OnFinish(fn func()) func() {
	if job.Finished {
		fn()
	} else {
		job.addListener(&fn)
	}
	return func() { job.removeListener(&fn) }
}

// run the job until completion writing the return status
func (job *Job) run(ctx context.Context, fn rc.Func, in rc.Params) {
	defer func() {
		if r := recover(); r != nil {
			job.finish(nil, fmt.Errorf("panic received: %v \n%s", r, string(debug.Stack())))
		}
	}()
	job.finish(fn(ctx, in))
}

// Jobs describes a collection of running tasks
type Jobs struct {
	mu            sync.RWMutex
	jobs          map[int64]*Job
	opt           *rc.Options
	expireRunning bool
}

var (
	running   = newJobs()
	jobID     atomic.Int64
	executeID = uuid.New().String()
)

// newJobs makes a new Jobs structure
func newJobs() *Jobs {
	return &Jobs{
		jobs: map[int64]*Job{},
		opt:  &rc.Opt,
	}
}

// SetOpt sets the options when they are known
func SetOpt(opt *rc.Options) {
	running.opt = opt
}

// SetInitialJobID allows for setting jobID before starting any jobs.
func SetInitialJobID(id int64) {
	if !jobID.CompareAndSwap(0, id) {
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

// Check to see if the group is set
func getGroup(ctx context.Context, in rc.Params, id int64) (context.Context, string, error) {
	group, err := in.GetString("_group")
	if rc.NotErrParamNotFound(err) {
		return ctx, "", err
	}
	delete(in, "_group")
	if group == "" {
		group = fmt.Sprintf("job/%d", id)
	}
	ctx = accounting.WithStatsGroup(ctx, group)
	return ctx, group, nil
}

// See if _async is set returning a boolean and a possible new context
func getAsync(ctx context.Context, in rc.Params) (context.Context, bool, error) {
	isAsync, err := in.GetBool("_async")
	if rc.NotErrParamNotFound(err) {
		return ctx, false, err
	}
	delete(in, "_async") // remove the async parameter after parsing
	if isAsync {
		// unlink this job from the current context
		ctx = context.Background()
	}
	return ctx, isAsync, nil
}

// See if _config is set and if so adjust ctx to include it
func getConfig(ctx context.Context, in rc.Params) (context.Context, error) {
	if _, ok := in["_config"]; !ok {
		return ctx, nil
	}
	ctx, ci := fs.AddConfig(ctx)
	err := in.GetStruct("_config", ci)
	if err != nil {
		return ctx, err
	}
	delete(in, "_config") // remove the parameter
	return ctx, nil
}

// See if _filter is set and if so adjust ctx to include it
func getFilter(ctx context.Context, in rc.Params) (context.Context, error) {
	if _, ok := in["_filter"]; !ok {
		return ctx, nil
	}
	// Copy of the current filter options
	opt := filter.GetConfig(ctx).Opt
	// Update the options from the parameter
	err := in.GetStruct("_filter", &opt)
	if err != nil {
		return ctx, err
	}
	fi, err := filter.NewFilter(&opt)
	if err != nil {
		return ctx, err
	}
	ctx = filter.ReplaceConfig(ctx, fi)
	delete(in, "_filter") // remove the parameter
	return ctx, nil
}

type jobKeyType struct{}

// Key for adding jobs to ctx
var jobKey = jobKeyType{}

// NewJob creates a Job and executes it, possibly in the background if _async is set
func (jobs *Jobs) NewJob(ctx context.Context, fn rc.Func, in rc.Params) (job *Job, out rc.Params, err error) {
	id := jobID.Add(1)
	in = in.Copy() // copy input so we can change it

	ctx, isAsync, err := getAsync(ctx, in)
	if err != nil {
		return nil, nil, err
	}

	ctx, err = getConfig(ctx, in)
	if err != nil {
		return nil, nil, err
	}

	ctx, err = getFilter(ctx, in)
	if err != nil {
		return nil, nil, err
	}

	ctx, group, err := getGroup(ctx, in, id)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	stop := func() {
		cancel()
		// Wait for cancel to propagate before returning.
		<-ctx.Done()
	}
	job = &Job{
		ID:        id,
		Group:     group,
		StartTime: time.Now(),
		Stop:      stop,
	}

	jobs.mu.Lock()
	jobs.jobs[job.ID] = job
	jobs.mu.Unlock()

	// Add the job to the context
	ctx = context.WithValue(ctx, jobKey, job)

	if isAsync {
		go job.run(ctx, fn, in)
		out = make(rc.Params)
		out["jobid"] = job.ID
		err = nil
	} else {
		job.run(ctx, fn, in)
		out = job.Output
		err = job.realErr
	}
	return job, out, err
}

// NewJob creates a Job and executes it on the global job queue,
// possibly in the background if _async is set
func NewJob(ctx context.Context, fn rc.Func, in rc.Params) (job *Job, out rc.Params, err error) {
	return running.NewJob(ctx, fn, in)
}

// OnFinish adds listener to jobid that will be triggered when job is finished.
// It returns a function to cancel listening.
func OnFinish(jobID int64, fn func()) (func(), error) {
	job := running.Get(jobID)
	if job == nil {
		return func() {}, errors.New("job not found")
	}
	return job.OnFinish(fn), nil
}

// GetJob gets the Job from the context if possible
func GetJob(ctx context.Context) (job *Job, ok bool) {
	job, ok = ctx.Value(jobKey).(*Job)
	return job, ok
}

// GetJobID gets the Job from the context if possible
func GetJobID(ctx context.Context) (jobID int64, ok bool) {
	job, ok := GetJob(ctx)
	if !ok {
		return -1, ok
	}
	return job.ID, true
}

func init() {
	rc.Add(rc.Call{
		Path:  "job/status",
		Fn:    rcJobStatus,
		Title: "Reads the status of the job ID",
		Help: `Parameters:

- jobid - id of the job (integer).

Results:

- finished - boolean
- duration - time in seconds that the job ran for
- endTime - time the job finished (e.g. "2018-10-26T18:50:20.528746884+01:00")
- error - error from the job or empty string for no error
- finished - boolean whether the job has finished or not
- id - as passed in above
- startTime - time the job started (e.g. "2018-10-26T18:50:20.528336039+01:00")
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
	job.mu.Lock()
	defer job.mu.Unlock()
	out = make(rc.Params)
	err = rc.Reshape(&out, job)
	if err != nil {
		return nil, fmt.Errorf("reshape failed in job status: %w", err)
	}
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:  "job/list",
		Fn:    rcJobList,
		Title: "Lists the IDs of the running jobs",
		Help: `Parameters: None.

Results:

- executeId - string id of rclone executing (change after restart)
- jobids - array of integer job ids (starting at 1 on each restart)
`,
	})
}

// Returns list of job ids.
func rcJobList(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	out = make(rc.Params)
	out["jobids"] = running.IDs()
	out["executeId"] = executeID
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:  "job/stop",
		Fn:    rcJobStop,
		Title: "Stop the running job",
		Help: `Parameters:

- jobid - id of the job (integer).
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

func init() {
	rc.Add(rc.Call{
		Path:  "job/stopgroup",
		Fn:    rcGroupStop,
		Title: "Stop all running jobs in a group",
		Help: `Parameters:

- group - name of the group (string).
`,
	})
}

// Stops all running jobs in a group
func rcGroupStop(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	group, err := in.GetString("group")
	if err != nil {
		return nil, err
	}
	running.mu.RLock()
	defer running.mu.RUnlock()
	for _, job := range running.jobs {
		if job.Group == group {
			job.mu.Lock()
			job.Stop()
			job.mu.Unlock()
		}
	}
	out = make(rc.Params)
	return out, nil
}
