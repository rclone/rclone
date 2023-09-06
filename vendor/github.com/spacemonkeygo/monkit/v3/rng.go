// Copyright (C) 2016 Space Monkey, Inc.

package monkit

import (
	"math/rand"
)

// lcg is a simple linear congruential generator based on Knuths MMIX.
type lcg uint64

// Make sure lcg is a rand.Source
var _ rand.Source = (*lcg)(nil)

func newLCG() lcg { return lcg(rand.Int63()) }

// See Knuth.
const (
	a = 6364136223846793005
	c = 1442695040888963407
	h = 0xffffffff00000000
)

// Uint64 returns a uint64.
func (l *lcg) Uint64() (ret uint64) {
	*l = a**l + c
	ret |= uint64(*l) >> 32
	*l = a**l + c
	ret |= uint64(*l) & h
	return
}

// Int63 returns a positive 63 bit integer in an int64
func (l *lcg) Int63() int64 {
	return int64(l.Uint64() >> 1)
}

// Seed sets the state of the lcg.
func (l *lcg) Seed(seed int64) {
	*l = lcg(seed)
}

//
// xorshift family of generators from https://en.wikipedia.org/wiki/Xorshift
//
// xorshift64   is the xorshift64* generator
// xorshift1024 is the xorshift1024* generator
// xorshift128  is the xorshift128+ generator
//

type xorshift64 uint64

var _ rand.Source = (*xorshift64)(nil)

func newXORShift64() xorshift64 { return xorshift64(rand.Int63()) }

// Uint64 returns a uint64.
func (s *xorshift64) Uint64() (ret uint64) {
	x := uint64(*s)
	x ^= x >> 12 // a
	x ^= x << 25 // b
	x ^= x >> 27 // c
	x *= 2685821657736338717
	*s = xorshift64(x)
	return x
}

// Int63 returns a positive 63 bit integer in an int64
func (s *xorshift64) Int63() int64 {
	return int64(s.Uint64() >> 1)
}

// Seed sets the state of the lcg.
func (s *xorshift64) Seed(seed int64) {
	*s = xorshift64(seed)
}

type xorshift1024 struct {
	s [16]uint64
	p int
}

var _ rand.Source = (*xorshift1024)(nil)

func newXORShift1024() xorshift1024 {
	var x xorshift1024
	x.Seed(rand.Int63())
	return x
}

// Seed sets the state of the lcg.
func (s *xorshift1024) Seed(seed int64) {
	rng := xorshift64(seed)
	*s = xorshift1024{
		s: [16]uint64{
			rng.Uint64(), rng.Uint64(), rng.Uint64(), rng.Uint64(),
			rng.Uint64(), rng.Uint64(), rng.Uint64(), rng.Uint64(),
			rng.Uint64(), rng.Uint64(), rng.Uint64(), rng.Uint64(),
			rng.Uint64(), rng.Uint64(), rng.Uint64(), rng.Uint64(),
		},
		p: 0,
	}
}

// Int63 returns a positive 63 bit integer in an int64
func (s *xorshift1024) Int63() int64 {
	return int64(s.Uint64() >> 1)
}

// Uint64 returns a uint64.
func (s *xorshift1024) Uint64() (ret uint64) {
	// factoring this out proves to SSA backend that the array checks below
	// do not need bounds checks
	p := s.p & 15
	s0 := s.s[p]
	p = (p + 1) & 15
	s.p = p
	s1 := s.s[p]
	s1 ^= s1 << 31
	s.s[p] = s1 ^ s0 ^ (s1 >> 1) ^ (s0 >> 30)
	return s.s[p] * 1181783497276652981
}

// Jump is used to advance the state 2^512 iterations.
func (s *xorshift1024) Jump() {
	var t [16]uint64
	for i := 0; i < 16; i++ {
		for b := uint(0); b < 64; b++ {
			if (xorshift1024jump[i] & (1 << b)) > 0 {
				for j := 0; j < 16; j++ {
					t[j] ^= s.s[(j+s.p)&15]
				}
			}
			_ = s.Uint64()
		}
	}
	for j := 0; j < 16; j++ {
		s.s[(j+s.p)&15] = t[j]
	}
}

var xorshift1024jump = [16]uint64{
	0x84242f96eca9c41d, 0xa3c65b8776f96855, 0x5b34a39f070b5837,
	0x4489affce4f31a1e, 0x2ffeeb0a48316f40, 0xdc2d9891fe68c022,
	0x3659132bb12fea70, 0xaac17d8efa43cab8, 0xc4cb815590989b13,
	0x5ee975283d71c93b, 0x691548c86c1bd540, 0x7910c41d10a1e6a5,
	0x0b5fc64563b3e2a8, 0x047f7684e9fc949d, 0xb99181f2d8f685ca,
	0x284600e3f30e38c3,
}

type xorshift128 [2]uint64

var _ rand.Source = (*xorshift128)(nil)

func newXORShift128() xorshift128 {
	var s xorshift128
	s.Seed(rand.Int63())
	return s
}

func (s *xorshift128) Seed(seed int64) {
	rng := xorshift64(seed)
	*s = xorshift128{
		rng.Uint64(), rng.Uint64(),
	}
}

// Int63 returns a positive 63 bit integer in an int64
func (s *xorshift128) Int63() int64 {
	return int64(s.Uint64() >> 1)
}

// Uint64 returns a uint64.
func (s *xorshift128) Uint64() (ret uint64) {
	x := s[0]
	y := s[1]
	s[0] = y
	x ^= x << 23
	s[1] = x ^ y ^ (x >> 17) ^ (y >> 26)
	return s[1] + y
}
