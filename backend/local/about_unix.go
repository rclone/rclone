//go:build darwin || dragonfly || freebsd || linux

package local

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"github.com/rclone/rclone/fs"
)

// About gets quota information
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	var s syscall.Statfs_t
	err := syscall.Statfs(f.root, &s)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fs.ErrorDirNotFound
		}
		return nil, fmt.Errorf("failed to read disk usage: %w", err)
	}
	bs := int64(s.Bsize) // nolint: unconvert
	usage := &fs.Usage{
		Total: fs.NewUsageValue(bs * int64(s.Blocks)),         //nolint: unconvert // quota of bytes that can be used
		Used:  fs.NewUsageValue(bs * int64(s.Blocks-s.Bfree)), //nolint: unconvert // bytes in use
		Free:  fs.NewUsageValue(bs * int64(s.Bavail)),         //nolint: unconvert // bytes which can be uploaded before reaching the quota
	}
	return usage, nil
}

// check interface
var _ fs.Abouter = &Fs{}
