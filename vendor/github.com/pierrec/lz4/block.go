package lz4

import (
	"encoding/binary"
	"fmt"
	"math/bits"
)

// blockHash hashes the lower 6 bytes into a value < htSize.
func blockHash(x uint64) uint32 {
	const prime6bytes = 227718039650203
	return uint32(((x << (64 - 48)) * prime6bytes) >> (64 - hashLog))
}

// CompressBlockBound returns the maximum size of a given buffer of size n, when not compressible.
func CompressBlockBound(n int) int {
	return n + n/255 + 16
}

// UncompressBlock uncompresses the source buffer into the destination one,
// and returns the uncompressed size.
//
// The destination buffer must be sized appropriately.
//
// An error is returned if the source data is invalid or the destination buffer is too small.
func UncompressBlock(src, dst []byte) (int, error) {
	if len(src) == 0 {
		return 0, nil
	}
	if di := decodeBlock(dst, src); di >= 0 {
		return di, nil
	}
	return 0, ErrInvalidSourceShortBuffer
}

// CompressBlock compresses the source buffer into the destination one.
// This is the fast version of LZ4 compression and also the default one.
// The size of hashTable must be at least 64Kb.
//
// The size of the compressed data is returned. If it is 0 and no error, then the data is incompressible.
//
// An error is returned if the destination buffer is too small.
func CompressBlock(src, dst []byte, hashTable []int) (_ int, err error) {
	if len(hashTable) < htSize {
		return 0, fmt.Errorf("hash table too small, should be at least %d in size", htSize)
	}
	defer recoverBlock(&err)

	// adaptSkipLog sets how quickly the compressor begins skipping blocks when data is incompressible.
	// This significantly speeds up incompressible data and usually has very small impact on compresssion.
	// bytes to skip =  1 + (bytes since last match >> adaptSkipLog)
	const adaptSkipLog = 7
	sn, dn := len(src)-mfLimit, len(dst)
	if sn <= 0 || dn == 0 {
		return 0, nil
	}
	// Prove to the compiler the table has at least htSize elements.
	// The compiler can see that "uint32() >> hashShift" cannot be out of bounds.
	hashTable = hashTable[:htSize]

	// si: Current position of the search.
	// anchor: Position of the current literals.
	var si, di, anchor int

	// Fast scan strategy: the hash table only stores the last 4 bytes sequences.
	for si < sn {
		// Hash the next 6 bytes (sequence)...
		match := binary.LittleEndian.Uint64(src[si:])
		h := blockHash(match)
		h2 := blockHash(match >> 8)

		// We check a match at s, s+1 and s+2 and pick the first one we get.
		// Checking 3 only requires us to load the source one.
		ref := hashTable[h]
		ref2 := hashTable[h2]
		hashTable[h] = si
		hashTable[h2] = si + 1
		offset := si - ref

		// If offset <= 0 we got an old entry in the hash table.
		if offset <= 0 || offset >= winSize || // Out of window.
			uint32(match) != binary.LittleEndian.Uint32(src[ref:]) { // Hash collision on different matches.
			// No match. Start calculating another hash.
			// The processor can usually do this out-of-order.
			h = blockHash(match >> 16)
			ref = hashTable[h]

			// Check the second match at si+1
			si += 1
			offset = si - ref2

			if offset <= 0 || offset >= winSize ||
				uint32(match>>8) != binary.LittleEndian.Uint32(src[ref2:]) {
				// No match. Check the third match at si+2
				si += 1
				offset = si - ref
				hashTable[h] = si

				if offset <= 0 || offset >= winSize ||
					uint32(match>>16) != binary.LittleEndian.Uint32(src[ref:]) {
					// Skip one extra byte (at si+3) before we check 3 matches again.
					si += 2 + (si-anchor)>>adaptSkipLog
					continue
				}
			}
		}

		// Match found.
		lLen := si - anchor // Literal length.
		// We already matched 4 bytes.
		mLen := 4

		// Extend backwards if we can, reducing literals.
		tOff := si - offset - 1
		for lLen > 0 && tOff >= 0 && src[si-1] == src[tOff] {
			si--
			tOff--
			lLen--
			mLen++
		}

		// Add the match length, so we continue search at the end.
		// Use mLen to store the offset base.
		si, mLen = si+mLen, si+minMatch

		// Find the longest match by looking by batches of 8 bytes.
		for si+8 < sn {
			x := binary.LittleEndian.Uint64(src[si:]) ^ binary.LittleEndian.Uint64(src[si-offset:])
			if x == 0 {
				si += 8
			} else {
				// Stop is first non-zero byte.
				si += bits.TrailingZeros64(x) >> 3
				break
			}
		}

		mLen = si - mLen
		if mLen < 0xF {
			dst[di] = byte(mLen)
		} else {
			dst[di] = 0xF
		}

		// Encode literals length.
		if lLen < 0xF {
			dst[di] |= byte(lLen << 4)
		} else {
			dst[di] |= 0xF0
			di++
			l := lLen - 0xF
			for ; l >= 0xFF; l -= 0xFF {
				dst[di] = 0xFF
				di++
			}
			dst[di] = byte(l)
		}
		di++

		// Literals.
		copy(dst[di:di+lLen], src[anchor:anchor+lLen])
		di += lLen + 2
		anchor = si

		// Encode offset.
		_ = dst[di] // Bound check elimination.
		dst[di-2], dst[di-1] = byte(offset), byte(offset>>8)

		// Encode match length part 2.
		if mLen >= 0xF {
			for mLen -= 0xF; mLen >= 0xFF; mLen -= 0xFF {
				dst[di] = 0xFF
				di++
			}
			dst[di] = byte(mLen)
			di++
		}
		// Check if we can load next values.
		if si >= sn {
			break
		}
		// Hash match end-2
		h = blockHash(binary.LittleEndian.Uint64(src[si-2:]))
		hashTable[h] = si - 2
	}

	if anchor == 0 {
		// Incompressible.
		return 0, nil
	}

	// Last literals.
	lLen := len(src) - anchor
	if lLen < 0xF {
		dst[di] = byte(lLen << 4)
	} else {
		dst[di] = 0xF0
		di++
		for lLen -= 0xF; lLen >= 0xFF; lLen -= 0xFF {
			dst[di] = 0xFF
			di++
		}
		dst[di] = byte(lLen)
	}
	di++

	// Write the last literals.
	if di >= anchor {
		// Incompressible.
		return 0, nil
	}
	di += copy(dst[di:di+len(src)-anchor], src[anchor:])
	return di, nil
}

// blockHash hashes 4 bytes into a value < winSize.
func blockHashHC(x uint32) uint32 {
	const hasher uint32 = 2654435761 // Knuth multiplicative hash.
	return x * hasher >> (32 - winSizeLog)
}

// CompressBlockHC compresses the source buffer src into the destination dst
// with max search depth (use 0 or negative value for no max).
//
// CompressBlockHC compression ratio is better than CompressBlock but it is also slower.
//
// The size of the compressed data is returned. If it is 0 and no error, then the data is not compressible.
//
// An error is returned if the destination buffer is too small.
func CompressBlockHC(src, dst []byte, depth int) (_ int, err error) {
	defer recoverBlock(&err)

	// adaptSkipLog sets how quickly the compressor begins skipping blocks when data is incompressible.
	// This significantly speeds up incompressible data and usually has very small impact on compresssion.
	// bytes to skip =  1 + (bytes since last match >> adaptSkipLog)
	const adaptSkipLog = 7

	sn, dn := len(src)-mfLimit, len(dst)
	if sn <= 0 || dn == 0 {
		return 0, nil
	}
	var si, di int

	// hashTable: stores the last position found for a given hash
	// chainTable: stores previous positions for a given hash
	var hashTable, chainTable [winSize]int

	if depth <= 0 {
		depth = winSize
	}

	anchor := si
	for si < sn {
		// Hash the next 4 bytes (sequence).
		match := binary.LittleEndian.Uint32(src[si:])
		h := blockHashHC(match)

		// Follow the chain until out of window and give the longest match.
		mLen := 0
		offset := 0
		for next, try := hashTable[h], depth; try > 0 && next > 0 && si-next < winSize; next = chainTable[next&winMask] {
			// The first (mLen==0) or next byte (mLen>=minMatch) at current match length
			// must match to improve on the match length.
			if src[next+mLen] != src[si+mLen] {
				continue
			}
			ml := 0
			// Compare the current position with a previous with the same hash.
			for ml < sn-si {
				x := binary.LittleEndian.Uint64(src[next+ml:]) ^ binary.LittleEndian.Uint64(src[si+ml:])
				if x == 0 {
					ml += 8
				} else {
					// Stop is first non-zero byte.
					ml += bits.TrailingZeros64(x) >> 3
					break
				}
			}
			if ml < minMatch || ml <= mLen {
				// Match too small (<minMath) or smaller than the current match.
				continue
			}
			// Found a longer match, keep its position and length.
			mLen = ml
			offset = si - next
			// Try another previous position with the same hash.
			try--
		}
		chainTable[si&winMask] = hashTable[h]
		hashTable[h] = si

		// No match found.
		if mLen == 0 {
			si += 1 + (si-anchor)>>adaptSkipLog
			continue
		}

		// Match found.
		// Update hash/chain tables with overlapping bytes:
		// si already hashed, add everything from si+1 up to the match length.
		winStart := si + 1
		if ws := si + mLen - winSize; ws > winStart {
			winStart = ws
		}
		for si, ml := winStart, si+mLen; si < ml; {
			match >>= 8
			match |= uint32(src[si+3]) << 24
			h := blockHashHC(match)
			chainTable[si&winMask] = hashTable[h]
			hashTable[h] = si
			si++
		}

		lLen := si - anchor
		si += mLen
		mLen -= minMatch // Match length does not include minMatch.

		if mLen < 0xF {
			dst[di] = byte(mLen)
		} else {
			dst[di] = 0xF
		}

		// Encode literals length.
		if lLen < 0xF {
			dst[di] |= byte(lLen << 4)
		} else {
			dst[di] |= 0xF0
			di++
			l := lLen - 0xF
			for ; l >= 0xFF; l -= 0xFF {
				dst[di] = 0xFF
				di++
			}
			dst[di] = byte(l)
		}
		di++

		// Literals.
		copy(dst[di:di+lLen], src[anchor:anchor+lLen])
		di += lLen
		anchor = si

		// Encode offset.
		di += 2
		dst[di-2], dst[di-1] = byte(offset), byte(offset>>8)

		// Encode match length part 2.
		if mLen >= 0xF {
			for mLen -= 0xF; mLen >= 0xFF; mLen -= 0xFF {
				dst[di] = 0xFF
				di++
			}
			dst[di] = byte(mLen)
			di++
		}
	}

	if anchor == 0 {
		// Incompressible.
		return 0, nil
	}

	// Last literals.
	lLen := len(src) - anchor
	if lLen < 0xF {
		dst[di] = byte(lLen << 4)
	} else {
		dst[di] = 0xF0
		di++
		lLen -= 0xF
		for ; lLen >= 0xFF; lLen -= 0xFF {
			dst[di] = 0xFF
			di++
		}
		dst[di] = byte(lLen)
	}
	di++

	// Write the last literals.
	if di >= anchor {
		// Incompressible.
		return 0, nil
	}
	di += copy(dst[di:di+len(src)-anchor], src[anchor:])
	return di, nil
}
