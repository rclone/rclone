// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package streamupload

import (
	"context"
	"sync"
)

type cancelGroup struct {
	mu     sync.Mutex
	wg     sync.WaitGroup
	cancel func()
	err    error
}

func newCancelGroup(ctx context.Context) (context.Context, *cancelGroup) {
	ctx, cancel := context.WithCancel(ctx)
	return ctx, &cancelGroup{cancel: cancel}
}

func (c *cancelGroup) Go(f func() error) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		if err := f(); err != nil {
			c.mu.Lock()
			defer c.mu.Unlock()

			if c.err == nil {
				c.err = err
				c.cancel()
			}
		}
	}()
}

func (c *cancelGroup) Close() {
	c.cancel()
	c.wg.Wait()
}

func (c *cancelGroup) Wait() error {
	c.wg.Wait()
	return c.err
}
