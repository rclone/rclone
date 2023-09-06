package saferith

import (
	"fmt"
	"math/big"
	"math/bits"
	"strings"
)

// General utilities

// add calculates a + b + carry, returning the sum, and carry
//
// This is a convenient wrapper around bits.Add, and should be optimized
// by the compiler to produce a single ADC instruction.
func add(a, b, carry Word) (sum Word, newCarry Word) {
	s, c := bits.Add(uint(a), uint(b), uint(carry))
	return Word(s), Word(c)
}

// Constant Time Utilities

// Choice represents a constant-time boolean.
//
// The value of Choice is always either 1 or 0.
//
// We use a separate type instead of bool, in order to be able to make decisions without leaking
// which decision was made.
//
// You can easily convert a Choice into a bool with the operation c == 1.
//
// In general, logical operations on bool become bitwise operations on choice:
//     a && b => a & b
//     a || b => a | b
//     a != b => a ^ b
//     !a     => 1 ^ a
type Choice Word

// ctEq compares x and y for equality, returning 1 if equal, and 0 otherwise
//
// This doesn't leak any information about either of them
func ctEq(x, y Word) Choice {
	// If x == y, then x ^ y should be all zero bits.
	q := uint(x ^ y)
	// For any q != 0, either the MSB of q, or the MSB of -q is 1.
	// We can thus or those together, and check the top bit. When q is zero,
	// that means that x and y are equal, so we negate that top bit.
	return 1 ^ Choice((q|-q)>>(_W-1))
}

// ctGt checks x > y, returning 1 or 0
//
// This doesn't leak any information about either of them
func ctGt(x, y Word) Choice {
	_, b := bits.Sub(uint(y), uint(x), 0)
	return Choice(b)
}

// ctIfElse selects x if v = 1, and y otherwise
//
// This doesn't leak the value of any of its inputs
func ctIfElse(v Choice, x, y Word) Word {
	// mask should be all 1s if v is 1, otherwise all 0s
	mask := -Word(v)
	return y ^ (mask & (y ^ x))
}

// ctCondCopy copies y into x, if v == 1, otherwise does nothing
//
// Both slices must have the same length.
//
// LEAK: the length of the slices
//
// Otherwise, which branch was taken isn't leaked
func ctCondCopy(v Choice, x, y []Word) {
	if len(x) != len(y) {
		panic("ctCondCopy: mismatched arguments")
	}
	for i := 0; i < len(x); i++ {
		x[i] = ctIfElse(v, y[i], x[i])
	}
}

// ctCondSwap swaps the contents of a and b, when v == 1, otherwise does nothing
//
// Both slices must have the same length.
//
// LEAK: the length of the slices
//
// Whether or not a swap happened isn't leaked
func ctCondSwap(v Choice, a, b []Word) {
	for i := 0; i < len(a) && i < len(b); i++ {
		ai := a[i]
		a[i] = ctIfElse(v, b[i], ai)
		b[i] = ctIfElse(v, ai, b[i])
	}
}

// CondAssign sets z <- yes ? x : z.
//
// This function doesn't leak any information about whether the assignment happened.
//
// The announced size of the result will be the largest size between z and x.
func (z *Nat) CondAssign(yes Choice, x *Nat) *Nat {
	maxBits := z.maxAnnounced(x)

	xLimbs := x.resizedLimbs(maxBits)
	z.limbs = z.resizedLimbs(maxBits)

	ctCondCopy(yes, z.limbs, xLimbs)

	// If the value we're potentially assigning has a different reduction,
	// then there's nothing we can conclude about the resulting reduction.
	if z.reduced != x.reduced {
		z.reduced = nil
	}
	z.announced = maxBits

	return z
}

// "Missing" Functions
// These are routines that could in theory be implemented in assembly,
// but aren't already present in Go's big number routines

// div calculates the quotient and remainder of hi:lo / d
//
// Unlike bits.Div, this doesn't leak anything about the inputs
func div(hi, lo, d Word) (Word, Word) {
	var quo Word
	hi = ctIfElse(ctEq(hi, d), 0, hi)
	for i := _W - 1; i > 0; i-- {
		j := _W - i
		w := (hi << j) | (lo >> i)
		sel := ctEq(w, d) | ctGt(w, d) | Choice(hi>>i)
		hi2 := (w - d) >> j
		lo2 := lo - (d << i)
		hi = ctIfElse(sel, hi2, hi)
		lo = ctIfElse(sel, lo2, lo)
		quo |= Word(sel)
		quo <<= 1
	}
	sel := ctEq(lo, d) | ctGt(lo, d) | Choice(hi)
	quo |= Word(sel)
	rem := ctIfElse(sel, lo-d, lo)
	return quo, rem
}

// mulSubVVW calculates z -= y * x
//
// This also results in a carry.
func mulSubVVW(z, x []Word, y Word) (c Word) {
	for i := 0; i < len(z) && i < len(x); i++ {
		hi, lo := mulAddWWW_g(x[i], y, c)
		sub, cc := bits.Sub(uint(z[i]), uint(lo), 0)
		c, z[i] = Word(cc), Word(sub)
		c += hi
	}
	return
}

// Nat represents an arbitrary sized natural number.
//
// Different methods on Nats will talk about a "capacity". The capacity represents
// the announced size of some number. Operations may vary in time *only* relative
// to this capacity, and not to the actual value of the number.
//
// The capacity of a number is usually inherited through whatever method was used to
// create the number in the first place.
type Nat struct {
	// The exact number of bits this number claims to have.
	//
	// This can differ from the actual number of bits needed to represent this number.
	announced int
	// If this is set, then the value of this Nat is in the range 0..reduced - 1.
	//
	// This value should get set based only on statically knowable things, like what
	// functions have been called. This means that we will have plenty of false
	// negatives, where a value is small enough, but we don't know statically
	// that this is the case.
	//
	// Invariant: If reduced is set, then announced should match the announced size of
	// this modulus.
	reduced *Modulus
	// The limbs representing this number, in little endian order.
	//
	// Invariant: The bits past announced will not be set. This includes when announced
	// isn't a multiple of the limb size.
	//
	// Invariant: two Nats are not allowed to share the same slice.
	// This allows us to use pointer comparison to check that Nats don't alias eachother
	limbs []Word
}

// checkInvariants does some internal sanity checks.
//
// This is useful for tests.
func (z *Nat) checkInvariants() bool {
	if z.reduced != nil && z.announced != z.reduced.nat.announced {
		return false
	}
	if len(z.limbs) != limbCount(z.announced) {
		return false
	}
	if len(z.limbs) > 0 {
		lastLimb := z.limbs[len(z.limbs)-1]
		if lastLimb != lastLimb&limbMask(z.announced) {
			return false
		}
	}
	return true
}

// maxAnnounced returns the larger announced length of z and y
func (z *Nat) maxAnnounced(y *Nat) int {
	maxBits := z.announced
	if y.announced > maxBits {
		maxBits = y.announced
	}
	return maxBits
}

// ensureLimbCapacity makes sure that a Nat has capacity for a certain number of limbs
//
// This will modify the slice contained inside the natural, but won't change the size of
// the slice, so it doesn't affect the value of the natural.
//
// LEAK: Probably the current number of limbs, and size
// OK: both of these should be public
func (z *Nat) ensureLimbCapacity(size int) {
	if cap(z.limbs) < size {
		newLimbs := make([]Word, len(z.limbs), size)
		copy(newLimbs, z.limbs)
		z.limbs = newLimbs
	}
}

// resizedLimbs returns a new slice of limbs accomodating a number of bits.
//
// This will clear out the end of the slice as necessary.
//
// LEAK: the current number of limbs, and bits
// OK: both are public
func (z *Nat) resizedLimbs(bits int) []Word {
	size := limbCount(bits)
	z.ensureLimbCapacity(size)
	res := z.limbs[:size]
	// Make sure that the expansion (if any) is cleared
	for i := len(z.limbs); i < size; i++ {
		res[i] = 0
	}
	maskEnd(res, bits)
	return res
}

// maskEnd applies the correct bit mask to some limbs
func maskEnd(limbs []Word, bits int) {
	if len(limbs) <= 0 {
		return
	}
	limbs[len(limbs)-1] &= limbMask(bits)
}

// unaliasedLimbs returns a set of limbs for z, such that they do not alias those of x
//
// This will create a copy of the limbs, if necessary.
//
// LEAK: the size of z, whether or not z and x are the same Nat
func (z *Nat) unaliasedLimbs(x *Nat) []Word {
	res := z.limbs
	if z == x {
		res = make([]Word, len(z.limbs))
		copy(res, z.limbs)
	}
	return res
}

// trueSize calculates the actual size necessary for representing these limbs
//
// This is the size with leading zeros removed. This leaks the number
// of such zeros, but nothing else.
func trueSize(limbs []Word) int {
	// Instead of checking == 0 directly, which may leak the value, we instead
	// compare with zero in constant time, and check if that succeeded in a leaky way.
	var size int
	for size = len(limbs); size > 0 && ctEq(limbs[size-1], 0) == 1; size-- {
	}
	return size
}

// AnnouncedLen returns the number of bits this number is publicly known to have
func (z *Nat) AnnouncedLen() int {
	return z.announced
}

// TrueLen calculates the exact number of bits needed to represent z
//
// This function violates the standard contract around Nats and announced length.
// For most purposes, `AnnouncedLen` should be used instead.
//
// That being said, this function does try to limit its leakage, and should
// only leak the number of leading zero bits in the number.
func (z *Nat) TrueLen() int {
	limbSize := trueSize(z.limbs)
	size := limbSize * _W
	if limbSize > 0 {
		size -= leadingZeros(z.limbs[limbSize-1])
	}
	return size
}

// FillBytes writes out the big endian bytes of a natural number.
//
// This will always write out the full capacity of the number, without
// any kind trimming.
func (z *Nat) FillBytes(buf []byte) []byte {
	for i := 0; i < len(buf); i++ {
		buf[i] = 0
	}

	i := len(buf)
	// LEAK: Number of limbs
	// OK: The number of limbs is public
	// LEAK: The addresses touched in the out array
	// OK: Every member of out is touched
Outer:
	for _, x := range z.limbs {
		y := x
		for j := 0; j < _S; j++ {
			i--
			if i < 0 {
				break Outer
			}
			buf[i] = byte(y)
			y >>= 8
		}
	}
	return buf
}

// SetBytes interprets a number in big-endian format, stores it in z, and returns z.
//
// The exact length of the buffer must be public information! This length also dictates
// the capacity of the number returned, and thus the resulting timings for operations
// involving that number.
func (z *Nat) SetBytes(buf []byte) *Nat {
	z.reduced = nil
	z.announced = 8 * len(buf)
	z.limbs = z.resizedLimbs(z.announced)
	bufI := len(buf) - 1
	for i := 0; i < len(z.limbs) && bufI >= 0; i++ {
		z.limbs[i] = 0
		for shift := 0; shift < _W && bufI >= 0; shift += 8 {
			z.limbs[i] |= Word(buf[bufI]) << shift
			bufI--
		}
	}
	return z
}

// Bytes creates a slice containing the contents of this Nat, in big endian
//
// This will always fill the output byte slice based on the announced length of this Nat.
func (z *Nat) Bytes() []byte {
	length := (z.announced + 7) / 8
	out := make([]byte, length)
	return z.FillBytes(out)
}

// MarshalBinary implements encoding.BinaryMarshaler.
// Returns the same value as Bytes().
func (i *Nat) MarshalBinary() ([]byte, error) {
	return i.Bytes(), nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
// Wraps SetBytes
func (i *Nat) UnmarshalBinary(data []byte) error {
	i.SetBytes(data)
	return nil
}

// convert a 4 bit value into an ASCII value in constant time
func nibbletoASCII(nibble byte) byte {
	w := Word(nibble)
	value := ctIfElse(ctGt(w, 9), w-0xA+Word('A'), w+Word('0'))
	return byte(value)
}

// convert an ASCII value into a 4 bit value, returning whether or not this value is valid.
func nibbleFromASCII(ascii byte) (byte, Choice) {
	w := Word(ascii)
	inFirstRange := ctGt(w, Word('0')-1) & (1 ^ ctGt(w, Word('9')))
	inSecondRange := ctGt(w, Word('A')-1) & (1 ^ ctGt(w, Word('F')))
	valid := inFirstRange | inSecondRange
	nibble := ctIfElse(inFirstRange, w-Word('0'), w-Word('A')+0xA)
	return byte(nibble), valid
}

// SetHex modifies the value of z to hold a hex string, returning z
//
// The hex string must be in big endian order. If it contains characters
// other than 0..9, A..F, the value of z will be undefined, and an error will
// be returned.
//
// The value of the string shouldn't be leaked, except in the case where the string
// contains invalid characters.
func (z *Nat) SetHex(hex string) (*Nat, error) {
	z.reduced = nil
	z.announced = 4 * len(hex)
	z.limbs = z.resizedLimbs(z.announced)
	hexI := len(hex) - 1
	for i := 0; i < len(z.limbs) && hexI >= 0; i++ {
		z.limbs[i] = 0
		for shift := 0; shift < _W && hexI >= 0; shift += 4 {
			nibble, valid := nibbleFromASCII(byte(hex[hexI]))
			if valid != 1 {
				return nil, fmt.Errorf("invalid hex character: %c", hex[hexI])
			}
			z.limbs[i] |= Word(nibble) << shift
			hexI--
		}
	}
	return z, nil
}

// Hex converts this number into a hexadecimal string.
//
// This string will be a multiple of 8 bits.
//
// This shouldn't leak any information about the value of this Nat, only its length.
func (z *Nat) Hex() string {
	bytes := z.Bytes()
	var builder strings.Builder
	for _, b := range bytes {
		_ = builder.WriteByte(nibbletoASCII((b >> 4) & 0xF))
		_ = builder.WriteByte(nibbletoASCII(b & 0xF))
	}
	return builder.String()
}

// the number of bytes to print in the string representation before an underscore
const underscoreAfterNBytes = 4

// String will represent this nat as a convenient Hex string
//
// This shouldn't leak any information about the value of this Nat, only its length.
func (z *Nat) String() string {
	bytes := z.Bytes()
	var builder strings.Builder
	_, _ = builder.WriteString("0x")
	i := 0
	for _, b := range bytes {
		if i == underscoreAfterNBytes {
			builder.WriteRune('_')
			i = 0
		}
		builder.WriteByte(nibbletoASCII((b >> 4) & 0xF))
		builder.WriteByte(nibbletoASCII(b & 0xF))
		i += 1

	}
	return builder.String()
}

// Byte will access the ith byte in this nat, with 0 being the least significant byte.
//
// This will leak the value of i, and panic if i is < 0.
func (z *Nat) Byte(i int) byte {
	if i < 0 {
		panic("negative byte")
	}
	limbCount := len(z.limbs)
	bytesPerLimb := _W / 8
	if i >= bytesPerLimb*limbCount {
		return 0
	}
	return byte(z.limbs[i/bytesPerLimb] >> (8 * (i % bytesPerLimb)))
}

// Big converts a Nat into a big.Int
//
// This will leak information about the true size of z, so caution
// should be exercised when using this method with sensitive values.
func (z *Nat) Big() *big.Int {
	res := new(big.Int)
	// Unfortunate that there's no good way to handle this
	bigLimbs := make([]big.Word, len(z.limbs))
	for i := 0; i < len(bigLimbs) && i < len(z.limbs); i++ {
		bigLimbs[i] = big.Word(z.limbs[i])
	}
	res.SetBits(bigLimbs)
	return res
}

// SetBig modifies z to contain the value of x
//
// The size parameter is used to pad or truncate z to a certain number of bits.
func (z *Nat) SetBig(x *big.Int, size int) *Nat {
	z.announced = size
	z.limbs = z.resizedLimbs(size)
	bigLimbs := x.Bits()
	for i := 0; i < len(z.limbs) && i < len(bigLimbs); i++ {
		z.limbs[i] = Word(bigLimbs[i])
	}
	maskEnd(z.limbs, size)
	return z
}

// SetUint64 sets z to x, and returns z
//
// This will have the exact same capacity as a 64 bit number
func (z *Nat) SetUint64(x uint64) *Nat {
	z.reduced = nil
	z.announced = 64
	z.limbs = z.resizedLimbs(z.announced)
	for i := 0; i < len(z.limbs); i++ {
		z.limbs[i] = Word(x)
		x >>= _W
	}
	return z
}

// Uint64 represents this number as uint64
//
// The behavior of this function is undefined if the announced length of z is > 64.
func (z *Nat) Uint64() uint64 {
	var ret uint64
	for i := len(z.limbs) - 1; i >= 0; i-- {
		ret = (ret << _W) | uint64(z.limbs[i])
	}
	return ret
}

// SetNat copies the value of x into z
//
// z will have the same announced length as x.
func (z *Nat) SetNat(x *Nat) *Nat {
	z.limbs = z.resizedLimbs(x.announced)
	copy(z.limbs, x.limbs)
	z.reduced = x.reduced
	z.announced = x.announced
	return z
}

// Clone returns a copy of this value.
//
// This copy can safely be mutated without affecting the original.
func (z *Nat) Clone() *Nat {
	return new(Nat).SetNat(z)
}

// Resize resizes z to a certain number of bits, returning z.
func (z *Nat) Resize(cap int) *Nat {
	z.limbs = z.resizedLimbs(cap)
	z.announced = cap
	return z
}

// Modulus represents a natural number used for modular reduction
//
// Unlike with natural numbers, the number of bits need to contain the modulus
// is assumed to be public. Operations are allowed to leak this size, and creating
// a modulus will remove unnecessary zeros.
//
// Operations on a Modulus may leak whether or not a Modulus is even.
type Modulus struct {
	nat Nat
	// the number of leading zero bits
	leading int
	// The inverse of the least significant limb, modulo W
	m0inv Word
	// If true, then this modulus is even
	even bool
}

// invertModW calculates x^-1 mod _W
func invertModW(x Word) Word {
	y := x
	// This is enough for 64 bits, and the extra iteration is not that costly for 32
	for i := 0; i < 5; i++ {
		y = y * (2 - x*y)
	}
	return y
}

// precomputeValues calculates the desirable modulus fields in advance
//
// This sets the leading number of bits, leaking the true bit size of m,
// as well as the inverse of the least significant limb (without leaking it).
//
// This will also do integrity checks, namely that the modulus isn't empty or even
func (m *Modulus) precomputeValues() {
	announced := m.nat.TrueLen()
	m.nat.announced = announced
	m.nat.limbs = m.nat.resizedLimbs(announced)
	if len(m.nat.limbs) < 1 {
		panic("Modulus is empty")
	}
	m.leading = leadingZeros(m.nat.limbs[len(m.nat.limbs)-1])
	// I think checking the bit directly might leak more data than we'd like
	m.even = ctEq(m.nat.limbs[0]&1, 0) == 1
	// There's no point calculating this if m isn't even, and we can leak evenness
	if !m.even {
		m.m0inv = invertModW(m.nat.limbs[0])
		m.m0inv = -m.m0inv
	}
}

// ModulusFromUint64 sets the modulus according to an integer
func ModulusFromUint64(x uint64) *Modulus {
	var m Modulus
	m.nat.SetUint64(x)
	m.precomputeValues()
	return &m
}

// ModulusFromBytes creates a new Modulus, converting from big endian bytes
//
// This function will remove leading zeros, thus leaking the true size of the modulus.
// See the documentation for the Modulus type, for more information about this contract.
func ModulusFromBytes(bytes []byte) *Modulus {
	var m Modulus
	// TODO: You could allocate a smaller buffer to begin with, versus using the Nat method
	m.nat.SetBytes(bytes)
	m.precomputeValues()
	return &m
}

// ModulusFromHex creates a new modulus from a hex string.
//
// The same rules as Nat.SetHex apply.
//
// Additionally, this function will remove leading zeros, leaking the true size of the modulus.
// See the documentation for the Modulus type, for more information about this contract.
func ModulusFromHex(hex string) (*Modulus, error) {
	var m Modulus
	_, err := m.nat.SetHex(hex)
	if err != nil {
		return nil, err
	}
	m.precomputeValues()
	return &m, nil
}

// FromNat creates a new Modulus, using the value of a Nat
//
// This will leak the true size of this natural number. Because of this,
// the true size of the number should not be sensitive information. This is
// a stronger requirement than we usually have for Nat.
func ModulusFromNat(nat *Nat) *Modulus {
	var m Modulus
	m.nat.SetNat(nat)
	m.precomputeValues()
	return &m
}

// Nat returns the value of this modulus as a Nat.
//
// This will create a copy of this modulus value, so the Nat can be safely
// mutated.
func (m *Modulus) Nat() *Nat {
	return new(Nat).SetNat(&m.nat)
}

// Bytes returns the big endian bytes making up the modulus
func (m *Modulus) Bytes() []byte {
	return m.nat.Bytes()
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (i *Modulus) MarshalBinary() ([]byte, error) {
	return i.nat.Bytes(), nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (i *Modulus) UnmarshalBinary(data []byte) error {
	i.nat.SetBytes(data)
	i.precomputeValues()
	return nil
}

// Big returns the value of this Modulus as a big.Int
func (m *Modulus) Big() *big.Int {
	return m.nat.Big()
}

// Hex will represent this Modulus as a Hex string.
//
// The hex string will hold a multiple of 8 bits.
//
// This shouldn't leak any information about the value of the modulus, beyond
// the usual leakage around its size.
func (m *Modulus) Hex() string {
	return m.nat.Hex()
}

// String will represent this Modulus as a convenient Hex string
//
// This shouldn't leak any information about the value of the modulus, only its length.
func (m *Modulus) String() string {
	return m.nat.String()
}

// BitLen returns the exact number of bits used to store this Modulus
//
// Moduli are allowed to leak this value.
func (m *Modulus) BitLen() int {
	return m.nat.announced
}

// Cmp compares two moduli, returning results for (>, =, <).
//
// This will not leak information about the value of these relations, or the moduli.
func (m *Modulus) Cmp(n *Modulus) (Choice, Choice, Choice) {
	return m.nat.Cmp(&n.nat)
}

// shiftAddInCommon exists to unify behavior between shiftAddIn and shiftAddInGeneric
//
// z, scratch, and m should have the same length.
//
// The two functions differ only in how the calculate a1:a0, and b0.
//
// hi should be what was previously the top limb of z.
//
// a1:a0 and b0 should be the most significant two limbs of z, and single limb of m,
// after shifting to discard leading zeros.
//
// The way these are calculated differs between the two versions of shiftAddIn,
// which is why this function exists.
func shiftAddInCommon(z, scratch, m []Word, hi, a1, a0, b0 Word) (q Word) {
	// We want to use a1:a0 / b0 - 1 as our estimate. If rawQ is 0, we should
	// use 0 as our estimate. Another edge case when an overflow happens in the quotient.
	// It can be shown that this happens when a1 == b0. In this case, we want
	// to use the maximum value for q
	rawQ, _ := div(a1, a0, b0)
	q = ctIfElse(ctEq(a1, b0), ^Word(0), ctIfElse(ctEq(rawQ, 0), 0, rawQ-1))

	// This estimate is off by +- 1, so we subtract q * m, and then either add
	// or subtract m, based on the result.
	c := mulSubVVW(z, m, q)
	// If the carry from subtraction is greater than the limb of z we've shifted out,
	// then we've underflowed, and need to add in m
	under := ctGt(c, hi)
	// For us to be too large, we first need to not be too low, as per the previous flag.
	// Then, if the lower limbs of z are still larger, or the top limb of z is equal to the carry,
	// we can conclude that we're too large, and need to subtract m
	stillBigger := cmpGeq(z, m)
	over := (1 ^ under) & (stillBigger | (1 ^ ctEq(c, hi)))
	addVV(scratch, z, m)
	ctCondCopy(under, z, scratch)
	q -= Word(under)
	subVV(scratch, z, m)
	ctCondCopy(over, z, scratch)
	q += Word(over)
	return
}

// shiftAddIn calculates z = z << _W + x mod m
//
// The length of z and scratch should be len(m)
func shiftAddIn(z, scratch []Word, x Word, m *Modulus) (q Word) {
	// Making tests on the exact bit length of m is ok,
	// since that's part of the contract for moduli
	size := len(m.nat.limbs)
	if size == 0 {
		return
	}
	if size == 1 {
		// In this case, z:x (/, %) m is exactly what we need to calculate
		q, r := div(z[0], x, m.nat.limbs[0])
		z[0] = r
		return q
	}

	// The idea is as follows:
	//
	// We want to shift x into z, and then divide by m. Instead of dividing by
	// m, we can get a good estimate, using the top two 2 * _W bits of z, and the
	// top _W bits of m. These are stored in a1:a0, and b0 respectively.

	// We need to keep around the top word of z, pre-shifting
	hi := z[size-1]

	a1 := (z[size-1] << m.leading) | (z[size-2] >> (_W - m.leading))
	// The actual shift can be performed by moving the limbs of z up, then inserting x
	for i := size - 1; i > 0; i-- {
		z[i] = z[i-1]
	}
	z[0] = x
	a0 := (z[size-1] << m.leading) | (z[size-2] >> (_W - m.leading))
	b0 := (m.nat.limbs[size-1] << m.leading) | (m.nat.limbs[size-2] >> (_W - m.leading))

	return shiftAddInCommon(z, scratch, m.nat.limbs, hi, a1, a0, b0)
}

// shiftAddInGeneric is like shiftAddIn, but works with arbitrary m.
//
// See shiftAddIn for what this function is trying to accomplish, and what the
// inputs represent.
//
// The big difference this entails is that z and m may have padding limbs, so
// we have to do a bit more work to recover their significant bits in constant-time.
func shiftAddInGeneric(z, scratch []Word, x Word, m []Word) Word {
	size := len(m)
	if size == 0 {
		return 0
	}
	if size == 1 {
		// In this case, z:x (/, %) m is exactly what we need to calculate
		q, r := div(z[0], x, m[0])
		z[0] = r
		return q
	}

	// We need to get match the two most significant 2 * _W bits of z with the most significant
	// _W bits of m. We also need to eliminate any leading zeros, possibly fetching a
	// these bits over multiple limbs. Because of this, we need to scan over both
	// arrays, with a window of 3 limbs for z, and 2 limbs for m, until we hit the
	// first non-zero limb for either of them. Because z < m, it suffices to check
	// for a non-zero limb from m.
	var a2, a1, a0, b1, b0 Word
	done := Choice(0)
	for i := size - 1; i > 1; i-- {
		a2 = ctIfElse(done, a2, z[i])
		a1 = ctIfElse(done, a1, z[i-1])
		a0 = ctIfElse(done, a0, z[i-2])
		b1 = ctIfElse(done, b1, m[i])
		b0 = ctIfElse(done, b0, m[i-1])
		done = 1 ^ ctEq(b1, 0)
	}
	// We also need to do one more iteration to potentially include x inside of our
	// significant bits from z.
	a2 = ctIfElse(done, a2, z[1])
	a1 = ctIfElse(done, a1, z[0])
	a0 = ctIfElse(done, a0, x)
	b1 = ctIfElse(done, b1, m[1])
	b0 = ctIfElse(done, b0, m[0])
	// Now, we need to shift away the leading zeros to get the most significant bits.
	// Converting to Word avoids a panic check
	l := Word(leadingZeros(b1))
	a2 = (a2 << l) | (a1 >> (_W - l))
	a1 = (a1 << l) | (a0 >> (_W - l))
	b1 = (b1 << l) | (b0 >> (_W - l))

	// Another adjustment we need to make before calling the next function is to actually
	// insert x inside of z, shifting out hi.
	hi := z[len(z)-1]
	for i := size - 1; i > 0; i-- {
		z[i] = z[i-1]
	}
	z[0] = x

	return shiftAddInCommon(z, scratch, m, hi, a2, a1, b1)
}

// Mod calculates z <- x mod m
//
// The capacity of the resulting number matches the capacity of the modulus.
func (z *Nat) Mod(x *Nat, m *Modulus) *Nat {
	if x.reduced == m {
		z.SetNat(x)
		return z
	}
	size := len(m.nat.limbs)
	xLimbs := x.unaliasedLimbs(z)
	z.limbs = z.resizedLimbs(2 * _W * size)
	for i := 0; i < len(z.limbs); i++ {
		z.limbs[i] = 0
	}
	// Multiple times in this section:
	// LEAK: the length of x
	// OK: this is public information
	i := len(xLimbs) - 1
	// We can inject at least size - 1 limbs while staying under m
	// Thus, we start injecting from index size - 2
	start := size - 2
	// That is, if there are at least that many limbs to choose from
	if i < start {
		start = i
	}
	for j := start; j >= 0; j-- {
		z.limbs[j] = xLimbs[i]
		i--
	}
	// We shift in the remaining limbs, making sure to reduce modulo M each time
	for ; i >= 0; i-- {
		shiftAddIn(z.limbs[:size], z.limbs[size:], xLimbs[i], m)
	}
	z.limbs = z.resizedLimbs(m.nat.announced)
	z.announced = m.nat.announced
	z.reduced = m
	return z
}

// Div calculates z <- x / m, with m a Modulus.
//
// This might seem like an odd signature, but by using a Modulus,
// we can achieve the same speed as the Mod method. This wouldn't be the case for
// an arbitrary Nat.
//
// cap determines the number of bits to keep in the result. If cap < 0, then
// the number of bits will be x.AnnouncedLen() - m.BitLen() + 2
func (z *Nat) Div(x *Nat, m *Modulus, cap int) *Nat {
	if cap < 0 {
		cap = x.announced - m.nat.announced + 2
	}
	if len(x.limbs) < len(m.nat.limbs) || x.reduced == m {
		z.limbs = z.resizedLimbs(cap)
		for i := 0; i < len(z.limbs); i++ {
			z.limbs[i] = 0
		}
		z.announced = cap
		z.reduced = nil
		return z
	}

	size := limbCount(m.nat.announced)

	xLimbs := x.unaliasedLimbs(z)

	// Enough for 2 buffers the size of m, and to store the full quotient
	startSize := limbCount(cap)
	if startSize < 2*size {
		startSize = 2 * size
	}
	z.limbs = z.resizedLimbs(_W * (startSize + len(xLimbs)))

	remainder := z.limbs[:size]
	for i := 0; i < len(remainder); i++ {
		remainder[i] = 0
	}
	scratch := z.limbs[size : 2*size]
	// Our full quotient, in big endian order.
	quotientBE := z.limbs[startSize:]
	// We use this to append without actually reallocating. We fill our quotient
	// in from 0 upwards.
	qI := 0

	i := len(xLimbs) - 1
	// We can inject at least size - 1 limbs while staying under m
	// Thus, we start injecting from index size - 2
	start := size - 2
	// That is, if there are at least that many limbs to choose from
	if i < start {
		start = i
	}
	for j := start; j >= 0; j-- {
		remainder[j] = xLimbs[i]
		i--
		quotientBE[qI] = 0
		qI++
	}

	for ; i >= 0; i-- {
		q := shiftAddIn(remainder, scratch, xLimbs[i], m)
		quotientBE[qI] = q
		qI++
	}
	z.limbs = z.resizedLimbs(cap)
	// First, reverse all the limbs we want, from the last part of the buffer we used.
	for i := 0; i < len(z.limbs) && i < len(quotientBE); i++ {
		z.limbs[i] = quotientBE[qI-i-1]
	}
	maskEnd(z.limbs, cap)
	z.reduced = nil
	z.announced = cap
	return z
}

// ModAdd calculates z <- x + y mod m
//
// The capacity of the resulting number matches the capacity of the modulus.
func (z *Nat) ModAdd(x *Nat, y *Nat, m *Modulus) *Nat {
	var xModM, yModM Nat
	// This is necessary for the correctness of the algorithm, since
	// we don't assume that x and y are in range.
	// Furthermore, we can now assume that x and y have the same number
	// of limbs as m
	xModM.Mod(x, m)
	yModM.Mod(y, m)

	// The only thing we have to resize is z, everything else has m's length
	size := limbCount(m.nat.announced)
	scratch := z.resizedLimbs(2 * _W * size)
	// This might hold some more bits, but masking isn't necessary, since the
	// result will be < m.
	z.limbs = scratch[:size]
	subResult := scratch[size:]

	addCarry := addVV(z.limbs, xModM.limbs, yModM.limbs)
	subCarry := subVV(subResult, z.limbs, m.nat.limbs)
	// Three cases are possible:
	//
	// addCarry, subCarry = 0 -> subResult
	// 	 we didn't overflow our buffer, but our result was big
	//   enough to subtract m without underflow, so it was larger than m
	// addCarry, subCarry = 1 -> subResult
	//   we overflowed the buffer, and the subtraction of m is correct,
	//   because our result only looks too small because of the missing carry bit
	// addCarry = 0, subCarry = 1 -> addResult
	// 	 we didn't overflow our buffer, and the subtraction of m is wrong,
	//   because our result was already smaller than m
	// The other case is impossible, because it would mean we have a result big
	// enough to both overflow the addition by at least m. But, we made sure that
	// x and y are at most m - 1, so this isn't possible.
	selectSub := ctEq(addCarry, subCarry)
	ctCondCopy(selectSub, z.limbs[:size], subResult)
	z.reduced = m
	z.announced = m.nat.announced
	return z
}

func (z *Nat) ModSub(x *Nat, y *Nat, m *Modulus) *Nat {
	var xModM, yModM Nat
	// First reduce x and y mod m
	xModM.Mod(x, m)
	yModM.Mod(y, m)

	size := len(m.nat.limbs)
	scratch := z.resizedLimbs(_W * 2 * size)
	z.limbs = scratch[:size]
	addResult := scratch[size:]

	subCarry := subVV(z.limbs, xModM.limbs, yModM.limbs)
	underflow := ctEq(subCarry, 1)
	addVV(addResult, z.limbs, m.nat.limbs)
	ctCondCopy(underflow, z.limbs, addResult)
	z.reduced = m
	z.announced = m.nat.announced
	return z
}

// ModNeg calculates z <- -x mod m
func (z *Nat) ModNeg(x *Nat, m *Modulus) *Nat {
	// First reduce x mod m
	z.Mod(x, m)

	size := len(m.nat.limbs)
	scratch := z.resizedLimbs(_W * 2 * size)
	z.limbs = scratch[:size]
	zero := scratch[size:]
	for i := 0; i < len(zero); i++ {
		zero[i] = 0
	}

	borrow := subVV(z.limbs, zero, z.limbs)
	underflow := ctEq(Word(borrow), 1)
	// Add back M if we underflowed
	addVV(zero, z.limbs, m.nat.limbs)
	ctCondCopy(underflow, z.limbs, zero)

	z.reduced = m
	z.announced = m.nat.announced
	return z
}

// Add calculates z <- x + y, modulo 2^cap
//
// The capacity is given in bits, and also controls the size of the result.
//
// If cap < 0, the capacity will be max(x.AnnouncedLen(), y.AnnouncedLen()) + 1
func (z *Nat) Add(x *Nat, y *Nat, cap int) *Nat {
	if cap < 0 {
		cap = x.maxAnnounced(y) + 1
	}
	xLimbs := x.resizedLimbs(cap)
	yLimbs := y.resizedLimbs(cap)
	z.limbs = z.resizedLimbs(cap)
	addVV(z.limbs, xLimbs, yLimbs)
	// Mask off the final bits
	z.limbs = z.resizedLimbs(cap)
	z.announced = cap
	z.reduced = nil
	return z
}

// Sub calculates z <- x - y, modulo 2^cap
//
// The capacity is given in bits, and also controls the size of the result.
//
// If cap < 0, the capacity will be max(x.AnnouncedLen(), y.AnnouncedLen())
func (z *Nat) Sub(x *Nat, y *Nat, cap int) *Nat {
	if cap < 0 {
		cap = x.maxAnnounced(y)
	}
	xLimbs := x.resizedLimbs(cap)
	yLimbs := y.resizedLimbs(cap)
	z.limbs = z.resizedLimbs(cap)
	subVV(z.limbs, xLimbs, yLimbs)
	// Mask off the final bits
	z.limbs = z.resizedLimbs(cap)
	z.announced = cap
	z.reduced = nil
	return z
}

// montgomeryRepresentation calculates zR mod m
func montgomeryRepresentation(z []Word, scratch []Word, m *Modulus) {
	// Our strategy is to shift by W, n times, each time reducing modulo m
	size := len(m.nat.limbs)
	// LEAK: the size of the modulus
	// OK: this is public
	for i := 0; i < size; i++ {
		shiftAddIn(z, scratch, 0, m)
	}
}

// You might have the urge to replace this with []Word, and use the routines
// that already exist for doing operations. This would be a mistake.
// Go doesn't seem to be able to optimize and inline slice operations nearly as
// well as it can for this little type. Attempts to replace this struct with a
// slice were an order of magnitude slower (as per the exponentiation operation)
type triple struct {
	w0 Word
	w1 Word
	w2 Word
}

func (a *triple) add(b triple) {
	w0, c0 := bits.Add(uint(a.w0), uint(b.w0), 0)
	w1, c1 := bits.Add(uint(a.w1), uint(b.w1), c0)
	w2, _ := bits.Add(uint(a.w2), uint(b.w2), c1)
	a.w0 = Word(w0)
	a.w1 = Word(w1)
	a.w2 = Word(w2)
}

func tripleFromMul(a Word, b Word) triple {
	// You might be tempted to use mulWW here, but for some reason, Go cannot
	// figure out how to inline that assembly routine, but using bits.Mul directly
	// gets inlined by the compiler into effectively the same assembly.
	//
	// Beats me.
	w1, w0 := bits.Mul(uint(a), uint(b))
	return triple{w0: Word(w0), w1: Word(w1), w2: 0}
}

// montgomeryMul performs z <- xy / R mod m
//
// LEAK: the size of the modulus
//
// out, x, y must have the same length as the modulus, and be reduced already.
//
// out can alias x and y, but not scratch
func montgomeryMul(x []Word, y []Word, out []Word, scratch []Word, m *Modulus) {
	size := len(m.nat.limbs)

	for i := 0; i < size; i++ {
		scratch[i] = 0
	}
	dh := Word(0)
	for i := 0; i < size; i++ {
		f := (scratch[0] + x[i]*y[0]) * m.m0inv
		var c triple
		for j := 0; j < size; j++ {
			z := triple{w0: scratch[j], w1: 0, w2: 0}
			z.add(tripleFromMul(x[i], y[j]))
			z.add(tripleFromMul(f, m.nat.limbs[j]))
			z.add(c)
			if j > 0 {
				scratch[j-1] = z.w0
			}
			c.w0 = z.w1
			c.w1 = z.w2
		}
		z := triple{w0: dh, w1: 0, w2: 0}
		z.add(c)
		scratch[size-1] = z.w0
		dh = z.w1
	}
	c := subVV(out, scratch, m.nat.limbs)
	ctCondCopy(1^ctEq(dh, c), out, scratch)
}

// ModMul calculates z <- x * y mod m
//
// The capacity of the resulting number matches the capacity of the modulus
func (z *Nat) ModMul(x *Nat, y *Nat, m *Modulus) *Nat {
	xModM := new(Nat).Mod(x, m)
	yModM := new(Nat).Mod(y, m)
	bitLen := m.BitLen()
	z.Mul(xModM, yModM, 2*bitLen)
	return z.Mod(z, m)
}

// Mul calculates z <- x * y, modulo 2^cap
//
// The capacity is given in bits, and also controls the size of the result.
//
// If cap < 0, the capacity will be x.AnnouncedLen() + y.AnnouncedLen()
func (z *Nat) Mul(x *Nat, y *Nat, cap int) *Nat {
	if cap < 0 {
		cap = x.announced + y.announced
	}
	size := limbCount(cap)
	// Since we neex to set z to zero, we have no choice to use a new buffer,
	// because we allow z to alias either of the arguments
	zLimbs := make([]Word, size)
	xLimbs := x.resizedLimbs(cap)
	yLimbs := y.resizedLimbs(cap)
	// LEAK: limbCount
	// OK: the capacity is public, or should be
	for i := 0; i < size; i++ {
		addMulVVW(zLimbs[i:], xLimbs, yLimbs[i])
	}
	z.limbs = zLimbs
	z.limbs = z.resizedLimbs(cap)
	z.announced = cap
	z.reduced = nil
	return z
}

// Rsh calculates z <- x >> shift, producing a certain number of bits
//
// This method will leak the value of shift.
//
// If cap < 0, the number of bits will be x.AnnouncedLen() - shift.
func (z *Nat) Rsh(x *Nat, shift uint, cap int) *Nat {
	if cap < 0 {
		cap = x.announced - int(shift)
		if cap < 0 {
			cap = 0
		}
	}

	zLimbs := z.resizedLimbs(x.announced)
	xLimbs := x.resizedLimbs(x.announced)
	singleShift := shift % _W
	shrVU(zLimbs, xLimbs, singleShift)

	limbShifts := (shift - singleShift) / _W
	if limbShifts > 0 {
		i := 0
		for ; i+int(limbShifts) < len(zLimbs); i++ {
			zLimbs[i] = zLimbs[i+int(limbShifts)]
		}
		for ; i < len(zLimbs); i++ {
			zLimbs[i] = 0
		}
	}

	z.limbs = zLimbs
	z.limbs = z.resizedLimbs(cap)
	z.announced = cap
	z.reduced = nil
	return z
}

// Lsh calculates z <- x << shift, producing a certain number of bits
//
// This method will leak the value of shift.
//
// If cap < 0, the number of bits will be x.AnnouncedLen() + shift.
func (z *Nat) Lsh(x *Nat, shift uint, cap int) *Nat {
	if cap < 0 {
		cap = x.announced + int(shift)
	}
	zLimbs := z.resizedLimbs(cap)
	xLimbs := x.resizedLimbs(cap)
	singleShift := shift % _W
	shlVU(zLimbs, xLimbs, singleShift)

	limbShifts := (shift - singleShift) / _W
	if limbShifts > 0 {
		i := len(zLimbs) - 1
		for ; i-int(limbShifts) >= 0; i-- {
			zLimbs[i] = zLimbs[i-int(limbShifts)]
		}
		for ; i >= 0; i-- {
			zLimbs[i] = 0
		}
	}

	z.limbs = zLimbs
	z.announced = cap
	z.reduced = nil
	return z
}

func (z *Nat) expOdd(x *Nat, y *Nat, m *Modulus) *Nat {
	size := len(m.nat.limbs)

	xModM := new(Nat).Mod(x, m)
	yLimbs := y.unaliasedLimbs(z)

	scratch := z.resizedLimbs(_W * 18 * size)
	scratch1 := scratch[16*size : 17*size]
	scratch2 := scratch[17*size:]

	z.limbs = scratch[:size]
	for i := 0; i < size; i++ {
		z.limbs[i] = 0
	}
	z.limbs[0] = 1
	montgomeryRepresentation(z.limbs, scratch1, m)

	x1 := scratch[size : 2*size]
	copy(x1, xModM.limbs)
	montgomeryRepresentation(scratch[size:2*size], scratch1, m)
	for i := 2; i < 16; i++ {
		ximinus1 := scratch[(i-1)*size : i*size]
		xi := scratch[i*size : (i+1)*size]
		montgomeryMul(ximinus1, x1, xi, scratch1, m)
	}

	// LEAK: y's length
	// OK: this should be public
	for i := len(yLimbs) - 1; i >= 0; i-- {
		yi := yLimbs[i]
		for j := _W - 4; j >= 0; j -= 4 {
			montgomeryMul(z.limbs, z.limbs, z.limbs, scratch1, m)
			montgomeryMul(z.limbs, z.limbs, z.limbs, scratch1, m)
			montgomeryMul(z.limbs, z.limbs, z.limbs, scratch1, m)
			montgomeryMul(z.limbs, z.limbs, z.limbs, scratch1, m)

			window := (yi >> j) & 0b1111
			for i := 1; i < 16; i++ {
				xToI := scratch[i*size : (i+1)*size]
				ctCondCopy(ctEq(window, Word(i)), scratch1, xToI)
			}
			montgomeryMul(z.limbs, scratch1, scratch1, scratch2, m)
			ctCondCopy(1^ctEq(window, 0), z.limbs, scratch1)
		}
	}
	for i := 0; i < size; i++ {
		scratch2[i] = 0
	}
	scratch2[0] = 1
	montgomeryMul(z.limbs, scratch2, z.limbs, scratch1, m)
	z.reduced = m
	z.announced = m.nat.announced
	return z
}

func (z *Nat) expEven(x *Nat, y *Nat, m *Modulus) *Nat {
	xModM := new(Nat).Mod(x, m)
	yLimbs := y.unaliasedLimbs(z)

	scratch := new(Nat)

	// LEAK: y's length
	// OK: this should be public
	for i := len(yLimbs) - 1; i >= 0; i-- {
		yi := yLimbs[i]
		for j := _W; j >= 0; j-- {
			z.ModMul(z, z, m)

			sel := Choice((yi >> j) & 1)
			scratch.ModMul(z, xModM, m)
			ctCondCopy(sel, z.limbs, scratch.limbs)
		}
	}
	return z
}

// Exp calculates z <- x^y mod m
//
// The capacity of the resulting number matches the capacity of the modulus
func (z *Nat) Exp(x *Nat, y *Nat, m *Modulus) *Nat {
	if m.even {
		return z.expEven(x, y, m)
	} else {
		return z.expOdd(x, y, m)
	}
}

// cmpEq compares two limbs (same size) returning 1 if x >= y, and 0 otherwise
func cmpEq(x []Word, y []Word) Choice {
	res := Choice(1)
	for i := 0; i < len(x) && i < len(y); i++ {
		res &= ctEq(x[i], y[i])
	}
	return res
}

// cmpGeq compares two limbs (same size) returning 1 if x >= y, and 0 otherwise
func cmpGeq(x []Word, y []Word) Choice {
	var c uint
	for i := 0; i < len(x) && i < len(y); i++ {
		_, c = bits.Sub(uint(x[i]), uint(y[i]), c)
	}
	return 1 ^ Choice(c)
}

// cmpZero checks if a slice is equal to zero, in constant time
//
// LEAK: the length of a
func cmpZero(a []Word) Choice {
	var v Word
	for i := 0; i < len(a); i++ {
		v |= a[i]
	}
	return ctEq(v, 0)
}

// Cmp compares two natural numbers, returning results for (>, =, <) in that order.
//
// Because these relations are mutually exclusive, exactly one of these values
// will be true.
//
// This function doesn't leak any information about the values involved, only
// their announced lengths.
func (z *Nat) Cmp(x *Nat) (Choice, Choice, Choice) {
	// Rough Idea: Resize both slices to the maximum length, then compare
	// using that length

	maxBits := z.maxAnnounced(x)
	zLimbs := z.resizedLimbs(maxBits)
	xLimbs := x.resizedLimbs(maxBits)

	eq := Choice(1)
	geq := Choice(1)
	for i := 0; i < len(zLimbs) && i < len(xLimbs); i++ {
		eq_at_i := ctEq(zLimbs[i], xLimbs[i])
		eq &= eq_at_i
		geq = (eq_at_i & geq) | ((1 ^ eq_at_i) & ctGt(zLimbs[i], xLimbs[i]))
	}
	if (eq & (1 ^ geq)) == 1 {
		panic("eq but not geq")
	}
	return geq & (1 ^ eq), eq, 1 ^ geq
}

// CmpMod compares this natural number with a modulus, returning results for (>, =, <)
//
// This doesn't leak anything about the values of the numbers, only their lengths.
func (z *Nat) CmpMod(m *Modulus) (Choice, Choice, Choice) {
	return z.Cmp(&m.nat)
}

// Eq checks if z = y.
//
// This is equivalent to looking at the second choice returned by Cmp.
// But, since looking at equality is so common, this function is provided
// as an extra utility.
func (z *Nat) Eq(y *Nat) Choice {
	_, eq, _ := z.Cmp(y)
	return eq
}

// EqZero compares z to 0.
//
// This is more efficient that calling Eq between this Nat and a zero Nat.
func (z *Nat) EqZero() Choice {
	return cmpZero(z.limbs)
}

// mixSigned calculates a <- alpha * a + beta * b, returning whether the result is negative.
//
// alpha and beta are signed integers, but whose absolute value is < 2^(_W / 2).
// They're represented in two's complement.
//
// a and b both have an extra limb. We use the extra limb of a to store the full
// result.
func mixSigned(a, b []Word, alpha, beta Word) Choice {
	// Get the sign and absolute value for alpha
	alphaNeg := alpha >> (_W - 1)
	alpha = (alpha ^ -alphaNeg) + alphaNeg
	// Get the sign and absolute value for beta
	betaNeg := beta >> (_W - 1)
	beta = (beta ^ -betaNeg) + betaNeg

	// Our strategy for representing the result is to use a two's complement
	// representation alongside an extra limb.

	// Multiply a by alpha
	var cc Word
	for i := 0; i < len(a)-1; i++ {
		cc, a[i] = mulAddWWW_g(alpha, a[i], cc)
	}
	a[len(a)-1] = cc
	// Correct for sign
	negateTwos(Choice(alphaNeg), a)

	// We want to do the same for b, and then add it to a, but without
	// creating a temporary array
	var mulCarry, negCarry, addCarry, si Word
	mulCarry, si = mulAddWWW_g(beta, b[0], 0)
	si, negCarry = add(si^-betaNeg, betaNeg, 0)
	a[0], addCarry = add(a[0], si, 0)
	for i := 1; i < len(b)-1; i++ {
		mulCarry, si = mulAddWWW_g(beta, b[i], mulCarry)
		si, negCarry = add(si^-betaNeg, 0, negCarry)
		a[i], addCarry = add(a[i], si, addCarry)
	}
	si, _ = add(mulCarry^-betaNeg, 0, negCarry)
	a[len(a)-1], _ = add(a[len(a)-1], si, addCarry)

	outNeg := Choice(a[len(a)-1] >> (_W - 1))
	negateTwos(outNeg, a)

	return outNeg
}

// topLimbs finds the most significant _W bits of a and b
//
// This function assumes that a and b have the same length.
//
// By this, we mean aligning a and b, and then reading down _W bits starting
// from the first bit that a or b have set.
func topLimbs(a, b []Word) (Word, Word) {
	// explicitly checking this avoids indexing checks later too
	if len(a) != len(b) {
		panic("topLimbs: mismatched arguments")
	}
	// We lookup pairs of elements from top to bottom, until a1 or b1 != 0
	var a1, a0, b1, b0 Word
	done := Choice(0)
	for i := len(a) - 1; i > 0; i-- {
		a1 = ctIfElse(done, a1, a[i])
		a0 = ctIfElse(done, a0, a[i-1])
		b1 = ctIfElse(done, b1, b[i])
		b0 = ctIfElse(done, b0, b[i-1])
		done = 1 ^ ctEq(a1|b1, 0)
	}
	// Now, we look at the leading zeros to make sure that we're looking at the top
	// bits completely.

	// Converting to Word avoids a panic check
	l := Word(leadingZeros(a1 | b1))
	return (a1 << l) | (a0 >> (_W - l)), (b1 << l) | (b0 >> (_W - l))
}

// invert calculates and returns v s.t. vx = 1 mod m, and a flag indicating success.
//
// This function assumes that m is and odd number, but doesn't assume
// that m is truncated to its full size.
//
// announced should be the number of significant bits in m.
//
// x should already be reduced modulo m.
//
// m0inv should be -invertModW(m[0]), which might have been precomputed in some
// cases.
func (z *Nat) invert(announced int, x []Word, m []Word, m0inv Word) Choice {
	// This function follows Thomas Pornin's optimized GCD method:
	//   https://eprint.iacr.org/2020/972
	if len(x) != len(m) {
		panic("invert: mismatched arguments")
	}

	size := len(m)
	// We need 4 normal buffers, and one scratch buffer.
	// We make each of them have an extra limb, because our updates produce an extra
	// _W / 2 bits or so, before shifting, or modular reduction, and it's convenient
	// to do these "large" updates in place.
	z.limbs = z.resizedLimbs(_W * 5 * (size + 1))
	// v = 0, u = 1, a = x, b = m
	v := z.limbs[:size+1]
	u := z.limbs[size+1 : 2*(size+1)]
	for i := 0; i < size; i++ {
		u[i] = 0
		v[i] = 0
	}
	u[0] = 1
	a := z.limbs[3*(size+1) : 4*(size+1)]
	copy(a, x)
	b := z.limbs[2*(size+1) : 3*(size+1)]
	copy(b, m)
	scratch := z.limbs[4*(size+1):]

	// k is half of our limb size
	//
	// We do k - 1 inner iterations inside our loop.
	const k = _W >> 1
	// kMask allows us to keep only this half of a limb
	const kMask = (1 << k) - 1
	// iterMask allows us to mask off first (k - 1) bits, which is useful, since
	// that's how many inner iterations we have.
	const iterMask = Word((1 << (k - 1)) - 1)
	// The minimum number of iterations is 2 * announced - 1. So, we calculate
	// the ceiling of this quantity divided by (k - 1), since that's the number
	// of iterations we do inside the inner loop
	iterations := ((2*announced - 1) + k - 2) / (k - 1)
	for i := 0; i < iterations; i++ {
		// The core idea is to use an approximation of a and b to calculate update
		// factors. We want to use the low k - 1 bits, combined with the high k + 1 bits.
		// This is because the low k - 1 bits suffice to give us odd / even information
		// for our k - 1 iterations, and the remaining high bits allow us to check
		// a < b as well.
		aBar := a[0]
		bBar := b[0]
		if size > 1 {
			aTop, bTop := topLimbs(a[:size], b[:size])
			aBar = (iterMask & aBar) | (^iterMask & aTop)
			bBar = (iterMask & bBar) | (^iterMask & bTop)
		}
		// We store two factors in a single register, to make the inner loop faster.
		//
		//  fg = f + (2^(k-1) - 1) + 2^k(g + (2^(k-1) - 1))
		//
		// The reason we add in 2^(k-1) - 1, is so that the result in each half
		// doesn't go negative. We then subtract this factor away when extracting
		// the coefficients.

		// This factor needs to be added when we subtract one double register from
		// another, and vice versa.
		const coefficientAdjust = iterMask * ((1 << k) + 1)
		fg0 := Word(1) + coefficientAdjust
		fg1 := Word(1<<k) + coefficientAdjust
		for j := 0; j < k-1; j++ {
			// Note: inlining the ctIfElse's produces worse assembly, for some reason;
			// there's a lot more register spilling.
			acp := aBar
			bcp := bBar
			fg0cp := fg0
			fg1cp := fg1

			_, carry := bits.Sub(uint(aBar), uint(bBar), 0)
			aSmaller := Choice(carry)
			aBar = ctIfElse(aSmaller, bcp, aBar)
			bBar = ctIfElse(aSmaller, acp, bBar)
			fg0 = ctIfElse(aSmaller, fg1cp, fg0)
			fg1 = ctIfElse(aSmaller, fg0cp, fg1)

			aBar -= bBar
			fg0 -= fg1
			fg0 += coefficientAdjust

			aOdd := Choice(acp & 1)
			aBar = ctIfElse(aOdd, aBar, acp)
			bBar = ctIfElse(aOdd, bBar, bcp)
			fg0 = ctIfElse(aOdd, fg0, fg0cp)
			fg1 = ctIfElse(aOdd, fg1, fg1cp)

			aBar >>= 1
			fg1 += fg1
			fg1 -= coefficientAdjust
		}
		// Extract out the actual coefficients, as per the previous discussion.
		f0 := (fg0 & kMask) - iterMask
		g0 := (fg0 >> k) - iterMask
		f1 := (fg1 & kMask) - iterMask
		g1 := (fg1 >> k) - iterMask

		// a, b <- (f0 * a + g0 * b), (f1 * a + g1 * b)
		copy(scratch, a)
		aNeg := Word(mixSigned(a, b, f0, g0))
		bNeg := Word(mixSigned(b, scratch, g1, f1))
		// This will always clear the low k - 1 bits, so we shift those away
		shrVU(a, a, k-1)
		shrVU(b, b, k-1)
		// The result may have been negative, in which case we need to negate
		// the coefficients for the updates to u and v.
		f0 = (f0 ^ -aNeg) + aNeg
		g0 = (g0 ^ -aNeg) + aNeg
		f1 = (f1 ^ -bNeg) + bNeg
		g1 = (g1 ^ -bNeg) + bNeg

		// u, v <- (f0 * u + g0 * v), (f1 * u + g1 * v)
		copy(scratch, u)
		uNeg := mixSigned(u, v, f0, g0)
		vNeg := mixSigned(v, scratch, g1, f1)

		// Now, reduce u and v mod m, making sure to conditionally negate the result.
		u0 := u[0]
		copy(u, u[1:])
		shiftAddInGeneric(u[:size], scratch[:size], u0, m)
		subVV(scratch[:size], m, u[:size])
		ctCondCopy(uNeg&(1^cmpZero(u)), u[:size], scratch[:size])

		v0 := v[0]
		copy(v, v[1:])
		shiftAddInGeneric(v[:size], scratch[:size], v0, m)
		subVV(scratch[:size], m, v[:size])
		ctCondCopy(vNeg&(1^cmpZero(v)), v[:size], scratch[:size])
	}

	// v now contains our inverse, multiplied by 2^(iterations). We need to correct
	// this by dividing by 2. We can use the same trick as in montgomery multiplication,
	// adding the correct multiple of m to clear the low bits, and then shifting
	totalIterations := iterations * (k - 1)
	// First, we try and do _W / 2 bits at a time. This is a convenient amount,
	// because then the coefficient only occupies a single limb.
	for i := 0; i < totalIterations/k; i++ {
		v[size] = addMulVVW(v[:size], m, (m0inv*v[0])&kMask)
		shrVU(v, v, k)
	}
	// If there are any iterations remaining, we can take care of them by clearing
	// a smaller number of bits.
	remaining := totalIterations % k
	if remaining > 0 {
		lastMask := Word((1 << remaining) - 1)
		v[size] = addMulVVW(v[:size], m, (m0inv*v[0])&lastMask)
		shrVU(v, v, uint(remaining))
	}

	z.Resize(announced)
	// Inversion succeeded if b, which contains gcd(x, m), is 1.
	return cmpZero(b[1:]) & ctEq(1, b[0])
}

// Coprime returns 1 if gcd(x, y) == 1, and 0 otherwise
func (x *Nat) Coprime(y *Nat) Choice {
	maxBits := x.maxAnnounced(y)
	size := limbCount(maxBits)
	if size == 0 {
		// technically the result should be 1 since 0 is not a divisor,
		// but we expect 0 when both arguments are equal.
		return 0
	}
	a := make([]Word, size)
	copy(a, x.limbs)
	b := make([]Word, size)
	copy(b, y.limbs)

	// Our gcd(a, b) routine requires b to be odd, and will return garbage otherwise.
	aOdd := Choice(a[0] & 1)
	ctCondSwap(aOdd, a, b)

	scratch := new(Nat)
	bOdd := Choice(b[0] & 1)
	// We make b odd so that our calculations aren't messed up, but this doesn't affect
	// our result
	b[0] |= 1
	invertible := scratch.invert(maxBits, a, b, -invertModW(b[0]))

	// If at least one of a or b is odd, then our GCD calculation will have been correct,
	// otherwise, both are even, so we want to return false anyways.
	return (aOdd | bOdd) & invertible
}

// IsUnit checks if x is a unit, i.e. invertible, mod m.
//
// This so happens to be when gcd(x, m) == 1.
func (x *Nat) IsUnit(m *Modulus) Choice {
	return x.Coprime(&m.nat)
}

// modInverse calculates the inverse of a reduced x modulo m
//
// This assumes that m is an odd number, but not that it's truncated
// to its true size. This routine will only leak the announced sizes of
// x and m.
//
// We also assume that x is already reduced modulo m
func (z *Nat) modInverse(x *Nat, m *Nat, m0inv Word) *Nat {
	// Make sure that z doesn't alias either of m or x
	xLimbs := x.unaliasedLimbs(z)
	mLimbs := m.unaliasedLimbs(z)
	z.invert(m.announced, xLimbs, mLimbs, m0inv)
	return z
}

// ModInverse calculates z <- x^-1 mod m
//
// This will produce nonsense if the modulus is even.
//
// The capacity of the resulting number matches the capacity of the modulus
func (z *Nat) ModInverse(x *Nat, m *Modulus) *Nat {
	z.Mod(x, m)
	if m.even {
		z.modInverseEven(x, m)
	} else {
		z.modInverse(z, &m.nat, m.m0inv)
	}
	z.reduced = m
	return z
}

// divDouble divides x by d, outputtting the quotient in out, and a remainder
//
// This routine assumes nothing about the padding of either of its inputs, and
// leaks nothing beyond their announced length.
//
// If out is not empty, it's assumed that x has at most twice the bit length of d,
// and the quotient can thus fit in a slice the length of d, which out is assumed to be.
//
// If out is nil, no quotient is produced, but the remainder is still calculated.
// This remainder will be correct regardless of the size difference between x and d.
func divDouble(x []Word, d []Word, out []Word) []Word {
	size := len(d)
	r := make([]Word, size)
	scratch := make([]Word, size)

	// We use free injection, like in Mod
	i := len(x) - 1
	// We can inject at least size - 1 limbs while staying under m
	// Thus, we start injecting from index size - 2
	start := size - 2
	// That is, if there are at least that many limbs to choose from
	if i < start {
		start = i
	}
	for j := start; j >= 0; j-- {
		r[j] = x[i]
		i--
	}

	for ; i >= 0; i-- {
		oi := shiftAddInGeneric(r, scratch, x[i], d)
		// Hopefully the branch predictor can make these checks not too expensive,
		// otherwise we'll have to duplicate the routine
		if out != nil {
			out[i] = oi
		}
	}
	return r
}

// ModInverseEven calculates the modular inverse of x, mod m
//
// This routine will work even if m is an even number, unlike ModInverse.
// Furthermore, it doesn't require the modulus to be truncated to its true size, and
// will only leak information about the public sizes of its inputs. It is slower
// than the standard routine though.
//
// This function assumes that x has an inverse modulo m, naturally
func (z *Nat) modInverseEven(x *Nat, m *Modulus) *Nat {
	if x.announced <= 0 {
		return z.Resize(0)
	}
	// Idea:
	//
	// You want to find Z such that ZX = 1 mod M. The problem is
	// that the usual routine assumes that m is odd. In this case m is even.
	// For X to be invertible, we need it to be odd. We can thus invert M mod X,
	// finding an A satisfying AM = 1 mod X. This means that AM = 1 + KX, for some
	// positive integer K. Modulo M, this entails that KX = -1 mod M, so -K provides
	// us with an inverse for X.
	//
	// To find K, we can calculate (AM - 1) / X, and then subtract this from M, to get our inverse.
	size := len(m.nat.limbs)
	// We want to invert m modulo x, so we first calculate the reduced version, before inverting
	var newZ Nat
	newZ.limbs = divDouble(m.nat.limbs, x.limbs, nil)
	newZ.modInverse(&newZ, x, -invertModW(x.limbs[0]))
	inverseZero := cmpZero(newZ.limbs)
	newZ.Mul(&newZ, &m.nat, 2*size*_W)
	newZ.limbs = newZ.resizedLimbs(_W * 2 * size)
	subVW(newZ.limbs, newZ.limbs, 1)
	divDouble(newZ.limbs, x.limbs, newZ.limbs)
	// The result fits on a single half of newZ, but we need to subtract it from m.
	// We can use the other half of newZ, and then copy it back over if we need to keep it
	subVV(newZ.limbs[size:], m.nat.limbs, newZ.limbs[:size])
	// If the inverse was zero, then x was 1, and so we should return 1.
	// We go ahead and prepare this result, but expect to copy over the subtraction
	// we just calculated soon over, in the usual case.
	newZ.limbs[0] = 1
	for i := 1; i < size; i++ {
		newZ.limbs[i] = 0
	}
	ctCondCopy(1^inverseZero, newZ.limbs[:size], newZ.limbs[size:])

	z.limbs = newZ.limbs
	z.Resize(m.nat.announced)
	return z
}

// modSqrt3Mod4 sets z <- sqrt(x) mod p, when p is a prime with p = 3 mod 4
func (z *Nat) modSqrt3Mod4(x *Nat, p *Modulus) *Nat {
	// In this case, we can do x^(p + 1) / 4
	e := new(Nat).SetNat(&p.nat)
	carry := addVW(e.limbs, e.limbs, 1)
	shrVU(e.limbs, e.limbs, 2)
	e.limbs[len(e.limbs)-1] |= (carry << (_W - 2))
	return z.Exp(x, e, p)
}

// tonelliShanks sets z <- sqrt(x) mod p, for any prime modulus
func (z *Nat) tonelliShanks(x *Nat, p *Modulus) *Nat {
	// c.f. https://datatracker.ietf.org/doc/html/draft-irtf-cfrg-hash-to-curve-09#appendix-G.4
	scratch := new(Nat)
	x = new(Nat).SetNat(x)

	one := new(Nat).SetUint64(1)
	trailingZeros := 1
	reducedPminusOne := new(Nat).Sub(&p.nat, one, p.BitLen())
	// In this case, p must have been 1, so sqrt(x) mod p is 0. Explicitly checking
	// this avoids an infinite loop when trying to remove the least significant zeros.
	// Checking this value is fine, since ModSqrt is explicitly allowed to branch
	// on the value of the modulus.
	if reducedPminusOne.EqZero() == 1 {
		return z.SetUint64(0)
	}
	shrVU(reducedPminusOne.limbs, reducedPminusOne.limbs, 1)

	nonSquare := new(Nat).SetUint64(2)
	for scratch.Exp(nonSquare, reducedPminusOne, p).Eq(one) == 1 {
		nonSquare.Add(nonSquare, one, p.BitLen())
	}

	for reducedPminusOne.limbs[0]&1 == 0 {
		trailingZeros += 1
		shrVU(reducedPminusOne.limbs, reducedPminusOne.limbs, 1)
	}

	reducedQminusOne := new(Nat).Sub(reducedPminusOne, one, p.BitLen())
	shrVU(reducedQminusOne.limbs, reducedQminusOne.limbs, 1)

	c := new(Nat).Exp(nonSquare, reducedPminusOne, p)

	z.Exp(x, reducedQminusOne, p)
	t := new(Nat).ModMul(z, z, p)
	t.ModMul(t, x, p)
	z.ModMul(z, x, p)
	b := new(Nat).SetNat(t)
	one.limbs = one.resizedLimbs(len(b.limbs))
	for i := trailingZeros; i > 1; i-- {
		for j := 1; j < i-1; j++ {
			b.ModMul(b, b, p)
		}
		sel := 1 ^ cmpEq(b.limbs, one.limbs)
		scratch.ModMul(z, c, p)
		ctCondCopy(sel, z.limbs, scratch.limbs)
		c.ModMul(c, c, p)
		scratch.ModMul(t, c, p)
		ctCondCopy(sel, t.limbs, scratch.limbs)
		b.SetNat(t)
	}
	z.reduced = p
	return z
}

// ModSqrt calculates the square root of x modulo p
//
// p must be an odd prime number, and x must actually have a square root
// modulo p. The result is undefined if these conditions aren't satisfied
//
// This function will leak information about the value of p. This isn't intended
// to be used in situations where the modulus isn't publicly known.
func (z *Nat) ModSqrt(x *Nat, p *Modulus) *Nat {
	if len(p.nat.limbs) == 0 {
		panic("Can't take square root mod 0")
	}
	if p.nat.limbs[0]&1 == 0 {
		panic("Can't take square root mod an even number")
	}
	if p.nat.limbs[0]&0b11 == 0b11 {
		return z.modSqrt3Mod4(x, p)
	}
	return z.tonelliShanks(x, p)
}
