package s3

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rclone/gofakes3"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestBackend serves a root directory containing a bucket directory, an
// object inside that bucket and a root-level file that is not part of any
// bucket. It returns the backend and the serve root path.
func newTestBackend(t *testing.T) (*s3Backend, string) {
	fstest.Initialise()
	ctx := context.Background()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "bucket"), 0777))
	require.NoError(t, os.WriteFile(filepath.Join(root, "root-secret.txt"), []byte(rootSecret), 0666))
	require.NoError(t, os.WriteFile(filepath.Join(root, "bucket", "object.txt"), []byte("normal object"), 0666))

	f, err := fs.NewFs(ctx, root)
	require.NoError(t, err)

	opt := Opt
	opt.HTTP.ListenAddr = []string{endpoint}
	w, err := newServer(ctx, f, &opt, &vfscommon.Opt, &proxy.Opt)
	require.NoError(t, err)

	return newBackend(w).(*s3Backend), root
}

const rootSecret = "ROOT_LEVEL_SECRET_MARKER"

// TestPathTraversal checks that dot-dot segments in an object key cannot
// escape the bucket namespace to read, overwrite or create files outside the
// selected bucket.
func TestPathTraversal(t *testing.T) {
	ctx := context.Background()

	t.Run("Get", func(t *testing.T) {
		b, _ := newTestBackend(t)
		_, err := b.GetObject(ctx, "bucket", "../root-secret.txt", nil)
		assert.Error(t, err, "GET with dot-dot key must not read the root-level file")
	})

	t.Run("Head", func(t *testing.T) {
		b, _ := newTestBackend(t)
		_, err := b.HeadObject(ctx, "bucket", "../root-secret.txt")
		assert.Error(t, err, "HEAD with dot-dot key must not find the root-level file")
	})

	t.Run("Put", func(t *testing.T) {
		b, root := newTestBackend(t)
		const payload = "OVERWRITTEN_BY_DOTDOT"
		_, err := b.PutObject(ctx, "bucket", "../root-secret.txt", map[string]string{}, strings.NewReader(payload), int64(len(payload)))
		assert.Error(t, err, "PUT with dot-dot key must not overwrite the root-level file")
		got, readErr := os.ReadFile(filepath.Join(root, "root-secret.txt"))
		require.NoError(t, readErr)
		assert.Equal(t, rootSecret, string(got), "root-level file must be unchanged")
	})

	// path.Dir of the joined path can re-introduce a traversal, so check that a
	// rejected PUT does not create directories outside the bucket as a side
	// effect of making the object's parent directory.
	t.Run("PutNoSideEffectDir", func(t *testing.T) {
		b, root := newTestBackend(t)
		_, err := b.PutObject(ctx, "bucket", "../sneaky-dir/f.txt", map[string]string{}, strings.NewReader("x"), 1)
		assert.Error(t, err, "PUT with dot-dot key must be rejected")
		_, statErr := os.Stat(filepath.Join(root, "sneaky-dir"))
		assert.True(t, os.IsNotExist(statErr), "no directory must be created outside the bucket")
	})

	// A legitimate object inside the bucket must still be readable.
	t.Run("LegitimateObject", func(t *testing.T) {
		b, _ := newTestBackend(t)
		obj, err := b.GetObject(ctx, "bucket", "object.txt", nil)
		require.NoError(t, err)
		defer func() { _ = obj.Contents.Close() }()
		contents, err := io.ReadAll(obj.Contents)
		require.NoError(t, err)
		assert.Equal(t, "normal object", string(contents))
	})

	// A legitimate nested object must still be writable, creating its parent
	// directory inside the bucket.
	t.Run("LegitimateNestedPut", func(t *testing.T) {
		b, root := newTestBackend(t)
		const payload = "nested"
		_, err := b.PutObject(ctx, "bucket", "sub/dir/file.txt", map[string]string{}, strings.NewReader(payload), int64(len(payload)))
		require.NoError(t, err)
		got, readErr := os.ReadFile(filepath.Join(root, "bucket", "sub", "dir", "file.txt"))
		require.NoError(t, readErr)
		assert.Equal(t, payload, string(got))
	})
}

// TestBucketObjectPath unit-tests the bucket/key join used by serve s3. Object
// keys are opaque, so non-canonical keys (containing "..", ".", "//", or
// leading/trailing slashes) are rejected rather than normalised.
func TestBucketObjectPath(t *testing.T) {
	for _, test := range []struct {
		bucket, key string
		want        string
		wantErr     bool
	}{
		{"bucket", "object.txt", "bucket/object.txt", false},
		{"bucket", "a/b/c.txt", "bucket/a/b/c.txt", false},
		{"bucket", "a/../b.txt", "", true}, // distinct from b.txt, not normalised
		{"bucket", "a/./b.txt", "", true},
		{"bucket", "a//b.txt", "", true},
		{"bucket", "/leading", "", true},
		{"bucket", "trailing/", "", true},
		{"bucket", "", "", true},
		{"bucket", "../root-secret.txt", "", true},
		{"bucket", "../../etc/passwd", "", true},
		{"bucket", "../otherbucket/x", "", true},
		{"bucket", "a/../../escape", "", true},
		{"bucket", "..", "", true},
	} {
		got, err := bucketObjectPath(test.bucket, test.key)
		if test.wantErr {
			assert.Error(t, err, "bucket=%q key=%q", test.bucket, test.key)
			assert.True(t, gofakes3.HasErrorCode(err, gofakes3.ErrInvalidArgument), "want 400 InvalidArgument for key=%q, got %v", test.key, err)
		} else {
			require.NoError(t, err, "bucket=%q key=%q", test.bucket, test.key)
			assert.Equal(t, test.want, got, "bucket=%q key=%q", test.bucket, test.key)
		}
	}
}

// TestBucketDirPath unit-tests the directory-prefix join: the empty prefix
// addresses the bucket root and a trailing slash is ignored, but traversal and
// other non-canonical prefixes are still rejected.
func TestBucketDirPath(t *testing.T) {
	for _, test := range []struct {
		bucket, dir string
		want        string
		wantErr     bool
	}{
		{"bucket", "", "bucket", false}, // empty prefix addresses the bucket root
		{"bucket", "dir", "bucket/dir", false},
		{"bucket", "dir/", "", true}, // trailing slash not normalised away
		{"bucket", "a/b", "bucket/a/b", false},
		{"bucket", "../x", "", true},
		{"bucket", "a//b", "", true},
		{"bucket", "..", "", true},
	} {
		got, err := bucketDirPath(test.bucket, test.dir)
		if test.wantErr {
			assert.Error(t, err, "bucket=%q dir=%q", test.bucket, test.dir)
		} else {
			require.NoError(t, err, "bucket=%q dir=%q", test.bucket, test.dir)
			assert.Equal(t, test.want, got, "bucket=%q dir=%q", test.bucket, test.dir)
		}
	}
}
