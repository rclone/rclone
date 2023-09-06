// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"sync"

	"storj.io/common/errs2"
)

// Concurrently runs fns concurrently and returns the non-nil errors.
func Concurrently(fns ...func() error) []error {
	var g errs2.Group
	for _, fn := range fns {
		g.Go(fn)
	}
	return g.Wait()
}

// Go runs fns concurrently and returns a func to wait for them to complete.
//
// See also Concurrently and errs2.Group.
func Go(fns ...func()) (wait func()) {
	var wg sync.WaitGroup
	wg.Add(len(fns))
	for _, fn := range fns {
		fn := fn
		go func() {
			defer wg.Done()
			fn()
		}()
	}
	return wg.Wait
}
