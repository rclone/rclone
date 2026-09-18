package bisync

import (
	"context"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newNoCommonHashFs returns a mock Fs that supports no hash types, so that a
// pair of them has no hash in common.
func newNoCommonHashFs(ctx context.Context, t *testing.T, name string) *mockfs.Fs {
	f, err := mockfs.NewFs(ctx, name, "", nil)
	require.NoError(t, err)
	mf := f.(*mockfs.Fs)
	mf.SetHashes(hash.Set(hash.None))
	return mf
}

func newObject(f *mockfs.Fs, content string, modTime time.Time) *mockobject.ContentMockObject {
	o := mockobject.New("file.txt").WithContent([]byte(content), mockobject.SeekModeNone)
	o.SetFs(f)
	_ = o.SetModTime(context.Background(), modTime)
	return o
}

func TestWhichEqualNoCommonHash(t *testing.T) {
	ctx, ci := fs.AddConfig(context.Background())
	ci.IgnoreChecksum = true

	fs1 := newNoCommonHashFs(ctx, t, "path1")
	fs2 := newNoCommonHashFs(ctx, t, "path2")
	b := &bisyncRun{opt: &Options{Compare: CompareOpt{Size: true, Modtime: true}}}
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	for _, test := range []struct {
		name    string
		content string
		modTime time.Time
		unknown bool
		want    bool
	}{
		{name: "same size and modtime", content: "hello", modTime: now, want: true},
		{name: "modtime within precision", content: "hello", modTime: now.Add(500 * time.Millisecond), want: true},
		{name: "different size", content: "hello, world", modTime: now, want: false},
		{name: "different modtime", content: "hello", modTime: now.Add(time.Hour), want: false},
		{name: "unknown size", content: "hello", modTime: now, unknown: true, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			src := newObject(fs1, "hello", now)
			dst := newObject(fs2, test.content, test.modTime)
			dst.SetUnknownSize(test.unknown)
			assert.Equal(t, test.want, b.WhichEqual(ctx, src, dst, fs1, fs2))
		})
	}
}

func TestEqualWithoutHashRequiresOptOut(t *testing.T) {
	ctx, _ := fs.AddConfig(context.Background())

	fs1 := newNoCommonHashFs(ctx, t, "path1")
	fs2 := newNoCommonHashFs(ctx, t, "path2")
	b := &bisyncRun{opt: &Options{Compare: CompareOpt{Size: true, Modtime: true}}}
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	src := newObject(fs1, "hello", now)
	dst := newObject(fs2, "hello", now)
	assert.False(t, b.equalWithoutHash(ctx, src, dst, fs1, fs2),
		"without --ignore-checksum or --size-only, equality must be left to cryptcheck or --download")
}

func TestEqualWithoutHashSizeOnly(t *testing.T) {
	ctx, ci := fs.AddConfig(context.Background())
	ci.SizeOnly = true

	fs1 := newNoCommonHashFs(ctx, t, "path1")
	fs2 := newNoCommonHashFs(ctx, t, "path2")
	b := &bisyncRun{opt: &Options{Compare: CompareOpt{Size: true}}}
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	src := newObject(fs1, "hello", now)
	dst := newObject(fs2, "world", now.Add(time.Hour))
	assert.True(t, b.equalWithoutHash(ctx, src, dst, fs1, fs2),
		"with --size-only, only size is compared")
}
