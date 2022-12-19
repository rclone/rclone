// Package hidrivehash implements the HiDrive hashing algorithm which combines SHA-1 hashes hierarchically to a single top-level hash.
//
// Note: This implementation does not grant access to any partial hashes generated.
//
// See: https://developer.hidrive.com/wp-content/uploads/2021/07/HiDrive_Synchronization-v3.3-rev28.pdf
// (link to newest version: https://static.hidrive.com/dev/0001)
package hidrivehash

import (
	"bytes"
	"crypto/sha1"
	"encoding"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"

	"github.com/rclone/rclone/backend/hidrive/hidrivehash/internal"
)

const (
	// BlockSize of the checksum in bytes.
	BlockSize = 4096
	// Size of the checksum in bytes.
	Size = sha1.Size
	// sumsPerLevel is the number of checksums
	sumsPerLevel = 256
)

var (
	// zeroSum is a special hash consisting of 20 null-bytes.
	// This will be the hash of any empty file (or ones containing only null-bytes).
	zeroSum = [Size]byte{}
	// ErrorInvalidEncoding is returned when a hash should be decoded from a binary form that is invalid.
	ErrorInvalidEncoding = errors.New("encoded binary form is invalid for this hash")
	// ErrorHashFull is returned when a hash reached its capacity and cannot accept any more input.
	ErrorHashFull = errors.New("hash reached its capacity")
)

// writeByBlock writes len(p) bytes from p to the io.Writer in blocks of size blockSize.
// It returns the number of bytes written from p (0 <= n <= len(p))
// and any error encountered that caused the write to stop early.
//
// A pointer bytesInBlock to a counter needs to be supplied,
// that is used to keep track how many bytes have been written to the writer already.
// A pointer onlyNullBytesInBlock to a boolean needs to be supplied,
// that is used to keep track whether the block so far only consists of null-bytes.
// The callback onBlockWritten is called whenever a full block has been written to the writer
// and is given as input the number of bytes that still need to be written.
func writeByBlock(p []byte, writer io.Writer, blockSize uint32, bytesInBlock *uint32, onlyNullBytesInBlock *bool, onBlockWritten func(remaining int) error) (n int, err error) {
	total := len(p)
	nullBytes := make([]byte, blockSize)
	for len(p) > 0 {
		toWrite := int(blockSize - *bytesInBlock)
		if toWrite > len(p) {
			toWrite = len(p)
		}
		c, err := writer.Write(p[:toWrite])
		*bytesInBlock += uint32(c)
		*onlyNullBytesInBlock = *onlyNullBytesInBlock && bytes.Equal(nullBytes[:toWrite], p[:toWrite])
		// Discard data written through a reslice
		p = p[c:]
		if err != nil {
			return total - len(p), err
		}
		if *bytesInBlock == blockSize {
			err = onBlockWritten(len(p))
			if err != nil {
				return total - len(p), err
			}
			*bytesInBlock = 0
			*onlyNullBytesInBlock = true
		}
	}
	return total, nil
}

// level is a hash.Hash that is used to aggregate the checksums produced by the level hierarchically beneath it.
// It is used to represent any level-n hash, except for level-0.
type level struct {
	checksum              [Size]byte // aggregated checksum of this level
	sumCount              uint32     // number of sums contained in this level so far
	bytesInHasher         uint32     //  number of bytes written into hasher so far
	onlyNullBytesInHasher bool       // whether the hasher only contains null-bytes so far
	hasher                hash.Hash
}

// NewLevel returns a new hash.Hash computing any level-n hash, except level-0.
func NewLevel() hash.Hash {
	l := &level{}
	l.Reset()
	return l
}

// Add takes a position-embedded SHA-1 checksum and adds it to the level.
func (l *level) Add(sha1sum []byte) {
	var tmp uint
	var carry bool
	for i := Size - 1; i >= 0; i-- {
		tmp = uint(sha1sum[i]) + uint(l.checksum[i])
		if carry {
			tmp++
		}
		carry = tmp > 255
		l.checksum[i] = byte(tmp)
	}
}

// IsFull returns whether the number of checksums added to this level reached its capacity.
func (l *level) IsFull() bool {
	return l.sumCount >= sumsPerLevel
}

// Write (via the embedded io.Writer interface) adds more data to the running hash.
// Contrary to the specification from hash.Hash, this DOES return an error,
// specifically ErrorHashFull if and only if IsFull() returns true.
func (l *level) Write(p []byte) (n int, err error) {
	if l.IsFull() {
		return 0, ErrorHashFull
	}
	onBlockWritten := func(remaining int) error {
		if !l.onlyNullBytesInHasher {
			c, err := l.hasher.Write([]byte{byte(l.sumCount)})
			l.bytesInHasher += uint32(c)
			if err != nil {
				return err
			}
			l.Add(l.hasher.Sum(nil))
		}
		l.sumCount++
		l.hasher.Reset()
		if remaining > 0 && l.IsFull() {
			return ErrorHashFull
		}
		return nil
	}
	return writeByBlock(p, l.hasher, uint32(l.BlockSize()), &l.bytesInHasher, &l.onlyNullBytesInHasher, onBlockWritten)
}

// Sum appends the current hash to b and returns the resulting slice.
// It does not change the underlying hash state.
func (l *level) Sum(b []byte) []byte {
	return append(b, l.checksum[:]...)
}

// Reset resets the Hash to its initial state.
func (l *level) Reset() {
	l.checksum = zeroSum // clear the current checksum
	l.sumCount = 0
	l.bytesInHasher = 0
	l.onlyNullBytesInHasher = true
	l.hasher = sha1.New()
}

// Size returns the number of bytes Sum will return.
func (l *level) Size() int {
	return Size
}

// BlockSize returns the hash's underlying block size.
// The Write method must be able to accept any amount
// of data, but it may operate more efficiently if all writes
// are a multiple of the block size.
func (l *level) BlockSize() int {
	return Size
}

// MarshalBinary encodes the hash into a binary form and returns the result.
func (l *level) MarshalBinary() ([]byte, error) {
	b := make([]byte, Size+4+4+1)
	copy(b, l.checksum[:])
	binary.BigEndian.PutUint32(b[Size:], l.sumCount)
	binary.BigEndian.PutUint32(b[Size+4:], l.bytesInHasher)
	if l.onlyNullBytesInHasher {
		b[Size+4+4] = 1
	}
	encodedHasher, err := l.hasher.(encoding.BinaryMarshaler).MarshalBinary()
	if err != nil {
		return nil, err
	}
	b = append(b, encodedHasher...)
	return b, nil
}

// UnmarshalBinary decodes the binary form generated by MarshalBinary.
// The hash will replace its internal state accordingly.
func (l *level) UnmarshalBinary(b []byte) error {
	if len(b) < Size+4+4+1 {
		return ErrorInvalidEncoding
	}
	copy(l.checksum[:], b)
	l.sumCount = binary.BigEndian.Uint32(b[Size:])
	l.bytesInHasher = binary.BigEndian.Uint32(b[Size+4:])
	switch b[Size+4+4] {
	case 0:
		l.onlyNullBytesInHasher = false
	case 1:
		l.onlyNullBytesInHasher = true
	default:
		return ErrorInvalidEncoding
	}
	err := l.hasher.(encoding.BinaryUnmarshaler).UnmarshalBinary(b[Size+4+4+1:])
	return err
}

// hidriveHash is the hash computing the actual checksum used by HiDrive by combining multiple level-hashes.
type hidriveHash struct {
	levels               []*level   // collection of level-hashes, one for each level starting at level-1
	lastSumWritten       [Size]byte // the last checksum written to any of the levels
	bytesInBlock         uint32     // bytes written into blockHash so far
	onlyNullBytesInBlock bool       // whether the hasher only contains null-bytes so far
	blockHash            hash.Hash
}

// New returns a new hash.Hash computing the HiDrive checksum.
func New() hash.Hash {
	h := &hidriveHash{}
	h.Reset()
	return h
}

// aggregateToLevel writes the checksum to the level at the given index
// and if necessary propagates any changes to levels above.
func (h *hidriveHash) aggregateToLevel(index int, sum []byte) {
	for i := index; ; i++ {
		if i >= len(h.levels) {
			h.levels = append(h.levels, NewLevel().(*level))
		}
		_, err := h.levels[i].Write(sum)
		copy(h.lastSumWritten[:], sum)
		if err != nil {
			panic(fmt.Errorf("level-hash should not have produced an error: %w", err))
		}
		if !h.levels[i].IsFull() {
			break
		}
		sum = h.levels[i].Sum(nil)
		h.levels[i].Reset()
	}
}

// Write (via the embedded io.Writer interface) adds more data to the running hash.
// It never returns an error.
func (h *hidriveHash) Write(p []byte) (n int, err error) {
	onBlockWritten := func(remaining int) error {
		var sum []byte
		if h.onlyNullBytesInBlock {
			sum = zeroSum[:]
		} else {
			sum = h.blockHash.Sum(nil)
		}
		h.blockHash.Reset()
		h.aggregateToLevel(0, sum)
		return nil
	}
	return writeByBlock(p, h.blockHash, uint32(BlockSize), &h.bytesInBlock, &h.onlyNullBytesInBlock, onBlockWritten)
}

// Sum appends the current hash to b and returns the resulting slice.
// It does not change the underlying hash state.
func (h *hidriveHash) Sum(b []byte) []byte {
	// Save internal state.
	state, err := h.MarshalBinary()
	if err != nil {
		panic(fmt.Errorf("saving the internal state should not have produced an error: %w", err))
	}

	if h.bytesInBlock > 0 {
		// Fill remainder of block with null-bytes.
		filler := make([]byte, h.BlockSize()-int(h.bytesInBlock))
		_, err = h.Write(filler)
		if err != nil {
			panic(fmt.Errorf("filling with null-bytes should not have an error: %w", err))
		}
	}

	checksum := zeroSum
	for i := 0; i < len(h.levels); i++ {
		level := h.levels[i]
		if i < len(h.levels)-1 {
			// Aggregate non-empty non-final levels.
			if level.sumCount >= 1 {
				h.aggregateToLevel(i+1, level.Sum(nil))
				level.Reset()
			}
		} else {
			// Determine sum of final level.
			if level.sumCount > 1 {
				copy(checksum[:], level.Sum(nil))
			} else {
				// This is needed, otherwise there is no way to return
				// the non-position-embedded checksum.
				checksum = h.lastSumWritten
			}
		}
	}

	// Restore internal state.
	err = h.UnmarshalBinary(state)
	if err != nil {
		panic(fmt.Errorf("restoring the internal state should not have produced an error: %w", err))
	}

	return append(b, checksum[:]...)
}

// Reset resets the Hash to its initial state.
func (h *hidriveHash) Reset() {
	h.levels = nil
	h.lastSumWritten = zeroSum // clear the last written checksum
	h.bytesInBlock = 0
	h.onlyNullBytesInBlock = true
	h.blockHash = sha1.New()
}

// Size returns the number of bytes Sum will return.
func (h *hidriveHash) Size() int {
	return Size
}

// BlockSize returns the hash's underlying block size.
// The Write method must be able to accept any amount
// of data, but it may operate more efficiently if all writes
// are a multiple of the block size.
func (h *hidriveHash) BlockSize() int {
	return BlockSize
}

// MarshalBinary encodes the hash into a binary form and returns the result.
func (h *hidriveHash) MarshalBinary() ([]byte, error) {
	b := make([]byte, Size+4+1+8)
	copy(b, h.lastSumWritten[:])
	binary.BigEndian.PutUint32(b[Size:], h.bytesInBlock)
	if h.onlyNullBytesInBlock {
		b[Size+4] = 1
	}

	binary.BigEndian.PutUint64(b[Size+4+1:], uint64(len(h.levels)))
	for _, level := range h.levels {
		encodedLevel, err := level.MarshalBinary()
		if err != nil {
			return nil, err
		}
		encodedLength := make([]byte, 8)
		binary.BigEndian.PutUint64(encodedLength, uint64(len(encodedLevel)))
		b = append(b, encodedLength...)
		b = append(b, encodedLevel...)
	}
	encodedBlockHash, err := h.blockHash.(encoding.BinaryMarshaler).MarshalBinary()
	if err != nil {
		return nil, err
	}
	b = append(b, encodedBlockHash...)
	return b, nil
}

// UnmarshalBinary decodes the binary form generated by MarshalBinary.
// The hash will replace its internal state accordingly.
func (h *hidriveHash) UnmarshalBinary(b []byte) error {
	if len(b) < Size+4+1+8 {
		return ErrorInvalidEncoding
	}
	copy(h.lastSumWritten[:], b)
	h.bytesInBlock = binary.BigEndian.Uint32(b[Size:])
	switch b[Size+4] {
	case 0:
		h.onlyNullBytesInBlock = false
	case 1:
		h.onlyNullBytesInBlock = true
	default:
		return ErrorInvalidEncoding
	}

	amount := binary.BigEndian.Uint64(b[Size+4+1:])
	h.levels = make([]*level, int(amount))
	offset := Size + 4 + 1 + 8
	for i := range h.levels {
		length := int(binary.BigEndian.Uint64(b[offset:]))
		offset += 8
		h.levels[i] = NewLevel().(*level)
		err := h.levels[i].UnmarshalBinary(b[offset : offset+length])
		if err != nil {
			return err
		}
		offset += length
	}
	err := h.blockHash.(encoding.BinaryUnmarshaler).UnmarshalBinary(b[offset:])
	return err
}

// Sum returns the HiDrive checksum of the data.
func Sum(data []byte) [Size]byte {
	h := New().(*hidriveHash)
	_, _ = h.Write(data)
	var result [Size]byte
	copy(result[:], h.Sum(nil))
	return result
}

// Check the interfaces are satisfied.
var (
	_ hash.Hash                  = (*level)(nil)
	_ encoding.BinaryMarshaler   = (*level)(nil)
	_ encoding.BinaryUnmarshaler = (*level)(nil)
	_ internal.LevelHash         = (*level)(nil)
	_ hash.Hash                  = (*hidriveHash)(nil)
	_ encoding.BinaryMarshaler   = (*hidriveHash)(nil)
	_ encoding.BinaryUnmarshaler = (*hidriveHash)(nil)
)
