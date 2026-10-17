package s3

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultipartETagConsistency(t *testing.T) {
	ctx := context.Background()
	data := [][]byte{bytes.Repeat([]byte("a"), 5<<20), bytes.Repeat([]byte("b"), 128<<10)}
	want := bytes.Join(data, nil)
	md5ETag := fmt.Sprintf("%x", md5.Sum(want))
	sha1ETag := fmt.Sprintf("%x", sha1.Sum(want))
	for _, tc := range []struct {
		name, backing, hash, etag string
		disableStreaming          bool
		vfsOpt                    *vfscommon.Options
	}{
		{name: "Local", hash: "md5", etag: md5ETag},
		{name: "Memory", backing: ":memory:", hash: "md5", etag: md5ETag},
		{name: "InMemoryUpload", hash: "md5", etag: md5ETag, disableStreaming: true},
		{name: "Writeback", hash: "md5", etag: md5ETag, vfsOpt: cacheWritesVFSOpt(time.Hour)},
		{name: "Auto", hash: "auto", etag: md5ETag},
		{name: "SHA1", hash: "sha1", etag: sha1ETag},
		{name: "NoHash"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			core, _, bucket := newMultipartTestServerVFS(t, tc.backing, tc.disableStreaming, func(opt *Options) {
				opt.EtagHash = tc.hash
			}, tc.vfsOpt)
			const object = "multipart.bin"
			uploadID, err := core.NewMultipartUpload(ctx, bucket, object, minio.PutObjectOptions{})
			require.NoError(t, err)
			var parts []minio.CompletePart
			for i, part := range data {
				uploaded, err := core.PutObjectPart(ctx, bucket, object, uploadID, i+1, bytes.NewReader(part), int64(len(part)), minio.PutObjectPartOptions{})
				require.NoError(t, err)
				parts = append(parts, minio.CompletePart{PartNumber: i + 1, ETag: uploaded.ETag})
			}
			completed, err := core.CompleteMultipartUpload(ctx, bucket, object, uploadID, parts, minio.PutObjectOptions{})
			require.NoError(t, err)
			assert.Equal(t, tc.etag, completed.ETag)

			head, err := core.StatObject(ctx, bucket, object, minio.StatObjectOptions{})
			require.NoError(t, err)
			assert.Equal(t, completed.ETag, head.ETag, "HEAD must return the completed object's ETag")
			body, got, _, err := core.GetObject(ctx, bucket, object, minio.GetObjectOptions{})
			require.NoError(t, err)
			contents, err := io.ReadAll(body)
			require.NoError(t, body.Close())
			require.NoError(t, err)
			assert.Equal(t, want, contents)
			assert.Equal(t, completed.ETag, got.ETag, "GET must return the completed object's ETag")

			listed, err := core.ListObjects(bucket, object, "", "", 1000)
			require.NoError(t, err)
			require.Len(t, listed.Contents, 1)
			assert.Equal(t, completed.ETag, listed.Contents[0].ETag, "LIST must return the completed object's ETag")
			listedV2, err := core.ListObjectsV2(bucket, object, "", "", "", 1000)
			require.NoError(t, err)
			require.Len(t, listedV2.Contents, 1)
			assert.Equal(t, completed.ETag, listedV2.Contents[0].ETag, "LIST v2 must return the completed object's ETag")

			if completed.ETag != "" {
				opts := minio.GetObjectOptions{}
				require.NoError(t, opts.SetMatchETagExcept(completed.ETag))
				body, _, _, err := core.GetObject(ctx, bucket, object, opts)
				if body != nil {
					require.NoError(t, body.Close())
				}
				require.Error(t, err, "the upload ETag must allow a client to reuse its cached object")
				assert.Equal(t, http.StatusNotModified, minio.ToErrorResponse(err).StatusCode)
			}
		})
	}
}
