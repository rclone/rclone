package xxhash

func u32(in []byte) uint32 {
	return uint32(in[0]) | uint32(in[1])<<8 | uint32(in[2])<<16 | uint32(in[3])<<24
}

func u64(in []byte) uint64 {
	return uint64(in[0]) | uint64(in[1])<<8 | uint64(in[2])<<16 | uint64(in[3])<<24 | uint64(in[4])<<32 | uint64(in[5])<<40 | uint64(in[6])<<48 | uint64(in[7])<<56
}

// Checksum32S returns the checksum of the input bytes with the specific seed.
func Checksum32S(in []byte, seed uint32) (h uint32) {
	var i int

	if len(in) > 15 {
		var (
			v1 = seed + prime32x1 + prime32x2
			v2 = seed + prime32x2
			v3 = seed + 0
			v4 = seed - prime32x1
		)
		for ; i < len(in)-15; i += 16 {
			in := in[i : i+16 : len(in)]
			v1 += u32(in[0:4:len(in)]) * prime32x2
			v1 = rotl32_13(v1) * prime32x1

			v2 += u32(in[4:8:len(in)]) * prime32x2
			v2 = rotl32_13(v2) * prime32x1

			v3 += u32(in[8:12:len(in)]) * prime32x2
			v3 = rotl32_13(v3) * prime32x1

			v4 += u32(in[12:16:len(in)]) * prime32x2
			v4 = rotl32_13(v4) * prime32x1
		}

		h = rotl32_1(v1) + rotl32_7(v2) + rotl32_12(v3) + rotl32_18(v4)

	} else {
		h = seed + prime32x5
	}

	h += uint32(len(in))
	for ; i <= len(in)-4; i += 4 {
		in := in[i : i+4 : len(in)]
		h += u32(in[0:4:len(in)]) * prime32x3
		h = rotl32_17(h) * prime32x4
	}

	for ; i < len(in); i++ {
		h += uint32(in[i]) * prime32x5
		h = rotl32_11(h) * prime32x1
	}

	h ^= h >> 15
	h *= prime32x2
	h ^= h >> 13
	h *= prime32x3
	h ^= h >> 16

	return
}

func (xx *XXHash32) Write(in []byte) (n int, err error) {
	i, ml := 0, int(xx.memIdx)
	n = len(in)
	xx.ln += int32(n)

	if d := 16 - ml; ml > 0 && ml+len(in) > 16 {
		xx.memIdx += int32(copy(xx.mem[xx.memIdx:], in[:d]))
		ml, in = 16, in[d:len(in):len(in)]
	} else if ml+len(in) < 16 {
		xx.memIdx += int32(copy(xx.mem[xx.memIdx:], in))
		return
	}

	if ml > 0 {
		i += 16 - ml
		xx.memIdx += int32(copy(xx.mem[xx.memIdx:len(xx.mem):len(xx.mem)], in))
		in := xx.mem[:16:len(xx.mem)]

		xx.v1 += u32(in[0:4:len(in)]) * prime32x2
		xx.v1 = rotl32_13(xx.v1) * prime32x1

		xx.v2 += u32(in[4:8:len(in)]) * prime32x2
		xx.v2 = rotl32_13(xx.v2) * prime32x1

		xx.v3 += u32(in[8:12:len(in)]) * prime32x2
		xx.v3 = rotl32_13(xx.v3) * prime32x1

		xx.v4 += u32(in[12:16:len(in)]) * prime32x2
		xx.v4 = rotl32_13(xx.v4) * prime32x1

		xx.memIdx = 0
	}

	for ; i <= len(in)-16; i += 16 {
		in := in[i : i+16 : len(in)]
		xx.v1 += u32(in[0:4:len(in)]) * prime32x2
		xx.v1 = rotl32_13(xx.v1) * prime32x1

		xx.v2 += u32(in[4:8:len(in)]) * prime32x2
		xx.v2 = rotl32_13(xx.v2) * prime32x1

		xx.v3 += u32(in[8:12:len(in)]) * prime32x2
		xx.v3 = rotl32_13(xx.v3) * prime32x1

		xx.v4 += u32(in[12:16:len(in)]) * prime32x2
		xx.v4 = rotl32_13(xx.v4) * prime32x1
	}

	if len(in)-i != 0 {
		xx.memIdx += int32(copy(xx.mem[xx.memIdx:], in[i:len(in):len(in)]))
	}

	return
}

func (xx *XXHash32) Sum32() (h uint32) {
	var i int32
	if xx.ln > 15 {
		h = rotl32_1(xx.v1) + rotl32_7(xx.v2) + rotl32_12(xx.v3) + rotl32_18(xx.v4)
	} else {
		h = xx.seed + prime32x5
	}

	h += uint32(xx.ln)

	if xx.memIdx > 0 {
		for ; i < xx.memIdx-3; i += 4 {
			in := xx.mem[i : i+4 : len(xx.mem)]
			h += u32(in[0:4:len(in)]) * prime32x3
			h = rotl32_17(h) * prime32x4
		}

		for ; i < xx.memIdx; i++ {
			h += uint32(xx.mem[i]) * prime32x5
			h = rotl32_11(h) * prime32x1
		}
	}
	h ^= h >> 15
	h *= prime32x2
	h ^= h >> 13
	h *= prime32x3
	h ^= h >> 16

	return
}

// Checksum64S returns the 64bit xxhash checksum for a single input
func Checksum64S(in []byte, seed uint64) uint64 {
	if len(in) == 0 && seed == 0 {
		return 0xef46db3751d8e999
	}

	if len(in) > 31 {
		return checksum64(in, seed)
	}

	return checksum64Short(in, seed)
}
