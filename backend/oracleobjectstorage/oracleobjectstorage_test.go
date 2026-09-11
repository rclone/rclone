//go:build !plan9 && !solaris && !js

package oracleobjectstorage

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName:  "TestOracleObjectStorage:",
		TiersToTest: []string{"standard", "archive"},
		NilObject:   (*Object)(nil),
		ChunkedUpload: fstests.ChunkedUploadConfig{
			MinChunkSize: minChunkSize,
		},
	})
}

func gz(t *testing.T, s string) string {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write([]byte(s))
	require.NoError(t, err)
	err = zw.Close()
	require.NoError(t, err)
	return buf.String()
}

func md5sum(t *testing.T, s string) string {
	hash := md5.Sum([]byte(s))
	return fmt.Sprintf("%x", hash)
}

// InternalTestGzipEncoding tests that a file uploaded with
// Content-Encoding: gzip can be downloaded with and without
// decompression.
func (f *Fs) InternalTestGzipEncoding(t *testing.T) {
	ctx := context.Background()
	original := random.String(1000)
	contents := gz(t, original)
	item := fstest.NewItem("test-gzip-encoding", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
	obj := fstests.PutTestContentsMetadata(ctx, t, f, &item, true, contents, true, "text/plain", nil, &fs.HTTPOption{Key: "Content-Encoding", Value: "gzip"})
	defer func() {
		assert.NoError(t, obj.Remove(ctx))
	}()
	o := obj.(*Object)

	checkDownload := func(wantContents string, wantSize int64, wantHash string) {
		gotContents := fstests.ReadObject(ctx, t, o, -1)
		assert.Equal(t, wantContents, gotContents)
		assert.Equal(t, wantSize, o.Size())
		gotHash, err := o.Hash(ctx, hash.MD5)
		require.NoError(t, err)
		assert.Equal(t, wantHash, gotHash)
	}

	t.Run("NoDecompress", func(t *testing.T) {
		checkDownload(contents, int64(len(contents)), md5sum(t, contents))
	})
	t.Run("Decompress", func(t *testing.T) {
		f.opt.Decompress = true
		defer func() {
			f.opt.Decompress = false
		}()
		checkDownload(original, -1, "")
	})
}

// InternalTestPurgeBatches tests purging more objects than fit in one batch.
func (f *Fs) InternalTestPurgeBatches(t *testing.T) {
	ctx := context.Background()
	const (
		dir               = "purge-batches"
		objectCount       = 1001
		uploadConcurrency = 16
	)
	defer func() { _ = f.Purge(ctx, dir) }()
	jobs := make(chan int)
	errs := make(chan error, objectCount)
	var wg sync.WaitGroup
	for range uploadConcurrency {
		wg.Go(func() {
			for i := range jobs {
				remote := fmt.Sprintf("%s/%04d", dir, i)
				src := object.NewStaticObjectInfo(remote, time.Time{}, 0, true, nil, f)
				_, err := f.Put(ctx, bytes.NewReader(nil), src)
				if err != nil {
					errs <- err
				}
			}
		})
	}
	for i := range objectCount {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.NoError(t, f.Purge(ctx, dir))
	entries, err := f.List(ctx, dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// InternalTestPurgeDirectoryMarkers tests purging directory marker objects.
func (f *Fs) InternalTestPurgeDirectoryMarkers(t *testing.T) {
	ctx := context.Background()
	const dir = "purge-directory-markers"
	bucketName, directory := f.split(dir)
	objectNames := []string{
		directory + "/",
		directory + "/child/",
		directory + "/child/object",
	}
	defer func() { _ = f.Purge(ctx, dir) }()
	for _, objectName := range objectNames {
		req := objectstorage.PutObjectRequest{
			NamespaceName: new(f.opt.Namespace),
			BucketName:    new(bucketName),
			ObjectName:    new(objectName),
			PutObjectBody: io.NopCloser(bytes.NewReader(nil)),
		}
		err := f.pacer.Call(func() (bool, error) {
			resp, err := f.srv.PutObject(ctx, req)
			return shouldRetry(ctx, resp.HTTPResponse(), err)
		})
		require.NoError(t, err)
	}
	// Use the SDK directly because normal OOS listings hide directory markers.
	listObjectNames := func() []string {
		req := objectstorage.ListObjectsRequest{
			NamespaceName: new(f.opt.Namespace),
			BucketName:    new(bucketName),
			Prefix:        new(directory + "/"),
		}
		var resp objectstorage.ListObjectsResponse
		err := f.pacer.Call(func() (bool, error) {
			var err error
			resp, err = f.srv.ListObjects(ctx, req)
			return shouldRetry(ctx, resp.HTTPResponse(), err)
		})
		require.NoError(t, err)
		names := make([]string, 0, len(resp.Objects))
		for _, object := range resp.Objects {
			names = append(names, *object.Name)
		}
		return names
	}
	assert.ElementsMatch(t, objectNames, listObjectNames())
	require.NoError(t, f.Purge(ctx, dir))
	assert.Empty(t, listObjectNames())
}

// InternalTest is called by fstests.Run to extra tests
func (f *Fs) InternalTest(t *testing.T) {
	t.Run("GzipEncoding", f.InternalTestGzipEncoding)
	t.Run("PurgeBatches", f.InternalTestPurgeBatches)
	t.Run("PurgeDirectoryMarkers", f.InternalTestPurgeDirectoryMarkers)
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
