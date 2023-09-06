package compress

import (
	"github.com/zeebo/blake3/internal/alg/compress/compress_pure"
	"github.com/zeebo/blake3/internal/alg/compress/compress_sse41"
	"github.com/zeebo/blake3/internal/consts"
)

func Compress(chain *[8]uint32, block *[16]uint32, counter uint64, blen uint32, flags uint32, out *[16]uint32) {
	if consts.HasSSE41 {
		compress_sse41.Compress(chain, block, counter, blen, flags, out)
	} else {
		compress_pure.Compress(chain, block, counter, blen, flags, out)
	}
}
