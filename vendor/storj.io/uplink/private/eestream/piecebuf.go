// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"io"
	"sync"

	"go.uber.org/zap"
)

// PieceBuffer is a synchronized buffer for storing erasure shares for a piece.
type PieceBuffer struct {
	log          *zap.Logger
	buf          []byte
	shareSize    int
	cond         *sync.Cond
	newDataCond  *sync.Cond
	rpos, wpos   int
	full         bool
	currentShare int64 // current erasure share number
	totalwr      int64 // total bytes ever written to the buffer
	lastwr       int64 // total bytes ever written when last notified newDataCond
	err          error
}

// NewPieceBuffer creates and initializes a new PieceBuffer using buf as its
// internal content. If new data is written to the buffer, newDataCond will be
// notified.
func NewPieceBuffer(log *zap.Logger, buf []byte, shareSize int, newDataCond *sync.Cond) *PieceBuffer {
	return &PieceBuffer{
		log:         log,
		buf:         buf,
		shareSize:   shareSize,
		cond:        sync.NewCond(&sync.Mutex{}),
		newDataCond: newDataCond,
	}
}

// Read reads the next len(p) bytes from the buffer or until the buffer is
// drained. The return value n is the number of bytes read. If the buffer has
// no data to return and no error is set, the call will block until new data is
// written to the buffer. Otherwise the error will be returned.
func (b *PieceBuffer) Read(p []byte) (n int, err error) {
	defer b.cond.Broadcast()
	b.cond.L.Lock()
	defer b.cond.L.Unlock()

	for b.empty() {
		if b.err != nil {
			return 0, b.err
		}
		b.cond.Wait()
	}

	if b.rpos >= b.wpos {
		nn := copy(p, b.buf[b.rpos:])
		n += nn
		b.rpos = (b.rpos + nn) % len(b.buf)
		p = p[nn:]
	}

	if b.rpos < b.wpos {
		nn := copy(p, b.buf[b.rpos:b.wpos])
		n += nn
		b.rpos += nn
	}

	if n > 0 {
		b.full = false
	}

	return n, nil
}

// Skip advances the read pointer with n bytes. It the buffered number of bytes
// are less than n, the method will block until enough data is written to the
// buffer.
func (b *PieceBuffer) Skip(n int) error {
	defer b.cond.Broadcast()
	b.cond.L.Lock()
	defer b.cond.L.Unlock()

	for n > 0 {
		for b.empty() {
			if b.err != nil {
				return b.err
			}
			b.cond.Wait()
		}

		if b.rpos >= b.wpos {
			if len(b.buf)-b.rpos > n {
				b.rpos = (b.rpos + n) % len(b.buf)
				n = 0
			} else {
				n -= len(b.buf) - b.rpos
				b.rpos = 0
			}
		} else {
			if b.wpos-b.rpos > n {
				b.rpos += n
				n = 0
			} else {
				n -= b.wpos - b.rpos
				b.rpos = b.wpos
			}
		}

		b.full = false
	}

	return nil
}

// Write writes the contents of p into the buffer. If the buffer is full it
// will block until some data is read from it, or an error is set. The return
// value n is the number of bytes written. If an error was set, it be returned.
func (b *PieceBuffer) Write(p []byte) (n int, err error) {
	for n < len(p) {
		nn, err := b.write(p[n:])
		n += nn
		if err != nil {
			return n, err
		}
		// Notify for new data only if a new complete erasure share is available
		b.totalwr += int64(nn)
		if b.totalwr/int64(b.shareSize)-b.lastwr/int64(b.shareSize) > 0 {
			b.lastwr = b.totalwr
			b.notifyNewData()
		}
	}
	return n, nil
}

// write is a helper method that takes care for the locking on each copy
// iteration.
func (b *PieceBuffer) write(p []byte) (n int, err error) {
	defer b.cond.Broadcast()
	b.cond.L.Lock()
	defer b.cond.L.Unlock()

	for b.full {
		if b.err != nil {
			return n, b.err
		}
		b.cond.Wait()
	}

	var wr int
	if b.wpos < b.rpos {
		wr = copy(b.buf[b.wpos:b.rpos], p)
	} else {
		wr = copy(b.buf[b.wpos:], p)
	}

	n += wr
	b.wpos = (b.wpos + wr) % len(b.buf)
	if b.wpos == b.rpos {
		b.full = true
	}

	return n, nil
}

// Close sets io.ErrClosedPipe to the buffer to prevent further writes and
// blocking on read.
func (b *PieceBuffer) Close() error {
	b.SetError(io.ErrClosedPipe)
	return nil
}

// SetError sets an error to be returned by Read and Write. Read will return
// the error after all data is read from the buffer.
func (b *PieceBuffer) SetError(err error) {
	b.setError(err)
	b.notifyNewData()
}

// setError is a helper method that locks the mutex before setting the error.
func (b *PieceBuffer) setError(err error) {
	defer b.cond.Broadcast()
	b.cond.L.Lock()
	defer b.cond.L.Unlock()

	b.err = err
}

// getError is a helper method that locks the mutex before getting the error.
func (b *PieceBuffer) getError() error {
	b.cond.L.Lock()
	defer b.cond.L.Unlock()

	return b.err
}

// notifyNewData notifies newDataCond that new data is written to the buffer.
func (b *PieceBuffer) notifyNewData() {
	b.newDataCond.L.Lock()
	defer b.newDataCond.L.Unlock()

	b.newDataCond.Broadcast()
}

// empty chacks if the buffer is empty.
func (b *PieceBuffer) empty() bool {
	return !b.full && b.rpos == b.wpos
}

// buffered returns the number of bytes that can be read from the buffer
// without blocking.
func (b *PieceBuffer) buffered() int {
	b.cond.L.Lock()
	defer b.cond.L.Unlock()

	switch {
	case b.rpos < b.wpos:
		return b.wpos - b.rpos
	case b.rpos > b.wpos:
		return len(b.buf) + b.wpos - b.rpos
	case b.full:
		return len(b.buf)
	default: // empty
		return 0
	}
}

// HasShare checks if the num-th share can be read from the buffer without
// blocking. If there are older erasure shares in the buffer, they will be
// discarded to leave room for the newer erasure shares to be written.
func (b *PieceBuffer) HasShare(num int64) bool {
	if num < b.currentShare {
		// we should never get here!
		b.log.Fatal("Requested erasure share was already read",
			zap.Int64("Requested Erasure Share", num),
			zap.Int64("Current Erasure Share", b.currentShare),
		)
	}

	if b.getError() != nil {
		return true
	}

	bufShares := int64(b.buffered() / b.shareSize)
	if num-b.currentShare > 0 {
		if bufShares > num-b.currentShare {
			// TODO: should this error be ignored?
			_ = b.discardUntil(num)
		} else {
			_ = b.discardUntil(b.currentShare + bufShares)
		}
		bufShares = int64(b.buffered() / b.shareSize)
	}

	return bufShares > num-b.currentShare
}

// ReadShare reads the num-th erasure share from the buffer into p. Any shares
// before num will be discarded from the buffer.
func (b *PieceBuffer) ReadShare(num int64, p []byte) error {
	if num < b.currentShare {
		// we should never get here!
		b.log.Fatal("Requested erasure share was already read",
			zap.Int64("Requested Erasure Share", num),
			zap.Int64("Current Erasure Share", b.currentShare),
		)
	}

	err := b.discardUntil(num)
	if err != nil {
		return err
	}

	_, err = io.ReadFull(b, p)
	if err != nil {
		return err
	}

	b.currentShare++

	return nil
}

// discardUntil discards all erasure shares from the buffer until the num-th
// erasure share exclusively.
func (b *PieceBuffer) discardUntil(num int64) error {
	if num <= b.currentShare {
		return nil
	}

	err := b.Skip(int(num-b.currentShare) * b.shareSize)
	if err != nil {
		return err
	}

	b.currentShare = num

	return nil
}
