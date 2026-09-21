// Regression test for #8188: HEAD/GET during the VFS writeback window.

package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHeadGetDuringWriteback uploads an object and immediately HEADs and GETs it
// while it is still in the VFS write-back window — i.e. before the dirty file is
// flushed to the backing remote, so node.DirEntry() is still nil. Previously
// HeadObject/GetObject returned a spurious 404 (KeyNotFound) in that window even
// though ListBucket already reported the object. A long --vfs-write-back keeps
// the window open deterministically. Regression test for #8188.
func TestHeadGetDuringWriteback(t *testing.T) {
	fstest.Initialise()
	ctx := context.Background()
	f, err := fs.NewFs(ctx, t.TempDir())
	require.NoError(t, err)
	const bucket = "test"
	require.NoError(t, f.Mkdir(ctx, bucket))

	keyid := random.String(16)
	keysec := random.String(16)
	opt := Opt
	opt.AuthKey = []string{fmt.Sprintf("%s,%s", keyid, keysec)}
	opt.HTTP.ListenAddr = []string{endpoint}

	// Cache writes and hold the writeback so DirEntry() stays nil for the HEAD/GET.
	vfsOpt := vfscommon.Opt
	vfsOpt.CacheMode = vfscommon.CacheModeWrites
	vfsOpt.WriteBack = fs.Duration(2 * time.Minute)

	w, err := newServer(ctx, f, &opt, &vfsOpt, &proxy.Opt)
	require.NoError(t, err)
	go func() { _ = w.Serve() }()
	t.Cleanup(func() { _ = w.Shutdown() })

	u, err := url.Parse(w.server.URLs()[0])
	require.NoError(t, err)
	client, err := minio.New(u.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(keyid, keysec, ""),
		Secure: false,
	})
	require.NoError(t, err)

	const object = "writeback.txt"
	want := []byte("hello writeback window")
	_, err = client.PutObject(ctx, bucket, object, bytes.NewReader(want), int64(len(want)), minio.PutObjectOptions{})
	require.NoError(t, err)

	// HEAD must not 404 while the object is still being written back.
	info, err := client.StatObject(ctx, bucket, object, minio.StatObjectOptions{})
	require.NoError(t, err)
	assert.Equal(t, int64(len(want)), info.Size)

	// GET must return the body from the VFS cache, not 404.
	rc, err := client.GetObject(ctx, bucket, object, minio.GetObjectOptions{})
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	assert.Equal(t, want, got)
}
