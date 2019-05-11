// This file implements the gzip algorithm.
package press
import (
	"compress/gzip"
	"bufio"
	"io"
)

// Header we add to a gzip file. We're contatenating GZIP files here, so we don't need this.
var GZIP_HEADER = []byte{}

// Function that compresses a block using gzip
func (c *Compression) compressBlockGz(in []byte, out io.Writer, compressionLevel int) (compressedSize uint32, uncompressedSize int64, err error) {
	// Initialize buffer
	bufw := bufio.NewWriterSize(out, int(c.maxCompressedBlockSize()))

	// Initialize block writer
	outw, err := gzip.NewWriterLevel(bufw, compressionLevel)
	if err != nil {
		return 0, 0, err
	}

	// Compress block
	outw.Write(in)

	// Finalize gzip file, flush buffer and return
	outw.Close()
	blockSize := uint32(bufw.Buffered())
	bufw.Flush()

	return blockSize, int64(len(in)), err
}

// Utility function to decompress a block range using gzip
func decompressBlockRangeGz(in io.Reader, out io.Writer) (n int, err error) {
	gzipReader, err := gzip.NewReader(in)
	if err != nil {
		return 0, err
	}
	written, err := io.Copy(out, gzipReader)
	return int(written), err
}