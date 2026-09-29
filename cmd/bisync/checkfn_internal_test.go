package bisync

import (
	"context"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blankHashObject supports MD5 through its Fs but returns a blank hash
type blankHashObject struct {
	*mockobject.ContentMockObject
}

func (o blankHashObject) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", nil
}

func newBlankHashObject(ctx context.Context, t *testing.T, fsName, content string) blankHashObject {
	f, err := mockfs.NewFs(ctx, fsName, "", nil)
	require.NoError(t, err)
	f.(*mockfs.Fs).SetHashes(hash.NewHashSet(hash.MD5))
	o := mockobject.New("file.txt").WithContent([]byte(content), mockobject.SeekModeNone)
	o.SetFs(f)
	return blankHashObject{o}
}

func TestCheckFnMissingHash(t *testing.T) {
	for _, test := range []struct {
		name       string
		content    string
		flag       func(ci *fs.ConfigInfo)
		wantDiffer bool
		wantNoHash bool
	}{
		{name: "identical", content: "hello"},
		{name: "same size, different content", content: "HELLO", wantDiffer: true},
		{name: "size-only", content: "HELLO", flag: func(ci *fs.ConfigInfo) { ci.SizeOnly = true }, wantNoHash: true},
		{name: "ignore-checksum", content: "HELLO", flag: func(ci *fs.ConfigInfo) { ci.IgnoreChecksum = true }, wantNoHash: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, ci := fs.AddConfig(context.Background())
			if test.flag != nil {
				test.flag(ci)
			}
			src := newBlankHashObject(ctx, t, "path1", "hello")
			dst := newBlankHashObject(ctx, t, "path2", test.content)
			differ, noHash, err := CheckFn(ctx, dst, src)
			require.NoError(t, err)
			assert.Equal(t, test.wantDiffer, differ, "differ")
			assert.Equal(t, test.wantNoHash, noHash, "noHash")
		})
	}
}

func newEmptyFs(ctx context.Context, t *testing.T, fsName string, hashes hash.Set) fs.Fs {
	f, err := mockfs.NewFs(ctx, fsName, "", nil)
	require.NoError(t, err)
	f.(*mockfs.Fs).SetHashes(hashes)
	return f
}

func TestCheckConflictsSkipsCheck(t *testing.T) {
	md5 := hash.NewHashSet(hash.MD5)
	none := hash.NewHashSet()
	for _, test := range []struct {
		name     string
		hashes2  hash.Set
		flag     func(ci *fs.ConfigInfo)
		wantSkip bool
	}{
		{name: "common hash", hashes2: md5},
		{name: "no common hash", hashes2: none},
		{name: "size-only with a common hash", hashes2: md5, flag: func(ci *fs.ConfigInfo) { ci.SizeOnly = true }, wantSkip: true},
		{name: "size-only without a common hash", hashes2: none, flag: func(ci *fs.ConfigInfo) { ci.SizeOnly = true }, wantSkip: true},
		{name: "ignore-checksum with a common hash", hashes2: md5, flag: func(ci *fs.ConfigInfo) { ci.IgnoreChecksum = true }},
		{name: "ignore-checksum without a common hash", hashes2: none, flag: func(ci *fs.ConfigInfo) { ci.IgnoreChecksum = true }, wantSkip: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, ci := fs.AddConfig(context.Background())
			if test.flag != nil {
				test.flag(ci)
			}
			ctx, fi := filter.AddConfig(ctx)
			require.NoError(t, fi.AddFile("file.txt"))

			b := &bisyncRun{opt: &Options{}}
			b.opt.Compare.Size = true
			b.opt.Compare.Modtime = true
			modtime := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
			b.march.ls1, b.march.ls2 = newFileList(), newFileList()
			b.march.ls1.put("file.txt", 5, modtime, "", "-", "-")
			b.march.ls2.put("file.txt", 5, modtime, "", "-", "-")

			fs1 := newEmptyFs(ctx, t, "path1", md5)
			fs2 := newEmptyFs(ctx, t, "path2", test.hashes2)
			matches, matches1, matches2, err := b.checkconflicts(ctx, fi, fs1, fs2)
			require.NoError(t, err)

			assert.Equal(t, test.wantSkip, matches.Has("file.txt"), "matched a file that is on neither path")
			if test.wantSkip {
				assert.True(t, b.sameCompared(b.march.ls1, matches1, "file.txt", fs1), "Path1 differs from the listing")
				assert.True(t, b.sameCompared(b.march.ls2, matches2, "file.txt", fs2), "Path2 differs from the listing")
			} else {
				assert.False(t, matches1.has("file.txt"), "Path1 checked")
				assert.False(t, matches2.has("file.txt"), "Path2 checked")
			}
		})
	}
}
