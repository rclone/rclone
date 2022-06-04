package chunksize

import (
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
)

func TestComputeChunkSize(t *testing.T) {
	tests := map[string]struct {
		fileSize         fs.SizeSuffix
		maxParts         int
		defaultChunkSize fs.SizeSuffix
		expected         fs.SizeSuffix
	}{
		"default size returned when file size is small enough":             {fileSize: 1000, maxParts: 10000, defaultChunkSize: toSizeSuffixMiB(10), expected: toSizeSuffixMiB(10)},
		"default size returned when file size is just 1 byte small enough": {fileSize: toSizeSuffixMiB(100000) - 1, maxParts: 10000, defaultChunkSize: toSizeSuffixMiB(10), expected: toSizeSuffixMiB(10)},
		"no rounding up when everything divides evenly":                    {fileSize: toSizeSuffixMiB(1000000), maxParts: 10000, defaultChunkSize: toSizeSuffixMiB(100), expected: toSizeSuffixMiB(100)},
		"rounding up to nearest MiB when not quite enough parts":           {fileSize: toSizeSuffixMiB(1000000), maxParts: 9999, defaultChunkSize: toSizeSuffixMiB(100), expected: toSizeSuffixMiB(101)},
		"rounding up to nearest MiB when one extra byte":                   {fileSize: toSizeSuffixMiB(1000000) + 1, maxParts: 10000, defaultChunkSize: toSizeSuffixMiB(100), expected: toSizeSuffixMiB(101)},
		"expected MiB value when rounding sets to absolute minimum":        {fileSize: toSizeSuffixMiB(1) - 1, maxParts: 1, defaultChunkSize: toSizeSuffixMiB(1), expected: toSizeSuffixMiB(1)},
		"expected MiB value when rounding to absolute min with extra":      {fileSize: toSizeSuffixMiB(1) + 1, maxParts: 1, defaultChunkSize: toSizeSuffixMiB(1), expected: toSizeSuffixMiB(2)},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			src := object.NewStaticObjectInfo("mock", time.Now(), int64(tc.fileSize), true, nil, nil)
			result := Calculator(src, tc.maxParts, tc.defaultChunkSize)
			if result != tc.expected {
				t.Fatalf("expected: %v, got: %v", tc.expected, result)
			}
		})
	}
}

func toSizeSuffixMiB(size int64) fs.SizeSuffix {
	return fs.SizeSuffix(size * int64(fs.Mebi))
}
