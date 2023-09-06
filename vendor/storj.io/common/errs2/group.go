// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package errs2

import "sync"

// Group is a collection of goroutines working on subtasks that are part of
// the same overall task.
type Group struct {
	wg     sync.WaitGroup
	mu     sync.Mutex
	errors []error
}

// Go calls the given function in a new goroutine.
func (group *Group) Go(f func() error) {
	group.wg.Add(1)

	go func() {
		defer group.wg.Done()

		if err := f(); err != nil {
			group.mu.Lock()
			defer group.mu.Unlock()

			group.errors = append(group.errors, err)
		}
	}()
}

// Wait blocks until all function calls from the Go method have returned, then
// returns all errors (if any) from them.
func (group *Group) Wait() []error {
	group.wg.Wait()

	return group.errors
}
