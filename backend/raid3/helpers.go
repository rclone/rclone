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
//   - moveState type for tracking move operations

import (
	"context"
	"fmt"
	"strings"
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

// disableRetriesForWrites disables retries for write operations to enforce strict RAID 3 policy
func (f *Fs) disableRetriesForWrites(ctx context.Context) context.Context {
	newCtx, ci := fs.AddConfig(ctx)
	ci.LowLevelRetries = 0 // Disable retries - fail fast
	fs.Debugf(f, "Disabled retries for write operation (strict RAID 3 policy)")
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

// rollbackUpdate restores original particles from temporary locations (best-effort cleanup)
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
