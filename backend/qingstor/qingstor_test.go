// Test QingStor filesystem interface

// +build !plan9

package qingstor_test

import (
	"testing"

	"github.com/ncw/rclone/backend/qingstor"
	"github.com/ncw/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestQingStor:",
		NilObject:  (*qingstor.Object)(nil),
	})
}
