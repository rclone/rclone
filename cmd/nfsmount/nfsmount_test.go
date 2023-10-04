//go:build darwin && !cmount
// +build darwin,!cmount

package nfsmount

import (
	"testing"

	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfstest"
)

func TestMount(t *testing.T) {
	vfstest.RunTests(t, false, vfscommon.CacheModeMinimal, false, mount)
}
