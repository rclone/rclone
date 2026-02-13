// Package raid3 implements a backend that splits data across three remotes using byte-level striping
package raid3

// This file contains file operations for the raid3 backend.
//
// It includes:
//   - Put: Upload objects (streaming)
//   - Move: Move objects between locations
//   - Copy: Copy objects between locations
//   - DirMove: Move directories between locations

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"golang.org/x/sync/errgroup"
)

// Put uploads an object using the streaming path (single code path).
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.putStreaming(ctx, in, src, options...)
}

// effectiveModTime returns the ModTime to use for the footer: from options metadata (e.g. Copy --metadata-set mtime) if present, else src.ModTime.
func effectiveModTime(ctx context.Context, f *Fs, src fs.ObjectInfo, options []fs.OpenOption) time.Time {
	meta, err := fs.GetMetadataOptions(ctx, f, src, options)
	if err != nil || meta == nil {
		return src.ModTime(ctx)
	}
	if s, ok := meta["mtime"]; ok && s != "" {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t
		}
	}
	return src.ModTime(ctx)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// createParticleInfo creates a particleObjectInfo for a given particle type.
// All particles use the same remote path (unified naming with footer).
func createParticleInfo(f *Fs, src fs.ObjectInfo, particleType string, size int64, isOddLength bool) *particleObjectInfo {
	info := &particleObjectInfo{
		ObjectInfo: src,
		size:       size,
	}
	return info
}

// hashingReader wraps a reader and feeds bytes to MD5 and SHA-256 hashers while counting total bytes.
type hashingReader struct {
	r      io.Reader
	md5    hash.Hash
	sha256 hash.Hash
	n      int64
}

func newHashingReader(r io.Reader) *hashingReader {
	return &hashingReader{
		r:      r,
		md5:    md5.New(),
		sha256: sha256.New(),
	}
}

func (h *hashingReader) Read(p []byte) (int, error) {
	n, err := h.r.Read(p)
	if n > 0 {
		_, _ = h.md5.Write(p[:n])
		_, _ = h.sha256.Write(p[:n])
		h.n += int64(n)
	}
	return n, err
}

func (h *hashingReader) ContentLength() int64   { return h.n }
func (h *hashingReader) MD5Sum() [16]byte      { return [16]byte(h.md5.Sum(nil)) }
func (h *hashingReader) SHA256Sum() [32]byte   { return [32]byte(h.sha256.Sum(nil)) }

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
	mtime := effectiveModTime(ctx, f, src, options)
	compression, _ := ConfigToFooterCompression(f.opt.Compression) // validated in NewFs
	if srcSize == 0 {
		// Empty file - create empty particles (90-byte footer only each)
		ft := FooterFromReconstructed(0, nil, nil, mtime, compression, ShardEven)
		fb, _ := ft.MarshalBinary()
		evenR := bytes.NewReader(fb)
		ft2 := FooterFromReconstructed(0, nil, nil, mtime, compression, ShardOdd)
		fb2, _ := ft2.MarshalBinary()
		oddR := bytes.NewReader(fb2)
		ft3 := FooterFromReconstructed(0, nil, nil, mtime, compression, ShardParity)
		fb3, _ := ft3.MarshalBinary()
		parityR := bytes.NewReader(fb3)
		evenInfo := createParticleInfo(f, src, "even", FooterSize, false)
		evenObj, err := f.even.Put(ctx, evenR, evenInfo, options...)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to upload even particle: %w", f.even.Name(), err)
		}
		uploadedParticles = append(uploadedParticles, evenObj)

		oddInfo := createParticleInfo(f, src, "odd", FooterSize, false)
		oddObj, err := f.odd.Put(ctx, oddR, oddInfo, options...)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to upload odd particle: %w", f.odd.Name(), err)
		}
		uploadedParticles = append(uploadedParticles, oddObj)

		parityInfo := createParticleInfo(f, src, "parity", FooterSize, false)
		parityObj, err := f.parity.Put(ctx, parityR, parityInfo, options...)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to upload parity particle: %w", f.parity.Name(), err)
		}
		uploadedParticles = append(uploadedParticles, parityObj)

		return &Object{fs: f, remote: src.Remote()}, nil
	}

	// Single streaming path for all non-empty files (chunker-style: one path, no size-based buffering).
	// Producer goroutine + 8 MiB bufio decouples source from splitter so the full stream
	// is read even when particle pipes backpressure. Small objects are just short streams.

	// Determine if file is odd-length from source size (if available)
	// If size is unknown (-1), we'll determine it during streaming
	isOddLength := srcSize > 0 && srcSize%2 == 1

	hasher := newHashingReader(in)
	var splitInput io.Reader = io.Reader(hasher)
	if f.opt.Compression != "none" && f.opt.Compression != "" {
		cr, err := newCompressingReader(hasher, f.opt.Compression)
		if err != nil {
			return nil, fmt.Errorf("compression: %w", err)
		}
		splitInput = cr
	}

	// Decouple source from splitter with a producer goroutine and large buffer (see
	// backend/compress: pipe + bufio so the full stream is read even when particle
	// pipes backpressure). The producer reads from splitInput into a pipe; the
	// splitter reads from the pipe via an 8 MiB bufio. When the splitter blocks on
	// writing to the three particle pipes, it stops reading from the bufio; the bufio
	// can hold up to streamProducerBufferSize, so the source is fully drained.
	producerPipeR, producerPipeW := io.Pipe()
	go func() {
		defer func() {
			if r := recover(); r != nil {
				_ = producerPipeW.CloseWithError(fmt.Errorf("producer panic: %v", r))
			}
		}()
		_, copyErr := io.Copy(producerPipeW, splitInput)
		_ = producerPipeW.CloseWithError(copyErr)
	}()
	splitInput = bufio.NewReaderSize(producerPipeR, streamProducerBufferSize)

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

	// Use errgroup for the Put goroutines; run the splitter in the calling goroutine
	// so the source reader (e.g. accounting.Account from rclone copy) is consumed
	// on the same goroutine that called Put, avoiding deadlocks with pipe/async readers.
	g, gCtx := errgroup.WithContext(ctx)
	var uploadedMu sync.Mutex

	splitter := NewStreamSplitter(evenPipeW, oddPipeW, parityPipeW, streamReadChunkSize, isOddLengthCh)

	// Start Put goroutines first so they are reading from pipes when the splitter writes
	// Goroutine 1: Put even particle (reads from evenPipeR)
	g.Go(func() error {
		defer func() { _ = evenPipeR.Close() }()
		// Use unknown size (-1) since we're streaming
		evenInfo := createParticleInfo(f, src, "even", -1, isOddLength)
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

	// Goroutine 2: Put odd particle (reads from oddPipeR)
	g.Go(func() error {
		defer func() { _ = oddPipeR.Close() }()
		oddInfo := createParticleInfo(f, src, "odd", -1, isOddLength)
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

	// Goroutine 3: Put parity particle (reads from parityPipeR)
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

		parityInfo := createParticleInfo(f, src, "parity", -1, parityIsOddLength)
		obj, err := f.parity.Put(gCtx, parityPipeR, parityInfo, options...)
		if err != nil {
			return formatParticleError(f.parity, "parity", "upload failed", fmt.Sprintf("remote %q", src.Remote()), err)
		}
		uploadedMu.Lock()
		uploadedParticles = append(uploadedParticles, obj)
		uploadedMu.Unlock()
		return nil
	})

	// Run splitter in this goroutine so the source reader (e.g. Account from copy) is consumed here
	errSplit := splitter.Split(splitInput)
	if errSplit != nil {
		_ = evenPipeW.CloseWithError(errSplit)
		_ = oddPipeW.CloseWithError(errSplit)
		_ = parityPipeW.CloseWithError(errSplit)
		_ = g.Wait()
		return nil, errSplit
	}
	contentLength := hasher.ContentLength()
	// When source size is known, detect truncation (e.g. pipe/AsyncReader backpressure or early EOF)
	if srcSize >= 0 && contentLength != srcSize {
		_ = evenPipeW.CloseWithError(fmt.Errorf("stream truncated: read %d bytes, expected %d", contentLength, srcSize))
		_ = oddPipeW.CloseWithError(fmt.Errorf("stream truncated: read %d bytes, expected %d", contentLength, srcSize))
		_ = parityPipeW.CloseWithError(fmt.Errorf("stream truncated: read %d bytes, expected %d", contentLength, srcSize))
		_ = g.Wait()
		err = formatOperationError("upload failed", fmt.Sprintf("stream truncated: read %d bytes, expected %d (possible pipe/async reader backpressure)", contentLength, srcSize), nil)
		return nil, err
	}
	// Defensive: splitter must produce even/odd sizes that differ by 0 or 1 (RAID 3 layout)
	totalEvenPayload := splitter.GetTotalEvenWritten()
	totalOddPayload := splitter.GetTotalOddWritten()
	if !ValidateParticleSizes(totalEvenPayload, totalOddPayload) {
		_ = evenPipeW.CloseWithError(fmt.Errorf("internal: splitter produced invalid sizes even=%d odd=%d", totalEvenPayload, totalOddPayload))
		_ = oddPipeW.CloseWithError(fmt.Errorf("internal: splitter produced invalid sizes even=%d odd=%d", totalEvenPayload, totalOddPayload))
		_ = parityPipeW.CloseWithError(fmt.Errorf("internal: splitter produced invalid sizes even=%d odd=%d", totalEvenPayload, totalOddPayload))
		_ = g.Wait()
		err = formatOperationError("upload failed", fmt.Sprintf("internal: splitter produced invalid particle sizes even=%d, odd=%d (expected even=odd or even=odd+1)", totalEvenPayload, totalOddPayload), nil)
		return nil, err
	}
	md5Sum := hasher.MD5Sum()
	sha256Sum := hasher.SHA256Sum()
	streamMtime := effectiveModTime(gCtx, f, src, options)
	for shard := 0; shard < 3; shard++ {
		ft := FooterFromReconstructed(contentLength, md5Sum[:], sha256Sum[:], streamMtime, compression, shard)
		fb, errMarshal := ft.MarshalBinary()
		if errMarshal != nil {
			_ = evenPipeW.Close()
			_ = oddPipeW.Close()
			_ = parityPipeW.Close()
			_ = g.Wait()
			return nil, errMarshal
		}
		var w io.Writer
		switch shard {
		case ShardEven:
			w = evenPipeW
		case ShardOdd:
			w = oddPipeW
		case ShardParity:
			w = parityPipeW
		}
		if _, err := w.Write(fb); err != nil {
			_ = evenPipeW.Close()
			_ = oddPipeW.Close()
			_ = parityPipeW.Close()
			_ = g.Wait()
			return nil, err
		}
	}
	_ = evenPipeW.Close()
	_ = oddPipeW.Close()
	_ = parityPipeW.Close()

	// Wait for all Put goroutines to complete
	if err = g.Wait(); err != nil {
		return nil, err
	}

	// Get written sizes from splitter for verification (payload + footer)
	totalEvenWritten := splitter.GetTotalEvenWritten() + FooterSize
	totalOddWritten := splitter.GetTotalOddWritten() + FooterSize

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

	// Parity uses same path as logical object (unified naming)
	srcParityName := srcRemote
	dstParityName := remote

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

	newObj := &Object{
		fs:     f,
		remote: remote,
	}
	// Apply metadata set from config (e.g. --metadata-set mtime=...) so Move with metadata works
	if ci := fs.GetConfig(ctx); ci.Metadata && len(ci.MetadataSet) > 0 {
		if err := newObj.SetMetadata(ctx, ci.MetadataSet); err != nil {
			return nil, err
		}
	}
	return newObj, nil
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

	// Parity uses same path as logical object (unified naming)
	srcParityName := srcRemote
	dstParityName := remote

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

	newObj := &Object{
		fs:     f,
		remote: remote,
	}
	// Apply metadata set from config (e.g. --metadata-set mtime=...) so Copy with metadata works
	if ci := fs.GetConfig(ctx); ci.Metadata && len(ci.MetadataSet) > 0 {
		if err := newObj.SetMetadata(ctx, ci.MetadataSet); err != nil {
			return nil, err
		}
	}
	return newObj, nil
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
