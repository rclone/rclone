package bisync

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs"
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
