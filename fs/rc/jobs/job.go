// Package jobs manages background jobs that the rc is running.
package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/rc"
	"golang.org/x/sync/errgroup"
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
	ExecuteID string    `json:"executeId"`
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

	wantLogs bool       // set if _logs was passed in
	logLevel slog.Level // return logs of this level or more severe
	logStart int64      // sequence number in log.Recent when the job started
	logEnd   int64      // sequence number in log.Recent when the job finished
}

// logs returns the logs made while the job was running so far or nil
// if they weren't asked for with _logs.
//
// It returns the logs from sequence number since, or from the start
// of the job if since is before that, and at most limit entries if
// limit > 0.
//
// As logs aren't currently attributed to jobs this includes the logs
// from anything else which was running at the same time.
//
// Call with job.mu held.
func (job *Job) logs(since int64, limit int) rc.Params {
	if !job.wantLogs {
		return nil
	}
	// Never read from before the job started, so since 0 means
	// the start of the job and lost counts what the job made.
	since = max(since, job.logStart)
	logEnd := int64(math.MaxInt64)
	if job.Finished {
		logEnd = job.logEnd
	}
	entries, next, lost := log.Recent.Get(since, logEnd, job.logLevel, limit)
	return rc.Params{
		"entries": entries,
		"next":    next,
		"lost":    lost,
	}
}

// mark the job as finished
func (job *Job) finish(out rc.Params, err error) {
	job.mu.Lock()
	job.EndTime = time.Now()
	job.logEnd = log.Recent.Seq()
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

func (job *Job) removeListener(fn *func()) {
	job.mu.Lock()
	defer job.mu.Unlock()
	for i, ln := range job.listeners {
		if ln == fn {
			job.listeners = slices.Delete(job.listeners, i, i+1)
			return
		}
	}
}

// OnFinish adds listener to job that will be triggered when job is finished.
// It returns a function to cancel listening.
func (job *Job) OnFinish(fn func()) func() {
	job.mu.Lock()
	defer job.mu.Unlock()
	if job.Finished {
		go fn()
	} else {
		job.listeners = append(job.listeners, &fn)
	}
	return func() { job.removeListener(&fn) }
}

// run the job until completion writing the return status
func (job *Job) run(ctx context.Context, fn rc.Func, in rc.Params) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			// Finish the job before logging the stack trace so the
			// log is outside the window returned by _logs.
			job.finish(nil, fmt.Errorf("panic received: %v", r))
			// Log the full stack trace server-side only - it must not
			// be returned to the rc caller as it leaks internal paths,
			// dependency versions and memory addresses.
			fs.Errorf(nil, "rc: job %d panic: %v\n%s", job.ID, r, string(stack))
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
	running = newJobs()
	jobID   atomic.Int64
	// executeID is a unique ID for this rclone execution
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
		time.AfterFunc(time.Duration(jobs.opt.JobExpireInterval), jobs.Expire)
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
		if job.Finished && now.Sub(job.EndTime) > time.Duration(jobs.opt.JobExpireDuration) {
			delete(jobs.jobs, ID)
		}
		job.mu.Unlock()
	}
	if len(jobs.jobs) != 0 {
		time.AfterFunc(time.Duration(jobs.opt.JobExpireInterval), jobs.Expire)
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

// Stats returns the IDs of the running and finished jobs
func (jobs *Jobs) Stats() (running []int64, finished []int64) {
	jobs.mu.RLock()
	defer jobs.mu.RUnlock()
	running = []int64{}
	finished = []int64{}
	for jobID := range jobs.jobs {
		if jobs.jobs[jobID].Finished {
			finished = append(finished, jobID)
		} else {
			running = append(running, jobID)
		}
	}
	return running, finished
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

type jobKeyType struct{}

// Key for adding jobs to ctx
var jobKey = jobKeyType{}

// LogsRequested returns true if in has _logs set to anything other
// than false.
//
// This doesn't check the value of _logs is valid.
func LogsRequested(in rc.Params) bool {
	wantLogs, err := in.GetBool("_logs")
	if rc.IsErrParamNotFound(err) {
		return false
	}
	return err != nil || wantLogs
}

// See if _logs is set returning whether the logs are wanted and the
// minimum level of logs to return.
//
// _logs can be a boolean or a log level, e.g. "INFO".
func getLogs(in rc.Params) (wantLogs bool, level slog.Level, err error) {
	level = slog.LevelDebug
	wantLogs = LogsRequested(in)
	_, boolErr := in.GetBool("_logs")
	levelString, levelErr := in.GetString("_logs")
	delete(in, "_logs") // remove the logs parameter after parsing
	if !wantLogs {
		return false, level, nil
	}
	if !log.Recent.Enabled() {
		return false, level, log.ErrBufferDisabled
	}
	// _logs is true or a level here
	if boolErr != nil {
		if levelErr != nil {
			return false, level, levelErr
		}
		level, err = log.ParseLevel(levelString)
		if err != nil {
			return false, level, rc.NewErrParamInvalid(fmt.Errorf("_logs must be a boolean or a log level: %w", err))
		}
		if level >= fs.SlogLevelOff {
			return false, level, rc.NewErrParamInvalid(fmt.Errorf("_logs level can't be %q", levelString))
		}
	}
	return wantLogs, level, nil
}

// NewJob creates a Job and executes it, possibly in the background if _async is set
func (jobs *Jobs) NewJob(ctx context.Context, fn rc.Func, in rc.Params) (job *Job, out rc.Params, err error) {
	logStart := log.Recent.Seq() // before anything is logged for this job
	id := jobID.Add(1)
	in = in.Copy() // copy input so we can change it

	ctx, isAsync, err := getAsync(ctx, in)
	if err != nil {
		return nil, nil, err
	}

	ctx, err = rc.AddConfig(ctx, in)
	if err != nil {
		return nil, nil, err
	}

	ctx, err = rc.AddFilter(ctx, in)
	if err != nil {
		return nil, nil, err
	}

	ctx, group, err := getGroup(ctx, in, id)
	if err != nil {
		return nil, nil, err
	}

	wantLogs, logLevel, err := getLogs(in)
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
		ExecuteID: executeID,
		Group:     group,
		StartTime: time.Now(),
		Stop:      stop,
		wantLogs:  wantLogs,
		logLevel:  logLevel,
		logStart:  logStart,
	}

	jobs.mu.Lock()
	jobs.jobs[job.ID] = job
	jobs.mu.Unlock()

	// Add the job to the context
	ctx = context.WithValue(ctx, jobKey, job)

	// Mark the context as created from an rc request
	ctx = fs.WithRCRequest(ctx)

	if isAsync {
		go job.run(ctx, fn, in)
		out = make(rc.Params)
		out["jobid"] = job.ID
		out["executeId"] = job.ExecuteID
		err = nil
	} else {
		job.run(ctx, fn, in)
		out = job.Output
		err = job.realErr
		if wantLogs {
			job.mu.Lock()
			logs := job.logs(-1, 0)
			job.mu.Unlock()
			if err != nil {
				err = rc.NewErrorWithLogs(err, logs)
			} else {
				// Don't add the logs to job.Output
				out = out.Copy()
				out["_logs"] = logs
			}
		}
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
- since - return logs with sequence numbers starting from this - optional
    - Defaults to the start of the job, and is clamped to it, so the
      logs made before the job started are never returned.
- limit - maximum number of log entries to return - optional, defaults to 1000, 0 for no limit

Results:

- finished - boolean
- duration - time in seconds that the job ran for
- endTime - time the job finished (e.g. "2018-10-26T18:50:20.528746884+01:00")
- error - error from the job or empty string for no error
- finished - boolean whether the job has finished or not
- id - as passed in above
- executeId - rclone instance ID (changes after restart); combined with id uniquely identifies a job
- startTime - time the job started (e.g. "2018-10-26T18:50:20.528336039+01:00")
- success - boolean - true for success false otherwise
- output - output of the job as would have been returned if called synchronously
    - _logs - the logs made while the job was running if it was started with _logs
        - entries - array of log entries, oldest first
        - next - pass this as since to carry on reading the logs from where this call finished
        - lost - the number of log entries which had already been dropped from the log buffer
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
	if job.wantLogs {
		since, err := in.GetInt64("since")
		if rc.IsErrParamNotFound(err) {
			since = -1
		} else if err != nil {
			return nil, err
		}
		limit, err := in.GetInt64("limit")
		if rc.IsErrParamNotFound(err) {
			limit = 1000
		} else if err != nil {
			return nil, err
		}
		// Put the logs in the output so it is the same as would
		// have been returned if called synchronously
		output, ok := out["output"].(map[string]any)
		if !ok {
			output = map[string]any{}
		}
		output["_logs"] = job.logs(since, int(min(limit, math.MaxInt32)))
		out["output"] = output
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
- runningIds - array of integer job ids that are running
- finishedIds - array of integer job ids that are finished
`,
	})
}

// Returns list of job ids.
func rcJobList(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	out = make(rc.Params)
	out["jobids"] = running.IDs()
	runningIDs, finishedIDs := running.Stats()
	out["runningIds"] = runningIDs
	out["finishedIds"] = finishedIDs
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

// NewJobFromParams creates an rc job rc.Params.
//
// The JSON blob should contain a _path entry.
//
// It returns a rc.Params as output which may be an error.
func NewJobFromParams(ctx context.Context, in rc.Params) (out rc.Params) {
	path := "unknown"

	// Return an rc error blob
	rcError := func(err error, status int) rc.Params {
		fs.Errorf(nil, "rc: %q: error: %v", path, err)
		out, _ = rc.Error(path, in, err, status)
		return out
	}

	// Find the call
	path, err := in.GetString("_path")
	if err != nil {
		return rcError(err, http.StatusNotFound)
	}
	delete(in, "_path")
	call := rc.Calls.Get(path)
	if call == nil {
		return rcError(fmt.Errorf("couldn't find path %q", path), http.StatusNotFound)
	}
	if call.NeedsRequest {
		return rcError(fmt.Errorf("can't run path %q as it needs the request", path), http.StatusBadRequest)
	}
	if call.NeedsResponse {
		return rcError(fmt.Errorf("can't run path %q as it needs the response", path), http.StatusBadRequest)
	}

	// Pass on the group if one is set in the context and it isn't set in the input.
	if _, found := in["_group"]; !found {
		group, ok := accounting.StatsGroupFromContext(ctx)
		if ok {
			in["_group"] = group
		}
	}

	fs.Debugf(nil, "rc: %q: with parameters %+v", path, in)
	_, out, err = NewJob(ctx, call.Fn, in)
	if err != nil {
		return rcError(err, http.StatusInternalServerError)
	}
	if out == nil {
		out = make(rc.Params)
	}

	fs.Debugf(nil, "rc: %q: reply %+v: %v", path, out, err)
	return out
}

// NewJobFromBytes creates an rc job from a JSON blob as bytes.
//
// The JSON blob should contain a _path entry.
//
// It returns a JSON blob as output which may be an error.
func NewJobFromBytes(ctx context.Context, inBuf []byte) (outBuf []byte) {
	var in rc.Params
	var out rc.Params

	// Parse a JSON blob from the input
	err := json.Unmarshal(inBuf, &in)
	if err != nil {
		out, _ = rc.Error("unknown", in, err, http.StatusBadRequest)
	} else {
		out = NewJobFromParams(ctx, in)
	}

	var w bytes.Buffer
	err = rc.WriteJSON(&w, out)
	if err != nil {
		fs.Errorf(nil, "rc: NewJobFromBytes: failed to write JSON output: %v", err)
		return []byte(`{"error":"failed to write JSON output"}`)
	}
	return w.Bytes()
}

func init() {
	rc.Add(rc.Call{
		Path:  "job/batch",
		Fn:    rcBatch,
		Title: "Run a batch of rclone rc commands concurrently.",
		Help: strings.ReplaceAll(`
This takes the following parameters:

- concurrency - int - do this many commands concurrently. Defaults to |--transfers| if not set.
- inputs - an list of inputs to the commands with an extra |_path| parameter

|||json
{
    "_path": "rc/path",
    "param1": "parameter for the path as documented",
    "param2": "parameter for the path as documented, etc",
}
|||

The inputs may use |_async|, |_group|, |_config|, |_filter| and |_logs| as normal when using the rc.

Returns:

- results - a list of results from the commands with one entry for each in inputs.

For example:

|||sh
rclone rc job/batch --json '{
  "inputs": [
    {
      "_path": "rc/noop",
      "parameter": "OK"
    },
    {
      "_path": "rc/error",
      "parameter": "BAD"
    }
  ]
}
'
|||

Gives the result:

|||json
{
  "results": [
    {
      "parameter": "OK"
    },
    {
      "error": "arbitrary error on input map[parameter:BAD]",
      "input": {
        "parameter": "BAD"
      },
      "path": "rc/error",
      "status": 500
    }
  ]
}
|||
`, "|", "`"),
	})
}

/*
// Run a single batch job
func runBatchJob(ctx context.Context, inputAny any) (out rc.Params, err error) {
	var in rc.Params
	path := "unknown"
	defer func() {
		if err != nil {
			out, _ = rc.Error(path, in, err, http.StatusInternalServerError)
		}
	}()

	// get the inputs to the job
	input, ok := inputAny.(map[string]any)
	if !ok {
		return nil, rc.NewErrParamInvalid(fmt.Errorf("\"inputs\" items must be objects not %T", inputAny))
	}
	in = rc.Params(input)
	path, err = in.GetString("_path")
	if err != nil {
		return nil, err
	}
	delete(in, "_path")
	call := rc.Calls.Get(path)

	// Check call
	if call == nil {
		return nil, rc.NewErrParamInvalid(fmt.Errorf("path %q does not exist", path))
	}
	path = call.Path
	if call.NeedsRequest {
		return nil, rc.NewErrParamInvalid(fmt.Errorf("can't run path %q as it needs the request", path))
	}
	if call.NeedsResponse {
		return nil, rc.NewErrParamInvalid(fmt.Errorf("can't run path %q as it needs the response", path))
	}

	// Run the job
	_, out, err = NewJob(ctx, call.Fn, in)
	if err != nil {
		return nil, err
	}

	// Reshape (serialize then deserialize) the data so it is in the form expected
	err = rc.Reshape(&out, out)
	if err != nil {
		return nil, err
	}
	return out, nil
        }
*/

// Batch the registered commands
func rcBatch(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	out = make(rc.Params)

	// Read inputs
	inputsAny, err := in.Get("inputs")
	if err != nil {
		return nil, err
	}
	inputs, ok := inputsAny.([]any)
	if !ok {
		return nil, rc.NewErrParamInvalid(fmt.Errorf("expecting list key %q (was %T)", "inputs", inputsAny))
	}

	// Read concurrency
	concurrency, err := in.GetInt("concurrency")
	if rc.IsErrParamNotFound(err) {
		ci := fs.GetConfig(ctx)
		concurrency = ci.Transfers
	} else if err != nil {
		return nil, err
	}

	// Prepare outputs
	results := make([]rc.Params, len(inputs))
	out["results"] = results

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)
	for i, inputAny := range inputs {
		input, ok := inputAny.(map[string]any)
		if !ok {
			results[i], _ = rc.Error("unknown", nil, fmt.Errorf("\"inputs\" items must be objects not %T", inputAny), http.StatusBadRequest)
			continue
		}
		in := rc.Params(input)
		if concurrency <= 1 {
			results[i] = NewJobFromParams(ctx, in)
		} else {
			g.Go(func() error {
				results[i] = NewJobFromParams(gCtx, in)
				return nil
			})
		}
	}
	_ = g.Wait()
	return out, nil
}
