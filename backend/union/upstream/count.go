package upstream

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/walk"
)

// minRetryInterval is the minimum time to wait before listing again
// after a listing failed
const minRetryInterval = time.Minute

// countFn counts the objects and bytes in an upstream
type countFn func(ctx context.Context) (objects, size int64, err error)

// listCounter counts the objects and bytes in an upstream by listing
// it, for use when the upstream's About doesn't return them.
//
// Listing is expensive so the result is cached and, once it expires,
// refreshed in the background while the old result is still
// returned. Changes made through the union are applied to the cached
// result with add so it stays accurate between listings.
type listCounter struct {
	name     string        // name of the upstream for logging
	list     countFn       // function to do the listing
	interval time.Duration // how long a listing is valid for, <0 for ever

	mu             sync.Mutex
	valid          bool          // set if objects and size have been counted
	objects        int64         // number of objects
	size           int64         // total size of the objects
	err            error         // error from the last listing if it failed
	expiry         time.Time     // when to list again - zero for never
	running        chan struct{} // closed when the listing in progress finishes, nil if none
	pendingObjects int64         // objects added while the listing is running
	pendingSize    int64         // bytes added while the listing is running
}

// newListCounter makes a new listCounter which uses list to do the
// counting and keeps the result for interval or for ever if interval
// is negative.
func newListCounter(name string, list countFn, interval time.Duration) *listCounter {
	return &listCounter{
		name:     name,
		list:     list,
		interval: interval,
	}
}

// expired returns true if the upstream should be listed again
//
// Call with mu held
func (c *listCounter) expired() bool {
	return !c.expiry.IsZero() && !time.Now().Before(c.expiry)
}

// startLocked starts a listing if one isn't running already and
// returns a channel which is closed when it finishes
//
// Call with mu held
func (c *listCounter) startLocked() <-chan struct{} {
	if c.running == nil {
		c.running = make(chan struct{})
		go c.run(c.running)
	}
	return c.running
}

// run does the listing and stores the result
func (c *listCounter) run(done chan struct{}) {
	start := time.Now()
	// Run in background, should not be cancelled by user
	objects, size, err := c.list(context.Background())
	if errors.Is(err, fs.ErrorDirNotFound) {
		objects, size, err = 0, 0, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	defer close(done)
	c.running = nil
	pendingObjects, pendingSize := c.pendingObjects, c.pendingSize
	c.pendingObjects, c.pendingSize = 0, 0
	if err != nil {
		fs.Errorf(c.name, "Failed to count objects by listing: %v", err)
		c.err = err
		c.expiry = time.Now().Add(max(c.interval, minRetryInterval))
		return
	}
	fs.Debugf(c.name, "Counted %d objects (%s) by listing in %v", objects, fs.SizeSuffix(size).ByteUnit(), time.Since(start))
	// The listing may or may not have seen the changes made while it
	// was running so add them on, erring on the side of overcounting.
	c.objects = max(objects+pendingObjects, 0)
	c.size = max(size+pendingSize, 0)
	c.valid = true
	c.err = nil
	if c.interval < 0 {
		c.expiry = time.Time{}
	} else {
		c.expiry = time.Now().Add(c.interval)
	}
}

// get returns the number of objects and bytes, listing the upstream
// if necessary.
//
// This only blocks if the upstream hasn't been counted yet. If the
// result has expired it is returned and refreshed in the background.
func (c *listCounter) get() (objects, size int64, err error) {
	c.mu.Lock()
	if c.valid {
		if c.expired() {
			c.startLocked()
		}
		defer c.mu.Unlock()
		return c.objects, c.size, nil
	}
	if c.err != nil && !c.expired() {
		defer c.mu.Unlock()
		return 0, 0, c.err
	}
	done := c.startLocked()
	c.mu.Unlock()

	<-done

	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.valid {
		return 0, 0, c.err
	}
	return c.objects, c.size, nil
}

// add adjusts the count by the number of objects and bytes given,
// which may be negative
func (c *listCounter) add(objects, size int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running != nil {
		c.pendingObjects += objects
		c.pendingSize += size
	}
	if c.valid {
		c.objects = max(c.objects+objects, 0)
		c.size = max(c.size+size, 0)
	}
}

// markStale makes the next get list the upstream again in the
// background, for use when changes have been made which can't be
// accounted with add.
func (c *listCounter) markStale() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.valid {
		c.expiry = time.Now()
	}
}

// listUsage counts the objects and bytes in f by listing all of it
//
// This uses the same algorithm as `rclone size` but ignores any
// filters or depth limits set for the command being run.
func listUsage(ctx context.Context, f fs.Fs) (objects, size int64, err error) {
	fi, err := filter.NewFilter(&filter.Options{
		MinAge:  fs.DurationOff,
		MaxAge:  fs.DurationOff,
		MinSize: -1,
		MaxSize: -1,
	})
	if err != nil {
		return 0, 0, err
	}
	ctx = filter.ReplaceConfig(ctx, fi)
	// Don't count listing errors in the stats of the command being run
	ctx = accounting.WithStatsGroup(ctx, "union-usage")
	var nObjects, nSize atomic.Int64
	err = walk.ListR(ctx, f, "", true, -1, walk.ListObjects, func(entries fs.DirEntries) error {
		entries.ForObject(func(o fs.Object) {
			nObjects.Add(1)
			if objectSize := o.Size(); objectSize > 0 {
				nSize.Add(objectSize)
			}
		})
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return nObjects.Load(), nSize.Load(), nil
}
