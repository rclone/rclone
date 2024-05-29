// The pkg/xattr module doesn't compile for openbsd or plan9

//go:build openbsd || plan9

package local

import "github.com/rclone/rclone/fs"

const (
	xattrSupported = false
)

// getXattr returns the extended attributes for an object
func (o *Object) getXattr() (metadata fs.Metadata, err error) {
	return nil, nil
}

// setXattr sets the metadata on the file Xattrs
func (o *Object) setXattr(metadata fs.Metadata) (err error) {
	return nil
}
