// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcstream

import (
	"sync"
	"sync/atomic"
)

type inspectMutex struct {
	held uint32
	mu   sync.Mutex
}

func (m *inspectMutex) Lock() {
	m.mu.Lock()
	atomic.StoreUint32(&m.held, 1)
}

func (m *inspectMutex) Unlock() {
	atomic.StoreUint32(&m.held, 0)
	m.mu.Unlock()
}

// Unlocked returns true if the mutex is either currently unlocked
// or in the process of unlocking, meaning that no potentially
// blocking operations will be executed before the mutex is
// unlocked. In the presence of concurrent Lock and Unlock calls
// this function can only be advisory at best. Any information
// returned from it is necessarily stale and does not reflect
// the current state of the mutex.
func (m *inspectMutex) Unlocked() bool {
	return atomic.LoadUint32(&m.held) == 0
}
