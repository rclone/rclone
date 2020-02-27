// +build !linux !go1.13
// +build !darwin !go1.13
// +build !freebsd !go1.13
// +build !windows
// +build !race

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
