//go:build !go1.20

package quickxorhash

func xorBytes(dst, src []byte) int {
	n := len(dst)
	if len(src) < n {
		n = len(src)
	}
	if n == 0 {
		return 0
	}
	dst = dst[:n]
	//src = src[:n]
	src = src[:len(dst)] // remove bounds check in loop
	for i := range dst {
		dst[i] ^= src[i]
	}
	return n
}
