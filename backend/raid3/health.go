// Package raid3 implements a backend that splits data across three remotes using byte-level striping
package raid3

// This file contains health check and degraded mode detection functions.
//
// It includes:
//   - checkAllBackendsAvailable: Health check for all three backends
//   - checkDirectoryExists: Check if directory exists on any backend
//   - formatDegradedModeError: User-friendly error messages with rebuild guidance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rclone/rclone/fs"
)

// checkAllBackendsAvailable performs a quick health check to see if all three
// backends are reachable. This is used to enforce strict write policy.
//
// Returns: enhanced error with rebuild guidance if any backend is unavailable
func (f *Fs) checkAllBackendsAvailable(ctx context.Context) error {
	// Quick timeout for health check
	checkCtx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	type healthResult struct {
		name string
		err  error
	}
	results := make(chan healthResult, 3)

	// Check each backend by attempting to access it
	// We check write capability since that's what we need for Put/Update/Move
	checkBackend := func(backend fs.Fs, name string) healthResult {
		// First, try to list (checks connectivity)
		_, listErr := backend.List(checkCtx, "")

		// Acceptable list errors (backend is available):
		//   - ErrorDirNotFound: Directory doesn't exist yet (empty backend)
		//   - ErrorIsFile: Path points to a file
		//   - InvalidBucketName: Configuration error (backend path misconfigured, not availability issue)
		//     This can happen with stale Fs cache or path parsing issues, but indicates config problem
		if listErr == nil || errors.Is(listErr, fs.ErrorDirNotFound) || errors.Is(listErr, fs.ErrorIsFile) {
			// Backend seems available, verify we can write
			// Try mkdir on a test path (won't actually create if Fs is at file level)
			testDir := ".raid3-health-check-" + name
			mkdirErr := backend.Mkdir(checkCtx, testDir)

			// Clean up test directory
			if mkdirErr == nil {
				_ = backend.Rmdir(checkCtx, testDir)
			}

			// Acceptable mkdir errors (backend is writable):
			//   - nil: Successfully created (backend is writable)
			//   - ErrorDirExists: Dir already exists (backend is writable)
			//   - os.IsExist: underlying filesystem reports existing dir/file
			if mkdirErr == nil || errors.Is(mkdirErr, fs.ErrorDirExists) || os.IsExist(mkdirErr) {
				return healthResult{name, nil} // Backend is available
			}

			// Mkdir failed with real error (permission, read-only filesystem, etc.)
			return healthResult{name, mkdirErr}
		}

		// List failed with real error
		// Context deadline exceeded: backend may be slow (e.g. MinIO under load), not necessarily down.
		if errors.Is(listErr, context.DeadlineExceeded) || strings.Contains(listErr.Error(), "deadline exceeded") {
			return healthResult{name, nil}
		}
		// MinIO "listPathRaw: 0 drives provided": list can fail on this path; backend may still accept writes.
		// Treat as available so copy can proceed (only degrade when direct write fails).
		if isMinIOListPathRawError(listErr) {
			return healthResult{name, nil}
		}
		// Check if it's an InvalidBucketName error (configuration issue, not availability)
		if strings.Contains(listErr.Error(), "InvalidBucketName") {
			// InvalidBucketName indicates a configuration/parsing issue, not backend unavailability
			// Return this as an error so it's reported, but it's different from connection errors
			return healthResult{name, fmt.Errorf("%s backend configuration error (InvalidBucketName): %w", name, listErr)}
		}
		// Other errors (connection refused, etc.)
		return healthResult{name, listErr}
	}

	go func() {
		results <- checkBackend(f.even, "even")
	}()
	go func() {
		results <- checkBackend(f.odd, "odd")
	}()
	go func() {
		results <- checkBackend(f.parity, "parity")
	}()

	// Collect results
	var failedBackend string
	var backendErr error
	evenOK := true
	oddOK := true
	parityOK := true

	for i := 0; i < 3; i++ {
		result := <-results
		if result.err != nil {
			failedBackend = result.name
			backendErr = result.err
			switch result.name {
			case "even":
				evenOK = false
			case "odd":
				oddOK = false
			case "parity":
				parityOK = false
			}
		}
	}

	// If any backend failed, return enhanced error with guidance
	if backendErr != nil {
		return f.formatDegradedModeError(failedBackend, evenOK, oddOK, parityOK, backendErr)
	}

	return nil
}

// checkDirectoryExists checks if a directory exists by calling List() on all backends.
// Returns true if directory exists on at least one backend, false if it doesn't exist on any backend.
// Returns an error if any backend returns a non-"not found" error.
func (f *Fs) checkDirectoryExists(ctx context.Context, dir string) (bool, error) {
	type listResult struct {
		exists bool
		err    error
	}
	results := make(chan listResult, 3)

	// Check each backend in parallel
	// For root directory (dir == ""), List("") returns ErrorDirNotFound if directory doesn't exist
	// For subdirectories, we need to check if the directory exists
	go func() {
		_, err := f.even.List(ctx, dir)
		// If List() returns no error, directory exists
		// If List() returns ErrorDirNotFound or os.ErrNotExist, directory doesn't exist
		exists := err == nil
		// If error is ErrorDirNotFound or os.ErrNotExist, directory doesn't exist (not an error)
		if err != nil && !errors.Is(err, fs.ErrorDirNotFound) && !os.IsNotExist(err) {
			results <- listResult{exists: false, err: fmt.Errorf("%s: %w", f.even.Name(), err)}
			return
		}
		// Preserve the error (even if it's ErrorDirNotFound) so we can detect it in collection
		results <- listResult{exists: exists, err: err}
	}()

	go func() {
		_, err := f.odd.List(ctx, dir)
		exists := err == nil
		if err != nil && !errors.Is(err, fs.ErrorDirNotFound) && !os.IsNotExist(err) {
			results <- listResult{exists: false, err: fmt.Errorf("%s: %w", f.odd.Name(), err)}
			return
		}
		results <- listResult{exists: exists, err: err}
	}()

	go func() {
		_, err := f.parity.List(ctx, dir)
		exists := err == nil
		if err != nil && !errors.Is(err, fs.ErrorDirNotFound) && !os.IsNotExist(err) {
			results <- listResult{exists: false, err: fmt.Errorf("%s: %w", f.parity.Name(), err)}
			return
		}
		results <- listResult{exists: exists, err: err}
	}()

	// Collect results
	var hasError error
	anyExists := false
	notFoundCount := 0
	for i := 0; i < 3; i++ {
		result := <-results
		if result.exists {
			// Directory exists on this backend
			anyExists = true
		} else if result.err != nil {
			// Check if this is a "not found" error
			if errors.Is(result.err, fs.ErrorDirNotFound) || os.IsNotExist(result.err) {
				notFoundCount++
			} else {
				// Non-"not found" error
				hasError = result.err
			}
		} else {
			// err is nil but exists is false - this shouldn't happen, treat as not found
			notFoundCount++
		}
	}

	// If any backend returned a non-"not found" error, return it
	if hasError != nil {
		return false, hasError
	}

	// If directory exists on any backend, it exists
	if anyExists {
		return true, nil
	}

	// If all backends returned "not found" errors, directory doesn't exist
	if notFoundCount == 3 {
		return false, nil
	}

	// Default: directory doesn't exist
	return false, nil
}

// formatDegradedModeError creates a user-friendly error message with rebuild guidance
// when the backend is in degraded mode (one or more backends unavailable).
//
// This implements Phase 1 of user-centric rebuild: guide users at point of failure.
func (f *Fs) formatDegradedModeError(failedBackend string, evenOK, oddOK, parityOK bool, backendErr error) error {
	// Status icons
	evenIcon := "✅"
	oddIcon := "✅"
	parityIcon := "✅"
	evenStatus := "Available"
	oddStatus := "Available"
	parityStatus := "Available"

	if !evenOK {
		evenIcon = "❌"
		evenStatus = "UNAVAILABLE"
	}
	if !oddOK {
		oddIcon = "❌"
		oddStatus = "UNAVAILABLE"
	}
	if !parityOK {
		parityIcon = "❌"
		parityStatus = "UNAVAILABLE"
	}

	// Build helpful error message with wrapped backend error
	return fmt.Errorf(`cannot write - raid3 backend is DEGRADED

Backend Status:
  %s even:   %s
  %s odd:    %s
  %s parity: %s

Impact:
  • Reads: ✅ Working (automatic parity reconstruction)
  • Writes: ❌ Blocked (RAID 3 safety - prevents corruption)

What to do:
  1. Check if %s backend is temporarily down:
     Run: rclone ls %s
     If it works, retry your operation
  
  2. If backend is permanently failed:
     Run: rclone backend status raid3:
     This will guide you through replacement and rebuild
  
  3. For more help:
     Documentation: rclone help raid3
     Error handling: See README.md

Technical details: %w`,
		evenIcon, evenStatus,
		oddIcon, oddStatus,
		parityIcon, parityStatus,
		failedBackend,
		f.getBackendPath(failedBackend),
		backendErr)
}
