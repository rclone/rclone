// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcsignal

import (
	"sync"
	"sync/atomic"
)

// Signal contains an error value that can be set one and exports
// a number of ways to inspect it.
type Signal struct {
	set uint32
	on  sync.Once
	mu  sync.Mutex
	sig chan struct{}
	err error
}

func (s *Signal) init() { s.sig = make(chan struct{}) }

// Signal returns a channel that will be closed when the signal is set.
func (s *Signal) Signal() chan struct{} {
	s.on.Do(s.init)
	return s.sig
}

// Set stores the error in the signal. It only keeps track of the first
// error set, and returns true if it was the first error set.
func (s *Signal) Set(err error) (ok bool) {
	if atomic.LoadUint32(&s.set) != 0 {
		return false
	}
	return s.setSlow(err)
}

// setSlow is the slow path for Set, so that the fast path is inlined into
// callers.
func (s *Signal) setSlow(err error) (ok bool) {
	s.mu.Lock()
	if s.set == 0 {
		s.err = err
		atomic.StoreUint32(&s.set, 1)
		s.on.Do(s.init)
		close(s.sig)
		ok = true
	}
	s.mu.Unlock()
	return ok
}

// Get returns the error set with the signal and a boolean indicating if
// the result is valid.
func (s *Signal) Get() (error, bool) { //nolint
	if atomic.LoadUint32(&s.set) != 0 {
		return s.err, true
	}
	return nil, false
}

// IsSet returns true if the Signal is set.
func (s *Signal) IsSet() bool {
	return atomic.LoadUint32(&s.set) != 0
}

// Err returns the error stored in the signal. Since one can store a nil error
// care must be taken. A non-nil error returned from this method means that
// the Signal has been set, but the inverse is not true.
func (s *Signal) Err() error {
	if atomic.LoadUint32(&s.set) != 0 {
		return s.err
	}
	return nil
}
