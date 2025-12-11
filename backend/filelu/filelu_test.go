package filelu_test

import (
	"strings"
	"testing"

	"github.com/rclone/rclone/fstest/fstests"
)

// Custom wrapper to filter out unwanted subtests
func TestIntegration(t *testing.T) {
	// Skip specific encoding tests manually
	if strings.Contains(t.Name(), "FsEncoding/punctuation") {
		t.Skip("Skipping FsEncoding/punctuation — not supported")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName:      "filelu:",
		NilObject:       nil,
		SkipInvalidUTF8: true,
	})
}
