// Test Drive filesystem interface

package drive

import (
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestDrive:",
		NilObject:  (*Object)(nil),
		ChunkedUpload: fstests.ChunkedUploadConfig{
			MinChunkSize:  minChunkSize,
			CeilChunkSize: fstests.NextPowerOfTwo,
		},
	})
}

func (f *Fs) SetUploadChunkSize(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	return f.setUploadChunkSize(cs)
}

func (f *Fs) SetUploadCutoff(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	return f.setUploadCutoff(cs)
}

var (
	_ fstests.SetUploadChunkSizer = (*Fs)(nil)
	_ fstests.SetUploadCutoffer   = (*Fs)(nil)
)

// TestCycleDetectionFlag tests that the cycle detection flag is parsed correctly
func TestCycleDetectionFlag(t *testing.T) {
	// Test default value
	opt := &Options{}
	if opt.CycleDetection != false {
		t.Errorf("Expected CycleDetection to default to false, got %v", opt.CycleDetection)
	}

	// Test setting to true
	opt.CycleDetection = true
	if opt.CycleDetection != true {
		t.Errorf("Expected CycleDetection to be set to true, got %v", opt.CycleDetection)
	}
}
