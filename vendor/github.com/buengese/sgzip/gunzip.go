// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sgzip implements a seekable version of gzip format compressed files,
// compliant with RFC 1952.
//
// This is a drop in replacement for "compress/gzip".
// This will split compression into blocks that are compressed in parallel.
// This can be useful for compressing big amounts of data.
// The gzip decompression has not been modified, but remains in the package,
// so you can use it as a complete replacement for "compress/gzip".
//
// See more at https://github.com/klauspost/pgzip
package sgzip

import (
	"bufio"
	"errors"
	"hash"
	"hash/crc32"
	"io"
	"sync"
	"time"

	"github.com/klauspost/compress/flate"
)

const (
	gzipID1     = 0x1f
	gzipID2     = 0x8b
	gzipDeflate = 8
	flagText    = 1 << 0
	flagHdrCrc  = 1 << 1
	flagExtra   = 1 << 2
	flagName    = 1 << 3
	flagComment = 1 << 4
)

func makeReader(r io.Reader) flate.Reader {
	if rr, ok := r.(flate.Reader); ok {
		return rr
	}
	return bufio.NewReader(r)
}

var (
	// ErrUnsupported is returned when atempting an unsupported operation.
	ErrUnsupported = errors.New("gzip: unsupported operation")
	// ErrChecksum is returned when reading GZIP data that has an invalid checksum.
	ErrChecksum = errors.New("gzip: invalid checksum")
	// ErrHeader is returned when reading GZIP data that has an invalid header.
	ErrHeader = errors.New("gzip: invalid header")
	// ErrInvalidSeek is returned when attempting to seek to negative position or beyond the file size.
	ErrInvalidSeek = errors.New("gzip: invalid seek position")
)

// The gzip file stores a header giving metadata about the compressed file.
// That header is exposed as the fields of the Writer and Reader structs.
type Header struct {
	Comment string    // comment
	Extra   []byte    // "extra data"
	ModTime time.Time // modification time
	Name    string    // file name
	OS      byte      // operating system type
}

// A Reader is an io.Reader that can be read to retrieve
// uncompressed data from a gzip-format compressed file.
//
// In general, a gzip file can be a concatenation of gzip files,
// each with its own header.  Reads from the Reader
// return the concatenation of the uncompressed data of each.
// Only the first header is recorded in the Reader fields.
//
// Gzip files store a length and checksum of the uncompressed data.
// The Reader will return a ErrChecksum when Read
// reaches the end of the uncompressed data if it does not
// have the expected length or checksum.  Clients should treat data
// returned by Read as tentative until they receive the io.EOF
// marking the end of the data.
type Reader struct {
	Header
	r            io.Reader
	bufr         flate.Reader
	decompressor io.ReadCloser
	digest       hash.Hash32
	size         uint32
	pos          int64
	flg          byte
	buf          [512]byte
	err          error
	closeErr     chan error
	multistream  bool
	canSeek      bool

	readAhead        chan read
	roff             int // read offset
	current          []byte
	closeReader      chan struct{}
	lastBlock        bool
	blockSize        int
	concurrentBlocks int
	blockOffset      int

	blockStarts    []int64 // The start of each block. These will be recovered from the block sizes
	isize          int64   // Size of the extracted data
	verifyChecksum bool    // verify checksum and size - not possible if the stream has been seeked

	activeRA bool       // Indication if readahead is active
	mu       sync.Mutex // Lock for above

	blockPool chan []byte
}

type read struct {
	b   []byte
	err error
}

// NewReader creates a new Reader reading the given reader.
// The implementation buffers input and may read more data than necessary from r.
// It is the caller's responsibility to call Close on the Reader when done.
func NewReader(r io.Reader) (*Reader, error) {
	z := new(Reader)
	z.concurrentBlocks = defaultBlocks
	z.blockSize = defaultBlockSize
	z.bufr = makeReader(r)
	z.digest = crc32.NewIEEE()

	z.roff = 0
	z.canSeek = false
	z.multistream = true
	z.verifyChecksum = true

	z.blockPool = make(chan []byte, z.concurrentBlocks)
	for i := 0; i < z.concurrentBlocks; i++ {
		z.blockPool <- make([]byte, z.blockSize)
	}
	if err := z.readHeader(true); err != nil {
		return nil, err
	}
	return z, nil
}

// NewReaderN creates a new Reader reading the given reader.
// The implementation buffers input and may read more data than necessary from r.
// It is the caller's responsibility to call Close on the Reader when done.
//
// With this you can control the approximate size of your blocks,
// as well as how many blocks you want to have prefetched.
//
// Default values for this is blockSize = 250000, blocks = 16,
// meaning up to 16 blocks of maximum 250000 bytes will be
// prefetched.
func NewReaderN(r io.Reader, blockSize, blocks int) (*Reader, error) {
	z := new(Reader)
	z.concurrentBlocks = blocks
	z.blockSize = blockSize
	z.bufr = makeReader(r)
	z.digest = crc32.NewIEEE()

	z.roff = 0
	z.canSeek = false
	z.multistream = true
	z.verifyChecksum = true

	// Account for too small values
	if z.concurrentBlocks <= 0 {
		z.concurrentBlocks = defaultBlocks
	}
	if z.blockSize <= 512 {
		z.blockSize = defaultBlockSize
	}
	z.blockPool = make(chan []byte, z.concurrentBlocks)
	for i := 0; i < z.concurrentBlocks; i++ {
		z.blockPool <- make([]byte, z.blockSize)
	}
	if err := z.readHeader(true); err != nil {
		return nil, err
	}
	return z, nil
}

// NewSeekingReader creates a new Reader reading the given reader.
// This is a special reader that allows seeking in the compressed file
// using the supplied metadata.
// It is the caller's responsibility to call Close on the Reader when done.
func NewSeekingReader(r io.ReadSeeker, meta *GzipMetadata) (*Reader, error) {
	z := new(Reader)
	z.concurrentBlocks = defaultBlocks
	z.blockSize = meta.BlockSize
	z.r = r
	z.bufr = makeReader(r)
	z.digest = crc32.NewIEEE()

	z.roff = 0
	z.canSeek = true
	z.multistream = false
	z.verifyChecksum = true

	z.blockStarts = parseBlockData(meta.BlockData, meta.BlockSize)
	z.isize = meta.Size

	z.blockPool = make(chan []byte, z.concurrentBlocks)
	for i := 0; i < z.concurrentBlocks; i++ {
		z.blockPool <- make([]byte, z.blockSize)
	}
	if err := z.readHeader(true); err != nil {
		return nil, err
	}
	return z, nil
}

// NewReaderAt creates a new Reader reading the given reader.
// This is a special reader that starts at an offset and allows
// seeking in the compressed file using the supplied metadata.
// It is the caller's responsibility to call Close on the Reader when done.
func NewReaderAt(r io.ReadSeeker, meta *GzipMetadata, pos int64) (*Reader, error) {
	z := new(Reader)
	z.concurrentBlocks = defaultBlocks
	z.blockSize = meta.BlockSize
	z.r = r
	z.bufr = makeReader(r)
	z.digest = crc32.NewIEEE()

	z.pos = pos
	z.roff = 0
	z.canSeek = true
	z.multistream = false
	z.verifyChecksum = false

	z.blockStarts = parseBlockData(meta.BlockData, meta.BlockSize)
	z.isize = meta.Size

	blockNumber := z.pos / int64(z.blockSize)
	blockStart := z.blockStarts[blockNumber]        // Start position of blocks to read
	z.blockOffset = int(z.pos % int64(z.blockSize)) // Offset of data to read in blocks to read

	// Seek underlying readseeker
	_, err := z.r.(io.ReadSeeker).Seek(blockStart, io.SeekStart)
	if err != nil {
		return nil, err
	}
	z.bufr = makeReader(z.r)
	z.blockPool = make(chan []byte, z.concurrentBlocks)
	for i := 0; i < z.concurrentBlocks; i++ {
		z.blockPool <- make([]byte, z.blockSize)
	}

	z.decompressor = flate.NewReader(z.bufr)
	z.doReadAhead()
	return z, nil
}

// Parses block data. Returns the number of blocks, the block start locations for each block, and the decompressed size of the entire file.
func parseBlockData(blockData []uint32, BlockSize int) (blockStarts []int64) {
	numBlocks := len(blockData)
	blockStarts = make([]int64, numBlocks+1) // Starts with start of first block (and end of header), ends with end of last block
	currentBlockPosition := int64(0)
	for i := 0; i < numBlocks; i++ { // Loop through block data, getting starts of blocks.
		currentBlockSize := blockData[i]
		currentBlockPosition += int64(currentBlockSize)
		blockStarts[i] = currentBlockPosition
	}
	blockStarts[numBlocks] = currentBlockPosition // End of last block

	return blockStarts
}

// Reset discards the Reader z's state and makes it equivalent to the
// result of its original state from NewReader, but reading from r instead.
// This permits reusing a Reader rather than allocating a new one.
func (z *Reader) Reset(r io.Reader) error {
	z.killReadAhead()
	z.bufr = makeReader(r)
	z.digest = crc32.NewIEEE()
	z.size = 0
	z.pos = 0
	z.roff = 0
	z.err = nil
	z.canSeek = false
	z.multistream = true
	z.verifyChecksum = true

	// Account for uninitialized values
	if z.concurrentBlocks <= 0 {
		z.concurrentBlocks = defaultBlocks
	}
	if z.blockSize <= 512 {
		z.blockSize = defaultBlockSize
	}

	z.blockPool = make(chan []byte, z.concurrentBlocks)
	for i := 0; i < z.concurrentBlocks; i++ {
		z.blockPool <- make([]byte, z.blockSize)
	}

	return z.readHeader(true)
}

// Seek ...
func (z *Reader) Seek(offset int64, whence int) (int64, error) {
	z.killReadAhead()
	if !z.canSeek {
		return z.pos, ErrUnsupported
	}

	if whence == io.SeekStart {
		z.pos = offset
	} else if whence == io.SeekCurrent {
		z.pos += offset
	} else if whence == io.SeekEnd {
		z.pos = z.isize + offset
	}
	if z.pos < 0 || z.pos > z.isize {
		return z.pos, ErrInvalidSeek
	}
	pos := z.pos

	// Calculate seek position
	blockNumber := pos / int64(z.blockSize)
	blockStart := z.blockStarts[blockNumber]      // Start position of blocks to read
	z.blockOffset = int(pos % int64(z.blockSize)) // Offset of data to read in blocks to read

	// Seek underlying readseeker
	_, err := z.r.(io.ReadSeeker).Seek(blockStart, io.SeekStart)
	if err != nil {
		return pos, err
	}

	// Reset everything
	z.bufr = makeReader(z.r)
	z.size = 0
	z.roff = 0
	z.err = nil
	z.verifyChecksum = false

	// Account for uninitialized values
	if z.concurrentBlocks <= 0 {
		z.concurrentBlocks = defaultBlocks
	}
	if z.blockSize <= 512 {
		z.blockSize = defaultBlockSize
	}

	z.blockPool = make(chan []byte, z.concurrentBlocks)
	for i := 0; i < z.concurrentBlocks; i++ {
		z.blockPool <- make([]byte, z.blockSize)
	}

	// We are not reading the header so we have to this here
	z.decompressor = flate.NewReader(z.bufr)
	z.doReadAhead()
	return pos, err
}

// Multistream controls whether the reader supports multistream files.
//
// If enabled (the default), the Reader expects the input to be a sequence
// of individually gzipped data streams, each with its own header and
// trailer, ending at EOF. The effect is that the concatenation of a sequence
// of gzipped files is treated as equivalent to the gzip of the concatenation
// of the sequence. This is standard behavior for gzip readers.
//
// Calling Multistream(false) disables this behavior; disabling the behavior
// can be useful when reading file formats that distinguish individual gzip
// data streams or mix gzip data streams with other data streams.
// In this mode, when the Reader reaches the end of the data stream,
// Read returns io.EOF. If the underlying reader implements io.ByteReader,
// it will be left positioned just after the gzip stream.
// To start the next stream, call z.Reset(r) followed by z.Multistream(false).
// If there is no next stream, z.Reset(r) will return io.EOF.
func (z *Reader) Multistream(ok bool) {
	z.multistream = ok
}

// GZIP (RFC 1952) is little-endian, unlike ZLIB (RFC 1950).
func get4(p []byte) uint32 {
	return uint32(p[0]) | uint32(p[1])<<8 | uint32(p[2])<<16 | uint32(p[3])<<24
}

func (z *Reader) readString() (string, error) {
	var err error
	needconv := false
	for i := 0; ; i++ {
		if i >= len(z.buf) {
			return "", ErrHeader
		}
		z.buf[i], err = z.bufr.ReadByte()
		if err != nil {
			return "", err
		}
		if z.buf[i] > 0x7f {
			needconv = true
		}
		if z.buf[i] == 0 {
			// GZIP (RFC 1952) specifies that strings are NUL-terminated ISO 8859-1 (Latin-1).
			if needconv {
				s := make([]rune, 0, i)
				for _, v := range z.buf[0:i] {
					s = append(s, rune(v))
				}
				return string(s), nil
			}
			return string(z.buf[0:i]), nil
		}
	}
}

func (z *Reader) read2() (uint32, error) {
	_, err := io.ReadFull(z.bufr, z.buf[0:2])
	if err != nil {
		return 0, err
	}
	return uint32(z.buf[0]) | uint32(z.buf[1])<<8, nil
}

func (z *Reader) readHeader(save bool) error {
	_, err := io.ReadFull(z.bufr, z.buf[0:10])
	if err != nil {
		z.err = err
		return err
	}
	if z.buf[0] != gzipID1 || z.buf[1] != gzipID2 || z.buf[2] != gzipDeflate {
		return ErrHeader
	}
	z.flg = z.buf[3]
	if save {
		z.ModTime = time.Unix(int64(get4(z.buf[4:8])), 0)
		// z.buf[8] is xfl, ignored
		z.OS = z.buf[9]
	}
	z.digest.Reset()
	z.digest.Write(z.buf[0:10])

	if z.flg&flagExtra != 0 {
		n, err := z.read2()
		if err != nil {
			return err
		}
		data := make([]byte, n)
		if _, err = io.ReadFull(z.bufr, data); err != nil {
			return err
		}
		if save {
			z.Extra = data
		}
	}

	var s string
	if z.flg&flagName != 0 {
		if s, err = z.readString(); err != nil {
			return err
		}
		if save {
			z.Name = s
		}
	}

	if z.flg&flagComment != 0 {
		if s, err = z.readString(); err != nil {
			return err
		}
		if save {
			z.Comment = s
		}
	}

	if z.flg&flagHdrCrc != 0 {
		n, err := z.read2()
		if err != nil {
			return err
		}
		sum := z.digest.Sum32() & 0xFFFF
		if n != sum {
			return ErrHeader
		}
	}

	z.digest.Reset()
	z.decompressor = flate.NewReader(z.bufr)
	z.doReadAhead()
	return nil
}

func (z *Reader) killReadAhead() error {
	z.mu.Lock()
	defer z.mu.Unlock()
	if z.activeRA {
		if z.closeReader != nil {
			close(z.closeReader)
		}

		// Wait for decompressor to be closed and return error, if any.
		e, ok := <-z.closeErr
		z.activeRA = false
		if !ok {
			// Channel is closed, so if there was any error it has already been returned.
			return nil
		}
		return e
	}
	return nil
}

// Starts readahead.
// Will return on error (including io.EOF)
// or when z.closeReader is closed.
func (z *Reader) doReadAhead() {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.activeRA = true

	if z.concurrentBlocks <= 0 {
		z.concurrentBlocks = defaultBlocks
	}
	if z.blockSize <= 512 {
		z.blockSize = defaultBlockSize
	}
	ra := make(chan read, z.concurrentBlocks)
	z.readAhead = ra
	closeReader := make(chan struct{}, 0)
	z.closeReader = closeReader
	z.lastBlock = false
	closeErr := make(chan error, 1)
	z.closeErr = closeErr
	z.size = 0
	z.current = nil
	decomp := z.decompressor

	go func() {
		// We hold a local reference to digest, since
		// it way be changed by reset.
		digest := z.digest
		var wg sync.WaitGroup
		defer func() {
			wg.Wait()
			closeErr <- decomp.Close()
			close(closeErr)
			close(ra)
		}()
		for {
			var buf []byte
			select {
			case buf = <-z.blockPool:
			case <-closeReader:
				return
			}
			buf = buf[0:z.blockSize]
			// Try to fill the buffer
			n, err := io.ReadFull(decomp, buf)
			if err == io.ErrUnexpectedEOF {
				if n > 0 {
					err = nil
				} else {
					// If we got zero bytes, we need to establish if
					// we reached end of stream or truncated stream.
					_, err = decomp.Read([]byte{})
					if err == io.EOF {
						err = nil
					}
				}
			}
			if n < len(buf) {
				buf = buf[0:n]
			}
			wg.Wait()
			wg.Add(1)
			go func() {
				digest.Write(buf)
				wg.Done()
			}()
			z.size += uint32(n)
			z.pos += int64(n)

			// If we return any error, out digest must be ready
			if err != nil {
				wg.Wait()
			}
			select {
			case z.readAhead <- read{b: buf, err: err}:
			case <-closeReader:
				// Sent on close, we don't care about the next results
				return
			}
			if err != nil {
				return
			}
		}
	}()
}

func (z *Reader) Read(p []byte) (n int, err error) {
	if z.err != nil {
		return 0, z.err
	}
	if len(p) == 0 {
		return 0, nil
	}

	for {
		if len(z.current) == 0 && !z.lastBlock {
			read := <-z.readAhead

			if read.err != nil {
				// If not nil, the reader will have exited
				z.closeReader = nil

				if read.err != io.EOF {
					z.err = read.err
					return
				}
				if read.err == io.EOF {
					z.lastBlock = true
					err = nil
				}
			}
			z.current = read.b
			z.roff = z.blockOffset
			z.blockOffset = 0
		}
		avail := z.current[z.roff:]
		if len(p) >= len(avail) {
			// If len(p) >= len(current), return all content of current
			n = copy(p, avail)
			z.blockPool <- z.current
			z.current = nil
			if z.lastBlock {
				err = io.EOF
				break
			}
		} else {
			// We copy as much as there is space for
			n = copy(p, avail)
			z.roff += n
		}
		return
	}

	// Finished file; check checksum + size.
	if _, err := io.ReadFull(z.bufr, z.buf[0:8]); err != nil {
		z.err = err
		return 0, err
	}
	if z.verifyChecksum {
		crc32, isize := get4(z.buf[0:4]), get4(z.buf[4:8])
		sum := z.digest.Sum32()
		if sum != crc32 || isize != z.size {
			z.err = ErrChecksum
			return 0, z.err
		}
	}

	// File is ok; should we attempt reading one more?
	if !z.multistream {
		return 0, io.EOF
	}

	// Is there another?
	if err = z.readHeader(false); err != nil {
		z.err = err
		return
	}

	// Yes.  Reset and read from it.
	return z.Read(p)
}

// WriteTo writes data to w until the buffer is drained or an error occurs.
// The return value n is the number of bytes written; it always fits into an
// int, but it is int64 to match the io.WriterTo interface. Any error
// encountered during the write is also returned.
func (z *Reader) WriteTo(w io.Writer) (n int64, err error) {
	var buf []byte
	var total int64 = 0
	for {
		if z.err != nil {
			return total, z.err
		}
		// We write both to output and digest.
		for {
			// Read from input
			read := <-z.readAhead
			if read.err != nil {
				// If not nil, the reader will have exited
				z.closeReader = nil

				if read.err != io.EOF {
					z.err = read.err
					return total, z.err
				}
				if read.err == io.EOF {
					z.lastBlock = true
					err = nil
				}
			}

			// discard initial bytes if we have a block offset
			if z.blockOffset > 0 {
				buf = read.b[z.blockOffset:]
				z.blockOffset = 0
			} else {
				buf = read.b
			}
			// Write what we got
			n, err := w.Write(buf)
			if n != len(buf) {
				return total, io.ErrShortWrite
			}
			total += int64(n)
			if err != nil {
				return total, err
			}
			// Put block back
			z.blockPool <- read.b
			if z.lastBlock {
				break
			}
		}

		// Finished file; check checksum + size.
		if _, err := io.ReadFull(z.bufr, z.buf[0:8]); err != nil {
			z.err = err
			return total, err
		}
		if z.verifyChecksum {
			crc32, isize := get4(z.buf[0:4]), get4(z.buf[4:8])
			sum := z.digest.Sum32()
			if sum != crc32 || isize != z.size {
				z.err = ErrChecksum
				return total, z.err
			}
		}
		// File is ok; should we attempt reading one more?
		if !z.multistream {
			return total, nil
		}

		// Is there another?
		err = z.readHeader(false)
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			z.err = err
			return total, err
		}
	}
}

// Close closes the Reader. It does not close the underlying io.Reader.
func (z *Reader) Close() error {
	return z.killReadAhead()
}
