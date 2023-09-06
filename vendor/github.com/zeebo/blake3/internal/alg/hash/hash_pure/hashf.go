package hash_pure

import (
	"unsafe"

	"github.com/zeebo/blake3/internal/alg/compress"
	"github.com/zeebo/blake3/internal/consts"
	"github.com/zeebo/blake3/internal/utils"
)

func HashF(input *[8192]byte, length, counter uint64, flags uint32, key *[8]uint32, out *[64]uint32, chain *[8]uint32) {
	var tmp [16]uint32

	for i := uint64(0); consts.ChunkLen*i < length && i < 8; i++ {
		bchain := *key
		bflags := flags | consts.Flag_ChunkStart
		start := consts.ChunkLen * i

		for n := uint64(0); n < 16; n++ {
			if n == 15 {
				bflags |= consts.Flag_ChunkEnd
			}
			if start+64*n >= length {
				break
			}
			if start+64+64*n >= length {
				*chain = bchain
			}

			var blockPtr *[16]uint32
			if consts.OptimizeLittleEndian {
				blockPtr = (*[16]uint32)(unsafe.Pointer(&input[consts.ChunkLen*i+consts.BlockLen*n]))
			} else {
				var block [16]uint32
				utils.BytesToWords((*[64]uint8)(unsafe.Pointer(&input[consts.ChunkLen*i+consts.BlockLen*n])), &block)
				blockPtr = &block
			}

			compress.Compress(&bchain, blockPtr, counter, consts.BlockLen, bflags, &tmp)

			bchain = *(*[8]uint32)(unsafe.Pointer(&tmp[0]))
			bflags = flags
		}

		out[i+0] = bchain[0]
		out[i+8] = bchain[1]
		out[i+16] = bchain[2]
		out[i+24] = bchain[3]
		out[i+32] = bchain[4]
		out[i+40] = bchain[5]
		out[i+48] = bchain[6]
		out[i+56] = bchain[7]

		counter++
	}
}
