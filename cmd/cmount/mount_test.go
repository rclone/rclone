// +build cmount
// +build cgo
// +build linux darwin freebsd windows
// +build !race !windows

// FIXME this doesn't work with the race detector under Windows either
// hanging or producing lots of differences.

package cmount

import (
	"testing"

	"github.com/rclone/rclone/vfs/vfstest"
)

func TestMount(t *testing.T) {
	vfstest.RunTests(t, false, mount)
}
