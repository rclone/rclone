//go:build linux

package local

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/rclone/rclone/fs"
	"golang.org/x/sys/unix"
)

var (
	statxCheckOnce         sync.Once
	readMetadataFromFileFn func(o *Object, m *fs.Metadata) (err error)
)

// Read the time specified from the os.FileInfo
func readTime(t timeType, fi os.FileInfo) time.Time {
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		fs.Debugf(nil, "didn't return Stat_t as expected")
		return fi.ModTime()
	}
	switch t {
	case aTime:
		return time.Unix(stat.Atim.Unix())
	case cTime:
		return time.Unix(stat.Ctim.Unix())
	}
	return fi.ModTime()
}

// Read the metadata from the file into metadata where possible
func (o *Object) readMetadataFromFile(m *fs.Metadata) (err error) {
	statxCheckOnce.Do(func() {
		// Check statx() is available as it was only introduced in kernel 4.11
		// If not, fall back to fstatat() which was introduced in 2.6.16 which is guaranteed for all Go versions
		var stat unix.Statx_t
		if runtime.GOOS != "android" && unix.Statx(unix.AT_FDCWD, ".", 0, unix.STATX_ALL, &stat) != unix.ENOSYS {
			readMetadataFromFileFn = readMetadataFromFileStatx
		} else {
			readMetadataFromFileFn = readMetadataFromFileFstatat
		}
	})
	return readMetadataFromFileFn(o, m)
}

// Read the metadata from the file into metadata where possible
func readMetadataFromFileStatx(o *Object, m *fs.Metadata) (err error) {
	flags := unix.AT_SYMLINK_NOFOLLOW
	if o.fs.opt.FollowSymlinks {
		flags = 0
	}
	var stat unix.Statx_t
	//  statx() was added to Linux in kernel 4.11
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

// Read the metadata from the file into metadata where possible
func readMetadataFromFileFstatat(o *Object, m *fs.Metadata) (err error) {
	flags := unix.AT_SYMLINK_NOFOLLOW
	if o.fs.opt.FollowSymlinks {
		flags = 0
	}
	var stat unix.Stat_t
	// fstatat() was added to Linux in  kernel  2.6.16
	// Go only supports 2.6.32 or later
	err = unix.Fstatat(unix.AT_FDCWD, o.path, &stat, flags)
	if err != nil {
		return err
	}
	m.Set("mode", fmt.Sprintf("%0o", stat.Mode))
	m.Set("uid", fmt.Sprintf("%d", stat.Uid))
	m.Set("gid", fmt.Sprintf("%d", stat.Gid))
	if stat.Rdev != 0 {
		m.Set("rdev", fmt.Sprintf("%x", stat.Rdev))
	}
	setTime := func(key string, t unix.Timespec) {
		// The types of t.Sec and t.Nsec vary from int32 to int64 on
		// different Linux architectures so we need to cast them to
		// int64 here and hence need to quiet the linter about
		// unnecessary casts.
		//
		// nolint: unconvert
		m.Set(key, time.Unix(int64(t.Sec), int64(t.Nsec)).Format(metadataTimeFormat))
	}
	setTime("atime", stat.Atim)
	setTime("mtime", stat.Mtim)
	return nil
}
