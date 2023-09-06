// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package splitter

import (
	"context"
	"io"
	"sync"

	"github.com/zeebo/errs"
)

// WriteFinisher is a Writer that can be signalled by the caller when it is
// done being written do. Subsequent calls to write should return an error
// after writing is "done".
type WriteFinisher interface {
	io.Writer
	DoneWriting(error)
}

type baseSplitter struct {
	split   int64
	minimum int64

	writeMu   sync.Mutex // ensures only a single Write call at once
	nextMu    sync.Mutex // ensures only a single Next call at once
	currentMu sync.Mutex // protects access to current

	emitted    bool               // set true when the first split is emitted
	term       chan struct{}      // closed when finish is called
	err        error              // captures the error passed to finish
	finishOnce sync.Once          // only want to finish once
	temp       []byte             // holds temporary data up to minimum
	written    int64              // how many bytes written into current
	next       chan WriteFinisher // channel for the next split to write into
	current    WriteFinisher      // current split being written to
}

func newBaseSplitter(split, minimum int64) *baseSplitter {
	return &baseSplitter{
		split:   split,
		minimum: minimum,

		term: make(chan struct{}),
		temp: make([]byte, 0, minimum),
		next: make(chan WriteFinisher),
	}
}

func (bs *baseSplitter) Finish(err error) {
	bs.finishOnce.Do(func() {
		bs.err = err
		close(bs.term)
		bs.currentMu.Lock()
		if bs.current != nil {
			bs.current.DoneWriting(err)
		}
		bs.currentMu.Unlock()
	})
}

func (bs *baseSplitter) Write(p []byte) (n int, err error) {
	// only ever allow one Write call at a time
	bs.writeMu.Lock()
	defer bs.writeMu.Unlock()

	select {
	case <-bs.term:
		if bs.err != nil {
			return 0, bs.err
		}
		return 0, errs.New("already finished")
	default:
	}

	for len(p) > 0 {
		// if we have no remaining bytes to write, close and move on
		rem := bs.split - bs.written
		if rem == 0 && bs.current != nil {
			bs.currentMu.Lock()
			bs.current.DoneWriting(nil)
			bs.current = nil
			bs.currentMu.Unlock()
			bs.written = 0
		}

		// if we have a current buffer, write up to the point of the next split
		if bs.current != nil {
			pp := p
			if rem < int64(len(pp)) {
				pp = p[:rem]
			}

			// drop the state mutex so that Finish calls can interrupt
			nn, err := bs.current.Write(pp)

			// update tracking of how many bytes have been written
			n += nn
			bs.written += int64(nn)
			p = p[nn:]

			if err != nil {
				bs.Finish(err)
				return n, err
			}

			continue
		}

		// if we can fully fit in temp, do so
		if len(bs.temp)+len(p) <= cap(bs.temp) {
			bs.temp = append(bs.temp, p...)

			n += len(p)
			p = p[len(p):]

			continue
		}

		// fill up temp as much as possible and wait for a new buffer
		nn := copy(bs.temp[len(bs.temp):cap(bs.temp)], p)
		bs.temp = bs.temp[:cap(bs.temp)]

		// update tracking of how many bytes have been written
		n += nn
		p = p[nn:]

		select {
		case wf := <-bs.next:
			bs.currentMu.Lock()
			bs.current = wf
			bs.currentMu.Unlock()

			n, err := wf.Write(bs.temp)

			bs.temp = bs.temp[:0]
			bs.written += int64(n)

			if err != nil {
				bs.Finish(err)
				return n, err
			}

		case <-bs.term:
			if bs.err != nil {
				return n, bs.err
			}
			return n, errs.New("write interrupted by finish")
		}
	}

	return n, nil
}

func (bs *baseSplitter) Next(ctx context.Context, wf WriteFinisher) (inline []byte, eof bool, err error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}

	bs.nextMu.Lock()
	defer bs.nextMu.Unlock()

	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()

	case bs.next <- wf:
		bs.emitted = true
		return nil, false, nil

	case <-bs.term:
		if bs.err != nil {
			return nil, false, bs.err
		}
		if len(bs.temp) > 0 || !bs.emitted {
			bs.emitted = true

			temp := bs.temp
			bs.temp = bs.temp[:0]

			return temp, false, nil
		}
		return nil, true, nil
	}
}
