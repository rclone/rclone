package compress

import (
	"context"
	"errors"
	"io"
	"runtime"
	"sync"

	szstd "github.com/a1ex3/zstd-seekable-format-go/pkg"
	"github.com/klauspost/compress/zstd"
)

const szstdChunkSize int = 1 << 20 // 1 MiB chunk size

// SzstdMetadata holds metadata for szstd compressed files.
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
		metadata: SzstdMetadata{
			BlockSize: szstdChunkSize,
			Size:      0,
		},
	}, nil
}

// Write writes data to the szstd writer in chunks of szstdChunkSize.
// It handles the block size and metadata updates automatically.
func (w *SzstdWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	if w.metadata.BlockData == nil {
		numBlocks := (len(p) + w.metadata.BlockSize - 1) / w.metadata.BlockSize
		w.metadata.BlockData = make([]uint32, 1, numBlocks+1)
		w.metadata.BlockData[0] = 0
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
		w.metadata.Size += int64(size)
		w.mu.Unlock()

		start = end
		return chunk, nil
	}

	// write sizes of compressed blocks in the callback
	err := w.w.WriteMany(context.Background(), writerFunc,
		szstd.WithWriteCallback(func(size uint32) {
			w.mu.Lock()
			lastOffset := w.metadata.BlockData[len(w.metadata.BlockData)-1]
			w.metadata.BlockData = append(w.metadata.BlockData, lastOffset+size)
			w.mu.Unlock()
		}),
	)
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
	return w.metadata
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

	endOff := min(off+int64(len(p)), s.metadata.Size)

	// Find all blocks covered by the range
	type blockInfo struct {
		index         int   // Block index
		offsetInBlock int64 // Offset within the block for starting reading
		bytesToRead   int64 // How many bytes to read from this block
	}

	var blocks []blockInfo
	uncompressedOffset := int64(0)
	currentOff := off

	for i := 0; i < len(s.metadata.BlockData)-1; i++ {
		blockUncompressedEnd := min(uncompressedOffset+int64(s.metadata.BlockSize), s.metadata.Size)

		if currentOff < blockUncompressedEnd && endOff > uncompressedOffset {
			offsetInBlock := max(0, currentOff-uncompressedOffset)
			bytesToRead := min(blockUncompressedEnd-uncompressedOffset-offsetInBlock, endOff-currentOff)

			blocks = append(blocks, blockInfo{
				index:         i,
				offsetInBlock: offsetInBlock,
				bytesToRead:   bytesToRead,
			})

			currentOff += bytesToRead
			if currentOff >= endOff {
				break
			}
		}
		uncompressedOffset = blockUncompressedEnd
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
	sem := make(chan struct{}, runtime.NumCPU())

	for _, block := range blocks {
		wg.Add(1)
		go func(block blockInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			startOffset := int64(s.metadata.BlockData[block.index])
			endOffset := int64(s.metadata.BlockData[block.index+1])
			compressedSize := endOffset - startOffset

			compressed := make([]byte, compressedSize)
			_, err := s.r.ReadAt(compressed, startOffset)
			if err != nil && err != io.EOF {
				resultCh <- decodeResult{index: block.index, err: err}
				return
			}

			decoded, err := s.decoder.DecodeAll(compressed, nil)
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
				// find the corresponding blockInfo
				var blk blockInfo
				for _, b := range blocks {
					if b.index == result.index {
						blk = b
						break
					}
				}

				start := blk.offsetInBlock
				end := start + blk.bytesToRead
				copy(p[totalRead:totalRead+int(blk.bytesToRead)], result.data[start:end])
				totalRead += int(blk.bytesToRead)
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
