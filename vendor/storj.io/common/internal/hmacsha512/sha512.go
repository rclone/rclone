// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package hmacsha512

import (
	"encoding/binary"
)

const chunk = 128

// BlockSize is the block size of SHA-512 hash function.
const BlockSize = 128

// Size is the size of SHA-512 checksum in bytes.
const Size = 64

// digest is the sha512 digest.
type digest struct {
	h   [8]uint64
	x   [chunk]byte
	nx  int
	len uint64
}

// Reset resets the sha512 digest to initial state.
func (d *digest) Reset() {
	d.nx = 0
	d.len = 0
	d.h = [...]uint64{
		0x6a09e667f3bcc908,
		0xbb67ae8584caa73b,
		0x3c6ef372fe94f82b,
		0xa54ff53a5f1d36f1,
		0x510e527fade682d1,
		0x9b05688c2b3e6c1f,
		0x1f83d9abfb41bd6b,
		0x5be0cd19137e2179,
	}
}

// Write appends data to the hash calculation.
func (d *digest) Write(p []byte) {
	d.len += uint64(len(p))
	if d.nx > 0 {
		n := copy(d.x[d.nx:], p)
		d.nx += n
		if d.nx == chunk {
			block(d, d.x[:])
			d.nx = 0
		}
		p = p[n:]
	}
	if len(p) >= chunk {
		n := len(p) &^ (chunk - 1)
		block(d, p[:n])
		p = p[n:]
	}
	if len(p) > 0 {
		d.nx = copy(d.x[:], p)
	}
}

// FinishAndSum finishes the hash calculation.
// It's not safe to call Write after calling this function.
func (d *digest) FinishAndSum() [Size]byte {
	// Padding. Add a 1 bit and 0 bits until 112 bytes mod 128.
	len := d.len
	var tmp [128 + 16]byte
	tmp[0] = 0x80
	var t uint64
	if len%128 < 112 {
		t = 112 - len%128
	} else {
		t = 128 + 112 - len%128
	}

	// Length in bits.
	len <<= 3
	padlen := tmp[:t+16]
	// binary.BigEndian.PutUint64(padlen[t+0:], 0) // upper 64 bits are always zero, because len variable has type uint64
	binary.BigEndian.PutUint64(padlen[t+8:], len)
	d.Write(padlen)

	if d.nx != 0 {
		panic("d.nx != 0")
	}

	var digest [Size]byte
	binary.BigEndian.PutUint64(digest[0:], d.h[0])
	binary.BigEndian.PutUint64(digest[8:], d.h[1])
	binary.BigEndian.PutUint64(digest[16:], d.h[2])
	binary.BigEndian.PutUint64(digest[24:], d.h[3])
	binary.BigEndian.PutUint64(digest[32:], d.h[4])
	binary.BigEndian.PutUint64(digest[40:], d.h[5])
	binary.BigEndian.PutUint64(digest[48:], d.h[6])
	binary.BigEndian.PutUint64(digest[56:], d.h[7])

	return digest
}
