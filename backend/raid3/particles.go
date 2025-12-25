// Package raid3 implements a backend that splits data across three remotes using byte-level striping
package raid3

// This file contains core particle operations for data splitting and reconstruction.
//
// It includes:
//   - SplitBytes: Split data into even and odd indexed bytes
//   - MergeBytes: Merge even and odd bytes back into original data
//   - CalculateParity: Compute XOR parity for even and odd particles
//   - Reconstruction functions: ReconstructFromEvenAndParity, ReconstructFromOddAndParity, ReconstructFromEvenAndOdd
//   - Particle counting and scanning utilities
//   - Parity filename helpers (GetParityFilename, StripParitySuffix)

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
)

// SplitBytes splits data into even and odd indexed bytes
func SplitBytes(data []byte) (even []byte, odd []byte) {
	evenLen := (len(data) + 1) / 2
	oddLen := len(data) / 2

	even = make([]byte, evenLen)
	odd = make([]byte, oddLen)

	for i := 0; i < len(data); i++ {
		if i%2 == 0 {
			even[i/2] = data[i]
		} else {
			odd[i/2] = data[i]
		}
	}
	return even, odd
}

// MergeBytes merges even and odd indexed bytes back into original data
func MergeBytes(even []byte, odd []byte) ([]byte, error) {
	// Validate sizes: even should equal odd or be one byte larger
	if len(even) != len(odd) && len(even) != len(odd)+1 {
		return nil, fmt.Errorf("invalid particle sizes: even=%d, odd=%d (expected even=odd or even=odd+1)", len(even), len(odd))
	}

	result := make([]byte, len(even)+len(odd))
	for i := 0; i < len(result); i++ {
		if i%2 == 0 {
			result[i] = even[i/2]
		} else {
			result[i] = odd[i/2]
		}
	}
	return result, nil
}

// CalculateParity calculates XOR parity for even and odd particles
// For even-length data: parity[i] = even[i] XOR odd[i]
// For odd-length data: parity[i] = even[i] XOR odd[i], except last byte which is just even[last]
func CalculateParity(even []byte, odd []byte) []byte {
	parityLen := len(even) // Parity size always equals even size
	parity := make([]byte, parityLen)

	// XOR pairs of even and odd bytes
	for i := 0; i < len(odd); i++ {
		parity[i] = even[i] ^ odd[i]
	}

	// If odd length, last parity byte is just the last even byte (no XOR partner)
	if len(even) > len(odd) {
		parity[len(even)-1] = even[len(even)-1]
	}

	return parity
}

// ReconstructFromEvenAndParity reconstructs the original data from even + parity.
// If isOddLength is true, the last even byte equals the last parity byte.
func ReconstructFromEvenAndParity(even []byte, parity []byte, isOddLength bool) ([]byte, error) {
	if len(even) != len(parity) {
		return nil, fmt.Errorf("invalid sizes for reconstruction (even=%d parity=%d)", len(even), len(parity))
	}

	// Reconstruct odd bytes from parity ^ even
	odd := make([]byte, len(even))
	for i := 0; i < len(even); i++ {
		odd[i] = parity[i] ^ even[i]
	}
	// If original length was odd, odd has one fewer byte
	if isOddLength {
		odd = odd[:len(odd)-1]
	}

	return MergeBytes(even, odd)
}

// ReconstructFromOddAndParity reconstructs the original data from odd + parity.
// If isOddLength is true, the last even byte equals the last parity byte.
func ReconstructFromOddAndParity(odd []byte, parity []byte, isOddLength bool) ([]byte, error) {
	// Even length equals parity length. For odd original, even has one extra byte.
	even := make([]byte, len(parity))
	// Reconstruct pairs where odd exists
	for i := 0; i < len(odd); i++ {
		even[i] = parity[i] ^ odd[i]
	}
	// If original length was odd, last even byte equals last parity byte
	if isOddLength {
		even[len(even)-1] = parity[len(parity)-1]
	}

	return MergeBytes(even, odd)
}

// GetParityFilename returns the parity filename with appropriate suffix
func GetParityFilename(original string, isOddLength bool) string {
	if isOddLength {
		return original + ".parity-ol"
	}
	return original + ".parity-el"
}

// StripParitySuffix removes the parity suffix from a filename
func StripParitySuffix(filename string) (string, bool, bool) {
	if strings.HasSuffix(filename, ".parity-ol") {
		return strings.TrimSuffix(filename, ".parity-ol"), true, true
	}
	if strings.HasSuffix(filename, ".parity-el") {
		return strings.TrimSuffix(filename, ".parity-el"), true, false
	}
	return filename, false, false
}

// IsTempFile checks if a filename is a temporary file created during Update rollback
// Temporary files have suffixes like .tmp.even, .tmp.odd, .tmp.parity
func IsTempFile(filename string) bool {
	return strings.HasSuffix(filename, ".tmp.even") ||
		strings.HasSuffix(filename, ".tmp.odd") ||
		strings.HasSuffix(filename, ".tmp.parity")
}

// ValidateParticleSizes checks if particle sizes are valid
func ValidateParticleSizes(evenSize, oddSize int64) bool {
	return evenSize == oddSize || evenSize == oddSize+1
}

// particleInfo holds information about which particles exist for an object
type particleInfo struct {
	remote       string
	evenExists   bool
	oddExists    bool
	parityExists bool
	count        int
	size         int64 // Size of single particle (for broken objects)
}

// countParticles counts the number of particles on a backend
func (f *Fs) countParticles(ctx context.Context, backend fs.Fs) (int64, error) {
	var count int64
	err := operations.ListFn(ctx, backend, func(obj fs.Object) {
		count++
	})
	if err != nil && !errors.Is(err, fs.ErrorDirNotFound) {
		return 0, err
	}
	return count, nil
}

// reconstructParityParticle reconstructs a parity particle from even + odd
func (f *Fs) reconstructParityParticle(ctx context.Context, evenFs, oddFs fs.Fs, remote string) ([]byte, error) {
	// Read even particle
	evenObj, err := evenFs.NewObject(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("%s: even particle not found: %w", evenFs.Name(), err)
	}
	evenReader, err := evenObj.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to open even particle: %w", evenFs.Name(), err)
	}
	evenData, err := io.ReadAll(evenReader)
	evenReader.Close()
	if err != nil {
		return nil, fmt.Errorf("%s: failed to read even particle: %w", evenFs.Name(), err)
	}

	// Read odd particle
	oddObj, err := oddFs.NewObject(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("%s: odd particle not found: %w", oddFs.Name(), err)
	}
	oddReader, err := oddObj.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to open odd particle: %w", oddFs.Name(), err)
	}
	oddData, err := io.ReadAll(oddReader)
	oddReader.Close()
	if err != nil {
		return nil, fmt.Errorf("%s: failed to read odd particle: %w", oddFs.Name(), err)
	}

	// Calculate parity
	parityData := CalculateParity(evenData, oddData)
	return parityData, nil
}

// reconstructDataParticle reconstructs a data particle (even or odd) from the other data + parity
func (f *Fs) reconstructDataParticle(ctx context.Context, dataFs, parityFs fs.Fs, remote string, targetType string) ([]byte, error) {
	// For data particles, we need to read from parity backend with suffix
	// First, try to find the parity file
	parityOdd := GetParityFilename(remote, true)
	parityEven := GetParityFilename(remote, false)

	var parityObj fs.Object
	var isOddLength bool
	var err error

	parityObj, err = parityFs.NewObject(ctx, parityOdd)
	if err == nil {
		isOddLength = true
	} else {
		parityObj, err = parityFs.NewObject(ctx, parityEven)
		if err == nil {
			isOddLength = false
		} else {
			return nil, fmt.Errorf("%s: parity particle not found (tried both suffixes): %w", parityFs.Name(), err)
		}
	}

	// Read parity data
	parityReader, err := parityObj.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to open parity particle: %w", parityFs.Name(), err)
	}
	parityData, err := io.ReadAll(parityReader)
	parityReader.Close()
	if err != nil {
		return nil, fmt.Errorf("%s: failed to read parity particle: %w", parityFs.Name(), err)
	}

	// Read the available data particle
	dataObj, err := dataFs.NewObject(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("%s: data particle not found: %w", dataFs.Name(), err)
	}
	dataReader, err := dataObj.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to open data particle: %w", dataFs.Name(), err)
	}
	dataData, err := io.ReadAll(dataReader)
	dataReader.Close()
	if err != nil {
		return nil, fmt.Errorf("%s: failed to read data particle: %w", dataFs.Name(), err)
	}

	// Reconstruct missing particle
	if targetType == "even" {
		// Reconstruct even from odd + parity
		reconstructed, err := ReconstructFromOddAndParity(dataData, parityData, isOddLength)
		if err != nil {
			return nil, fmt.Errorf("failed to reconstruct even particle: %w", err)
		}
		evenData, _ := SplitBytes(reconstructed)
		return evenData, nil
	} else {
		// Reconstruct odd from even + parity
		reconstructed, err := ReconstructFromEvenAndParity(dataData, parityData, isOddLength)
		if err != nil {
			return nil, fmt.Errorf("failed to reconstruct odd particle: %w", err)
		}
		_, oddData := SplitBytes(reconstructed)
		return oddData, nil
	}
}

// countParticlesSync counts how many particles exist for an object (0-3)
// This is used by List() when auto_cleanup is enabled
func (f *Fs) countParticlesSync(ctx context.Context, remote string) int {
	type result struct {
		name   string
		exists bool
	}
	resultCh := make(chan result, 3)

	// Check even particle
	go func() {
		_, err := f.even.NewObject(ctx, remote)
		resultCh <- result{"even", err == nil}
	}()

	// Check odd particle
	go func() {
		_, err := f.odd.NewObject(ctx, remote)
		resultCh <- result{"odd", err == nil}
	}()

	// Check parity particle (both suffixes)
	go func() {
		_, errOL := f.parity.NewObject(ctx, GetParityFilename(remote, true))
		_, errEL := f.parity.NewObject(ctx, GetParityFilename(remote, false))
		resultCh <- result{"parity", errOL == nil || errEL == nil}
	}()

	// Collect results
	count := 0
	for i := 0; i < 3; i++ {
		res := <-resultCh
		if res.exists {
			count++
		}
	}

	return count
}

// particleInfoForObject inspects a single object and returns which particles exist.
func (f *Fs) particleInfoForObject(ctx context.Context, remote string) (particleInfo, error) {
	pi := particleInfo{remote: remote}

	// Check even
	if _, err := f.even.NewObject(ctx, remote); err == nil {
		pi.evenExists = true
	}

	// Check odd
	if _, err := f.odd.NewObject(ctx, remote); err == nil {
		pi.oddExists = true
	}

	// Check parity (both suffixes)
	if _, errOL := f.parity.NewObject(ctx, GetParityFilename(remote, true)); errOL == nil {
		pi.parityExists = true
	} else if _, errEL := f.parity.NewObject(ctx, GetParityFilename(remote, false)); errEL == nil {
		pi.parityExists = true
	}

	pi.count = 0
	if pi.evenExists {
		pi.count++
	}
	if pi.oddExists {
		pi.count++
	}
	if pi.parityExists {
		pi.count++
	}

	return pi, nil
}

// scanParticles scans a directory and returns particle information for all objects
// This is used by the Cleanup() command to identify broken objects
func (f *Fs) scanParticles(ctx context.Context, dir string) ([]particleInfo, error) {
	// Collect all entries from all backends (without filtering)
	entriesEven, _ := f.even.List(ctx, dir)
	entriesOdd, _ := f.odd.List(ctx, dir)
	entriesParity, _ := f.parity.List(ctx, dir)

	// Build map of all unique object paths
	objectMap := make(map[string]*particleInfo)

	// Process even particles
	for _, entry := range entriesEven {
		if _, ok := entry.(fs.Object); ok {
			remote := entry.Remote()
			if objectMap[remote] == nil {
				objectMap[remote] = &particleInfo{remote: remote}
			}
			objectMap[remote].evenExists = true
		}
	}

	// Process odd particles
	for _, entry := range entriesOdd {
		if _, ok := entry.(fs.Object); ok {
			remote := entry.Remote()
			if objectMap[remote] == nil {
				objectMap[remote] = &particleInfo{remote: remote}
			}
			objectMap[remote].oddExists = true
		}
	}

	// Process parity particles
	for _, entry := range entriesParity {
		if _, ok := entry.(fs.Object); ok {
			remote := entry.Remote()
			// Strip parity suffix to get base object name
			baseRemote, isParity, _ := StripParitySuffix(remote)
			if isParity {
				// Proper parity file with suffix
				if objectMap[baseRemote] == nil {
					objectMap[baseRemote] = &particleInfo{remote: baseRemote}
				}
				objectMap[baseRemote].parityExists = true
			} else {
				// File in parity remote without suffix (orphaned/manually created)
				// Still track it as it might be a broken object
				if objectMap[remote] == nil {
					objectMap[remote] = &particleInfo{remote: remote}
				}
				objectMap[remote].parityExists = true
			}
		}
	}

	// Calculate counts
	result := make([]particleInfo, 0, len(objectMap))
	for _, info := range objectMap {
		if info.evenExists {
			info.count++
		}
		if info.oddExists {
			info.count++
		}
		if info.parityExists {
			info.count++
		}
		result = append(result, *info)
	}

	return result, nil
}

// StreamMerger merges even and odd particle streams into original data stream
// It processes data in chunks to maintain constant memory usage
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
	evenPending  []byte
	oddPending   []byte
	mu           sync.Mutex
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
	if m.outputBuffer != nil && len(m.outputBuffer) > 0 && m.evenPos < len(m.outputBuffer) {
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
			log.Printf("[StreamMerger] Read: INVALID PARTICLE SIZES at EOF - even=%d, odd=%d, evenPending=%d, oddPending=%d", len(evenData), len(oddData), len(m.evenPending), len(m.oddPending))
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
				log.Printf("[StreamMerger] Read: UNEXPECTED SIZE MISMATCH during streaming - even=%d, odd=%d, evenEOF=%v, oddEOF=%v, evenPending=%d, oddPending=%d", len(evenData), len(oddData), m.evenEOF, m.oddEOF, len(m.evenPending), len(m.oddPending))
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

// StreamReconstructor reconstructs missing particle from available data + parity stream
// It processes data in chunks to maintain constant memory usage
type StreamReconstructor struct {
	dataReader   io.ReadCloser // even or odd
	parityReader io.ReadCloser
	mode         string // "even+parity" or "odd+parity"
	isOddLength  bool
	chunkSize    int
	dataBuffer   []byte
	parityBuffer []byte
	outputBuffer []byte
	dataPos      int
	parityPos    int
	dataEOF      bool
	parityEOF    bool
	mu           sync.Mutex
}

// NewStreamReconstructor creates a new StreamReconstructor for degraded mode
func NewStreamReconstructor(dataReader, parityReader io.ReadCloser, mode string, isOddLength bool, chunkSize int) *StreamReconstructor {
	return &StreamReconstructor{
		dataReader:   dataReader,
		parityReader: parityReader,
		mode:         mode,
		isOddLength:  isOddLength,
		chunkSize:    chunkSize,
		dataBuffer:   make([]byte, chunkSize),
		parityBuffer: make([]byte, chunkSize),
		outputBuffer: make([]byte, 0, chunkSize*2), // Output buffer (empty initially, capacity 2x chunk size)
		dataPos:      0,
		parityPos:     0,
		dataEOF:       false,
		parityEOF:     false,
	}
}

// Read reads reconstructed data from data + parity streams
func (r *StreamReconstructor) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// If we have buffered output, return it first
	if r.outputBuffer != nil && len(r.outputBuffer) > 0 && r.dataPos < len(r.outputBuffer) {
		n = copy(p, r.outputBuffer[r.dataPos:])
		r.dataPos += n
		if r.dataPos >= len(r.outputBuffer) {
			// Clear buffer completely - create new empty slice
			r.outputBuffer = nil
			r.dataPos = 0
		}
		return n, nil
	}

	// If both streams are EOF, we're done
	if r.dataEOF && r.parityEOF {
		return 0, io.EOF
	}

	// Read chunks from both streams concurrently
	// Unlock mutex during I/O operations (they may block)
	type readResult struct {
		n      int
		err    error
		hitEOF bool // Track if io.EOF was encountered
	}
	dataCh := make(chan readResult, 1)
	parityCh := make(chan readResult, 1)

	// Read from data stream concurrently
	if !r.dataEOF {
		go func() {
			n, err := r.dataReader.Read(r.dataBuffer)
			hitEOF := (err == io.EOF)
			// Convert io.EOF to nil error (standard Go pattern: EOF is not an error)
			if err == io.EOF {
				err = nil
			}
			dataCh <- readResult{n: n, err: err, hitEOF: hitEOF}
		}()
	} else {
		dataCh <- readResult{n: 0, err: nil, hitEOF: true} // Stream is EOF
	}

	// Read from parity stream concurrently
	if !r.parityEOF {
		go func() {
			n, err := r.parityReader.Read(r.parityBuffer)
			hitEOF := (err == io.EOF)
			// Convert io.EOF to nil error (standard Go pattern: EOF is not an error)
			if err == io.EOF {
				err = nil
			}
			parityCh <- readResult{n: n, err: err, hitEOF: hitEOF}
		}()
	} else {
		parityCh <- readResult{n: 0, err: nil, hitEOF: true} // Stream is EOF
	}

	// Wait for both reads to complete
	r.mu.Unlock()
	dataRes := <-dataCh
	parityRes := <-parityCh
	r.mu.Lock()

	// Process results
	var dataN, parityN int
	var dataErr, parityErr error

	dataN = dataRes.n
	dataErr = dataRes.err
	if dataRes.hitEOF {
		r.dataEOF = true
	}
	if dataErr != nil {
		return 0, fmt.Errorf("failed to read data particle: %w", dataErr)
	}

	parityN = parityRes.n
	parityErr = parityRes.err
	if parityRes.hitEOF {
		r.parityEOF = true
	}
	if parityErr != nil {
		return 0, fmt.Errorf("failed to read parity particle: %w", parityErr)
	}

	// If both are EOF, we're done
	if r.dataEOF && r.parityEOF {
		return 0, io.EOF
	}

	// Handle case where both are empty (shouldn't happen after EOF check, but be safe)
	if dataN == 0 && parityN == 0 {
		return 0, io.EOF
	}

	// Validate sizes: data and parity should be same size
	// For RAID3, data and parity particles should always be the same size
	dataData := r.dataBuffer[:dataN]
	parityData := r.parityBuffer[:parityN]

	if len(dataData) != len(parityData) {
		// Size mismatch - this should not happen for valid RAID3 files
		// But during streaming, we might read different amounts
		// Only validate strictly if both streams are at EOF
		if r.dataEOF && r.parityEOF {
			return 0, fmt.Errorf("invalid sizes for reconstruction (data=%d parity=%d)", len(dataData), len(parityData))
		}
		// During streaming, allow size mismatch but log a warning
		// This can happen if one stream reads more than the other
		// We'll process the minimum and buffer the rest (future enhancement)
		minSize := len(dataData)
		if len(parityData) < minSize {
			minSize = len(parityData)
		}
		if minSize == 0 {
			return 0, nil // Wait for more data
		}
		dataData = dataData[:minSize]
		parityData = parityData[:minSize]
	}

	// Reconstruct missing particle
	var reconstructed []byte
	var reconErr error

	if r.mode == "even+parity" {
		reconstructed, reconErr = ReconstructFromEvenAndParity(dataData, parityData, r.isOddLength)
	} else if r.mode == "odd+parity" {
		reconstructed, reconErr = ReconstructFromOddAndParity(dataData, parityData, r.isOddLength)
	} else {
		return 0, fmt.Errorf("invalid reconstruction mode: %s", r.mode)
	}

	if reconErr != nil {
		return 0, fmt.Errorf("failed to reconstruct: %w", reconErr)
	}

	// Store reconstructed data in output buffer
	// Reset output buffer completely - create a new slice with exact size
	r.outputBuffer = make([]byte, len(reconstructed))
	copy(r.outputBuffer, reconstructed)
	r.dataPos = 0

	// Return data to caller
	if len(r.outputBuffer) == 0 {
		return 0, io.EOF
	}
	n = copy(p, r.outputBuffer[r.dataPos:])
	r.dataPos += n
	if r.dataPos >= len(r.outputBuffer) {
		// Clear buffer completely - create new empty slice
		r.outputBuffer = nil
		r.dataPos = 0
	}

	return n, nil
}

// Close closes all underlying readers
func (r *StreamReconstructor) Close() error {
	var errs []error
	if r.dataReader != nil {
		if err := r.dataReader.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close data reader: %w", err))
		}
	}
	if r.parityReader != nil {
		if err := r.parityReader.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close parity reader: %w", err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing readers: %v", errs)
	}
	return nil
}

