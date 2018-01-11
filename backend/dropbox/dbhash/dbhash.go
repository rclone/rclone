// Package dbhash implements the dropbox hash as described in
//
// https://www.dropbox.com/developers/reference/content-hash
package dbhash

import (
	"crypto/sha256"
	"hash"
)

const (
	// BlockSize of the checksum in bytes.
	BlockSize = sha256.BlockSize
	// Size of the checksum in bytes.
	Size              = sha256.BlockSize
	bytesPerBlock     = 4 * 1024 * 1024
	hashReturnedError = "hash function returned error"
)

type digest struct {
	n           int // bytes written into blockHash so far
	blockHash   hash.Hash
	totalHash   hash.Hash
	sumCalled   bool
	writtenMore bool
}

// New returns a new hash.Hash computing the Dropbox checksum.
func New() hash.Hash {
	d := &digest{}
	d.Reset()
	return d
}

// writeBlockHash writes the current block hash into the total hash
func (d *digest) writeBlockHash() {
	blockHash := d.blockHash.Sum(nil)
	_, err := d.totalHash.Write(blockHash)
	if err != nil {
		panic(hashReturnedError)
	}
	// reset counters for blockhash
	d.n = 0
	d.blockHash.Reset()
}

// Write writes len(p) bytes from p to the underlying data stream. It returns
// the number of bytes written from p (0 <= n <= len(p)) and any error
// encountered that caused the write to stop early. Write must return a non-nil
// error if it returns n < len(p). Write must not modify the slice data, even
// temporarily.
//
// Implementations must not retain p.
func (d *digest) Write(p []byte) (n int, err error) {
	n = len(p)
	for len(p) > 0 {
		d.writtenMore = true
		toWrite := bytesPerBlock - d.n
		if toWrite > len(p) {
			toWrite = len(p)
		}
		_, err = d.blockHash.Write(p[:toWrite])
		if err != nil {
			panic(hashReturnedError)
		}
		d.n += toWrite
		p = p[toWrite:]
		// Accumulate the total hash
		if d.n == bytesPerBlock {
			d.writeBlockHash()
		}
	}
	return n, nil
}

// Sum appends the current hash to b and returns the resulting slice.
// It does not change the underlying hash state.
//
// TODO(ncw) Sum() can only be called once for this type of hash.
// If you call Sum(), then Write() then Sum() it will result in
// a panic.  Calling Write() then Sum(), then Sum() is OK.
func (d *digest) Sum(b []byte) []byte {
	if d.sumCalled && d.writtenMore {
		panic("digest.Sum() called more than once")
	}
	d.sumCalled = true
	d.writtenMore = false
	if d.n != 0 {
		d.writeBlockHash()
	}
	return d.totalHash.Sum(b)
}

// Reset resets the Hash to its initial state.
func (d *digest) Reset() {
	d.n = 0
	d.totalHash = sha256.New()
	d.blockHash = sha256.New()
	d.sumCalled = false
	d.writtenMore = false
}

// Size returns the number of bytes Sum will return.
func (d *digest) Size() int {
	return d.totalHash.Size()
}

// BlockSize returns the hash's underlying block size.
// The Write method must be able to accept any amount
// of data, but it may operate more efficiently if all writes
// are a multiple of the block size.
func (d *digest) BlockSize() int {
	return d.totalHash.BlockSize()
}

// Sum returns the Dropbox checksum of the data.
func Sum(data []byte) [Size]byte {
	var d digest
	d.Reset()
	_, _ = d.Write(data)
	var out [Size]byte
	d.Sum(out[:0])
	return out
}

// must implement this interface
var _ hash.Hash = (*digest)(nil)
