// +build !linux !go1.11
// +build !darwin !go1.11
// +build !freebsd !go1.11
// +build !windows

package cache_test

import (
	"testing"

	"github.com/rclone/rclone/fs"
)

func (r *run) mountFs(t *testing.T, f fs.Fs) {
	panic("mountFs not defined for this platform")
}

func (r *run) unmountFs(t *testing.T, f fs.Fs) {
	panic("unmountFs not defined for this platform")
}
