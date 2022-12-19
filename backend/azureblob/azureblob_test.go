// Test AzureBlob filesystem interface

//go:build !plan9 && !solaris && !js && go1.18
// +build !plan9,!solaris,!js,go1.18

package azureblob

import (
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/stretchr/testify/assert"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName:  "TestAzureBlob:",
		NilObject:   (*Object)(nil),
		TiersToTest: []string{"Hot", "Cool"},
		ChunkedUpload: fstests.ChunkedUploadConfig{
			MinChunkSize: defaultChunkSize,
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
