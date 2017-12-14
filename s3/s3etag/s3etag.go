// Package s3etag implements the AWS ETag hash as described in
//
// http://permalink.gmane.org/gmane.comp.file-systems.s3.s3tools/583
package s3etag

import (
	"crypto/md5"
	"encoding/binary"
	"hash"

	"github.com/ncw/rclone/s3/s3sizes"
)

const (
	hashSize          = 32
	partCountSize     = 4
	size              = hashSize + partCountSize
	hashReturnedError = "hash function returned error"
)

type digest struct {
	n           int64 // bytes written into blockHash so far
	blockHash   hash.Hash
	totalHash   hash.Hash
	sumCalled   bool
	writtenMore bool
	partSize    int64
	partCount   uint16
}

// New returns a new hash.Hash computing the S3ETag checksum.
func New(size int64) hash.Hash {
	partSize := s3sizes.PartSize(size)
	d := &digest{
		partSize: partSize,
	}
	d.Reset()
	return d
}

// Reset resets the Hash to its initial state.
func (d *digest) Reset() {
	d.n = 0
	d.totalHash = md5.New()
	d.blockHash = md5.New()
	d.sumCalled = false
	d.writtenMore = false
	d.partCount = 1
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
		if d.n == d.partSize {
			d.writeBlockHash()
			d.partCount++
		}
		d.writtenMore = true
		toWrite := d.partSize - d.n
		if toWrite > int64(len(p)) {
			toWrite = int64(len(p))
		}
		_, err = d.blockHash.Write(p[:toWrite])
		if err != nil {
			panic(hashReturnedError)
		}
		d.n += toWrite
		p = p[toWrite:]
		// Accumulate the total hash
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
	if d.partCount > 1 {
		if d.n != 0 {
			d.writeBlockHash()
		}
		b = d.totalHash.Sum(b)
	} else {
		b = d.blockHash.Sum(b)
	}
	c := make([]byte, 2)
	binary.BigEndian.PutUint16(c, d.partCount)
	return append(b, c...)
}

// Size returns the number of bytes Sum will return.
func (d *digest) Size() int {
	return size
}

// BlockSize returns the hash's underlying block size.
// The Write method must be able to accept any amount
// of data, but it may operate more efficiently if all writes
// are a multiple of the block size.
func (d *digest) BlockSize() int {
	return d.blockHash.BlockSize()
}

// Sum returns the S3 ETag checksum of the data.
func Sum(data []byte) [size]byte {
	d := New(int64(len(data)))
	_, _ = d.Write(data)
	var out [size]byte
	d.Sum(out[:0])
	return out
}

// must implement this interface
var _ hash.Hash = (*digest)(nil)
