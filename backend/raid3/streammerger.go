// Package raid3 implements a backend that splits data across three remotes using byte-level striping
package raid3

import (
	"fmt"
	"io"
	"sync"

	"github.com/rclone/rclone/fs"
)

// StreamMerger merges even and odd particle streams into original data stream.
// It processes data in chunks to maintain constant memory usage.
//
// Read must not be called concurrently from multiple goroutines;
// the caller is responsible for sequential access.
type StreamMerger struct {
	evenReader   io.ReadCloser
	oddReader    io.ReadCloser
	chunkSize    int
	evenBuffer   []byte
	oddBuffer    []byte
	outputBuffer []byte
	evenPos      int
	oddPos       int
	evenEOF      bool
	oddEOF       bool
	// Buffers for excess data when reads don't match
	evenPending []byte
	oddPending  []byte
	mu          sync.Mutex
}

// NewStreamMerger creates a new StreamMerger that merges even and odd particle streams
func NewStreamMerger(evenReader, oddReader io.ReadCloser, chunkSize int) *StreamMerger {
	return &StreamMerger{
		evenReader:   evenReader,
		oddReader:    oddReader,
		chunkSize:    chunkSize,
		evenBuffer:   make([]byte, chunkSize),
		oddBuffer:    make([]byte, chunkSize),
		outputBuffer: make([]byte, 0, chunkSize*2), // Output buffer (empty initially, capacity 2x chunk size)
		evenPos:      0,
		oddPos:       0,
		evenEOF:      false,
		oddEOF:       false,
		evenPending:  make([]byte, 0, chunkSize),
		oddPending:   make([]byte, 0, chunkSize),
	}
}

// Read reads merged data from the even and odd streams
func (m *StreamMerger) Read(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// If we have buffered output, return it first
	if len(m.outputBuffer) > 0 && m.evenPos < len(m.outputBuffer) {
		n = copy(p, m.outputBuffer[m.evenPos:])
		m.evenPos += n
		if m.evenPos >= len(m.outputBuffer) {
			// Clear buffer completely - create new empty slice
			m.outputBuffer = nil
			m.evenPos = 0
		}
		return n, nil
	}

	// If both streams are EOF, we're done
	if m.evenEOF && m.oddEOF {
		return 0, io.EOF
	}

	// Read chunks from both streams concurrently
	// Unlock mutex during I/O operations (they may block)
	type readResult struct {
		n      int
		err    error
		hitEOF bool // Track if io.EOF was encountered
	}
	evenCh := make(chan readResult, 1)
	oddCh := make(chan readResult, 1)

	// Read from even stream concurrently
	if !m.evenEOF {
		go func() {
			n, err := m.evenReader.Read(m.evenBuffer)
			hitEOF := (err == io.EOF)
			// Convert io.EOF to nil error (standard Go pattern: EOF is not an error)
			if err == io.EOF {
				err = nil
			}
			evenCh <- readResult{n: n, err: err, hitEOF: hitEOF}
		}()
	} else {
		evenCh <- readResult{n: 0, err: nil, hitEOF: true} // Stream is EOF
	}

	// Read from odd stream concurrently
	if !m.oddEOF {
		go func() {
			n, err := m.oddReader.Read(m.oddBuffer)
			hitEOF := (err == io.EOF)
			// Convert io.EOF to nil error (standard Go pattern: EOF is not an error)
			if err == io.EOF {
				err = nil
			}
			oddCh <- readResult{n: n, err: err, hitEOF: hitEOF}
		}()
	} else {
		oddCh <- readResult{n: 0, err: nil, hitEOF: true} // Stream is EOF
	}

	// Wait for both reads to complete
	m.mu.Unlock()
	evenRes := <-evenCh
	oddRes := <-oddCh
	m.mu.Lock()

	// Process results
	var evenN, oddN int
	var evenErr, oddErr error

	evenN = evenRes.n
	evenErr = evenRes.err
	if evenRes.hitEOF {
		m.evenEOF = true
	}
	if evenErr != nil {
		return 0, fmt.Errorf("failed to read even particle: %w", evenErr)
	}

	oddN = oddRes.n
	oddErr = oddRes.err
	if oddRes.hitEOF {
		m.oddEOF = true
	}
	if oddErr != nil {
		return 0, fmt.Errorf("failed to read odd particle: %w", oddErr)
	}

	// Combine new reads with any pending data
	evenData := m.evenBuffer[:evenN]
	oddData := m.oddBuffer[:oddN]

	// Prepend pending data if any
	if len(m.evenPending) > 0 {
		combined := make([]byte, len(m.evenPending)+len(evenData))
		copy(combined, m.evenPending)
		copy(combined[len(m.evenPending):], evenData)
		evenData = combined
		m.evenPending = m.evenPending[:0] // Clear pending
	}
	if len(m.oddPending) > 0 {
		combined := make([]byte, len(m.oddPending)+len(oddData))
		copy(combined, m.oddPending)
		copy(combined[len(m.oddPending):], oddData)
		oddData = combined
		m.oddPending = m.oddPending[:0] // Clear pending
	}

	// If both are EOF and we have no data (including after combining pending), we're done
	if m.evenEOF && m.oddEOF && len(evenData) == 0 && len(oddData) == 0 {
		return 0, io.EOF
	}

	// Handle case where both are empty (shouldn't happen after EOF check, but be safe)
	if len(evenData) == 0 && len(oddData) == 0 {
		return 0, io.EOF
	}

	// Determine how much we can merge
	// Strategy:
	// - If both are at EOF and even is 1 byte larger, merge all (handles odd-length files)
	// - Otherwise, merge the minimum and buffer the excess
	if m.evenEOF && m.oddEOF {
		// At EOF: allow even to be 1 byte larger (for odd-length files)
		// Merge all data - MergeBytes can handle even being 1 byte larger
		if len(evenData) != len(oddData) && len(evenData) != len(oddData)+1 {
			fs.Logf(nil, "[StreamMerger] Read: INVALID PARTICLE SIZES at EOF - even=%d, odd=%d, evenPending=%d, oddPending=%d", len(evenData), len(oddData), len(m.evenPending), len(m.oddPending))
			return 0, fmt.Errorf("invalid particle sizes: even=%d, odd=%d (expected even=odd or even=odd+1)", len(evenData), len(oddData))
		}
		// Don't buffer - merge all data
	} else {
		// During streaming: merge the minimum, buffer excess
		// BUT: if one stream is at EOF and the other has data, we need to handle it
		// For odd-length files, even can be 1 byte larger than odd, and odd might be EOF
		if m.oddEOF && !m.evenEOF && len(oddData) == 0 && len(evenData) > 0 {
			// Odd stream is done and empty, even still has data
			// This is valid for odd-length files (even is 1 byte larger)
			// Merge all even data with empty odd (MergeBytes handles this)
			// Don't buffer - merge all even data
		} else if m.evenEOF && !m.oddEOF && len(evenData) == 0 && len(oddData) > 0 {
			// Even stream is done and empty, odd still has data
			// This shouldn't happen (even should always be >= odd), but handle it
			// Merge all odd data with empty even
		} else {
			// Both streams still active or both have data - merge the minimum, buffer excess
			mergeSize := len(evenData)
			if len(oddData) < mergeSize {
				mergeSize = len(oddData)
			}

			// Special case: if mergeSize is 0 but we have data in one stream and the other is EOF,
			// we need to merge what we have (for odd-length files)
			if mergeSize == 0 && len(evenData) > 0 && m.oddEOF {
				// Odd is EOF and empty, even has data - merge all even
				mergeSize = len(evenData)
			} else if mergeSize == 0 && len(oddData) > 0 && m.evenEOF {
				// Even is EOF and empty, odd has data - merge all odd
				mergeSize = len(oddData)
			}

			// Buffer excess data
			if len(evenData) > mergeSize {
				m.evenPending = append(m.evenPending[:0], evenData[mergeSize:]...)
				evenData = evenData[:mergeSize]
			}
			if len(oddData) > mergeSize {
				m.oddPending = append(m.oddPending[:0], oddData[mergeSize:]...)
				oddData = oddData[:mergeSize]
			}

			// After buffering, sizes should match during streaming (unless one is EOF and empty)
			// For odd-length files, even can be 1 byte larger when odd is EOF
			if len(evenData) != len(oddData) && !m.evenEOF && !m.oddEOF {
				fs.Logf(nil, "[StreamMerger] Read: UNEXPECTED SIZE MISMATCH during streaming - even=%d, odd=%d, evenEOF=%v, oddEOF=%v, evenPending=%d, oddPending=%d", len(evenData), len(oddData), m.evenEOF, m.oddEOF, len(m.evenPending), len(m.oddPending))
				return 0, fmt.Errorf("unexpected size mismatch during streaming: even=%d, odd=%d", len(evenData), len(oddData))
			}
		}
	}

	// Merge the chunks
	merged, err := MergeBytes(evenData, oddData)
	if err != nil {
		return 0, fmt.Errorf("failed to merge particles: %w", err)
	}

	// Store merged data in output buffer
	// Reset output buffer completely - create a new slice with exact size
	m.outputBuffer = make([]byte, len(merged))
	copy(m.outputBuffer, merged)
	m.evenPos = 0

	// Return data to caller
	if len(m.outputBuffer) == 0 {
		return 0, io.EOF
	}
	n = copy(p, m.outputBuffer[m.evenPos:])
	m.evenPos += n
	if m.evenPos >= len(m.outputBuffer) {
		// Clear buffer completely - create new empty slice
		m.outputBuffer = nil
		m.evenPos = 0
	}

	return n, nil
}

// Close closes all underlying readers
func (m *StreamMerger) Close() error {
	var errs []error
	if m.evenReader != nil {
		if err := m.evenReader.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close even reader: %w", err))
		}
	}
	if m.oddReader != nil {
		if err := m.oddReader.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close odd reader: %w", err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing readers: %v", errs)
	}
	return nil
}
