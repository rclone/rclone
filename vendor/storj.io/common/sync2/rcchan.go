// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package sync2

import (
	"context"
)

// ReceiverClosableChan is a channel with altered semantics
// from the go runtime channels. It is designed to work
// well in a many-producer, single-receiver environment,
// where the receiver consumes until it is shut down and
// must signal to many senders to stop sending.
type ReceiverClosableChan[T any] struct {
	outstanding   int
	slots         chan struct{}
	vals          chan T
	receiverCalls int
}

// MakeReceiverClosableChan makes a new buffered channel of
// the given buffer size. A zero buffer size is currently
// undefined behavior.
func MakeReceiverClosableChan[T any](bufferSize int) *ReceiverClosableChan[T] {
	if bufferSize <= 0 {
		panic("invalid buffer size")
	}
	c := &ReceiverClosableChan[T]{
		slots:       make(chan struct{}, bufferSize),
		vals:        make(chan T, bufferSize),
		outstanding: bufferSize,
	}
	for i := 0; i < bufferSize; i++ {
		c.slots <- struct{}{}
	}
	return c
}

// BlockingSend will send the value into the channel's buffer. If the
// buffer is full, BlockingSend will block. BlockingSend will fail and return
// false if StopReceiving is called.
func (c *ReceiverClosableChan[T]) BlockingSend(v T) (ok bool) {
	if _, ok := <-c.slots; !ok {
		return false
	}
	c.vals <- v
	return true
}

// Receive returns the next request, until and unless ctx is canceled.
// Receive does not stop if there are no more requests and StopReceiving
// has been called, as it is expected that the caller of Receive is
// who called StopReceiving.
// The error is not nil if and only if the context was canceled.
func (c *ReceiverClosableChan[T]) Receive(ctx context.Context) (v T, err error) {
	// trigger the race detector if someone tries to call StopReceiving
	// concurrently.
	c.receiverCalls++
	select {
	case <-ctx.Done():
		return v, ctx.Err()
	case v := <-c.vals:
		c.slots <- struct{}{}
		return v, nil
	}
}

// StopReceiving will cause all currently blocked and future
// sends to return false. StopReceiving will return what
// remains in the queue.
func (c *ReceiverClosableChan[T]) StopReceiving() (drained []T) {
	// trigger the race detector if someone tries to call Receive concurrently.
	c.receiverCalls++
	close(c.slots)
	for range c.slots {
		c.outstanding--
	}
	for c.outstanding > 0 {
		drained = append(drained, <-c.vals)
		c.outstanding--
	}
	return drained
}
