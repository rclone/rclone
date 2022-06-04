// This file contains the implementation of the sync batcher for uploads
//
// Dropbox rules say you can start as many batches as you want, but
// you may only have one batch being committed and must wait for the
// batch to be finished before committing another.

package dropbox

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/async"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/files"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/atexit"
)

const (
	maxBatchSize          = 1000                   // max size the batch can be
	defaultTimeoutSync    = 500 * time.Millisecond // kick off the batch if nothing added for this long (sync)
	defaultTimeoutAsync   = 10 * time.Second       // kick off the batch if nothing added for this long (ssync)
	defaultBatchSizeAsync = 100                    // default batch size if async
)

// batcher holds info about the current items waiting for upload
type batcher struct {
	f        *Fs                 // Fs this batch is part of
	mode     string              // configured batch mode
	size     int                 // maximum size for batch
	timeout  time.Duration       // idle timeout for batch
	async    bool                // whether we are using async batching
	in       chan batcherRequest // incoming items to batch
	closed   chan struct{}       // close to indicate batcher shut down
	atexit   atexit.FnHandle     // atexit handle
	shutOnce sync.Once           // make sure we shutdown once only
	wg       sync.WaitGroup      // wait for shutdown
}

// batcherRequest holds an incoming request with a place for a reply
type batcherRequest struct {
	commitInfo *files.UploadSessionFinishArg
	result     chan<- batcherResponse
}

// Return true if batcherRequest is the quit request
func (br *batcherRequest) isQuit() bool {
	return br.commitInfo == nil
}

// Send this to get the engine to quit
var quitRequest = batcherRequest{}

// batcherResponse holds a response to be delivered to clients waiting
// for a batch to complete.
type batcherResponse struct {
	err   error
	entry *files.FileMetadata
}

// newBatcher creates a new batcher structure
func newBatcher(ctx context.Context, f *Fs, mode string, size int, timeout time.Duration) (*batcher, error) {
	// fs.Debugf(f, "Creating batcher with mode %q, size %d, timeout %v", mode, size, timeout)
	if size > maxBatchSize || size < 0 {
		return nil, fmt.Errorf("dropbox: batch size must be < %d and >= 0 - it is currently %d", maxBatchSize, size)
	}

	async := false

	switch mode {
	case "sync":
		if size <= 0 {
			ci := fs.GetConfig(ctx)
			size = ci.Transfers
		}
		if timeout <= 0 {
			timeout = defaultTimeoutSync
		}
	case "async":
		if size <= 0 {
			size = defaultBatchSizeAsync
		}
		if timeout <= 0 {
			timeout = defaultTimeoutAsync
		}
		async = true
	case "off":
		size = 0
	default:
		return nil, fmt.Errorf("dropbox: batch mode must be sync|async|off not %q", mode)
	}

	b := &batcher{
		f:       f,
		mode:    mode,
		size:    size,
		timeout: timeout,
		async:   async,
		in:      make(chan batcherRequest, size),
		closed:  make(chan struct{}),
	}
	if b.Batching() {
		b.atexit = atexit.Register(b.Shutdown)
		b.wg.Add(1)
		go b.commitLoop(context.Background())
	}
	return b, nil

}

// Batching returns true if batching is active
func (b *batcher) Batching() bool {
	return b.size > 0
}

// finishBatch commits the batch, returning a batch status to poll or maybe complete
func (b *batcher) finishBatch(ctx context.Context, items []*files.UploadSessionFinishArg) (complete *files.UploadSessionFinishBatchResult, err error) {
	var arg = &files.UploadSessionFinishBatchArg{
		Entries: items,
	}
	err = b.f.pacer.Call(func() (bool, error) {
		complete, err = b.f.srv.UploadSessionFinishBatchV2(arg)
		// If error is insufficient space then don't retry
		if e, ok := err.(files.UploadSessionFinishAPIError); ok {
			if e.EndpointError != nil && e.EndpointError.Path != nil && e.EndpointError.Path.Tag == files.WriteErrorInsufficientSpace {
				err = fserrors.NoRetryError(err)
				return false, err
			}
		}
		// after the first chunk is uploaded, we retry everything
		return err != nil, err
	})
	if err != nil {
		return nil, fmt.Errorf("batch commit failed: %w", err)
	}
	return complete, nil
}

// finishBatchJobStatus waits for the batch to complete returning completed entries
func (b *batcher) finishBatchJobStatus(ctx context.Context, launchBatchStatus *files.UploadSessionFinishBatchLaunch) (complete *files.UploadSessionFinishBatchResult, err error) {
	if launchBatchStatus.AsyncJobId == "" {
		return nil, errors.New("wait for batch completion: empty job ID")
	}
	var batchStatus *files.UploadSessionFinishBatchJobStatus
	sleepTime := 100 * time.Millisecond
	const maxSleepTime = 1 * time.Second
	startTime := time.Now()
	try := 1
	for {
		remaining := time.Duration(b.f.opt.BatchCommitTimeout) - time.Since(startTime)
		if remaining < 0 {
			break
		}
		err = b.f.pacer.Call(func() (bool, error) {
			batchStatus, err = b.f.srv.UploadSessionFinishBatchCheck(&async.PollArg{
				AsyncJobId: launchBatchStatus.AsyncJobId,
			})
			return shouldRetry(ctx, err)
		})
		if err != nil {
			fs.Debugf(b.f, "Wait for batch: sleeping for %v after error: %v: try %d remaining %v", sleepTime, err, try, remaining)
		} else {
			if batchStatus.Tag == "complete" {
				fs.Debugf(b.f, "Upload batch completed in %v", time.Since(startTime))
				return batchStatus.Complete, nil
			}
			fs.Debugf(b.f, "Wait for batch: sleeping for %v after status: %q: try %d remaining %v", sleepTime, batchStatus.Tag, try, remaining)
		}
		time.Sleep(sleepTime)
		sleepTime *= 2
		if sleepTime > maxSleepTime {
			sleepTime = maxSleepTime
		}
		try++
	}
	if err == nil {
		err = errors.New("batch didn't complete")
	}
	return nil, fmt.Errorf("wait for batch failed after %d tries in %v: %w", try, time.Since(startTime), err)
}

// commit a batch
func (b *batcher) commitBatch(ctx context.Context, items []*files.UploadSessionFinishArg, results []chan<- batcherResponse) (err error) {
	// If commit fails then signal clients if sync
	var signalled = b.async
	defer func() {
		if err != nil && signalled {
			// Signal to clients that there was an error
			for _, result := range results {
				result <- batcherResponse{err: err}
			}
		}
	}()
	desc := fmt.Sprintf("%s batch length %d starting with: %s", b.mode, len(items), items[0].Commit.Path)
	fs.Debugf(b.f, "Committing %s", desc)

	// finalise the batch getting either a result or a job id to poll
	complete, err := b.finishBatch(ctx, items)
	if err != nil {
		return err
	}

	// Check we got the right number of entries
	entries := complete.Entries
	if len(entries) != len(results) {
		return fmt.Errorf("expecting %d items in batch but got %d", len(results), len(entries))
	}

	// Report results to clients
	var (
		errorTag   = ""
		errorCount = 0
	)
	for i := range results {
		item := entries[i]
		resp := batcherResponse{}
		if item.Tag == "success" {
			resp.entry = item.Success
		} else {
			errorCount++
			errorTag = item.Tag
			if item.Failure != nil {
				errorTag = item.Failure.Tag
				if item.Failure.LookupFailed != nil {
					errorTag += "/" + item.Failure.LookupFailed.Tag
				}
				if item.Failure.Path != nil {
					errorTag += "/" + item.Failure.Path.Tag
				}
				if item.Failure.PropertiesError != nil {
					errorTag += "/" + item.Failure.PropertiesError.Tag
				}
			}
			resp.err = fmt.Errorf("batch upload failed: %s", errorTag)
		}
		if !b.async {
			results[i] <- resp
		}
	}
	// Show signalled so no need to report error to clients from now on
	signalled = true

	// Report an error if any failed in the batch
	if errorTag != "" {
		return fmt.Errorf("batch had %d errors: last error: %s", errorCount, errorTag)
	}

	fs.Debugf(b.f, "Committed %s", desc)
	return nil
}

// commitLoop runs the commit engine in the background
func (b *batcher) commitLoop(ctx context.Context) {
	var (
		items     []*files.UploadSessionFinishArg // current batch of uncommitted files
		results   []chan<- batcherResponse        // current batch of clients awaiting results
		idleTimer = time.NewTimer(b.timeout)
		commit    = func() {
			err := b.commitBatch(ctx, items, results)
			if err != nil {
				fs.Errorf(b.f, "%s batch commit: failed to commit batch length %d: %v", b.mode, len(items), err)
			}
			items, results = nil, nil
		}
	)
	defer b.wg.Done()
	defer idleTimer.Stop()
	idleTimer.Stop()

outer:
	for {
		select {
		case req := <-b.in:
			if req.isQuit() {
				break outer
			}
			items = append(items, req.commitInfo)
			results = append(results, req.result)
			idleTimer.Stop()
			if len(items) >= b.size {
				commit()
			} else {
				idleTimer.Reset(b.timeout)
			}
		case <-idleTimer.C:
			if len(items) > 0 {
				fs.Debugf(b.f, "Batch idle for %v so committing", b.timeout)
				commit()
			}
		}

	}
	// commit any remaining items
	if len(items) > 0 {
		commit()
	}
}

// Shutdown finishes any pending batches then shuts everything down
//
// Can be called from atexit handler
func (b *batcher) Shutdown() {
	b.shutOnce.Do(func() {
		atexit.Unregister(b.atexit)
		fs.Infof(b.f, "Commiting uploads - please wait...")
		// show that batcher is shutting down
		close(b.closed)
		// quit the commitLoop by sending a quitRequest message
		//
		// Note that we don't close b.in because that will
		// cause write to closed channel in Commit when we are
		// exiting due to a signal.
		b.in <- quitRequest
		b.wg.Wait()
	})
}

// Commit commits the file using a batch call, first adding it to the
// batch and then waiting for the batch to complete in a synchronous
// way if async is not set.
func (b *batcher) Commit(ctx context.Context, commitInfo *files.UploadSessionFinishArg) (entry *files.FileMetadata, err error) {
	select {
	case <-b.closed:
		return nil, fserrors.FatalError(errors.New("batcher is shutting down"))
	default:
	}
	fs.Debugf(b.f, "Adding %q to batch", commitInfo.Commit.Path)
	resp := make(chan batcherResponse, 1)
	b.in <- batcherRequest{
		commitInfo: commitInfo,
		result:     resp,
	}
	// If running async then don't wait for the result
	if b.async {
		return nil, nil
	}
	result := <-resp
	return result.entry, result.err
}
