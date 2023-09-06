// Package stream allows iterating over sequences of values where iteration may fail, for example
// when it involves I/O.
package stream

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bradenaw/juniper/iterator"
	"github.com/bradenaw/juniper/xmath"
)

var (
	// End is returned from Stream.Next when iteration ends successfully.
	End = errors.New("end of stream")
	// ErrClosedPipe is returned from PipeSender.Send() when the associated stream has already been
	// closed.
	ErrClosedPipe = errors.New("closed pipe")
	// ErrMoreThanOne is returned from One when a Stream yielded more than one item.
	ErrMoreThanOne = errors.New("stream had more than one item")
	// ErrEmpty is returned from One when a Stream yielded no items.
	ErrEmpty = errors.New("stream empty")
)

// Stream is used to iterate over a sequence of values. It is similar to Iterator, except intended
// for use when iteration may fail for some reason, usually because the sequence requires I/O to
// produce.
//
// Streams and the combinator functions are lazy, meaning they do no work until a call to Next().
//
// Streams do not need to be fully consumed, but streams must be closed. Functions in this package
// that are passed streams expect to be the sole user of that stream going forward, and so will
// handle closing on your behalf so long as all streams they return are closed appropriately.
type Stream[T any] interface {
	// Next advances the stream and returns the next item. If the stream is already over, Next
	// returns stream.End in the second return. Note that the final item of the stream has nil in
	// the second return, and it's the following call that returns stream.End.
	//
	// Once a Next call returns stream.End, it is expected that the Stream will return stream.End to
	// every Next call afterwards.
	Next(ctx context.Context) (T, error)
	// Close ends receiving from the stream. It is invalid to call Next after calling Close.
	Close()
}

////////////////////////////////////////////////////////////////////////////////////////////////////
// Converters + Constructors                                                                      //
// Functions that produce a Stream.                                                               //
////////////////////////////////////////////////////////////////////////////////////////////////////

// Chan returns a Stream that receives values from c.
func Chan[T any](c <-chan T) Stream[T] {
	return &chanStream[T]{c: c}
}

type chanStream[T any] struct {
	c <-chan T
}

func (s *chanStream[T]) Next(ctx context.Context) (T, error) {
	var zero T
	select {
	case item, ok := <-s.c:
		if !ok {
			return zero, End
		}
		return item, nil
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

func (s *chanStream[T]) Close() {}

// Empty returns a Stream that yields stream.End immediately.
func Empty[T any]() Stream[T] {
	return emptyStream[T]{}
}

type emptyStream[T any] struct{}

func (s emptyStream[T]) Next(ctx context.Context) (T, error) {
	var zero T
	return zero, End
}

func (s emptyStream[T]) Close() {}

// Error returns a Stream that immediately produces err from Next.
func Error[T any](err error) Stream[T] {
	return errorStream[T]{err}
}

type errorStream[T any] struct {
	err error
}

func (s errorStream[T]) Next(ctx context.Context) (T, error) {
	var zero T
	return zero, s.err
}

func (s errorStream[T]) Close() {}

// FromIterator returns a Stream that yields the values from iter. This stream ignores the context
// passed to Next during the call to iter.Next.
func FromIterator[T any](iter iterator.Iterator[T]) Stream[T] {
	return &iteratorStream[T]{iter: iter}
}

type iteratorStream[T any] struct {
	iter iterator.Iterator[T]
}

func (s *iteratorStream[T]) Next(ctx context.Context) (T, error) {
	var zero T
	if ctx.Err() != nil {
		return zero, ctx.Err()
	}
	item, ok := s.iter.Next()
	if !ok {
		return zero, End
	}
	return item, nil
}

func (s *iteratorStream[T]) Close() {}

// Pipe returns a linked sender and receiver pair. Values sent using sender.Send will be delivered
// to the given Stream. The Stream will terminate when the sender is closed.
//
// bufferSize is the number of elements in the buffer between the sender and the receiver. 0 has the
// same meaning as for the built-in make(chan).
func Pipe[T any](bufferSize int) (*PipeSender[T], Stream[T]) {
	c := make(chan T, bufferSize)
	senderDone := make(chan struct{})
	senderErr := new(error)
	streamDone := make(chan struct{})

	sender := &PipeSender[T]{
		c:          c,
		senderErr:  senderErr,
		senderDone: senderDone,
		streamDone: streamDone,
	}
	receiver := &pipeStream[T]{
		c:          c,
		senderErr:  senderErr,
		senderDone: senderDone,
		streamDone: streamDone,
	}

	return sender, receiver
}

// PipeSender is the send half of a pipe returned by Pipe.
type PipeSender[T any] struct {
	c          chan<- T
	senderErr  *error
	senderDone chan struct{}
	streamDone <-chan struct{}
}

// Send attempts to send x to the receiver. If the receiver closes before x can be sent, returns
// ErrClosedPipe immediately. If ctx expires before x can be sent, returns ctx.Err().
//
// A nil return does not necessarily mean that the receiver will see x, since the receiver may close
// early.
//
// Send may be called concurrently with other Sends and with Close.
func (s *PipeSender[T]) Send(ctx context.Context, x T) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.streamDone:
		return ErrClosedPipe
	case <-s.senderDone:
		return *s.senderErr
	case s.c <- x:
		return nil
	}
}

// TrySend attempts to send x to the receiver, but returns (false, nil) if the pipe's buffer is
// already full instead of blocking. If the receiver is already closed, returns ErrClosedPipe. If
// ctx expires before x can be sent, returns ctx.Err().
//
// A (true, nil) return does not necessarily mean that the receiver will see x, since the receiver
// may close early.
//
// TrySend may be called concurrently with other Sends and with Close.
func (s *PipeSender[T]) TrySend(ctx context.Context, x T) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-s.streamDone:
		return false, ErrClosedPipe
	case <-s.senderDone:
		return false, *s.senderErr
	default:
	}
	select {
	case s.c <- x:
		return true, nil
	default:
		return false, nil
	}
}

// Close closes the PipeSender, signalling to the receiver that no more values will be sent. If an
// error is provided, it will surface to the receiver's Next and to any concurrent Sends.
//
// Close may only be called once.
func (s *PipeSender[T]) Close(err error) {
	*s.senderErr = err
	close(s.senderDone)
}

type pipeStream[T any] struct {
	c          <-chan T
	senderErr  *error
	senderDone <-chan struct{}
	streamDone chan<- struct{}
}

func (s *pipeStream[T]) Next(ctx context.Context) (T, error) {
	var zero T
	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case item := <-s.c:
		return item, nil
	case <-s.senderDone:
		err := *s.senderErr
		if err != nil {
			return zero, err
		}
		return zero, End
	}
}

func (s *pipeStream[T]) Close() { close(s.streamDone) }

// Peekable allows viewing the next item from a stream without consuming it.
type Peekable[T any] interface {
	Stream[T]
	// Peek returns the next item of the stream if there is one without consuming it.
	//
	// If Peek returns a value, the next call to Next will return the same value.
	Peek(ctx context.Context) (T, error)
}

// WithPeek returns iter with a Peek() method attached.
func WithPeek[T any](s Stream[T]) Peekable[T] {
	return &peekable[T]{inner: s, has: false}
}

type peekable[T any] struct {
	inner Stream[T]
	curr  T
	has   bool
}

func (s *peekable[T]) Next(ctx context.Context) (T, error) {
	if s.has {
		item := s.curr
		s.has = false
		var zero T
		s.curr = zero
		return item, nil
	}
	return s.inner.Next(ctx)
}
func (s *peekable[T]) Peek(ctx context.Context) (T, error) {
	var zero T
	if !s.has {
		var err error
		s.curr, err = s.inner.Next(ctx)
		if err == End {
			s.has = false
			return zero, End
		} else if err != nil {
			return zero, err
		}
		s.has = true
	}
	return s.curr, nil
}
func (s *peekable[T]) Close() {
	s.inner.Close()
}

////////////////////////////////////////////////////////////////////////////////////////////////////
// Reducers                                                                                       //
// Functions that consume a stream and produce some kind of final value.                          //
////////////////////////////////////////////////////////////////////////////////////////////////////

// Collect advances s to the end and returns all of the items seen as a slice.
func Collect[T any](ctx context.Context, s Stream[T]) ([]T, error) {
	defer s.Close()

	var out []T
	for {
		item, err := s.Next(ctx)
		if err == End {
			return out, nil
		} else if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
}

// Last consumes s and returns the last n items. If s yields fewer than n items, Last returns
// all of them.
func Last[T any](ctx context.Context, s Stream[T], n int) ([]T, error) {
	defer s.Close()
	buf := make([]T, n)
	i := 0
	for {
		item, err := s.Next(ctx)
		if err == End {
			break
		} else if err != nil {
			return nil, err
		}
		buf[i%n] = item
		i++
	}
	if i < n {
		return buf[:i], nil
	}
	out := make([]T, n)
	idx := i % n
	copy(out, buf[idx:])
	copy(out[n-idx:], buf[:idx])
	return out, nil
}

// One returns the only item that s yields. Returns an error if encountered, or if s yields zero or
// more than one item.
func One[T any](ctx context.Context, s Stream[T]) (T, error) {
	var zero T
	x, err := s.Next(ctx)
	if err == End {
		return zero, ErrEmpty
	} else if err != nil {
		return zero, err
	}
	_, err = s.Next(ctx)
	if err == End {
		return x, nil
	} else if err != nil {
		return zero, err
	}
	return zero, ErrMoreThanOne
}

// Reduce reduces s to a single value using the reduction function f.
func Reduce[T any, U any](
	ctx context.Context,
	s Stream[T],
	initial U,
	f func(U, T) (U, error),
) (U, error) {
	defer s.Close()

	acc := initial
	for {
		item, err := s.Next(ctx)
		if err == End {
			return acc, nil
		} else if err != nil {
			return acc, err
		}
		acc, err = f(acc, item)
		if err != nil {
			return acc, err
		}
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////
// Combinators                                                                                    //
// Functions that take and return iterators, transforming the output somehow.                     //
////////////////////////////////////////////////////////////////////////////////////////////////////

// Batch returns a stream of non-overlapping batches from s of size batchSize. Batch is similar to
// Chunk with the added feature that an underfilled batch will be delivered to the output stream if
// any item has been in the batch for more than maxWait.
func Batch[T any](s Stream[T], maxWait time.Duration, batchSize int) Stream[[]T] {
	return BatchFunc(s, maxWait, func(batch []T) bool {
		return len(batch) >= batchSize
	})
}

// BatchFunc returns a stream of non-overlapping batches from s, using full to determine when a
// batch is full. BatchFunc is similar to Chunk with the added feature that an underfilled batch
// will be delivered to the output stream if any item has been in the batch for more than maxWait.
func BatchFunc[T any](
	s Stream[T],
	maxWait time.Duration,
	full func(batch []T) bool,
) Stream[[]T] {
	bgCtx, bgCancel := context.WithCancel(context.Background())

	out := &batchStream[T]{
		batchC:   make(chan []T),
		waiting:  make(chan struct{}),
		bgCancel: bgCancel,
	}

	c := make(chan T)

	out.wg.Add(2)

	go func() {
		defer out.wg.Done()
		defer s.Close()
		defer close(c)

		for {
			item, err := s.Next(bgCtx)
			if err == End {
				break
			} else if err == context.Canceled && bgCtx.Err() == context.Canceled {
				break
			} else if err != nil {
				out.err = err
				return
			}
			c <- item
		}
	}()

	// Build up batches and flush them when either:
	// A) The batch is full.
	// B) It's been at least maxWait since the first item arrived _and_ there is somebody waiting.
	// No sense in underfilling a batch if nobody's actually asking for it yet.
	// C) There aren't any more items.
	go func() {
		defer out.wg.Done()

		var batch []T
		var batchStart time.Time
		batchSizeEstimate := 0
		var timer *time.Timer
		// Starts off as nil so that the timerC select arm isn't chosen until populated.  Also set
		// to nil when we've already stopped or received from timer to know when it needs to be
		// drained.
		var timerC <-chan time.Time
		waitingAtEmpty := false

		defer func() {
			if timer != nil {
				timer.Stop()
			}
			close(out.batchC)
		}()

		flush := func() bool {
			select {
			case <-bgCtx.Done():
				return false
			case out.batchC <- batch:
			}
			batchSizeEstimate = (batchSizeEstimate + len(batch)) / 2
			batch = make([]T, 0, xmath.Max(len(batch), batchSizeEstimate*11/10))
			waitingAtEmpty = false
			return true
		}

		stopTimer := func() {
			if timer == nil {
				return
			}
			stopped := timer.Stop()
			if !stopped && timerC != nil {
				<-timerC
			}
			timerC = nil
		}

		startTimer := func() {
			stopTimer()
			if timer == nil {
				timer = time.NewTimer(maxWait - time.Since(batchStart))
			} else {
				timer.Reset(maxWait - time.Since(batchStart))
			}
			timerC = timer.C
		}

		for {
			select {
			case item, ok := <-c:
				if !ok { // Case (C): we're done.
					// Flush what we have so far, if any.
					if len(batch) > 0 {
						_ = flush()
					}
					return
				}
				batch = append(batch, item)
				if full(batch) { // Case (A): the batch is full.
					stopTimer()
					if !flush() {
						return
					}
				}
				if len(batch) == 1 { // Bookkeeping for case (B).
					batchStart = time.Now()
					if waitingAtEmpty {
						startTimer()
					}
				}
			case <-timerC: // Case (B).
				timerC = nil
				// Being here already implies the conditions are true, since the timer is only
				// running while the batch is non-empty and there's somebody waiting.
				if !flush() {
					return
				}
			case <-out.waiting: // Bookkeeping for case (B).
				if len(batch) > 0 {
					// Time already elapsed, just deliver the batch now.
					if time.Since(batchStart) > maxWait {
						if !flush() {
							return
						}
					} else {
						startTimer()
					}
				} else {
					// Timer will start when the first item shows up.
					waitingAtEmpty = true
				}
			}
		}
	}()

	return out
}

type batchStream[T any] struct {
	bgCancel context.CancelFunc
	wg       sync.WaitGroup
	batchC   chan []T
	// populated at most once and always before batchC closes
	err     error
	waiting chan struct{}
}

func (iter *batchStream[T]) Next(ctx context.Context) ([]T, error) {
	select {
	// There might be a batch already ready because it filled before we even asked.
	case batch, ok := <-iter.batchC:
		if !ok {
			if iter.err != nil {
				return nil, iter.err
			}
			return nil, End
		}
		return batch, nil
	// Otherwise, we need to let the sender know we're waiting so that they can flush an underfilled
	// batch at interval.
	case iter.waiting <- struct{}{}:
		select {
		case batch, ok := <-iter.batchC:
			if !ok {
				if iter.err != nil {
					return nil, iter.err
				}
				return nil, End
			}
			return batch, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (iter *batchStream[T]) Close() {
	iter.bgCancel()
	iter.wg.Wait()
}

// Chunk returns a stream of non-overlapping chunks from s of size chunkSize. The last chunk will be
// smaller than chunkSize if the stream does not contain an even multiple.
func Chunk[T any](s Stream[T], chunkSize int) Stream[[]T] {
	return &chunkStream[T]{
		inner:     s,
		chunkSize: chunkSize,
	}
}

type chunkStream[T any] struct {
	inner     Stream[T]
	chunkSize int
	chunk     []T
}

func (s *chunkStream[T]) Next(ctx context.Context) ([]T, error) {
	for {
		item, err := s.inner.Next(ctx)
		if err == End {
			break
		} else if err != nil {
			return nil, err
		}
		s.chunk = append(s.chunk, item)
		if len(s.chunk) == s.chunkSize {
			chunk := s.chunk
			s.chunk = make([]T, 0, s.chunkSize)
			return chunk, nil
		}
	}
	if len(s.chunk) > 0 {
		chunk := s.chunk
		s.chunk = make([]T, 0, s.chunkSize)
		return chunk, nil
	}
	return nil, End
}

func (s *chunkStream[T]) Close() {
	s.inner.Close()
}

// Compact elides adjacent duplicates from s.
func Compact[T comparable](s Stream[T]) Stream[T] {
	return CompactFunc(s, func(a, b T) bool {
		return a == b
	})
}

// CompactFunc elides adjacent duplicates from s, using eq to determine duplicates.
func CompactFunc[T any](s Stream[T], eq func(T, T) bool) Stream[T] {
	return &compactStream[T]{
		inner: s,
		first: true,
		eq:    eq,
	}
}

type compactStream[T any] struct {
	inner Stream[T]
	prev  T
	first bool
	eq    func(T, T) bool
}

func (s *compactStream[T]) Next(ctx context.Context) (T, error) {
	for {
		item, err := s.inner.Next(ctx)
		if err != nil {
			return item, err
		}

		if s.first {
			s.first = false
			s.prev = item
			return item, nil
		} else if !s.eq(s.prev, item) {
			s.prev = item
			return item, nil
		}
	}
}

func (s *compactStream[T]) Close() {
	s.inner.Close()
}

// Filter returns a Stream that yields only the items from s for which keep returns true. If keep
// returns an error, terminates the stream early.
func Filter[T any](s Stream[T], keep func(context.Context, T) (bool, error)) Stream[T] {
	return &filterStream[T]{inner: s, keep: keep}
}

type filterStream[T any] struct {
	inner Stream[T]
	keep  func(context.Context, T) (bool, error)
}

func (s *filterStream[T]) Next(ctx context.Context) (T, error) {
	var zero T
	for {
		item, err := s.inner.Next(ctx)
		if err != nil {
			return zero, err
		}
		ok, err := s.keep(ctx, item)
		if err != nil {
			return zero, err
		}
		if ok {
			return item, nil
		}
	}
}

func (s *filterStream[T]) Close() {
	s.inner.Close()
}

// First returns a Stream that yields the first n items from s.
func First[T any](s Stream[T], n int) Stream[T] {
	return &firstStream[T]{inner: s, x: n}
}

type firstStream[T any] struct {
	inner Stream[T]
	x     int
}

func (s *firstStream[T]) Next(ctx context.Context) (T, error) {
	if s.x <= 0 {
		var zero T
		return zero, End
	}
	item, err := s.inner.Next(ctx)
	if err != nil {
		return item, err
	}
	s.x--
	return item, nil
}

func (s *firstStream[T]) Close() {
	s.inner.Close()
}

// Flatten returns a stream that yields all items from all streams yielded by s.
func Flatten[T any](s Stream[Stream[T]]) Stream[T] {
	return &flattenStream[T]{inner: s}
}

type flattenStream[T any] struct {
	inner Stream[Stream[T]]
	curr  Stream[T]
}

func (s *flattenStream[T]) Next(ctx context.Context) (T, error) {
	for {
		if s.curr == nil {
			var err error
			s.curr, err = s.inner.Next(ctx)
			if err != nil {
				var zero T
				return zero, err
			}
		}

		item, err := s.curr.Next(ctx)
		if err == End {
			s.curr.Close()
			s.curr = nil
			continue
		} else if err != nil {
			return item, err
		}

		return item, nil
	}
}

func (s *flattenStream[T]) Close() {
	if s.curr != nil {
		s.curr.Close()
	}
	s.inner.Close()
}

// Join returns a Stream that yields all elements from streams[0], then all elements from
// streams[1], and so on.
func Join[T any](streams ...Stream[T]) Stream[T] {
	return &joinStream[T]{remaining: streams}
}

type joinStream[T any] struct {
	remaining []Stream[T]
}

func (s *joinStream[T]) Next(ctx context.Context) (T, error) {
	var zero T
	for len(s.remaining) > 0 {
		item, err := s.remaining[0].Next(ctx)
		if err == End {
			s.remaining[0].Close()
			s.remaining = s.remaining[1:]
			continue
		} else if err != nil {
			return zero, err
		}
		return item, nil
	}
	return zero, End
}

func (s *joinStream[T]) Close() {
	for i := range s.remaining {
		s.remaining[i].Close()
	}
}

// Map transforms the values of s using the conversion f. If f returns an error, terminates the
// stream early.
func Map[T any, U any](s Stream[T], f func(context.Context, T) (U, error)) Stream[U] {
	return &mapStream[T, U]{inner: s, f: f}
}

type mapStream[T any, U any] struct {
	inner Stream[T]
	f     func(context.Context, T) (U, error)
}

func (s *mapStream[T, U]) Next(ctx context.Context) (U, error) {
	var zero U
	item, err := s.inner.Next(ctx)
	if err != nil {
		return zero, err
	}
	mapped, err := s.f(ctx, item)
	if err != nil {
		return zero, err
	}
	return mapped, nil
}

func (s *mapStream[T, U]) Close() {
	s.inner.Close()
}

// Merge merges the in streams, returning a stream that yields all elements from all of them as they
// arrive.
func Merge[T any](in ...Stream[T]) Stream[T] {
	sender, receiver := Pipe[T](0)
	nDone := uint32(0)
	closeOnce := uint32(0)
	ctx, cancel := context.WithCancel(context.Background())
	for i := 0; i < len(in); i++ {
		i := i
		go func() {
			defer func() {
				if int(atomic.AddUint32(&nDone, 1)) == len(in) &&
					atomic.LoadUint32(&closeOnce) == 0 {
					sender.Close(nil)
				}
			}()
			for {
				item, err := in[i].Next(ctx)
				if err == End {
					return
				} else if err != nil {
					if atomic.CompareAndSwapUint32(&closeOnce, 0, 1) {
						cancel()
						sender.Close(err)
					}
					return
				}
				err = sender.Send(ctx, item)
				if err != nil {
					// Implies ctx has expired or the receiver closed, either way we're done.
					return
				}
			}
		}()
	}
	return receiver
}

type mergeStream[T any] struct {
	inner  Stream[T]
	cancel func()
}

func (s *mergeStream[T]) Next(ctx context.Context) (T, error) {
	return s.inner.Next(ctx)
}

func (s *mergeStream[T]) Close() {
	s.inner.Close()
	s.cancel()
}

// Runs returns a stream of streams. The inner streams yield contiguous elements from s such that
// same(a, b) returns true for any a and b in the run.
//
// The inner stream should be drained before calling Next on the outer stream.
//
// same(a, a) must return true. If same(a, b) and same(b, c) both return true, then same(a, c) must
// also.
func Runs[T any](s Stream[T], same func(a, b T) bool) Stream[Stream[T]] {
	return &runsStream[T]{
		inner: WithPeek(s),
		same:  same,
		curr:  nil,
	}
}

type runsStream[T any] struct {
	inner Peekable[T]
	same  func(a, b T) bool
	curr  *runsInnerStream[T]
}

func (s *runsStream[T]) Next(ctx context.Context) (Stream[T], error) {
	if s.curr != nil {
		for {
			_, err := s.curr.Next(ctx)
			if err == End {
				break
			} else if err != nil {
				return nil, err
			}
		}
		s.curr.Close()
		s.curr = nil
	}
	item, err := s.inner.Peek(ctx)
	if err != nil {
		return nil, err
	}
	s.curr = &runsInnerStream[T]{parent: s, prev: item}
	return s.curr, nil
}

func (s *runsStream[T]) Close() {
	s.inner.Close()
}

type runsInnerStream[T any] struct {
	parent *runsStream[T]
	prev   T
}

func (s *runsInnerStream[T]) Next(ctx context.Context) (T, error) {
	var zero T
	if s.parent == nil {
		return zero, End
	}
	item, err := s.parent.inner.Peek(ctx)
	if err == End {
		return zero, End
	} else if err != nil {
		return zero, err
	} else if !s.parent.same(s.prev, item) {
		return zero, End
	}
	return s.parent.inner.Next(ctx)
}

func (s *runsInnerStream[T]) Close() { s.parent = nil }

// While returns a Stream that terminates before the first item from s for which f returns false.
// If f returns an error, terminates the stream early.
func While[T any](s Stream[T], f func(context.Context, T) (bool, error)) Stream[T] {
	return &whileStream[T]{
		inner: s,
		f:     f,
	}
}

type whileStream[T any] struct {
	inner Stream[T]
	f     func(context.Context, T) (bool, error)
	item  T
	has   bool
	done  bool
}

func (s *whileStream[T]) Next(ctx context.Context) (T, error) {
	var zero T
	if s.done {
		return zero, End
	}
	if !s.has {
		var err error
		s.item, err = s.inner.Next(ctx)
		if err != nil {
			return zero, err
		}
		s.has = true
	}
	ok, err := s.f(ctx, s.item)
	if err != nil {
		return zero, err
	}
	if !ok {
		s.done = true
		return zero, End
	}
	s.has = false
	return s.item, nil
}

func (s *whileStream[T]) Close() {
	s.inner.Close()
}
