package chunksize

import (
	"testing"

	"github.com/rclone/rclone/fs"
)

func TestComputeChunkSize(t *testing.T) {
	for _, test := range []struct {
		name             string
		size             fs.SizeSuffix
		maxParts         int
		defaultChunkSize fs.SizeSuffix
		want             fs.SizeSuffix
	}{
		{
			name:             "streaming file",
			size:             -1,
			maxParts:         10000,
			defaultChunkSize: toSizeSuffixMiB(10),
			want:             toSizeSuffixMiB(10),
		}, {
			name:             "default size returned when file size is small enough",
			size:             1000,
			maxParts:         10000,
			defaultChunkSize: toSizeSuffixMiB(10),
			want:             toSizeSuffixMiB(10),
		}, {
			name:             "default size returned when file size is just 1 byte small enough",
			size:             toSizeSuffixMiB(100000) - 1,
			maxParts:         10000,
			defaultChunkSize: toSizeSuffixMiB(10),
			want:             toSizeSuffixMiB(10),
		}, {
			name:             "no rounding up when everything divides evenly",
			size:             toSizeSuffixMiB(1000000),
			maxParts:         10000,
			defaultChunkSize: toSizeSuffixMiB(100),
			want:             toSizeSuffixMiB(100),
		}, {
			name:             "rounding up to nearest MiB when not quite enough parts",
			size:             toSizeSuffixMiB(1000000),
			maxParts:         9999,
			defaultChunkSize: toSizeSuffixMiB(100),
			want:             toSizeSuffixMiB(101),
		}, {
			name:             "rounding up to nearest MiB when one extra byte",
			size:             toSizeSuffixMiB(1000000) + 1,
			maxParts:         10000,
			defaultChunkSize: toSizeSuffixMiB(100),
			want:             toSizeSuffixMiB(101),
		}, {
			name:             "expected MiB value when rounding sets to absolute minimum",
			size:             toSizeSuffixMiB(1) - 1,
			maxParts:         1,
			defaultChunkSize: toSizeSuffixMiB(1),
			want:             toSizeSuffixMiB(1),
		}, {
			name:             "expected MiB value when rounding to absolute min with extra",
			size:             toSizeSuffixMiB(1) + 1,
			maxParts:         1,
			defaultChunkSize: toSizeSuffixMiB(1),
			want:             toSizeSuffixMiB(2),
		}, {
			name:             "issue from forum #1",
			size:             120864818840,
			maxParts:         10000,
			defaultChunkSize: 5 * 1024 * 1024,
			want:             toSizeSuffixMiB(12),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := Calculator(test.name, int64(test.size), test.maxParts, test.defaultChunkSize)
			if got != test.want {
				t.Fatalf("expected: %v, got: %v", test.want, got)
			}
			if test.size < 0 {
				return
			}
			parts := func(result fs.SizeSuffix) int {
				n := test.size / result
				r := test.size % result
				if r != 0 {
					n++
				}
				return int(n)
			}
			// Check this gives the parts in range
			if parts(got) > test.maxParts {
				t.Fatalf("too many parts %d", parts(got))
			}
			// Check that setting chunk size smaller gave too many parts
			if got > test.defaultChunkSize {
				if parts(got-toSizeSuffixMiB(1)) <= test.maxParts {
					t.Fatalf("chunk size %v too big as %v only gives %d parts", got, got-toSizeSuffixMiB(1), parts(got-toSizeSuffixMiB(1)))
				}

			}
		})
	}
}

func toSizeSuffixMiB(size int64) fs.SizeSuffix {
	return fs.SizeSuffix(size * int64(fs.Mebi))
}
