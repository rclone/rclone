// Package raid3: compression helpers for write/read path (config mapping, compress, decompress).
package raid3

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/golang/snappy"
	"github.com/klauspost/compress/zstd"
)

// ConfigToFooterCompression maps config string to footer compression bytes.
// Returns error if compression is not "none", "snappy", or "zstd".
func ConfigToFooterCompression(compression string) ([4]byte, error) {
	switch compression {
	case "none", "":
		return CompressionNone, nil
	case "snappy":
		return CompressionSnappy, nil
	case "zstd":
		return CompressionZstd, nil
	default:
		return [4]byte{}, fmt.Errorf("raid3: invalid compression %q: only none, snappy, and zstd are supported", compression)
	}
}

// errUnsupportedCompression is returned when the footer indicates an unsupported compression (e.g. LZ4).
var errUnsupportedCompression = errors.New("raid3: unsupported compression in object footer")

// inventoryLength returns the byte length of the compression inventory for numBlocks blocks (numBlocks * 4).
func inventoryLength(numBlocks int) int { return numBlocks * 4 }

// blockIndex returns the block index for the given byte offset (0-based).
func blockIndex(byteOffset int64) int { return int(byteOffset / BlockSize) }

// fullStreamRangeForBlocks returns the byte range in the full compressed stream
// for blocks [firstBlock, lastBlock]. The full stream is the concatenation of
// compressed blocks. Returns (fullStart, fullLen) for range [fullStart, fullStart+fullLen).
// Caller must ensure 0 <= firstBlock <= lastBlock < len(inventory).
func fullStreamRangeForBlocks(inventory []uint32, firstBlock, lastBlock int) (fullStart, fullLen int64) {
	for i := 0; i < firstBlock; i++ {
		fullStart += int64(inventory[i])
	}
	for i := firstBlock; i <= lastBlock; i++ {
		fullLen += int64(inventory[i])
	}
	return fullStart, fullLen
}

// alignFullStreamToPairs rounds fullStart down and fullLen up so the range covers complete
// even/odd pairs. The StreamMerger reads in pairs and requires len(even) >= len(odd).
func alignFullStreamToPairs(fullStart, fullLen int64) (alignedStart, alignedLen int64) {
	if fullLen <= 0 {
		return fullStart, fullLen
	}
	alignedStart = (fullStart / 2) * 2
	endByte := fullStart + fullLen - 1
	alignedEnd := (endByte/2)*2 + 2
	alignedLen = alignedEnd - alignedStart
	return alignedStart, alignedLen
}

// particleRangesForFullStream maps full stream byte range [fullStart, fullStart+fullLen)
// to particle byte indices. Full stream byte P maps to even[P/2] (P even) or odd[(P-1)/2] (P odd).
// Returns inclusive End for use with RangeOption.
// If fullLen is 0, returns zeros; caller must avoid Open with empty range.
func particleRangesForFullStream(fullStart, fullLen int64) (evenStart, evenEnd, oddStart, oddEnd int64) {
	if fullLen <= 0 {
		return 0, 0, 0, 0
	}
	// Even: full bytes 0,2,4,... at particle index i = fullByte/2
	// Start: ceil(fullStart/2) = (fullStart+1)/2, End: floor((fullStart+fullLen-1)/2)
	evenStart = (fullStart + 1) / 2
	lastByte := fullStart + fullLen - 1
	evenEnd = lastByte / 2
	if evenEnd < evenStart {
		evenEnd = evenStart - 1
	}
	// Odd: full bytes 1,3,5,... at particle index j = (fullByte-1)/2
	// Start: fullStart/2, End: (lastOdd-1)/2 where lastOdd is largest odd position in range
	oddStart = fullStart / 2
	if (fullStart+fullLen-1)%2 == 1 {
		oddEnd = (fullStart + fullLen - 2) / 2
	} else {
		oddEnd = (fullStart + fullLen - 3) / 2
	}
	if oddEnd < oddStart {
		oddEnd = oddStart - 1
	}
	return evenStart, evenEnd, oddStart, oddEnd
}

// uncompressedInventory builds a virtual block inventory for uncompressed content.
// Content is treated as blocks of BlockSize (128 KiB); the last block may be smaller.
// Returns inventory suitable for fullStreamRangeForBlocks.
func uncompressedInventory(contentLength int64) []uint32 {
	if contentLength <= 0 {
		return nil
	}
	n := int((contentLength + BlockSize - 1) / BlockSize)
	inv := make([]uint32, n)
	for i := 0; i < n-1; i++ {
		inv[i] = uint32(BlockSize)
	}
	inv[n-1] = uint32(lastBlockUncompressedSize(contentLength))
	return inv
}

// lastBlockUncompressedSize returns the uncompressed size of the last block.
// For contentLength % BlockSize == 0 the last block is full (BlockSize); otherwise it's the remainder.
func lastBlockUncompressedSize(contentLength int64) int {
	if contentLength == 0 {
		return 0
	}
	r := int(contentLength % BlockSize)
	if r == 0 {
		return BlockSize
	}
	return r
}

// compressBlock compresses a single block using the given compression. Returns the compressed bytes.
func compressBlock(block []byte, compression [4]byte) ([]byte, error) {
	if compression == CompressionNone {
		return append([]byte(nil), block...), nil
	}
	if compression == CompressionSnappy {
		return snappy.Encode(nil, block), nil
	}
	if compression == CompressionZstd {
		enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstdDefaultLevel))
		if err != nil {
			return nil, err
		}
		defer enc.Close()
		return enc.EncodeAll(block, nil), nil
	}
	return nil, errUnsupportedCompression
}

// decompressBlock decompresses a single block using the given compression. Returns the decompressed bytes.
func decompressBlock(compressed []byte, compression [4]byte) ([]byte, error) {
	if compression == CompressionNone {
		return append([]byte(nil), compressed...), nil
	}
	if compression == CompressionSnappy {
		return snappy.Decode(nil, compressed)
	}
	if compression == CompressionZstd {
		dec, err := zstd.NewReader(nil)
		if err != nil {
			return nil, err
		}
		defer dec.Close()
		return dec.DecodeAll(compressed, nil)
	}
	return nil, errUnsupportedCompression
}

// buildInventory builds the inventory bytes from compressed block sizes (N × 4-byte little-endian uint32).
func buildInventory(sizes []uint32) []byte {
	b := make([]byte, len(sizes)*4)
	for i, s := range sizes {
		binary.LittleEndian.PutUint32(b[i*4:], s)
	}
	return b
}

// parseInventory parses inventory bytes into a slice of compressed block sizes.
func parseInventory(buf []byte) []uint32 {
	n := len(buf) / 4
	out := make([]uint32, n)
	for i := 0; i < n; i++ {
		out[i] = binary.LittleEndian.Uint32(buf[i*4:])
	}
	return out
}

// zstdDefaultLevel is the default zstd encoder level (good balance of speed and ratio).
var zstdDefaultLevel = zstd.SpeedDefault

// newCompressingReader returns a reader that compresses data from src using the given algorithm.
// For "none" returns src unchanged. For "snappy" or "zstd" returns a reader that streams compressed output.
// The hasher (or other caller) remains the source of truth for uncompressed size and hashes.
func newCompressingReader(src io.Reader, algo string) (io.Reader, error) {
	if algo == "none" || algo == "" {
		return src, nil
	}
	pr, pw := io.Pipe()
	switch algo {
	case "snappy":
		go func() {
			sw := snappy.NewWriter(pw)
			_, err := io.Copy(sw, src)
			if err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			if err := sw.Close(); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			_ = pw.Close()
		}()
		return pr, nil
	case "zstd":
		go func() {
			enc, err := zstd.NewWriter(pw, zstd.WithEncoderLevel(zstdDefaultLevel))
			if err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			_, err = io.Copy(enc, src)
			if err != nil {
				_ = enc.Close()
				_ = pw.CloseWithError(err)
				return
			}
			if err := enc.Close(); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			_ = pw.Close()
		}()
		return pr, nil
	default:
		_ = pw.Close()
		return nil, fmt.Errorf("raid3: unsupported compression algorithm %q", algo)
	}
}

// decompressReadCloser wraps a snappy.Reader and closes the underlying ReadCloser on Close.
type decompressReadCloser struct {
	*snappy.Reader
	src io.Closer
}

func (d *decompressReadCloser) Close() error {
	return d.src.Close()
}

// zstdDecompressReadCloser wraps a zstd Decoder and closes both the decoder and the source on Close.
type zstdDecompressReadCloser struct {
	dec *zstd.Decoder
	src io.Closer
}

func (z *zstdDecompressReadCloser) Read(p []byte) (n int, err error) {
	return z.dec.Read(p)
}

func (z *zstdDecompressReadCloser) Close() error {
	z.dec.Close()
	return z.src.Close()
}

// blockDecompressReadCloser reads compressed blocks from src, decompresses each using inventory, and yields decompressed bytes.
type blockDecompressReadCloser struct {
	src         io.ReadCloser
	inventory   []uint32
	compression [4]byte
	blockIdx    int
	decBuf      []byte
	decPos      int
	srcBuf      []byte
}

func newBlockDecompressReadCloser(src io.ReadCloser, inventory []uint32, compression [4]byte) *blockDecompressReadCloser {
	return &blockDecompressReadCloser{
		src:         src,
		inventory:   inventory,
		compression: compression,
		srcBuf:      make([]byte, 0, BlockSize*2), // max compressed block can be larger than BlockSize
	}
}

func (b *blockDecompressReadCloser) Read(p []byte) (n int, err error) {
	for len(p) > 0 {
		if b.decPos < len(b.decBuf) {
			copied := copy(p, b.decBuf[b.decPos:])
			b.decPos += copied
			n += copied
			return n, nil
		}
		if b.blockIdx >= len(b.inventory) {
			return n, io.EOF
		}
		compressedLen := int(b.inventory[b.blockIdx])
		b.blockIdx++
		if compressedLen == 0 {
			continue
		}
		if cap(b.srcBuf) < compressedLen {
			b.srcBuf = make([]byte, compressedLen)
		}
		b.srcBuf = b.srcBuf[:compressedLen]
		if _, err := io.ReadFull(b.src, b.srcBuf); err != nil {
			return n, err
		}
		b.decBuf, err = decompressBlock(b.srcBuf, b.compression)
		if err != nil {
			return n, err
		}
		b.decPos = 0
	}
	return n, nil
}

func (b *blockDecompressReadCloser) Close() error {
	return b.src.Close()
}

// newDecompressingReadCloser returns a ReadCloser that decompresses data from rc using the footer's compression.
// If compression is CompressionNone, returns rc unchanged. If CompressionSnappy or CompressionZstd, wraps with the appropriate decoder.
// For any other value (e.g. LZ4) returns errUnsupportedCompression.
func newDecompressingReadCloser(rc io.ReadCloser, compression [4]byte) (io.ReadCloser, error) {
	if compression == CompressionNone {
		return rc, nil
	}
	if compression == CompressionSnappy {
		return &decompressReadCloser{Reader: snappy.NewReader(rc), src: rc}, nil
	}
	if compression == CompressionZstd {
		dec, err := zstd.NewReader(rc)
		if err != nil {
			_ = rc.Close()
			return nil, err
		}
		return &zstdDecompressReadCloser{dec: dec, src: rc}, nil
	}
	_ = rc.Close()
	return nil, errUnsupportedCompression
}
