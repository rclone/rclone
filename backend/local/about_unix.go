// +build darwin dragonfly freebsd linux

package local

import (
	"syscall"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
)

// About gets quota information
func (f *Fs) About() (*fs.Usage, error) {
	var s syscall.Statfs_t
	err := syscall.Statfs(f.root, &s)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read disk usage")
	}
	bs := int64(s.Bsize) // nolint: unconvert
	usage := &fs.Usage{
		Total: fs.NewUsageValue(bs * int64(s.Blocks)),         // quota of bytes that can be used
		Used:  fs.NewUsageValue(bs * int64(s.Blocks-s.Bfree)), // bytes in use
		Free:  fs.NewUsageValue(bs * int64(s.Bavail)),         // bytes which can be uploaded before reaching the quota
	}
	return usage, nil
}

// check interface
var _ fs.Abouter = &Fs{}
