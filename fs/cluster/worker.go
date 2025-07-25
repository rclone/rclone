package cluster

import (
	"context"
	"path"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc/jobs"
	"github.com/rclone/rclone/lib/random"
)

// Worker describes a single instance of a cluster worker.
type Worker struct {
	jobs   *Jobs
	cancel func()         // stop bg job
	wg     sync.WaitGroup //  bg job finished
	id     string         // id of this worker
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
		jobs: jobs,
		id:   ci.ClusterID,
	}
	if w.id == "" {
		w.id = random.String(10)
	}

	// Start the background worker
	bgCtx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.wg.Add(1)
	go w.run(bgCtx)

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
	fs.Debugf(nil, "cluster: processing pending job %q", name)
	inBuf, err := w.jobs.readFile(ctx, obj)
	if err != nil {
		fs.Errorf(nil, "check jobs read: %v", err)
		w.jobs.finish(ctx, obj, "input-error", false)
		return
	}
	w.jobs.finish(ctx, obj, "input-ok", true)
	outBuf := jobs.NewJobFromBytes(ctx, inBuf)
	remote := path.Join(clusterDone, name+".json")
	err = w.jobs.writeFile(ctx, remote, time.Now(), outBuf)
	if err != nil {
		fs.Errorf(nil, "check jobs failed to write output: %v", err)
		return
	}
	fs.Debugf(nil, "cluster: processed pending job %q", name)
}

// Run the background process
func (w *Worker) run(ctx context.Context) {
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

// Shutdown the worker regardless of whether it has work to process or not.
func (w *Worker) Shutdown(ctx context.Context) error {
	w.cancel()
	w.wg.Wait()
	return nil
}
