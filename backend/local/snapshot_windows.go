//go:build windows

package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mxk/go-vss"
	"github.com/rclone/rclone/fs"
)

// CreateSnapshot creates a point-in-time snapshot of a Fs,
// which may be used for copy operations.
//
// It returns the Fs snapshot, a cleanup function, and a possible error.
func (f *Fs) createSnapshot(ctx context.Context) (fs.Fs, func(ctx context.Context) error, error) {
	// Check if the snapshot already exists
	rootPath := f.root
	isSnapshot, err := vss.IsShadowCopy(rootPath)
	if err != nil {
		return nil, nil, fmt.Errorf("checking if path is a volume shadow copy during creation: %s (%w)", rootPath, err)
	} else if isSnapshot {
		return nil, nil, fmt.Errorf("path is already a volume shadow copy (skipping creation): %s", rootPath)
	}

	if _, err := os.Stat(rootPath); err != nil {
		return nil, nil, err
	}

	// Windows VSS only snapshots volumes at a time
	vol, rel, err := vss.SplitVolume(rootPath)
	fs.Infof(f, "Creating snapshot for volume %s with relative path %s", vol, rel)
	if err != nil {
		return nil, nil, err
	}

	// Create the snapshot itself.
	// Allow retries for "Another shadow copy operation is already in progress" (error code 9), but not for anything else.
	// See https://learn.microsoft.com/en-us/previous-versions/windows/desktop/vsswmi/create-method-in-class-win32-shadowcopy#return-value
	var id string
	p := fs.NewPacer(ctx, nil)
	err = p.Call(func() (bool, error) {
		var createErr error
		id, createErr = vss.Create(vol)

		// Return true if retryable (error code 9), false if not
		if createErr != nil {
			var e vss.CreateError
			if errors.As(createErr, &e) && e == 9 {
				return true, createErr // retry
			}
			return false, createErr // don't retry
		}
		return false, nil // success
	})
	if err != nil {
		return nil, nil, err
	}
	fs.Infof(f, "Created snapshot volume with ID %s", id)

	alreadyCleaned := false
	cleanup := func(ctx context.Context) error {
		if alreadyCleaned {
			return nil
		}
		fs.Infof(f, "Removing snapshot volume with ID %s", id)
		alreadyCleaned = true
		if err := vss.Remove(id); err != nil {
			fs.Errorf(f, "Failed to remove snapshot volume: %v", err)
			return err
		}
		return nil
	}

	// Create a new Fs object based on the snapshot
	sc, err := vss.Get(id)
	if err != nil {
		if err := cleanup(ctx); err != nil {
			fs.Errorf(f, "Error while cleaning up snapshot: %v", err)
		}
		return nil, nil, err
	}

	newPath := filepath.Join(sc.DeviceObject, rel)
	snapshotFs, err := fs.NewFs(ctx, newPath)
	if err != nil {
		if err := cleanup(ctx); err != nil {
			fs.Errorf(f, "Error while cleaning up snapshot: %v", err)
		}
		return nil, nil, err
	}

	// For extra safety with the remote control API, make sure this is cleaned up when the Fs shuts down
	snapshotFs.Features().Shutdown = cleanup

	return snapshotFs, cleanup, nil
}
