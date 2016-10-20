// +build linux darwin freebsd

package mount

import (
	"time"

	"github.com/ncw/rclone/fs"
)

// info to create a new object
type createInfo struct {
	f      fs.Fs
	remote string
}

func newCreateInfo(f fs.Fs, remote string) *createInfo {
	return &createInfo{
		f:      f,
		remote: remote,
	}
}

// Fs returns read only access to the Fs that this object is part of
func (ci *createInfo) Fs() fs.Info {
	return ci.f
}

// String returns the remote path
func (ci *createInfo) String() string {
	return ci.remote
}

// Remote returns the remote path
func (ci *createInfo) Remote() string {
	return ci.remote
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (ci *createInfo) Hash(fs.HashType) (string, error) {
	return "", fs.ErrHashUnsupported
}

// ModTime returns the modification date of the file
// It should return a best guess if one isn't available
func (ci *createInfo) ModTime() time.Time {
	return time.Now()
}

// Size returns the size of the file
func (ci *createInfo) Size() int64 {
	// FIXME this means this won't work with all remotes...
	return 0
}

// Storable says whether this object can be stored
func (ci *createInfo) Storable() bool {
	return true
}

var _ fs.ObjectInfo = (*createInfo)(nil)
