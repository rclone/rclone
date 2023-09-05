//go:build openbsd

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
	info.Free = uint64(statfs.F_bfree) * uint64(statfs.F_bsize)
	//nolint:unconvert
	info.Available = uint64(statfs.F_bavail) * uint64(statfs.F_bsize)
	//nolint:unconvert
	info.Total = uint64(statfs.F_blocks) * uint64(statfs.F_bsize)
	return info, nil
}
