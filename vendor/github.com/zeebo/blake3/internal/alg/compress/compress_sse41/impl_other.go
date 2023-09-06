//go:build !amd64
// +build !amd64

package compress_sse41

import "github.com/zeebo/blake3/internal/alg/compress/compress_pure"

func Compress(chain *[8]uint32, block *[16]uint32, counter uint64, blen uint32, flags uint32, out *[16]uint32) {
	compress_pure.Compress(chain, block, counter, blen, flags, out)
}
