package press

import (
	"bufio"
	"io"

	"github.com/klauspost/compress/gzip"
)

// AlgGzip represents gzip compression algorithm
type AlgGzip struct {
	level     int
	blockSize uint32
}

// InitializeGzip initializes the gzip compression Algorithm
func InitializeGzip(bs uint32, level int) Algorithm {
	a := new(AlgGzip)
	a.blockSize = bs
	a.level = level
	return a
}

// GetFileExtension returns file extension
func (a *AlgGzip) GetFileExtension() string {
	return ".gz"
}

// GetHeader returns the Lz4 compression header
func (a *AlgGzip) GetHeader() []byte {
	return []byte{}
}

// GetFooter returns
func (a *AlgGzip) GetFooter() []byte {
	return []byte{}
}

// CompressBlock that compresses a block using gzip
func (a *AlgGzip) CompressBlock(in []byte, out io.Writer) (compressedSize uint32, uncompressedSize uint64, err error) {
	// Initialize buffer
	bufw := bufio.NewWriterSize(out, int(a.blockSize+(a.blockSize)>>4))

	// Initialize block writer
	outw, err := gzip.NewWriterLevel(bufw, a.level)
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

	return blockSize, uint64(len(in)), err
}

// DecompressBlock decompresses Lz4 compressed block
func (a *AlgGzip) DecompressBlock(in io.Reader, out io.Writer, BlockSize uint32) (n int, err error) {
	gzipReader, err := gzip.NewReader(in)
	if err != nil {
		return 0, err
	}
	written, err := io.Copy(out, gzipReader)
	return int(written), err
}
