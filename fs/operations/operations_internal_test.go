// Internal tests for operations

package operations

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
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

func TestSizeString(t *testing.T) {
	for _, test := range []struct {
		size          int64
		humanReadable bool
		want          string
	}{
		{-1, true, "-"},
		{-1, false, "-"},
		{0, true, "0"},
		{0, false, "0"},
		{1024, true, "1Ki"},
		{1024, false, "1024"},
		{1048576, true, "1Mi"},
		{1048576, false, "1048576"},
		{-2, true, "-2"},
		{-2, false, "-2"},
		{-1024, true, "-1Ki"},
		{-1024, false, "-1024"},
	} {
		got := SizeString(test.size, test.humanReadable)
		assert.Equal(t, test.want, got, fmt.Sprintf("size=%v, humanReadable=%v", test.size, test.humanReadable))
	}
}

func TestCountString(t *testing.T) {
	for _, test := range []struct {
		count         int64
		humanReadable bool
		want          string
	}{
		{-1, true, "-"},
		{-1, false, "-"},
		{0, true, "0"},
		{0, false, "0"},
		{1000, true, "1k"},
		{1000, false, "1000"},
		{1000000, true, "1M"},
		{1000000, false, "1000000"},
		{-2, true, "-2"},
		{-2, false, "-2"},
		{-1000, true, "-1k"},
		{-1000, false, "-1000"},
	} {
		got := CountString(test.count, test.humanReadable)
		assert.Equal(t, test.want, got, fmt.Sprintf("count=%v, humanReadable=%v", test.count, test.humanReadable))
	}
}

func TestSizeStringField(t *testing.T) {
	for _, test := range []struct {
		size          int64
		humanReadable bool
		rawWidth      int
		want          string
	}{
		{-1, true, 12, "        -"},
		{-1, false, 12, "           -"},
		{0, true, 12, "        0"},
		{0, false, 12, "           0"},
		{1024, true, 12, "      1Ki"},
		{1024, false, 12, "        1024"},
	} {
		got := SizeStringField(test.size, test.humanReadable, test.rawWidth)
		assert.Equal(t, test.want, got, fmt.Sprintf("size=%v, humanReadable=%v, rawWidth=%v", test.size, test.humanReadable, test.rawWidth))
	}
}

func TestCountStringField(t *testing.T) {
	for _, test := range []struct {
		count         int64
		humanReadable bool
		rawWidth      int
		want          string
	}{
		{-1, true, 9, "       -"},
		{-1, false, 9, "        -"},
		{0, true, 9, "       0"},
		{0, false, 9, "        0"},
		{1000, true, 9, "      1k"},
		{1000, false, 9, "     1000"},
	} {
		got := CountStringField(test.count, test.humanReadable, test.rawWidth)
		assert.Equal(t, test.want, got, fmt.Sprintf("count=%v, humanReadable=%v, rawWidth=%v", test.count, test.humanReadable, test.rawWidth))
	}
}
