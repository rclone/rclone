package saferith

import "math/bits"

// _WShift can be used to multiply or divide by _W
//
// This assumes that _W = 64, 32
const _WShift = 5 + (_W >> 6)
const _WMask = _W - 1

// limbCount returns the number of limbs needed to accomodate bits.
func limbCount(bits int) int {
	return (bits + _W - 1) >> _WShift
}

// limbMask returns the mask used for the final limb of a Nat with this number of bits.
//
// Note that this function will leak the number of bits. For our library, this isn't
// a problem, since we always call this function with announced sizes.
func limbMask(bits int) Word {
	remaining := bits & _WMask
	allOnes := ^Word(0)
	if remaining == 0 {
		return allOnes
	}
	return ^(allOnes << remaining)
}

// leadingZeros calculates the number of leading zero bits in x.
//
// This shouldn't leak any information about the value of x.
func leadingZeros(x Word) int {
	// Go will replace this call with the appropriate instruction on amd64 and arm64.
	//
	// Unfortunately, the fallback function is not constant-time, but the platforms
	// for which there is no fallback aren't all that common anyways.
	return bits.LeadingZeros(uint(x))
}
