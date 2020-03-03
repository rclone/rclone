package press

import (
	"bufio"
	"io"

	"github.com/ulikunitz/xz"
)

// AlgXZ represents the XZ compression algorithm
type AlgXZ struct {
	blockSize uint32
	config    xz.WriterConfig
}

// InitializeXZ creates an Lz4 compression algorithm
func InitializeXZ(bs uint32) Algorithm {
	a := new(AlgXZ)
	a.blockSize = bs
	a.config = xz.WriterConfig{}
	return a
}

// GetFileExtension returns file extension
func (a *AlgXZ) GetFileExtension() string {
	return ".xz"
}

// GetHeader returns the Lz4 compression header
func (a *AlgXZ) GetHeader() []byte {
	return []byte{}
}

// GetFooter returns
func (a *AlgXZ) GetFooter() []byte {
	return []byte{}
}

// CompressBlock that compresses a block using lz4
func (a *AlgXZ) CompressBlock(in []byte, out io.Writer) (compressedSize uint32, uncompressedSize uint64, err error) {
	// Initialize buffer
	bufw := bufio.NewWriterSize(out, int(a.blockSize+(a.blockSize)>>4))

	// Initialize block writer
	outw, err := a.config.NewWriter(bufw)
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
func (a *AlgXZ) DecompressBlock(in io.Reader, out io.Writer, BlockSize uint32) (n int, err error) {
	xzReader, err := xz.NewReader(in)
	if err != nil {
		return 0, err
	}
	written, err := io.Copy(out, xzReader)
	return int(written), err
}
