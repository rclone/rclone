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
//
// Note: StreamMerger has been moved to streammerger.go

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"golang.org/x/sync/errgroup"
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
	// Input validation - allow nil slices (they will be treated as empty)
	// This is necessary for edge cases like empty files or single-byte files
	if even == nil {
		even = []byte{}
	}
	if odd == nil {
		odd = []byte{}
	}

	// Validate sizes: even should equal odd or be one byte larger
	if len(even) != len(odd) && len(even) != len(odd)+1 {
		return nil, formatOperationError("merge particles failed", fmt.Sprintf("invalid particle sizes: even=%d, odd=%d (expected even=odd or even=odd+1)", len(even), len(odd)), nil)
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
		return nil, formatOperationError("reconstruct failed", fmt.Sprintf("invalid sizes for reconstruction (even=%d parity=%d)", len(even), len(parity)), nil)
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
	// Input validation
	if err := validateContext(ctx, "reconstructParityParticle"); err != nil {
		return nil, err
	}
	if err := validateBackend(evenFs, "evenFs", "reconstructParityParticle"); err != nil {
		return nil, err
	}
	if err := validateBackend(oddFs, "oddFs", "reconstructParityParticle"); err != nil {
		return nil, err
	}
	if err := validateRemote(remote, "reconstructParityParticle"); err != nil {
		return nil, err
	}
	// Read even particle
	evenObj, err := evenFs.NewObject(ctx, remote)
	if err != nil {
		return nil, formatNotFoundError(evenFs, "even particle", fmt.Sprintf("remote %q", remote), err)
	}
	evenReader, err := evenObj.Open(ctx)
	if err != nil {
		return nil, formatParticleError(evenFs, "even", "open failed", fmt.Sprintf("remote %q", remote), err)
	}
	evenData, err := io.ReadAll(evenReader)
	evenReader.Close()
	if err != nil {
		return nil, formatParticleError(evenFs, "even", "read failed", fmt.Sprintf("remote %q", remote), err)
	}

	// Read odd particle
	oddObj, err := oddFs.NewObject(ctx, remote)
	if err != nil {
		return nil, formatNotFoundError(oddFs, "odd particle", fmt.Sprintf("remote %q", remote), err)
	}
	oddReader, err := oddObj.Open(ctx)
	if err != nil {
		return nil, formatParticleError(oddFs, "odd", "open failed", fmt.Sprintf("remote %q", remote), err)
	}
	oddData, err := io.ReadAll(oddReader)
	oddReader.Close()
	if err != nil {
		return nil, formatParticleError(oddFs, "odd", "read failed", fmt.Sprintf("remote %q", remote), err)
	}

	// Calculate parity
	parityData := CalculateParity(evenData, oddData)
	return parityData, nil
}

// reconstructDataParticle reconstructs a data particle (even or odd) from the other data + parity
func (f *Fs) reconstructDataParticle(ctx context.Context, dataFs, parityFs fs.Fs, remote string, targetType string) ([]byte, error) {
	// Input validation
	if err := validateContext(ctx, "reconstructDataParticle"); err != nil {
		return nil, err
	}
	if err := validateBackend(dataFs, "dataFs", "reconstructDataParticle"); err != nil {
		return nil, err
	}
	if err := validateBackend(parityFs, "parityFs", "reconstructDataParticle"); err != nil {
		return nil, err
	}
	if err := validateRemote(remote, "reconstructDataParticle"); err != nil {
		return nil, err
	}
	if targetType != "even" && targetType != "odd" {
		return nil, formatOperationError("reconstructDataParticle failed", fmt.Sprintf("invalid targetType: %s (must be: even or odd)", targetType), nil)
	}
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
			return nil, formatNotFoundError(parityFs, "parity particle", fmt.Sprintf("remote %q (tried both suffixes)", remote), err)
		}
	}

	// Read parity data
	parityReader, err := parityObj.Open(ctx)
	if err != nil {
		return nil, formatParticleError(parityFs, "parity", "open failed", fmt.Sprintf("remote %q", remote), err)
	}
	parityData, err := io.ReadAll(parityReader)
	parityReader.Close()
	if err != nil {
		return nil, formatParticleError(parityFs, "parity", "read failed", fmt.Sprintf("remote %q", remote), err)
	}

	// Read the available data particle
	dataObj, err := dataFs.NewObject(ctx, remote)
	if err != nil {
		return nil, formatNotFoundError(dataFs, "data particle", fmt.Sprintf("remote %q", remote), err)
	}
	dataReader, err := dataObj.Open(ctx)
	if err != nil {
		return nil, formatParticleError(dataFs, "data", "open failed", fmt.Sprintf("remote %q", remote), err)
	}
	dataData, err := io.ReadAll(dataReader)
	dataReader.Close()
	if err != nil {
		return nil, formatParticleError(dataFs, "data", "read failed", fmt.Sprintf("remote %q", remote), err)
	}

	// Reconstruct missing particle
	if targetType == "even" {
		// Reconstruct even from odd + parity
		reconstructed, err := ReconstructFromOddAndParity(dataData, parityData, isOddLength)
		if err != nil {
			return nil, formatOperationError("reconstruct even particle failed", fmt.Sprintf("remote %q", remote), err)
		}
		evenData, _ := SplitBytes(reconstructed)
		return evenData, nil
	} else {
		// Reconstruct odd from even + parity
		reconstructed, err := ReconstructFromEvenAndParity(dataData, parityData, isOddLength)
		if err != nil {
			return nil, formatOperationError("reconstruct odd particle failed", fmt.Sprintf("remote %q", remote), err)
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
// All particle existence checks are performed in parallel for better performance.
func (f *Fs) particleInfoForObject(ctx context.Context, remote string) (particleInfo, error) {
	pi := particleInfo{remote: remote}
	g, gCtx := errgroup.WithContext(ctx)

	// Use local variables to avoid race conditions
	var evenExists, oddExists, parityExists bool

	// Check even particle in parallel
	g.Go(func() error {
		if _, err := f.even.NewObject(gCtx, remote); err == nil {
			evenExists = true
		}
		return nil
	})

	// Check odd particle in parallel
	g.Go(func() error {
		if _, err := f.odd.NewObject(gCtx, remote); err == nil {
			oddExists = true
		}
		return nil
	})

	// Check parity particle (both suffixes) in parallel
	g.Go(func() error {
		// Try odd-length suffix first
		if _, errOL := f.parity.NewObject(gCtx, GetParityFilename(remote, true)); errOL == nil {
			parityExists = true
		} else {
			// Try even-length suffix
			if _, errEL := f.parity.NewObject(gCtx, GetParityFilename(remote, false)); errEL == nil {
				parityExists = true
			}
		}
		return nil
	})

	// Wait for all checks to complete
	if err := g.Wait(); err != nil {
		return pi, err
	}

	// Set results after all goroutines complete (no race condition)
	pi.evenExists = evenExists
	pi.oddExists = oddExists
	pi.parityExists = parityExists

	// Calculate count
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
// All backend List operations are performed in parallel for better performance
func (f *Fs) scanParticles(ctx context.Context, dir string) ([]particleInfo, error) {
	// Collect all entries from all backends in parallel (without filtering)
	var entriesEven, entriesOdd, entriesParity fs.DirEntries

	g, gCtx := errgroup.WithContext(ctx)

	// List even backend in parallel
	g.Go(func() error {
		entriesEven, _ = f.even.List(gCtx, dir)
		return nil // Ignore errors, same as original implementation
	})

	// List odd backend in parallel
	g.Go(func() error {
		entriesOdd, _ = f.odd.List(gCtx, dir)
		return nil // Ignore errors, same as original implementation
	})

	// List parity backend in parallel
	g.Go(func() error {
		entriesParity, _ = f.parity.List(gCtx, dir)
		return nil // Ignore errors, same as original implementation
	})

	// Wait for all List operations to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}

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
		parityPos:    0,
		dataEOF:      false,
		parityEOF:    false,
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
		return 0, formatOperationError("stream reconstruction failed", "failed to read data particle", dataErr)
	}

	parityN = parityRes.n
	parityErr = parityRes.err
	if parityRes.hitEOF {
		r.parityEOF = true
	}
	if parityErr != nil {
		return 0, formatOperationError("stream reconstruction failed", "failed to read parity particle", parityErr)
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
			return 0, formatOperationError("stream reconstruction failed", fmt.Sprintf("invalid sizes for reconstruction (data=%d parity=%d)", len(dataData), len(parityData)), nil)
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
		return 0, formatOperationError("stream reconstruction failed", fmt.Sprintf("invalid reconstruction mode: %s", r.mode), nil)
	}

	if reconErr != nil {
		return 0, formatOperationError("stream reconstruction failed", "", reconErr)
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
			errs = append(errs, formatOperationError("close failed", "failed to close data reader", err))
		}
	}
	if r.parityReader != nil {
		if err := r.parityReader.Close(); err != nil {
			errs = append(errs, formatOperationError("close failed", "failed to close parity reader", err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing readers: %v", errs)
	}
	return nil
}
