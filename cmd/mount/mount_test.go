// +build linux,go1.13 darwin,go1.13 freebsd,go1.13

package mount

import (
	"runtime"
	"testing"

	"github.com/rclone/rclone/vfs/vfstest"
)

func TestMount(t *testing.T) {
	if runtime.NumCPU() <= 2 {
		t.Skip("FIXME skipping mount tests as they lock up on <= 2 CPUs - See: https://github.com/rclone/rclone/issues/3154")
	}
	vfstest.RunTests(t, false, mount)
}
