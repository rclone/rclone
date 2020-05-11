// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcstream

import "sync"

type chMutex struct {
	ch   chan struct{}
	once sync.Once
}

func (m *chMutex) init() { m.ch = make(chan struct{}, 1) }

func (m *chMutex) Chan() chan struct{} {
	m.once.Do(m.init)
	return m.ch
}

func (m *chMutex) Lock() {
	m.once.Do(m.init)
	m.ch <- struct{}{}
}

func (m *chMutex) TryLock() bool {
	m.once.Do(m.init)
	select {
	case m.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

func (m *chMutex) Unlock() {
	m.once.Do(m.init)
	<-m.ch
}
