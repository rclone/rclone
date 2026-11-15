package fs_test

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeaturesMaskNoMultiThreading(t *testing.T) {
	ctx := context.Background()
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
		f.Features().NoMultiThreading = test.wrapped
		ft := &fs.Features{NoMultiThreading: test.wrapper}
		ft.Mask(ctx, f)
		assert.Equal(t, test.want, ft.NoMultiThreading, "wrapper=%v wrapped=%v", test.wrapper, test.wrapped)
	}
}
