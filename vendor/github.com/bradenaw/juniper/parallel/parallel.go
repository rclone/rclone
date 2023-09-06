package parallel

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"

	"github.com/bradenaw/juniper/container/xheap"
	"github.com/bradenaw/juniper/iterator"
	"github.com/bradenaw/juniper/stream"
)

// Do calls f from parallelism goroutines n times, providing each invocation a unique i in [0, n).
//
// If parallelism <= 0, uses GOMAXPROCS instead.
func Do(
	parallelism int,
	n int,
	f func(i int),
) {
	if parallelism <= 0 {
		parallelism = runtime.GOMAXPROCS(-1)
	}

	if parallelism > n {
		parallelism = n
	}

	if parallelism == 1 {
		for i := 0; i < n; i++ {
			f(i)
		}
		return
	}

	x := int32(-1)
	var wg sync.WaitGroup
	wg.Add(parallelism)
	for j := 0; j < parallelism; j++ {
		go func() {
			defer wg.Done()
			for {
				i := int(atomic.AddInt32(&x, 1))
				if i >= n {
					return
				}
				f(i)
			}
		}()
	}
	wg.Wait()
	return
}

// DoContext calls f from parallelism goroutines n times, providing each invocation a unique i in
// [0, n).
//
// If any call to f returns an error the context passed to invocations of f is cancelled, no further
// calls to f are made, and Do returns the first error encountered.
//
// If parallelism <= 0, uses GOMAXPROCS instead.
func DoContext(
	ctx context.Context,
	parallelism int,
	n int,
	f func(ctx context.Context, i int) error,
) error {
	if parallelism <= 0 {
		parallelism = runtime.GOMAXPROCS(-1)
	}

	if parallelism > n {
		parallelism = n
	}

	if parallelism == 1 {
		for i := 0; i < n; i++ {
			err := f(ctx, i)
			if err != nil {
				return err
			}
		}
		return nil
	}

	x := int32(-1)
	eg, ctx := errgroup.WithContext(ctx)
	for j := 0; j < parallelism; j++ {
		eg.Go(func() error {
			for {
				i := int(atomic.AddInt32(&x, 1))
				if i >= n {
					return nil
				}

				if ctx.Err() != nil {
					return ctx.Err()
				}

				err := f(ctx, i)
				if err != nil {
					return err
				}
			}
		})
	}
	return eg.Wait()
}

// Map uses parallelism goroutines to call f once for each element of in. out[i] is the
// result of f for in[i].
//
// If parallelism <= 0, uses GOMAXPROCS instead.
func Map[T any, U any](
	parallelism int,
	in []T,
	f func(in T) U,
) []U {
	out := make([]U, len(in))
	Do(parallelism, len(in), func(i int) {
		out[i] = f(in[i])
	})
	return out
}

// MapContext uses parallelism goroutines to call f once for each element of in. out[i] is the
// result of f for in[i].
//
// If any call to f returns an error the context passed to invocations of f is cancelled, no further
// calls to f are made, and Map returns the first error encountered.
//
// If parallelism <= 0, uses GOMAXPROCS instead.
func MapContext[T any, U any](
	ctx context.Context,
	parallelism int,
	in []T,
	f func(ctx context.Context, in T) (U, error),
) ([]U, error) {
	out := make([]U, len(in))
	err := DoContext(ctx, parallelism, len(in), func(ctx context.Context, i int) error {
		var err error
		out[i], err = f(ctx, in[i])
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// MapIterator uses parallelism goroutines to call f once for each element yielded by iter. The
// returned iterator returns these results in the same order that iter yielded them in.
//
// This iterator, in contrast with most, must be consumed completely or it will leak the goroutines.
//
// If parallelism <= 0, uses GOMAXPROCS instead.
//
// bufferSize is the size of the work buffer. A larger buffer uses more memory but gives better
// throughput in the face of larger variance in the processing time for f.
func MapIterator[T any, U any](
	iter iterator.Iterator[T],
	parallelism int,
	bufferSize int,
	f func(T) U,
) iterator.Iterator[U] {
	if parallelism <= 0 {
		parallelism = runtime.GOMAXPROCS(-1)
	}
	if bufferSize < parallelism {
		bufferSize = parallelism
	}

	in := make(chan valueAndIndex[T])
	mIter := &mapIterator[U]{
		ch: make(chan valueAndIndex[U]),
		h: xheap.New(func(a, b valueAndIndex[U]) bool {
			return a.idx < b.idx
		}, nil),
		i:          0,
		bufferSize: bufferSize,
		inFlight:   0,
	}
	mIter.cond = sync.NewCond(&mIter.m)

	go func() {
		i := 0
		for {
			item, ok := iter.Next()
			if !ok {
				break
			}

			mIter.m.Lock()
			for mIter.inFlight >= bufferSize {
				mIter.cond.Wait()
			}
			mIter.inFlight++
			mIter.m.Unlock()

			in <- valueAndIndex[T]{
				value: item,
				idx:   i,
			}
			i++
		}
		close(in)
	}()

	nDone := uint32(0)
	for i := 0; i < parallelism; i++ {
		go func() {
			for item := range in {
				u := f(item.value)
				mIter.ch <- valueAndIndex[U]{value: u, idx: item.idx}
			}
			if atomic.AddUint32(&nDone, 1) == uint32(parallelism) {
				close(mIter.ch)
			}
		}()
	}
	return mIter
}

type mapIterator[U any] struct {
	ch         chan valueAndIndex[U]
	m          sync.Mutex
	cond       *sync.Cond
	bufferSize int
	inFlight   int
	h          xheap.Heap[valueAndIndex[U]]
	i          int
}

func (iter *mapIterator[U]) Next() (U, bool) {
	for {
		if iter.h.Len() > 0 && iter.h.Peek().idx == iter.i {
			item := iter.h.Pop()
			iter.i++

			iter.m.Lock()
			iter.inFlight--
			if iter.inFlight == iter.bufferSize-1 {
				iter.cond.Signal()
			}
			iter.m.Unlock()
			return item.value, true
		}
		item, ok := <-iter.ch
		if !ok {
			var zero U
			return zero, false
		}
		iter.h.Push(item)
	}
}

type valueAndIndex[T any] struct {
	value T
	idx   int
}

// MapStream uses parallelism goroutines to call f once for each element yielded by s. The returned
// stream returns these results in the same order that s yielded them in.
//
// If any call to f returns an error the context passed to invocations of f is cancelled, no further
// calls to f are made, and the returned stream's Next returns the first error encountered.
//
// If parallelism <= 0, uses GOMAXPROCS instead.
//
// bufferSize is the size of the work buffer. A larger buffer uses more memory but gives better
// throughput in the face of larger variance in the processing time for f.
func MapStream[T any, U any](
	ctx context.Context,
	s stream.Stream[T],
	parallelism int,
	bufferSize int,
	f func(context.Context, T) (U, error),
) stream.Stream[U] {
	if parallelism <= 0 {
		parallelism = runtime.GOMAXPROCS(-1)
	}
	if bufferSize < parallelism {
		bufferSize = parallelism
	}

	in := make(chan valueAndIndex[T])
	ready := make(chan struct{}, bufferSize)
	for i := 0; i < bufferSize; i++ {
		ready <- struct{}{}
	}

	ctx, cancel := context.WithCancel(ctx)
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		defer s.Close()
		defer close(in)
		i := 0
		for {
			item, err := s.Next(ctx)
			if err == stream.End {
				break
			} else if err != nil {
				return err
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ready:
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case in <- valueAndIndex[T]{
				value: item,
				idx:   i,
			}:
			}
			i++
		}
		return nil
	})

	c := make(chan valueAndIndex[U], bufferSize)
	nDone := uint32(0)
	for i := 0; i < parallelism; i++ {
		eg.Go(func() error {
			defer func() {
				if atomic.AddUint32(&nDone, 1) == uint32(parallelism) {
					close(c)
				}
			}()
			for item := range in {
				u, err := f(ctx, item.value)
				if err != nil {
					return err
				}
				select {
				case c <- valueAndIndex[U]{value: u, idx: item.idx}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		})
	}

	return &mapStream[U]{
		cancel: cancel,
		eg:     eg,
		c:      c,
		ready:  ready,
		h: xheap.New(func(a, b valueAndIndex[U]) bool {
			return a.idx < b.idx
		}, nil),
		i: 0,
	}
}

type mapStream[U any] struct {
	cancel context.CancelFunc
	eg     *errgroup.Group
	c      <-chan valueAndIndex[U]
	ready  chan struct{}
	h      xheap.Heap[valueAndIndex[U]]
	i      int
}

func (s *mapStream[U]) Next(ctx context.Context) (U, error) {
	var zero U
	for {
		if s.h.Len() > 0 && s.h.Peek().idx == s.i {
			item := s.h.Pop()
			s.i++
			s.ready <- struct{}{}
			return item.value, nil
		}
		select {
		case item, ok := <-s.c:
			if !ok {
				err := s.eg.Wait()
				if err != nil {
					return zero, err
				}
				return zero, stream.End
			}
			s.h.Push(item)
		case <-ctx.Done():
			return zero, ctx.Err()
		}
	}
}

func (s *mapStream[U]) Close() {
	s.cancel()
	_ = s.eg.Wait()
}
