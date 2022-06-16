//go:build linux || freebsd
// +build linux freebsd

package mount

import (
	"testing"

	"github.com/rclone/rclone/vfs/vfstest"
)

func TestMount(t *testing.T) {
	vfstest.RunTests(t, false, mount)
}
