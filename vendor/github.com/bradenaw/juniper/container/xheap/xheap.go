// Package xheap contains extensions to the standard library package container/heap.
package xheap

import (
	"github.com/bradenaw/juniper/internal/heap"
	"github.com/bradenaw/juniper/iterator"
	"github.com/bradenaw/juniper/xsort"
)

// Heap is a min-heap (https://en.wikipedia.org/wiki/Binary_heap). Min-heaps are a collection
// structure that provide constant-time access to the minimum element, and logarithmic-time removal.
// They are most commonly used as a priority queue.
//
// Push and Pop take amortized O(log(n)) time where n is the number of items in the heap.
//
// Len and Peek take O(1) time.
type Heap[T any] struct {
	// Indirect here so that Heap behaves as a reference type, like the map builtin.
	inner *heap.Heap[T]
}

// New returns a new Heap which uses less to determine the minimum element.
//
// The elements from initial are added to the heap. initial is modified by New and utilized by the
// Heap, so it should not be used after passing to New(). Passing initial is faster (O(n)) than
// creating an empty heap and pushing each item (O(n * log(n))).
func New[T any](less xsort.Less[T], initial []T) Heap[T] {
	inner := heap.New(
		func(a, b T) bool {
			return less(a, b)
		},
		func(a T, i int) {},
		initial,
	)
	return Heap[T]{
		inner: &inner,
	}
}

// Len returns the current number of elements in the heap.
func (h Heap[T]) Len() int {
	return h.inner.Len()
}

// Grow allocates sufficient space to add n more elements without needing to reallocate.
func (h Heap[T]) Grow(n int) {
	h.inner.Grow(n)
}

// Shrink reallocates the backing buffer for h, if necessary, so that it fits only the current size
// plus at most n extra items.
func (h Heap[T]) Shrink(n int) {
	h.inner.Shrink(n)
}

// Push adds item to the heap.
func (h Heap[T]) Push(item T) {
	h.inner.Push(item)
}

// Pop removes and returns the minimum item in the heap. It panics if h.Len()==0.
func (h Heap[T]) Pop() T {
	return h.inner.Pop()
}

// Peek returns the minimum item in the heap. It panics if h.Len()==0.
func (h Heap[T]) Peek() T {
	return h.inner.Peek()
}

// Iterate iterates over the elements of the heap.
//
// The iterator panics if the heap has been modified since iteration started.
func (h Heap[T]) Iterate() iterator.Iterator[T] {
	return h.inner.Iterate()
}

// KP holds key and priority for PriorityQueue.
type KP[K any, P any] struct {
	K K
	P P
}

// PriorityQueue is a queue that yields items in increasing order of priority.
type PriorityQueue[K comparable, P any] struct {
	// Indirect here so that Heap behaves as a reference type, like the map builtin.
	inner *heap.Heap[KP[K, P]]
	m     map[K]int
}

// NewPriorityQueue returns a new PriorityQueue which uses less to determine the minimum element.
//
// The elements from initial are added to the priority queue. initial is modified by
// NewPriorityQueue and utilized by the PriorityQueue, so it should not be used after passing to
// NewPriorityQueue. Passing initial is faster (O(n)) than creating an empty priority queue and
// pushing each item (O(n * log(n))).
//
// Pop, Remove, and Update all take amortized O(log(n)) time where n is the number of items in the
// queue.
//
// Len, Peek, Contains, and Priority take O(1) time.
func NewPriorityQueue[K comparable, P any](
	less xsort.Less[P],
	initial []KP[K, P],
) PriorityQueue[K, P] {
	h := PriorityQueue[K, P]{
		m: make(map[K]int),
	}
	filtered := initial[:0]
	for _, kp := range initial {
		_, ok := h.m[kp.K]
		if ok {
			continue
		}
		h.m[kp.K] = -1
		filtered = append(filtered, kp)
	}
	initial = filtered
	inner := heap.New(
		func(a, b KP[K, P]) bool {
			return less(a.P, b.P)
		},
		func(x KP[K, P], i int) {
			h.m[x.K] = i
		},
		initial,
	)
	h.inner = &inner
	return h
}

// Len returns the current number of elements in the priority queue.
func (h PriorityQueue[K, P]) Len() int {
	return h.inner.Len()
}

// Grow allocates sufficient space to add n more elements without needing to reallocate.
func (h PriorityQueue[K, P]) Grow(n int) {
	h.inner.Grow(n)
}

// Update updates the priority of k to p, or adds it to the priority queue if not present.
func (h PriorityQueue[K, P]) Update(k K, p P) {
	idx, ok := h.m[k]
	if ok {
		h.inner.UpdateAt(idx, KP[K, P]{k, p})
	} else {
		h.inner.Push(KP[K, P]{k, p})
	}
}

// Pop removes and returns the lowest-P item in the priority queue. It panics if h.Len()==0.
func (h PriorityQueue[K, P]) Pop() K {
	item := h.inner.Pop()
	delete(h.m, item.K)
	return item.K
}

// Peek returns the key of the lowest-P item in the priority queue. It panics if h.Len()==0.
func (h PriorityQueue[K, P]) Peek() K {
	return h.inner.Peek().K
}

// Contains returns true if the given key is present in the priority queue.
func (h PriorityQueue[K, P]) Contains(k K) bool {
	_, ok := h.m[k]
	return ok
}

// Priority returns the priority of k, or the zero value of P if k is not present.
func (h PriorityQueue[K, P]) Priority(k K) P {
	idx, ok := h.m[k]
	if ok {
		return h.inner.Item(idx).P
	}
	var zero P
	return zero
}

// Remove removes the item with the given key if present.
func (h PriorityQueue[K, P]) Remove(k K) {
	i, ok := h.m[k]
	if !ok {
		return
	}
	h.inner.RemoveAt(i)
	delete(h.m, k)
}

// Iterate iterates over the elements of the priority queue.
//
// The iterator panics if the priority queue has been modified since iteration started.
func (h PriorityQueue[K, P]) Iterate() iterator.Iterator[K] {
	return iterator.Map(h.inner.Iterate(), func(kp KP[K, P]) K { return kp.K })
}
