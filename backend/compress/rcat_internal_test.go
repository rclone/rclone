package compress

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/memory"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/lib/pool"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// putRecorder wraps an fs.Fs and records what rcat hands to Put and
// PutStream. It reads each body twice when it is seekable to model a
// backend retrying an upload.
type putRecorder struct {
	fs.Fs
	t        *testing.T
	stream   bool // expose PutStream from the wrapped Fs
	size     int64
	seekable bool
	body     []byte
	streamed bool
}

func (r *putRecorder) Features() *fs.Features {
	features := *r.Fs.Features()
	features.PutStream = nil
	if r.stream {
		features.PutStream = r.PutStream
	}
	return &features
}

func (r *putRecorder) record(in io.Reader, src fs.ObjectInfo) []byte {
	r.size = src.Size()
	body, err := io.ReadAll(in)
	require.NoError(r.t, err)
	if seeker, ok := in.(io.Seeker); ok {
		r.seekable = true
		_, err = seeker.Seek(0, io.SeekStart)
		require.NoError(r.t, err)
		again, err := io.ReadAll(in)
		require.NoError(r.t, err)
		assert.Equal(r.t, body, again, "retried body should be identical")
	}
	r.body = body
	return body
}

func (r *putRecorder) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	body := r.record(in, src)
	return r.Fs.Put(ctx, bytes.NewReader(body), object.NewStaticObjectInfo(src.Remote(), src.ModTime(ctx), int64(len(body)), true, nil, r.Fs), options...)
}

func (r *putRecorder) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	r.streamed = true
	return r.Put(ctx, in, src, options...)
}

// TestRcat checks the three upload paths rcat takes: small files buffered
// in memory and uploaded with a known size and a seekable body, large
// files streamed when the wrapped remote supports it, and large files
// spooled to a temporary file when it doesn't. Pool pages taken for the
// buffer must be returned on every path.
func TestRcat(t *testing.T) {
	ctx := context.Background()
	const limit = 1024
	for _, test := range []struct {
		name     string
		size     int
		stream   bool
		wantSize int64
	}{
		{name: "Small", size: limit / 2, stream: true, wantSize: limit / 2},
		{name: "Empty", size: 0, stream: true, wantSize: 0},
		{name: "ExactLimit", size: limit, stream: true, wantSize: -1},
		{name: "LargeStream", size: 3 * limit, stream: true, wantSize: -1},
		{name: "LargeSpool", size: 3 * limit, stream: false, wantSize: 3 * limit},
	} {
		t.Run(test.name, func(t *testing.T) {
			remote, err := fs.NewFs(ctx, ":memory:rcat-"+test.name)
			require.NoError(t, err)
			rec := &putRecorder{Fs: remote, t: t, stream: test.stream}
			f := &Fs{Fs: rec}
			f.opt.RAMCacheLimit = limit
			content := []byte(random.String(test.size))
			inUse := pool.Global().InUse()

			o, err := f.rcat(ctx, "file", io.NopCloser(bytes.NewReader(content)), time.Now(), nil)
			require.NoError(t, err)
			assert.Equal(t, inUse, pool.Global().InUse(), "pool pages not returned")

			assert.Equal(t, test.wantSize, rec.size)
			assert.Equal(t, content, rec.body)
			assert.Equal(t, test.stream && test.size >= limit, rec.streamed)
			if test.size < limit {
				assert.True(t, rec.seekable, "small file body should be seekable")
			}
			rc, err := o.Open(ctx)
			require.NoError(t, err)
			got, err := io.ReadAll(rc)
			require.NoError(t, err)
			require.NoError(t, rc.Close())
			assert.Equal(t, content, got)
		})
	}
}

// TestIsCompressible checks the compressibility heuristic on data at
// both ends of the scale for every compression mode which has one.
func TestIsCompressible(t *testing.T) {
	compressible := bytes.Repeat([]byte("compress me "), 4096)
	incompressible := make([]byte, len(compressible))
	_, err := io.ReadFull(rand.Reader, incompressible)
	require.NoError(t, err)
	for name, handler := range map[string]compressionModeHandler{
		"gzip": &gzipModeHandler{},
		"zstd": &zstdModeHandler{},
	} {
		t.Run(name, func(t *testing.T) {
			ok, err := handler.isCompressible(bytes.NewReader(compressible), 0)
			require.NoError(t, err)
			assert.True(t, ok)
			ok, err = handler.isCompressible(bytes.NewReader(incompressible), 0)
			require.NoError(t, err)
			assert.False(t, ok)
		})
	}
}
