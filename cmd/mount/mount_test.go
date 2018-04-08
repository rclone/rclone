// +build linux darwin freebsd

package mount

import (
	"testing"

	"github.com/ncw/rclone/cmd/mountlib/mounttest"
)

func TestMount(t *testing.T) {
	mounttest.RunTests(t, mount)
}
