// Test AzureBlob filesystem interface

//go:build !plan9 && !solaris && !js

package azureblob

import (
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/stretchr/testify/assert"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName:  "TestAzureBlob:",
		NilObject:   (*Object)(nil),
		TiersToTest: []string{"Hot", "Cool", "Cold"},
		ChunkedUpload: fstests.ChunkedUploadConfig{
			MinChunkSize: defaultChunkSize,
		},
	})
}

// TestIntegration2 runs integration tests against the remote
func TestIntegration2(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	name := "TestAzureBlob"
	fstests.Run(t, &fstests.Opt{
		RemoteName:  name + ":",
		NilObject:   (*Object)(nil),
		TiersToTest: []string{"Hot", "Cool", "Cold"},
		ChunkedUpload: fstests.ChunkedUploadConfig{
			MinChunkSize: defaultChunkSize,
		},
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "directory_markers", Value: "true"},
		},
	})
}

func (f *Fs) SetUploadChunkSize(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	return f.setUploadChunkSize(cs)
}

var (
	_ fstests.SetUploadChunkSizer = (*Fs)(nil)
)

func TestValidateAccessTier(t *testing.T) {
	tests := map[string]struct {
		accessTier string
		want       bool
	}{
		"hot":     {"hot", true},
		"HOT":     {"HOT", true},
		"Hot":     {"Hot", true},
		"cool":    {"cool", true},
		"cold":    {"cold", true},
		"archive": {"archive", true},
		"empty":   {"", false},
		"unknown": {"unknown", false},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := validateAccessTier(test.accessTier)
			assert.Equal(t, test.want, got)
		})
	}
}
