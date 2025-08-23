package compress

import (
	"context"
	"errors"
	"io"
	"runtime"
	"sync"

	szstd "github.com/SaveTheRbtz/zstd-seekable-format-go/pkg"
	"github.com/klauspost/compress/zstd"
)

const szstdChunkSize int = 1 << 20 // 1 MiB chunk size

type SzstdMetadata struct {
	BlockSize int      // BlockSize is the size of the blocks in the zstd file
	Size      int64    // Size is the uncompressed size of the file
	BlockData []uint32 // BlockData is the block data for the zstd file, used for seeking
}

// SzstdWriter is a writer that compresses data in szstd format.
type SzstdWriter struct {
	enc      *zstd.Encoder
	w        szstd.ConcurrentWriter
	metadata SzstdMetadata
	mu       sync.Mutex
}

// NewWriterSzstd creates a new szstd writer with the specified options.
// It initializes the szstd writer with a zstd encoder and returns a pointer to the SzstdWriter.
// The writer can be used to write data in chunks, and it will automatically handle block sizes and metadata.
func NewWriterSzstd(w io.Writer, opts ...zstd.EOption) (*SzstdWriter, error) {
	encoder, err := zstd.NewWriter(nil, opts...)
	if err != nil {
		return nil, err
	}

	sw, err := szstd.NewWriter(w, encoder)
	if err != nil {
		if err := encoder.Close(); err != nil {
			return nil, err
		}
		return nil, err
	}

	return &SzstdWriter{
		enc: encoder,
		w:   sw,
	}, nil
}

// Write writes data to the szstd writer in chunks of szstdChunkSize.
// It handles the block size and metadata updates automatically.
func (w *SzstdWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	if w.metadata.BlockSize == 0 {
		w.metadata.BlockSize = szstdChunkSize
	}
	if w.metadata.BlockData == nil {
		numChunks := (len(p) + w.metadata.BlockSize - 1) / w.metadata.BlockSize
		w.metadata.BlockData = make([]uint32, 0, numChunks)
	}

	start := 0
	total := len(p)

	var writerFunc szstd.FrameSource = func() ([]byte, error) {
		if start >= total {
			return nil, nil
		}

		end := min(start+w.metadata.BlockSize, total)

		chunk := p[start:end]
		size := end - start

		w.mu.Lock()
		w.metadata.BlockData = append(w.metadata.BlockData, uint32(size))
		w.metadata.Size += int64(size)
		w.mu.Unlock()

		start = end
		return chunk, nil
	}

	err := w.w.WriteMany(context.Background(), writerFunc)
	if err != nil {
		return 0, err
	}

	return total, nil
}

// Close closes the SzstdWriter and its underlying encoder.
func (w *SzstdWriter) Close() error {
	if err := w.w.Close(); err != nil {
		return err
	}
	if err := w.enc.Close(); err != nil {
		return err
	}

	return nil
}

// GetMetadata returns the metadata of the szstd writer.
func (w *SzstdWriter) GetMetadata() SzstdMetadata {
	return SzstdMetadata(w.metadata)
}

// SzstdReaderAt is a reader that allows random access in szstd compressed data.
type SzstdReaderAt struct {
	r        szstd.Reader
	decoder  *zstd.Decoder
	metadata *SzstdMetadata
	pos      int64
	mu       sync.Mutex
}

// NewReaderAtSzstd creates a new SzstdReaderAt at the specified io.ReadSeeker.
func NewReaderAtSzstd(rs io.ReadSeeker, meta *SzstdMetadata, offset int64, opts ...zstd.DOption) (*SzstdReaderAt, error) {
	decoder, err := zstd.NewReader(nil, opts...)
	if err != nil {
		return nil, err
	}

	r, err := szstd.NewReader(rs, decoder)
	if err != nil {
		decoder.Close()
		return nil, err
	}

	sr := &SzstdReaderAt{
		r:        r,
		decoder:  decoder,
		metadata: meta,
		pos:      0,
	}

	// Set initial position to the provided offset
	if _, err := sr.Seek(offset, io.SeekStart); err != nil {
		if err := sr.Close(); err != nil {
			return nil, err
		}
		return nil, err
	}

	return sr, nil
}

// Seek sets the offset for the next Read.
func (s *SzstdReaderAt) Seek(offset int64, whence int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pos, err := s.r.Seek(offset, whence)
	if err == nil {
		s.pos = pos
	}
	return pos, err
}

func (s *SzstdReaderAt) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, err := s.r.Read(p)
	if err == nil {
		s.pos += int64(n)
	}
	return n, err
}

// ReadAt reads data at the specified offset.
func (s *SzstdReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errors.New("invalid offset")
	}
	if off >= s.metadata.Size {
		return 0, io.EOF
	}

	// Calculate the requested range
	endOff := min(off+int64(len(p)), s.metadata.Size)

	// Find all blocks covered by the range
	type blockInfo struct {
		index         int   // Block index
		startOffset   int64 // Start of block in uncompressed stream
		offsetInBlock int64 // Offset within the block for starting reading
		bytesToRead   int64 // How many bytes to read from this block
	}

	var blocks []blockInfo
	var blockStartOffset int64
	currentOff := off

	for i, size := range s.metadata.BlockData {
		blockEndOffset := blockStartOffset + int64(size)
		if currentOff >= blockEndOffset {
			blockStartOffset = blockEndOffset
			continue
		}

		// If the current block intersects with the range
		if currentOff < blockEndOffset && endOff > blockStartOffset {
			offsetInBlock := max(0, currentOff-blockStartOffset)
			bytesToRead := min(blockEndOffset-blockStartOffset-offsetInBlock, endOff-currentOff)

			blocks = append(blocks, blockInfo{
				index:         i,
				startOffset:   blockStartOffset,
				offsetInBlock: offsetInBlock,
				bytesToRead:   bytesToRead,
			})

			currentOff += bytesToRead
			if currentOff >= endOff {
				break
			}
		}

		blockStartOffset = blockEndOffset
	}

	if len(blocks) == 0 {
		return 0, io.EOF
	}

	// Parallel block decoding
	type decodeResult struct {
		index int
		data  []byte
		err   error
	}

	resultCh := make(chan decodeResult, len(blocks))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU()) // Limit concurrency by the number of CPU cores

	for _, block := range blocks {
		wg.Add(1)
		go func(block blockInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			_, err := s.r.Seek(block.startOffset, io.SeekStart)
			if err != nil {
				resultCh <- decodeResult{index: block.index, err: err}
				return
			}

			compressed := make([]byte, s.metadata.BlockData[block.index])
			n, err := s.r.Read(compressed)
			if err != nil && err != io.EOF {
				resultCh <- decodeResult{index: block.index, err: err}
				return
			}

			decoded, err := s.decoder.DecodeAll(compressed[:n], nil)
			if err != nil {
				resultCh <- decodeResult{index: block.index, err: err}
				return
			}

			resultCh <- decodeResult{index: block.index, data: decoded, err: nil}
		}(block)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect results in block index order
	totalRead := 0
	results := make(map[int]decodeResult)
	expected := len(blocks)
	minIndex := blocks[0].index

	for res := range resultCh {
		results[res.index] = res
		for {
			if result, ok := results[minIndex]; ok {
				if result.err != nil {
					return 0, result.err
				}
				block := blocks[result.index-blocks[0].index]
				start := block.offsetInBlock
				end := start + block.bytesToRead
				copy(p[totalRead:totalRead+int(block.bytesToRead)], result.data[start:end])
				totalRead += int(block.bytesToRead)
				minIndex++
				if minIndex-blocks[0].index >= len(blocks) {
					break
				}
			} else {
				break
			}
		}
		if len(results) == expected && minIndex-blocks[0].index >= len(blocks) {
			break
		}
	}

	return totalRead, nil
}

// Close closes the SzstdReaderAt and underlying decoder.
func (s *SzstdReaderAt) Close() error {
	if err := s.r.Close(); err != nil {
		return err
	}
	s.decoder.Close()
	return nil
}
