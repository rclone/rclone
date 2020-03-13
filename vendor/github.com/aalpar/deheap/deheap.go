//
// Copyright 2019 Aaron H. Alpar
//
// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files
// (the "Software"), to deal in the Software without restriction,
// including without limitation the rights to use, copy, modify, merge,
// publish, distribute, sublicense, and/or sell copies of the Software,
// and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
// IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY
// CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
// TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
// SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
//

//
// Package deheap provides the implementation of a doubly ended heap.
// Doubly ended heaps are heaps with two sides, a min side and a max side.
// Like normal single-sided heaps, elements can be pushed onto and pulled
// off of a deheap.  deheaps have an additional Pop function, PopMax, that
// returns elements from the opposite side of the ordering.
//
// This implementation has emphasized compatibility with existing libraries
// in the sort and heap packages.
//
// Performace of the deheap functions should be very close to the
// performance of the functions of the heap library
//
package deheap

import (
	"container/heap"
	"math/bits"
)

func hparent(i int) int {
	return (i - 1) / 2
}

func hlchild(i int) int {
	return (i * 2) + 1
}

func parent(i int) int {
	return ((i + 1) / 4) - 1
}

func lchild(i int) int {
	return ((i + 1 ) * 4) - 1
}

func level(i int) int {
	return bits.Len(uint(i)+1) - 1
}

func isMinHeap(i int) bool {
	return level(i) % 2 == 0
}

func min4(h heap.Interface, l int, min bool, i int)  int {
	q := i
	i++
	if i >= l {
		return q
	}
	if min == h.Less(i, q) {
		q = i
	}
	i++
	if i >= l {
		return q
	}
	if min == h.Less(i, q) {
		q = i
	}
	i++
	if i >= l {
		return q
	}
	if min == h.Less(i, q) {
		q = i
	}
	return q
}

// min2
func min2(h heap.Interface, l int, min bool, i int) int {
	if i+1 >= l {
		return i
	}
	if min != h.Less(i+1, i) {
		return i
	}
	return i + 1
}

// min3
func min3(h heap.Interface, l int, min bool, i, j, k int) int {
	q := i
	if j < l && h.Less(j, q) == min {
		q = j
	}
	if k < l && h.Less(k, q) == min {
		q = k
	}
	return q
}

// bubbledown
func bubbledown(h heap.Interface, l int, min bool, i int) (q int, r int) {
	q = i
	r = i
	for {
		// find min of children
		j := min2(h, l, min, hlchild(i))
		if j >= l {
			break
		}
		// find min of grandchildren
		k := min4(h, l, min, lchild(i))
		// swap of less than the element at i
		v := min3(h, l, min, i, j, k)
		if v == i || v >= l {
			break
		}
		// v == k
		q = v
		h.Swap(v, i)
		if v == j {
			break
		}
		p := hparent(v)
		if h.Less(p, v) == min {
			h.Swap(p, v)
			r = p
		}
		i = v
	}
	return q, r
}

// bubbleup
func bubbleup(h heap.Interface, min bool, i int) (q bool) {
	if i < 0 {
		return false
	}
	j := parent(i)
	for j >= 0 && min == h.Less(i, j) {
		q = true
		h.Swap(i, j)
		i = j
		j = parent(i)
	}
	min = !min
	j = hparent(i)
	for j >= 0 && min == h.Less(i, j) {
		q = true
		h.Swap(i, j)
		i = j
		j = parent(i)
	}
	return q
}

// Pop the smallest value off the heap.  See heap.Pop().
// Time complexity is O(log n), where n = h.Len()
func Pop(h heap.Interface) interface{} {
	l := h.Len()-1
	h.Swap(0, l)
	q := h.Pop()
	bubbledown(h, l, true, 0)
	return q
}

// Pop the largest value off the heap.  See heap.Pop().
// Time complexity is O(log n), where n = h.Len()
func PopMax(h heap.Interface) interface{} {
	l := h.Len()
	j := 0
	if l > 1 {
		j = min2(h, l,false, 1)
	}
	l = l - 1
	h.Swap(j, l)
	q := h.Pop()
	bubbledown(h, l,false, j)
	return q
}

// Remove element at index i.  See heap.Remove().
// The complexity is O(log n) where n = h.Len().
func Remove(h heap.Interface, i int) (q interface{}) {
	l := h.Len() - 1
	h.Swap(i, l)
	q = h.Pop()
	if l != i {
		q, r := bubbledown(h, l, isMinHeap(i), i)
		bubbleup(h, isMinHeap(q), q)
		bubbleup(h, isMinHeap(r), r)
	}
	return q
}

// Push an element onto the heap.  See heap.Push()
// Time complexity is O(log n), where n = h.Len()
func Push(h heap.Interface, o interface{}) {
	h.Push(o)
	l := h.Len()
	i := l - 1
	bubbleup(h, isMinHeap(i), i)
}

// Init initializes the heap.
// This should be called once on non-empty heaps before calling Pop(), PopMax() or Push().  See heap.Init()
func Init(h heap.Interface) {
	l := h.Len()
	for i := 0; i < l; i++ {
		bubbleup(h, isMinHeap(i), i)
	}
}
