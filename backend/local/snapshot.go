//go:build !windows

package local

import (
	"context"
	"fmt"

	"github.com/rclone/rclone/fs"
)

// CreateSnapshot creates a point-in-time snapshot of a Fs,
// which may be used for copy operations.
//
// Any required cleanup function should be saved within each
// backend's Fs struct and called in Shutdown().
//
// It returns the Fs snapshot and a possible error.
func (f *Fs) createSnapshot(_ context.Context) (fs.Fs, error) {
	return nil, fmt.Errorf("creating snapshots is not supported on this platform: %w", fs.ErrorNotImplemented)
}
