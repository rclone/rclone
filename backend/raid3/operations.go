// Package raid3 implements a backend that splits data across three remotes using byte-level striping
package raid3

// This file contains file operations for the raid3 backend.
//
// It includes:
//   - Put: Upload objects (buffered and streaming)
//   - Move: Move objects between locations
//   - Copy: Copy objects between locations
//   - DirMove: Move directories between locations

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/rclone/rclone/fs"
	"golang.org/x/sync/errgroup"
)

// Put uploads an object
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	if f.opt.UseStreaming {
		return f.putStreaming(ctx, in, src, options...)
	}
	return f.putBuffered(ctx, in, src, options...)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// putBuffered uploads an object using the buffered (memory-based) approach
func (f *Fs) putBuffered(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Input validation
	if err := validateContext(ctx, "put"); err != nil {
		return nil, err
	}
	if err := validateObjectInfo(src, "put"); err != nil {
		return nil, err
	}
	if err := validateRemote(src.Remote(), "put"); err != nil {
		return nil, err
	}
	if in == nil {
		return nil, formatOperationError("put failed", "input reader cannot be nil", nil)
	}

	// Pre-flight check: Enforce strict RAID 3 write policy
	// Fail immediately if any backend is unavailable to prevent degraded writes
	if err := f.checkAllBackendsAvailable(ctx); err != nil {
		return nil, formatOperationError("write blocked in degraded mode (RAID 3 policy)", "", err)
	}

	// Disable retries for strict RAID 3 write policy
	// This prevents rclone's retry logic from creating degraded files
	ctx = f.disableRetriesForWrites(ctx)

	// Read all data
	data, err := io.ReadAll(in)
	if err != nil {
		return nil, formatOperationError("put failed", "failed to read data", err)
	}

	// Split into even and odd bytes
	evenData, oddData := SplitBytes(data)

	// Calculate parity
	parityData := CalculateParity(evenData, oddData)

	// Determine if original data is odd length
	isOddLength := len(data)%2 == 1

	// Create wrapper ObjectInfo for each particle
	evenInfo := &particleObjectInfo{
		ObjectInfo: src,
		size:       int64(len(evenData)),
	}
	oddInfo := &particleObjectInfo{
		ObjectInfo: src,
		size:       int64(len(oddData)),
	}
	parityInfo := &particleObjectInfo{
		ObjectInfo: src,
		size:       int64(len(parityData)),
		remote:     GetParityFilename(src.Remote(), isOddLength),
	}

	// Track uploaded particles for rollback
	var uploadedParticles []fs.Object
	var uploadedMu sync.Mutex
	defer func() {
		if err != nil && f.opt.Rollback {
			uploadedMu.Lock()
			particles := uploadedParticles
			uploadedMu.Unlock()
			if rollbackErr := f.rollbackPut(ctx, particles); rollbackErr != nil {
				fs.Errorf(f, "Rollback failed during Put: %v", rollbackErr)
			}
		}
	}()

	g, gCtx := errgroup.WithContext(ctx)

	// Upload even bytes
	g.Go(func() error {
		reader := bytes.NewReader(evenData)
		obj, err := f.even.Put(gCtx, reader, evenInfo, options...)
		if err != nil {
			return formatParticleError(f.even, "even", "upload failed", fmt.Sprintf("remote %q", src.Remote()), err)
		}
		uploadedMu.Lock()
		uploadedParticles = append(uploadedParticles, obj)
		uploadedMu.Unlock()
		return nil
	})

	// Upload odd bytes
	g.Go(func() error {
		reader := bytes.NewReader(oddData)
		obj, err := f.odd.Put(gCtx, reader, oddInfo, options...)
		if err != nil {
			return formatParticleError(f.odd, "odd", "upload failed", fmt.Sprintf("remote %q", src.Remote()), err)
		}
		uploadedMu.Lock()
		uploadedParticles = append(uploadedParticles, obj)
		uploadedMu.Unlock()
		return nil
	})

	// Upload parity
	g.Go(func() error {
		reader := bytes.NewReader(parityData)
		obj, err := f.parity.Put(gCtx, reader, parityInfo, options...)
		if err != nil {
			return formatParticleError(f.parity, "parity", "upload failed", fmt.Sprintf("remote %q", GetParityFilename(src.Remote(), isOddLength)), err)
		}
		uploadedMu.Lock()
		uploadedParticles = append(uploadedParticles, obj)
		uploadedMu.Unlock()
		return nil
	})

	err = g.Wait()
	if err != nil {
		return nil, err
	}

	return &Object{
		fs:     f,
		remote: src.Remote(),
	}, nil
}

// createParticleInfo creates a particleObjectInfo for a given particle type
func createParticleInfo(src fs.ObjectInfo, particleType string, size int64, isOddLength bool) *particleObjectInfo {
	info := &particleObjectInfo{
		ObjectInfo: src,
		size:       size,
	}

	if particleType == "parity" {
		info.remote = GetParityFilename(src.Remote(), isOddLength)
	}

	return info
}

// verifyParticleSizes verifies that uploaded particles have the correct sizes
func verifyParticleSizes(ctx context.Context, f *Fs, evenObj, oddObj fs.Object, evenWritten, oddWritten int64) error {
	if evenObj == nil || oddObj == nil {
		return formatOperationError("verify particle sizes failed", fmt.Sprintf("cannot verify sizes: evenObj=%v, oddObj=%v", evenObj != nil, oddObj != nil), nil)
	}

	// Refresh objects from S3 to get actual committed sizes
	evenRefreshed, err := f.even.NewObject(ctx, evenObj.Remote())
	if err != nil {
		return formatBackendError(f.even, "refresh object failed", fmt.Sprintf("remote %q", evenObj.Remote()), err)
	}
	oddRefreshed, err := f.odd.NewObject(ctx, oddObj.Remote())
	if err != nil {
		return formatBackendError(f.odd, "refresh object failed", fmt.Sprintf("remote %q", oddObj.Remote()), err)
	}

	evenSize := evenRefreshed.Size()
	oddSize := oddRefreshed.Size()

	// Verify sizes match what we wrote
	if evenSize != evenWritten {
		return formatOperationError("verify particle sizes failed", fmt.Sprintf("even particle size mismatch: wrote %d bytes but object is %d bytes", evenWritten, evenSize), nil)
	}
	if oddSize != oddWritten {
		return formatOperationError("verify particle sizes failed", fmt.Sprintf("odd particle size mismatch: wrote %d bytes but object is %d bytes", oddWritten, oddSize), nil)
	}

	return nil
}

// putStreaming uploads an object using the streaming approach with io.Pipe
// This mirrors the Get/Open pattern: streams data to Put() calls instead of calling Put() multiple times
func (f *Fs) putStreaming(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Pre-flight check: Enforce strict RAID 3 write policy
	if err := f.checkAllBackendsAvailable(ctx); err != nil {
		return nil, formatOperationError("write blocked in degraded mode (RAID 3 policy)", "", err)
	}

	// Disable retries for strict RAID 3 write policy
	ctx = f.disableRetriesForWrites(ctx)

	// Track uploaded particles for rollback
	var uploadedParticles []fs.Object
	var err error
	defer func() {
		if err != nil && f.opt.Rollback {
			if rollbackErr := f.rollbackPut(ctx, uploadedParticles); rollbackErr != nil {
				fs.Errorf(f, "Rollback failed during Put (streaming): %v", rollbackErr)
			}
		}
	}()

	// Handle empty file case
	srcSize := src.Size()
	if srcSize == 0 {
		// Empty file - create empty particles
		evenInfo := createParticleInfo(src, "even", 0, false)
		evenObj, err := f.even.Put(ctx, bytes.NewReader(nil), evenInfo, options...)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to upload even particle: %w", f.even.Name(), err)
		}
		uploadedParticles = append(uploadedParticles, evenObj)

		oddInfo := createParticleInfo(src, "odd", 0, false)
		oddObj, err := f.odd.Put(ctx, bytes.NewReader(nil), oddInfo, options...)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to upload odd particle: %w", f.odd.Name(), err)
		}
		uploadedParticles = append(uploadedParticles, oddObj)

		parityInfo := createParticleInfo(src, "parity", 0, false)
		parityObj, err := f.parity.Put(ctx, bytes.NewReader(nil), parityInfo, options...)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to upload parity particle: %w", f.parity.Name(), err)
		}
		uploadedParticles = append(uploadedParticles, parityObj)

		return &Object{fs: f, remote: src.Remote()}, nil
	}

	// Configuration: Read 2MB chunks (produces ~1MB per particle)
	readChunkSize := int64(f.opt.ChunkSize) * 2
	if readChunkSize < minReadChunkSize {
		readChunkSize = minReadChunkSize
	}

	// Determine if file is odd-length from source size (if available)
	// If size is unknown (-1), we'll determine it during streaming
	isOddLength := srcSize > 0 && srcSize%2 == 1

	// Create pipes for streaming even, odd, and parity data
	evenPipeR, evenPipeW := io.Pipe()
	oddPipeR, oddPipeW := io.Pipe()
	parityPipeR, parityPipeW := io.Pipe()

	// Channel to communicate isOddLength from splitter to parity uploader
	// Only needed if size is unknown (-1)
	var isOddLengthCh chan bool
	if srcSize < 0 {
		isOddLengthCh = make(chan bool, 1)
		isOddLengthCh <- false // Default to even-length
	}

	var evenObj, oddObj fs.Object

	// Use errgroup to coordinate input reading/splitting and Put operations
	g, gCtx := errgroup.WithContext(ctx)
	var uploadedMu sync.Mutex

	// Goroutine 1: Read input, split into even/odd/parity, write to pipes
	splitter := NewStreamSplitter(evenPipeW, oddPipeW, parityPipeW, int(readChunkSize), isOddLengthCh)
	g.Go(func() error {
		defer func() { _ = evenPipeW.Close() }()
		defer func() { _ = oddPipeW.Close() }()
		defer func() { _ = parityPipeW.Close() }()
		return splitter.Split(in)
	})

	// Goroutine 2: Put even particle (reads from evenPipeR)
	g.Go(func() error {
		defer func() { _ = evenPipeR.Close() }()
		// Use unknown size (-1) since we're streaming
		evenInfo := createParticleInfo(src, "even", -1, isOddLength)
		obj, err := f.even.Put(gCtx, evenPipeR, evenInfo, options...)
		if err != nil {
			return formatParticleError(f.even, "even", "upload failed", fmt.Sprintf("remote %q", src.Remote()), err)
		}
		uploadedMu.Lock()
		evenObj = obj
		uploadedParticles = append(uploadedParticles, obj)
		uploadedMu.Unlock()
		return nil
	})

	// Goroutine 3: Put odd particle (reads from oddPipeR)
	g.Go(func() error {
		defer func() { _ = oddPipeR.Close() }()
		oddInfo := createParticleInfo(src, "odd", -1, isOddLength)
		obj, err := f.odd.Put(gCtx, oddPipeR, oddInfo, options...)
		if err != nil {
			return formatParticleError(f.odd, "odd", "upload failed", fmt.Sprintf("remote %q", src.Remote()), err)
		}
		uploadedMu.Lock()
		oddObj = obj
		uploadedParticles = append(uploadedParticles, obj)
		uploadedMu.Unlock()
		return nil
	})

	// Goroutine 4: Put parity particle (reads from parityPipeR)
	g.Go(func() error {
		defer func() { _ = parityPipeR.Close() }()

		// Get isOddLength - use source size if known, otherwise from channel
		parityIsOddLength := isOddLength
		if srcSize < 0 && isOddLengthCh != nil {
			// Try to get from channel (non-blocking, use latest value)
			select {
			case parityIsOddLength = <-isOddLengthCh:
			default:
				// Use default (even-length)
			}
		}

		// Create parity info with correct filename
		parityInfo := createParticleInfo(src, "parity", -1, parityIsOddLength)
		obj, err := f.parity.Put(gCtx, parityPipeR, parityInfo, options...)
		if err != nil {
			return formatParticleError(f.parity, "parity", "upload failed", fmt.Sprintf("remote %q", GetParityFilename(src.Remote(), isOddLength)), err)
		}
		uploadedMu.Lock()
		uploadedParticles = append(uploadedParticles, obj)
		uploadedMu.Unlock()
		return nil
	})

	// Wait for all goroutines to complete
	if err = g.Wait(); err != nil {
		return nil, err
	}

	// Get written sizes from splitter for verification
	totalEvenWritten := splitter.GetTotalEvenWritten()
	totalOddWritten := splitter.GetTotalOddWritten()

	// Verify sizes
	if err := verifyParticleSizes(ctx, f, evenObj, oddObj, totalEvenWritten, totalOddWritten); err != nil {
		return nil, err
	}

	return &Object{
		fs:     f,
		remote: src.Remote(),
	}, nil
}

// Move moves an object to a new remote location
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	// Check if src is from a raid3 backend (may be different Fs instance for cross-remote moves)
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}

	// Normalize remote path: remove leading slashes and clean the path.
	// The remote parameter should be relative to f.Root(), but it might include
	// path components if extracted from a full path. We normalize it to ensure
	// consistent behavior regardless of how it was constructed.
	remote = strings.TrimPrefix(remote, "/")
	remote = path.Clean(remote)

	// Handle cross-remote moves: if source is from different Fs instance,
	// we need to get particles from source Fs's backends
	srcFs := srcObj.fs
	srcRemote := srcObj.remote
	isCrossRemote := srcFs != f

	// Early return for move over self (no-op) - POSIX convention
	// Only check this if source and destination are the same Fs instance
	// (moving within the same remote, not between different remotes)
	if srcRemote == remote && !isCrossRemote {
		// Move over self is a no-op - return source object unchanged
		return srcObj, nil
	}

	// Determine source parity name (needed for cleanup)
	// For cross-remote moves, check source Fs's parity backend
	var srcParityName string
	parityOddSrc := GetParityFilename(srcRemote, true)
	parityEvenSrc := GetParityFilename(srcRemote, false)

	// Check in source Fs's parity backend for cross-remote moves
	parityFs := f.parity
	if isCrossRemote {
		parityFs = srcFs.parity
	}
	_, errOdd := parityFs.NewObject(ctx, parityOddSrc)
	if errOdd == nil {
		srcParityName = parityOddSrc
	} else {
		srcParityName = parityEvenSrc
	}
	_, isParity, isOddLength := StripParitySuffix(srcParityName)
	if !isParity {
		isOddLength = false
	}
	dstParityName := GetParityFilename(remote, isOddLength)

	// Pre-flight check: Enforce strict RAID 3 write policy
	// Fail immediately if any backend is unavailable to prevent degraded moves
	if err := f.checkAllBackendsAvailable(ctx); err != nil {
		// Even though we're failing early, clean up any destination files from previous attempts
		if f.opt.Rollback {
			allDestinations := []moveState{
				{"even", srcObj.remote, remote},
				{"odd", srcObj.remote, remote},
				{"parity", srcParityName, dstParityName},
			}
			if cleanupErr := f.rollbackMoves(ctx, allDestinations); cleanupErr != nil {
				fs.Debugf(f, "Cleanup of destination files failed: %v", cleanupErr)
			}
		}
		return nil, formatOperationError("move blocked in degraded mode (RAID 3 policy)", "", err)
	}
	fs.Debugf(f, "Move: all backends available, proceeding with move")

	// Disable retries for strict RAID 3 write policy
	ctx = f.disableRetriesForWrites(ctx)

	// Track successful moves for rollback
	var successMoves []moveState
	var movesMu sync.Mutex
	var moveErr error
	defer func() {
		if moveErr != nil && f.opt.Rollback {
			movesMu.Lock()
			moves := successMoves
			movesMu.Unlock()

			// If we have tracked moves, roll them back
			if len(moves) > 0 {
				if rollbackErr := f.rollbackMoves(ctx, moves); rollbackErr != nil {
					fs.Errorf(f, "Rollback failed during Move: %v", rollbackErr)
				}
			}

			// Also check for destination files that might exist even if not tracked
			// This handles edge cases where Copy succeeded but Delete failed
			allDestinations := []moveState{
				{"even", srcObj.remote, remote},
				{"odd", srcObj.remote, remote},
				{"parity", srcParityName, dstParityName},
			}
			if cleanupErr := f.rollbackMoves(ctx, allDestinations); cleanupErr != nil {
				fs.Debugf(f, "Cleanup check for untracked destination files failed: %v", cleanupErr)
			}
		}
	}()

	// Get source backends (may be different for cross-remote moves)
	srcEven, srcOdd, srcParity := f.getSourceBackends(srcFs, isCrossRemote)

	// Perform move operations on all three backends
	successMoves, moveErr = f.performMoves(ctx, srcEven, srcOdd, srcParity, srcRemote, remote, srcParityName, dstParityName)

	// If any failed, rollback will happen in defer
	if moveErr != nil {
		return nil, moveErr
	}

	return &Object{
		fs:     f,
		remote: remote,
	}, nil
}

// Copy src to this remote using server-side copy operations if possible
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	// Input validation
	if err := validateContext(ctx, "copy"); err != nil {
		return nil, err
	}
	if src == nil {
		return nil, formatOperationError("copy failed", "source object cannot be nil", nil)
	}
	if err := validateRemote(remote, "copy"); err != nil {
		return nil, err
	}

	// Check if src is from a raid3 backend (may be different Fs instance for cross-remote copies)
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	if err := validateRemote(srcObj.remote, "copy"); err != nil {
		return nil, err
	}

	// Normalize remote path: remove leading slashes and clean the path.
	// The remote parameter should be relative to f.Root(), but it might include
	// path components if extracted from a full path. We normalize it to ensure
	// consistent behavior regardless of how it was constructed.
	remote = strings.TrimPrefix(remote, "/")
	remote = path.Clean(remote)

	// Handle cross-remote copies: if source is from different Fs instance,
	// we need to get particles from source Fs's backends
	srcFs := srcObj.fs
	srcRemote := srcObj.remote
	isCrossRemote := srcFs != f

	// Determine source parity name (needed for destination parity name)
	// For cross-remote copies, check source Fs's parity backend
	var srcParityName string
	parityOddSrc := GetParityFilename(srcRemote, true)
	parityEvenSrc := GetParityFilename(srcRemote, false)

	// Check in source Fs's parity backend for cross-remote copies
	parityFs := f.parity
	if isCrossRemote {
		parityFs = srcFs.parity
	}
	_, errOdd := parityFs.NewObject(ctx, parityOddSrc)
	if errOdd == nil {
		srcParityName = parityOddSrc
	} else {
		srcParityName = parityEvenSrc
	}
	_, isParity, isOddLength := StripParitySuffix(srcParityName)
	if !isParity {
		isOddLength = false
	}
	dstParityName := GetParityFilename(remote, isOddLength)

	// Pre-flight check: Enforce strict RAID 3 write policy
	// Fail immediately if any backend is unavailable to prevent degraded copies
	if err := f.checkAllBackendsAvailable(ctx); err != nil {
		return nil, formatOperationError("copy blocked in degraded mode (RAID 3 policy)", "", err)
	}
	fs.Debugf(f, "Copy: all backends available, proceeding with copy")

	// Disable retries for strict RAID 3 write policy
	ctx = f.disableRetriesForWrites(ctx)

	// Track successful copies for cleanup on error
	type copyResult struct {
		backend string
		success bool
		err     error
	}

	var successCopies []string
	var copiesMu sync.Mutex
	var copyErr error
	defer func() {
		if copyErr != nil && f.opt.Rollback {
			copiesMu.Lock()
			copies := successCopies
			copiesMu.Unlock()

			// Clean up any successfully copied destination files
			if len(copies) > 0 {
				g, gCtx := errgroup.WithContext(ctx)
				for _, backendName := range copies {
					backendName := backendName
					g.Go(func() error {
						var backend fs.Fs
						var dstRemote string
						switch backendName {
						case "even":
							backend = f.even
							dstRemote = remote
						case "odd":
							backend = f.odd
							dstRemote = remote
						case "parity":
							backend = f.parity
							dstRemote = dstParityName
						default:
							return nil
						}
						dstObj, err := backend.NewObject(gCtx, dstRemote)
						if err != nil {
							return nil // Already cleaned up or doesn't exist
						}
						if delErr := dstObj.Remove(gCtx); delErr != nil {
							fs.Errorf(f, "Failed to clean up copied %s particle: %v", backendName, delErr)
						}
						return nil
					})
				}
				_ = g.Wait() // Best effort cleanup
			}
		}
	}()

	results := make(chan copyResult, 3)
	g, gCtx := errgroup.WithContext(ctx)

	// Get source backends (may be different for cross-remote copies)
	srcEven := f.even
	srcOdd := f.odd
	srcParity := f.parity
	if isCrossRemote {
		srcEven = srcFs.even
		srcOdd = srcFs.odd
		srcParity = srcFs.parity
	}

	// Copy on even
	g.Go(func() error {
		obj, err := srcEven.NewObject(gCtx, srcRemote)
		if err != nil {
			results <- copyResult{"even", false, nil}
			return nil // Ignore if not found
		}
		_, err = copyParticle(gCtx, f.even, obj, remote)
		if err != nil {
			results <- copyResult{"even", false, err}
			return err
		}
		results <- copyResult{"even", true, nil}
		return nil
	})

	// Copy on odd
	g.Go(func() error {
		obj, err := srcOdd.NewObject(gCtx, srcRemote)
		if err != nil {
			results <- copyResult{"odd", false, nil}
			return nil // Ignore if not found
		}
		_, err = copyParticle(gCtx, f.odd, obj, remote)
		if err != nil {
			results <- copyResult{"odd", false, err}
			return err
		}
		results <- copyResult{"odd", true, nil}
		return nil
	})

	// Copy parity
	g.Go(func() error {
		obj, err := srcParity.NewObject(gCtx, srcParityName)
		if err != nil {
			results <- copyResult{"parity", false, nil}
			return nil // Ignore if not found
		}
		_, err = copyParticle(gCtx, f.parity, obj, dstParityName)
		if err != nil {
			results <- copyResult{"parity", false, err}
			return err
		}
		results <- copyResult{"parity", true, nil}
		return nil
	})

	copyErr = g.Wait()
	close(results)

	// Collect results
	var firstError error
	for result := range results {
		if result.success {
			copiesMu.Lock()
			successCopies = append(successCopies, result.backend)
			copiesMu.Unlock()
		} else if result.err != nil && firstError == nil {
			firstError = result.err
		}
	}

	// If any failed, cleanup will happen in defer
	if firstError != nil || copyErr != nil {
		if firstError != nil {
			copyErr = firstError
		}
		return nil, copyErr
	}

	return &Object{
		fs:     f,
		remote: remote,
	}, nil
}

// DirMove moves src:srcRemote to this remote at dstRemote
// using server-side move operations
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	// Pre-flight check: Enforce strict RAID 3 write policy
	// Fail immediately if any backend is unavailable to prevent incomplete moves
	if err := f.checkAllBackendsAvailable(ctx); err != nil {
		return formatOperationError("dirmove blocked in degraded mode (RAID 3 policy)", "", err)
	}

	// Check if src is a raid3 backend
	srcFs, ok := src.(*Fs)
	if !ok {
		return fs.ErrorCantDirMove
	}

	// Check if source and destination are the same backend
	if f.name != srcFs.name {
		return fs.ErrorCantDirMove
	}

	// Prepare destination: if the target directory already exists but is empty (common
	// when the test harness pre-creates it), remove it so the backend DirMove can succeed.
	// If it exists with contents we must fail with fs.ErrorDirExists.
	checkAndCleanupDestination := func(remote string) error {
		entries, err := f.List(ctx, remote)
		switch {
		case err == nil:
			if len(entries) > 0 {
				return fs.ErrorDirExists
			}
			if rmErr := f.Rmdir(ctx, remote); rmErr != nil && !errors.Is(rmErr, fs.ErrorDirNotFound) {
				return formatOperationError("dirmove failed", fmt.Sprintf("failed to remove empty destination %q", remote), rmErr)
			}
		case errors.Is(err, fs.ErrorDirNotFound):
			// Nothing to do - destination does not exist
		default:
			return err
		}
		return nil
	}

	if dstRemote == "" {
		if err := checkAndCleanupDestination(""); err != nil {
			return err
		}
	} else {
		// Ensure intermediate parent directories exist so per-backend DirMove
		// gets consistent expectations.
		parent := path.Dir(dstRemote)
		if parent != "." && parent != "/" {
			if mkErr := f.Mkdir(ctx, parent); mkErr != nil && !errors.Is(mkErr, fs.ErrorDirExists) {
				return formatOperationError("dirmove failed", fmt.Sprintf("failed to prepare destination parent %q", parent), mkErr)
			}
		}
		if err := checkAndCleanupDestination(dstRemote); err != nil {
			return err
		}
	}

	// Disable retries for strict RAID 3 write policy
	ctx = f.disableRetriesForWrites(ctx)

	g, gCtx := errgroup.WithContext(ctx)

	// Move on even - use best-effort approach for degraded directories
	g.Go(func() error {
		if do := f.even.Features().DirMove; do != nil {
			err := do(gCtx, srcFs.even, srcRemote, dstRemote)
			if err != nil {
				// If source doesn't exist on this backend, create destination instead (reconstruction)
				// Only if auto_heal is enabled
				if (os.IsNotExist(err) || errors.Is(err, fs.ErrorDirNotFound)) && f.opt.AutoHeal {
					fs.Infof(f, "DirMove: source missing on even, creating destination (reconstruction)")
					return f.even.Mkdir(gCtx, dstRemote)
				}
				if errors.Is(err, fs.ErrorDirExists) {
					return fs.ErrorDirExists
				}
				return formatBackendError(f.even, "dirmove failed", fmt.Sprintf("src=%q dst=%q", srcRemote, dstRemote), err)
			}
			return nil
		}
		return fs.ErrorCantDirMove
	})

	// Move on odd
	g.Go(func() error {
		if do := f.odd.Features().DirMove; do != nil {
			err := do(gCtx, srcFs.odd, srcRemote, dstRemote)
			if err != nil {
				// If source doesn't exist on this backend, create destination instead (reconstruction)
				// Only if auto_heal is enabled
				if (os.IsNotExist(err) || errors.Is(err, fs.ErrorDirNotFound)) && f.opt.AutoHeal {
					fs.Infof(f, "DirMove: source missing on odd, creating destination (reconstruction)")
					return f.odd.Mkdir(gCtx, dstRemote)
				}
				if errors.Is(err, fs.ErrorDirExists) {
					return fs.ErrorDirExists
				}
				return formatBackendError(f.odd, "dirmove failed", fmt.Sprintf("src=%q dst=%q", srcRemote, dstRemote), err)
			}
			return nil
		}
		return fs.ErrorCantDirMove
	})

	// Move on parity
	g.Go(func() error {
		if do := f.parity.Features().DirMove; do != nil {
			err := do(gCtx, srcFs.parity, srcRemote, dstRemote)
			if err != nil {
				// If source doesn't exist on this backend, create destination instead (reconstruction)
				// Only if auto_heal is enabled
				if (os.IsNotExist(err) || errors.Is(err, fs.ErrorDirNotFound)) && f.opt.AutoHeal {
					fs.Infof(f, "DirMove: source missing on parity, creating destination (reconstruction)")
					return f.parity.Mkdir(gCtx, dstRemote)
				}
				if errors.Is(err, fs.ErrorDirExists) {
					return fs.ErrorDirExists
				}
				return formatBackendError(f.parity, "dirmove failed", fmt.Sprintf("src=%q dst=%q", srcRemote, dstRemote), err)
			}
			return nil
		}
		return fs.ErrorCantDirMove
	})

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}
