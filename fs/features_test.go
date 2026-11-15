package fs_test

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFeaturesMaskRestrictions checks flags which restrict what can be
// done are kept if either Fs sets them.
func TestFeaturesMaskRestrictions(t *testing.T) {
	ctx := context.Background()
	for _, flag := range []struct {
		name string
		get  func(*fs.Features) *bool
	}{
		{"NoMultiThreading", func(ft *fs.Features) *bool { return &ft.NoMultiThreading }},
		{"SlowModTime", func(ft *fs.Features) *bool { return &ft.SlowModTime }},
	} {
		for _, test := range []struct {
			wrapper, wrapped, want bool
		}{
			{false, false, false},
			{false, true, true},
			{true, false, true},
			{true, true, true},
		} {
			f, err := mockfs.NewFs(ctx, "test", "root", nil)
			require.NoError(t, err)
			*flag.get(f.Features()) = test.wrapped
			ft := &fs.Features{}
			*flag.get(ft) = test.wrapper
			ft.Mask(ctx, f)
			assert.Equal(t, test.want, *flag.get(ft), "%s wrapper=%v wrapped=%v", flag.name, test.wrapper, test.wrapped)
		}
	}
}
