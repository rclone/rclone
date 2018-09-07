//+build go1.9

package fstests

import (
	"math/bits"
)

func leadingZeros64(x uint64) int {
	return bits.LeadingZeros64(x)
}
