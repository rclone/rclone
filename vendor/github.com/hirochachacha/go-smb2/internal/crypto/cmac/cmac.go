// Copyright 2009 The Go Authors. All rights reserved.
// Portions Copyright 2016 Hiroshi Ioka. All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//    * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//    * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

// CMAC message authentication code, defined in
// NIST Special Publication SP 800-38B.

package cmac

import (
	"crypto/cipher"
	"hash"
)

const (
	// minimal irreducible polynomial of degree b
	r64  = 0x1b
	r128 = 0x87
)

type cmac struct {
	k1, k2, ci, digest []byte
	p                  int // position in ci
	c                  cipher.Block
}

// TODO(rsc): Should this return an error instead of panic?

// NewCMAC returns a new instance of a CMAC message authentication code
// digest using the given Cipher.
func New(c cipher.Block) hash.Hash {
	var r byte
	n := c.BlockSize()
	switch n {
	case 64 / 8:
		r = r64
	case 128 / 8:
		r = r128
	default:
		panic("crypto/cmac: NewCMAC: invalid cipher block size")
	}

	d := new(cmac)
	d.c = c
	d.k1 = make([]byte, n)
	d.k2 = make([]byte, n)
	d.ci = make([]byte, n)
	d.digest = make([]byte, n)

	// Subkey generation, p. 7
	c.Encrypt(d.k1, d.k1)
	if shift1(d.k1, d.k1) != 0 {
		d.k1[n-1] ^= r
	}
	if shift1(d.k1, d.k2) != 0 {
		d.k2[n-1] ^= r
	}

	return d
}

// Reset clears the digest state, starting a new digest.
func (d *cmac) Reset() {
	for i := range d.ci {
		d.ci[i] = 0
	}
	d.p = 0
}

// Write adds the given data to the digest state.
func (d *cmac) Write(p []byte) (n int, err error) {
	// Xor input into ci.
	for _, c := range p {
		// If ci is full, encrypt and start over.
		if d.p >= len(d.ci) {
			d.c.Encrypt(d.ci, d.ci)
			d.p = 0
		}
		d.ci[d.p] ^= c
		d.p++
	}
	return len(p), nil
}

// Sum returns the CMAC digest, one cipher block in length,
// of the data written with Write.
func (d *cmac) Sum(in []byte) []byte {
	// Finish last block, mix in key, encrypt.
	// Don't edit ci, in case caller wants
	// to keep digesting after call to Sum.
	k := d.k1
	if d.p < len(d.digest) {
		k = d.k2
	}
	for i := 0; i < len(d.ci); i++ {
		d.digest[i] = d.ci[i] ^ k[i]
	}
	if d.p < len(d.digest) {
		d.digest[d.p] ^= 0x80
	}
	d.c.Encrypt(d.digest, d.digest)
	return append(in, d.digest...)
}

func (d *cmac) Size() int { return len(d.digest) }

func (d *cmac) BlockSize() int { return 16 }

// Utility routines

func shift1(src, dst []byte) byte {
	var b byte
	for i := len(src) - 1; i >= 0; i-- {
		bb := src[i] >> 7
		dst[i] = src[i]<<1 | b
		b = bb
	}
	return b
}
