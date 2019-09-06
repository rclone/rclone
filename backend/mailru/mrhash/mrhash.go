// Package mrhash implements the mailru hash, which is a modified SHA1.
// If file size is less than or equal to the SHA1 block size (20 bytes),
// its hash is simply its data right-padded with zero bytes.
// Hash sum of a larger file is computed as a SHA1 sum of the file data
// bytes concatenated with a decimal representation of the data length.
package mrhash

import (
	"crypto/sha1"
	"encoding"
	"encoding/hex"
	"errors"
	"hash"
	"strconv"
)

const (
	// BlockSize of the checksum in bytes.
	BlockSize = sha1.BlockSize
	// Size of the checksum in bytes.
	Size        = sha1.Size
	startString = "mrCloud"
	hashError   = "hash function returned error"
)

// Global errors
var (
	ErrorInvalidHash = errors.New("invalid hash")
)

type digest struct {
	total int       // bytes written into hash so far
	sha   hash.Hash // underlying SHA1
	small []byte    // small content
}

// New returns a new hash.Hash computing the Mailru checksum.
func New() hash.Hash {
	d := &digest{}
	d.Reset()
	return d
}

// Write writes len(p) bytes from p to the underlying data stream. It returns
// the number of bytes written from p (0 <= n <= len(p)) and any error
// encountered that caused the write to stop early. Write must return a non-nil
// error if it returns n < len(p). Write must not modify the slice data, even
// temporarily.
//
// Implementations must not retain p.
func (d *digest) Write(p []byte) (n int, err error) {
	n, err = d.sha.Write(p)
	if err != nil {
		panic(hashError)
	}
	d.total += n
	if d.total <= Size {
		d.small = append(d.small, p...)
	}
	return n, nil
}

// Sum appends the current hash to b and returns the resulting slice.
// It does not change the underlying hash state.
func (d *digest) Sum(b []byte) []byte {
	// If content is small, return it padded to Size
	if d.total <= Size {
		padded := make([]byte, Size)
		copy(padded, d.small)
		return append(b, padded...)
	}
	endString := strconv.Itoa(d.total)
	copy, err := cloneSHA1(d.sha)
	if err == nil {
		_, err = copy.Write([]byte(endString))
	}
	if err != nil {
		panic(hashError)
	}
	return copy.Sum(b)
}

// cloneSHA1 clones state of SHA1 hash
func cloneSHA1(orig hash.Hash) (clone hash.Hash, err error) {
	state, err := orig.(encoding.BinaryMarshaler).MarshalBinary()
	if err != nil {
		return nil, err
	}
	clone = sha1.New()
	err = clone.(encoding.BinaryUnmarshaler).UnmarshalBinary(state)
	return
}

// Reset resets the Hash to its initial state.
func (d *digest) Reset() {
	d.sha = sha1.New()
	_, _ = d.sha.Write([]byte(startString))
	d.total = 0
}

// Size returns the number of bytes Sum will return.
func (d *digest) Size() int {
	return Size
}

// BlockSize returns the hash's underlying block size.
// The Write method must be able to accept any amount
// of data, but it may operate more efficiently if all writes
// are a multiple of the block size.
func (d *digest) BlockSize() int {
	return BlockSize
}

// Sum returns the Mailru checksum of the data.
func Sum(data []byte) []byte {
	var d digest
	d.Reset()
	_, _ = d.Write(data)
	return d.Sum(nil)
}

// DecodeString converts a string to the Mailru hash
func DecodeString(s string) ([]byte, error) {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != Size {
		return nil, ErrorInvalidHash
	}
	return b, nil
}

// must implement this interface
var (
	_ hash.Hash = (*digest)(nil)
)
