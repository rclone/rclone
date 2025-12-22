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

	// Read chunks from both streams
	var evenN, oddN int
	var evenErr, oddErr error

	if !m.evenEOF {
		evenN, evenErr = m.evenReader.Read(m.evenBuffer)
		if evenErr == io.EOF {
			m.evenEOF = true
			evenErr = nil
		} else if evenErr != nil {
			return 0, fmt.Errorf("failed to read even particle: %w", evenErr)
		}
	} else {
		evenN = 0 // Stream is EOF, no data read
	}

	if !m.oddEOF {
		oddN, oddErr = m.oddReader.Read(m.oddBuffer)
		if oddErr == io.EOF {
			m.oddEOF = true
			oddErr = nil
		} else if oddErr != nil {
			return 0, fmt.Errorf("failed to read odd particle: %w", oddErr)
		}
	} else {
		oddN = 0 // Stream is EOF, no data read
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
		mergeSize := len(evenData)
		if len(oddData) < mergeSize {
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

		// After buffering, sizes should match during streaming
		if len(evenData) != len(oddData) {
			log.Printf("[StreamMerger] Read: UNEXPECTED SIZE MISMATCH during streaming - even=%d, odd=%d, evenEOF=%v, oddEOF=%v, evenPending=%d, oddPending=%d", len(evenData), len(oddData), m.evenEOF, m.oddEOF, len(m.evenPending), len(m.oddPending))
			return 0, fmt.Errorf("unexpected size mismatch during streaming: even=%d, odd=%d", len(evenData), len(oddData))
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

	// Read chunks from both streams
	var dataN, parityN int
	var dataErr, parityErr error

	if !r.dataEOF {
		dataN, dataErr = r.dataReader.Read(r.dataBuffer)
		if dataErr == io.EOF {
			r.dataEOF = true
			dataErr = nil
		} else if dataErr != nil {
			return 0, fmt.Errorf("failed to read data particle: %w", dataErr)
		}
	}

	if !r.parityEOF {
		parityN, parityErr = r.parityReader.Read(r.parityBuffer)
		if parityErr == io.EOF {
			r.parityEOF = true
			parityErr = nil
		} else if parityErr != nil {
			return 0, fmt.Errorf("failed to read parity particle: %w", parityErr)
		}
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

// StreamSplitter splits an input stream into even, odd, and parity streams
// It processes data in chunks to maintain constant memory usage
// CRITICAL: Must maintain global byte indices for correct byte-level striping
//
// DEPRECATED: This type is no longer used in the production code path.
// The streaming implementation now uses a pipelined chunked approach instead of io.Pipe.
// This type is kept for backward compatibility and testing purposes only.
// TODO: Remove this type once tests are updated to use the new approach.
type StreamSplitter struct {
	evenWriter   io.Writer
	oddWriter    io.Writer
	parityWriter io.Writer
	chunkSize    int
	buffer       []byte
	evenBuffer   []byte
	oddBuffer    []byte
	parityBuffer []byte
	totalBytes   int64
	isOddLength  bool
	globalOffset int64 // Track global byte position for correct striping
	evenWritten  int64 // Track total bytes written to even stream
	oddWritten   int64 // Track total bytes written to odd stream
	mu           sync.Mutex
	debugName    string // For debug logging
}

// NewStreamSplitter creates a new StreamSplitter that splits input into even, odd, and parity streams
func NewStreamSplitter(evenWriter, oddWriter, parityWriter io.Writer, chunkSize int) *StreamSplitter {
	return &StreamSplitter{
		evenWriter:   evenWriter,
		oddWriter:    oddWriter,
		parityWriter: parityWriter,
		chunkSize:    chunkSize,
		buffer:       make([]byte, chunkSize),
		evenBuffer:   make([]byte, 0, chunkSize),
		oddBuffer:    make([]byte, 0, chunkSize),
		parityBuffer: make([]byte, chunkSize),
		totalBytes:   0,
		isOddLength:  false,
		globalOffset: 0,
		evenWritten:  0,
		oddWritten:   0,
		debugName:    "StreamSplitter",
	}
}

// Write processes input data in chunks, splitting into even, odd, and parity streams
// CRITICAL: Must maintain global byte indices for correct byte-level striping
// Optimized to process data in bulk rather than byte-by-byte
func (s *StreamSplitter) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	initialTotalBytes := s.totalBytes
	initialEvenWritten := s.evenWritten
	initialOddWritten := s.oddWritten
	initialEvenBufferLen := len(s.evenBuffer)
	initialOddBufferLen := len(s.oddBuffer)
	s.mu.Unlock()
	
	log.Printf("[%s] Write: called with %d bytes, totalBytes=%d, evenWritten=%d, oddWritten=%d, evenBuffer=%d, oddBuffer=%d",
		s.debugName, len(p), initialTotalBytes, initialEvenWritten, initialOddWritten, initialEvenBufferLen, initialOddBufferLen)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Process input in bulk based on global offset
	startOffset := s.globalOffset
	remaining := p
	
	for len(remaining) > 0 {
		// Determine if current global position is even or odd
		isEvenStart := (startOffset % 2) == 0
		
		// Calculate how much we can process in this iteration
		// We want to process enough to fill buffers or complete the input
		processSize := len(remaining)
		if processSize > s.chunkSize*2 {
			processSize = s.chunkSize * 2 // Process up to 2 chunks worth
		}
		
		chunk := remaining[:processSize]
		remaining = remaining[processSize:]
		
		// Split chunk based on starting position using bulk operations
		if isEvenStart {
			// Starting at even position: pattern is even, odd, even, odd, ...
			// Calculate sizes needed
			evenCount := (len(chunk) + 1) / 2
			oddCount := len(chunk) / 2
			
			// Pre-allocate space if needed
			if cap(s.evenBuffer) < len(s.evenBuffer)+evenCount {
				newBuf := make([]byte, len(s.evenBuffer), len(s.evenBuffer)+evenCount+s.chunkSize)
				copy(newBuf, s.evenBuffer)
				s.evenBuffer = newBuf
			}
			if cap(s.oddBuffer) < len(s.oddBuffer)+oddCount {
				newBuf := make([]byte, len(s.oddBuffer), len(s.oddBuffer)+oddCount+s.chunkSize)
				copy(newBuf, s.oddBuffer)
				s.oddBuffer = newBuf
			}
			
			// Bulk copy even bytes (indices 0, 2, 4, ...)
			oldEvenLen := len(s.evenBuffer)
			s.evenBuffer = s.evenBuffer[:oldEvenLen+evenCount]
			for i, j := 0, oldEvenLen; i < len(chunk); i += 2 {
				s.evenBuffer[j] = chunk[i]
				j++
			}
			
			// Bulk copy odd bytes (indices 1, 3, 5, ...)
			oldOddLen := len(s.oddBuffer)
			s.oddBuffer = s.oddBuffer[:oldOddLen+oddCount]
			for i, j := 1, oldOddLen; i < len(chunk); i += 2 {
				s.oddBuffer[j] = chunk[i]
				j++
			}
		} else {
			// Starting at odd position: pattern is odd, even, odd, even, ...
			// Calculate sizes needed
			oddCount := (len(chunk) + 1) / 2
			evenCount := len(chunk) / 2
			
			// Pre-allocate space if needed
			if cap(s.oddBuffer) < len(s.oddBuffer)+oddCount {
				newBuf := make([]byte, len(s.oddBuffer), len(s.oddBuffer)+oddCount+s.chunkSize)
				copy(newBuf, s.oddBuffer)
				s.oddBuffer = newBuf
			}
			if cap(s.evenBuffer) < len(s.evenBuffer)+evenCount {
				newBuf := make([]byte, len(s.evenBuffer), len(s.evenBuffer)+evenCount+s.chunkSize)
				copy(newBuf, s.evenBuffer)
				s.evenBuffer = newBuf
			}
			
			// Bulk copy odd bytes (indices 0, 2, 4, ... of chunk -> odd stream)
			oldOddLen := len(s.oddBuffer)
			s.oddBuffer = s.oddBuffer[:oldOddLen+oddCount]
			for i, j := 0, oldOddLen; i < len(chunk); i += 2 {
				s.oddBuffer[j] = chunk[i]
				j++
			}
			
			// Bulk copy even bytes (indices 1, 3, 5, ... of chunk -> even stream)
			oldEvenLen := len(s.evenBuffer)
			s.evenBuffer = s.evenBuffer[:oldEvenLen+evenCount]
			for i, j := 1, oldEvenLen; i < len(chunk); i += 2 {
				s.evenBuffer[j] = chunk[i]
				j++
			}
		}
		
		// Update global offset for next iteration (before flushing, in case flush modifies state)
		startOffset += int64(processSize)
		
		// Flush buffers if they're large enough
		// Extract data while holding lock to prevent race conditions
		// CRITICAL: Check buffer sizes AFTER adding current chunk data
		// Use a smaller flush threshold (64KB) to avoid overwhelming io.Pipe with large writes
		// io.Pipe has a small buffer (~64KB), so large writes can block or fail
		// Writing chunks larger than the pipe buffer causes blocking and potential data loss
		flushThreshold := 64 * 1024 // 64KB - matches io.Pipe buffer size
		
		// Flush in a loop until buffers are below threshold
		// This ensures we never write more than 64KB at once to io.Pipe
		for len(s.evenBuffer) >= flushThreshold || len(s.oddBuffer) >= flushThreshold {
			// Determine how much to flush (min of both buffers, but limit to flushThreshold)
			// This ensures we never write more than 64KB at once to io.Pipe
			flushSize := len(s.evenBuffer)
			if len(s.oddBuffer) < flushSize {
				flushSize = len(s.oddBuffer)
			}
			// CRITICAL: Limit flush size to flushThreshold to avoid overwhelming io.Pipe
			if flushSize > flushThreshold {
				flushSize = flushThreshold
			}
			
			if flushSize > 0 {
				// Extract data to flush WHILE HOLDING LOCK
				evenData := make([]byte, flushSize)
				copy(evenData, s.evenBuffer[:flushSize])
				oddData := make([]byte, flushSize)
				copy(oddData, s.oddBuffer[:flushSize])
				
				// Calculate parity
				parityData := CalculateParity(evenData, oddData)
				
				// Update buffers WHILE HOLDING LOCK
				remainingEven := len(s.evenBuffer) - flushSize
				remainingOdd := len(s.oddBuffer) - flushSize
				
				if remainingEven > 0 {
					remaining := make([]byte, remainingEven, s.chunkSize)
					copy(remaining, s.evenBuffer[flushSize:])
					s.evenBuffer = remaining
				} else {
					s.evenBuffer = s.evenBuffer[:0] // Reset to empty but keep capacity
				}
				if remainingOdd > 0 {
					remaining := make([]byte, remainingOdd, s.chunkSize)
					copy(remaining, s.oddBuffer[flushSize:])
					s.oddBuffer = remaining
				} else {
					s.oddBuffer = s.oddBuffer[:0] // Reset to empty but keep capacity
				}
				
				// NOW unlock for I/O operations (these may block, but that's OK)
				s.mu.Unlock()
				
				// Calculate remaining BEFORE modifying buffers (for logging)
				remainingEvenAfterFlush := remainingEven
				remainingOddAfterFlush := remainingOdd
				log.Printf("[%s] Write: flushing %d bytes (even=%d, odd=%d), evenBuffer=%d, oddBuffer=%d remaining after flush",
					s.debugName, flushSize, len(evenData), len(oddData), remainingEvenAfterFlush, remainingOddAfterFlush)
				
				// Write to all three streams sequentially (io.Pipe is NOT thread-safe for concurrent writes)
				// CRITICAL: Write all data BEFORE updating counters to ensure atomicity
				// If any write fails, we don't update counters, maintaining consistency
				
				// Write even data
				log.Printf("[%s] Write: writing %d bytes to even pipe", s.debugName, len(evenData))
				n, err := s.evenWriter.Write(evenData)
				if err != nil {
					s.mu.Lock()
					return len(p) - len(remaining), fmt.Errorf("failed to write even data: %w", err)
				}
				if n != len(evenData) {
					s.mu.Lock()
					return len(p) - len(remaining), fmt.Errorf("partial write to even stream: wrote %d of %d bytes", n, len(evenData))
				}
				
				// Write odd data
				log.Printf("[%s] Write: writing %d bytes to odd pipe", s.debugName, len(oddData))
				n, err = s.oddWriter.Write(oddData)
				if err != nil {
					s.mu.Lock()
					return len(p) - len(remaining), fmt.Errorf("failed to write odd data: %w", err)
				}
				if n != len(oddData) {
					s.mu.Lock()
					return len(p) - len(remaining), fmt.Errorf("partial write to odd stream: wrote %d of %d bytes", n, len(oddData))
				}
				
				// Write parity data
				log.Printf("[%s] Write: writing %d bytes to parity pipe", s.debugName, len(parityData))
				if _, err := s.parityWriter.Write(parityData); err != nil {
					s.mu.Lock()
					return len(p) - len(remaining), fmt.Errorf("failed to write parity data: %w", err)
				}
				
				// All writes succeeded - now update counters while relocking
				s.mu.Lock()
				s.evenWritten += int64(flushSize)
				s.oddWritten += int64(flushSize)
				log.Printf("[%s] Write: flush complete, evenWritten=%d, oddWritten=%d, evenBuffer=%d, oddBuffer=%d", 
					s.debugName, s.evenWritten, s.oddWritten, len(s.evenBuffer), len(s.oddBuffer))
				// Continue loop if buffers are still above threshold
			} else {
				break // No more data to flush
			}
		}
	}
	
	// Update global offset
	s.globalOffset += int64(len(p))
	s.totalBytes += int64(len(p))
	
	log.Printf("[%s] Write: returning, processed %d bytes, totalBytes=%d, evenWritten=%d, oddWritten=%d, evenBuffer=%d, oddBuffer=%d",
		s.debugName, len(p), s.totalBytes, s.evenWritten, s.oddWritten, len(s.evenBuffer), len(s.oddBuffer))
	
	return len(p), nil
}

// flushBuffers writes buffered even/odd bytes and calculates parity
// NOTE: Must be called WITHOUT holding the mutex to avoid deadlocks
// This is used by Close() which needs to flush remaining data
func (s *StreamSplitter) flushBuffers() error {
	s.mu.Lock()
	// Determine how much to flush (min of both buffers, but limit to 64KB)
	// This ensures we never write more than 64KB at once to io.Pipe
	flushThreshold := 64 * 1024 // 64KB - matches io.Pipe buffer size
	flushSize := len(s.evenBuffer)
	if len(s.oddBuffer) < flushSize {
		flushSize = len(s.oddBuffer)
	}
	// CRITICAL: Limit flush size to flushThreshold to avoid overwhelming io.Pipe
	if flushSize > flushThreshold {
		flushSize = flushThreshold
	}
	
	if flushSize == 0 {
		s.mu.Unlock()
		return nil
	}
	
	// Extract data to flush - COPY the data to avoid issues with buffer modifications
	evenData := make([]byte, flushSize)
	copy(evenData, s.evenBuffer[:flushSize])
	oddData := make([]byte, flushSize)
	copy(oddData, s.oddBuffer[:flushSize])
	
	// Remove flushed data from buffers
	// Use slice operations but ensure we don't share underlying array with written data
	remainingEven := len(s.evenBuffer) - flushSize
	remainingOdd := len(s.oddBuffer) - flushSize
	
	// If there's remaining data, copy it to a new slice to avoid sharing underlying array
	if remainingEven > 0 {
		remaining := make([]byte, remainingEven, s.chunkSize)
		copy(remaining, s.evenBuffer[flushSize:])
		s.evenBuffer = remaining
	} else {
		s.evenBuffer = s.evenBuffer[:0] // Reset to empty but keep capacity
	}
	if remainingOdd > 0 {
		remaining := make([]byte, remainingOdd, s.chunkSize)
		copy(remaining, s.oddBuffer[flushSize:])
		s.oddBuffer = remaining
	} else {
		s.oddBuffer = s.oddBuffer[:0] // Reset to empty but keep capacity
	}
	
	// Calculate parity for this chunk
	parityData := CalculateParity(evenData, oddData)
	
	// Track bytes to write (for counter update after successful writes)
	evenBytes := int64(len(evenData))
	oddBytes := int64(len(oddData))
	
	// Release lock before I/O operations
	s.mu.Unlock()
	
	log.Printf("[%s] flushBuffers: flushing %d bytes (even=%d, odd=%d)", s.debugName, flushSize, len(evenData), len(oddData))
	
	// Write to all three streams sequentially (io.Pipe is NOT thread-safe for concurrent writes)
	// CRITICAL: Write all data BEFORE updating counters to ensure atomicity
	// If any write fails, we don't update counters, maintaining consistency
	
	// Write even data
	log.Printf("[%s] flushBuffers: writing %d bytes to even pipe", s.debugName, len(evenData))
	n, err := s.evenWriter.Write(evenData)
	if err != nil {
		return fmt.Errorf("failed to write even data: %w", err)
	}
	if n != len(evenData) {
		return fmt.Errorf("partial write to even stream: wrote %d of %d bytes", n, len(evenData))
	}
	
	// Write odd data
	log.Printf("[%s] flushBuffers: writing %d bytes to odd pipe", s.debugName, len(oddData))
	n, err = s.oddWriter.Write(oddData)
	if err != nil {
		return fmt.Errorf("failed to write odd data: %w", err)
	}
	if n != len(oddData) {
		return fmt.Errorf("partial write to odd stream: wrote %d of %d bytes", n, len(oddData))
	}
	
	// Write parity data
	log.Printf("[%s] flushBuffers: writing %d bytes to parity pipe", s.debugName, len(parityData))
	if _, err := s.parityWriter.Write(parityData); err != nil {
		log.Printf("[%s] flushBuffers: parity write failed: %v", s.debugName, err)
		return fmt.Errorf("failed to write parity data: %w", err)
	}
	
	// All writes succeeded - update counters
	s.mu.Lock()
	s.evenWritten += evenBytes
	s.oddWritten += oddBytes
	log.Printf("[%s] flushBuffers: completed, evenWritten=%d, oddWritten=%d", s.debugName, s.evenWritten, s.oddWritten)
	s.mu.Unlock()
	
	return nil
}

// Close finalizes the splitter and handles odd-length files
func (s *StreamSplitter) Close() error {
	s.mu.Lock()
	initialTotalBytes := s.totalBytes
	initialEvenWritten := s.evenWritten
	initialOddWritten := s.oddWritten
	initialEvenBufferLen := len(s.evenBuffer)
	initialOddBufferLen := len(s.oddBuffer)
	s.mu.Unlock()
	
	log.Printf("[%s] Close: starting, totalBytes=%d, evenWritten=%d, oddWritten=%d, evenBuffer=%d, oddBuffer=%d",
		s.debugName, initialTotalBytes, initialEvenWritten, initialOddWritten, initialEvenBufferLen, initialOddBufferLen)
	
	// Flush all remaining buffered data
	// Keep flushing until both buffers are empty (or only one has remaining data for odd-length)
	flushCount := 0
	for {
		s.mu.Lock()
		evenLen := len(s.evenBuffer)
		oddLen := len(s.oddBuffer)
		s.mu.Unlock()
		
		log.Printf("[%s] Close: flush loop iteration %d, evenBuffer=%d, oddBuffer=%d", s.debugName, flushCount, evenLen, oddLen)
		
		// If both buffers are empty, we're done
		if evenLen == 0 && oddLen == 0 {
			break
		}
		
		// If buffers are equal, flush them
		if evenLen == oddLen && evenLen > 0 {
			log.Printf("[%s] Close: flushing equal buffers (%d bytes each)", s.debugName, evenLen)
			if err := s.flushBuffers(); err != nil {
				log.Printf("[%s] Close: flushBuffers() failed: %v", s.debugName, err)
				return err
			}
			flushCount++
			continue
		}
		
		// If even has one more byte (odd-length file), flush matching pairs first
		if evenLen == oddLen+1 {
			// First flush the matching pairs (oddLen bytes from both)
			if oddLen > 0 {
				log.Printf("[%s] Close: flushing odd-length file pairs (%d bytes each, 1 byte remaining)", s.debugName, oddLen)
				if err := s.flushBuffers(); err != nil {
					log.Printf("[%s] Close: flushBuffers() failed: %v", s.debugName, err)
					return err
				}
				flushCount++
				// After flushing, evenBuffer should have 1 byte left, oddBuffer should be empty
				continue // Loop to handle the remaining even byte
			}
			
			// Only the last even byte remains (oddLen was 0)
			s.mu.Lock()
			evenData := make([]byte, len(s.evenBuffer))
			copy(evenData, s.evenBuffer)
			s.evenBuffer = s.evenBuffer[:0]
			s.mu.Unlock()
			
			// Write final even byte
			n, err := s.evenWriter.Write(evenData)
			if err != nil {
				return fmt.Errorf("failed to write final even byte: %w", err)
			}
			if n != len(evenData) {
				return fmt.Errorf("partial write of final even byte: wrote %d of %d bytes", n, len(evenData))
			}
			s.mu.Lock()
			s.evenWritten += int64(n)
			s.mu.Unlock()
			// Parity for last byte is just the even byte itself
			if _, err := s.parityWriter.Write(evenData); err != nil {
				return fmt.Errorf("failed to write final parity byte: %w", err)
			}
			break
		}
		
		// If buffers are unequal in unexpected way, error
		if evenLen != oddLen {
			return fmt.Errorf("invalid buffer state: even=%d, odd=%d", evenLen, oddLen)
		}
		
		// If we get here, both are 0, break
		break
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Determine if file is odd length
	s.isOddLength = (s.totalBytes % 2) == 1

	log.Printf("[%s] Close: completed, totalBytes=%d, evenWritten=%d, oddWritten=%d, isOddLength=%v, flushCount=%d",
		s.debugName, s.totalBytes, s.evenWritten, s.oddWritten, s.isOddLength, flushCount)

	// No additional cleanup needed - writers are managed externally
	return nil
}

// IsOddLength returns whether the processed file was odd length
func (s *StreamSplitter) IsOddLength() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isOddLength
}

// GetEvenWritten returns the total number of bytes written to the even stream
func (s *StreamSplitter) GetEvenWritten() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.evenWritten
}

// GetOddWritten returns the total number of bytes written to the odd stream
func (s *StreamSplitter) GetOddWritten() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.oddWritten
}

// GetTotalBytes returns the total number of bytes processed
func (s *StreamSplitter) GetTotalBytes() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.totalBytes
}
