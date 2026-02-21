// Package raid3 implements a backend that splits data across three remotes using byte-level striping
package raid3

// This file contains utility functions and helpers for the raid3 backend.
//
// It includes:
//   - Timeout mode configuration (applyTimeoutMode, disableRetriesForWrites)
//   - Rollback operations (rollbackPut, rollbackUpdate, rollbackMoves)
//   - Particle move/copy helpers (moveOrCopyParticle, moveOrCopyParticleToTemp, copyParticle)
//   - Directory listing helpers (listDirectories)
//   - Math utilities (minInt64, maxInt64)
//   - Error formatting helpers (formatBackendError, formatParticleError)
//   - Input validation helpers (validateRemote, validateChunkSize, validateContext)
//   - moveState type for tracking move operations

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"golang.org/x/sync/errgroup"
)

// applyTimeoutMode creates a context with timeout settings based on the configured mode
func applyTimeoutMode(ctx context.Context, mode string) context.Context {
	switch mode {
	case "standard", "":
		// Don't modify context - use global settings
		fs.Debugf(nil, "raid3: Using standard timeout mode (global settings)")
		return ctx

	case "balanced":
		newCtx, ci := fs.AddConfig(ctx)
		ci.LowLevelRetries = 3
		ci.ConnectTimeout = fs.Duration(15 * time.Second)
		ci.Timeout = fs.Duration(30 * time.Second)
		fs.Debugf(nil, "raid3: Using balanced timeout mode (retries=%d, contimeout=%v, timeout=%v)",
			ci.LowLevelRetries, ci.ConnectTimeout, ci.Timeout)
		return newCtx

	case "aggressive":
		newCtx, ci := fs.AddConfig(ctx)
		ci.LowLevelRetries = 1
		ci.ConnectTimeout = fs.Duration(5 * time.Second)
		ci.Timeout = fs.Duration(10 * time.Second)
		fs.Debugf(nil, "raid3: Using aggressive timeout mode (retries=%d, contimeout=%v, timeout=%v)",
			ci.LowLevelRetries, ci.ConnectTimeout, ci.Timeout)
		return newCtx

	default:
		fs.Errorf(nil, "raid3: Unknown timeout_mode %q, using standard", mode)
		return ctx
	}
}

// writeRetries is the number of low-level retries allowed for write operations.
// Kept low to enforce RAID 3 policy (fail when a backend is down) but high enough
// to retry transient MinIO/S3 errors (503 SlowDown, CreateBucket/upload init).
const writeRetries = 5

// ensureMinRetriesForBackends returns a context with LowLevelRetries at least writeRetries.
// Used when creating the underlying even/odd/parity Fs so their S3 client can retry
// transient errors (e.g. 503 SlowDown on CreateBucket) even when timeout_mode=aggressive.
func ensureMinRetriesForBackends(ctx context.Context) context.Context {
	ci := fs.GetConfig(ctx)
	if ci.LowLevelRetries >= writeRetries {
		return ctx
	}
	newCtx, newCI := fs.AddConfig(ctx)
	newCI.LowLevelRetries = writeRetries
	return newCtx
}

// disableRetriesForWrites limits retries for write operations to enforce strict RAID 3 policy.
// We allow writeRetries so transient errors (503 SlowDown, CreateMultipartUpload hang) are
// retried; a backend that stays failing will still cause the write to fail.
func (f *Fs) disableRetriesForWrites(ctx context.Context) context.Context {
	newCtx, ci := fs.AddConfig(ctx)
	ci.LowLevelRetries = writeRetries
	fs.Debugf(f, "Limited retries for write operation (LowLevelRetries=%d, strict RAID 3 policy)", writeRetries)
	return newCtx
}

// Helper to get min of three int64 values
func minInt64(a, b, c int64) int64 {
	min := a
	if b < min {
		min = b
	}
	if c < min {
		min = c
	}
	return min
}

// Helper to get max of three int64 values
func maxInt64(a, b, c int64) int64 {
	max := a
	if b > max {
		max = b
	}
	if c > max {
		max = c
	}
	return max
}

// listDirectories lists only directories (not objects) from a path
// This is a helper for recursive directory scanning
func (f *Fs) listDirectories(ctx context.Context, dir string) (fs.DirEntries, error) {
	// Temporarily disable auto_cleanup to see all entries
	origAutoCleanup := f.opt.AutoCleanup
	f.opt.AutoCleanup = false
	defer func() {
		f.opt.AutoCleanup = origAutoCleanup
	}()

	entries, err := f.List(ctx, dir)
	if err != nil {
		return nil, err
	}

	var dirs fs.DirEntries
	for _, entry := range entries {
		if _, ok := entry.(fs.Directory); ok {
			dirs = append(dirs, entry)
		}
	}

	return dirs, nil
}

// moveState tracks the state of a move operation for rollback purposes
type moveState struct {
	backend string
	srcName string
	dstName string
}

// rollbackPut removes all successfully uploaded particles (best-effort cleanup)
func (f *Fs) rollbackPut(ctx context.Context, uploadedParticles []fs.Object) error {
	for _, obj := range uploadedParticles {
		if err := obj.Remove(ctx); err != nil {
			fs.Errorf(f, "Failed to remove uploaded particle during rollback: %v", err)
			// Continue with cleanup - best effort
		}
	}
	return nil
}

// rollbackUpdate restores original particles from temporary locations (best-effort cleanup).
// Not used by updateStreaming, which updates in place and uses rollbackPut on failure.
func (f *Fs) rollbackUpdate(ctx context.Context, tempParticles map[string]fs.Object) error {
	g, _ := errgroup.WithContext(ctx)

	for backendName, tempObj := range tempParticles {
		backendName := backendName // Capture for goroutine
		tempObj := tempObj         // Capture for goroutine

		g.Go(func() error {
			tempRemote := tempObj.Remote()
			var originalRemote string

			// Extract original remote from temp remote name
			if strings.HasSuffix(tempRemote, ".tmp."+backendName) {
				originalRemote = tempRemote[:len(tempRemote)-len(".tmp."+backendName)]
			} else {
				fs.Errorf(f, "Unexpected temp remote format for %s: %s", backendName, tempRemote)
				return nil
			}

			var backend fs.Fs
			switch backendName {
			case "even":
				backend = f.even
			case "odd":
				backend = f.odd
			case "parity":
				backend = f.parity
			default:
				return nil
			}

			// Try to move back from temp location to original
			if do := backend.Features().Move; do != nil {
				_, err := do(ctx, tempObj, originalRemote)
				if err == nil {
					return nil // Successfully moved back
				}
				fs.Debugf(f, "Rollback move back failed for %s, trying to delete temp: %v", backendName, err)
			}

			// Fallback: Delete temp particle (best effort)
			if err := tempObj.Remove(ctx); err != nil {
				fs.Errorf(f, "Rollback cleanup failed for %s: could not remove temp particle: %v", backendName, err)
			}
			return nil
		})
	}

	_ = g.Wait() // Best effort - don't return errors
	return nil
}

// rollbackMoves attempts to move particles back from destination to source (best-effort cleanup)
func (f *Fs) rollbackMoves(ctx context.Context, moves []moveState) error {
	g, _ := errgroup.WithContext(ctx)

	for _, move := range moves {
		move := move // Capture for goroutine
		g.Go(func() error {
			var backend fs.Fs
			switch move.backend {
			case "even":
				backend = f.even
			case "odd":
				backend = f.odd
			case "parity":
				backend = f.parity
			default:
				return nil
			}

			// Try to move back
			dstObj, err := backend.NewObject(ctx, move.dstName)
			if err != nil {
				return nil // Already rolled back or doesn't exist
			}

			if do := backend.Features().Move; do != nil {
				_, err := do(ctx, dstObj, move.srcName)
				if err == nil {
					return nil // Successfully moved back
				}
				// Move back failed - try to delete from destination
				fs.Debugf(f, "Rollback move back failed for %s, deleting: %v", move.backend, err)
			}

			// Fallback: Delete from destination
			if err := dstObj.Remove(ctx); err != nil {
				fs.Errorf(f, "Rollback delete failed for %s: %v", move.backend, err)
			}
			return nil
		})
	}

	_ = g.Wait() // Best effort - don't return errors
	return nil
}

// moveOrCopyParticleToTemp moves or copies a particle to a temporary location using
// server-side Move if available, or Copy+Delete as fallback (like operations.Move).
// This allows the move-to-temp rollback pattern to work with backends like S3/MinIO
// that support Copy but not Move.
func moveOrCopyParticleToTemp(ctx context.Context, backend fs.Fs, obj fs.Object, tempRemote string) (fs.Object, error) {
	// Input validation
	if err := validateContext(ctx, "moveOrCopyParticleToTemp"); err != nil {
		return nil, err
	}
	if err := validateBackend(backend, "backend", "moveOrCopyParticleToTemp"); err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, formatOperationError("moveOrCopyParticleToTemp failed", "object cannot be nil", nil)
	}
	if err := validateRemote(tempRemote, "moveOrCopyParticleToTemp"); err != nil {
		return nil, err
	}

	// Try Move first if available
	if doMove := backend.Features().Move; doMove != nil {
		moved, err := doMove(ctx, obj, tempRemote)
		if err == nil {
			return moved, nil
		}
		// Move failed - fall back to Copy+Delete
		fs.Debugf(obj, "Move failed, falling back to Copy+Delete: %v", err)
	}

	// Fallback to Copy+Delete (like operations.Move does)
	if doCopy := backend.Features().Copy; doCopy != nil {
		// Copy to temp location
		copied, err := doCopy(ctx, obj, tempRemote)
		if err != nil {
			return nil, fmt.Errorf("failed to copy particle to temp: %w", err)
		}
		// Delete original (best effort - if this fails, we still have the copy)
		if delErr := obj.Remove(ctx); delErr != nil {
			fs.Errorf(obj, "Failed to delete original after copy to temp (non-fatal): %v", delErr)
		}
		return copied, nil
	}

	// Neither Move nor Copy is available
	return nil, fmt.Errorf("backend does not support Move or Copy")
}

// moveOrCopyParticle moves or copies a particle from source to destination using
// server-side Move if available, or Copy+Delete as fallback (consistent with union backend pattern).
// This allows Move operations to work with backends like S3/MinIO that support Copy but not Move.
func moveOrCopyParticle(ctx context.Context, backend fs.Fs, obj fs.Object, destRemote string) (fs.Object, error) {
	// Input validation
	if err := validateContext(ctx, "moveOrCopyParticle"); err != nil {
		return nil, err
	}
	if err := validateBackend(backend, "backend", "moveOrCopyParticle"); err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, formatOperationError("moveOrCopyParticle failed", "object cannot be nil", nil)
	}
	if err := validateRemote(destRemote, "moveOrCopyParticle"); err != nil {
		return nil, err
	}

	// Check if backend supports Move or Copy (consistent with union backend)
	backendFeatures := backend.Features()
	do := backendFeatures.Move
	if backendFeatures.Move == nil {
		do = backendFeatures.Copy
	}
	if do == nil {
		return nil, fmt.Errorf("backend does not support Move or Copy")
	}

	// Perform Move or Copy
	dstObj, err := do(ctx, obj, destRemote)
	if err != nil {
		return nil, fmt.Errorf("failed to move/copy particle: %w", err)
	}
	if dstObj == nil {
		return nil, fmt.Errorf("destination object not found after move/copy")
	}

	// Delete the source object if Copy was used (consistent with union backend pattern)
	if backendFeatures.Move == nil {
		if delErr := obj.Remove(ctx); delErr != nil {
			return nil, fmt.Errorf("failed to delete original after copy: %w", delErr)
		}
	}

	return dstObj, nil
}

// copyParticle copies a particle from source to destination using server-side Copy.
// Unlike moveOrCopyParticle, this does not delete the source object.
func copyParticle(ctx context.Context, backend fs.Fs, obj fs.Object, destRemote string) (fs.Object, error) {
	// Input validation
	if err := validateContext(ctx, "copyParticle"); err != nil {
		return nil, err
	}
	if err := validateBackend(backend, "backend", "copyParticle"); err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, formatOperationError("copyParticle failed", "object cannot be nil", nil)
	}
	if err := validateRemote(destRemote, "copyParticle"); err != nil {
		return nil, err
	}

	backendFeatures := backend.Features()
	if backendFeatures.Copy == nil {
		return nil, fmt.Errorf("backend does not support Copy")
	}

	// Perform Copy
	dstObj, err := backendFeatures.Copy(ctx, obj, destRemote)
	if err != nil {
		return nil, fmt.Errorf("failed to copy particle: %w", err)
	}
	if dstObj == nil {
		return nil, fmt.Errorf("destination object not found after copy")
	}

	return dstObj, nil
}

// Error formatting helpers for consistent error messages across the codebase

// formatBackendError formats an error with backend name prefix
// Format: "backend: operation failed: context: %w"
func formatBackendError(backend fs.Fs, operation, context string, err error) error {
	if context != "" {
		return fmt.Errorf("%s: %s: %s: %w", backend.Name(), operation, context, err)
	}
	return fmt.Errorf("%s: %s: %w", backend.Name(), operation, err)
}

// formatParticleError formats an error for particle operations
// Format: "backend: particle operation failed: context: %w"
func formatParticleError(backend fs.Fs, particleType, operation, context string, err error) error {
	if context != "" {
		return fmt.Errorf("%s: %s particle %s: %s: %w", backend.Name(), particleType, operation, context, err)
	}
	return fmt.Errorf("%s: %s particle %s: %w", backend.Name(), particleType, operation, err)
}

// isMinIOListPathRawError reports whether err is (or wraps) MinIO's "listPathRaw: 0 drives provided"
// ListObjectsV2 500 InternalError. This occurs when listing a non-existent prefix on MinIO;
// S3 semantics do not require listing the destination before write/copy. Treating this as
// "directory not found" allows copy/sync to proceed instead of failing the whole backend.
func isMinIOListPathRawError(err error) bool {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if strings.Contains(e.Error(), "listPathRaw: 0 drives provided") ||
			(strings.Contains(e.Error(), "InternalError") && strings.Contains(e.Error(), "0 drives")) {
			return true
		}
	}
	return false
}

// formatOperationError formats an error for general operations
// Format: "operation failed: context: %w"
func formatOperationError(operation, context string, err error) error {
	if context != "" {
		if err != nil {
			return fmt.Errorf("%s: %s: %w", operation, context, err)
		}
		return fmt.Errorf("%s: %s", operation, context)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return fmt.Errorf("%s", operation)
}

// formatNotFoundError formats a "not found" error
// Format: "backend: resource not found: context: %w"
func formatNotFoundError(backend fs.Fs, resource, context string, err error) error {
	if context != "" {
		return fmt.Errorf("%s: %s not found: %s: %w", backend.Name(), resource, context, err)
	}
	return fmt.Errorf("%s: %s not found: %w", backend.Name(), resource, err)
}

// Input validation helpers

// validateRemote validates that a remote path is not empty and properly formatted
func validateRemote(remote string, operation string) error {
	if remote == "" {
		return formatOperationError(operation+" failed", "remote path cannot be empty", nil)
	}
	// Check for invalid characters that could cause issues
	if strings.Contains(remote, "\x00") {
		return formatOperationError(operation+" failed", fmt.Sprintf("remote path contains null byte: %q", remote), nil)
	}
	return nil
}

// validateContext validates that a context is not nil
func validateContext(ctx context.Context, operation string) error {
	if ctx == nil {
		return formatOperationError(operation+" failed", "context cannot be nil", nil)
	}
	// Check if context is already cancelled
	if err := ctx.Err(); err != nil {
		return formatOperationError(operation+" failed", "context already cancelled", err)
	}
	return nil
}

// validateObjectInfo validates that ObjectInfo is not nil and has required fields
func validateObjectInfo(src fs.ObjectInfo, operation string) error {
	if src == nil {
		return formatOperationError(operation+" failed", "ObjectInfo cannot be nil", nil)
	}
	if src.Remote() == "" {
		return formatOperationError(operation+" failed", "ObjectInfo.Remote() cannot be empty", nil)
	}
	return nil
}

// validateBackend validates that a backend filesystem is not nil
func validateBackend(backend fs.Fs, backendName, operation string) error {
	if backend == nil {
		return formatOperationError(operation+" failed", fmt.Sprintf("%s backend cannot be nil", backendName), nil)
	}
	return nil
}

// getSourceBackends returns the appropriate source backends for cross-remote operations
func (f *Fs) getSourceBackends(srcFs *Fs, isCrossRemote bool) (srcEven, srcOdd, srcParity fs.Fs) {
	if isCrossRemote {
		return srcFs.even, srcFs.odd, srcFs.parity
	}
	return f.even, f.odd, f.parity
}

// moveResult tracks the result of a single particle move operation
type moveResult struct {
	state   moveState
	success bool
	err     error
}

// performMoves performs move operations on all three backends in parallel
func (f *Fs) performMoves(ctx context.Context, srcEven, srcOdd, srcParity fs.Fs, srcRemote, remote, srcParityName, dstParityName string) ([]moveState, error) {
	// Input validation
	if err := validateContext(ctx, "performMoves"); err != nil {
		return nil, err
	}
	if err := validateBackend(srcEven, "srcEven", "performMoves"); err != nil {
		return nil, err
	}
	if err := validateBackend(srcOdd, "srcOdd", "performMoves"); err != nil {
		return nil, err
	}
	if err := validateBackend(srcParity, "srcParity", "performMoves"); err != nil {
		return nil, err
	}
	if err := validateBackend(f.even, "even", "performMoves"); err != nil {
		return nil, err
	}
	if err := validateBackend(f.odd, "odd", "performMoves"); err != nil {
		return nil, err
	}
	if err := validateBackend(f.parity, "parity", "performMoves"); err != nil {
		return nil, err
	}
	type trackedMove struct {
		successMoves []moveState
		movesMu      sync.Mutex
	}

	tracked := &trackedMove{}
	results := make(chan moveResult, 3)
	g, gCtx := errgroup.WithContext(ctx)

	// Move on even
	g.Go(func() error {
		return f.moveParticle(gCtx, srcEven, f.even, srcRemote, remote, "even", results)
	})

	// Move on odd
	g.Go(func() error {
		return f.moveParticle(gCtx, srcOdd, f.odd, srcRemote, remote, "odd", results)
	})

	// Move parity
	g.Go(func() error {
		return f.moveParticle(gCtx, srcParity, f.parity, srcParityName, dstParityName, "parity", results)
	})

	moveErr := g.Wait()
	close(results)

	// Collect results
	var firstError error
	for result := range results {
		if result.success {
			tracked.movesMu.Lock()
			tracked.successMoves = append(tracked.successMoves, result.state)
			tracked.movesMu.Unlock()
		} else if result.err != nil && firstError == nil {
			firstError = result.err
		}
	}

	if firstError != nil {
		return tracked.successMoves, firstError
	}
	return tracked.successMoves, moveErr
}

// moveParticle performs a single particle move operation
func (f *Fs) moveParticle(ctx context.Context, srcBackend, dstBackend fs.Fs, srcRemote, dstRemote, particleType string, results chan<- moveResult) error {
	obj, err := srcBackend.NewObject(ctx, srcRemote)
	if err != nil {
		results <- moveResult{moveState{particleType, srcRemote, dstRemote}, false, nil}
		return nil // Ignore if not found
	}
	_, err = moveOrCopyParticle(ctx, dstBackend, obj, dstRemote)
	if err != nil {
		results <- moveResult{moveState{particleType, srcRemote, dstRemote}, false, err}
		return err
	}
	results <- moveResult{moveState{particleType, srcRemote, dstRemote}, true, nil}
	return nil
}
