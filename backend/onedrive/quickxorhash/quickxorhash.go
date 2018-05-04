// Package quickxorhash provides the quickXorHash algorithm which is a
// quick, simple non-cryptographic hash algorithm that works by XORing
// the bytes in a circular-shifting fashion.
//
// It is used by Microsoft Onedrive for Business to hash data.
//
// See: https://docs.microsoft.com/en-us/onedrive/developer/code-snippets/quickxorhash
package quickxorhash

// This code was ported from the code snippet linked from
// https://docs.microsoft.com/en-us/onedrive/developer/code-snippets/quickxorhash
// Which has the copyright

// ------------------------------------------------------------------------------
//  Copyright (c) 2016 Microsoft Corporation
//
//  Permission is hereby granted, free of charge, to any person obtaining a copy
//  of this software and associated documentation files (the "Software"), to deal
//  in the Software without restriction, including without limitation the rights
//  to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
//  copies of the Software, and to permit persons to whom the Software is
//  furnished to do so, subject to the following conditions:
//
//  The above copyright notice and this permission notice shall be included in
//  all copies or substantial portions of the Software.
//
//  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
//  IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
//  FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
//  AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
//  LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
//  OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
//  THE SOFTWARE.
// ------------------------------------------------------------------------------

import (
	"hash"
)

const (
	// BlockSize is the preferred size for hashing
	BlockSize = 64
	// Size of the output checksum
	Size           = 20
	bitsInLastCell = 32
	shift          = 11
	widthInBits    = 8 * Size
	dataSize       = (widthInBits-1)/64 + 1
)

type quickXorHash struct {
	data        [dataSize]uint64
	lengthSoFar uint64
	shiftSoFar  int
}

// New returns a new hash.Hash computing the quickXorHash checksum.
func New() hash.Hash {
	return &quickXorHash{}
}

// Write (via the embedded io.Writer interface) adds more data to the running hash.
// It never returns an error.
//
// Write writes len(p) bytes from p to the underlying data stream. It returns
// the number of bytes written from p (0 <= n <= len(p)) and any error
// encountered that caused the write to stop early. Write must return a non-nil
// error if it returns n < len(p). Write must not modify the slice data, even
// temporarily.
//
// Implementations must not retain p.
func (q *quickXorHash) Write(p []byte) (n int, err error) {
	currentshift := q.shiftSoFar

	// The bitvector where we'll start xoring
	vectorArrayIndex := currentshift / 64

	// The position within the bit vector at which we begin xoring
	vectorOffset := currentshift % 64
	iterations := len(p)
	if iterations > widthInBits {
		iterations = widthInBits
	}

	for i := 0; i < iterations; i++ {
		isLastCell := vectorArrayIndex == len(q.data)-1
		var bitsInVectorCell int
		if isLastCell {
			bitsInVectorCell = bitsInLastCell
		} else {
			bitsInVectorCell = 64
		}

		// There's at least 2 bitvectors before we reach the end of the array
		if vectorOffset <= bitsInVectorCell-8 {
			for j := i; j < len(p); j += widthInBits {
				q.data[vectorArrayIndex] ^= uint64(p[j]) << uint(vectorOffset)
			}
		} else {
			index1 := vectorArrayIndex
			var index2 int
			if isLastCell {
				index2 = 0
			} else {
				index2 = vectorArrayIndex + 1
			}
			low := byte(bitsInVectorCell - vectorOffset)

			xoredByte := byte(0)
			for j := i; j < len(p); j += widthInBits {
				xoredByte ^= p[j]
			}
			q.data[index1] ^= uint64(xoredByte) << uint(vectorOffset)
			q.data[index2] ^= uint64(xoredByte) >> low
		}
		vectorOffset += shift
		for vectorOffset >= bitsInVectorCell {
			if isLastCell {
				vectorArrayIndex = 0
			} else {
				vectorArrayIndex = vectorArrayIndex + 1
			}
			vectorOffset -= bitsInVectorCell
		}
	}

	// Update the starting position in a circular shift pattern
	q.shiftSoFar = (q.shiftSoFar + shift*(len(p)%widthInBits)) % widthInBits

	q.lengthSoFar += uint64(len(p))

	return len(p), nil
}

// Calculate the current checksum
func (q *quickXorHash) checkSum() (h [Size]byte) {
	// Output the data as little endian bytes
	ph := 0
	for _, d := range q.data[:len(q.data)-1] {
		_ = h[ph+7] // bounds check
		h[ph+0] = byte(d >> (8 * 0))
		h[ph+1] = byte(d >> (8 * 1))
		h[ph+2] = byte(d >> (8 * 2))
		h[ph+3] = byte(d >> (8 * 3))
		h[ph+4] = byte(d >> (8 * 4))
		h[ph+5] = byte(d >> (8 * 5))
		h[ph+6] = byte(d >> (8 * 6))
		h[ph+7] = byte(d >> (8 * 7))
		ph += 8
	}
	// remaining 32 bits
	d := q.data[len(q.data)-1]
	h[Size-4] = byte(d >> (8 * 0))
	h[Size-3] = byte(d >> (8 * 1))
	h[Size-2] = byte(d >> (8 * 2))
	h[Size-1] = byte(d >> (8 * 3))

	// XOR the file length with the least significant bits in little endian format
	d = q.lengthSoFar
	h[Size-8] ^= byte(d >> (8 * 0))
	h[Size-7] ^= byte(d >> (8 * 1))
	h[Size-6] ^= byte(d >> (8 * 2))
	h[Size-5] ^= byte(d >> (8 * 3))
	h[Size-4] ^= byte(d >> (8 * 4))
	h[Size-3] ^= byte(d >> (8 * 5))
	h[Size-2] ^= byte(d >> (8 * 6))
	h[Size-1] ^= byte(d >> (8 * 7))

	return h
}

// Sum appends the current hash to b and returns the resulting slice.
// It does not change the underlying hash state.
func (q *quickXorHash) Sum(b []byte) []byte {
	hash := q.checkSum()
	return append(b, hash[:]...)
}

// Reset resets the Hash to its initial state.
func (q *quickXorHash) Reset() {
	*q = quickXorHash{}
}

// Size returns the number of bytes Sum will return.
func (q *quickXorHash) Size() int {
	return Size
}

// BlockSize returns the hash's underlying block size.
// The Write method must be able to accept any amount
// of data, but it may operate more efficiently if all writes
// are a multiple of the block size.
func (q *quickXorHash) BlockSize() int {
	return BlockSize
}

// Sum returns the quickXorHash checksum of the data.
func Sum(data []byte) [Size]byte {
	var d quickXorHash
	_, _ = d.Write(data)
	return d.checkSum()
}
