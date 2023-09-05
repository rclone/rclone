//go:build aix || android || darwin || dragonfly || freebsd || ios || linux

package diskusage

import (
	"golang.org/x/sys/unix"
)

// New returns the disk status for dir.
//
// May return Unsupported error if it doesn't work on this platform.
func New(dir string) (info Info, err error) {
	var statfs unix.Statfs_t
	err = unix.Statfs(dir, &statfs)
	if err != nil {
		return info, err
	}
	// Note that these can be different sizes on different OSes so
	// we upcast them all to uint64
	//nolint:unconvert
	info.Free = uint64(statfs.Bfree) * uint64(statfs.Bsize)
	//nolint:unconvert
	info.Available = uint64(statfs.Bavail) * uint64(statfs.Bsize)
	//nolint:unconvert
	info.Total = uint64(statfs.Blocks) * uint64(statfs.Bsize)
	return info, nil
}
