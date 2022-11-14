// Test AzureBlob filesystem interface

//go:build !plan9 && !solaris && !js
// +build !plan9,!solaris,!js

package azureblob

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/stretchr/testify/assert"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName:    "TestAzureBlob:",
		NilObject:     (*Object)(nil),
		TiersToTest:   []string{"Hot", "Cool"},
		ChunkedUpload: fstests.ChunkedUploadConfig{},
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
	var spCredentials servicePrincipalCredentials
	jerr := json.Unmarshal([]byte(credentials), &spCredentials)
	assert.Nil(t, jerr)

	tokenRefresher, err := newServicePrincipalTokenRefresher(ctx, spCredentials)
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
	var spCredentials servicePrincipalCredentials
	jerr := json.Unmarshal([]byte(credentials), &spCredentials)
	assert.Nil(t, jerr)

	_, err := newServicePrincipalTokenRefresher(ctx, spCredentials)
	assert.Error(t, err)
	assert.EqualError(t, err, "error creating service principal token: parameter 'secret' cannot be empty")
}

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
