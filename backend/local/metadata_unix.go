//go:build openbsd || solaris

package local

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/rclone/rclone/fs"
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
	info, err := o.fs.lstat(o.path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		fs.Debugf(o, "didn't return Stat_t as expected")
		return nil
	}
	m.Set("mode", fmt.Sprintf("%0o", stat.Mode))
	m.Set("uid", fmt.Sprintf("%d", stat.Uid))
	m.Set("gid", fmt.Sprintf("%d", stat.Gid))
	if stat.Rdev != 0 {
		m.Set("rdev", fmt.Sprintf("%x", stat.Rdev))
	}
	setTime := func(key string, t syscall.Timespec) {
		m.Set(key, time.Unix(t.Unix()).Format(metadataTimeFormat))
	}
	setTime("atime", stat.Atim)
	setTime("mtime", stat.Mtim)
	return nil
}
