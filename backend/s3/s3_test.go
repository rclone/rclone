// Test S3 filesystem interface
package s3

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/rclone/rclone/fs/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestIntegrationObjectLocking(t *testing.T) {
	var bucketName = "rclone-integration-test-object-locking"
	fstest.Initialise()
	ctx := context.Background()
	ci := fs.GetConfig(ctx)
	ci.Dump = fs.DumpBodies
	ci.LogLevel = fs.LogLevelDebug

	remote, err := fs.NewFs(ctx, *fstest.RemoteName)
	require.NoError(t, err, "remote not configured")
	f, ok := remote.(*Fs)
	if !ok {
		require.Fail(t, "remote is not of type s3")
	}

	f.opt.BucketObjectLockEnabled = true
	require.NoError(t, f.makeBucket(ctx, bucketName), "failed to create bucket")
	f.setRoot(bucketName)

	supported, enabled, err := f.setObjectLockingStatus(ctx)
	require.NoError(t, err, "failed to check object-locking configuration")
	require.True(t, supported, "object locking should be supported")
	require.True(t, enabled, "object locking should be enabled")

	// PutObject with GOVERNANCE + Legal Hold
	testContent := "Test content"
	testFile := bytes.NewReader([]byte(testContent))
	testFilePath := "testFile"
	retention, _ := fs.ParseTime("1M", true)
	f.opt.ObjectLockRetainUntil = fs.TimeFuture(retention)
	f.opt.ObjectLockMode = s3.ObjectLockModeGovernance
	f.opt.ObjectLockLegalHold = fs.Tristate{Value: true, Valid: true}

	object, err := f.Put(ctx, testFile, &Object{remote: testFilePath, fs: f, bytes: int64(len(testContent))})
	require.NoError(t, err, "upload failed")
	o := object.(*Object)
	require.Equal(t, s3.ObjectLockModeGovernance, *o.objectLockMode)
	require.True(t, retention.Equal(*o.objectLockRetainUntil)) // ignores timezone
	assert.Equal(t, s3.ObjectLockLegalHoldStatusOn, *o.objectLockLegalHoldStatus, "legal hold not supported by server")

	// PutObjectMultipart
	testFile = bytes.NewReader([]byte(testContent))
	testFilePath = "testFileMultipart"
	_, _ = f.SetUploadCutoff(0) // force multipart

	object, err = f.Put(ctx, testFile, &Object{remote: testFilePath, fs: f, bytes: int64(len(testContent))})
	require.NoError(t, err, "multipart upload failed")
	o2 := object.(*Object)
	require.Equal(t, s3.ObjectLockModeGovernance, *o2.objectLockMode)
	require.True(t, retention.Equal(*o2.objectLockRetainUntil)) // ignores timezone
	assert.Equal(t, s3.ObjectLockLegalHoldStatusOn, *o2.objectLockLegalHoldStatus, "legal hold not supported by server")

	// Server-Side Copy
	retention, _ = fs.ParseTime("2M", true) // different retention time is used here just in case it got copied
	f.opt.ObjectLockRetainUntil = fs.TimeFuture(retention)

	copied, err := f.Copy(ctx, o, "copy")
	require.NoError(t, err, "failed server-side copy")
	o3 := copied.(*Object)
	require.Equal(t, s3.ObjectLockModeGovernance, *o3.objectLockMode)
	require.True(t, retention.Equal(*o3.objectLockRetainUntil))
	assert.Equal(t, s3.ObjectLockLegalHoldStatusOn, *o3.objectLockLegalHoldStatus, "legal hold not supported by server")

	require.NoError(t, o.SetLegalHold(ctx, false))
	require.NoError(t, o2.SetLegalHold(ctx, false))
	require.NoError(t, o3.SetLegalHold(ctx, false))
	// Convert everything to delete marker
	require.NoError(t, operations.Delete(ctx, f))
	// Real Delete
	f.opt.BypassGovernanceRetention = true
	err = f.CleanUpHidden(ctx)
	require.NoError(t, err)

	_, err = f.c.DeleteBucketWithContext(ctx, &s3.DeleteBucketInput{Bucket: &bucketName})
	require.NoError(t, err, "cleanup failed")
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
