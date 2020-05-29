// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package errs2

import (
	"time"

	"github.com/zeebo/errs"
)

// Collect returns first error from channel and all errors that happen within duration.
func Collect(errch chan error, duration time.Duration) error {
	errch = discardNil(errch)
	errlist := []error{<-errch}
	timeout := time.After(duration)
	for {
		select {
		case err := <-errch:
			errlist = append(errlist, err)
		case <-timeout:
			return errs.Combine(errlist...)
		}
	}
}

// discard nil errors that are returned from services.
func discardNil(ch chan error) chan error {
	r := make(chan error)
	go func() {
		for err := range ch {
			if err == nil {
				continue
			}
			r <- err
		}
		close(r)
	}()
	return r
}
