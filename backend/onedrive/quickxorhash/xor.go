//go:build !1.20 && !1.21 && !1.22

package quickxorhash

import "unsafe"

const wordSize = unsafe.Sizeof(uintptr(0))

// words returns a []uintptr pointing at the same data as x,
// with any trailing partial word removed.
func words(x []byte) []uintptr {
	return unsafe.Slice((*uintptr)(unsafe.Pointer(&x[0])), uintptr(len(x))/wordSize)
}

func xorLoop(dst, src []uintptr) {
	src = src[:len(dst)] // remove bounds check in loop
	for i := range dst {
		dst[i] ^= src[i]
	}
}

func xorLoopBytes(dst, src []byte) {
	src = src[:len(dst)] // remove bounds check in loop
	for i := range dst {
		dst[i] ^= src[i]
	}
}

func xorBytes(dst, src []byte) int {
	n := len(dst)
	if len(src) < n {
		n = len(src)
	}
	if n == 0 {
		return 0
	}
	dst = dst[:n]
	src = src[:n]
	xorLoop(words(dst), words(src))
	if uintptr(n)%wordSize == 0 {
		return n
	}
	done := n &^ int(wordSize-1)
	dst = dst[done:]
	src = src[done:]
	xorLoopBytes(dst, src)

	return n
}
