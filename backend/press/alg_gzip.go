package press

// This file implements the gzip algorithm.
import (
	"bufio"
	"compress/gzip"
	"io"
)

// GzipHeader - Header we add to a gzip file. We're contatenating GZIP files here, so we don't need this.
var GzipHeader = []byte{}

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
	_, err = outw.Write(in)
	if err != nil {
		return 0, 0, err
	}

	// Finalize gzip file, flush buffer and return
	err = outw.Close()
	if err != nil {
		return 0, 0, err
	}
	blockSize := uint32(bufw.Buffered())
	err = bufw.Flush()

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
