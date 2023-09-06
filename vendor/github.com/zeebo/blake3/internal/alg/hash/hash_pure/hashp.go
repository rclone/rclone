package hash_pure

import "github.com/zeebo/blake3/internal/alg/compress"

func HashP(left, right *[64]uint32, flags uint32, key *[8]uint32, out *[64]uint32, n int) {
	var tmp [16]uint32
	var block [16]uint32

	for i := 0; i < n && i < 8; i++ {
		block[0] = left[i+0]
		block[1] = left[i+8]
		block[2] = left[i+16]
		block[3] = left[i+24]
		block[4] = left[i+32]
		block[5] = left[i+40]
		block[6] = left[i+48]
		block[7] = left[i+56]
		block[8] = right[i+0]
		block[9] = right[i+8]
		block[10] = right[i+16]
		block[11] = right[i+24]
		block[12] = right[i+32]
		block[13] = right[i+40]
		block[14] = right[i+48]
		block[15] = right[i+56]

		compress.Compress(key, &block, 0, 64, flags, &tmp)

		out[i+0] = tmp[0]
		out[i+8] = tmp[1]
		out[i+16] = tmp[2]
		out[i+24] = tmp[3]
		out[i+32] = tmp[4]
		out[i+40] = tmp[5]
		out[i+48] = tmp[6]
		out[i+56] = tmp[7]
	}
}
