//go:build linux
// +build linux

package mount2

import (
	"testing"

	"github.com/rclone/rclone/vfs/vfstest"
)

func TestMount(t *testing.T) {
	vfstest.RunTests(t, false, mount)
}
