// Internal tests for operations

package operations

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/object"
	"github.com/stretchr/testify/assert"
)

func TestSizeDiffers(t *testing.T) {
	ctx := context.Background()
	ci := fs.GetConfig(ctx)
	when := time.Now()
	for _, test := range []struct {
		ignoreSize bool
		srcSize    int64
		dstSize    int64
		want       bool
	}{
		{false, 0, 0, false},
		{false, 1, 2, true},
		{false, 1, -1, false},
		{false, -1, 1, false},
		{true, 0, 0, false},
		{true, 1, 2, false},
		{true, 1, -1, false},
		{true, -1, 1, false},
	} {
		src := object.NewStaticObjectInfo("a", when, test.srcSize, true, nil, nil)
		dst := object.NewStaticObjectInfo("a", when, test.dstSize, true, nil, nil)
		oldIgnoreSize := ci.IgnoreSize
		ci.IgnoreSize = test.ignoreSize
		got := sizeDiffers(ctx, src, dst)
		ci.IgnoreSize = oldIgnoreSize
		assert.Equal(t, test.want, got, fmt.Sprintf("ignoreSize=%v, srcSize=%v, dstSize=%v", test.ignoreSize, test.srcSize, test.dstSize))
	}
}

// stubFs is a minimal fs.Info implementation for unit testing equal/CheckHashes.
type stubFs struct {
	name   string
	hashes hash.Set
}

func (f *stubFs) Name() string             { return f.name }
func (f *stubFs) Root() string             { return "" }
func (f *stubFs) String() string           { return f.name }
func (f *stubFs) Precision() time.Duration { return time.Nanosecond }
func (f *stubFs) Hashes() hash.Set         { return f.hashes }
func (f *stubFs) Features() *fs.Features   { return &fs.Features{} }

// stubObject is a minimal fs.Object implementation that lets the test
// control both the parent Fs and the value returned from Hash().
type stubObject struct {
	parent   fs.Info
	remote   string
	size     int64
	modTime  time.Time
	hashes   map[hash.Type]string
	hashErr  error
	hashSeen []hash.Type
}

func (o *stubObject) Fs() fs.Info                       { return o.parent }
func (o *stubObject) String() string                    { return o.remote }
func (o *stubObject) Remote() string                    { return o.remote }
func (o *stubObject) ModTime(context.Context) time.Time { return o.modTime }
func (o *stubObject) Size() int64                       { return o.size }
func (o *stubObject) Storable() bool                    { return true }
func (o *stubObject) Hash(_ context.Context, ht hash.Type) (string, error) {
	o.hashSeen = append(o.hashSeen, ht)
	if o.hashErr != nil {
		return "", o.hashErr
	}
	return o.hashes[ht], nil
}
func (o *stubObject) SetModTime(context.Context, time.Time) error { return nil }
func (o *stubObject) Open(context.Context, ...fs.OpenOption) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}
func (o *stubObject) Update(context.Context, io.Reader, fs.ObjectInfo, ...fs.OpenOption) error {
	return nil
}
func (o *stubObject) Remove(context.Context) error { return nil }

// captureLogger redirects fs log output into the supplied buffer for the
// duration of the test and restores the previous handler on cleanup.
func captureLogger(t *testing.T, buf *bytes.Buffer) {
	t.Helper()
	var mu sync.Mutex
	oldLevel := log.Handler.SetLevel(slog.LevelDebug)
	log.Handler.SetOutput(func(level slog.Level, text string) {
		mu.Lock()
		defer mu.Unlock()
		buf.WriteString(text)
	})
	t.Cleanup(func() {
		log.Handler.ResetOutput()
		log.Handler.SetLevel(oldLevel)
	})
}

// TestEqualChecksumEmptyDstHashWarning covers issue #9540.
//
// When `--checksum` is in use and the two filesystems share a common hash
// type, but the destination object happens to have no stored hash (e.g.
// because it was uploaded via `rcat` and its multipart ETag is not a plain
// MD5), `equal` silently falls back to a size-only comparison. Verify that
// rclone now emits a one-shot warning in that situation so the operator is
// not misled into thinking a checksum check actually happened.
func TestEqualChecksumEmptyDstHashWarning(t *testing.T) {
	var buf bytes.Buffer
	captureLogger(t, &buf)

	// Reset the package-level once so the warning is allowed to fire even if
	// some other test in this binary already tripped it.
	emptyHashWarning = sync.Once{}

	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	ci.CheckSum = true

	srcFs := &stubFs{name: "srcFs", hashes: hash.NewHashSet(hash.MD5)}
	dstFs := &stubFs{name: "dstFs", hashes: hash.NewHashSet(hash.MD5)}

	when := time.Now()
	src := &stubObject{
		parent:  srcFs,
		remote:  "rcat-uploaded.bin",
		size:    1024,
		modTime: when,
		hashes:  map[hash.Type]string{hash.MD5: "d41d8cd98f00b204e9800998ecf8427e"},
	}
	dst := &stubObject{
		parent:  dstFs,
		remote:  "rcat-uploaded.bin",
		size:    1024,
		modTime: when,
		// no MD5 stored — mirrors the rcat/multipart case from #9540
		hashes: map[hash.Type]string{},
	}

	got := equal(ctx, src, dst, defaultEqualOpt(ctx))
	assert.True(t, got, "files should still be treated as equal (size matches and hash check is skipped)")

	const want = "--checksum is in use but the destination has no stored hash"
	assert.True(t,
		strings.Contains(buf.String(), want),
		"expected warning %q, got log output:\n%s", want, buf.String(),
	)
}

// TestEqualChecksumNoCommonHashWarningUnchanged guards the existing "no
// hashes in common" warning so that the fix for #9540 does not regress it.
func TestEqualChecksumNoCommonHashWarningUnchanged(t *testing.T) {
	var buf bytes.Buffer
	captureLogger(t, &buf)

	checksumWarning = sync.Once{}

	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	ci.CheckSum = true

	srcFs := &stubFs{name: "srcFs", hashes: hash.NewHashSet(hash.MD5)}
	dstFs := &stubFs{name: "dstFs", hashes: hash.NewHashSet(hash.SHA1)}

	when := time.Now()
	src := &stubObject{
		parent:  srcFs,
		remote:  "no-shared-hash.bin",
		size:    42,
		modTime: when,
		hashes:  map[hash.Type]string{hash.MD5: "d41d8cd98f00b204e9800998ecf8427e"},
	}
	dst := &stubObject{
		parent:  dstFs,
		remote:  "no-shared-hash.bin",
		size:    42,
		modTime: when,
		hashes:  map[hash.Type]string{hash.SHA1: "da39a3ee5e6b4b0d3255bfef95601890afd80709"},
	}

	got := equal(ctx, src, dst, defaultEqualOpt(ctx))
	assert.True(t, got)

	const want = "no hashes in common"
	assert.True(t,
		strings.Contains(buf.String(), want),
		"expected existing no-common-hash warning %q, got log output:\n%s", want, buf.String(),
	)
}

// TestEqualChecksumHashPresentNoWarning verifies the negative case requested
// in review: when --checksum is in use, a common hash type exists, and both
// objects have a stored, matching hash, no size-only fallback warning is
// emitted (the checksum check really happened). This guards against the
// empty-hash warning firing spuriously on ordinary checksum matches.
func TestEqualChecksumHashPresentNoWarning(t *testing.T) {
	var buf bytes.Buffer
	captureLogger(t, &buf)

	checksumWarning = sync.Once{}
	emptyHashWarning = sync.Once{}

	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	ci.CheckSum = true

	srcFs := &stubFs{name: "srcFs", hashes: hash.NewHashSet(hash.MD5)}
	dstFs := &stubFs{name: "dstFs", hashes: hash.NewHashSet(hash.MD5)}

	const md5sum = "d41d8cd98f00b204e9800998ecf8427e"
	when := time.Now()
	src := &stubObject{
		parent:  srcFs,
		remote:  "normal.bin",
		size:    1024,
		modTime: when,
		hashes:  map[hash.Type]string{hash.MD5: md5sum},
	}
	dst := &stubObject{
		parent:  dstFs,
		remote:  "normal.bin",
		size:    1024,
		modTime: when,
		hashes:  map[hash.Type]string{hash.MD5: md5sum},
	}

	got := equal(ctx, src, dst, defaultEqualOpt(ctx))
	assert.True(t, got, "identical hashes should compare equal")

	logged := buf.String()
	assert.NotContains(t, logged, "falling back to --size-only",
		"no size-only fallback warning expected when a matching hash is present, got:\n%s", logged)
	assert.NotContains(t, logged, "no stored hash",
		"no empty-hash warning expected when a matching hash is present, got:\n%s", logged)
}
