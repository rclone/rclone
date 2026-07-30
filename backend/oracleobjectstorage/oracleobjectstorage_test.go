//go:build !plan9 && !solaris && !js

package oracleobjectstorage

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"fmt"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
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

// InternalTest is called by fstests.Run to extra tests
func (f *Fs) InternalTest(t *testing.T) {
	t.Run("GzipEncoding", f.InternalTestGzipEncoding)
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
