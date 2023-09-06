// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcmanager

import (
	"sync"
	"sync/atomic"

	"storj.io/drpc/drpcstream"
)

type streamBuffer struct {
	mu     sync.Mutex
	cond   sync.Cond
	stream atomic.Value // *drpcstream.Stream
	closed bool
}

func (sb *streamBuffer) init() {
	sb.cond.L = &sb.mu
}

func (sb *streamBuffer) Close() {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	sb.closed = true
	sb.cond.Broadcast()
}

func (sb *streamBuffer) Get() *drpcstream.Stream {
	stream, _ := sb.stream.Load().(*drpcstream.Stream)
	return stream
}

func (sb *streamBuffer) Set(stream *drpcstream.Stream) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.closed {
		return
	}

	sb.stream.Store(stream)
	sb.cond.Broadcast()
}

func (sb *streamBuffer) Wait(sid uint64) bool {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	for !sb.closed && sb.Get().ID() == sid {
		sb.cond.Wait()
	}

	return !sb.closed
}
