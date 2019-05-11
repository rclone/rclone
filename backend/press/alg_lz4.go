package press
import (
	"io"
//	"io/ioutil"
	"bytes"
	"errors"

	lz4 "github.com/id01/go-lz4"
	xxh "bitbucket.org/StephaneBunel/xxhash-go"
)

var LZ4_HEADER = []byte{0x04, 0x22, 0x4d, 0x18, 0x70, 0x50, 0x84}
var LZ4_FOOTER = []byte{0x00, 0x00, 0x00, 0x00}

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
	checksum := uint32ToBytes(xxh.Checksum32(compressedBytes[4:])) // The checksum doesn't include the size
	n2, err := out.Write(checksum)
	if err != nil {
		return 0, 0, err
	}
	// Return sizes
	return uint32(n1+n2), int64(len(in)), err
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
	if bytesToUint32(hash) != xxh.Checksum32(compressedBytes[4:]) {
		return 0, errors.New("XXHash checksum invalid")
	}
	dst := make([]byte, BlockSize*2)
	decompressed, err := lz4.Decode(dst, compressedBytes)
//	log.Printf("%d: %v", len(decompressed), decompressed)
	out.Write(decompressed)
	return len(decompressed), err
}