package jottacloud

import (
	"crypto/md5"
	"fmt"
	"io"
	"testing"

	"github.com/rclone/rclone/lib/readers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadMD5(t *testing.T) {
	// Check readMD5 for different size and threshold
	for _, size := range []int64{0, 1024, 10 * 1024, 100 * 1024} {
		t.Run(fmt.Sprintf("%d", size), func(t *testing.T) {
			hasher := md5.New()
			n, err := io.Copy(hasher, readers.NewPatternReader(size))
			require.NoError(t, err)
			assert.Equal(t, n, size)
			wantMD5 := fmt.Sprintf("%x", hasher.Sum(nil))
			for _, threshold := range []int64{512, 1024, 10 * 1024, 20 * 1024} {
				t.Run(fmt.Sprintf("%d", threshold), func(t *testing.T) {
					in := readers.NewPatternReader(size)
					gotMD5, out, cleanup, err := readMD5(in, size, threshold)
					defer cleanup()
					require.NoError(t, err)
					assert.Equal(t, wantMD5, gotMD5)

					// check md5hash of out
					hasher := md5.New()
					n, err := io.Copy(hasher, out)
					require.NoError(t, err)
					assert.Equal(t, n, size)
					outMD5 := fmt.Sprintf("%x", hasher.Sum(nil))
					assert.Equal(t, wantMD5, outMD5)
				})
			}
		})
	}
}
