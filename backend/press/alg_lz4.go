package press

// This file implements the LZ4 algorithm.
import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"

	"github.com/buengese/xxh32"
	lz4 "github.com/pierrec/lz4"
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
//var LZ4Header = []byte{0x04, 0x22, 0x4d, 0x18, 0x70, 0x50, 0x84}

// LZ4Footer - Footer of our LZ4 file
var LZ4Footer = []byte{0x00, 0x00, 0x00, 0x00} // This is just an empty block

const (
	frameMagic uint32 = 0x184D2204

	compressedBlockFlag = 1 << 31
	compressedBlockMask = compressedBlockFlag - 1
)

// AlgLz4 is the Lz4 Compression algorithm
type AlgLz4 struct {
	Header lz4.Header
	buf    [19]byte // magic number(4) + header(flags(2)+[Size(8)+DictID(4)]+checksum(1)) does not exceed 19 bytes
}

// InitializeLz4 creates an Lz4 compression algorithm
func InitializeLz4(bs uint32, blockChecksum bool) Algorithm {
	a := new(AlgLz4)
	a.Header.Reset()
	a.Header = lz4.Header{
		BlockChecksum: blockChecksum,
		BlockMaxSize:  int(bs),
	}
	return a
}

// GetFileExtension returns file extension
func (a *AlgLz4) GetFileExtension() string {
	return ".lz4"
}

// GetHeader returns the Lz4 compression header
func (a *AlgLz4) GetHeader() []byte {
	// Size is optional.
	buf := a.buf[:]

	// Set the fixed size data: magic number, block max size and flags.
	binary.LittleEndian.PutUint32(buf[0:], frameMagic)
	flg := byte(lz4.Version << 6)
	flg |= 1 << 5 // No block dependency.
	if a.Header.BlockChecksum {
		flg |= 1 << 4
	}
	if a.Header.Size > 0 {
		flg |= 1 << 3
	}
	buf[4] = flg
	buf[5] = blockSizeValueToIndex(a.Header.BlockMaxSize) << 4

	// Current buffer size: magic(4) + flags(1) + block max size (1).
	n := 6
	if a.Header.Size > 0 {
		binary.LittleEndian.PutUint64(buf[n:], a.Header.Size)
		n += 8
	}

	// The header checksum includes the flags, block max size and optional Size.
	buf[n] = byte(xxh32.ChecksumZero(buf[4:n]) >> 8 & 0xFF)

	// Header ready, write it out.
	return buf[0 : n+1]
}

// GetFooter returns
func (a *AlgLz4) GetFooter() []byte {
	return LZ4Footer
}

// CompressBlock that compresses a block using lz4
func (a *AlgLz4) CompressBlock(in []byte, out io.Writer) (compressedSize uint32, uncompressedSize uint64, err error) {
	if len(in) > 0 {
		n, err := a.compressBlock(in, out)
		if err != nil {
			return 0, 0, err
		}
		return n, uint64(len(in)), nil
	}

	return 0, 0, nil
}

// compressBlock compresses a block.
func (a *AlgLz4) compressBlock(data []byte, dst io.Writer) (uint32, error) {
	zdata := make([]byte, a.Header.BlockMaxSize) // The compressed block size cannot exceed the input's.
	var zn int
	if level := a.Header.CompressionLevel; level != 0 {
		zn, _ = lz4.CompressBlockHC(data, zdata, level)
	} else {
		var hashTable [1 << 16]int
		zn, _ = lz4.CompressBlock(data, zdata, hashTable[:])
	}

	var bLen uint32
	if zn > 0 && zn < len(data) {
		// Compressible and compressed size smaller than uncompressed: ok!
		bLen = uint32(zn)
		zdata = zdata[:zn]
	} else {
		// Uncompressed block.
		bLen = uint32(len(data)) | compressedBlockFlag
		zdata = data
	}

	// Write the block.
	if err := a.writeUint32(bLen, dst); err != nil {
		return 0, err
	}
	_, err := dst.Write(zdata)
	if err != nil {
		return 0, err
	}

	if !a.Header.BlockChecksum {
		return bLen, nil
	}
	checksum := xxh32.ChecksumZero(zdata)
	if err := a.writeUint32(checksum, dst); err != nil {
		return 0, err
	}
	return bLen, nil
}

// writeUint32 writes a uint32 to the underlying writer.
func (a *AlgLz4) writeUint32(x uint32, dst io.Writer) error {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, x)
	_, err := dst.Write(buf)
	return err
}

func blockSizeValueToIndex(size int) byte {
	return 4 + byte(bits.TrailingZeros(uint(size)>>16)/2)
}

// DecompressBlock decompresses Lz4 compressed block
func (a *AlgLz4) DecompressBlock(in io.Reader, out io.Writer, BlockSize uint32) (n int, err error) {
	// Get our compressed data
	var b bytes.Buffer
	_, err = io.Copy(&b, in)
	if err != nil {
		return 0, err
	}
	zdata := b.Bytes()
	bLen := binary.LittleEndian.Uint32(zdata[:4])

	if bLen&compressedBlockFlag > 0 {
		// Uncompressed block.
		bLen &= compressedBlockMask

		if bLen > BlockSize {
			return 0, fmt.Errorf("lz4: invalid block size: %d", bLen)
		}
		data := zdata[4 : bLen+4]

		if a.Header.BlockChecksum {
			checksum := binary.LittleEndian.Uint32(zdata[4+bLen:])

			if h := xxh32.ChecksumZero(data); h != checksum {
				return 0, fmt.Errorf("lz4: invalid block checksum: got %x; expected %x", h, checksum)
			}
		}
		_, err := out.Write(data)
		return len(data), err
	}

	// compressed block
	if bLen > BlockSize {
		return 0, fmt.Errorf("lz4: invalid block size: %d", bLen)
	}

	if a.Header.BlockChecksum {
		checksum := binary.LittleEndian.Uint32(zdata[4+bLen:])

		if h := xxh32.ChecksumZero(zdata[4 : bLen+4]); h != checksum {
			return 0, fmt.Errorf("lz4: invalid block checksum: got %x; expected %x", h, checksum)
		}
	}

	data := make([]byte, BlockSize)
	n, err = lz4.UncompressBlock(zdata[4:bLen+4], data)
	if err != nil {
		return 0, err
	}
	_, err = out.Write(data[:n])
	return n, err
}
