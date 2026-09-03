package gopro

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/gopro/api"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fileNameUpload = "rclone-test-image2.jpg"

// TestIntegration runs against a real account (TestGoPro: by default). It
// is read-only except for the Upload sub-test, which uploads and then
// removes one small test image.
func TestIntegration(t *testing.T) {
	ctx := context.Background()
	fstest.Initialise()

	if *fstest.RemoteName == "" {
		*fstest.RemoteName = "TestGoPro:"
	}
	f, err := fs.NewFs(ctx, *fstest.RemoteName)
	if errors.Is(err, fs.ErrorNotFoundInConfigFile) {
		t.Skipf("Couldn't create gopro backend - skipping tests: %v", err)
	}
	require.NoError(t, err)

	t.Run("Name", func(t *testing.T) {
		assert.Equal(t, (*fstest.RemoteName)[:len(*fstest.RemoteName)-1], f.Name())
	})

	t.Run("Root", func(t *testing.T) {
		assert.Equal(t, "", f.Root())
	})

	t.Run("Features", func(t *testing.T) {
		features := f.Features()
		assert.True(t, features.ReadMimeType)
	})

	t.Run("Precision", func(t *testing.T) {
		assert.Equal(t, fs.ModTimeNotSupported, f.Precision())
	})

	t.Run("Hashes", func(t *testing.T) {
		assert.Equal(t, hash.Set(hash.None), f.Hashes())
	})

	t.Run("About", func(t *testing.T) {
		abouter, ok := f.(fs.Abouter)
		require.True(t, ok, "Fs should implement Abouter")
		usage, err := abouter.About(ctx)
		require.NoError(t, err)
		require.NotNil(t, usage.Used)
		assert.True(t, *usage.Used >= 0)
	})

	t.Run("ListRoot", func(t *testing.T) {
		entries, err := f.List(ctx, "")
		require.NoError(t, err)
		var names []string
		for _, e := range entries {
			names = append(names, e.Remote())
		}
		assert.ElementsMatch(t, []string{"media", "upload"}, names)
	})

	t.Run("ListMedia", func(t *testing.T) {
		entries, err := f.List(ctx, "media")
		require.NoError(t, err)
		var names []string
		for _, e := range entries {
			names = append(names, e.Remote())
		}
		assert.ElementsMatch(t, []string{"media/all", "media/by-year", "media/by-month", "media/by-day"}, names)
	})

	t.Run("ListMediaAll", func(t *testing.T) {
		// Just check this doesn't error - the account's library contents
		// aren't controlled by this test.
		_, err := f.List(ctx, "media/all")
		require.NoError(t, err)
	})

	t.Run("ListMediaByYear", func(t *testing.T) {
		entries, err := f.List(ctx, "media/by-year")
		require.NoError(t, err)
		assert.NotEmpty(t, entries)
	})

	t.Run("BadDirectory", func(t *testing.T) {
		_, err := f.List(ctx, "not-a-real-directory")
		assert.Equal(t, fs.ErrorDirNotFound, err)
	})

	t.Run("MkdirRmdirOnMedia", func(t *testing.T) {
		// media/* is entirely synthetic and can't be created or removed,
		// same as googlephotos' equivalent (non-album) directories.
		assert.Error(t, f.Mkdir(ctx, "media/all"))
		assert.Error(t, f.Rmdir(ctx, "media/all"))
	})

	t.Run("UploadMkdirRmdir", func(t *testing.T) {
		require.NoError(t, f.Mkdir(ctx, "upload/dir"))

		entries, err := f.List(ctx, "upload")
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, "upload/dir", entries[0].Remote())

		require.NoError(t, f.Rmdir(ctx, "upload/dir"))

		entries, err = f.List(ctx, "upload")
		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("Upload", func(t *testing.T) {
		localFs, err := fs.NewFs(ctx, "testfiles")
		require.NoError(t, err)
		srcObj, err := localFs.NewObject(ctx, fileNameUpload)
		require.NoError(t, err)
		in, err := srcObj.Open(ctx)
		require.NoError(t, err)

		remote := "upload/" + fileNameUpload
		dstObj, err := f.Put(ctx, in, fs.NewOverrideRemote(srcObj, remote))
		require.NoError(t, err)
		_ = in.Close()
		assert.Equal(t, remote, dstObj.Remote())

		gpObj, ok := dstObj.(*Object)
		require.True(t, ok)
		assert.NotEmpty(t, gpObj.id)

		t.Run("ObjectFs", func(t *testing.T) {
			assert.Equal(t, f, dstObj.Fs())
		})

		t.Run("ObjectHash", func(t *testing.T) {
			h, err := dstObj.Hash(ctx, hash.MD5)
			assert.Equal(t, "", h)
			assert.Equal(t, hash.ErrUnsupported, err)
		})

		t.Run("ObjectSetModTime", func(t *testing.T) {
			err := dstObj.SetModTime(ctx, dstObj.ModTime(ctx))
			assert.Equal(t, fs.ErrorCantSetModTime, err)
		})

		t.Run("ObjectStorable", func(t *testing.T) {
			assert.True(t, dstObj.Storable())
		})

		t.Run("ObjectOpen", func(t *testing.T) {
			in, err := dstObj.Open(ctx)
			require.NoError(t, err)
			buf, err := io.ReadAll(in)
			require.NoError(t, err)
			require.NoError(t, in.Close())
			assert.True(t, len(buf) > 1000)
			contentType := http.DetectContentType(buf[:512])
			assert.Equal(t, "image/jpeg", contentType)
		})

		t.Run("NewObject", func(t *testing.T) {
			o, err := f.NewObject(ctx, remote)
			require.NoError(t, err)
			assert.Equal(t, remote, o.Remote())
		})

		t.Run("Remove", func(t *testing.T) {
			require.NoError(t, dstObj.Remove(ctx))
		})
	})
}

func TestAddID(t *testing.T) {
	assert.Equal(t, "potato {123}", addID("potato", "123"))
	assert.Equal(t, "{123}", addID("", "123"))
}

func TestAddFileID(t *testing.T) {
	assert.Equal(t, "potato {123}.txt", addFileID("potato.txt", "123"))
	assert.Equal(t, "potato {123}", addFileID("potato", "123"))
	assert.Equal(t, "{123}", addFileID("", "123"))
}

func TestShouldAddID(t *testing.T) {
	t.Run("always_add_id forces it even with no collision", func(t *testing.T) {
		assert.True(t, shouldAddID(true, "GX010123.MP4", 1))
	})

	t.Run("a real collision forces it regardless of the option", func(t *testing.T) {
		assert.True(t, shouldAddID(false, "GX010123.MP4", 2))
	})

	t.Run("an empty remote always forces it, even alone", func(t *testing.T) {
		assert.True(t, shouldAddID(false, "", 1))
	})

	t.Run("no collision and the option off leaves the name alone", func(t *testing.T) {
		assert.False(t, shouldAddID(false, "GX010123.MP4", 1))
	})
}

func TestFindID(t *testing.T) {
	assert.Equal(t, "", findID("potato"))
	id := "68b22325df3cf752557ac6d7" // a real 24-char lowercase hex medium id
	assert.Equal(t, id, findID("GX010294 {"+id+"}.MP4"))
	assert.Equal(t, "", findID("GX010294.MP4")) // a real filename must never match
	assert.Equal(t, "", findID("potato {too-short}.txt"))
}

func TestItemLeaf(t *testing.T) {
	assert.Equal(t, "GX010294-1.MP4", itemLeaf("GX010294.MP4", 1))
	assert.Equal(t, "GX010294-2.MP4", itemLeaf("GX010294.MP4", 2))
	assert.Equal(t, "GPAA1945-30.JPG", itemLeaf("GPAA1945.JPG", 30))
}

func TestObjectSetMetaDataReprocessed(t *testing.T) {
	size := int64(100)

	t.Run("reprocessed_at set marks the object reprocessed", func(t *testing.T) {
		reprocessedAt := time.Now()
		o := &Object{}
		o.setMetaData(&api.Medium{ItemCount: 1, FileSize: &size, ReprocessedAt: &reprocessedAt}, 1)
		assert.True(t, o.reprocessed)
	})

	t.Run("reprocessed_at unset (the common case) leaves the object not reprocessed", func(t *testing.T) {
		o := &Object{}
		o.setMetaData(&api.Medium{ItemCount: 1, FileSize: &size}, 1)
		assert.False(t, o.reprocessed)
	})
}

// testFile is a shorthand for building api.File fixtures in table tests
type testFile struct {
	url        string
	label      string
	quality    string
	itemNumber int
}

// makeDownloadResponse builds an api.DownloadResponse from shorthand
// fixtures, matching the shapes for a single-item medium, a chaptered
// video and a burst photo set (see selectRendition's doc comment).
func makeDownloadResponse(files, variations []testFile) *api.DownloadResponse {
	dl := &api.DownloadResponse{}
	for _, f := range files {
		dl.Embedded.Files = append(dl.Embedded.Files, api.File{URL: f.url, Head: f.url, ItemNumber: f.itemNumber})
	}
	for _, v := range variations {
		dl.Embedded.Variations = append(dl.Embedded.Variations, api.File{
			URL: v.url, Head: v.url, Label: v.label, Quality: v.quality, ItemNumber: v.itemNumber,
		})
	}
	return dl
}

func TestObjectFixSize(t *testing.T) {
	t.Run("200 with a correct Content-Length is a no-op", func(t *testing.T) {
		o := &Object{bytes: 100}
		o.fixSize(&http.Response{StatusCode: http.StatusOK, ContentLength: 100, Header: http.Header{}})
		assert.Equal(t, int64(100), o.bytes)
	})

	t.Run("200 corrects a wrong file_size to the real Content-Length", func(t *testing.T) {
		// The real numbers from a live account: file_size for a Glacier
		// Instant Retrieval-archived video was 3245 bytes larger than what
		// the source rendition actually delivered.
		o := &Object{bytes: 10940989419}
		o.fixSize(&http.Response{StatusCode: http.StatusOK, ContentLength: 10940986174, Header: http.Header{}})
		assert.Equal(t, int64(10940986174), o.bytes)
	})

	t.Run("206 partial content uses the total from Content-Range, not the range length", func(t *testing.T) {
		o := &Object{bytes: 10940989419}
		o.fixSize(&http.Response{
			StatusCode:    http.StatusPartialContent,
			ContentLength: 1024, // just this range's length, not the whole file
			Header:        http.Header{"Content-Range": []string{"bytes 5242880-5243903/10940986174"}},
		})
		assert.Equal(t, int64(10940986174), o.bytes)
	})

	t.Run("206 with an unparseable Content-Range never falls back to the range length", func(t *testing.T) {
		o := &Object{bytes: 12345}
		o.fixSize(&http.Response{
			StatusCode:    http.StatusPartialContent,
			ContentLength: 1024,
			Header:        http.Header{"Content-Range": []string{"bytes */*"}},
		})
		assert.Equal(t, int64(12345), o.bytes)
	})

	t.Run("206 with no Content-Range header at all is left unchanged", func(t *testing.T) {
		o := &Object{bytes: 12345}
		o.fixSize(&http.Response{StatusCode: http.StatusPartialContent, ContentLength: 1024, Header: http.Header{}})
		assert.Equal(t, int64(12345), o.bytes)
	})

	t.Run("unknown Content-Length (-1) is left unchanged", func(t *testing.T) {
		o := &Object{bytes: 12345}
		o.fixSize(&http.Response{StatusCode: http.StatusOK, ContentLength: -1, Header: http.Header{}})
		assert.Equal(t, int64(12345), o.bytes)
	})

	t.Run("a correction marks the size as checked", func(t *testing.T) {
		o := &Object{bytes: 10940989419}
		o.fixSize(&http.Response{StatusCode: http.StatusOK, ContentLength: 10940986174, Header: http.Header{}})
		assert.True(t, o.sizeChecked)
	})

	t.Run("once checked, a later call never overrides bytes again", func(t *testing.T) {
		// Size() already resolved and cached the true size (10940986174).
		// A later Open() seeing a *different* Content-Length (e.g. a CDN
		// serving a differently-sized response for some other reason)
		// must not clobber the value Size() already committed to - the
		// whole point of caching is that once-corrected values are final
		// for this Object's lifetime.
		o := &Object{bytes: 10940986174, sizeChecked: true}
		o.fixSize(&http.Response{StatusCode: http.StatusOK, ContentLength: 999, Header: http.Header{}})
		assert.Equal(t, int64(10940986174), o.bytes)
	})
}

func TestObjectReportSizeMismatch(t *testing.T) {
	t.Run("known size differing from actual is corrected", func(t *testing.T) {
		o := &Object{bytes: 10940989419}
		o.reportSizeMismatch(10940986174)
		assert.Equal(t, int64(10940986174), o.bytes)
	})

	t.Run("known size matching actual is a no-op", func(t *testing.T) {
		o := &Object{bytes: 100}
		o.reportSizeMismatch(100)
		assert.Equal(t, int64(100), o.bytes)
	})

	t.Run("unknown (-1) size is simply resolved, not treated as a mismatch", func(t *testing.T) {
		o := &Object{bytes: -1}
		o.reportSizeMismatch(4096)
		assert.Equal(t, int64(4096), o.bytes)
	})
}

// TestObjectSizeFastPaths covers every case where Size must return without
// touching the network - each uses a bare &Fs{} with a nil srv/unAuth/pacer,
// so an unwanted network attempt panics on a nil dereference instead of
// silently passing.
func TestObjectSizeFastPaths(t *testing.T) {
	t.Run("verify_size off skips the check for a known size", func(t *testing.T) {
		o := &Object{fs: &Fs{opt: Options{VerifySize: verifySizeOff}}, bytes: 12345}
		assert.Equal(t, int64(12345), o.Size())
	})

	t.Run("verify_size reprocessed skips an object whose medium wasn't reprocessed", func(t *testing.T) {
		o := &Object{fs: &Fs{opt: Options{VerifySize: verifySizeReprocessed}}, bytes: 12345, reprocessed: false}
		assert.Equal(t, int64(12345), o.Size())
	})

	t.Run("read_size disabled leaves an unknown multi-item size alone", func(t *testing.T) {
		o := &Object{fs: &Fs{opt: Options{ReadSize: false}}, bytes: -1}
		assert.Equal(t, int64(-1), o.Size())
	})

	t.Run("an already-checked size is returned as-is regardless of options", func(t *testing.T) {
		o := &Object{fs: &Fs{opt: Options{VerifySize: verifySizeAlways}}, bytes: 999, sizeChecked: true}
		assert.Equal(t, int64(999), o.Size())
	})
}

func TestShouldVerifySize(t *testing.T) {
	assert.False(t, shouldVerifySize(verifySizeOff, false))
	assert.False(t, shouldVerifySize(verifySizeOff, true))
	assert.True(t, shouldVerifySize(verifySizeAlways, false))
	assert.True(t, shouldVerifySize(verifySizeAlways, true))
	assert.False(t, shouldVerifySize(verifySizeReprocessed, false))
	assert.True(t, shouldVerifySize(verifySizeReprocessed, true))
}

func TestCheckVerifySizeMode(t *testing.T) {
	assert.NoError(t, checkVerifySizeMode(verifySizeReprocessed))
	assert.NoError(t, checkVerifySizeMode(verifySizeAlways))
	assert.NoError(t, checkVerifySizeMode(verifySizeOff))
	assert.Error(t, checkVerifySizeMode("sometimes"))
	assert.Error(t, checkVerifySizeMode(""))
}

func TestCheckUploadChunkSize(t *testing.T) {
	assert.NoError(t, checkUploadChunkSize(minUploadChunkSize))
	assert.NoError(t, checkUploadChunkSize(minUploadChunkSize+1))
	assert.Error(t, checkUploadChunkSize(minUploadChunkSize-1))
}

func TestSetUploadChunkSize(t *testing.T) {
	f := &Fs{opt: Options{UploadChunkSize: defaultUploadChunkSize}}

	old, err := f.setUploadChunkSize(2 * minUploadChunkSize)
	require.NoError(t, err)
	assert.Equal(t, defaultUploadChunkSize, old)
	assert.Equal(t, 2*minUploadChunkSize, f.opt.UploadChunkSize)

	// A rejected change must leave the existing chunk size alone.
	_, err = f.setUploadChunkSize(minUploadChunkSize - 1)
	assert.Error(t, err)
	assert.Equal(t, 2*minUploadChunkSize, f.opt.UploadChunkSize)
}

// newTestChunkWriterFs builds a minimal *Fs whose unauthenticated client
// can call a local httptest.Server, for testing gpChunkWriter.WriteChunk
// without a live GoPro account - WriteChunk only ever uses f.unAuth and
// f.pacer.
func newTestChunkWriterFs() *Fs {
	f := &Fs{
		unAuth: rest.NewClient(&http.Client{}),
		pacer:  fs.NewPacer(context.Background(), pacer.NewDefault(pacer.MinSleep(time.Millisecond), pacer.MaxSleep(5*time.Millisecond))),
	}
	f.unAuth.SetErrorHandler(errorHandler)
	return f
}

func TestGpChunkWriterWriteChunk(t *testing.T) {
	t.Run("a successful PUT returns the chunk's actual size", func(t *testing.T) {
		var gotBody []byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error
			gotBody, err = io.ReadAll(r.Body)
			require.NoError(t, err)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		f := newTestChunkWriterFs()
		cw := &gpChunkWriter{f: f, parts: []api.UploadAuthorization{{URL: srv.URL, Part: 1}}}

		want := []byte("hello chunked world")
		n, err := cw.WriteChunk(context.Background(), 0, bytes.NewReader(want))
		require.NoError(t, err)
		assert.Equal(t, int64(len(want)), n)
		assert.Equal(t, want, gotBody)
	})

	t.Run("a retryable failure is retried with the reader correctly rewound", func(t *testing.T) {
		// Regression test for the CONTRIBUTING.md "Managing memory" contract:
		// a pooled, seekable chunk buffer must be rewound to the start
		// before every attempt, including retries, or a retry after a
		// partial read would resend truncated or missing data instead of
		// the original chunk.
		want := []byte("this chunk must survive a retry intact")
		var attempts int
		var bodies [][]byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			bodies = append(bodies, body)
			if attempts == 1 {
				w.WriteHeader(http.StatusInternalServerError) // retryable, see retryErrorCodes
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		f := newTestChunkWriterFs()
		cw := &gpChunkWriter{f: f, parts: []api.UploadAuthorization{{URL: srv.URL, Part: 1}}}

		n, err := cw.WriteChunk(context.Background(), 0, bytes.NewReader(want))
		require.NoError(t, err)
		assert.Equal(t, int64(len(want)), n)
		require.Equal(t, 2, attempts)
		// Both attempts - including the one that failed - must have seen
		// the complete, correct chunk, not a partially-drained reader.
		assert.Equal(t, want, bodies[0])
		assert.Equal(t, want, bodies[1])
	})

	t.Run("an out of range chunk number is rejected without a request", func(t *testing.T) {
		var called bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		f := newTestChunkWriterFs()
		cw := &gpChunkWriter{f: f, parts: []api.UploadAuthorization{{URL: srv.URL, Part: 1}}}

		_, err := cw.WriteChunk(context.Background(), 1, bytes.NewReader([]byte("x")))
		assert.Error(t, err)
		assert.False(t, called)
	})
}

func TestSelectRendition(t *testing.T) {
	// Single-item medium: files[0] is a proxy, the "source" variation is
	// the real original.
	t.Run("single item video", func(t *testing.T) {
		dl := makeDownloadResponse(
			[]testFile{{url: "https://cdn/proxy.mp4", itemNumber: 1}},
			[]testFile{{url: "https://cdn/source.mp4", label: "source", itemNumber: 0}},
		)
		u, _, err := selectRendition(dl, "source", 1)
		require.NoError(t, err)
		assert.Equal(t, "https://cdn/source.mp4", u)
	})

	t.Run("single item falls back to files when no source variation", func(t *testing.T) {
		dl := makeDownloadResponse(
			[]testFile{{url: "https://cdn/only.mp4", itemNumber: 1}},
			nil,
		)
		u, _, err := selectRendition(dl, "source", 1)
		require.NoError(t, err)
		assert.Equal(t, "https://cdn/only.mp4", u)
	})

	t.Run("chaptered video addresses by item_number in variations", func(t *testing.T) {
		dl := makeDownloadResponse(
			[]testFile{{url: "https://cdn/proxy.mp4", itemNumber: 1}},
			[]testFile{
				{url: "https://cdn/ch1.mp4", label: "source", itemNumber: 1},
				{url: "https://cdn/ch2.mp4", label: "source", itemNumber: 2},
			},
		)
		u1, _, err := selectRendition(dl, "source", 1)
		require.NoError(t, err)
		assert.Equal(t, "https://cdn/ch1.mp4", u1)
		u2, _, err := selectRendition(dl, "source", 2)
		require.NoError(t, err)
		assert.Equal(t, "https://cdn/ch2.mp4", u2)
	})

	t.Run("burst addresses by item_number in files, not the cover variation", func(t *testing.T) {
		dl := makeDownloadResponse(
			[]testFile{
				{url: "https://cdn/1.jpg", itemNumber: 1},
				{url: "https://cdn/2.jpg", itemNumber: 2},
				{url: "https://cdn/3.jpg", itemNumber: 3},
			},
			[]testFile{{url: "https://cdn/cover.jpg", label: "source", itemNumber: 0}},
		)
		u2, _, err := selectRendition(dl, "source", 2)
		require.NoError(t, err)
		assert.Equal(t, "https://cdn/2.jpg", u2)
	})

	t.Run("explicit variation matches by label or quality", func(t *testing.T) {
		dl := makeDownloadResponse(
			[]testFile{{url: "https://cdn/proxy.mp4", itemNumber: 1}},
			[]testFile{
				{url: "https://cdn/source.mp4", label: "source"},
				{url: "https://cdn/1080p.mp4", label: "high_res_proxy_mp4", quality: "1080p"},
			},
		)
		u, _, err := selectRendition(dl, "1080p", 1)
		require.NoError(t, err)
		assert.Equal(t, "https://cdn/1080p.mp4", u)
	})

	t.Run("no rendition found returns an error", func(t *testing.T) {
		dl := makeDownloadResponse(nil, nil)
		_, _, err := selectRendition(dl, "source", 1)
		assert.Error(t, err)
	})
}
