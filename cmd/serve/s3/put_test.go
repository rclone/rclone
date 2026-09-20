// PutObject error handling tests for serve s3.
//
// A PUT which fails part way through its body must never disturb the object
// already stored at the key.

package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"testing"
	"time"

	"github.com/ncw/swift/v2"
	"github.com/rclone/gofakes3"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errorReader yields data then fails with err.
type errorReader struct {
	data []byte
	err  error
}

func (r *errorReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, r.err
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

// newPutTestBackend returns an s3Backend for direct PutObject tests, backed by
// the named remote, plus the backing Fs and a bucket created on it. vfsOpt
// overrides the VFS options (nil for the defaults).
func newPutTestBackend(t *testing.T, backing string, vfsOpt *vfscommon.Options) (*s3Backend, fs.Fs, string) {
	fstest.Initialise()
	ctx := context.Background()
	if backing == "" {
		backing = t.TempDir()
	}
	f, err := fs.NewFs(ctx, backing)
	require.NoError(t, err)
	bucket := fmt.Sprintf("test-%d", testBackingCounter.Add(1))
	require.NoError(t, f.Mkdir(ctx, bucket))
	if vfsOpt == nil {
		vfsOpt = &vfscommon.Opt
	}
	// The VFS is cached per remote, so a shared ":memory:" backing reuses a
	// VFS whose cached root listing predates the bucket just created; forget
	// it so the new bucket is visible.
	if root, err := vfs.New(ctx, f, vfsOpt).Root(); err == nil {
		root.ForgetAll()
	}
	opt := Opt
	opt.HTTP.ListenAddr = []string{endpoint}
	w, err := newServer(ctx, f, &opt, vfsOpt, &proxy.Opt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Shutdown() })
	return newBackend(w), f, bucket
}

var errBoom = errors.New("boom")

// failureModes enumerates the ways a PUT body can fail to arrive in full:
// an arbitrary read error, the unexpected EOF a dropped connection gives
// (which must not be mistaken for a clean end of stream), and a body which
// ends cleanly short of its declared size.
var failureModes = []struct {
	name      string
	readerErr error
	wantErr   error
}{
	{"ReadError", errBoom, errBoom},
	{"UnexpectedEOF", io.ErrUnexpectedEOF, io.ErrUnexpectedEOF},
	{"ShortBody", io.EOF, gofakes3.ErrIncompleteBody},
}

// failPut makes a PutObject call to bucket/object whose body fails with
// readerErr part way through and asserts wantErr is passed back.
func failPut(t *testing.T, b *s3Backend, bucket, object string, readerErr, wantErr error) {
	_, err := b.PutObject(context.Background(), bucket, object, map[string]string{},
		&errorReader{data: []byte(random.String(50)), err: readerErr}, 1000)
	require.ErrorIs(t, err, wantErr)
}

// TestPutObjectFailurePreservesExisting checks that a failed PUT leaves the
// object stored at the key untouched - neither removed nor overwritten with
// truncated data - with no temporary object left behind.
func TestPutObjectFailurePreservesExisting(t *testing.T) {
	for _, tc := range testRemotes {
		for _, fm := range failureModes {
			t.Run(tc.name+"/"+fm.name, func(t *testing.T) {
				b, f, bucket := newPutTestBackend(t, tc.backing, nil)
				ctx := context.Background()
				const object = "existing.txt"

				existing := []byte(random.String(100))
				_, err := b.PutObject(ctx, bucket, object, map[string]string{}, bytes.NewReader(existing), int64(len(existing)))
				require.NoError(t, err)
				assert.Equal(t, existing, readObject(t, f, bucket, object))
				requireOnly(t, f, bucket, object)

				failPut(t, b, bucket, object, fm.readerErr, fm.wantErr)

				assert.Equal(t, existing, readObject(t, f, bucket, object))
				requireOnly(t, f, bucket, object)

				// The object must still be served correctly - in particular
				// HeadObject must not report the failed upload's size.
				head, err := b.HeadObject(ctx, bucket, object)
				require.NoError(t, err)
				assert.Equal(t, int64(len(existing)), head.Size)
			})
		}
	}
}

// TestPutObjectFailureNewKey checks that a failed PUT to a key with no
// existing object leaves nothing behind - no partial object at the key and
// no temporary object.
func TestPutObjectFailureNewKey(t *testing.T) {
	for _, tc := range testRemotes {
		for _, fm := range failureModes {
			t.Run(tc.name+"/"+fm.name, func(t *testing.T) {
				b, f, bucket := newPutTestBackend(t, tc.backing, nil)
				ctx := context.Background()
				const object = "new.txt"

				failPut(t, b, bucket, object, fm.readerErr, fm.wantErr)

				_, err := f.NewObject(ctx, path.Join(bucket, object))
				require.ErrorIs(t, err, fs.ErrorObjectNotFound)
				requireOnly(t, f, bucket)

				// The failed upload must not be served either
				_, err = b.HeadObject(ctx, bucket, object)
				require.Error(t, err)
			})
		}
	}
}

// TestPutObjectMtime checks that the object's modtime is set from the
// "X-Amz-Meta-Mtime" or "mtime" metadata supplied with the PUT.
func TestPutObjectMtime(t *testing.T) {
	want := fstest.Time("2011-12-25T12:59:59.123456789Z")
	for _, metaKey := range []string{"X-Amz-Meta-Mtime", "mtime"} {
		t.Run(metaKey, func(t *testing.T) {
			b, f, bucket := newPutTestBackend(t, "", nil)
			ctx := context.Background()
			const object = "mtime.txt"

			contents := []byte(random.String(50))
			meta := map[string]string{metaKey: swift.TimeToFloatString(want)}
			_, err := b.PutObject(ctx, bucket, object, meta, bytes.NewReader(contents), int64(len(contents)))
			require.NoError(t, err)

			o, err := f.NewObject(ctx, path.Join(bucket, object))
			require.NoError(t, err)
			fstest.AssertTimeEqualWithPrecision(t, object, want, o.ModTime(ctx), f.Precision())
		})
	}
}

// waitForObject waits for bucket/object to appear on the backing Fs (e.g.
// after the VFS write-back delay).
func waitForObject(t *testing.T, f fs.Fs, bucket, object string) {
	ctx := context.Background()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := f.NewObject(ctx, path.Join(bucket, object)); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("object %s/%s never appeared on the backing remote", bucket, object)
}

// TestPutObjectFailureCached checks the failed PUT semantics with the cache
// modes that write through the VFS cache - minimal and above, since the PUT
// opens its handle read-write: the truncated file the failed upload leaves
// in the cache must be discarded, not written back over the object at the
// key.
func TestPutObjectFailureCached(t *testing.T) {
	for _, cm := range []vfscommon.CacheMode{vfscommon.CacheModeMinimal, vfscommon.CacheModeWrites} {
		for _, tc := range testRemotes {
			t.Run(cm.String()+"/"+tc.name, func(t *testing.T) {
				vfsOpt := vfscommon.Opt
				vfsOpt.CacheMode = cm
				vfsOpt.WriteBack = fs.Duration(100 * time.Millisecond)
				b, f, bucket := newPutTestBackend(t, tc.backing, &vfsOpt)
				const object = "cached.txt"

				existing := []byte(random.String(100))
				_, err := b.PutObject(context.Background(), bucket, object, map[string]string{}, bytes.NewReader(existing), int64(len(existing)))
				require.NoError(t, err)
				waitForObject(t, f, bucket, object)
				assert.Equal(t, existing, readObject(t, f, bucket, object))

				failPut(t, b, bucket, object, errBoom, errBoom)

				// Wait out several write-back intervals to catch the truncated
				// data being written back before checking the object is
				// untouched.
				time.Sleep(time.Second)
				assert.Equal(t, existing, readObject(t, f, bucket, object))
				requireOnly(t, f, bucket, object)
			})
		}
	}
}
