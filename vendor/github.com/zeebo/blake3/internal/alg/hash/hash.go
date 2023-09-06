package hash

import (
	"github.com/zeebo/blake3/internal/alg/hash/hash_avx2"
	"github.com/zeebo/blake3/internal/alg/hash/hash_pure"
	"github.com/zeebo/blake3/internal/consts"
)

func HashF(input *[8192]byte, length, counter uint64, flags uint32, key *[8]uint32, out *[64]uint32, chain *[8]uint32) {
	if consts.HasAVX2 && length > 2*consts.ChunkLen {
		hash_avx2.HashF(input, length, counter, flags, key, out, chain)
	} else {
		hash_pure.HashF(input, length, counter, flags, key, out, chain)
	}
}

func HashP(left, right *[64]uint32, flags uint32, key *[8]uint32, out *[64]uint32, n int) {
	if consts.HasAVX2 && n >= 2 {
		hash_avx2.HashP(left, right, flags, key, out, n)
	} else {
		hash_pure.HashP(left, right, flags, key, out, n)
	}
}
