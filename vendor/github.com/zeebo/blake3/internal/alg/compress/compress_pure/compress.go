package compress_pure

import (
	"math/bits"

	"github.com/zeebo/blake3/internal/consts"
)

func Compress(
	chain *[8]uint32,
	block *[16]uint32,
	counter uint64,
	blen uint32,
	flags uint32,
	out *[16]uint32,
) {

	*out = [16]uint32{
		chain[0], chain[1], chain[2], chain[3],
		chain[4], chain[5], chain[6], chain[7],
		consts.IV0, consts.IV1, consts.IV2, consts.IV3,
		uint32(counter), uint32(counter >> 32), blen, flags,
	}

	rcompress(out, block)
}

func g(a, b, c, d, mx, my uint32) (uint32, uint32, uint32, uint32) {
	a += b + mx
	d = bits.RotateLeft32(d^a, -16)
	c += d
	b = bits.RotateLeft32(b^c, -12)
	a += b + my
	d = bits.RotateLeft32(d^a, -8)
	c += d
	b = bits.RotateLeft32(b^c, -7)
	return a, b, c, d
}

func rcompress(s *[16]uint32, m *[16]uint32) {
	const (
		a = 10
		b = 11
		c = 12
		d = 13
		e = 14
		f = 15
	)

	s0, s1, s2, s3 := s[0+0], s[0+1], s[0+2], s[0+3]
	s4, s5, s6, s7 := s[0+4], s[0+5], s[0+6], s[0+7]
	s8, s9, sa, sb := s[8+0], s[8+1], s[8+2], s[8+3]
	sc, sd, se, sf := s[8+4], s[8+5], s[8+6], s[8+7]

	s0, s4, s8, sc = g(s0, s4, s8, sc, m[0], m[1])
	s1, s5, s9, sd = g(s1, s5, s9, sd, m[2], m[3])
	s2, s6, sa, se = g(s2, s6, sa, se, m[4], m[5])
	s3, s7, sb, sf = g(s3, s7, sb, sf, m[6], m[7])
	s0, s5, sa, sf = g(s0, s5, sa, sf, m[8], m[9])
	s1, s6, sb, sc = g(s1, s6, sb, sc, m[a], m[b])
	s2, s7, s8, sd = g(s2, s7, s8, sd, m[c], m[d])
	s3, s4, s9, se = g(s3, s4, s9, se, m[e], m[f])

	s0, s4, s8, sc = g(s0, s4, s8, sc, m[2], m[6])
	s1, s5, s9, sd = g(s1, s5, s9, sd, m[3], m[a])
	s2, s6, sa, se = g(s2, s6, sa, se, m[7], m[0])
	s3, s7, sb, sf = g(s3, s7, sb, sf, m[4], m[d])
	s0, s5, sa, sf = g(s0, s5, sa, sf, m[1], m[b])
	s1, s6, sb, sc = g(s1, s6, sb, sc, m[c], m[5])
	s2, s7, s8, sd = g(s2, s7, s8, sd, m[9], m[e])
	s3, s4, s9, se = g(s3, s4, s9, se, m[f], m[8])

	s0, s4, s8, sc = g(s0, s4, s8, sc, m[3], m[4])
	s1, s5, s9, sd = g(s1, s5, s9, sd, m[a], m[c])
	s2, s6, sa, se = g(s2, s6, sa, se, m[d], m[2])
	s3, s7, sb, sf = g(s3, s7, sb, sf, m[7], m[e])
	s0, s5, sa, sf = g(s0, s5, sa, sf, m[6], m[5])
	s1, s6, sb, sc = g(s1, s6, sb, sc, m[9], m[0])
	s2, s7, s8, sd = g(s2, s7, s8, sd, m[b], m[f])
	s3, s4, s9, se = g(s3, s4, s9, se, m[8], m[1])

	s0, s4, s8, sc = g(s0, s4, s8, sc, m[a], m[7])
	s1, s5, s9, sd = g(s1, s5, s9, sd, m[c], m[9])
	s2, s6, sa, se = g(s2, s6, sa, se, m[e], m[3])
	s3, s7, sb, sf = g(s3, s7, sb, sf, m[d], m[f])
	s0, s5, sa, sf = g(s0, s5, sa, sf, m[4], m[0])
	s1, s6, sb, sc = g(s1, s6, sb, sc, m[b], m[2])
	s2, s7, s8, sd = g(s2, s7, s8, sd, m[5], m[8])
	s3, s4, s9, se = g(s3, s4, s9, se, m[1], m[6])

	s0, s4, s8, sc = g(s0, s4, s8, sc, m[c], m[d])
	s1, s5, s9, sd = g(s1, s5, s9, sd, m[9], m[b])
	s2, s6, sa, se = g(s2, s6, sa, se, m[f], m[a])
	s3, s7, sb, sf = g(s3, s7, sb, sf, m[e], m[8])
	s0, s5, sa, sf = g(s0, s5, sa, sf, m[7], m[2])
	s1, s6, sb, sc = g(s1, s6, sb, sc, m[5], m[3])
	s2, s7, s8, sd = g(s2, s7, s8, sd, m[0], m[1])
	s3, s4, s9, se = g(s3, s4, s9, se, m[6], m[4])

	s0, s4, s8, sc = g(s0, s4, s8, sc, m[9], m[e])
	s1, s5, s9, sd = g(s1, s5, s9, sd, m[b], m[5])
	s2, s6, sa, se = g(s2, s6, sa, se, m[8], m[c])
	s3, s7, sb, sf = g(s3, s7, sb, sf, m[f], m[1])
	s0, s5, sa, sf = g(s0, s5, sa, sf, m[d], m[3])
	s1, s6, sb, sc = g(s1, s6, sb, sc, m[0], m[a])
	s2, s7, s8, sd = g(s2, s7, s8, sd, m[2], m[6])
	s3, s4, s9, se = g(s3, s4, s9, se, m[4], m[7])

	s0, s4, s8, sc = g(s0, s4, s8, sc, m[b], m[f])
	s1, s5, s9, sd = g(s1, s5, s9, sd, m[5], m[0])
	s2, s6, sa, se = g(s2, s6, sa, se, m[1], m[9])
	s3, s7, sb, sf = g(s3, s7, sb, sf, m[8], m[6])
	s0, s5, sa, sf = g(s0, s5, sa, sf, m[e], m[a])
	s1, s6, sb, sc = g(s1, s6, sb, sc, m[2], m[c])
	s2, s7, s8, sd = g(s2, s7, s8, sd, m[3], m[4])
	s3, s4, s9, se = g(s3, s4, s9, se, m[7], m[d])

	s[8+0] = s8 ^ s[0]
	s[8+1] = s9 ^ s[1]
	s[8+2] = sa ^ s[2]
	s[8+3] = sb ^ s[3]
	s[8+4] = sc ^ s[4]
	s[8+5] = sd ^ s[5]
	s[8+6] = se ^ s[6]
	s[8+7] = sf ^ s[7]

	s[0] = s0 ^ s8
	s[1] = s1 ^ s9
	s[2] = s2 ^ sa
	s[3] = s3 ^ sb
	s[4] = s4 ^ sc
	s[5] = s5 ^ sd
	s[6] = s6 ^ se
	s[7] = s7 ^ sf
}
