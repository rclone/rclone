// Package raid3 implements a backend that splits data across three remotes using byte-level striping
package raid3

// This file contains metadata operations for the raid3 backend.
//
// It includes:
//   - MkdirMetadata: Create directories with metadata
//   - DirSetModTime: Set modification time for directories

import (
	"context"
	"fmt"
	"time"

	"github.com/rclone/rclone/fs"
	"golang.org/x/sync/errgroup"
)

func (f *Fs) MkdirMetadata(ctx context.Context, dir string, metadata fs.Metadata) (fs.Directory, error) {
	// Pre-flight health check: Enforce strict RAID 3 write policy
	// Consistent with Put/Update/Move/Mkdir operations
	if err := f.checkAllBackendsAvailable(ctx); err != nil {
		return nil, fmt.Errorf("mkdirmetadata blocked in degraded mode (RAID 3 policy): %w", err)
	}

	// Disable retries for strict RAID 3 write policy
	ctx = f.disableRetriesForWrites(ctx)

	// Create directory on all three backends that support MkdirMetadata
	// For backends that don't support it, fall back to regular Mkdir
	g, gCtx := errgroup.WithContext(ctx)
	dirs := make([]fs.Directory, 3)
	errs := make([]error, 3)

	g.Go(func() error {
		if do := f.even.Features().MkdirMetadata; do != nil {
			newDir, err := do(gCtx, dir, metadata)
			if err != nil {
				errs[0] = fmt.Errorf("%s: %w", f.even.Name(), err)
				return errs[0]
			}
			dirs[0] = newDir
		} else {
			// Fallback to regular Mkdir
			err := f.even.Mkdir(gCtx, dir)
			if err != nil {
				errs[0] = fmt.Errorf("%s: %w", f.even.Name(), err)
				return errs[0]
			}
		}
		return nil
	})

	g.Go(func() error {
		if do := f.odd.Features().MkdirMetadata; do != nil {
			newDir, err := do(gCtx, dir, metadata)
			if err != nil {
				errs[1] = fmt.Errorf("%s: %w", f.odd.Name(), err)
				return errs[1]
			}
			dirs[1] = newDir
		} else {
			// Fallback to regular Mkdir
			err := f.odd.Mkdir(gCtx, dir)
			if err != nil {
				errs[1] = fmt.Errorf("%s: %w", f.odd.Name(), err)
				return errs[1]
			}
		}
		return nil
	})

	g.Go(func() error {
		if do := f.parity.Features().MkdirMetadata; do != nil {
			newDir, err := do(gCtx, dir, metadata)
			if err != nil {
				errs[2] = fmt.Errorf("%s: %w", f.parity.Name(), err)
				return errs[2]
			}
			dirs[2] = newDir
		} else {
			// Fallback to regular Mkdir
			err := f.parity.Mkdir(gCtx, dir)
			if err != nil {
				errs[2] = fmt.Errorf("%s: %w", f.parity.Name(), err)
				return errs[2]
			}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Return a raid3 Directory
	return &Directory{
		fs:     f,
		remote: dir,
	}, nil
}
func (f *Fs) DirSetModTime(ctx context.Context, dir string, modTime time.Time) error {
	// Check if directory exists before health check (union backend pattern)
	// This ensures we return fs.ErrorDirNotFound immediately when directory doesn't exist
	dirExists, err := f.checkDirectoryExists(ctx, dir)
	if err != nil {
		return fmt.Errorf("failed to check directory existence: %w", err)
	}
	if !dirExists {
		return fs.ErrorDirNotFound
	}

	// Pre-flight check: Enforce strict RAID 3 write policy
	// DirSetModTime is a metadata write operation (modifies directory metadata)
	// Consistent with Object.SetModTime, Put, Update, Move, Mkdir operations
	if err := f.checkAllBackendsAvailable(ctx); err != nil {
		return fmt.Errorf("dirsetmodtime blocked in degraded mode (RAID 3 policy): %w", err)
	}

	// Disable retries for strict RAID 3 write policy
	ctx = f.disableRetriesForWrites(ctx)

	// Set modtime on all three backends that support it
	// Use errgroup to collect errors from all backends
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if do := f.even.Features().DirSetModTime; do != nil {
			err := do(gCtx, dir, modTime)
			if err != nil {
				return fmt.Errorf("%s: %w", f.even.Name(), err)
			}
		}
		return nil
	})

	g.Go(func() error {
		if do := f.odd.Features().DirSetModTime; do != nil {
			err := do(gCtx, dir, modTime)
			if err != nil {
				return fmt.Errorf("%s: %w", f.odd.Name(), err)
			}
		}
		return nil
	})

	g.Go(func() error {
		if do := f.parity.Features().DirSetModTime; do != nil {
			err := do(gCtx, dir, modTime)
			if err != nil {
				return fmt.Errorf("%s: %w", f.parity.Name(), err)
			}
		}
		return nil
	})

	return g.Wait()
}
