// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcsignal

import (
	"sync"
	"sync/atomic"
)

type signalStatus = uint32

const (
	statusErrorSet       = 0b10
	statusChannelCreated = 0b01
)

// Signal contains an error value that can be set one and exports
// a number of ways to inspect it.
type Signal struct {
	status signalStatus
	mu     sync.Mutex
	ch     chan struct{}
	err    error
}

// Wait blocks until the signal has been Set.
func (s *Signal) Wait() {
	<-s.Signal()
}

// Signal returns a channel that will be closed when the signal is set.
func (s *Signal) Signal() chan struct{} {
	if atomic.LoadUint32(&s.status)&statusChannelCreated != 0 {
		return s.ch
	}
	return s.signalSlow()
}

// signalSlow is the slow path for Signal, so that the fast path is inlined into
// callers.
func (s *Signal) signalSlow() chan struct{} {
	s.mu.Lock()
	if set := s.status; set&statusChannelCreated == 0 {
		s.ch = make(chan struct{})
		atomic.StoreUint32(&s.status, set|statusChannelCreated)
	}
	s.mu.Unlock()
	return s.ch
}

// Set stores the error in the signal. It only keeps track of the first
// error set, and returns true if it was the first error set.
func (s *Signal) Set(err error) (ok bool) {
	if atomic.LoadUint32(&s.status)&statusErrorSet != 0 {
		return false
	}
	return s.setSlow(err)
}

// setSlow is the slow path for Set, so that the fast path is inlined into
// callers.
func (s *Signal) setSlow(err error) (ok bool) {
	s.mu.Lock()
	if status := s.status; status&statusErrorSet == 0 {
		ok = true

		s.err = err
		if status&statusChannelCreated == 0 {
			s.ch = closed
		}

		// we have to store the flags after we set the channel but before we
		// close it, otherwise there are races where a caller can hit the
		// atomic fast path and observe invalid values.
		atomic.StoreUint32(&s.status, statusErrorSet|statusChannelCreated)

		if status&statusChannelCreated != 0 {
			close(s.ch)
		}
	}
	s.mu.Unlock()
	return ok
}

// Get returns the error set with the signal and a boolean indicating if
// the result is valid.
func (s *Signal) Get() (error, bool) { //nolint
	if atomic.LoadUint32(&s.status)&statusErrorSet != 0 {
		return s.err, true
	}
	return nil, false
}

// IsSet returns true if the Signal is set.
func (s *Signal) IsSet() bool {
	return atomic.LoadUint32(&s.status)&statusErrorSet != 0
}

// Err returns the error stored in the signal. Since one can store a nil error
// care must be taken. A non-nil error returned from this method means that
// the Signal has been set, but the inverse is not true.
func (s *Signal) Err() error {
	if atomic.LoadUint32(&s.status)&statusErrorSet != 0 {
		return s.err
	}
	return nil
}
