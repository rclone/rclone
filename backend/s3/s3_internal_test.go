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
	"github.com/rclone/rclone/lib/version"
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

func (f *Fs) InternalTestVersions(t *testing.T) {
	ctx := context.Background()

	// Enable versioning for this bucket during this test
	_, err := f.setGetVersioning(ctx, "Enabled")
	if err != nil {
		t.Skipf("Couldn't enable versioning: %v", err)
	}
	defer func() {
		// Disable versioning for this bucket
		_, err := f.setGetVersioning(ctx, "Suspended")
		assert.NoError(t, err)
	}()

	// Create an object
	const fileName = "test-versions.txt"
	contents := random.String(100)
	item := fstest.NewItem(fileName, contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
	obj := fstests.PutTestContents(ctx, t, f, &item, contents, true)
	defer func() {
		assert.NoError(t, obj.Remove(ctx))
	}()

	// Remove it
	assert.NoError(t, obj.Remove(ctx))

	// And create it with different size and contents
	newContents := random.String(101)
	newItem := fstest.NewItem(fileName, newContents, fstest.Time("2002-05-06T04:05:06.499999999Z"))
	_ = fstests.PutTestContents(ctx, t, f, &newItem, newContents, true)

	// Add the expected version suffix to the old version
	item.Path = version.Add(item.Path, obj.(*Object).lastModified)

	t.Run("S3Version", func(t *testing.T) {
		// Set --s3-versions for this test
		f.opt.Versions = true
		defer func() {
			f.opt.Versions = false
		}()

		// Check listing
		items := append([]fstest.Item{item, newItem}, fstests.InternalTestFiles...)
		fstest.CheckListing(t, f, items)

		// Read the contents
		entries, err := f.List(ctx, "")
		require.NoError(t, err)
		tests := 0
		for _, entry := range entries {
			switch entry.Remote() {
			case newItem.Path:
				t.Run("ReadCurrent", func(t *testing.T) {
					assert.Equal(t, newContents, fstests.ReadObject(ctx, t, entry.(fs.Object), -1))
				})
				tests++
			case item.Path:
				t.Run("ReadVersion", func(t *testing.T) {
					assert.Equal(t, contents, fstests.ReadObject(ctx, t, entry.(fs.Object), -1))
				})
				tests++
			}
		}
		assert.Equal(t, 2, tests)

		// Check we can read the object with a version suffix
		t.Run("NewObject", func(t *testing.T) {
			o, err := f.NewObject(ctx, item.Path)
			require.NoError(t, err)
			require.NotNil(t, o)
			assert.Equal(t, int64(100), o.Size(), o.Remote())
		})
	})
}

func (f *Fs) InternalTest(t *testing.T) {
	t.Run("Metadata", f.InternalTestMetadata)
	t.Run("NoHead", f.InternalTestNoHead)
	t.Run("Versions", f.InternalTestVersions)
}

var _ fstests.InternalTester = (*Fs)(nil)
