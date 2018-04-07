// Test Cache filesystem interface

// +build !plan9

package cache_test

import (
	"testing"

	"github.com/ncw/rclone/backend/cache"
	_ "github.com/ncw/rclone/backend/local"
	"github.com/ncw/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestCache:",
		NilObject:  (*cache.Object)(nil),
	})
}
