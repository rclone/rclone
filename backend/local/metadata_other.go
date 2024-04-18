//go:build dragonfly || plan9 || js

package local

import (
	"fmt"
	"os"
	"time"

	"github.com/rclone/rclone/fs"
)

// Read the time specified from the os.FileInfo
func readTime(t timeType, fi os.FileInfo) time.Time {
	return fi.ModTime()
}

// Read the metadata from the file into metadata where possible
func (o *Object) readMetadataFromFile(m *fs.Metadata) (err error) {
	info, err := o.fs.lstat(o.path)
	if err != nil {
		return err
	}
	m.Set("mode", fmt.Sprintf("%0o", info.Mode()))
	m.Set("mtime", info.ModTime().Format(metadataTimeFormat))
	return nil
}
