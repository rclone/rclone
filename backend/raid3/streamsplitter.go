// Package raid3 implements a backend that splits data across three remotes using byte-level striping
package raid3

import (
	"fmt"
	"io"
)

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

// Split reads from the input reader and splits the data into even, odd, and parity streams
func (s *StreamSplitter) Split(reader io.Reader) error {
	// Buffer for reading chunks
	buffer := make([]byte, s.chunkSize)

	for {
		// Read chunk from input
		n, readErr := reader.Read(buffer)
		if n > 0 {
			// Split chunk into even and odd bytes
			evenData, oddData := SplitBytes(buffer[:n])
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

			// Write to pipes (these may block if readers are slow, which is fine)
			if _, err := s.evenWriter.Write(evenData); err != nil {
				return fmt.Errorf("failed to write even data: %w", err)
			}
			if _, err := s.oddWriter.Write(oddData); err != nil {
				return fmt.Errorf("failed to write odd data: %w", err)
			}
			if _, err := s.parityWriter.Write(parityData); err != nil {
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

// GetTotalEvenWritten returns the total number of bytes written to the even stream
func (s *StreamSplitter) GetTotalEvenWritten() int64 {
	return s.totalEvenWritten
}

// GetTotalOddWritten returns the total number of bytes written to the odd stream
func (s *StreamSplitter) GetTotalOddWritten() int64 {
	return s.totalOddWritten
}
