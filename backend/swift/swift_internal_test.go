package swift

import (
	"context"
	"maps"
	"testing"
	"time"

	"github.com/ncw/swift/v2"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/stretchr/testify/assert"
)

func TestInternalUrlEncode(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{"abcdefghijklmnopqrstuvwxyz", "abcdefghijklmnopqrstuvwxyz"},
		{"ABCDEFGHIJKLMNOPQRSTUVWXYZ", "ABCDEFGHIJKLMNOPQRSTUVWXYZ"},
		{"0123456789", "0123456789"},
		{"abc/ABC/123", "abc/ABC/123"},
		{"   ", "%20%20%20"},
		{"&", "%26"},
		{"ß£", "%C3%9F%C2%A3"},
		{"Vidéo Potato Sausage?&£.mkv", "Vid%C3%A9o%20Potato%20Sausage%3F%26%C2%A3.mkv"},
	} {
		got := urlEncode(test.in)
		if got != test.want {
			t.Logf("%q: want %q got %q", test.in, test.want, got)
		}
	}
}

func TestInternalMetadataToHeaders(t *testing.T) {
	modTime := time.Date(2002, 2, 3, 4, 5, 6, 0, time.UTC)

	t.Run("UserMetadata", func(t *testing.T) {
		options := metadataToHeaders(fs.Metadata{
			"potato":      "jersey",
			"Foo_Bar":     "baz",
			"encrypttype": "SM4",
		})
		metadata := options.headers.ObjectMetadata()
		assert.Equal(t, "jersey", metadata["potato"])
		assert.Equal(t, "baz", metadata["foo_bar"])
		assert.Equal(t, "SM4", metadata["encrypttype"])
	})

	t.Run("SystemAndIgnoredMetadata", func(t *testing.T) {
		options := metadataToHeaders(fs.Metadata{
			"btime":               modTime.Format(time.RFC3339Nano),
			"cache-control":       "no-cache",
			"content-disposition": "inline",
			"content-encoding":    "gzip",
			"content-language":    "en-US",
			"content-type":        "text/plain",
			"mtime":               modTime.Format(time.RFC3339Nano),
			"tier":                "STANDARD",
		})
		assert.Equal(t, "text/plain", options.contentType)
		assert.Equal(t, swift.TimeToFloatString(modTime), options.headers.ObjectMetadata()["mtime"])
		assert.Equal(t, "no-cache", options.headers["Cache-Control"])
		assert.Equal(t, "inline", options.headers["Content-Disposition"])
		// content-encoding is read only as rclone can't read gzip encoded
		// objects back
		assert.NotContains(t, options.headers, "Content-Encoding")
		assert.Equal(t, "en-US", options.headers["Content-Language"])
	})

	t.Run("Invalid", func(t *testing.T) {
		options := metadataToHeaders(fs.Metadata{
			"bad key": "value",
			"badval":  "line\nbreak",
			"ok":      "value",
		})
		metadata := options.headers.ObjectMetadata()
		assert.NotContains(t, metadata, "bad key")
		assert.NotContains(t, metadata, "badval")
		assert.Equal(t, "value", metadata["ok"])
	})

	t.Run("InvalidSystemMetadata", func(t *testing.T) {
		options := metadataToHeaders(fs.Metadata{
			"cache-control": "bad\nvalue",
			"content-type":  "bad\nvalue",
		})
		assert.NotContains(t, options.headers, "Cache-Control")
		assert.Empty(t, options.contentType)
	})
}

func TestInternalMergeObjectHeaders(t *testing.T) {
	src := swift.Headers{
		"Content-Type":        "text/plain",
		"X-Delete-At":         "123",
		"X-Object-Meta-Foo":   "old",
		"X-Object-Meta-Btime": "old",
	}
	metadata := swift.Headers{
		"Content-Type":      "application/octet-stream",
		"X-Object-Meta-Foo": "new",
	}

	merged := mergeObjectHeaders(src, metadata)
	assert.Equal(t, "application/octet-stream", merged["Content-Type"])
	assert.Equal(t, "123", merged["X-Delete-At"])
	assert.Equal(t, "new", merged["X-Object-Meta-Foo"])
	assert.Equal(t, "old", merged["X-Object-Meta-Btime"])
	assert.Equal(t, "old", src["X-Object-Meta-Foo"])
}

func TestInternalWithoutObjectMetadata(t *testing.T) {
	src := swift.Headers{
		"Content-Type":        "text/plain",
		"X-Delete-At":         "123",
		"X-Object-Meta-Foo":   "bar",
		"X-Object-Meta-Btime": "old",
	}

	filtered := withoutObjectMetadata(src)
	assert.Equal(t, "text/plain", filtered["Content-Type"])
	assert.Equal(t, "123", filtered["X-Delete-At"])
	assert.NotContains(t, filtered, "X-Object-Meta-Foo")
	assert.NotContains(t, filtered, "X-Object-Meta-Btime")
	// the source headers must not be modified
	assert.Equal(t, "bar", src["X-Object-Meta-Foo"])
}

func TestInternalObjectMetadata(t *testing.T) {
	modTime := time.Date(2002, 2, 3, 4, 5, 6, 0, time.UTC)
	o := &Object{
		contentType: "text/plain",
		headers: swift.Headers{
			"Content-Type":      "text/plain",
			"Cache-Control":     "no-cache",
			"X-Object-Meta-Foo": "bar",
		},
	}
	meta := swift.Metadata{}
	meta.SetModTime(modTime)
	maps.Copy(o.headers, meta.ObjectHeaders())

	metadata, err := o.Metadata(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "bar", metadata["foo"])
	assert.Equal(t, "no-cache", metadata["cache-control"])
	assert.Equal(t, "text/plain", metadata["content-type"])
	gotModTime, err := time.Parse(time.RFC3339Nano, metadata["mtime"])
	assert.NoError(t, err)
	assert.WithinDuration(t, modTime, gotModTime, time.Millisecond)
}

func TestInternalSystemMetadataInfo(t *testing.T) {
	// The system metadata keys must all be documented and, apart from the
	// read only ones, recognised by metadataToHeaders.
	options := metadataToHeaders(fs.Metadata{
		"cache-control":       "no-cache",
		"content-disposition": "inline",
		"content-encoding":    "gzip",
		"content-language":    "en-US",
		"content-type":        "text/plain",
	})
	assert.Equal(t, "no-cache", options.headers["Cache-Control"])
	assert.Equal(t, "inline", options.headers["Content-Disposition"])
	assert.Equal(t, "en-US", options.headers["Content-Language"])
	assert.Equal(t, "text/plain", options.contentType)
	// content-encoding is read only as rclone can't read gzip encoded
	// objects back
	assert.True(t, systemMetadataInfo["content-encoding"].ReadOnly)
	assert.NotContains(t, options.headers, "Content-Encoding")
	for _, key := range []string{
		"cache-control",
		"content-disposition",
		"content-encoding",
		"content-language",
		"content-type",
		"mtime",
	} {
		assert.Contains(t, systemMetadataInfo, key)
	}
}

func TestInternalShouldRetryHeaders(t *testing.T) {
	ctx := context.Background()
	headers := swift.Headers{
		"Content-Length": "64",
		"Content-Type":   "text/html; charset=UTF-8",
		"Date":           "Mon: 18 Mar 2019 12:11:23 GMT",
		"Retry-After":    "1",
	}
	err := &swift.Error{
		StatusCode: 429,
		Text:       "Too Many Requests",
	}

	// Short sleep should just do the sleep
	start := time.Now()
	retry, gotErr := shouldRetryHeaders(ctx, headers, err)
	dt := time.Since(start)
	assert.True(t, retry)
	assert.Equal(t, err, gotErr)
	assert.True(t, dt > time.Second/2)

	// Long sleep should return RetryError
	headers["Retry-After"] = "3600"
	start = time.Now()
	retry, gotErr = shouldRetryHeaders(ctx, headers, err)
	dt = time.Since(start)
	assert.True(t, dt < time.Second)
	assert.False(t, retry)
	assert.Equal(t, true, fserrors.IsRetryAfterError(gotErr))
	after := gotErr.(fserrors.RetryAfter).RetryAfter()
	dt = after.Sub(start)
	assert.True(t, dt >= time.Hour-time.Second && dt <= time.Hour+time.Second)

}
