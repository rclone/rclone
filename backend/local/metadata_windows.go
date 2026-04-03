//go:build windows

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
	stat, ok := fi.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		fs.Debugf(nil, "didn't return Win32FileAttributeData as expected")
		return fi.ModTime()
	}
	switch t {
	case aTime:
		return time.Unix(0, stat.LastAccessTime.Nanoseconds())
	case bTime:
		return time.Unix(0, stat.CreationTime.Nanoseconds())
	}
	return fi.ModTime()
}

// Read the metadata from the file into metadata where possible
func (o *Object) readMetadataFromFile(m *fs.Metadata) (err error) {
	info, err := o.fs.lstat(o.path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		fs.Debugf(o, "didn't return Win32FileAttributeData as expected")
		return nil
	}
	// FIXME do something with stat.FileAttributes ?
	m.Set("mode", fmt.Sprintf("%0o", info.Mode()))
	setTime := func(key string, t syscall.Filetime) {
		m.Set(key, time.Unix(0, t.Nanoseconds()).Format(metadataTimeFormat))
	}
	setTime("atime", stat.LastAccessTime)
	setTime("mtime", stat.LastWriteTime)
	setTime("btime", stat.CreationTime)
	return nil
}
