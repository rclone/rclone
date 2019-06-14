// +build appengine safe ppc64le ppc64be mipsle mips s390x

package xxhash

// Backend returns the current version of xxhash being used.
const Backend = "GoSafe"

func ChecksumString32S(s string, seed uint32) uint32 {
	return Checksum32S([]byte(s), seed)
}

func (xx *XXHash32) WriteString(s string) (int, error) {
	if len(s) == 0 {
		return 0, nil
	}
	return xx.Write([]byte(s))
}

func ChecksumString64S(s string, seed uint64) uint64 {
	return Checksum64S([]byte(s), seed)
}

func (xx *XXHash64) WriteString(s string) (int, error) {
	if len(s) == 0 {
		return 0, nil
	}
	return xx.Write([]byte(s))
}

func checksum64(in []byte, seed uint64) (h uint64) {
	var (
		v1, v2, v3, v4 = resetVs64(seed)

		i int
	)

	for ; i < len(in)-31; i += 32 {
		in := in[i : i+32 : len(in)]
		v1 = round64(v1, u64(in[0:8:len(in)]))
		v2 = round64(v2, u64(in[8:16:len(in)]))
		v3 = round64(v3, u64(in[16:24:len(in)]))
		v4 = round64(v4, u64(in[24:32:len(in)]))
	}

	h = rotl64_1(v1) + rotl64_7(v2) + rotl64_12(v3) + rotl64_18(v4)

	h = mergeRound64(h, v1)
	h = mergeRound64(h, v2)
	h = mergeRound64(h, v3)
	h = mergeRound64(h, v4)

	h += uint64(len(in))

	for ; i < len(in)-7; i += 8 {
		h ^= round64(0, u64(in[i:len(in):len(in)]))
		h = rotl64_27(h)*prime64x1 + prime64x4
	}

	for ; i < len(in)-3; i += 4 {
		h ^= uint64(u32(in[i:len(in):len(in)])) * prime64x1
		h = rotl64_23(h)*prime64x2 + prime64x3
	}

	for ; i < len(in); i++ {
		h ^= uint64(in[i]) * prime64x5
		h = rotl64_11(h) * prime64x1
	}

	return mix64(h)
}

func checksum64Short(in []byte, seed uint64) uint64 {
	var (
		h = seed + prime64x5 + uint64(len(in))
		i int
	)

	for ; i < len(in)-7; i += 8 {
		k := u64(in[i : i+8 : len(in)])
		h ^= round64(0, k)
		h = rotl64_27(h)*prime64x1 + prime64x4
	}

	for ; i < len(in)-3; i += 4 {
		h ^= uint64(u32(in[i:i+4:len(in)])) * prime64x1
		h = rotl64_23(h)*prime64x2 + prime64x3
	}

	for ; i < len(in); i++ {
		h ^= uint64(in[i]) * prime64x5
		h = rotl64_11(h) * prime64x1
	}

	return mix64(h)
}

func (xx *XXHash64) Write(in []byte) (n int, err error) {
	var (
		ml = int(xx.memIdx)
		d  = 32 - ml
	)

	n = len(in)
	xx.ln += uint64(n)

	if ml+len(in) < 32 {
		xx.memIdx += int8(copy(xx.mem[xx.memIdx:len(xx.mem):len(xx.mem)], in))
		return
	}

	i, v1, v2, v3, v4 := 0, xx.v1, xx.v2, xx.v3, xx.v4
	if ml > 0 && ml+len(in) > 32 {
		xx.memIdx += int8(copy(xx.mem[xx.memIdx:len(xx.mem):len(xx.mem)], in[:d:len(in)]))
		in = in[d:len(in):len(in)]

		in := xx.mem[0:32:len(xx.mem)]

		v1 = round64(v1, u64(in[0:8:len(in)]))
		v2 = round64(v2, u64(in[8:16:len(in)]))
		v3 = round64(v3, u64(in[16:24:len(in)]))
		v4 = round64(v4, u64(in[24:32:len(in)]))

		xx.memIdx = 0
	}

	for ; i < len(in)-31; i += 32 {
		in := in[i : i+32 : len(in)]
		v1 = round64(v1, u64(in[0:8:len(in)]))
		v2 = round64(v2, u64(in[8:16:len(in)]))
		v3 = round64(v3, u64(in[16:24:len(in)]))
		v4 = round64(v4, u64(in[24:32:len(in)]))
	}

	if len(in)-i != 0 {
		xx.memIdx += int8(copy(xx.mem[xx.memIdx:], in[i:len(in):len(in)]))
	}

	xx.v1, xx.v2, xx.v3, xx.v4 = v1, v2, v3, v4

	return
}

func (xx *XXHash64) Sum64() (h uint64) {
	var i int
	if xx.ln > 31 {
		v1, v2, v3, v4 := xx.v1, xx.v2, xx.v3, xx.v4
		h = rotl64_1(v1) + rotl64_7(v2) + rotl64_12(v3) + rotl64_18(v4)

		h = mergeRound64(h, v1)
		h = mergeRound64(h, v2)
		h = mergeRound64(h, v3)
		h = mergeRound64(h, v4)
	} else {
		h = xx.seed + prime64x5
	}

	h += uint64(xx.ln)
	if xx.memIdx > 0 {
		in := xx.mem[:xx.memIdx]
		for ; i < int(xx.memIdx)-7; i += 8 {
			in := in[i : i+8 : len(in)]
			k := u64(in[0:8:len(in)])
			k *= prime64x2
			k = rotl64_31(k)
			k *= prime64x1
			h ^= k
			h = rotl64_27(h)*prime64x1 + prime64x4
		}

		for ; i < int(xx.memIdx)-3; i += 4 {
			in := in[i : i+4 : len(in)]
			h ^= uint64(u32(in[0:4:len(in)])) * prime64x1
			h = rotl64_23(h)*prime64x2 + prime64x3
		}

		for ; i < int(xx.memIdx); i++ {
			h ^= uint64(in[i]) * prime64x5
			h = rotl64_11(h) * prime64x1
		}
	}

	return mix64(h)
}
