package fs_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
)

func TestFingerprint(t *testing.T) {
	ctx := context.Background()
	f := mockfs.NewFs("test", "root")
	f.SetHashes(hash.NewHashSet(hash.MD5))

	for i, test := range []struct {
		fast        bool
		slowModTime bool
		slowHash    bool
		want        string
	}{
		{fast: false, slowModTime: false, slowHash: false, want: "4,0001-01-01 00:00:00 +0000 UTC,8d777f385d3dfec8815d20f7496026dc"},
		{fast: false, slowModTime: false, slowHash: true, want: "4,0001-01-01 00:00:00 +0000 UTC,8d777f385d3dfec8815d20f7496026dc"},
		{fast: false, slowModTime: true, slowHash: false, want: "4,0001-01-01 00:00:00 +0000 UTC,8d777f385d3dfec8815d20f7496026dc"},
		{fast: false, slowModTime: true, slowHash: true, want: "4,0001-01-01 00:00:00 +0000 UTC,8d777f385d3dfec8815d20f7496026dc"},
		{fast: true, slowModTime: false, slowHash: false, want: "4,0001-01-01 00:00:00 +0000 UTC,8d777f385d3dfec8815d20f7496026dc"},
		{fast: true, slowModTime: false, slowHash: true, want: "4,0001-01-01 00:00:00 +0000 UTC"},
		{fast: true, slowModTime: true, slowHash: false, want: "4,8d777f385d3dfec8815d20f7496026dc"},
		{fast: true, slowModTime: true, slowHash: true, want: "4"},
	} {
		what := fmt.Sprintf("#%d fast=%v,slowModTime=%v,slowHash=%v", i, test.fast, test.slowModTime, test.slowHash)
		o := mockobject.New("potato").WithContent([]byte("data"), mockobject.SeekModeRegular)
		o.SetFs(f)
		f.Features().SlowModTime = test.slowModTime
		f.Features().SlowHash = test.slowHash
		got := fs.Fingerprint(ctx, o, test.fast)
		assert.Equal(t, test.want, got, what)
	}
}
