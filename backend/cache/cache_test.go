// Test Cache filesystem interface

// +build !plan9,!js
// +build !race

package cache_test

import (
	"testing"

	"github.com/rclone/rclone/backend/cache"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName:                   "TestCache:",
		NilObject:                    (*cache.Object)(nil),
		UnimplementableFsMethods:     []string{"PublicLink", "OpenWriterAt"},
		UnimplementableObjectMethods: []string{"MimeType", "ID", "GetTier", "SetTier"},
		SkipInvalidUTF8:              true, // invalid UTF-8 confuses the cache
	})
}
