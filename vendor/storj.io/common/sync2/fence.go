// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"context"
	"sync"
)

// Fence allows to wait for something to happen.
type Fence struct {
	noCopy noCopy // nolint: structcheck

	setup   sync.Once
	release sync.Once
	done    chan struct{}
}

// init sets up the initial lock into wait
func (fence *Fence) init() {
	fence.setup.Do(func() {
		fence.done = make(chan struct{})
	})
}

// Release releases everyone from Wait
func (fence *Fence) Release() {
	fence.init()
	fence.release.Do(func() { close(fence.done) })
}

// Wait waits for wait to be unlocked.
// Returns true when it was successfully released.
func (fence *Fence) Wait(ctx context.Context) bool {
	fence.init()

	select {
	case <-fence.done:
		return true
	default:
		select {
		case <-ctx.Done():
			return false
		case <-fence.done:
			return true
		}
	}
}

// Released returns whether the fence has been released.
func (fence *Fence) Released() bool {
	fence.init()

	select {
	case <-fence.done:
		return true
	default:
		return false
	}
}

// Done returns channel that will be closed when the fence is released.
func (fence *Fence) Done() chan struct{} {
	fence.init()
	return fence.done
}
