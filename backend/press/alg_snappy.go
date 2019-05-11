// This file implements compression/decompression using snappy.
package press
import (
	"bytes"
	"io"
	"github.com/golang/snappy"
)

// Header we add to a snappy file. We don't need this.
var SNAPPY_HEADER = []byte{}

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
	io.Copy(&b, in)
	decompressed, err := snappy.Decode(nil, b.Bytes())
	out.Write(decompressed)
	return len(decompressed), err
}