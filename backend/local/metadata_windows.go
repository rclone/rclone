//go:build windows
// +build windows

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
