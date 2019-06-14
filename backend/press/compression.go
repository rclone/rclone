// Package press provides wrappers for Fs and Object which implement compression.
// This file is the backend implementation for seekable compression.
package press

/*
NOTES:
Structure of the metadata we store is:
gzipExtraify(gzip([4-byte header size][4-byte block size] ... [4-byte block size][4-byte raw size of last block]))
This is appended to any compressed file, and is ignored as trailing garbage in our LZ4 and SNAPPY implementations, and seen as empty archives in our GZIP and XZ_IN_GZ implementations.

There are two possible compression/decompression function pairs to be used:
The two functions that store data internally are:
- Compression.CompressFileAppendingBlockData. Appends block data in extra data fields of empty gzip files at the end.
- DecompressFile. Reads block data from extra fields of these empty gzip files.
The two functions that require externally stored data are:
- Compression.CompressFileReturningBlockData. Returns a []uint32 containing raw (uncompressed and unencoded) block data, which must be externally stored.
- DecompressFileExtData. Takes in the []uint32 that was returned by Compression.CompressFileReturningBlockData
WARNING: These function pairs are incompatible with each other. Don't use CompressFileAppendingBlockData with DecompressFileExtData, or the other way around. It won't work.
*/

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"log"
)

// Compression modes
const (
	Uncompressed = -1
	GzipStore    = 0
	GzipMin      = 1
	GzipDefault  = 2
	GzipMax      = 3
	LZ4          = 4
	Snappy       = 5
	XZMin        = 6
	XZDefault    = 7
)

// Errors
var (
	ErrMetadataCorrupted = errors.New("metadata may have been corrupted")
)

// DEBUG - flag for debug mode
const DEBUG = false

// Compression is a struct containing configurable variables (what used to be constants)
type Compression struct {
	CompressionMode int    // Compression mode
	BlockSize       uint32 // Size of blocks. Higher block size means better compression but more download bandwidth needed for small downloads
	// ~1MB is recommended for xz, while ~128KB is recommended for gzip and lz4
	HeuristicBytes      int64   // Bytes to perform gzip heuristic on to determine whether a file should be compressed
	NumThreads          int     // Number of threads to use for compression
	MaxCompressionRatio float64 // Maximum compression ratio for a file to be considered compressible
	BinPath             string  // Path to compression binary. This is used for all non-gzip compression.
}

// NewCompressionPreset creates a Compression object with a preset mode/bs
func NewCompressionPreset(preset string) (*Compression, error) {
	switch preset {
	case "gzip-store":
		return NewCompression(GzipStore, 131070) // GZIP-store (dummy) compression
	case "lz4":
		return NewCompression(LZ4, 262140) // LZ4 compression (very fast)
	case "snappy":
		return NewCompression(Snappy, 262140) // Snappy compression (like LZ4, but slower and worse)
	case "gzip-min":
		return NewCompression(GzipMin, 131070) // GZIP-min compression (fast)
	case "gzip-default":
		return NewCompression(GzipDefault, 131070) // GZIP-default compression (medium)
	case "xz-min":
		return NewCompression(XZMin, 524288) // XZ-min compression (slow)
	case "xz-default":
		return NewCompression(XZDefault, 1048576) // XZ-default compression (very slow)
	}
	return nil, errors.New("Compression mode doesn't exist")
}

// NewCompressionPresetNumber creates a Compression object with a preset mode/bs
func NewCompressionPresetNumber(preset int) (*Compression, error) {
	switch preset {
	case GzipStore:
		return NewCompression(GzipStore, 131070) // GZIP-store (dummy) compression
	case LZ4:
		return NewCompression(LZ4, 262140) // LZ4 compression (very fast)
	case Snappy:
		return NewCompression(Snappy, 262140) // Snappy compression (like LZ4, but slower and worse)
	case GzipMin:
		return NewCompression(GzipMin, 131070) // GZIP-min compression (fast)
	case GzipDefault:
		return NewCompression(GzipDefault, 131070) // GZIP-default compression (medium)
	case XZMin:
		return NewCompression(XZMin, 524288) // XZ-min compression (slow)
	case XZDefault:
		return NewCompression(XZDefault, 1048576) // XZ-default compression (very slow)
	}
	return nil, errors.New("Compression mode doesn't exist")
}

// NewCompression creates a Compression object with some default configuration values
func NewCompression(mode int, bs uint32) (*Compression, error) {
	return NewCompressionAdvanced(mode, bs, 1048576, 12, 0.9)
}

// NewCompressionAdvanced creates a Compression object
func NewCompressionAdvanced(mode int, bs uint32, hb int64, threads int, mcr float64) (c *Compression, err error) {
	// Set vars
	c = new(Compression)
	c.CompressionMode = mode
	c.BlockSize = bs
	c.HeuristicBytes = hb
	c.NumThreads = threads
	c.MaxCompressionRatio = mcr
	// Get binary path if needed
	err = getBinPaths(c, mode)
	return c, err
}

/*** UTILITY FUNCTIONS ***/
// Gets an overestimate for the maximum compressed block size
func (c *Compression) maxCompressedBlockSize() uint32 {
	return c.BlockSize + (c.BlockSize >> 2) + 256
}

// GetFileExtension gets a file extension for current compression mode
func (c *Compression) GetFileExtension() string {
	switch c.CompressionMode {
	case GzipStore, GzipMin, GzipDefault, GzipMax:
		return ".gz"
	case XZMin, XZDefault:
		return ".xzgz"
	case LZ4:
		return ".lz4"
	case Snappy:
		return ".snap"
	}
	panic("Compression mode doesn't exist")
}

// GetFileCompressionInfo gets a file extension along with compressibility of file
// It is currently not being used but may be usable in the future.
func (c *Compression) GetFileCompressionInfo(reader io.Reader) (compressable bool, extension string, err error) {
	// Use our compression algorithm to do a heuristic on the first few bytes
	var emulatedBlock, emulatedBlockCompressed bytes.Buffer
	_, err = io.CopyN(&emulatedBlock, reader, c.HeuristicBytes)
	if err != nil && err != io.EOF {
		return false, "", err
	}
	compressedSize, uncompressedSize, err := c.compressBlock(emulatedBlock.Bytes(), &emulatedBlockCompressed)
	if err != nil {
		return false, "", err
	}
	compressionRatio := float64(compressedSize) / float64(uncompressedSize)

	// If the data is not compressible, return so
	if compressionRatio > c.MaxCompressionRatio {
		return false, ".bin", nil
	}

	// If the file is compressible, select file extension based on compression mode
	return true, c.GetFileExtension(), nil
}

// Gets the file header we add to files of the currently used algorithm. Currently only used for lz4.
func (c *Compression) getHeader() []byte {
	switch c.CompressionMode {
	case GzipStore, GzipMin, GzipDefault, GzipMax:
		return GzipHeader
	case XZMin, XZDefault:
		return ExecHeader
	case LZ4:
		return LZ4Header
	case Snappy:
		return SnappyHeader
	}
	panic("Compression mode doesn't exist")
}

// Gets the file footer we add to files of the currently used algorithm. Currently only used for lz4.
func (c *Compression) getFooter() []byte {
	switch c.CompressionMode {
	case GzipStore, GzipMin, GzipDefault, GzipMax:
		return []byte{}
	case XZMin, XZDefault:
		return []byte{}
	case LZ4:
		return LZ4Footer
	case Snappy:
		return []byte{}
	}
	panic("Compression mode doesn't exist")
}

/*** BLOCK COMPRESSION FUNCTIONS ***/
// Wrapper function to compress a block
func (c *Compression) compressBlock(in []byte, out io.Writer) (compressedSize uint32, uncompressedSize int64, err error) {
	switch c.CompressionMode { // Select compression function (and arguments) based on compression mode
	case GzipStore:
		return c.compressBlockGz(in, out, 0)
	case GzipMin:
		return c.compressBlockGz(in, out, 1)
	case GzipDefault:
		return c.compressBlockGz(in, out, 6)
	case GzipMax:
		return c.compressBlockGz(in, out, 9)
	case XZDefault:
		return c.compressBlockExec(in, out, c.BinPath, []string{"-c"})
	case XZMin:
		return c.compressBlockExec(in, out, c.BinPath, []string{"-c1"})
	case LZ4:
		return c.compressBlockLz4(in, out)
	case Snappy:
		return c.compressBlockSnappy(in, out)
	}
	panic("Compression mode doesn't exist")
}

/*** MAIN COMPRESSION INTERFACE ***/
// compressionResult represents the result of compression for a single block (gotten by a single thread)
type compressionResult struct {
	buffer *bytes.Buffer
	n      int64
	err    error
}

// CompressFileReturningBlockData compresses a file returning the block data for that file.
func (c *Compression) CompressFileReturningBlockData(in io.Reader, out io.Writer) (blockData []uint32, err error) {
	// Initialize buffered writer
	bufw := bufio.NewWriterSize(out, int(c.maxCompressedBlockSize()*uint32(c.NumThreads)))

	// Get blockData, copy over header, add length of header to blockData
	blockData = make([]uint32, 0)
	header := c.getHeader()
	_, err = bufw.Write(header)
	if err != nil {
		return nil, err
	}
	blockData = append(blockData, uint32(len(header)))

	// Compress blocks
	for {
		// Loop through threads, spawning a go procedure for each thread. If we get eof on one thread, set eofAt to that thread and break
		compressionResults := make([]chan compressionResult, c.NumThreads)
		eofAt := -1
		for i := 0; i < c.NumThreads; i++ {
			// Create thread channel and allocate buffer to pass to thread
			compressionResults[i] = make(chan compressionResult)
			var inputBuffer bytes.Buffer
			_, err = io.CopyN(&inputBuffer, in, int64(c.BlockSize))
			if err == io.EOF {
				eofAt = i
			} else if err != nil {
				return nil, err
			}
			// Run thread
			go func(i int, in []byte) {
				// Initialize thread writer and result struct
				var res compressionResult
				var buffer bytes.Buffer

				// Compress block
				_, n, err := c.compressBlock(in, &buffer)
				if err != nil && err != io.EOF { // This errored out.
					res.buffer = nil
					res.n = 0
					res.err = err
					compressionResults[i] <- res
					return
				}
				// Pass our data back to the main thread as a compression result
				res.buffer = &buffer
				res.n = n
				res.err = err
				compressionResults[i] <- res
				return
			}(i, inputBuffer.Bytes())
			// If we have reached eof, we don't need more threads
			if eofAt != -1 {
				break
			}
		}

		// Process writers in order
		for i := 0; i < c.NumThreads; i++ {
			if compressionResults[i] != nil {
				// Get current compression result, get buffer, and copy buffer over to output
				res := <-compressionResults[i]
				close(compressionResults[i])
				if res.buffer == nil {
					return nil, res.err
				}
				blockSize := uint32(res.buffer.Len())
				_, err = io.Copy(bufw, res.buffer)
				if err != nil {
					return nil, err
				}
				if DEBUG {
					log.Printf("%d %d\n", res.n, blockSize)
				}

				// Append block size to block data
				blockData = append(blockData, blockSize)

				// If this is the last block, add the raw size of the last block to the end of blockData and break
				if eofAt == i {
					if DEBUG {
						log.Printf("%d %d %d\n", res.n, byte(res.n%256), byte(res.n/256))
					}
					blockData = append(blockData, uint32(res.n))
					break
				}
			}
		}

		// Get number of bytes written in this block (they should all be in the bufio buffer), then close gzip and flush buffer
		err = bufw.Flush()
		if err != nil {
			return nil, err
		}

		// If eof happened, break
		if eofAt != -1 {
			if DEBUG {
				log.Printf("%d", eofAt)
				log.Printf("%v", blockData)
			}
			break
		}
	}

	// Write footer and flush
	_, err = bufw.Write(c.getFooter())
	if err != nil {
		return nil, err
	}
	err = bufw.Flush()

	// Return
	return blockData, err
}

/*** BLOCK DECOMPRESSION FUNCTIONS ***/
// Wrapper function to decompress a block
func (d *Decompressor) decompressBlock(in io.Reader, out io.Writer) (n int, err error) {
	switch d.c.CompressionMode { // Select decompression function based off compression mode
	case GzipStore, GzipMin, GzipDefault, GzipMax:
		return decompressBlockRangeGz(in, out)
	case XZMin:
		return decompressBlockRangeExec(in, out, d.c.BinPath, []string{"-dc1"})
	case XZDefault:
		return decompressBlockRangeExec(in, out, d.c.BinPath, []string{"-dc"})
	case LZ4:
		return decompressBlockLz4(in, out, int64(d.c.BlockSize))
	case Snappy:
		return decompressBlockSnappy(in, out)
	}
	panic("Compression mode doesn't exist") // If none of the above returned
}

// Wrapper function for decompressBlock that implements multithreading
// decompressionResult represents the result of decompressing a block
type decompressionResult struct {
	err    error
	buffer *bytes.Buffer
}

func (d *Decompressor) decompressBlockRangeMultithreaded(in io.Reader, out io.Writer, startingBlock uint32) (n int, err error) {
	// First, use bufio.Reader to reduce the number of reads and bufio.Writer to reduce the number of writes
	bufin := bufio.NewReader(in)
	bufout := bufio.NewWriter(out)

	// Decompress each block individually.
	currBatch := startingBlock // Block # of start of current batch of blocks
	totalBytesCopied := 0
	for {
		// Loop through threads
		eofAt := -1
		decompressionResults := make([]chan decompressionResult, d.c.NumThreads)

		for i := 0; i < d.c.NumThreads; i++ {
			// Get currBlock
			currBlock := currBatch + uint32(i)

			// Create channel
			decompressionResults[i] = make(chan decompressionResult)

			// Check if we've reached EOF
			if currBlock >= d.numBlocks {
				eofAt = i
				break
			}

			// Get block to decompress
			var compressedBlock bytes.Buffer
			var err error
			n, err := io.CopyN(&compressedBlock, bufin, d.blockStarts[currBlock+1]-d.blockStarts[currBlock])
			if err != nil || n == 0 { // End of stream
				eofAt = i
				break
			}

			// Spawn thread to decompress block
			if DEBUG {
				log.Printf("Spawning %d", i)
			}
			go func(i int, currBlock uint32, in io.Reader) {
				var block bytes.Buffer
				var res decompressionResult

				// Decompress block
				_, res.err = d.decompressBlock(in, &block)
				res.buffer = &block
				decompressionResults[i] <- res
			}(i, currBlock, &compressedBlock)
		}
		if DEBUG {
			log.Printf("Eof at %d", eofAt)
		}

		// Process results
		for i := 0; i < d.c.NumThreads; i++ {
			// If we got EOF, return
			if eofAt == i {
				return totalBytesCopied, bufout.Flush() // Flushing bufout is needed to prevent us from getting all nulls
			}

			// Get result and close
			res := <-decompressionResults[i]
			close(decompressionResults[i])
			if res.err != nil {
				return totalBytesCopied, res.err
			}

			// Copy to output and add to total bytes copied
			n, err := io.Copy(bufout, res.buffer)
			totalBytesCopied += int(n)
			if err != nil {
				return totalBytesCopied, err
			}
		}

		// Add NumThreads to currBatch
		currBatch += uint32(d.c.NumThreads)
	}
}

/*** MAIN DECOMPRESSION INTERFACE ***/

// Decompressor is the ReadSeeker implementation for decompression
type Decompressor struct {
	cursorPos        *int64        // The current location we have seeked to
	blockStarts      []int64       // The start of each block. These will be recovered from the block sizes
	numBlocks        uint32        // Number of blocks
	decompressedSize int64         // Decompressed size of the file.
	in               io.ReadSeeker // Input
	c                *Compression  // Compression options
}

// Parses block data. Returns the number of blocks, the block start locations for each block, and the decompressed size of the entire file.
func parseBlockData(blockData []uint32, BlockSize uint32) (numBlocks uint32, blockStarts []int64, decompressedSize int64) {
	// Parse the block data
	blockDataLen := len(blockData)
	numBlocks = uint32(blockDataLen - 1)
	if DEBUG {
		log.Printf("%v\n", blockData)
		log.Printf("metadata len, numblocks = %d, %d", blockDataLen, numBlocks)
	}
	blockStarts = make([]int64, numBlocks+1) // Starts with start of first block (and end of header), ends with end of last block
	currentBlockPosition := int64(0)
	for i := uint32(0); i < numBlocks; i++ { // Loop through block data, getting starts of blocks.
		currentBlockSize := blockData[i]
		currentBlockPosition += int64(currentBlockSize)
		blockStarts[i] = currentBlockPosition
	}
	blockStarts[numBlocks] = currentBlockPosition // End of last block

	//log.Printf("Block Starts: %v\n", d.blockStarts)

	numBlocks-- // Subtract 1 from number of blocks because our header technically isn't a block

	// Get uncompressed size of last block and derive uncompressed size of file
	lastBlockRawSize := blockData[blockDataLen-1]
	decompressedSize = int64(numBlocks-1)*int64(BlockSize) + int64(lastBlockRawSize)
	if DEBUG {
		log.Printf("Decompressed size = %d", decompressedSize)
	}

	return numBlocks, blockStarts, decompressedSize
}

// Initializes decompressor with the block data specified.
func (d *Decompressor) initWithBlockData(c *Compression, in io.ReadSeeker, size int64, blockData []uint32) (err error) {
	// Copy over compression object
	d.c = c

	// Initialize cursor position
	d.cursorPos = new(int64)

	// Parse the block data
	d.numBlocks, d.blockStarts, d.decompressedSize = parseBlockData(blockData, d.c.BlockSize)

	// Initialize cursor position value and copy over reader
	*d.cursorPos = 0
	_, err = in.Seek(0, io.SeekStart)
	d.in = in

	return err
}

// Read reads data using a decompressor
func (d Decompressor) Read(p []byte) (int, error) {
	if DEBUG {
		log.Printf("Cursor pos before: %d\n", *d.cursorPos)
	}
	// Check if we're at the end of the file or before the beginning of the file
	if *d.cursorPos >= d.decompressedSize || *d.cursorPos < 0 {
		if DEBUG {
			log.Println("Out of bounds EOF")
		}
		return 0, io.EOF
	}

	// Get block range to read
	blockNumber := *d.cursorPos / int64(d.c.BlockSize)
	blockStart := d.blockStarts[blockNumber]                                 // Start position of blocks to read
	dataOffset := *d.cursorPos % int64(d.c.BlockSize)                        // Offset of data to read in blocks to read
	bytesToRead := len(p)                                                    // Number of bytes to read
	blocksToRead := (int64(bytesToRead)+dataOffset)/int64(d.c.BlockSize) + 1 // Number of blocks to read
	returnEOF := false
	if blockNumber+blocksToRead > int64(d.numBlocks) { // Overflowed the last block
		blocksToRead = int64(d.numBlocks) - blockNumber
		returnEOF = true
	}
	var blockEnd int64                                 // End position of blocks to read
	blockEnd = d.blockStarts[blockNumber+blocksToRead] // Start of the block after the last block we want to get is the end of the last block we want to get
	blockLen := blockEnd - blockStart

	// Read compressed block range into buffer
	var compressedBlocks bytes.Buffer
	_, err := d.in.Seek(blockStart, io.SeekStart)
	if err != nil {
		return 0, err
	}
	n1, err := io.CopyN(&compressedBlocks, d.in, blockLen)
	if DEBUG {
		log.Printf("block # = %d @ %d <- %d, len %d, copied %d bytes", blockNumber, blockStart, *d.cursorPos, blockLen, n1)
	}
	if err != nil {
		if DEBUG {
			log.Println("Copy Error")
		}
		return 0, err
	}

	// Decompress block range
	var b bytes.Buffer
	n, err := d.decompressBlockRangeMultithreaded(&compressedBlocks, &b, uint32(blockNumber))
	if err != nil {
		log.Println("Decompression error")
		return n, err
	}

	// Calculate bytes read
	readOverflow := *d.cursorPos + int64(bytesToRead) - d.decompressedSize
	if readOverflow < 0 {
		readOverflow = 0
	}
	bytesRead := int64(bytesToRead) - readOverflow
	if DEBUG {
		log.Printf("Read offset = %d, overflow = %d", dataOffset, readOverflow)
		log.Printf("Decompressed %d bytes; read %d out of %d bytes\n", n, bytesRead, bytesToRead)
		//	log.Printf("%v", b.Bytes())
	}

	// If we read 0 bytes, we reached the end of the file
	if bytesRead == 0 {
		log.Println("EOF")
		return 0, io.EOF
	}

	// Copy from buffer+offset to p
	_, err = io.CopyN(ioutil.Discard, &b, dataOffset)
	if err != nil {
		return 0, err
	}
	n, err = b.Read(p) // Note: everything after bytesToRead bytes will be discarded; we are returning bytesToRead instead of n
	if err != nil {
		return n, err
	}

	// Increment cursor position and return
	*d.cursorPos += bytesRead
	if returnEOF {
		if DEBUG {
			log.Println("EOF")
		}
		return int(bytesRead), io.EOF
	}
	return int(bytesRead), nil
}

// Seek seeks to a location in compressed stream
func (d Decompressor) Seek(offset int64, whence int) (int64, error) {
	// Seek to offset in cursorPos
	if whence == io.SeekStart {
		*d.cursorPos = offset
	} else if whence == io.SeekCurrent {
		*d.cursorPos += offset
	} else if whence == io.SeekEnd {
		*d.cursorPos = d.decompressedSize + offset
	}

	// Return
	return offset, nil
}

// DecompressFileExtData decompresses a file using external block data. Argument "size" is very useful here.
func (c *Compression) DecompressFileExtData(in io.ReadSeeker, size int64, blockData []uint32) (FileHandle io.ReadSeeker, decompressedSize int64, err error) {
	var decompressor Decompressor
	err = decompressor.initWithBlockData(c, in, size, blockData)
	return decompressor, decompressor.decompressedSize, err
}
