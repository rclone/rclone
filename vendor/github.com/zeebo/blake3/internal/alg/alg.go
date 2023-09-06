package alg

import (
	"github.com/zeebo/blake3/internal/alg/compress"
	"github.com/zeebo/blake3/internal/alg/hash"
)

func HashF(input *[8192]byte, length, counter uint64, flags uint32, key *[8]uint32, out *[64]uint32, chain *[8]uint32) {
	hash.HashF(input, length, counter, flags, key, out, chain)
}

func HashP(left, right *[64]uint32, flags uint32, key *[8]uint32, out *[64]uint32, n int) {
	hash.HashP(left, right, flags, key, out, n)
}

func Compress(chain *[8]uint32, block *[16]uint32, counter uint64, blen uint32, flags uint32, out *[16]uint32) {
	compress.Compress(chain, block, counter, blen, flags, out)
}
