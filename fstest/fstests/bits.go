//+build !go1.9

package fstests

func leadingZeros64(x uint64) int {
	var n uint64 = 64

	if y := x >> 32; y != 0 {
		n = n - 32
		x = y
	}
	if y := x >> 16; y != 0 {
		n = n - 16
		x = y
	}
	if y := x >> 8; y != 0 {
		n = n - 8
		x = y
	}
	if y := x >> 4; y != 0 {
		n = n - 4
		x = y
	}
	if y := x >> 2; y != 0 {
		n = n - 2
		x = y
	}
	if y := x >> 1; y != 0 {
		return int(n - 2)
	}

	return int(n - x)
}
