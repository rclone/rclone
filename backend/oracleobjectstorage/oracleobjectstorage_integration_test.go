//go:build !plan9 && !solaris && !js

package oracleobjectstorage

import (
	"testing"

	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration2 runs integration tests with directory markers enabled.
func TestIntegration2(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	name := "TestOracleObjectStorage"
	fstests.Run(t, &fstests.Opt{
		RemoteName:  name + ":",
		TiersToTest: []string{"standard", "archive"},
		NilObject:   (*Object)(nil),
		ChunkedUpload: fstests.ChunkedUploadConfig{
			MinChunkSize: minChunkSize,
		},
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "directory_markers", Value: "true"},
		},
	})
}
