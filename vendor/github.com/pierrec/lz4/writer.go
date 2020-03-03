package lz4

import (
	"encoding/binary"
	"fmt"
	"github.com/pierrec/lz4/internal/xxh32"
	"io"
	"runtime"
)

// zResult contains the results of compressing a block.
type zResult struct {
	size     uint32 // Block header
	data     []byte // Compressed data
	checksum uint32 // Data checksum
}

// Writer implements the LZ4 frame encoder.
type Writer struct {
	Header
	// Handler called when a block has been successfully written out.
	// It provides the number of bytes written.
	OnBlockDone func(size int)

	buf       [19]byte      // magic number(4) + header(flags(2)+[Size(8)+DictID(4)]+checksum(1)) does not exceed 19 bytes
	dst       io.Writer     // Destination.
	checksum  xxh32.XXHZero // Frame checksum.
	data      []byte        // Data to be compressed + buffer for compressed data.
	idx       int           // Index into data.
	hashtable [winSize]int  // Hash table used in CompressBlock().

	// For concurrency.
	c   chan chan zResult // Channel for block compression goroutines and writer goroutine.
	err error             // Any error encountered while writing to the underlying destination.
}

// NewWriter returns a new LZ4 frame encoder.
// No access to the underlying io.Writer is performed.
// The supplied Header is checked at the first Write.
// It is ok to change it before the first Write but then not until a Reset() is performed.
func NewWriter(dst io.Writer) *Writer {
	z := new(Writer)
	z.Reset(dst)
	return z
}

// WithConcurrency sets the number of concurrent go routines used for compression.
// A negative value sets the concurrency to GOMAXPROCS.
func (z *Writer) WithConcurrency(n int) *Writer {
	switch {
	case n == 0 || n == 1:
		z.c = nil
		return z
	case n < 0:
		n = runtime.GOMAXPROCS(0)
	}
	z.c = make(chan chan zResult, n)
	// Writer goroutine managing concurrent block compression goroutines.
	go func() {
		// Process next block compression item.
		for c := range z.c {
			// Read the next compressed block result.
			// Waiting here ensures that the blocks are output in the order they were sent.
			// The incoming channel is always closed as it indicates to the caller that
			// the block has been processed.
			res := <-c
			n := len(res.data)
			if n == 0 {
				// Notify the block compression routine that we are done with its result.
				// This is used when a sentinel block is sent to terminate the compression.
				close(c)
				return
			}
			// Write the block.
			if err := z.writeUint32(res.size); err != nil && z.err == nil {
				z.err = err
			}
			if _, err := z.dst.Write(res.data); err != nil && z.err == nil {
				z.err = err
			}
			if z.BlockChecksum {
				if err := z.writeUint32(res.checksum); err != nil && z.err == nil {
					z.err = err
				}
			}
			if isCompressed := res.size&compressedBlockFlag == 0; isCompressed {
				// It is now safe to release the buffer as no longer in use by any goroutine.
				putBuffer(cap(res.data), res.data)
			}
			if h := z.OnBlockDone; h != nil {
				h(n)
			}
			close(c)
		}
	}()
	return z
}

// newBuffers instantiates new buffers which size matches the one in Header.
// The returned buffers are for decompression and compression respectively.
func (z *Writer) newBuffers() {
	bSize := z.Header.BlockMaxSize
	buf := getBuffer(bSize)
	z.data = buf[:bSize] // Uncompressed buffer is the first half.
}

// freeBuffers puts the writer's buffers back to the pool.
func (z *Writer) freeBuffers() {
	// Put the buffer back into the pool, if any.
	putBuffer(z.Header.BlockMaxSize, z.data)
	z.data = nil
}

// writeHeader builds and writes the header (magic+header) to the underlying io.Writer.
func (z *Writer) writeHeader() error {
	// Default to 4Mb if BlockMaxSize is not set.
	if z.Header.BlockMaxSize == 0 {
		z.Header.BlockMaxSize = blockSize4M
	}
	// The only option that needs to be validated.
	bSize := z.Header.BlockMaxSize
	if !isValidBlockSize(z.Header.BlockMaxSize) {
		return fmt.Errorf("lz4: invalid block max size: %d", bSize)
	}
	// Allocate the compressed/uncompressed buffers.
	// The compressed buffer cannot exceed the uncompressed one.
	z.newBuffers()
	z.idx = 0

	// Size is optional.
	buf := z.buf[:]

	// Set the fixed size data: magic number, block max size and flags.
	binary.LittleEndian.PutUint32(buf[0:], frameMagic)
	flg := byte(Version << 6)
	flg |= 1 << 5 // No block dependency.
	if z.Header.BlockChecksum {
		flg |= 1 << 4
	}
	if z.Header.Size > 0 {
		flg |= 1 << 3
	}
	if !z.Header.NoChecksum {
		flg |= 1 << 2
	}
	buf[4] = flg
	buf[5] = blockSizeValueToIndex(z.Header.BlockMaxSize) << 4

	// Current buffer size: magic(4) + flags(1) + block max size (1).
	n := 6
	// Optional items.
	if z.Header.Size > 0 {
		binary.LittleEndian.PutUint64(buf[n:], z.Header.Size)
		n += 8
	}

	// The header checksum includes the flags, block max size and optional Size.
	buf[n] = byte(xxh32.ChecksumZero(buf[4:n]) >> 8 & 0xFF)
	z.checksum.Reset()

	// Header ready, write it out.
	if _, err := z.dst.Write(buf[0 : n+1]); err != nil {
		return err
	}
	z.Header.done = true
	if debugFlag {
		debug("wrote header %v", z.Header)
	}

	return nil
}

// Write compresses data from the supplied buffer into the underlying io.Writer.
// Write does not return until the data has been written.
func (z *Writer) Write(buf []byte) (int, error) {
	if !z.Header.done {
		if err := z.writeHeader(); err != nil {
			return 0, err
		}
	}
	if debugFlag {
		debug("input buffer len=%d index=%d", len(buf), z.idx)
	}

	zn := len(z.data)
	var n int
	for len(buf) > 0 {
		if z.idx == 0 && len(buf) >= zn {
			// Avoid a copy as there is enough data for a block.
			if err := z.compressBlock(buf[:zn]); err != nil {
				return n, err
			}
			n += zn
			buf = buf[zn:]
			continue
		}
		// Accumulate the data to be compressed.
		m := copy(z.data[z.idx:], buf)
		n += m
		z.idx += m
		buf = buf[m:]
		if debugFlag {
			debug("%d bytes copied to buf, current index %d", n, z.idx)
		}

		if z.idx < len(z.data) {
			// Buffer not filled.
			if debugFlag {
				debug("need more data for compression")
			}
			return n, nil
		}

		// Buffer full.
		if err := z.compressBlock(z.data); err != nil {
			return n, err
		}
		z.idx = 0
	}

	return n, nil
}

// compressBlock compresses a block.
func (z *Writer) compressBlock(data []byte) error {
	if !z.NoChecksum {
		_, _ = z.checksum.Write(data)
	}

	if z.c != nil {
		c := make(chan zResult)
		z.c <- c // Send now to guarantee order
		go writerCompressBlock(c, z.Header, data)
		return nil
	}

	zdata := z.data[z.Header.BlockMaxSize:cap(z.data)]
	// The compressed block size cannot exceed the input's.
	var zn int

	if level := z.Header.CompressionLevel; level != 0 {
		zn, _ = CompressBlockHC(data, zdata, level)
	} else {
		zn, _ = CompressBlock(data, zdata, z.hashtable[:])
	}

	var bLen uint32
	if debugFlag {
		debug("block compression %d => %d", len(data), zn)
	}
	if zn > 0 && zn < len(data) {
		// Compressible and compressed size smaller than uncompressed: ok!
		bLen = uint32(zn)
		zdata = zdata[:zn]
	} else {
		// Uncompressed block.
		bLen = uint32(len(data)) | compressedBlockFlag
		zdata = data
	}
	if debugFlag {
		debug("block compression to be written len=%d data len=%d", bLen, len(zdata))
	}

	// Write the block.
	if err := z.writeUint32(bLen); err != nil {
		return err
	}
	written, err := z.dst.Write(zdata)
	if err != nil {
		return err
	}
	if h := z.OnBlockDone; h != nil {
		h(written)
	}

	if !z.BlockChecksum {
		if debugFlag {
			debug("current frame checksum %x", z.checksum.Sum32())
		}
		return nil
	}
	checksum := xxh32.ChecksumZero(zdata)
	if debugFlag {
		debug("block checksum %x", checksum)
		defer func() { debug("current frame checksum %x", z.checksum.Sum32()) }()
	}
	return z.writeUint32(checksum)
}

// Flush flushes any pending compressed data to the underlying writer.
// Flush does not return until the data has been written.
// If the underlying writer returns an error, Flush returns that error.
func (z *Writer) Flush() error {
	if debugFlag {
		debug("flush with index %d", z.idx)
	}
	if z.idx == 0 {
		return nil
	}

	data := z.data[:z.idx]
	z.idx = 0
	if z.c == nil {
		return z.compressBlock(data)
	}
	if !z.NoChecksum {
		_, _ = z.checksum.Write(data)
	}
	c := make(chan zResult)
	z.c <- c
	writerCompressBlock(c, z.Header, data)
	return nil
}

func (z *Writer) close() error {
	if z.c == nil {
		return nil
	}
	// Send a sentinel block (no data to compress) to terminate the writer main goroutine.
	c := make(chan zResult)
	z.c <- c
	c <- zResult{}
	// Wait for the main goroutine to complete.
	<-c
	// At this point the main goroutine has shut down or is about to return.
	z.c = nil
	return z.err
}

// Close closes the Writer, flushing any unwritten data to the underlying io.Writer, but does not close the underlying io.Writer.
func (z *Writer) Close() error {
	if !z.Header.done {
		if err := z.writeHeader(); err != nil {
			return err
		}
	}
	if err := z.Flush(); err != nil {
		return err
	}
	if err := z.close(); err != nil {
		return err
	}
	z.freeBuffers()

	if debugFlag {
		debug("writing last empty block")
	}
	if err := z.writeUint32(0); err != nil {
		return err
	}
	if z.NoChecksum {
		return nil
	}
	checksum := z.checksum.Sum32()
	if debugFlag {
		debug("stream checksum %x", checksum)
	}
	return z.writeUint32(checksum)
}

// Reset clears the state of the Writer z such that it is equivalent to its
// initial state from NewWriter, but instead writing to w.
// No access to the underlying io.Writer is performed.
func (z *Writer) Reset(w io.Writer) {
	n := cap(z.c)
	_ = z.close()
	z.freeBuffers()
	z.Header.Reset()
	z.dst = w
	z.checksum.Reset()
	z.idx = 0
	z.err = nil
	z.WithConcurrency(n)
}

// writeUint32 writes a uint32 to the underlying writer.
func (z *Writer) writeUint32(x uint32) error {
	buf := z.buf[:4]
	binary.LittleEndian.PutUint32(buf, x)
	_, err := z.dst.Write(buf)
	return err
}

// writerCompressBlock compresses data into a pooled buffer and writes its result
// out to the input channel.
func writerCompressBlock(c chan zResult, header Header, data []byte) {
	zdata := getBuffer(header.BlockMaxSize)
	// The compressed block size cannot exceed the input's.
	var zn int
	if level := header.CompressionLevel; level != 0 {
		zn, _ = CompressBlockHC(data, zdata, level)
	} else {
		var hashTable [winSize]int
		zn, _ = CompressBlock(data, zdata, hashTable[:])
	}
	var res zResult
	if zn > 0 && zn < len(data) {
		res.size = uint32(zn)
		res.data = zdata[:zn]
	} else {
		res.size = uint32(len(data)) | compressedBlockFlag
		res.data = data
	}
	if header.BlockChecksum {
		res.checksum = xxh32.ChecksumZero(res.data)
	}
	c <- res
}
