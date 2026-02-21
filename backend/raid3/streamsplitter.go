// Package raid3 implements a backend that splits data across three remotes using byte-level striping
package raid3

import (
	"fmt"
	"io"
)

// streamWriteChunkSize is the size of each Write to the downstream pipes.
// Writing in small chunks avoids deadlock when pipe buffers are small (e.g. 16 KB on macOS):
// the splitter must not write more than the consumer can accept without blocking, or the
// splitter blocks and the source (e.g. compressing reader) never gets drained.
const streamWriteChunkSize = 4 * 1024 // 4 KB, well under typical pipe capacity (16–64 KB) to avoid deadlock with slow consumers

// StreamSplitter splits an input stream into even, odd, and parity particle streams
// It processes data in chunks to maintain constant memory usage
type StreamSplitter struct {
	evenWriter   io.Writer
	oddWriter    io.Writer
	parityWriter io.Writer
	chunkSize    int
	// Optional channel to communicate odd-length detection (nil if not needed)
	isOddLengthCh chan bool
	// Track sizes for verification and odd-length detection
	totalEvenWritten int64
	totalOddWritten  int64
}

// NewStreamSplitter creates a new StreamSplitter that splits input into even, odd, and parity streams
// isOddLengthCh can be nil if odd-length detection is not needed (when srcSize is known)
func NewStreamSplitter(evenWriter, oddWriter, parityWriter io.Writer, chunkSize int, isOddLengthCh chan bool) *StreamSplitter {
	return &StreamSplitter{
		evenWriter:       evenWriter,
		oddWriter:        oddWriter,
		parityWriter:     parityWriter,
		chunkSize:        chunkSize,
		isOddLengthCh:    isOddLengthCh,
		totalEvenWritten: 0,
		totalOddWritten:  0,
	}
}

// Split reads from the input reader and splits the data into even, odd, and parity streams.
// It reads one chunk at a time and does not read the next chunk until the current one
// has been written to the downstream pipes (writeInChunks blocks until accepted).
// This backpressure keeps memory bounded and avoids truncation when the source is
// an async reader (e.g. accounting with WithBuffer).
func (s *StreamSplitter) Split(reader io.Reader) error {
	// Buffer for reading chunks
	buffer := make([]byte, s.chunkSize)
	var totalRead int64

	for {
		// Read chunk from input
		n, readErr := reader.Read(buffer)
		if n > 0 {
			// Split by global position so multiple small reads (e.g. 1-byte at end of compressed stream) assign bytes correctly
			evenData, oddData := SplitBytesWithOffset(buffer[:n], int(totalRead))
			totalRead += int64(n)
			parityData := CalculateParity(evenData, oddData)

			// Track sizes
			s.totalEvenWritten += int64(len(evenData))
			s.totalOddWritten += int64(len(oddData))

			// If size was unknown, detect odd-length from chunks
			if s.isOddLengthCh != nil && len(evenData) > len(oddData) {
				// Update channel (non-blocking, will overwrite previous value)
				select {
				case s.isOddLengthCh <- true:
				default:
					// Channel already has a value, drain and send new one
					select {
					case <-s.isOddLengthCh:
						s.isOddLengthCh <- true
					default:
					}
				}
			}

			// Write to pipes in small chunks so we don't fill pipe buffers and deadlock
			// (consumers may be slow to start or have small buffers, e.g. 16 KB on macOS)
			if err := writeInChunks(s.evenWriter, evenData, streamWriteChunkSize); err != nil {
				return fmt.Errorf("failed to write even data: %w", err)
			}
			if err := writeInChunks(s.oddWriter, oddData, streamWriteChunkSize); err != nil {
				return fmt.Errorf("failed to write odd data: %w", err)
			}
			if err := writeInChunks(s.parityWriter, parityData, streamWriteChunkSize); err != nil {
				return fmt.Errorf("failed to write parity data: %w", err)
			}
		}

		if readErr == io.EOF {
			break // End of input
		}
		if readErr != nil {
			return fmt.Errorf("failed to read input: %w", readErr)
		}
	}

	// If size was unknown, final check: evenWritten > oddWritten means odd-length
	if s.isOddLengthCh != nil && s.totalEvenWritten > s.totalOddWritten {
		select {
		case s.isOddLengthCh <- true:
		default:
			select {
			case <-s.isOddLengthCh:
				s.isOddLengthCh <- true
			default:
			}
		}
	}

	return nil
}

// writeInChunks writes data to w in chunks of at most chunkSize bytes to avoid
// blocking when w is a pipe with limited buffer capacity.
func writeInChunks(w io.Writer, data []byte, chunkSize int) error {
	for len(data) > 0 {
		n := chunkSize
		if n > len(data) {
			n = len(data)
		}
		written, err := w.Write(data[:n])
		if err != nil {
			return err
		}
		if written != n {
			return fmt.Errorf("short write: wrote %d, expected %d", written, n)
		}
		data = data[written:]
	}
	return nil
}

// GetTotalEvenWritten returns the total number of bytes written to the even stream
func (s *StreamSplitter) GetTotalEvenWritten() int64 {
	return s.totalEvenWritten
}

// GetTotalOddWritten returns the total number of bytes written to the odd stream
func (s *StreamSplitter) GetTotalOddWritten() int64 {
	return s.totalOddWritten
}
