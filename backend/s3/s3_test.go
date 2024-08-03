// Test S3 filesystem interface
package s3

import (
	"context"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func SetupS3Test(t *testing.T) (context.Context, *Options, *http.Client) {
	ctx, opt := context.Background(), new(Options)
	opt.Provider = "AWS"
	client := getClient(ctx, opt)
	return ctx, opt, client
}

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName:  "TestS3:",
		NilObject:   (*Object)(nil),
		TiersToTest: []string{"STANDARD", "STANDARD_IA"},
		ChunkedUpload: fstests.ChunkedUploadConfig{
			MinChunkSize: minChunkSize,
		},
	})
}

func TestIntegration2(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("skipping as -remote is set")
	}
	name := "TestS3"
	fstests.Run(t, &fstests.Opt{
		RemoteName:  name + ":",
		NilObject:   (*Object)(nil),
		TiersToTest: []string{"STANDARD", "STANDARD_IA"},
		ChunkedUpload: fstests.ChunkedUploadConfig{
			MinChunkSize: minChunkSize,
		},
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "directory_markers", Value: "true"},
		},
	})
}

func TestAWSDualStackOption(t *testing.T) {
	{
		// test enabled
		ctx, opt, client := SetupS3Test(t)
		opt.UseDualStack = true
		s3Conn, err := s3Connection(ctx, opt, client)
		require.NoError(t, err)
		assert.Equal(t, aws.DualStackEndpointStateEnabled, s3Conn.Options().EndpointOptions.UseDualStackEndpoint)
	}
	{
		// test default case
		ctx, opt, client := SetupS3Test(t)
		s3Conn, err := s3Connection(ctx, opt, client)
		require.NoError(t, err)
		assert.Equal(t, aws.DualStackEndpointStateDisabled, s3Conn.Options().EndpointOptions.UseDualStackEndpoint)
	}
}

func (f *Fs) SetUploadChunkSize(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	return f.setUploadChunkSize(cs)
}

func (f *Fs) SetUploadCutoff(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	return f.setUploadCutoff(cs)
}

func (f *Fs) SetCopyCutoff(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	return f.setCopyCutoff(cs)
}

var (
	_ fstests.SetUploadChunkSizer = (*Fs)(nil)
	_ fstests.SetUploadCutoffer   = (*Fs)(nil)
	_ fstests.SetCopyCutoffer     = (*Fs)(nil)
)
