// Package mockobject provides a mock object which can be created from a string
package mockobject

import (
	"errors"
	"io"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/hash"
)

var errNotImpl = errors.New("not implemented")

// Object is a mock fs.Object useful for testing
type Object string

// String returns a description of the Object
func (o Object) String() string {
	return string(o)
}

// Fs returns read only access to the Fs that this object is part of
func (o Object) Fs() fs.Info {
	return nil
}

// Remote returns the remote path
func (o Object) Remote() string {
	return string(o)
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o Object) Hash(hash.Type) (string, error) {
	return "", errNotImpl
}

// ModTime returns the modification date of the file
// It should return a best guess if one isn't available
func (o Object) ModTime() (t time.Time) {
	return t
}

// Size returns the size of the file
func (o Object) Size() int64 { return 0 }

// Storable says whether this object can be stored
func (o Object) Storable() bool {
	return true
}

// SetModTime sets the metadata on the object to set the modification date
func (o Object) SetModTime(time.Time) error {
	return errNotImpl
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o Object) Open(options ...fs.OpenOption) (io.ReadCloser, error) {
	return nil, errNotImpl
}

// Update in to the object with the modTime given of the given size
func (o Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return errNotImpl
}

// Remove this object
func (o Object) Remove() error {
	return errNotImpl
}
