//go:build plan9 || js
// +build plan9 js

package local

import (
	"fmt"

	"github.com/rclone/rclone/fs"
)

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
