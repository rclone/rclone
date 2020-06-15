// +build linux darwin,amd64

package mount2

import (
	"testing"

	"github.com/rclone/rclone/fstest/testy"
	"github.com/rclone/rclone/vfs/vfstest"
)

func TestMount(t *testing.T) {
	testy.SkipUnreliable(t)
	vfstest.RunTests(t, false, mount)
}
