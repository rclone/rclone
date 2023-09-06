//go:build amd64
// +build amd64

package hash_avx2

//go:noescape
func HashF(input *[8192]byte, length, counter uint64, flags uint32, key *[8]uint32, out *[64]uint32, chain *[8]uint32)

//go:noescape
func HashP(left, right *[64]uint32, flags uint32, key *[8]uint32, out *[64]uint32, n int)
