// Package lz4 implements reading and writing lz4 compressed data (a frame),
// as specified in http://fastcompression.blogspot.fr/2013/04/lz4-streaming-format-final.html.
//
// Although the block level compression and decompression functions are exposed and are fully compatible
// with the lz4 block format definition, they are low level and should not be used directly.
// For a complete description of an lz4 compressed block, see:
// http://fastcompression.blogspot.fr/2011/05/lz4-explained.html
//
// See https://github.com/Cyan4973/lz4 for the reference C implementation.
//
package lz4

import "math/bits"

import "sync"

const (
	// Extension is the LZ4 frame file name extension
	Extension = ".lz4"
	// Version is the LZ4 frame format version
	Version = 1

	frameMagic     uint32 = 0x184D2204
	frameSkipMagic uint32 = 0x184D2A50

	// The following constants are used to setup the compression algorithm.
	minMatch            = 4  // the minimum size of the match sequence size (4 bytes)
	winSizeLog          = 16 // LZ4 64Kb window size limit
	winSize             = 1 << winSizeLog
	winMask             = winSize - 1 // 64Kb window of previous data for dependent blocks
	compressedBlockFlag = 1 << 31
	compressedBlockMask = compressedBlockFlag - 1

	// hashLog determines the size of the hash table used to quickly find a previous match position.
	// Its value influences the compression speed and memory usage, the lower the faster,
	// but at the expense of the compression ratio.
	// 16 seems to be the best compromise for fast compression.
	hashLog = 16
	htSize  = 1 << hashLog

	mfLimit = 10 + minMatch // The last match cannot start within the last 14 bytes.
)

// map the block max size id with its value in bytes: 64Kb, 256Kb, 1Mb and 4Mb.
const (
	blockSize64K = 1 << (16 + 2*iota)
	blockSize256K
	blockSize1M
	blockSize4M
)

var (
	// Keep a pool of buffers for each valid block sizes.
	bsMapValue = [...]*sync.Pool{
		newBufferPool(2 * blockSize64K),
		newBufferPool(2 * blockSize256K),
		newBufferPool(2 * blockSize1M),
		newBufferPool(2 * blockSize4M),
	}
)

// newBufferPool returns a pool for buffers of the given size.
func newBufferPool(size int) *sync.Pool {
	return &sync.Pool{
		New: func() interface{} {
			return make([]byte, size)
		},
	}
}

// getBuffer returns a buffer to its pool.
func getBuffer(size int) []byte {
	idx := blockSizeValueToIndex(size) - 4
	return bsMapValue[idx].Get().([]byte)
}

// putBuffer returns a buffer to its pool.
func putBuffer(size int, buf []byte) {
	if cap(buf) > 0 {
		idx := blockSizeValueToIndex(size) - 4
		bsMapValue[idx].Put(buf[:cap(buf)])
	}
}
func blockSizeIndexToValue(i byte) int {
	return 1 << (16 + 2*uint(i))
}
func isValidBlockSize(size int) bool {
	const blockSizeMask = blockSize64K | blockSize256K | blockSize1M | blockSize4M

	return size&blockSizeMask > 0 && bits.OnesCount(uint(size)) == 1
}
func blockSizeValueToIndex(size int) byte {
	return 4 + byte(bits.TrailingZeros(uint(size)>>16)/2)
}

// Header describes the various flags that can be set on a Writer or obtained from a Reader.
// The default values match those of the LZ4 frame format definition
// (http://fastcompression.blogspot.com/2013/04/lz4-streaming-format-final.html).
//
// NB. in a Reader, in case of concatenated frames, the Header values may change between Read() calls.
// It is the caller responsibility to check them if necessary.
type Header struct {
	BlockChecksum    bool   // Compressed blocks checksum flag.
	NoChecksum       bool   // Frame checksum flag.
	BlockMaxSize     int    // Size of the uncompressed data block (one of [64KB, 256KB, 1MB, 4MB]). Default=4MB.
	Size             uint64 // Frame total size. It is _not_ computed by the Writer.
	CompressionLevel int    // Compression level (higher is better, use 0 for fastest compression).
	done             bool   // Header processed flag (Read or Write and checked).
}

func (h *Header) Reset() {
	h.done = false
}
