// Run fuzz tests on the package
//
// This is done with a byte heap to test and a simple reimplementation
// to check correctness against.
//
// First install go-fuzz
//
//     go get -u github.com/dvyukov/go-fuzz/go-fuzz github.com/dvyukov/go-fuzz/go-fuzz-build
//
// Next build the instrumented package
//
//     go-fuzz-build
//
// Finally fuzz away
//
//     go-fuzz
//
// See https://github.com/dvyukov/go-fuzz for more instructions

//+build gofuzz

package deheap

import (
	"fmt"
	"sort"
)

// An byteHeap is a double ended heap of bytes
type byteDeheap []byte

func (h byteDeheap) Len() int           { return len(h) }
func (h byteDeheap) Less(i, j int) bool { return h[i] < h[j] }
func (h byteDeheap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *byteDeheap) Push(x interface{}) {
	*h = append(*h, x.(byte))
}

func (h *byteDeheap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// sortedHeap is an inefficient reimplementation for test purposes
type sortedHeap []byte

func (h *sortedHeap) Push(x byte) {
	data := *h
	i := sort.Search(len(data), func(i int) bool { return data[i] >= x })
	// i is the either the position of x or where it should be inserted
	data = append(data, 0)
	copy(data[i+1:], data[i:])
	data[i] = x
	*h = data
}

func (h *sortedHeap) Pop() (x byte) {
	data := *h
	x = data[0]
	*h = data[1:]
	return x
}

func (h *sortedHeap) PopMax() (x byte) {
	data := *h
	x = data[len(data)-1]
	*h = data[:len(data)-1]
	return x
}

// Fuzzer input is a string of bytes.
//
// If the byte is one of these, then the action is performed
//   '<' Pop (minimum)
//   '>' PopMax
// Otherwise the bytes is Pushed onto the heap
func Fuzz(data []byte) int {
	h := &byteDeheap{}
	Init(h)
	s := sortedHeap{}

	for _, c := range data {
		switch c {
		case '<':
			if h.Len() > 0 {
				got := Pop(h)
				want := s.Pop()
				if got != want {
					panic(fmt.Sprintf("Pop: want = %d, got = %d", want, got))
				}
			}
		case '>':
			if h.Len() > 0 {
				got := PopMax(h)
				want := s.PopMax()
				if got != want {
					panic(fmt.Sprintf("PopMax: want = %d, got = %d", want, got))
				}
			}
		default:
			Push(h, c)
			s.Push(c)
		}
		if len(s) != h.Len() {
			panic("wrong length")
		}
	}
	return 1
}
