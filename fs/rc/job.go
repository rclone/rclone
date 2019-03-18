// Manage background jobs that the rc is running

package rc

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
)

const (
	// expire the job when it is finished and older than this
	expireDuration = 60 * time.Second
	// inteval to run the expire cache
	expireInterval = 10 * time.Second
)

// Job describes a asynchronous task started via the rc package
type Job struct {
	mu        sync.Mutex
	ID        int64     `json:"id"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	Error     string    `json:"error"`
	Finished  bool      `json:"finished"`
	Success   bool      `json:"success"`
	Duration  float64   `json:"duration"`
	Output    Params    `json:"output"`
}

// Jobs describes a collection of running tasks
type Jobs struct {
	mu             sync.RWMutex
	jobs           map[int64]*Job
	expireInterval time.Duration
	expireRunning  bool
}

var (
	running = newJobs()
	jobID   = int64(0)
)

// newJobs makes a new Jobs structure
func newJobs() *Jobs {
	return &Jobs{
		jobs:           map[int64]*Job{},
		expireInterval: expireInterval,
	}
}

// kickExpire makes sure Expire is running
func (jobs *Jobs) kickExpire() {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	if !jobs.expireRunning {
		time.AfterFunc(jobs.expireInterval, jobs.Expire)
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
		if job.Finished && now.Sub(job.EndTime) > expireDuration {
			delete(jobs.jobs, ID)
		}
		job.mu.Unlock()
	}
	if len(jobs.jobs) != 0 {
		time.AfterFunc(jobs.expireInterval, jobs.Expire)
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

// mark the job as finished
func (job *Job) finish(out Params, err error) {
	job.mu.Lock()
	job.EndTime = time.Now()
	if out == nil {
		out = make(Params)
	}
	job.Output = out
	job.Duration = job.EndTime.Sub(job.StartTime).Seconds()
	if err != nil {
		job.Error = err.Error()
		job.Success = false
	} else {
		job.Error = ""
		job.Success = true
	}
	job.Finished = true
	job.mu.Unlock()
	running.kickExpire() // make sure this job gets expired
}

// run the job until completion writing the return status
func (job *Job) run(fn Func, in Params) {
	defer func() {
		if r := recover(); r != nil {
			job.finish(nil, errors.Errorf("panic received: %v", r))
		}
	}()
	job.finish(fn(in))
}

// NewJob start a new Job off
func (jobs *Jobs) NewJob(fn Func, in Params) *Job {
	job := &Job{
		ID:        atomic.AddInt64(&jobID, 1),
		StartTime: time.Now(),
	}
	go job.run(fn, in)
	jobs.mu.Lock()
	jobs.jobs[job.ID] = job
	jobs.mu.Unlock()
	return job

}

// StartJob starts a new job and returns a Param suitable for output
func StartJob(fn Func, in Params) (Params, error) {
	job := running.NewJob(fn, in)
	out := make(Params)
	out["jobid"] = job.ID
	return out, nil
}

func init() {
	Add(Call{
		Path:  "job/status",
		Fn:    rcJobStatus,
		Title: "Reads the status of the job ID",
		Help: `Parameters
- jobid - id of the job (integer)

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
`,
	})
}

// Returns the status of a job
func rcJobStatus(in Params) (out Params, err error) {
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
	out = make(Params)
	err = Reshape(&out, job)
	if err != nil {
		return nil, errors.Wrap(err, "reshape failed in job status")
	}
	return out, nil
}

func init() {
	Add(Call{
		Path:  "job/list",
		Fn:    rcJobList,
		Title: "Lists the IDs of the running jobs",
		Help: `Parameters - None

Results
- jobids - array of integer job ids
`,
	})
}

// Returns the status of a job
func rcJobList(in Params) (out Params, err error) {
	out = make(Params)
	out["jobids"] = running.IDs()
	return out, nil
}
