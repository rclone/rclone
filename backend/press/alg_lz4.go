package press

// This file implements the LZ4 algorithm.
import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"

	"github.com/OneOfOne/xxhash"
	lz4 "github.com/id01/go-lz4"
)

/*
Structure of LZ4 header:
Flags:
	Version = 01
	Independent = 1
	Block Checksum = 1
	Content Size = 0
	Content Checksum = 0
	Reserved = 0
	Dictionary ID = 0

BD byte:
	Reserved = 0
	Block Max Size = 101 (or 5; 256kb)
	Reserved = 0000

Header checksum byte (xxhash(flags and bd byte) >> 1) & 0xff
*/

// LZ4Header - Header of our LZ4 file
var LZ4Header = []byte{0x04, 0x22, 0x4d, 0x18, 0x70, 0x50, 0x84}

// LZ4Footer - Footer of our LZ4 file
var LZ4Footer = []byte{0x00, 0x00, 0x00, 0x00} // This is just an empty block

// Function that compresses a block using lz4
func (c *Compression) compressBlockLz4(in []byte, out io.Writer) (compressedSize uint32, uncompressedSize int64, err error) {
	// Write lz4 compressed data
	compressedBytes, err := lz4.Encode(nil, in)
	if err != nil {
		return 0, 0, err
	}
	// Write compressed bytes
	n1, err := out.Write(compressedBytes)
	if err != nil {
		return 0, 0, err
	}
	// Get checksum
	h := xxhash.New32()
	_, err = h.Write(compressedBytes[4:]) // The checksum doesn't include the size
	if err != nil {
		return 0, 0, err
	}
	checksum := make([]byte, 4)
	binary.LittleEndian.PutUint32(checksum, h.Sum32())
	n2, err := out.Write(checksum)
	if err != nil {
		return 0, 0, err
	}
	// Return sizes
	return uint32(n1 + n2), int64(len(in)), err
}

// Utility function to decompress a block using LZ4
func decompressBlockLz4(in io.Reader, out io.Writer, BlockSize int64) (n int, err error) {
	// Get our compressed data
	var b bytes.Buffer
	_, err = io.Copy(&b, in)
	if err != nil {
		return 0, err
	}
	// Add the length in byte form to the begining of the buffer. Because the length is not equal to BlockSize for the last block, the last block might screw this code up.
	compressedBytesWithHash := b.Bytes()
	compressedBytes := compressedBytesWithHash[:len(compressedBytesWithHash)-4]
	hash := compressedBytesWithHash[len(compressedBytesWithHash)-4:]
	// Verify, decode, write, and return
	h := xxhash.New32()
	_, err = h.Write(compressedBytes[4:])
	if err != nil {
		return 0, err
	}
	if binary.LittleEndian.Uint32(hash) != h.Sum32() {
		return 0, errors.New("XXHash checksum invalid")
	}
	dst := make([]byte, BlockSize*2)
	decompressed, err := lz4.Decode(dst, compressedBytes)
	if err != nil {
		return 0, err
	}
	_, err = out.Write(decompressed)
	return len(decompressed), err
}
