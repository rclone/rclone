// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcsignal

import (
	"sync"
	"sync/atomic"
)

var closed = make(chan struct{})

func init() { close(closed) }

// Chan is a lazily allocated chan struct{} that avoids allocating if
// it is closed before being used for anything.
type Chan struct {
	done uint32
	mu   sync.Mutex
	ch   chan struct{}
}

func (c *Chan) do(f func()) bool {
	return atomic.LoadUint32(&c.done) == 0 && c.doSlow(f)
}

func (c *Chan) doSlow(f func()) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.done == 0 {
		defer atomic.StoreUint32(&c.done, 1)
		f()
		return true
	}
	return false
}

// setFresh sets the channel to a freshly allocated one.
func (c *Chan) setFresh() {
	c.ch = make(chan struct{})
}

// setClosed sets the channel to an already closed one.
func (c *Chan) setClosed() {
	c.ch = closed
}

// Close tries to set the channel to an already closed one if
// a fresh one has not already been set, and closes the fresh
// one otherwise.
func (c *Chan) Close() {
	if !c.do(c.setClosed) {
		close(c.ch)
	}
}

// Make sets the channel to a freshly allocated channel with the
// provided capacity. It is a no-op if called after any other
// methods.
func (c *Chan) Make(cap uint) {
	c.do(func() { c.ch = make(chan struct{}, cap) })
}

// Get returns the channel, allocating if necessary.
func (c *Chan) Get() chan struct{} {
	c.do(c.setFresh)
	return c.ch
}

// Send sends a value on the channel, allocating if necessary.
func (c *Chan) Send() {
	c.do(c.setFresh)
	c.ch <- struct{}{}
}

// Recv receives a value on the channel, allocating if necessary.
func (c *Chan) Recv() {
	c.do(c.setFresh)
	<-c.ch
}

// Full returns true if the channel is currently full. The information
// is immediately invalid in the sense that a send could always block.
func (c *Chan) Full() bool {
	c.do(c.setFresh)

	select {
	case c.ch <- struct{}{}:
		<-c.ch
		return false
	default:
		return true
	}
}
