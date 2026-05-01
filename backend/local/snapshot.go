//go:build !windows

package local

import (
	"context"
	"errors"

	"github.com/rclone/rclone/fs"
)

// CreateSnapshot creates a point-in-time snapshot of a Fs,
// which may be used for copy operations.
//
// It returns the Fs snapshot, a cleanup function, and a possible error.
func (f *Fs) createSnapshot(_ context.Context) (fs.Fs, func(ctx context.Context) error, error) {
	return nil, func(ctx context.Context) error {
		return nil
	}, errors.New("creating snapshots is not supported on this platform")
}
