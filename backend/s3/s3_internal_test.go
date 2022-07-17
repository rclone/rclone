package s3

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gz(t *testing.T, s string) string {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write([]byte(s))
	require.NoError(t, err)
	err = zw.Close()
	require.NoError(t, err)
	return buf.String()
}

func (f *Fs) InternalTestMetadata(t *testing.T) {
	ctx := context.Background()
	contents := gz(t, random.String(1000))

	item := fstest.NewItem("test-metadata", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
	btime := time.Now()
	metadata := fs.Metadata{
		"cache-control":       "no-cache",
		"content-disposition": "inline",
		"content-encoding":    "gzip",
		"content-language":    "en-US",
		"content-type":        "text/plain",
		"mtime":               "2009-05-06T04:05:06.499999999Z",
		// "tier" - read only
		// "btime" - read only
	}
	obj := fstests.PutTestContentsMetadata(ctx, t, f, &item, contents, true, "text/html", metadata)
	defer func() {
		assert.NoError(t, obj.Remove(ctx))
	}()
	o := obj.(*Object)
	gotMetadata, err := o.Metadata(ctx)
	require.NoError(t, err)
	for k, v := range metadata {
		got := gotMetadata[k]
		switch k {
		case "mtime":
			assert.True(t, fstest.Time(v).Equal(fstest.Time(got)))
		case "btime":
			gotBtime := fstest.Time(got)
			dt := gotBtime.Sub(btime)
			assert.True(t, dt < time.Minute && dt > -time.Minute, fmt.Sprintf("btime more than 1 minute out want %v got %v delta %v", btime, gotBtime, dt))
			assert.True(t, fstest.Time(v).Equal(fstest.Time(got)))
		case "tier":
			assert.NotEqual(t, "", got)
		default:
			assert.Equal(t, v, got, k)
		}
	}
}

func (f *Fs) InternalTestNoHead(t *testing.T) {
	ctx := context.Background()
	// Set NoHead for this test
	f.opt.NoHead = true
	defer func() {
		f.opt.NoHead = false
	}()
	contents := random.String(1000)
	item := fstest.NewItem("test-no-head", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
	obj := fstests.PutTestContents(ctx, t, f, &item, contents, true)
	defer func() {
		assert.NoError(t, obj.Remove(ctx))
	}()
	// PutTestcontests checks the received object

}

func (f *Fs) InternalTest(t *testing.T) {
	t.Run("Metadata", f.InternalTestMetadata)
	t.Run("NoHead", f.InternalTestNoHead)
}

var _ fstests.InternalTester = (*Fs)(nil)
