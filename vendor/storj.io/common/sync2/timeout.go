// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package sync2

import (
	"time"
)

// WithTimeout calls `do` and when the timeout is reached and `do`
// has not finished, it'll call `onTimeout` concurrently.
//
// When WithTimeout returns it's guaranteed to not call onTimeout.
func WithTimeout(timeout time.Duration, do, onTimeout func()) {
	workDone := make(chan struct{})
	timeoutExited := make(chan struct{})

	go func() {
		defer close(timeoutExited)

		t := time.NewTimer(timeout)
		defer t.Stop()

		select {
		case <-workDone:
		case <-t.C:
			onTimeout()
		}
	}()

	do()

	close(workDone)
	<-timeoutExited
}
