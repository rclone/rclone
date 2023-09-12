// Package batcher implements a generic batcher.
//
// It uses two types:
//
//	Item - the thing to be batched
//	Result - the result from the batching
//
// And one function of type CommitBatchFn which is called to do the actual batching.
package batcher

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/atexit"
)

// Options for configuring the batcher
type Options struct {
	Mode                  string        // mode of the batcher "sync", "async" or "off"
	Size                  int           // size of batch
	Timeout               time.Duration // timeout before committing the batch
	MaxBatchSize          int           // max size the batch can be
	DefaultTimeoutSync    time.Duration // default time to kick off the batch if nothing added for this long (sync)
	DefaultTimeoutAsync   time.Duration // default time to kick off the batch if nothing added for this long (async)
	DefaultBatchSizeAsync int           // default batch size if async
}

// CommitBatchFn is called to commit a batch of Item and return Result to the callers.
//
// It should commit the batch of items then for each result i (of
// which there should be len(items)) it should set either results[i]
// or errors[i]
type CommitBatchFn[Item, Result any] func(ctx context.Context, items []Item, results []Result, errors []error) (err error)

// Batcher holds info about the current items waiting to be acted on.
type Batcher[Item, Result any] struct {
	opt      Options                     // options for configuring the batcher
	f        any                         // logging identity for fs.Debugf(f, ...)
	commit   CommitBatchFn[Item, Result] // User defined function to commit the batch
	async    bool                        // whether we are using async batching
	in       chan request[Item, Result]  // incoming items to batch
	closed   chan struct{}               // close to indicate batcher shut down
	atexit   atexit.FnHandle             // atexit handle
	shutOnce sync.Once                   // make sure we shutdown once only
	wg       sync.WaitGroup              // wait for shutdown
}

// request holds an incoming request with a place for a reply
type request[Item, Result any] struct {
	item   Item
	name   string
	result chan<- response[Result]
	quit   bool // if set then quit
}

// response holds a response to be delivered to clients waiting
// for a batch to complete.
type response[Result any] struct {
	err   error
	entry Result
}

// New creates a Batcher for Item and Result calling commit to do the actual committing.
func New[Item, Result any](ctx context.Context, f any, commit CommitBatchFn[Item, Result], opt Options) (*Batcher[Item, Result], error) {
	// fs.Debugf(f, "Creating batcher with mode %q, size %d, timeout %v", mode, size, timeout)
	if opt.Size > opt.MaxBatchSize || opt.Size < 0 {
		return nil, fmt.Errorf("batcher: batch size must be < %d and >= 0 - it is currently %d", opt.MaxBatchSize, opt.Size)
	}

	async := false

	switch opt.Mode {
	case "sync":
		if opt.Size <= 0 {
			ci := fs.GetConfig(ctx)
			opt.Size = ci.Transfers
		}
		if opt.Timeout <= 0 {
			opt.Timeout = opt.DefaultTimeoutSync
		}
	case "async":
		if opt.Size <= 0 {
			opt.Size = opt.DefaultBatchSizeAsync
		}
		if opt.Timeout <= 0 {
			opt.Timeout = opt.DefaultTimeoutAsync
		}
		async = true
	case "off":
		opt.Size = 0
	default:
		return nil, fmt.Errorf("batcher: batch mode must be sync|async|off not %q", opt.Mode)
	}

	b := &Batcher[Item, Result]{
		opt:    opt,
		f:      f,
		commit: commit,
		async:  async,
		in:     make(chan request[Item, Result], opt.Size),
		closed: make(chan struct{}),
	}
	if b.Batching() {
		b.atexit = atexit.Register(b.Shutdown)
		b.wg.Add(1)
		go b.commitLoop(context.Background())
	}
	return b, nil

}

// Batching returns true if batching is active
func (b *Batcher[Item, Result]) Batching() bool {
	return b.opt.Size > 0
}

// commit a batch calling the user defined commit function then distributing the results.
func (b *Batcher[Item, Result]) commitBatch(ctx context.Context, requests []request[Item, Result]) (err error) {
	// If commit fails then signal clients if sync
	var signalled = b.async
	defer func() {
		if err != nil && !signalled {
			// Signal to clients that there was an error
			for _, req := range requests {
				req.result <- response[Result]{err: err}
			}
		}
	}()
	desc := fmt.Sprintf("%s batch length %d starting with: %s", b.opt.Mode, len(requests), requests[0].name)
	fs.Debugf(b.f, "Committing %s", desc)

	var (
		items   = make([]Item, len(requests))
		results = make([]Result, len(requests))
		errors  = make([]error, len(requests))
	)

	for i := range requests {
		items[i] = requests[i].item
	}

	// Commit the batch
	err = b.commit(ctx, items, results, errors)
	if err != nil {
		return err
	}

	// Report results to clients
	var (
		lastError  error
		errorCount = 0
	)
	for i, req := range requests {
		result := results[i]
		err := errors[i]
		resp := response[Result]{}
		if err == nil {
			resp.entry = result
		} else {
			errorCount++
			lastError = err
			resp.err = fmt.Errorf("batch upload failed: %w", err)
		}
		if !b.async {
			req.result <- resp
		}
	}

	// show signalled so no need to report error to clients from now on
	signalled = true

	// Report an error if any failed in the batch
	if lastError != nil {
		return fmt.Errorf("batch had %d errors: last error: %w", errorCount, lastError)
	}

	fs.Debugf(b.f, "Committed %s", desc)
	return nil
}

// commitLoop runs the commit engine in the background
func (b *Batcher[Item, Result]) commitLoop(ctx context.Context) {
	var (
		requests  []request[Item, Result] // current batch of uncommitted Items
		idleTimer = time.NewTimer(b.opt.Timeout)
		commit    = func() {
			err := b.commitBatch(ctx, requests)
			if err != nil {
				fs.Errorf(b.f, "%s batch commit: failed to commit batch length %d: %v", b.opt.Mode, len(requests), err)
			}
			requests = nil
		}
	)
	defer b.wg.Done()
	defer idleTimer.Stop()
	idleTimer.Stop()

outer:
	for {
		select {
		case req := <-b.in:
			if req.quit {
				break outer
			}
			requests = append(requests, req)
			idleTimer.Stop()
			if len(requests) >= b.opt.Size {
				commit()
			} else {
				idleTimer.Reset(b.opt.Timeout)
			}
		case <-idleTimer.C:
			if len(requests) > 0 {
				fs.Debugf(b.f, "Batch idle for %v so committing", b.opt.Timeout)
				commit()
			}
		}

	}
	// commit any remaining items
	if len(requests) > 0 {
		commit()
	}
}

// Shutdown finishes any pending batches then shuts everything down.
//
// This is registered as an atexit handler by New.
func (b *Batcher[Item, Result]) Shutdown() {
	if !b.Batching() {
		return
	}
	b.shutOnce.Do(func() {
		atexit.Unregister(b.atexit)
		fs.Infof(b.f, "Committing uploads - please wait...")
		// show that batcher is shutting down
		close(b.closed)
		// quit the commitLoop by sending a quitRequest message
		//
		// Note that we don't close b.in because that will
		// cause write to closed channel in Commit when we are
		// exiting due to a signal.
		b.in <- request[Item, Result]{quit: true}
		b.wg.Wait()
	})
}

// Commit commits the Item getting a Result or error using a batch
// call, first adding it to the batch and then waiting for the batch
// to complete in a synchronous way if async is not set.
//
// If async is set then this will return no error and a nil/empty
// Result.
//
// This should not be called if batching is off - check first with
// IsBatching.
func (b *Batcher[Item, Result]) Commit(ctx context.Context, name string, item Item) (entry Result, err error) {
	select {
	case <-b.closed:
		return entry, fserrors.FatalError(errors.New("batcher is shutting down"))
	default:
	}
	fs.Debugf(b.f, "Adding %q to batch", name)
	resp := make(chan response[Result], 1)
	b.in <- request[Item, Result]{
		item:   item,
		name:   name,
		result: resp,
	}
	// If running async then don't wait for the result
	if b.async {
		return entry, nil
	}
	result := <-resp
	return result.entry, result.err
}
