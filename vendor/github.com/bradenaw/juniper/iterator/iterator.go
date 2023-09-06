// Package iterator allows iterating over sequences of values, for example the contents of a
// container.
package iterator

// Iterator is used to iterate over a sequence of values.
//
// Iterators are lazy, meaning they do no work until a call to Next().
//
// Iterators do not need to be fully consumed, callers may safely abandon an iterator before Next
// returns false.
type Iterator[T any] interface {
	// Next advances the iterator and returns the next item. Once the iterator is finished, the
	// first return is meaningless and the second return is false. Note that the final value of the
	// iterator has true in the second return, and it's the following call that returns false in the
	// second return.
	//
	// Once Next returns false in the second return, it is expected that it will always return false
	// afterwards.
	Next() (T, bool)
}

////////////////////////////////////////////////////////////////////////////////////////////////////
// Converters                                                                                     //
// Functions that produce an Iterator from some other type.                                       //
////////////////////////////////////////////////////////////////////////////////////////////////////

// Chan returns an Iterator that yields the values received on c.
func Chan[T any](c <-chan T) Iterator[T] {
	return &chanIterator[T]{c: c}
}

type chanIterator[T any] struct {
	c <-chan T
}

func (iter *chanIterator[T]) Next() (T, bool) {
	item, ok := <-iter.c
	return item, ok
}

// Counter returns an iterator that counts up from 0, yielding n items.
//
// The following are equivalent:
//
//	for i := 0; i < n; i++ {
//	  fmt.Println(n)
//	}
//
//
//	iter := iterator.Counter(n)
//	for {
//	  item, ok := iter.Next()
//	  if !ok {
//	    break
//	  }
//	  fmt.Println(item)
//	}
func Counter(n int) Iterator[int] {
	return &counterIterator{i: 0, n: n}
}

type counterIterator struct {
	i int
	n int
}

func (iter *counterIterator) Next() (int, bool) {
	if iter.i >= iter.n {
		return 0, false
	}
	item := iter.i
	iter.i++
	return item, true
}

// Empty returns an iterator that yields no items.
func Empty[T any]() Iterator[T] {
	return emptyIterator[T]{}
}

type emptyIterator[T any] struct{}

func (iter emptyIterator[T]) Next() (T, bool) {
	var zero T
	return zero, false
}

// Repeat returns an iterator that yields item n times.
func Repeat[T any](item T, n int) Iterator[T] {
	return &repeatIterator[T]{
		item: item,
		x:    n,
	}
}

type repeatIterator[T any] struct {
	item T
	x    int
}

func (iter *repeatIterator[T]) Next() (T, bool) {
	if iter.x <= 0 {
		var zero T
		return zero, false
	}
	iter.x--
	return iter.item, true
}

// Slice returns an iterator over the elements of s.
func Slice[T any](s []T) Iterator[T] {
	return &sliceIterator[T]{
		a: s,
	}
}

type sliceIterator[T any] struct {
	a []T
}

func (iter *sliceIterator[T]) Next() (T, bool) {
	if len(iter.a) == 0 {
		var zero T
		return zero, false
	}
	item := iter.a[0]
	iter.a = iter.a[1:]
	return item, true
}

// Peekable allows viewing the next item from an iterator without consuming it.
type Peekable[T any] interface {
	Iterator[T]
	// Peek returns the next item of the iterator if there is one without consuming it.
	//
	// If Peek returns a value, the next call to Next will return the same value.
	Peek() (T, bool)
}

// WithPeek returns iter with a Peek() method attached.
func WithPeek[T any](iter Iterator[T]) Peekable[T] {
	return &peekable[T]{inner: iter, has: false}
}

type peekable[T any] struct {
	inner Iterator[T]
	curr  T
	has   bool
}

func (iter *peekable[T]) Next() (T, bool) {
	if iter.has {
		item := iter.curr
		iter.has = false
		var zero T
		iter.curr = zero
		return item, true
	}
	return iter.inner.Next()
}
func (iter *peekable[T]) Peek() (T, bool) {
	if !iter.has {
		iter.curr, iter.has = iter.inner.Next()
	}
	return iter.curr, iter.has
}

////////////////////////////////////////////////////////////////////////////////////////////////////
// Reducers                                                                                       //
// Functions that consume an iterator and produce some kind of final value.                       //
////////////////////////////////////////////////////////////////////////////////////////////////////

// Collect advances iter to the end and returns all of the items seen as a slice.
func Collect[T any](iter Iterator[T]) []T {
	return Reduce(iter, nil, func(out []T, item T) []T {
		return append(out, item)
	})
}

// Equal returns true if the given iterators yield the same items in the same order. Consumes the
// iterators.
func Equal[T comparable](iters ...Iterator[T]) bool {
	if len(iters) == 0 {
		return true
	}
	for {
		item, ok := iters[0].Next()
		for i := 1; i < len(iters); i++ {
			iterIItem, iterIOk := iters[i].Next()
			if ok != iterIOk {
				return false
			}
			if ok && item != iterIItem {
				return false
			}
		}
		if !ok {
			return true
		}
	}
}

// Last consumes iter and returns the last n items. If iter yields fewer than n items, Last returns
// all of them.
func Last[T any](iter Iterator[T], n int) []T {
	buf := make([]T, n)
	i := 0
	for {
		item, ok := iter.Next()
		if !ok {
			break
		}
		buf[i%n] = item
		i++
	}
	if i < n {
		return buf[:i]
	}
	out := make([]T, n)
	idx := i % n
	copy(out, buf[idx:])
	copy(out[n-idx:], buf[:idx])
	return out
}

// One returns the only item yielded by iter. Returns false in the second return if iter yields zero
// or more than one item.
func One[T any](iter Iterator[T]) (T, bool) {
	var zero T
	x, ok := iter.Next()
	if !ok {
		return zero, false
	}
	_, ok = iter.Next()
	if ok {
		return zero, false
	}
	return x, true
}

// Reduce reduces iter to a single value using the reduction function f.
func Reduce[T any, U any](iter Iterator[T], initial U, f func(U, T) U) U {
	acc := initial
	for {
		item, ok := iter.Next()
		if !ok {
			return acc
		}
		acc = f(acc, item)
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////
// Combinators                                                                                    //
// Functions that take and return iterators, transforming the output somehow.                     //
////////////////////////////////////////////////////////////////////////////////////////////////////

// Chunk returns an iterator over non-overlapping chunks of size chunkSize. The last chunk will be
// smaller than chunkSize if the iterator does not contain an even multiple.
func Chunk[T any](iter Iterator[T], chunkSize int) Iterator[[]T] {
	return &chunkIterator[T]{
		inner:     iter,
		chunkSize: chunkSize,
	}
}

type chunkIterator[T any] struct {
	inner     Iterator[T]
	chunkSize int
}

func (iter *chunkIterator[T]) Next() ([]T, bool) {
	chunk := make([]T, 0, iter.chunkSize)
	for {
		item, ok := iter.inner.Next()
		if !ok {
			break
		}
		chunk = append(chunk, item)
		if len(chunk) == iter.chunkSize {
			return chunk, true
		}
	}
	if len(chunk) > 0 {
		return chunk, true
	}
	return nil, false
}

// Compact elides adjacent duplicates from iter.
func Compact[T comparable](iter Iterator[T]) Iterator[T] {
	return CompactFunc(iter, func(a, b T) bool {
		return a == b
	})
}

// CompactFunc elides adjacent duplicates from iter, using eq to determine duplicates.
func CompactFunc[T any](iter Iterator[T], eq func(T, T) bool) Iterator[T] {
	return &compactIterator[T]{
		inner: iter,
		first: true,
		eq:    eq,
	}
}

type compactIterator[T any] struct {
	inner Iterator[T]
	prev  T
	first bool
	eq    func(T, T) bool
}

func (iter *compactIterator[T]) Next() (T, bool) {
	for {
		item, ok := iter.inner.Next()
		if !ok {
			return item, false
		}

		if iter.first {
			iter.first = false
			iter.prev = item
			return item, true
		} else if !iter.eq(iter.prev, item) {
			iter.prev = item
			return item, true
		}
	}
}

// Filter returns an iterator that yields only the items from iter for which keep returns true.
func Filter[T any](iter Iterator[T], keep func(T) bool) Iterator[T] {
	return &filterIterator[T]{inner: iter, keep: keep}
}

type filterIterator[T any] struct {
	inner Iterator[T]
	keep  func(T) bool
}

func (iter *filterIterator[T]) Next() (T, bool) {
	for {
		item, ok := iter.inner.Next()
		if !ok {
			break
		}
		if iter.keep(item) {
			return item, true
		}
	}
	var zero T
	return zero, false
}

// First returns an iterator that yields the first n items from iter.
func First[T any](iter Iterator[T], n int) Iterator[T] {
	return &firstIterator[T]{inner: iter, x: n}
}

type firstIterator[T any] struct {
	inner Iterator[T]
	x     int
}

func (iter *firstIterator[T]) Next() (T, bool) {
	if iter.x <= 0 {
		var zero T
		return zero, false
	}
	iter.x--
	return iter.inner.Next()
}

// Flatten returns an iterator that yields all items from all iterators yielded by iter.
func Flatten[T any](iter Iterator[Iterator[T]]) Iterator[T] {
	return &flattenIterator[T]{inner: iter}
}

type flattenIterator[T any] struct {
	inner Iterator[Iterator[T]]
	curr  Iterator[T]
}

func (iter *flattenIterator[T]) Next() (T, bool) {
	for {
		if iter.curr == nil {
			var ok bool
			iter.curr, ok = iter.inner.Next()
			if !ok {
				var zero T
				return zero, false
			}
		}

		item, ok := iter.curr.Next()
		if !ok {
			iter.curr = nil
			continue
		}
		return item, true
	}
}

// Join returns an Iterator that returns all elements of iters[0], then all elements of iters[1],
// and so on.
func Join[T any](iters ...Iterator[T]) Iterator[T] {
	return &joinIterator[T]{
		iters: iters,
	}
}

type joinIterator[T any] struct {
	iters []Iterator[T]
}

func (iter *joinIterator[T]) Next() (T, bool) {
	for len(iter.iters) > 0 {
		item, ok := iter.iters[0].Next()
		if ok {
			return item, true
		}
		iter.iters = iter.iters[1:]
	}
	var zero T
	return zero, false
}

// Map transforms the results of iter using the conversion f.
func Map[T any, U any](iter Iterator[T], f func(t T) U) Iterator[U] {
	return &mapIterator[T, U]{
		inner: iter,
		f:     f,
	}
}

type mapIterator[T any, U any] struct {
	inner Iterator[T]
	f     func(T) U
}

func (iter *mapIterator[T, U]) Next() (U, bool) {
	var zero U
	item, ok := iter.inner.Next()
	if !ok {
		return zero, false
	}
	return iter.f(item), true
}

// Runs returns an iterator of iterators. The inner iterators yield contiguous elements from iter
// such that same(a, b) returns true for any a and b in the run.
//
// The inner iterator should be drained before calling Next on the outer iterator.
//
// same(a, a) must return true. If same(a, b) and same(b, c) both return true, then same(a, c) must
// also.
func Runs[T any](iter Iterator[T], same func(a, b T) bool) Iterator[Iterator[T]] {
	return &runsIterator[T]{
		inner: WithPeek(iter),
		same:  same,
		curr:  nil,
	}
}

type runsIterator[T any] struct {
	inner Peekable[T]
	same  func(a, b T) bool
	curr  *runsInnerIterator[T]
}

func (iter *runsIterator[T]) Next() (Iterator[T], bool) {
	if iter.curr != nil {
		for {
			_, ok := iter.curr.Next()
			if !ok {
				break
			}
		}
		iter.curr = nil
	}
	item, ok := iter.inner.Peek()
	if !ok {
		return nil, false
	}
	iter.curr = &runsInnerIterator[T]{parent: iter, prev: item}
	return iter.curr, true
}

type runsInnerIterator[T any] struct {
	parent *runsIterator[T]
	prev   T
}

func (iter *runsInnerIterator[T]) Next() (T, bool) {
	var zero T
	if iter.parent == nil {
		return zero, false
	}
	item, ok := iter.parent.inner.Peek()
	if !ok || !iter.parent.same(iter.prev, item) {
		iter.parent = nil
		return zero, false
	}
	return iter.parent.inner.Next()
}

// While returns an iterator that terminates before the first item from iter for which f returns
// false.
func While[T any](iter Iterator[T], f func(T) bool) Iterator[T] {
	return &whileIterator[T]{
		inner: iter,
		f:     f,
		done:  false,
	}
}

type whileIterator[T any] struct {
	inner Iterator[T]
	f     func(T) bool
	done  bool
}

func (iter *whileIterator[T]) Next() (T, bool) {
	var zero T
	if iter.done {
		return zero, false
	}
	item, ok := iter.inner.Next()
	if !ok {
		return zero, false
	}
	if !iter.f(item) {
		iter.done = true
		return zero, false
	}
	return item, true
}
