// Test S3 filesystem interface
package s3

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
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
		s3Conn, _, _ := s3Connection(ctx, opt, client)
		if !strings.Contains(s3Conn.Endpoint, "dualstack") {
			t.Errorf("dualstack failed got: %s, wanted: dualstack", s3Conn.Endpoint)
			t.Fail()
		}
	}
	{
		// test default case
		ctx, opt, client := SetupS3Test(t)
		s3Conn, _, _ := s3Connection(ctx, opt, client)
		if strings.Contains(s3Conn.Endpoint, "dualstack") {
			t.Errorf("dualstack failed got: %s, NOT wanted: dualstack", s3Conn.Endpoint)
			t.Fail()
		}
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
