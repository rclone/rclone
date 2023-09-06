//go:build !amd64
// +build !amd64

package hash_avx2

import "github.com/zeebo/blake3/internal/alg/hash/hash_pure"

func HashF(input *[8192]byte, length, counter uint64, flags uint32, key *[8]uint32, out *[64]uint32, chain *[8]uint32) {
	hash_pure.HashF(input, length, counter, flags, key, out, chain)
}

func HashP(left, right *[64]uint32, flags uint32, key *[8]uint32, out *[64]uint32, n int) {
	hash_pure.HashP(left, right, flags, key, out, n)
}
