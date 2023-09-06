package saferith

import (
	"errors"
	"math/big"
	"math/bits"
)

// Int represents a signed integer of arbitrary size.
//
// Similarly to Nat, each Int comes along with an announced size, representing
// the number of bits need to represent its absolute value. This can be
// larger than its true size, the number of bits actually needed.
type Int struct {
	// This number is represented by (-1)^sign * abs, essentially

	// When 1, this is a negative number, when 0 a positive number.
	//
	// There's a bit of redundancy to note, because -0 and +0 represent the same
	// number. We need to be careful around this edge case.
	sign Choice
	// The absolute value.
	//
	// Not using a point is important, that way the zero value for Int is actually zero.
	abs Nat
}

// SetBytes interprets a number in big-endian form, stores it in z, and returns z.
//
// This number will be positive.
func (z *Int) SetBytes(data []byte) *Int {
	z.sign = 0
	z.abs.SetBytes(data)
	return z
}

// MarshalBinary implements encoding.BinaryMarshaler.
// The retrned byte slice is always of length 1 + len(i.Abs().Bytes()),
// where the first byte encodes the sign.
func (i *Int) MarshalBinary() ([]byte, error) {
	length := 1 + (i.abs.announced+7)/8
	out := make([]byte, length)
	out[0] = byte(i.sign)
	i.abs.FillBytes(out[1:])
	return out, nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
// Returns an error when the length of data is 0,
// since we always expect the first byte to encode the sign.
func (i *Int) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return errors.New("data must contain a sign byte")
	}
	i.abs.SetBytes(data[1:])
	i.sign = Choice(data[0] & 1)
	return nil
}

// SetUint64 sets the value of z to x.
//
// This number will be positive.
func (z *Int) SetUint64(x uint64) *Int {
	z.sign = 0
	z.abs.SetUint64(x)
	return z
}

// SetNat will set the absolute value of z to x, and the sign to zero, returning z.
func (z *Int) SetNat(x *Nat) *Int {
	z.sign = 0
	z.abs.SetNat(x)
	return z
}

// Clone returns a copy of this Int.
//
// The copy can safely be mutated without affecting the original value.
func (z *Int) Clone() *Int {
	out := new(Int)
	out.sign = z.sign
	out.abs.SetNat(&z.abs)
	return out
}

// SetBig will set the value of this number to the value of a big.Int, including sign.
//
// The size dicates the number of bits to use for the absolute value. This is important,
// in order to include additional padding that the big.Int might have stripped off.
//
// Since big.Int stores its sign as a boolean, it's likely that this conversion
// will leak the value of the sign.
func (z *Int) SetBig(x *big.Int, size int) *Int {
	// x.Sign() = {-1, 0, 1},
	// 1 - x.Sign() = {2, 1, 0},
	// so this comparison correctly sniffs out negative numbers
	z.sign = ctGt(Word(1-x.Sign()), 1)
	z.abs.SetBig(x, size)
	return z
}

// Big will convert this number into a big.Int, including sign.
//
// This will leak the true size of this number, and its sign, because of the leakiness
// of big.Int, so caution should be exercises when using this function.
func (z *Int) Big() *big.Int {
	abs := z.abs.Big()
	if z.sign == 1 {
		abs.Neg(abs)
	}
	return abs
}

// Resize adjust the announced size of this number, possibly truncating the absolute value.
func (z *Int) Resize(cap int) *Int {
	z.abs.Resize(cap)
	return z
}

// String formats this number as a signed hex string.
//
// This isn't a format that Int knows how to parse. This function exists mainly
// to help debugging, and whatnot.
func (z *Int) String() string {
	sign := ctIfElse(z.sign, Word('-'), Word('+'))
	return string(rune(sign)) + z.abs.String()
}

// Eq checks if this Int has the same value as another Int.
//
// Note that negative zero and positive zero are the same number.
func (z *Int) Eq(x *Int) Choice {
	zero := z.abs.EqZero()
	// If this is zero, then any number as the same sign,
	// otherwise, check that the signs aren't different
	sameSign := zero | (1 ^ z.sign ^ x.sign)
	return sameSign & z.abs.Eq(&x.abs)
}

// Abs returns the absolute value of this Int.
func (z *Int) Abs() *Nat {
	return new(Nat).SetNat(&z.abs)
}

// IsNegative checks if this value is negative
func (z *Int) IsNegative() Choice {
	return z.sign
}

// AnnouncedLen returns the announced size of this int's absolute value.
//
// See Nat.AnnouncedLen
func (z *Int) AnnouncedLen() int {
	return z.abs.AnnouncedLen()
}

// TrueLen returns the actual number of bits need to represent this int's absolute value.
//
// This leaks this value.
//
// See Nat.TrueLen
func (z *Int) TrueLen() int {
	return z.abs.TrueLen()
}

// Neg calculates z <- -x.
//
// The result has the same announced size.
func (z *Int) Neg(doit Choice) *Int {
	z.sign ^= doit
	return z
}

func (z *Int) SetInt(x *Int) *Int {
	z.sign = x.sign
	z.abs.SetNat(&x.abs)
	return z
}

// Mul calculates z <- x * y, returning z.
//
// This will truncate the resulting absolute value, based on the bit capacity passed in.
//
// If cap < 0, then capacity is x.AnnouncedLen() + y.AnnouncedLen().
func (z *Int) Mul(x *Int, y *Int, cap int) *Int {
	// (-1)^sx * ax * (-1)^sy * ay = (-1)^(sx + sy) * ax * ay
	z.sign = x.sign ^ y.sign
	z.abs.Mul(&x.abs, &y.abs, cap)
	return z
}

// Mod calculates z mod M, handling negatives correctly.
//
// As indicated by the types, this function will return a number in the range 0..m-1.
func (z *Int) Mod(m *Modulus) *Nat {
	out := new(Nat).Mod(&z.abs, m)
	negated := new(Nat).ModNeg(out, m)
	out.CondAssign(z.sign, negated)
	return out
}

// SetModSymmetric takes a number x mod M, and returns a signed number centered around 0.
//
// This effectively takes numbers in the range:
//    {0, .., m - 1}
// And returns numbers in the range:
//    {-(m - 1)/2, ..., 0, ..., (m - 1)/2}
// In the case that m is even, there will simply be an extra negative number.
func (z *Int) SetModSymmetric(x *Nat, m *Modulus) *Int {
	z.abs.Mod(x, m)
	negated := new(Nat).ModNeg(&z.abs, m)
	gt, _, _ := negated.Cmp(&z.abs)
	negatedLeq := 1 ^ gt
	// Always use the smaller value
	z.abs.CondAssign(negatedLeq, negated)
	// A negative modular number, by definition, will have it's negation <= itself
	z.sign = negatedLeq
	return z
}

// CheckInRange checks whether or not this Int is in the range for SetModSymmetric.
func (z *Int) CheckInRange(m *Modulus) Choice {
	// First check that the absolute value makes sense
	_, _, absOk := z.abs.CmpMod(m)

	negated := new(Nat).ModNeg(&z.abs, m)
	_, _, lt := negated.Cmp(&z.abs)
	// If the negated value is strictly smaller, then we have a number out of range
	signOk := 1 ^ lt

	return absOk & signOk
}

// ExpI calculates z <- x^i mod m.
//
// This works with negative exponents, but requires x to be invertible mod m, of course.
func (z *Nat) ExpI(x *Nat, i *Int, m *Modulus) *Nat {
	z.Exp(x, &i.abs, m)
	inverted := new(Nat).ModInverse(z, m)
	z.CondAssign(i.sign, inverted)
	return z
}

// conditionally negate a slice of words based on two's complement
func negateTwos(doit Choice, z []Word) {
	if len(z) <= 0 {
		return
	}
	sign := Word(doit)
	zi, carry := bits.Add(uint(-sign^z[0]), uint(sign), 0)
	z[0] = Word(zi)
	for i := 1; i < len(z); i++ {
		zi, carry = bits.Add(uint(-sign^z[i]), 0, carry)
		z[i] = Word(zi)
	}
}

// convert a slice to two's complement, using a sign, and writing the result to out
func toTwos(sign Choice, abs []Word, out []Word) {
	copy(out, abs)
	negateTwos(sign, out)
}

// convert a slice from two's complement, writing it in place, and producing a sign
func fromTwos(bits int, mut []Word) Choice {
	if len(mut) <= 0 {
		return 0
	}
	sign := Choice(mut[len(mut)-1] >> (_W - 1))
	negateTwos(sign, mut)
	return sign
}

// Add calculates z <- x + y.
//
// The cap determines the number of bits to use for the absolute value of the result.
//
// If cap < 0, cap gets set to max(x.AnnouncedLen(), y.AnnouncedLen()) + 1
func (z *Int) Add(x *Int, y *Int, cap int) *Int {
	// Rough idea, convert x and y to two's complement representation, add, and
	// then convert back, before truncating as necessary.
	if cap < 0 {
		cap = x.abs.maxAnnounced(&y.abs) + 1
	}

	xLimbs := x.abs.unaliasedLimbs(&z.abs)
	yLimbs := y.abs.unaliasedLimbs(&z.abs)

	// We need an extra bit for the sign
	size := limbCount(cap + 1)
	scratch := z.abs.resizedLimbs(_W * 2 * size)
	// Convert both to two's complement
	xTwos := scratch[:size]
	yTwos := scratch[size:]
	toTwos(x.sign, xLimbs, xTwos)
	toTwos(y.sign, yLimbs, yTwos)
	// The addition will now produce the right result
	addVV(xTwos, xTwos, yTwos)
	// Convert back from two's complement
	z.sign = fromTwos(cap, xTwos)
	size = limbCount(cap)
	z.abs.limbs = scratch[:size]
	copy(z.abs.limbs, xTwos)
	maskEnd(z.abs.limbs, cap)
	z.abs.reduced = nil
	z.abs.announced = cap

	return z
}
