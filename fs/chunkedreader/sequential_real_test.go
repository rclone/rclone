package chunkedreader_test

import (
	"bytes"
	"context"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	httpbackend "github.com/rclone/rclone/backend/http"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/chunkedreader"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/require"
)

func TestSequentialRealFileEOF(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		size, offset, remaining, length int
		truncate                        bool
	}{
		{name: "empty", size: 0},
		{name: "short-final-chunk", size: 1500},
		{name: "exact-chunk", size: 1024},
		{name: "multiple-chunks", size: 3073},
		{name: "seek", size: 3073, offset: 513},
		{name: "range-first-chunk", size: 1500, offset: 127, length: 31},
		{name: "range-exact-end", size: 1500, offset: 1470, length: 30},
		{name: "range-past-end", size: 1500, offset: 1470, length: 64},
		{name: "premature-first-boundary", size: 4096, truncate: true, remaining: 1024},
		{name: "premature-second-chunk", size: 4096, truncate: true, remaining: 1200},
		{name: "premature-second-boundary", size: 4096, truncate: true, remaining: 3072},
		{name: "premature-third-chunk", size: 8192, truncate: true, remaining: 4096},
		{name: "premature-seek-boundary", size: 4096, offset: 100, truncate: true, remaining: 1124},
		{name: "premature-range-boundary", size: 4096, offset: 100, length: 31, truncate: true, remaining: 131},
		{name: "premature-after-range", size: 4096, offset: 100, length: 31, truncate: true, remaining: 1200},
		{name: "premature-zero", size: 1500, truncate: true, remaining: 0},
		{name: "premature-partial", size: 1500, truncate: true, remaining: 127},
		{name: "premature-after-seek", size: 1500, offset: 100, truncate: true, remaining: 227},
		{name: "premature-short-range", size: 1500, offset: 100, length: 31, truncate: true, remaining: 127},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			payload := []byte(random.StringFn(tc.size, rand.New(rand.NewSource(42))))
			path := filepath.Join(dir, "payload")
			require.NoError(t, os.WriteFile(path, payload, 0600))
			f, err := fs.NewFs(ctx, dir)
			require.NoError(t, err)
			obj, err := f.NewObject(ctx, "payload")
			require.NoError(t, err)
			reader := chunkedreader.New(ctx, obj, 1024, 4096, 0)
			t.Cleanup(func() { require.NoError(t, reader.Close()) })
			if tc.length > 0 {
				offset, err := reader.RangeSeek(ctx, int64(tc.offset), io.SeekStart, int64(tc.length))
				require.NoError(t, err)
				require.Equal(t, int64(tc.offset), offset)
			} else if tc.offset != 0 {
				offset, err := reader.Seek(int64(tc.offset), io.SeekStart)
				require.NoError(t, err)
				require.Equal(t, int64(tc.offset), offset)
			}
			_, err = reader.Open()
			require.NoError(t, err)
			// Truncate after opening so metadata initially advertises the full size.
			if tc.truncate {
				require.NoError(t, os.Truncate(path, int64(tc.remaining)))
			}
			actual, err := io.ReadAll(reader)
			end := tc.size
			if tc.truncate {
				end = tc.remaining
				require.ErrorIs(t, err, io.ErrUnexpectedEOF)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, payload[tc.offset:end], actual)
		})
	}
}

func TestSequentialHTTPSizeRefresh(t *testing.T) {
	for _, noHead := range []string{"false", "true"} {
		t.Run("no-head="+noHead, func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			payload := []byte(random.StringFn(4096, rand.New(rand.NewSource(42))))
			path := filepath.Join(dir, "payload")
			require.NoError(t, os.WriteFile(path, payload, 0600))
			server := httptest.NewServer(http.FileServer(http.Dir(dir)))
			t.Cleanup(server.Close)
			f, err := httpbackend.NewFs(ctx, "test", "", configmap.Simple{
				"url": server.URL + "/", "no_head": noHead,
			})
			require.NoError(t, err)
			obj, err := f.NewObject(ctx, "payload")
			require.NoError(t, err)
			if noHead == "true" {
				require.Equal(t, int64(-1), obj.Size())
			} else {
				require.Equal(t, int64(len(payload)), obj.Size())
			}
			reader := chunkedreader.New(ctx, obj, 1024, 4096, 0)
			_, err = reader.Open()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, reader.Close()) })
			require.Equal(t, int64(len(payload)), obj.Size())
			first := make([]byte, 1024)
			_, err = io.ReadFull(reader, first)
			require.NoError(t, err)
			require.Equal(t, payload[:1024], first)

			// The next real range response advertises a smaller total size;
			// it must not erase the length learned from HEAD or the first GET.
			require.NoError(t, os.Truncate(path, 1200))
			var rest bytes.Buffer
			n, err := io.Copy(&rest, reader)
			require.ErrorIs(t, err, io.ErrUnexpectedEOF)
			require.Equal(t, int64(176), n)
			require.Equal(t, payload[1024:1200], rest.Bytes())
			require.Equal(t, int64(1200), obj.Size())
		})
	}
}
