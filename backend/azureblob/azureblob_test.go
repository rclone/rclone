// Test AzureBlob filesystem interface

// +build !plan9,!solaris,!js,go1.14

package azureblob

import (
	"context"
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
			MaxChunkSize: maxChunkSize,
		},
	})
}

func (f *Fs) SetUploadChunkSize(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	return f.setUploadChunkSize(cs)
}

var (
	_ fstests.SetUploadChunkSizer = (*Fs)(nil)
)

// TestServicePrincipalFileSuccess checks that, given a proper JSON file, we can create a token.
func TestServicePrincipalFileSuccess(t *testing.T) {
	ctx := context.TODO()
	credentials := `
{
    "appId": "my application (client) ID",
    "password": "my secret",
    "tenant": "my active directory tenant ID"
}
`
	tokenRefresher, err := newServicePrincipalTokenRefresher(ctx, []byte(credentials))
	if assert.NoError(t, err) {
		assert.NotNil(t, tokenRefresher)
	}
}

// TestServicePrincipalFileFailure checks that, given a JSON file with a missing secret, it returns an error.
func TestServicePrincipalFileFailure(t *testing.T) {
	ctx := context.TODO()
	credentials := `
{
    "appId": "my application (client) ID",
    "tenant": "my active directory tenant ID"
}
`
	_, err := newServicePrincipalTokenRefresher(ctx, []byte(credentials))
	assert.Error(t, err)
	assert.EqualError(t, err, "error creating service principal token: parameter 'secret' cannot be empty")
}
