package cluster

import (
	"context"
	"encoding/json"
	"path"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/rc/jobs"
	"github.com/rclone/rclone/lib/random"
)

const maxWorkersDone = 16 // maximum jobs in the done list

// Worker describes a single instance of a cluster worker.
type Worker struct {
	jobs   *Jobs
	cancel func()         // stop bg job
	wg     sync.WaitGroup //  bg job finished
	id     string         // id of this worker
	status string         // place it stores it status

	jobsMu  sync.Mutex
	running map[string]struct{} // IDs of the jobs being processed
	done    []string            // IDs of finished jobs
}

// WorkerStatus shows the status of this worker including jobs
// running.
type WorkerStatus struct {
	ID      string               `json:"id"`
	Running map[string]rc.Params `json:"running"` // Job ID => accounting.RemoteStats
	Done    map[string]bool      `json:"done"`    // Job ID => finished status
	Updated time.Time            `json:"updated"`
}

// NewWorker creates a new cluster from the config in ctx.
//
// It may return nil for no cluster is configured.
func NewWorker(ctx context.Context) (*Worker, error) {
	ci := fs.GetConfig(ctx)
	if ci.Cluster == "" {
		return nil, nil
	}
	jobs, err := NewJobs(ctx)
	if err != nil {
		return nil, err
	}
	w := &Worker{
		jobs:    jobs,
		id:      ci.ClusterID,
		running: make(map[string]struct{}),
	}
	if w.id == "" {
		w.id = random.String(10)
	}
	w.status = path.Join(clusterStatus, w.id+".json")

	// Start the background workers
	bgCtx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.wg.Add(1)
	go w.runJobs(bgCtx)
	w.wg.Add(1)
	go w.runStatus(bgCtx)

	fs.Logf(w.jobs.f, "Started cluster worker")

	return w, nil
}

// Check to see if a job exists and run it if available
func (w *Worker) checkJobs(ctx context.Context) {
	name, obj, err := w.jobs.getJob(ctx, w.id)
	if err != nil {
		fs.Errorf(nil, "check jobs get: %v", err)
		return
	}
	if obj == nil {
		return // no jobs available
	}

	// make a stats group for this job
	ctx = accounting.WithStatsGroup(ctx, name)

	// Add job ID
	w.jobsMu.Lock()
	w.running[name] = struct{}{}
	w.jobsMu.Unlock()
	fs.Infof(nil, "write jobID %q", name)

	// Remove job ID on exit
	defer func() {
		w.jobsMu.Lock()
		delete(w.running, name)
		w.done = append(w.done, name)
		if len(w.done) > maxWorkersDone {
			w.done = w.done[len(w.done)-maxWorkersDone : len(w.done)]
		}
		w.jobsMu.Unlock()
	}()

	fs.Debugf(nil, "cluster: processing pending job %q", name)
	inBuf, err := w.jobs.readFile(ctx, obj)
	if err != nil {
		fs.Errorf(nil, "check jobs read: %v", err)
		w.jobs.finish(ctx, obj, "input-error", false)
		return
	}
	outBuf := jobs.NewJobFromBytes(ctx, inBuf)
	remote := path.Join(clusterDone, name+".json")
	err = w.jobs.writeFile(ctx, remote, time.Now(), outBuf)
	if err != nil {
		fs.Errorf(nil, "check jobs failed to write output: %v", err)
		return
	}
	w.jobs.finish(ctx, obj, "input-ok", true)
	fs.Debugf(nil, "cluster: processed pending job %q", name)
}

// Run the background process to pick up jobs
func (w *Worker) runJobs(ctx context.Context) {
	defer w.wg.Done()
	checkJobs := time.NewTicker(clusterCheckJobsInterval)
	defer checkJobs.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-checkJobs.C:
			w.checkJobs(ctx)
		}
	}
}

// Write the worker status
func (w *Worker) writeStatus(ctx context.Context) {
	// Create the worker status from the jobIDs and the short stats
	status := WorkerStatus{
		ID:      w.id,
		Running: make(map[string]rc.Params),
		Updated: time.Now(),
		Done:    make(map[string]bool),
	}
	w.jobsMu.Lock()
	for _, jobID := range w.done {
		status.Done[jobID] = true
	}
	for jobID := range w.running {
		fs.Infof(nil, "read jobID %q", jobID)
		si := accounting.StatsGroup(ctx, jobID)
		out, err := si.RemoteStats(true)
		if err != nil {
			fs.Errorf(nil, "cluster: write status: stats: %v", err)
			status.Running[jobID] = rc.Params{}
		} else {
			status.Running[jobID] = out
		}
		status.Done[jobID] = false
	}
	w.jobsMu.Unlock()

	// Write the stats to a file
	buf, err := json.MarshalIndent(status, "", "\t")
	if err != nil {
		fs.Errorf(nil, "cluster: write status: json: %w", err)
		return
	}
	err = w.jobs.writeFile(ctx, w.status, status.Updated, buf)
	if err != nil {
		fs.Errorf(nil, "cluster: write status: %w", err)
	}
}

// Remove the worker status
func (w *Worker) clearStatus(ctx context.Context) {
	err := w.jobs.removeFile(ctx, w.status)
	if err != nil {
		fs.Errorf(nil, "cluster: clear status: %w", err)
	}
}

// Run the background process to write status
func (w *Worker) runStatus(ctx context.Context) {
	defer w.wg.Done()
	w.writeStatus(ctx)
	defer w.clearStatus(ctx)
	writeStatus := time.NewTicker(clusterWriteStatusInterval)
	defer writeStatus.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-writeStatus.C:
			t0 := time.Now()
			w.writeStatus(ctx)
			fs.Debugf(nil, "write status took %v at %v", time.Since(t0), t0)
		}
	}
}

// Shutdown the worker regardless of whether it has work to process or not.
func (w *Worker) Shutdown(ctx context.Context) error {
	w.cancel()
	w.wg.Wait()
	return nil
}
