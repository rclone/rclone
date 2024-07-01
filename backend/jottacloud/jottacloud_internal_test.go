package jottacloud

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/random"
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

func (f *Fs) InternalTestMetadata(t *testing.T) {
	ctx := context.Background()
	contents := random.String(1000)

	item := fstest.NewItem("test-metadata", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
	utime := time.Now()
	metadata := fs.Metadata{
		"btime": "2009-05-06T04:05:06.499999999Z",
		"mtime": "2010-06-07T08:09:07.599999999Z",
		//"utime" - read-only
		//"content-type" - read-only
	}
	obj := fstests.PutTestContentsMetadata(ctx, t, f, &item, false, contents, true, "text/html", metadata)
	defer func() {
		assert.NoError(t, obj.Remove(ctx))
	}()
	o := obj.(*Object)
	gotMetadata, err := o.Metadata(ctx)
	require.NoError(t, err)
	for k, v := range metadata {
		got := gotMetadata[k]
		switch k {
		case "btime":
			assert.True(t, fstest.Time(v).Truncate(f.Precision()).Equal(fstest.Time(got)), fmt.Sprintf("btime not equal want %v got %v", v, got))
		case "mtime":
			assert.True(t, fstest.Time(v).Truncate(f.Precision()).Equal(fstest.Time(got)), fmt.Sprintf("btime not equal want %v got %v", v, got))
		case "utime":
			gotUtime := fstest.Time(got)
			dt := gotUtime.Sub(utime)
			assert.True(t, dt < time.Minute && dt > -time.Minute, fmt.Sprintf("utime more than 1 minute out want %v got %v delta %v", utime, gotUtime, dt))
			assert.True(t, fstest.Time(v).Equal(fstest.Time(got)))
		case "content-type":
			assert.True(t, o.MimeType(ctx) == got)
		default:
			assert.Equal(t, v, got, k)
		}
	}
}

func (f *Fs) InternalTest(t *testing.T) {
	t.Run("Metadata", f.InternalTestMetadata)
}

var _ fstests.InternalTester = (*Fs)(nil)
