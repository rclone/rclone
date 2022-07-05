//go:build linux
// +build linux

package local

import (
	"fmt"
	"time"

	"github.com/rclone/rclone/fs"
	"golang.org/x/sys/unix"
)

// Read the metadata from the file into metadata where possible
func (o *Object) readMetadataFromFile(m *fs.Metadata) (err error) {
	flags := unix.AT_SYMLINK_NOFOLLOW
	if o.fs.opt.FollowSymlinks {
		flags = 0
	}
	var stat unix.Statx_t
	err = unix.Statx(unix.AT_FDCWD, o.path, flags, (0 |
		unix.STATX_TYPE | // Want stx_mode & S_IFMT
		unix.STATX_MODE | // Want stx_mode & ~S_IFMT
		unix.STATX_UID | // Want stx_uid
		unix.STATX_GID | // Want stx_gid
		unix.STATX_ATIME | // Want stx_atime
		unix.STATX_MTIME | // Want stx_mtime
		unix.STATX_CTIME | // Want stx_ctime
		unix.STATX_BTIME), // Want stx_btime
		&stat)
	if err != nil {
		return err
	}
	m.Set("mode", fmt.Sprintf("%0o", stat.Mode))
	m.Set("uid", fmt.Sprintf("%d", stat.Uid))
	m.Set("gid", fmt.Sprintf("%d", stat.Gid))
	if stat.Rdev_major != 0 || stat.Rdev_minor != 0 {
		m.Set("rdev", fmt.Sprintf("%x", uint64(stat.Rdev_major)<<32|uint64(stat.Rdev_minor)))
	}
	setTime := func(key string, t unix.StatxTimestamp) {
		m.Set(key, time.Unix(t.Sec, int64(t.Nsec)).Format(metadataTimeFormat))
	}
	setTime("atime", stat.Atime)
	setTime("mtime", stat.Mtime)
	setTime("btime", stat.Btime)
	return nil
}
