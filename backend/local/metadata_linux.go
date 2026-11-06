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
	mask := unix.STATX_TYPE | unix.STATX_MODE | unix.STATX_UID | unix.STATX_GID |
		unix.STATX_ATIME | unix.STATX_MTIME | unix.STATX_CTIME | unix.STATX_BTIME
	if o.fs.preservePOSIXProperties {
		mask = unix.STATX_ALL
	}
	//  statx() was added to Linux in kernel 4.11
	err = unix.Statx(unix.AT_FDCWD, o.path, flags, mask, &stat)
	if err != nil {
		if o.fs.preservePOSIXProperties && err == unix.ENOSYS {
			return readMetadataFromFileFstatat(o, m)
		}
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
	if o.fs.preservePOSIXProperties {
		m.Set("statx-mask", fmt.Sprintf("%d", stat.Mask))
		m.Set("attributes", fmt.Sprintf("%d", stat.Attributes&stat.Attributes_mask))
		m.Set("attributes-mask", fmt.Sprintf("%d", stat.Attributes_mask))
		m.Set("dev", fmt.Sprintf("%d", unix.Mkdev(stat.Dev_major, stat.Dev_minor)))
		if stat.Mask&unix.STATX_NLINK != 0 {
			m.Set("nlink", fmt.Sprintf("%d", stat.Nlink))
		}
		if stat.Mask&unix.STATX_INO != 0 {
			m.Set("ino", fmt.Sprintf("%d", stat.Ino))
		}
		setTime("ctime", stat.Ctime)
		for key, bit := range map[string]uint32{
			"mode": unix.STATX_MODE, "uid": unix.STATX_UID, "gid": unix.STATX_GID,
			"mtime": unix.STATX_MTIME, "ctime": unix.STATX_CTIME, "btime": unix.STATX_BTIME,
		} {
			if stat.Mask&bit == 0 {
				delete(*m, key)
			}
		}
		// Linux can omit STATX_ATIME on noatime mounts while still returning a valid atime.
		if stat.Mask&unix.STATX_ATIME == 0 && time.Unix(stat.Atime.Sec, int64(stat.Atime.Nsec)).UnixNano() <= 0 {
			delete(*m, "atime")
		}
		if stat.Mode&unix.S_IFMT == unix.S_IFCHR || stat.Mode&unix.S_IFMT == unix.S_IFBLK {
			m.Set("rdev", fmt.Sprintf("%x", unix.Mkdev(stat.Rdev_major, stat.Rdev_minor)))
		} else {
			delete(*m, "rdev")
		}
	}
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
	if o.fs.preservePOSIXProperties {
		m.Set("nlink", fmt.Sprintf("%d", stat.Nlink))
		m.Set("ino", fmt.Sprintf("%d", stat.Ino))
		m.Set("dev", fmt.Sprintf("%d", stat.Dev))
		setTime("ctime", stat.Ctim)
		if stat.Mode&unix.S_IFMT == unix.S_IFCHR || stat.Mode&unix.S_IFMT == unix.S_IFBLK {
			m.Set("rdev", fmt.Sprintf("%x", stat.Rdev))
		}
	}
	return nil
}
