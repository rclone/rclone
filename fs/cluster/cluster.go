// Package cluster implements a machanism to distribute work over a
// cluster of rclone instances.
package cluster

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/lib/atexit"
)

// ErrClusterNotConfigured is returned from creation functions.
var ErrClusterNotConfigured = errors.New("cluster is not configured")

// Cluster describes the workings of the current cluster.
type Cluster struct {
	jobs        *Jobs
	id          string
	batchFiles  int
	batchSize   fs.SizeSuffix
	cleanup     fs.ClusterCleanup    // how we cleanup cluster files
	_config     rc.Params            // for rc
	_filter     rc.Params            // for rc
	cancel      func()               // stop bg job
	wg          sync.WaitGroup       // bg job finished
	quit        chan struct{}        // signal graceful stop
	sync        chan chan<- struct{} // sync the current jobs
	quitWorkers bool                 // if set, send workers a stop signal on Shutdown

	mu           sync.Mutex
	currentBatch Batch
	inflight     map[string]Batch
	shutdown     bool
}

// Batch is a collection of rc tasks to do
type Batch struct {
	size   int64       // size in batch
	Path   string      `json:"_path"`
	Inputs []rc.Params `json:"inputs"`
	Config rc.Params   `json:"_config,omitempty"`
	Filter rc.Params   `json:"_filter,omitempty"`

	trs   []*accounting.Transfer // transfer for each Input
	sizes []int64                // sizes for each Input
}

// BatchResult has the results of the batch as received.
type BatchResult struct {
	Results []rc.Params `json:"results"`

	// Error returns
	Error  string `json:"error"`
	Status int    `json:"status"`
	Input  string `json:"input"`
	Path   string `json:"path"`
}

// NewCluster creates a new cluster from the config in ctx.
//
// It may return nil for no cluster is configured.
func NewCluster(ctx context.Context) (*Cluster, error) {
	ci := fs.GetConfig(ctx)
	if ci.Cluster == "" {
		return nil, nil
	}
	jobs, err := NewJobs(ctx)
	if err != nil {
		return nil, err
	}
	c := &Cluster{
		jobs:        jobs,
		id:          ci.ClusterID,
		quitWorkers: ci.ClusterQuitWorkers,
		batchFiles:  ci.ClusterBatchFiles,
		batchSize:   ci.ClusterBatchSize,
		cleanup:     ci.ClusterCleanup,
		quit:        make(chan struct{}),
		sync:        make(chan chan<- struct{}),
		inflight:    make(map[string]Batch),
	}

	// Configure _config
	configParams, err := fs.ConfigOptionsInfo.NonDefaultRC(ci)
	if err != nil {
		return nil, fmt.Errorf("failed to read global config: %w", err)
	}
	// Remove any global cluster config
	for k := range configParams {
		if strings.HasPrefix(k, "Cluster") {
			delete(configParams, k)
		}
	}
	if len(configParams) != 0 {
		fs.Debugf(nil, "Overridden global config: %#v", configParams)
	}
	c._config = rc.Params(configParams)

	// Configure _filter
	fi := filter.GetConfig(ctx)
	if !fi.InActive() {
		filterParams, err := filter.OptionsInfo.NonDefaultRC(fi)
		if err != nil {
			return nil, fmt.Errorf("failed to read filter config: %w", err)
		}
		fs.Debugf(nil, "Overridden filter config: %#v", filterParams)
		c._filter = rc.Params(filterParams)
	}

	err = c.jobs.createDirectoryStructure(ctx)
	if err != nil {
		return nil, err
	}

	// Start the background worker
	bgCtx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.wg.Add(1)
	go c.run(bgCtx)

	fs.Logf(c.jobs.f, "Started cluster master")

	return c, nil
}

var (
	globalClusterMu sync.Mutex
	globalCluster   *Cluster
)

// GetCluster starts or gets a cluster.
//
// If no cluster is configured or the cluster can't be started then it
// returns nil.
func GetCluster(ctx context.Context) *Cluster {
	globalClusterMu.Lock()
	defer globalClusterMu.Unlock()

	if globalCluster != nil {
		return globalCluster
	}

	cluster, err := NewCluster(ctx)
	if err != nil {
		fs.Errorf(nil, "Failed to start cluster: %v", err)
		return nil
	}
	if cluster != nil {
		atexit.Register(func() {
			err := cluster.Shutdown(context.Background())
			if err != nil {
				fs.Errorf(nil, "Failed to shutdown cluster: %v", err)
			}
		})
	}

	globalCluster = cluster
	return globalCluster
}

// Send the current batch for processing
//
// call with c.mu held
func (c *Cluster) sendBatch(ctx context.Context) (err error) {
	// Do nothing if the batch is empty
	if len(c.currentBatch.Inputs) == 0 {
		return nil
	}

	// Get and reset current batch
	b := c.currentBatch
	c.currentBatch = Batch{}

	b.Path = "job/batch"
	b.Config = c._config
	b.Filter = c._filter

	// write the pending job
	name, err := c.jobs.writeJob(ctx, clusterPending, &b)
	if err != nil {
		return err
	}

	fs.Infof(name, "written cluster batch file")
	c.inflight[name] = b
	return nil
}

// Add the command to the current batch
func (c *Cluster) addToBatch(ctx context.Context, obj fs.Object, in rc.Params, size int64, tr *accounting.Transfer) (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.shutdown {
		return errors.New("internal error: can't add file to Shutdown cluster")
	}

	c.currentBatch.Inputs = append(c.currentBatch.Inputs, in)
	c.currentBatch.size += size
	c.currentBatch.trs = append(c.currentBatch.trs, tr)
	c.currentBatch.sizes = append(c.currentBatch.sizes, size)

	if c.currentBatch.size >= int64(c.batchSize) || len(c.currentBatch.Inputs) >= c.batchFiles {
		err = c.sendBatch(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// Move does operations.Move via the cluster.
//
// Move src object to dst or fdst if nil.  If dst is nil then it uses
// remote as the name of the new object.
func (c *Cluster) Move(ctx context.Context, fdst fs.Fs, dst fs.Object, remote string, src fs.Object) (err error) {
	tr := accounting.Stats(ctx).NewTransfer(src, fdst)
	if operations.SkipDestructive(ctx, src, "cluster move") {
		in := tr.Account(ctx, nil)
		in.DryRun(src.Size())
		tr.Done(ctx, nil)
		return nil
	}
	fsrc, ok := src.Fs().(fs.Fs)
	if !ok {
		err = errors.New("internal error: cluster move: can't cast src.Fs() to fs.Fs")
		tr.Done(ctx, err)
		return err
	}
	in := rc.Params{
		"_path":     "operations/movefile",
		"dstFs":     fs.ConfigStringFull(fdst),
		"dstRemote": remote,
		"srcFs":     fs.ConfigStringFull(fsrc),
		"srcRemote": src.Remote(),
	}
	if dst != nil {
		in["dstRemote"] = dst.Remote()
	}
	return c.addToBatch(ctx, src, in, src.Size(), tr)
}

// Copy does operations.Copy via the cluster.
//
// Copy src object to dst or fdst if nil.  If dst is nil then it uses
// remote as the name of the new object.
func (c *Cluster) Copy(ctx context.Context, fdst fs.Fs, dst fs.Object, remote string, src fs.Object) (err error) {
	tr := accounting.Stats(ctx).NewTransfer(src, fdst)
	if operations.SkipDestructive(ctx, src, "cluster copy") {
		in := tr.Account(ctx, nil)
		in.DryRun(src.Size())
		tr.Done(ctx, nil)
		return nil
	}
	fsrc, ok := src.Fs().(fs.Fs)
	if !ok {
		err = errors.New("internal error: cluster copy: can't cast src.Fs() to fs.Fs")
		tr.Done(ctx, err)
		return err
	}
	in := rc.Params{
		"_path":     "operations/copyfile",
		"dstFs":     fs.ConfigStringFull(fdst),
		"dstRemote": remote,
		"srcFs":     fs.ConfigStringFull(fsrc),
		"srcRemote": src.Remote(),
	}
	if dst != nil {
		in["dstRemote"] = dst.Remote()
	}
	return c.addToBatch(ctx, src, in, src.Size(), tr)
}

// DeleteFile does operations.DeleteFile via the cluster
//
// If useBackupDir is set and --backup-dir is in effect then it moves
// the file to there instead of deleting
func (c *Cluster) DeleteFile(ctx context.Context, dst fs.Object) (err error) {
	tr := accounting.Stats(ctx).NewCheckingTransfer(dst, "deleting")
	err = accounting.Stats(ctx).DeleteFile(ctx, dst.Size())
	if err != nil {
		tr.Done(ctx, err)
		return err
	}
	if operations.SkipDestructive(ctx, dst, "cluster delete") {
		tr.Done(ctx, nil)
		return
	}
	fdst, ok := dst.Fs().(fs.Fs)
	if !ok {
		err = errors.New("internal error: cluster delete: can't cast dst.Fs() to fs.Fs")
		tr.Done(ctx, nil)
		return err
	}
	in := rc.Params{
		"_path":  "operations/deletefile",
		"fs":     fs.ConfigStringFull(fdst),
		"remote": dst.Remote(),
	}
	return c.addToBatch(ctx, dst, in, 0, tr)
}

// processCompletedJob loads the job and checks it off
func (c *Cluster) processCompletedJob(ctx context.Context, obj fs.Object) error {
	name := path.Base(obj.Remote())
	name, _ = strings.CutSuffix(name, ".json")
	fs.Debugf(nil, "cluster: processing completed job %q", name)

	var output BatchResult
	err := c.jobs.readJob(ctx, obj, &output)
	if err != nil {
		return fmt.Errorf("check jobs read: %w", err)
	}

	c.mu.Lock()
	input, ok := c.inflight[name]
	// FIXME delete or save job
	if !ok {
		for k := range c.inflight {
			fs.Debugf(nil, "key %q", k)
		}
		c.mu.Unlock()
		return fmt.Errorf("check jobs: job %q not found", name)
	}
	c.mu.Unlock()

	// Delete the inflight entry when batch is processed
	defer func() {
		c.mu.Lock()
		delete(c.inflight, name)
		c.mu.Unlock()
	}()

	// Check job
	if output.Error != "" {
		return fmt.Errorf("cluster: failed to run batch job: %s (%d)", output.Error, output.Status)
	}
	if len(input.Inputs) != len(output.Results) {
		return fmt.Errorf("cluster: input had %d jobs but output had %d", len(input.Inputs), len(output.Results))
	}

	// Run through the batch and mark operations as successful or not
	for i := range input.Inputs {
		in := input.Inputs[i]
		tr := input.trs[i]
		size := input.sizes[i]
		out := output.Results[i]
		errorString, hasError := out["error"]
		var err error
		if hasError && errorString != "" {
			err = fmt.Errorf("cluster: worker error: %s (%v)", errorString, out["status"])
		}
		if err == nil && in["_path"] == "operations/movefile" {
			accounting.Stats(ctx).Renames(1)
		}
		acc := tr.Account(ctx, nil)
		acc.AccountReadN(size)
		tr.Done(ctx, err)
		remote, ok := in["dstRemote"]
		if !ok {
			remote = in["remote"]
		}
		if err == nil {
			fs.Infof(remote, "cluster %s successful", in["_path"])
		} else {
			fs.Errorf(remote, "cluster %s failed: %v", in["_path"], err)
		}
	}

	return nil
}

// checkJobs sees if there are any completed jobs
func (c *Cluster) checkJobs(ctx context.Context) {
	objs, err := c.jobs.listDir(ctx, clusterDone)
	if err != nil {
		fs.Errorf(nil, "cluster: get completed job list failed: %v", err)
		return
	}
	for _, obj := range objs {
		err := c.processCompletedJob(ctx, obj)
		status := "output-ok"
		ok := true
		if err != nil {
			status = "output-failed"
			ok = false
			fs.Errorf(nil, "cluster: process completed job failed: %v", err)
		}
		c.jobs.finish(ctx, obj, status, ok)
	}
}

// Run the background process
func (c *Cluster) run(ctx context.Context) {
	defer c.wg.Done()
	checkJobs := time.NewTicker(clusterCheckJobsInterval)
	defer checkJobs.Stop()
	var syncedChans []chan<- struct{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.quit:
			fs.Debugf(nil, "cluster: quit request received")
			return
		case synced := <-c.sync:
			syncedChans = append(syncedChans, synced)
			fs.Debugf(nil, "cluster: sync request received")
		case <-checkJobs.C:
		}
		c.checkJobs(ctx)
		if len(syncedChans) > 0 {
			c.mu.Lock()
			n := len(c.inflight)
			c.mu.Unlock()
			if n == 0 {
				fs.Debugf(nil, "cluster: synced")
				for _, synced := range syncedChans {
					synced <- struct{}{}
				}
				syncedChans = nil
			}
		}
	}
}

// Sync the cluster.
//
// Call this when all job items have been added to the cluster.
//
// This will wait for any outstanding jobs to finish regardless of who
// put them in
func (c *Cluster) Sync(ctx context.Context) error {
	// Flush any outstanding
	c.mu.Lock()
	err := c.sendBatch(ctx)
	c.mu.Unlock()

	// Wait for the cluster to be empty
	done := make(chan struct{})
	c.sync <- done
	<-done

	return err
}

// Shutdown the cluster.
//
// Call this when all job items have been added to the cluster.
//
// This will wait for any outstanding jobs to finish.
func (c *Cluster) Shutdown(ctx context.Context) (err error) {
	c.mu.Lock()
	inBatch := len(c.currentBatch.Inputs)
	inFlight := len(c.inflight)
	shutdown := c.shutdown
	c.shutdown = true
	c.mu.Unlock()

	if inBatch > 0 {
		err = errors.Join(nil, fmt.Errorf("%d items batched on cluster shutdown", inBatch))
	}
	if inFlight > 0 {
		err = errors.Join(nil, fmt.Errorf("%d items in flight on cluster shutdown", inFlight))
	}
	if shutdown {
		fs.Debugf(nil, "cluster: already shutdown")
		return nil
	}
	c.quit <- struct{}{}
	fs.Debugf(nil, "Waiting for cluster to finish")
	c.wg.Wait()

	// Send a quit job
	if c.quitWorkers {
		fs.Logf(nil, "Sending quit to workers")
		quitErr := c.jobs.writeQuitJob(ctx, clusterPending)
		if quitErr != nil {
			err = errors.Join(err, fmt.Errorf("shutdown quit: %w", quitErr))
		}
	}
	return err
}

// Abort the cluster and any outstanding jobs.
func (c *Cluster) Abort() {
	c.cancel()
	c.wg.Wait()
}
