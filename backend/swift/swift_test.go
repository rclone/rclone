// Test Swift filesystem interface
package swift

import (
	"testing"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestSwift:",
		NilObject:  (*Object)(nil),
	})
}

func (f *Fs) SetUploadChunkSize(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	return f.setUploadChunkSize(cs)
}

var _ fstests.SetUploadChunkSizer = (*Fs)(nil)
