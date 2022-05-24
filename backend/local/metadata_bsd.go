//go:build darwin || freebsd || netbsd
// +build darwin freebsd netbsd

package local

import (
	"fmt"
	"syscall"
	"time"

	"github.com/rclone/rclone/fs"
)

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
	setTime("atime", stat.Atimespec)
	setTime("mtime", stat.Mtimespec)
	setTime("btime", stat.Birthtimespec)
	return nil
}
