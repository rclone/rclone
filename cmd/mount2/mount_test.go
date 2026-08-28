//go:build linux

package mount2

import (
	"testing"

	"github.com/PhateValleyman/rclone/vfs/vfscommon"
	"github.com/PhateValleyman/rclone/vfs/vfstest"
)

func TestMount(t *testing.T) {
	vfstest.RunTests(t, false, vfscommon.CacheModeOff, true, mount)
}
