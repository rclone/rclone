package xxhash

const (
	prime32x1 uint32 = 2654435761
	prime32x2 uint32 = 2246822519
	prime32x3 uint32 = 3266489917
	prime32x4 uint32 = 668265263
	prime32x5 uint32 = 374761393

	prime64x1 uint64 = 11400714785074694791
	prime64x2 uint64 = 14029467366897019727
	prime64x3 uint64 = 1609587929392839161
	prime64x4 uint64 = 9650029242287828579
	prime64x5 uint64 = 2870177450012600261

	maxInt32 int32 = (1<<31 - 1)

	// precomputed zero Vs for seed 0
	zero64x1 = 0x60ea27eeadc0b5d6
	zero64x2 = 0xc2b2ae3d27d4eb4f
	zero64x3 = 0x0
	zero64x4 = 0x61c8864e7a143579
)

// Checksum32 returns the checksum of the input data with the seed set to 0.
func Checksum32(in []byte) uint32 {
	return Checksum32S(in, 0)
}

// ChecksumString32 returns the checksum of the input data, without creating a copy, with the seed set to 0.
func ChecksumString32(s string) uint32 {
	return ChecksumString32S(s, 0)
}

type XXHash32 struct {
	mem            [16]byte
	ln, memIdx     int32
	v1, v2, v3, v4 uint32
	seed           uint32
}

// Size returns the number of bytes Sum will return.
func (xx *XXHash32) Size() int {
	return 4
}

// BlockSize returns the hash's underlying block size.
// The Write method must be able to accept any amount
// of data, but it may operate more efficiently if all writes
// are a multiple of the block size.
func (xx *XXHash32) BlockSize() int {
	return 16
}

// NewS32 creates a new hash.Hash32 computing the 32bit xxHash checksum starting with the specific seed.
func NewS32(seed uint32) (xx *XXHash32) {
	xx = &XXHash32{
		seed: seed,
	}
	xx.Reset()
	return
}

// New32 creates a new hash.Hash32 computing the 32bit xxHash checksum starting with the seed set to 0.
func New32() *XXHash32 {
	return NewS32(0)
}

func (xx *XXHash32) Reset() {
	xx.v1 = xx.seed + prime32x1 + prime32x2
	xx.v2 = xx.seed + prime32x2
	xx.v3 = xx.seed
	xx.v4 = xx.seed - prime32x1
	xx.ln, xx.memIdx = 0, 0
}

// Sum appends the current hash to b and returns the resulting slice.
// It does not change the underlying hash state.
func (xx *XXHash32) Sum(in []byte) []byte {
	s := xx.Sum32()
	return append(in, byte(s>>24), byte(s>>16), byte(s>>8), byte(s))
}

// Checksum64 an alias for Checksum64S(in, 0)
func Checksum64(in []byte) uint64 {
	return Checksum64S(in, 0)
}

// ChecksumString64 returns the checksum of the input data, without creating a copy, with the seed set to 0.
func ChecksumString64(s string) uint64 {
	return ChecksumString64S(s, 0)
}

type XXHash64 struct {
	v1, v2, v3, v4 uint64
	seed           uint64
	ln             uint64
	mem            [32]byte
	memIdx         int8
}

// Size returns the number of bytes Sum will return.
func (xx *XXHash64) Size() int {
	return 8
}

// BlockSize returns the hash's underlying block size.
// The Write method must be able to accept any amount
// of data, but it may operate more efficiently if all writes
// are a multiple of the block size.
func (xx *XXHash64) BlockSize() int {
	return 32
}

// NewS64 creates a new hash.Hash64 computing the 64bit xxHash checksum starting with the specific seed.
func NewS64(seed uint64) (xx *XXHash64) {
	xx = &XXHash64{
		seed: seed,
	}
	xx.Reset()
	return
}

// New64 creates a new hash.Hash64 computing the 64bit xxHash checksum starting with the seed set to 0x0.
func New64() *XXHash64 {
	return NewS64(0)
}

func (xx *XXHash64) Reset() {
	xx.ln, xx.memIdx = 0, 0
	xx.v1, xx.v2, xx.v3, xx.v4 = resetVs64(xx.seed)
}

// Sum appends the current hash to b and returns the resulting slice.
// It does not change the underlying hash state.
func (xx *XXHash64) Sum(in []byte) []byte {
	s := xx.Sum64()
	return append(in, byte(s>>56), byte(s>>48), byte(s>>40), byte(s>>32), byte(s>>24), byte(s>>16), byte(s>>8), byte(s))
}

// force the compiler to use ROTL instructions

func rotl32_1(x uint32) uint32  { return (x << 1) | (x >> (32 - 1)) }
func rotl32_7(x uint32) uint32  { return (x << 7) | (x >> (32 - 7)) }
func rotl32_11(x uint32) uint32 { return (x << 11) | (x >> (32 - 11)) }
func rotl32_12(x uint32) uint32 { return (x << 12) | (x >> (32 - 12)) }
func rotl32_13(x uint32) uint32 { return (x << 13) | (x >> (32 - 13)) }
func rotl32_17(x uint32) uint32 { return (x << 17) | (x >> (32 - 17)) }
func rotl32_18(x uint32) uint32 { return (x << 18) | (x >> (32 - 18)) }

func rotl64_1(x uint64) uint64  { return (x << 1) | (x >> (64 - 1)) }
func rotl64_7(x uint64) uint64  { return (x << 7) | (x >> (64 - 7)) }
func rotl64_11(x uint64) uint64 { return (x << 11) | (x >> (64 - 11)) }
func rotl64_12(x uint64) uint64 { return (x << 12) | (x >> (64 - 12)) }
func rotl64_18(x uint64) uint64 { return (x << 18) | (x >> (64 - 18)) }
func rotl64_23(x uint64) uint64 { return (x << 23) | (x >> (64 - 23)) }
func rotl64_27(x uint64) uint64 { return (x << 27) | (x >> (64 - 27)) }
func rotl64_31(x uint64) uint64 { return (x << 31) | (x >> (64 - 31)) }

func mix64(h uint64) uint64 {
	h ^= h >> 33
	h *= prime64x2
	h ^= h >> 29
	h *= prime64x3
	h ^= h >> 32
	return h
}

func resetVs64(seed uint64) (v1, v2, v3, v4 uint64) {
	if seed == 0 {
		return zero64x1, zero64x2, zero64x3, zero64x4
	}
	return (seed + prime64x1 + prime64x2), (seed + prime64x2), (seed), (seed - prime64x1)
}

// borrowed from cespare
func round64(h, v uint64) uint64 {
	h += v * prime64x2
	h = rotl64_31(h)
	h *= prime64x1
	return h
}

func mergeRound64(h, v uint64) uint64 {
	v = round64(0, v)
	h ^= v
	h = h*prime64x1 + prime64x4
	return h
}
