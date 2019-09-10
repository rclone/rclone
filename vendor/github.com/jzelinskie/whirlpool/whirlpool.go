// Copyright 2012 Jimmy Zelinskie. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package whirlpool implements the ISO/IEC 10118-3:2004 whirlpool
// cryptographic hash. Whirlpool is defined in
// http://www.larc.usp.br/~pbarreto/WhirlpoolPage.html
package whirlpool

import (
	"encoding/binary"
	"hash"
)

// whirlpool represents the partial evaluation of a checksum.
type whirlpool struct {
	bitLength  [lengthBytes]byte       // Number of hashed bits.
	buffer     [wblockBytes]byte       // Buffer of data to be hashed.
	bufferBits int                     // Current number of bits on the buffer.
	bufferPos  int                     // Current byte location on buffer.
	hash       [digestBytes / 8]uint64 // Hash state.
}

// New returns a new hash.Hash computing the whirlpool checksum.
func New() hash.Hash {
	return new(whirlpool)
}

func (w *whirlpool) Reset() {
	// Cleanup the buffer.
	w.buffer = [wblockBytes]byte{}
	w.bufferBits = 0
	w.bufferPos = 0

	// Cleanup the digest.
	w.hash = [digestBytes / 8]uint64{}

	// Clean up the number of hashed bits.
	w.bitLength = [lengthBytes]byte{}
}

func (w *whirlpool) Size() int {
	return digestBytes
}

func (w *whirlpool) BlockSize() int {
	return wblockBytes
}

func (w *whirlpool) transform() {
	var (
		K     [8]uint64 // Round key.
		block [8]uint64 // μ(buffer).
		state [8]uint64 // Cipher state.
		L     [8]uint64
	)

	// Map the buffer to a block.
	for i := 0; i < 8; i++ {
		b := 8 * i
		block[i] = binary.BigEndian.Uint64(w.buffer[b:])
	}

	// Compute & apply K^0 to the cipher state.
	for i := 0; i < 8; i++ {
		K[i] = w.hash[i]
		state[i] = block[i] ^ K[i]
	}

	// Iterate over all the rounds.
	for r := 1; r <= rounds; r++ {
		// Compute K^rounds from K^(rounds-1).
		for i := 0; i < 8; i++ {
			L[i] = _C0[byte(K[i%8]>>56)] ^
				_C1[byte(K[(i+7)%8]>>48)] ^
				_C2[byte(K[(i+6)%8]>>40)] ^
				_C3[byte(K[(i+5)%8]>>32)] ^
				_C4[byte(K[(i+4)%8]>>24)] ^
				_C5[byte(K[(i+3)%8]>>16)] ^
				_C6[byte(K[(i+2)%8]>>8)] ^
				_C7[byte(K[(i+1)%8])]
		}
		L[0] ^= rc[r]

		for i := 0; i < 8; i++ {
			K[i] = L[i]
		}

		// Apply r-th round transformation.
		for i := 0; i < 8; i++ {
			L[i] = _C0[byte(state[i%8]>>56)] ^
				_C1[byte(state[(i+7)%8]>>48)] ^
				_C2[byte(state[(i+6)%8]>>40)] ^
				_C3[byte(state[(i+5)%8]>>32)] ^
				_C4[byte(state[(i+4)%8]>>24)] ^
				_C5[byte(state[(i+3)%8]>>16)] ^
				_C6[byte(state[(i+2)%8]>>8)] ^
				_C7[byte(state[(i+1)%8])] ^
				K[i%8]
		}

		for i := 0; i < 8; i++ {
			state[i] = L[i]
		}
	}

	// Apply the Miyaguchi-Preneel compression function.
	for i := 0; i < 8; i++ {
		w.hash[i] ^= state[i] ^ block[i]
	}
}

func (w *whirlpool) Write(source []byte) (int, error) {
	var (
		sourcePos  int                                            // Index of the leftmost source.
		nn         int    = len(source)                           // Num of bytes to process.
		sourceBits uint64 = uint64(nn * 8)                        // Num of bits to process.
		sourceGap  uint   = uint((8 - (int(sourceBits & 7))) & 7) // Space on source[sourcePos].
		bufferRem  uint   = uint(w.bufferBits & 7)                // Occupied bits on buffer[bufferPos].
		b          uint32                                         // Current byte.
	)

	// Tally the length of the data added.
	for i, carry, value := 31, uint32(0), uint64(sourceBits); i >= 0 && (carry != 0 || value != 0); i-- {
		carry += uint32(w.bitLength[i]) + (uint32(value & 0xff))
		w.bitLength[i] = byte(carry)
		carry >>= 8
		value >>= 8
	}

	// Process data in chunks of 8 bits.
	for sourceBits > 8 {
		// Take a byte form the source.
		b = uint32(((source[sourcePos] << sourceGap) & 0xff) |
			((source[sourcePos+1] & 0xff) >> (8 - sourceGap)))

		// Process this byte.
		w.buffer[w.bufferPos] |= uint8(b >> bufferRem)
		w.bufferPos++
		w.bufferBits += int(8 - bufferRem)

		if w.bufferBits == digestBits {
			// Process this block.
			w.transform()
			// Reset the buffer.
			w.bufferBits = 0
			w.bufferPos = 0
		}
		w.buffer[w.bufferPos] = byte(b << (8 - bufferRem))
		w.bufferBits += int(bufferRem)

		// Proceed to remaining data.
		sourceBits -= 8
		sourcePos++
	}

	// 0 <= sourceBits <= 8; All data leftover is in source[sourcePos].
	if sourceBits > 0 {
		b = uint32((source[sourcePos] << sourceGap) & 0xff) // The bits are left-justified.

		// Process the remaining bits.
		w.buffer[w.bufferPos] |= byte(b) >> bufferRem
	} else {
		b = 0
	}

	if uint64(bufferRem)+sourceBits < 8 {
		// The remaining data fits on the buffer[bufferPos].
		w.bufferBits += int(sourceBits)
	} else {
		// The buffer[bufferPos] is full.
		w.bufferPos++
		w.bufferBits += 8 - int(bufferRem) // bufferBits = 8*bufferPos
		sourceBits -= uint64(8 - bufferRem)

		// Now, 0 <= sourceBits <= 8; all data leftover is in source[sourcePos].
		if w.bufferBits == digestBits {
			// Process this data block.
			w.transform()
			// Reset buffer.
			w.bufferBits = 0
			w.bufferPos = 0
		}
		w.buffer[w.bufferPos] = byte(b << (8 - bufferRem))
		w.bufferBits += int(sourceBits)
	}
	return nn, nil
}

func (w *whirlpool) Sum(in []byte) []byte {
	// Copy the whirlpool so that the caller can keep summing.
	n := *w

	// Append a 1-bit.
	n.buffer[n.bufferPos] |= 0x80 >> (uint(n.bufferBits) & 7)
	n.bufferPos++

	// The remaining bits should be 0. Pad with 0s to be complete.
	if n.bufferPos > wblockBytes-lengthBytes {
		if n.bufferPos < wblockBytes {
			for i := 0; i < wblockBytes-n.bufferPos; i++ {
				n.buffer[n.bufferPos+i] = 0
			}
		}
		// Process this data block.
		n.transform()
		// Reset the buffer.
		n.bufferPos = 0
	}

	if n.bufferPos < wblockBytes-lengthBytes {
		for i := 0; i < (wblockBytes-lengthBytes)-n.bufferPos; i++ {
			n.buffer[n.bufferPos+i] = 0
		}
	}
	n.bufferPos = wblockBytes - lengthBytes

	// Append the bit length of the hashed data.
	for i := 0; i < lengthBytes; i++ {
		n.buffer[n.bufferPos+i] = n.bitLength[i]
	}

	// Process this data block.
	n.transform()

	// Return the final digest as []byte.
	var digest [digestBytes]byte
	for i := 0; i < digestBytes/8; i++ {
		digest[i*8] = byte(n.hash[i] >> 56)
		digest[i*8+1] = byte(n.hash[i] >> 48)
		digest[i*8+2] = byte(n.hash[i] >> 40)
		digest[i*8+3] = byte(n.hash[i] >> 32)
		digest[i*8+4] = byte(n.hash[i] >> 24)
		digest[i*8+5] = byte(n.hash[i] >> 16)
		digest[i*8+6] = byte(n.hash[i] >> 8)
		digest[i*8+7] = byte(n.hash[i])
	}

	return append(in, digest[:digestBytes]...)
}
