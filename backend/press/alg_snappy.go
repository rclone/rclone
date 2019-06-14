package press

// This file implements compression/decompression using snappy.
import (
	"bytes"
	"io"

	"github.com/golang/snappy"
)

// SnappyHeader - Header we add to a snappy file. We don't need this.
var SnappyHeader = []byte{}

// Function that compresses a block using snappy
func (c *Compression) compressBlockSnappy(in []byte, out io.Writer) (compressedSize uint32, uncompressedSize int64, err error) {
	// Compress and return
	outBytes := snappy.Encode(nil, in)
	_, err = out.Write(outBytes)
	return uint32(len(outBytes)), int64(len(in)), err
}

// Utility function to decompress a block using snappy
func decompressBlockSnappy(in io.Reader, out io.Writer) (n int, err error) {
	var b bytes.Buffer
	_, err = io.Copy(&b, in)
	if err != nil {
		return 0, err
	}
	decompressed, err := snappy.Decode(nil, b.Bytes())
	if err != nil {
		return 0, err
	}
	_, err = out.Write(decompressed)
	return len(decompressed), err
}
