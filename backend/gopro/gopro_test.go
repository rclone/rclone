package gopro

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/gopro/api"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/dirtree"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeJSON writes v as the JSON body of a mocked API response, matching
// what rest.Client.CallJSON expects to be able to decode.
func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(v))
}

// newTestAPIFs builds a minimal *Fs whose srv and unAuth clients talk to a
// local httptest.Server, for unit-testing the single-HTTP-call API wrapper
// methods (getMedium, updateMedium, getUserInfo, ...) without a live
// account. Callers needing a specific f.opt or f.resourceOwnerID set them
// on the returned *Fs before use.
func newTestAPIFs(handler http.Handler) (*Fs, *httptest.Server) {
	srv := httptest.NewServer(handler)
	f := &Fs{
		srv:      rest.NewClient(&http.Client{}).SetRoot(srv.URL),
		unAuth:   rest.NewClient(&http.Client{}),
		pacer:    fs.NewPacer(context.Background(), pacer.NewDefault(pacer.MinSleep(time.Millisecond), pacer.MaxSleep(5*time.Millisecond))),
		dlCache:  map[string]*dlCacheEntry{},
		uploaded: dirtree.New(),
		media:    &listCache{},
		trash:    &listCache{},
	}
	f.srv.SetErrorHandler(errorHandler)
	f.unAuth.SetErrorHandler(errorHandler)
	return f, srv
}

// cachedList returns a listCache already holding items, fetched just now
func cachedList(items []api.Medium) *listCache {
	return &listCache{items: items, at: time.Now()}
}

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

	// remoteWithOptions builds a second Fs against the same account using
	// rclone's connection-string syntax ("remote,opt=val:") to override one
	// or more backend options without touching the configured remote -
	// see the "Connection strings" section of docs/content/docs.md.
	remoteWithOptions := func(t *testing.T, opts string) fs.Fs {
		t.Helper()
		base := (*fstest.RemoteName)[:len(*fstest.RemoteName)-1]
		f2, err := fs.NewFs(ctx, base+","+opts+":")
		require.NoError(t, err)
		return f2
	}

	t.Run("ShowAllNeverListsFewerEntriesThanTheDefault", func(t *testing.T) {
		// show_all bypasses every server- and client-side filter this
		// backend otherwise applies, so it can only ever surface the same
		// items or more, never fewer, regardless of the account's actual
		// content.
		def, err := f.List(ctx, "media/all")
		require.NoError(t, err)
		f2 := remoteWithOptions(t, "show_all=true")
		all, err := f2.List(ctx, "media/all")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(all), len(def))
	})

	t.Run("IncludeProcessingAndIncludeFailedNeverListFewerEntriesThanTheDefault", func(t *testing.T) {
		def, err := f.List(ctx, "media/all")
		require.NoError(t, err)

		f2 := remoteWithOptions(t, "include_processing=true")
		withProcessing, err := f2.List(ctx, "media/all")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(withProcessing), len(def))

		f3 := remoteWithOptions(t, "include_failed=true")
		withFailed, err := f3.List(ctx, "media/all")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(withFailed), len(def))
	})

	t.Run("ShowEmptyDirsNeverListsFewerYearsThanTheDefault", func(t *testing.T) {
		def, err := f.List(ctx, "media/by-year")
		require.NoError(t, err)
		f2 := remoteWithOptions(t, "show_empty_dirs=true")
		all, err := f2.List(ctx, "media/by-year")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(all), len(def))
	})

	t.Run("StartYearWidensTheRangeWhenCombinedWithShowEmptyDirs", func(t *testing.T) {
		f2 := remoteWithOptions(t, "start_year=2000,show_empty_dirs=true")
		entries, err := f2.List(ctx, "media/by-year")
		require.NoError(t, err)
		var names []string
		for _, e := range entries {
			names = append(names, e.Remote())
		}
		assert.Contains(t, names, "media/by-year/2000")

		// The empty year is still directly addressable even though it
		// doesn't appear in a browsed listing under the default remote.
		_, err = f.List(ctx, "media/by-day/2000/2000-01-01")
		assert.NoError(t, err)
	})

	t.Run("RawFilesDownload", func(t *testing.T) {
		entries, err := f.List(ctx, "media/all")
		require.NoError(t, err)
		var raw fs.Object
		for _, e := range entries {
			if o, ok := e.(fs.Object); ok && strings.EqualFold(path.Ext(o.Remote()), ".gpr") {
				raw = o
				break
			}
		}
		if raw == nil {
			t.Skip("no RAW photos in this account")
		}
		in, err := raw.Open(ctx)
		require.NoError(t, err)
		head := make([]byte, 4)
		_, err = io.ReadFull(in, head)
		require.NoError(t, err)
		require.NoError(t, in.Close())
		// GPR is DNG, which is TIFF-based.
		assert.Contains(t, []string{"II*\x00", "MM\x00*"}, string(head), "%s isn't a RAW file", raw.Remote())
	})

	t.Run("PhotoFormatOnlyDropsOneHalfOfEachPair", func(t *testing.T) {
		list := func(format string) map[string]bool {
			entries, err := remoteWithOptions(t, "photo_format="+format).List(ctx, "media/all")
			require.NoError(t, err)
			names := map[string]bool{}
			for _, e := range entries {
				names[e.Remote()] = true
			}
			return names
		}
		both, jpegs, raws := list("both"), list("jpeg"), list("raw")
		wantJPEG, wantRaw := map[string]bool{}, map[string]bool{}
		for name := range both {
			base, ext := strings.TrimSuffix(name, path.Ext(name)), strings.ToLower(path.Ext(name))
			paired := (ext == ".gpr" && (both[base+".JPG"] || both[base+".jpg"])) ||
				(ext == ".jpg" && (both[base+".GPR"] || both[base+".gpr"]))
			if !paired || ext == ".jpg" {
				wantJPEG[name] = true
			}
			if !paired || ext == ".gpr" {
				wantRaw[name] = true
			}
		}
		assert.Equal(t, wantJPEG, jpegs)
		assert.Equal(t, wantRaw, raws)
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
			newTime := time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)
			require.NoError(t, dstObj.SetModTime(ctx, newTime))
			assert.True(t, newTime.Equal(dstObj.ModTime(ctx)))
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
			// Remove only trashes it by default - purge it so test runs
			// don't fill the trash.
			time.Sleep(deletePermanentDelay)
			require.NoError(t, f.(*Fs).doDeleteMedium(ctx, gpObj.id, "permanent", "true"))
		})
	})

	t.Run("UpdateReplacesTheExistingUpload", func(t *testing.T) {
		gf, ok := f.(*Fs)
		require.True(t, ok)
		localFs, err := fs.NewFs(ctx, "testfiles")
		require.NoError(t, err)
		put := func(name string, update fs.Object) fs.Object {
			srcObj, err := localFs.NewObject(ctx, name)
			require.NoError(t, err)
			in, err := srcObj.Open(ctx)
			require.NoError(t, err)
			defer func() { _ = in.Close() }()
			src := fs.NewOverrideRemote(srcObj, "upload/rclone-test-update.jpg")
			if update != nil {
				require.NoError(t, update.Update(ctx, in, src))
				return update
			}
			o, err := f.Put(ctx, in, src)
			require.NoError(t, err)
			return o
		}
		readLocal := func(name string) []byte {
			b, err := os.ReadFile(filepath.Join("testfiles", name))
			require.NoError(t, err)
			return b
		}
		isNotFound := func(err error) bool {
			var apiErr *api.Error
			return errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound
		}

		o := put("rclone-test-image1.jpg", nil)
		oldID := o.(*Object).id
		// Purge whatever this test leaves behind, active or trashed.
		defer func() {
			for _, id := range []string{oldID, o.(*Object).id} {
				if _, err := gf.getMedium(ctx, id); err == nil {
					_ = gf.deleteMedium(ctx, id, true)
				} else {
					_ = gf.doDeleteMedium(ctx, id, "permanent", "true")
				}
			}
		}()

		put("rclone-test-image2.jpg", o)
		newID := o.(*Object).id
		assert.NotEqual(t, oldID, newID, "Update must upload a new medium")

		_, err = gf.getMedium(ctx, oldID)
		assert.True(t, isNotFound(err), "the replaced medium must be gone from the library, got %v", err)
		trash, err := gf.allTrash(ctx)
		require.NoError(t, err)
		var trashed bool
		for _, m := range trash {
			trashed = trashed || m.ID == oldID
		}
		assert.True(t, trashed, "with use_trash the replaced medium must be in the trash")

		entries, err := f.List(ctx, "upload")
		require.NoError(t, err)
		var ids []string
		for _, e := range entries {
			if e.Remote() == "upload/rclone-test-update.jpg" {
				ids = append(ids, e.(*Object).id)
			}
		}
		assert.Equal(t, []string{newID}, ids, "only the new upload must be listed")

		in, err := o.Open(ctx)
		require.NoError(t, err)
		got, err := io.ReadAll(in)
		require.NoError(t, err)
		require.NoError(t, in.Close())
		assert.Equal(t, readLocal("rclone-test-image2.jpg"), got, "the new content must be served")
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
	assert.Equal(t, "media/all/{123}.mp4", addFileID("media/all/.mp4", "123"))
	assert.Equal(t, "media/all/{123}", addFileID("media/all/", "123"))
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

	t.Run("an unnamed leaf under a directory prefix forces it, even alone", func(t *testing.T) {
		assert.True(t, shouldAddID(false, "media/all/.mp4", 1))
		assert.True(t, shouldAddID(false, "media/all/", 1))
	})

	t.Run("a named leaf under a directory prefix is left alone", func(t *testing.T) {
		assert.False(t, shouldAddID(false, "media/all/GX010123.MP4", 1))
	})

	t.Run("no collision and the option off leaves the name alone", func(t *testing.T) {
		assert.False(t, shouldAddID(false, "GX010123.MP4", 1))
	})
}

func TestExpectedIDSuffixedName(t *testing.T) {
	f := &Fs{}

	t.Run("single-item medium", func(t *testing.T) {
		item := &api.Medium{Filename: "GX010123.MP4", ID: "68b22325df3cf752557ac6d7", ItemCount: 1}
		assert.Equal(t, "GX010123 {68b22325df3cf752557ac6d7}.MP4", expectedIDSuffixedName(f, item))
	})

	t.Run("multi-item medium resolves to item 1", func(t *testing.T) {
		item := &api.Medium{Filename: "GX010123.MP4", ID: "68b22325df3cf752557ac6d7", ItemCount: 3}
		assert.Equal(t, "GX010123-1 {68b22325df3cf752557ac6d7}.MP4", expectedIDSuffixedName(f, item))
	})

	t.Run("a renamed medium with an arbitrary filename never matches an unrelated id embedded in a path", func(t *testing.T) {
		// A GoPro medium can be renamed to any filename via the API (see
		// the rename support), so a real listed name for one medium's id
		// must never be confused for a different, unrelated {id}-shaped
		// substring a user's chosen filename happens to contain.
		item := &api.Medium{Filename: "giveEditAName.mp4", ID: "6a9362c08c23f474301c0899", ItemCount: 1}
		fabricated := "totally unrelated name {6a9362c08c23f474301c0899}.mp4"
		assert.NotEqual(t, fabricated, expectedIDSuffixedName(f, item))
	})
}

func TestFindID(t *testing.T) {
	assert.Equal(t, "", findID("potato"))
	id := "68b22325df3cf752557ac6d7" // a real 24-char lowercase hex medium id
	assert.Equal(t, id, findID("GX010294 {"+id+"}.MP4"))
	assert.Equal(t, "", findID("GX010294.MP4"))
	assert.Equal(t, "", findID("potato {too-short}.txt"))

	t.Run("only the trailing suffix counts, not an id-shaped substring earlier in the name", func(t *testing.T) {
		unrelated := "aaaaaaaaaaaaaaaaaaaaaaaa"
		assert.Equal(t, id, findID("note {"+unrelated+"} clip {"+id+"}.mp4"))
	})

	t.Run("an id-shaped substring with nothing after it is not a trailing suffix", func(t *testing.T) {
		assert.Equal(t, "", findID("note {"+id+"} halfway through.mp4"))
	})
}

func TestStripSuffixID(t *testing.T) {
	id := "68b22325df3cf752557ac6d7"
	assert.Equal(t, "GX010294.MP4", stripSuffixID("GX010294 {"+id+"}.MP4", id))
	assert.Equal(t, "GX010294.MP4", stripSuffixID("GX010294.MP4", id))
	assert.Equal(t, ".MP4", stripSuffixID("{"+id+"}.MP4", id))

	t.Run("only strips the suffix, not an id-shaped substring elsewhere", func(t *testing.T) {
		// A renamed medium can carry an arbitrary filename - an id-shaped
		// substring that isn't in the exact " {id}" suffix position must
		// survive untouched.
		name := "note {" + id + "} halfway through.mp4"
		assert.Equal(t, name, stripSuffixID(name, id))
	})

	t.Run("a trailing id-shaped suffix that isn't this object's own id is left alone", func(t *testing.T) {
		// A legitimate rename to a name that happens to end in an
		// unrelated id-shaped suffix must be sent to GoPro untouched,
		// not have that suffix silently stripped as if this backend
		// had added it.
		other := "aaaaaaaaaaaaaaaaaaaaaaaa"
		name := "clip {" + other + "}.mp4"
		assert.Equal(t, name, stripSuffixID(name, id))
	})
}

func TestDestCapturedAt(t *testing.T) {
	modTime := time.Date(2025, 3, 14, 9, 30, 15, 0, time.UTC)

	t.Run("media/all implies no date", func(t *testing.T) {
		p := &dirPattern{re: `^media/all/([^/]+)$`}
		_, ok, err := destCapturedAt(p, []string{"", "x.mp4"}, modTime)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("by-year changes only the year, keeps month/day/time-of-day", func(t *testing.T) {
		p := &dirPattern{re: `^media/by-year/(\d{4})/([^/]+)$`}
		got, ok, err := destCapturedAt(p, []string{"", "2026", "x.mp4"}, modTime)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, time.Date(2026, 3, 14, 9, 30, 15, 0, time.UTC), got)
	})

	t.Run("by-month changes year and month, keeps day/time-of-day", func(t *testing.T) {
		p := &dirPattern{re: `^media/by-month/\d{4}/(\d{4})-(\d{2})/([^/]+)$`}
		got, ok, err := destCapturedAt(p, []string{"", "2026", "07", "x.mp4"}, modTime)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, time.Date(2026, 7, 14, 9, 30, 15, 0, time.UTC), got)
	})

	t.Run("by-day pins the whole date, keeps time-of-day", func(t *testing.T) {
		p := &dirPattern{re: `^media/by-day/\d{4}/(\d{4})-(\d{2})-(\d{2})/([^/]+)$`}
		got, ok, err := destCapturedAt(p, []string{"", "2026", "07", "04", "x.mp4"}, modTime)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, time.Date(2026, 7, 4, 9, 30, 15, 0, time.UTC), got)
	})

	t.Run("a destination date matching modTime already needs no change", func(t *testing.T) {
		p := &dirPattern{re: `^media/by-day/\d{4}/(\d{4})-(\d{2})-(\d{2})/([^/]+)$`}
		_, ok, err := destCapturedAt(p, []string{"", "2025", "03", "14", "x.mp4"}, modTime)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("an explicitly invalid by-day path is rejected, not normalized", func(t *testing.T) {
		p := &dirPattern{re: `^media/by-day/\d{4}/(\d{4})-(\d{2})-(\d{2})/([^/]+)$`}
		_, _, err := destCapturedAt(p, []string{"", "2026", "02", "31", "x.mp4"}, modTime)
		assert.Error(t, err)
	})

	t.Run("moving a day-31 item to a by-month bucket with no 31st is rejected, not rolled into the next month", func(t *testing.T) {
		p := &dirPattern{re: `^media/by-month/\d{4}/(\d{4})-(\d{2})/([^/]+)$`}
		day31 := time.Date(2025, 3, 31, 9, 30, 15, 0, time.UTC)
		_, _, err := destCapturedAt(p, []string{"", "2026", "02", "x.mp4"}, day31)
		assert.Error(t, err)
	})

	t.Run("moving a leap-day item to a by-year bucket with no Feb 29 is rejected, not rolled into March", func(t *testing.T) {
		p := &dirPattern{re: `^media/by-year/(\d{4})/([^/]+)$`}
		leapDay := time.Date(2024, 2, 29, 9, 30, 15, 0, time.UTC)
		_, _, err := destCapturedAt(p, []string{"", "2026", "x.mp4"}, leapDay)
		assert.Error(t, err)
	})
}

func TestMoveFastPaths(t *testing.T) {
	f := &Fs{}
	ctx := context.Background()

	t.Run("a non-Object src is refused", func(t *testing.T) {
		_, err := f.Move(ctx, nil, "media/all/x.mp4")
		assert.Equal(t, fs.ErrorCantMove, err)
	})

	t.Run("a multi-item object is refused", func(t *testing.T) {
		src := &Object{fs: f, itemCount: 2}
		_, err := f.Move(ctx, src, "media/all/x.mp4")
		assert.Equal(t, fs.ErrorCantMove, err)
	})

	t.Run("a destination outside media/ is refused", func(t *testing.T) {
		src := &Object{fs: f, itemCount: 1}
		_, err := f.Move(ctx, src, "upload/x.mp4")
		assert.Equal(t, fs.ErrorCantMove, err)
	})

	t.Run("a directory-shaped destination is refused", func(t *testing.T) {
		src := &Object{fs: f, itemCount: 1}
		_, err := f.Move(ctx, src, "media/all")
		assert.Equal(t, fs.ErrorCantMove, err)
	})
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

func TestMediaTypes(t *testing.T) {
	withEdits := (&Fs{opt: Options{IncludeEdits: true}}).mediaTypes()
	assert.Contains(t, withEdits, "MultiClipEdit")
	assert.Contains(t, withEdits, "Edit")
	assert.Contains(t, withEdits, includedTypes)

	withoutEdits := (&Fs{opt: Options{IncludeEdits: false}}).mediaTypes()
	assert.Equal(t, includedTypes, withoutEdits)
	assert.NotContains(t, withoutEdits, "MultiClipEdit")
}

func TestIsEditType(t *testing.T) {
	assert.True(t, isEditType("MultiClipEdit"))
	assert.True(t, isEditType("Edit"))
	assert.False(t, isEditType("Video"))
	assert.False(t, isEditType(""))
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

	// A TimeLapse photo series (play_as "multi_shot_photo"), as served live
	// from a HERO13 Black: the burst shape, with the lone "source"
	// variation being item 1 again.
	t.Run("time lapse photo series addresses every frame by item_number", func(t *testing.T) {
		var files []testFile
		for n := 1; n <= 6; n++ {
			files = append(files, testFile{url: fmt.Sprintf("https://cdn/source/default/%d.jpg", n), itemNumber: n})
		}
		dl := makeDownloadResponse(files,
			[]testFile{{url: "https://cdn/source/default/1.jpg", label: "source", quality: "4872p"}},
		)
		for n := 1; n <= 6; n++ {
			u, _, err := selectRendition(dl, "source", n)
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("https://cdn/source/default/%d.jpg", n), u)
		}
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

	// A MultiClipEdit's descriptor, as served live: no "source" variation,
	// just the rendered video labelled "baked_source".
	t.Run("edit picks the baked_source variation over files", func(t *testing.T) {
		dl := makeDownloadResponse(
			[]testFile{{url: "https://cdn/file.mp4", itemNumber: 1}},
			[]testFile{{url: "https://cdn/baked.mp4", label: "baked_source"}},
		)
		u, _, err := selectRendition(dl, "source", 1)
		require.NoError(t, err)
		assert.Equal(t, "https://cdn/baked.mp4", u)
	})

	t.Run("no rendition found returns an error", func(t *testing.T) {
		dl := makeDownloadResponse(nil, nil)
		_, _, err := selectRendition(dl, "source", 1)
		assert.Error(t, err)
	})

	t.Run("explicit variation on a chaptered video matches by item_number, not the first match", func(t *testing.T) {
		dl := makeDownloadResponse(
			nil,
			[]testFile{
				{url: "https://cdn/ch1-1080p.mp4", label: "1080p", itemNumber: 1},
				{url: "https://cdn/ch2-1080p.mp4", label: "1080p", itemNumber: 2},
			},
		)
		u, _, err := selectRendition(dl, "1080p", 2)
		require.NoError(t, err)
		assert.Equal(t, "https://cdn/ch2-1080p.mp4", u, "must pick chapter 2's own rendition, not chapter 1's")
	})

	t.Run("explicit variation falls back to an unnumbered shared rendition", func(t *testing.T) {
		dl := makeDownloadResponse(
			nil,
			[]testFile{{url: "https://cdn/shared-1080p.mp4", label: "1080p", itemNumber: 0}},
		)
		u, _, err := selectRendition(dl, "1080p", 2)
		require.NoError(t, err)
		assert.Equal(t, "https://cdn/shared-1080p.mp4", u)
	})

	t.Run("explicit variation falls back to the offered file for the item when no variation matches", func(t *testing.T) {
		dl := makeDownloadResponse(
			[]testFile{
				{url: "https://cdn/1.jpg", itemNumber: 1},
				{url: "https://cdn/2.jpg", itemNumber: 2},
			},
			[]testFile{{url: "https://cdn/cover.jpg", label: "source", itemNumber: 0}},
		)
		u, _, err := selectRendition(dl, "1080p", 2)
		require.NoError(t, err, "the option contract promises a file fallback rather than erroring out")
		assert.Equal(t, "https://cdn/2.jpg", u)
	})
}

func TestProcessingStates(t *testing.T) {
	t.Run("defaults to ready only", func(t *testing.T) {
		f := &Fs{}
		assert.Equal(t, "ready", f.processingStates())
	})

	t.Run("include_processing adds every pre-ready pipeline state", func(t *testing.T) {
		f := &Fs{opt: Options{IncludeProcessing: true}}
		assert.Equal(t, "ready,uploading,registered,transcoding,stabilizing", f.processingStates())
	})

	t.Run("include_failed adds failure and unknown", func(t *testing.T) {
		f := &Fs{opt: Options{IncludeFailed: true}}
		assert.Equal(t, "ready,failure,unknown", f.processingStates())
	})

	t.Run("both options combine", func(t *testing.T) {
		f := &Fs{opt: Options{IncludeProcessing: true, IncludeFailed: true}}
		assert.Equal(t, "ready,uploading,registered,transcoding,stabilizing,failure,unknown", f.processingStates())
	})
}

func TestInvalidateCaches(t *testing.T) {
	f := &Fs{
		media: cachedList([]api.Medium{{ID: "1"}}),
		trash: cachedList([]api.Medium{{ID: "2"}}),
	}

	f.invalidateMediaCache()
	assert.Nil(t, f.media.items)
	assert.NotNil(t, f.trash.items, "invalidating the media cache must leave the trash cache alone")

	f.invalidateTrashCache()
	assert.Nil(t, f.trash.items)
}

func TestAllMediaCacheHit(t *testing.T) {
	// f.srv is deliberately left nil: a cache hit must return without ever
	// dereferencing it, so a nil-pointer panic here means the TTL check is
	// broken, not that the assertion below failed.
	want := []api.Medium{{ID: "cached"}}
	f := &Fs{media: cachedList(want)}
	got, err := f.allMedia(context.Background())
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestAllTrashCacheHit(t *testing.T) {
	want := []api.Medium{{ID: "cached"}}
	f := &Fs{trash: cachedList(want)}
	got, err := f.allTrash(context.Background())
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// newTestListFs builds a minimal *Fs whose srv talks to a local
// httptest.Server, for testing allMedia/allTrash without a live account -
// both only ever use f.srv, f.pacer and f.opt.
func newTestListFs(baseURL string) *Fs {
	f := &Fs{
		srv:   rest.NewClient(&http.Client{}).SetRoot(baseURL),
		pacer: fs.NewPacer(context.Background(), pacer.NewDefault(pacer.MinSleep(time.Millisecond), pacer.MaxSleep(5*time.Millisecond))),
		media: &listCache{},
		trash: &listCache{},
	}
	f.srv.SetErrorHandler(errorHandler)
	return f
}

func jsonHandler(t *testing.T, wantPath string, respond func(r *http.Request) any) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, wantPath, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(respond(r)))
	}
}

func TestAllMediaFetchesEveryPageAndDedupsTheBoundary(t *testing.T) {
	pages := [][]api.Medium{
		{{ID: "1"}, {ID: "2"}},
		{{ID: "2"}, {ID: "3"}}, // GoPro's own page boundary can repeat the last item of the previous page
	}
	var gotParams []url.Values
	srv := httptest.NewServer(jsonHandler(t, "/media/search", func(r *http.Request) any {
		gotParams = append(gotParams, r.URL.Query())
		page, err := strconv.Atoi(r.URL.Query().Get("page"))
		require.NoError(t, err)
		resp := &api.SearchResponse{Pages: api.PageInfo{TotalPages: len(pages)}}
		resp.Embedded.Media = pages[page-1]
		return resp
	}))
	defer srv.Close()

	f := newTestListFs(srv.URL)
	items, err := f.allMedia(context.Background())
	require.NoError(t, err)
	var ids []string
	for _, m := range items {
		ids = append(ids, m.ID)
	}
	assert.Equal(t, []string{"1", "2", "3"}, ids)
	require.Len(t, gotParams, 2)
	assert.Equal(t, includedTypes, gotParams[0].Get("type"), "the default type filter is sent unless show_all is set")
	assert.Equal(t, "ready", gotParams[0].Get("processing_states"))
	assert.Equal(t, "export", gotParams[0].Get("xcomposition"))
}

func TestAllMediaShowAllOmitsEveryServerSideFilter(t *testing.T) {
	var gotParams url.Values
	srv := httptest.NewServer(jsonHandler(t, "/media/search", func(r *http.Request) any {
		gotParams = r.URL.Query()
		resp := &api.SearchResponse{Pages: api.PageInfo{TotalPages: 1}}
		resp.Embedded.Media = []api.Medium{{ID: "1"}}
		return resp
	}))
	defer srv.Close()

	f := newTestListFs(srv.URL)
	f.opt.ShowAll = true
	_, err := f.allMedia(context.Background())
	require.NoError(t, err)
	assert.Empty(t, gotParams.Get("type"))
	assert.Empty(t, gotParams.Get("processing_states"))
	assert.Empty(t, gotParams.Get("xcomposition"))
}

func TestAllMediaCachesWithinTTLAndRefetchesAfter(t *testing.T) {
	var calls int
	srv := httptest.NewServer(jsonHandler(t, "/media/search", func(r *http.Request) any {
		calls++
		resp := &api.SearchResponse{Pages: api.PageInfo{TotalPages: 1}}
		resp.Embedded.Media = []api.Medium{{ID: "1"}}
		return resp
	}))
	defer srv.Close()

	f := newTestListFs(srv.URL)
	_, err := f.allMedia(context.Background())
	require.NoError(t, err)
	_, err = f.allMedia(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, calls, "a second call within the TTL must be served from cache, not refetched")

	f.media.at = time.Now().Add(-mediaCacheTTL - time.Second)
	_, err = f.allMedia(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, calls, "a call after the TTL has elapsed must refetch")
}

func TestAllMediaCachesASuccessfulEmptyResult(t *testing.T) {
	var calls int
	srv := httptest.NewServer(jsonHandler(t, "/media/search", func(r *http.Request) any {
		calls++
		resp := &api.SearchResponse{Pages: api.PageInfo{TotalPages: 1}}
		resp.Embedded.Media = nil
		return resp
	}))
	defer srv.Close()

	f := newTestListFs(srv.URL)
	_, err := f.allMedia(context.Background())
	require.NoError(t, err)
	_, err = f.allMedia(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, calls, "an empty library must still be cached, not refetched on every call")
}

func TestAllTrashAppliesNoFilteringAtAll(t *testing.T) {
	// Trash always shows everything, matching GoPro's own "Recently
	// Deleted" view - none of include_edits/include_processing/
	// include_failed/show_all make any difference here, since there's
	// nothing left to opt into.
	items := []api.Medium{
		{ID: "ready", Type: "Video", ReadyToView: "ready"},
		{ID: "edit", Type: "Edit", ReadyToView: "ready"},
		{ID: "processing", Type: "Video", ReadyToView: "transcoding"},
		{ID: "failed", Type: "Video", ReadyToView: "failure"},
		{ID: "unknown-state", Type: "Video", ReadyToView: "some-future-state"},
	}
	srv := httptest.NewServer(jsonHandler(t, "/media/deleted", func(r *http.Request) any {
		return &api.DeletedMediaResponse{DeletedMedia: items, Pages: api.PageInfo{TotalPages: 1}}
	}))
	defer srv.Close()

	for _, tc := range []struct {
		name string
		opt  Options
	}{
		{"every option at its default", Options{}},
		{"include_edits/include_processing/include_failed all on", Options{IncludeEdits: true, IncludeProcessing: true, IncludeFailed: true}},
		{"show_all on", Options{ShowAll: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newTestListFs(srv.URL)
			f.opt = tc.opt
			got, err := f.allTrash(context.Background())
			require.NoError(t, err)
			var ids []string
			for _, m := range got {
				ids = append(ids, m.ID)
			}
			assert.ElementsMatch(t, []string{"ready", "edit", "processing", "failed", "unknown-state"}, ids)
		})
	}
}

func TestAllTrashCachesASuccessfulEmptyResult(t *testing.T) {
	var calls int
	srv := httptest.NewServer(jsonHandler(t, "/media/deleted", func(r *http.Request) any {
		calls++
		return &api.DeletedMediaResponse{DeletedMedia: nil, Pages: api.PageInfo{TotalPages: 1}}
	}))
	defer srv.Close()

	f := newTestListFs(srv.URL)
	_, err := f.allTrash(context.Background())
	require.NoError(t, err)
	_, err = f.allTrash(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, calls, "an empty trash must still be cached, not refetched on every call")
}

func TestAllMediaFetchesTheRemainingPagesConcurrently(t *testing.T) {
	const nPages = 5
	var mu sync.Mutex
	var inFlight, maxInFlight int
	var perPage []string
	srv := httptest.NewServer(jsonHandler(t, "/media/search", func(r *http.Request) any {
		mu.Lock()
		inFlight++
		maxInFlight = max(maxInFlight, inFlight)
		perPage = append(perPage, r.URL.Query().Get("per_page"))
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		inFlight--
		mu.Unlock()
		page, err := strconv.Atoi(r.URL.Query().Get("page"))
		require.NoError(t, err)
		resp := &api.SearchResponse{Pages: api.PageInfo{TotalPages: nPages}}
		resp.Embedded.Media = []api.Medium{{ID: strconv.Itoa(page)}}
		return resp
	}))
	defer srv.Close()

	f := newTestListFs(srv.URL)
	items, err := f.allMedia(context.Background())
	require.NoError(t, err)
	var ids []string
	for _, m := range items {
		ids = append(ids, m.ID)
	}
	assert.Equal(t, []string{"1", "2", "3", "4", "5"}, ids, "items must stay in page order")
	assert.Greater(t, maxInFlight, 1, "pages after the first must be fetched concurrently")
	for _, pp := range perPage {
		assert.Equal(t, strconv.Itoa(listPageSize), pp)
	}
}

func TestListCacheIsSharedBetweenFsOfTheSameRemote(t *testing.T) {
	ctx := context.Background()
	newFs := func(name, root string, extra configmap.Simple) *Fs {
		m := configmap.Simple{"access_token": "test-token", "verify_size": verifySizeReprocessed}
		for k, v := range extra {
			m[k] = v
		}
		f, err := NewFs(ctx, name, root, m)
		require.NoError(t, err)
		return f.(*Fs)
	}
	a := newFs("shared-cache-test", "", nil)
	b := newFs("shared-cache-test", "media/by-year", nil)
	assert.Same(t, a.media, b.media, "different roots of one remote must share the library listing")
	assert.Same(t, a.trash, b.trash)

	other := newFs("shared-cache-test-other", "", nil)
	assert.NotSame(t, a.media, other.media, "different remotes must not share a listing")
	assert.NotSame(t, a.trash, other.trash)

	showAll := newFs("shared-cache-test", "", configmap.Simple{"show_all": "true"})
	assert.NotSame(t, a.media, showAll.media, "options that change the listing must not share it")
	assert.Same(t, a.trash, showAll.trash, "the trash is unfiltered, so it's shared regardless")

	a.media.items = []api.Medium{}
	b.invalidateMediaCache()
	assert.Nil(t, a.media.items, "invalidating through one Fs must invalidate them all")
}

func TestListDirNamesUnnamedEditsAsMP4(t *testing.T) {
	// Auto-generated Highlights, as served live: empty filename, the EDL's
	// "json" file_extension, null file_size - but the download is an MP4.
	const id = "6a9362c0b7d89053ceb33de9"
	items := []api.Medium{
		{ID: id, Type: "MultiClipEdit", FileExtension: "json", ItemCount: 1, CapturedAt: fstest.Time("2024-01-01T00:00:00Z")},
		{ID: "6a9362c08c23f474301c0899", Type: "MultiClipEdit", Filename: "named", FileExtension: "json", ItemCount: 1, CapturedAt: fstest.Time("2024-01-01T00:00:00Z")},
	}
	f := newTestMediaFs(items)
	f.opt.AlwaysAddID = false
	entries, err := f.List(context.Background(), "media/all")
	require.NoError(t, err)
	require.Len(t, entries, 2)

	got := map[string]string{}
	for _, entry := range entries {
		got[entry.Remote()] = entry.(*Object).MimeType(context.Background())
	}
	assert.Equal(t, map[string]string{
		"media/all/{" + id + "}.mp4": "video/mp4",
		"media/all/named.mp4":        "video/mp4",
	}, got)

	t.Run("the always_add_id fast path expects the same name", func(t *testing.T) {
		assert.Equal(t, "{"+id+"}.mp4", expectedIDSuffixedName(f, &items[0]))
		assert.Equal(t, id, findID("{"+id+"}.mp4"))
	})

	t.Run("a non-edit without a filename extension gets its file_extension", func(t *testing.T) {
		assert.Equal(t, "GX010001.MP4", mediumLeaf(f, &api.Medium{Filename: "GX010001.MP4", FileExtension: "mp4"}))
		assert.Equal(t, "GX010001.jpg", mediumLeaf(f, &api.Medium{Filename: "GX010001", FileExtension: "jpg"}))
		assert.Equal(t, "GX010001", mediumLeaf(f, &api.Medium{Filename: "GX010001"}))
	})
}

func TestListDirShowsNullFileSizeTrashedItemsUnconditionally(t *testing.T) {
	// A null file_size item is skipped in the active library by default,
	// but never in a trashed listing - restoring or permanently deleting
	// it doesn't need a usable size, so there's nothing to protect by
	// hiding it, matching GoPro's own "Recently Deleted" view.
	items := []api.Medium{
		{ID: "1", Filename: "broken.mp4", ItemCount: 1, CapturedAt: fstest.Time("2024-01-01T00:00:00Z")},
	}

	t.Run("the active library still hides it by default", func(t *testing.T) {
		f := newTestMediaFs(items)
		entries, err := f.List(context.Background(), "media/all")
		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("a trashed listing shows it unconditionally", func(t *testing.T) {
		f := newTestMediaFs(nil)
		f.opt.TrashedOnly = true
		f.trash = cachedList(items)
		entries, err := f.List(context.Background(), "media/all")
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, "media/all/broken {1}.mp4", entries[0].Remote())
	})
}

func TestListDirShowsFailedItemsWhenIncludeFailedIsSet(t *testing.T) {
	// include_failed's whole point is to surface stuck failure/unknown
	// media so it can be inspected or removed - such media is never
	// confirmed to have a usable file_size, so the null-size skip must
	// not undo what include_failed just opted into.
	items := []api.Medium{
		{ID: "1", Filename: "stuck.mp4", ItemCount: 1, ReadyToView: "failure", CapturedAt: fstest.Time("2024-01-01T00:00:00Z")},
		{ID: "2", Filename: "unknown.mp4", ItemCount: 1, ReadyToView: "unknown", CapturedAt: fstest.Time("2024-01-01T00:00:00Z")},
	}
	f := newTestMediaFs(items)
	f.opt.IncludeFailed = true

	entries, err := f.List(context.Background(), "media/all")
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Remote())
	}
	assert.ElementsMatch(t, []string{"media/all/stuck {1}.mp4", "media/all/unknown {2}.mp4"}, names)
}

func TestListedMedia(t *testing.T) {
	ctx := context.Background()
	media := []api.Medium{{ID: "1"}}
	trash := []api.Medium{{ID: "2"}}

	t.Run("reads the library by default", func(t *testing.T) {
		f := &Fs{media: cachedList(media), trash: cachedList(trash)}
		got, err := f.listedMedia(ctx)
		require.NoError(t, err)
		assert.Equal(t, media, got)
	})

	t.Run("reads the trash under trashed_only", func(t *testing.T) {
		f := &Fs{opt: Options{TrashedOnly: true}, media: cachedList(media), trash: cachedList(trash)}
		got, err := f.listedMedia(ctx)
		require.NoError(t, err)
		assert.Equal(t, trash, got)
	})
}

func TestStartYear(t *testing.T) {
	ctx := context.Background()

	t.Run("an explicit start_year override always wins, even over library content", func(t *testing.T) {
		f := &Fs{
			opt:   Options{StartYear: 1999},
			media: cachedList([]api.Medium{{CapturedAt: fstest.Time("2020-01-01T00:00:00Z")}}),
		}
		assert.Equal(t, 1999, f.startYear(ctx))
	})

	t.Run("without an override, scans every cached item for the true minimum regardless of position", func(t *testing.T) {
		f := &Fs{media: cachedList([]api.Medium{
			{CapturedAt: fstest.Time("2024-06-01T00:00:00Z")},
			{CapturedAt: fstest.Time("2016-01-01T00:00:00Z")}, // earliest - neither first nor last
			{CapturedAt: fstest.Time("2020-01-01T00:00:00Z")},
		})}
		assert.Equal(t, 2016, f.startYear(ctx))
	})

	t.Run("an empty library falls back to the current year", func(t *testing.T) {
		f := &Fs{startTime: startTime, media: cachedList([]api.Medium{})}
		assert.Equal(t, startTime.Year(), f.startYear(ctx))
	})

	t.Run("under trashed_only, scans the trash instead of the active library", func(t *testing.T) {
		f := &Fs{
			opt:   Options{TrashedOnly: true},
			media: cachedList([]api.Medium{{CapturedAt: fstest.Time("2024-06-01T00:00:00Z")}}),
			trash: cachedList([]api.Medium{
				{CapturedAt: fstest.Time("2010-01-01T00:00:00Z")},
				{CapturedAt: fstest.Time("2020-01-01T00:00:00Z")},
			}),
		}
		assert.Equal(t, 2010, f.startYear(ctx), "the active library's 2024 item must not win over the trash's earlier 2010 one")
	})
}

func TestShowEmptyDirsGetter(t *testing.T) {
	assert.False(t, (&Fs{}).showEmptyDirs())
	assert.True(t, (&Fs{opt: Options{ShowEmptyDirs: true}}).showEmptyDirs())
}

func TestCapturedDates(t *testing.T) {
	ctx := context.Background()
	mediaTime := fstest.Time("2020-01-01T00:00:00Z")
	trashTime := fstest.Time("2021-01-01T00:00:00Z")

	t.Run("reads the library by default", func(t *testing.T) {
		f := &Fs{media: cachedList([]api.Medium{{CapturedAt: mediaTime}})}
		got, err := f.capturedDates(ctx)
		require.NoError(t, err)
		assert.Equal(t, []time.Time{mediaTime}, got)
	})

	t.Run("reads the trash instead under trashed_only", func(t *testing.T) {
		f := &Fs{
			opt:   Options{TrashedOnly: true},
			trash: cachedList([]api.Medium{{CapturedAt: trashTime}}),
		}
		got, err := f.capturedDates(ctx)
		require.NoError(t, err)
		assert.Equal(t, []time.Time{trashTime}, got)
	})
}

func TestFsAccessors(t *testing.T) {
	features := &fs.Features{}
	f := &Fs{name: "remote", root: "some/root", features: features}
	assert.Equal(t, "remote", f.Name())
	assert.Equal(t, "some/root", f.Root())
	assert.Equal(t, `GoPro Media Library path "some/root"`, f.String())
	assert.Equal(t, fs.ModTimeNotSupported, f.Precision())
	assert.Equal(t, hash.Set(hash.None), f.Hashes())
	assert.Same(t, features, f.Features())
}

func TestObjectAccessors(t *testing.T) {
	f := &Fs{}
	o := &Object{fs: f, remote: "media/all/x.mp4", id: "abc123", mimeType: "video/mp4"}
	assert.Equal(t, f, o.Fs())
	assert.Equal(t, "media/all/x.mp4", o.String())
	assert.Equal(t, "media/all/x.mp4", o.Remote())
	assert.True(t, o.Storable())
	assert.Equal(t, "video/mp4", o.MimeType(context.Background()))
	assert.Equal(t, "abc123", o.ID())

	h, err := o.Hash(context.Background(), hash.MD5)
	assert.Equal(t, "", h)
	assert.Equal(t, hash.ErrUnsupported, err)

	var nilObj *Object
	assert.Equal(t, "<nil>", nilObj.String())
}

func TestMediumTypeForFilename(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"GOPR0001.JPG", "Photo"},
		{"gopr0001.jpeg", "Photo"},
		{"photo.GPR", "Photo"},
		{"photo.dng", "Photo"},
		{"photo.png", "Photo"},
		{"photo.heic", "Photo"},
		{"GX010001.MP4", "Video"},
		{"noext", "Video"},
	} {
		assert.Equal(t, tc.want, mediumTypeForFilename(tc.name), tc.name)
	}
}

func TestRestoreArg(t *testing.T) {
	id := "68b22325df3cf752557ac6d7"
	assert.Equal(t, id, restoreArg(id))
	assert.Equal(t, id, restoreArg("GX010294 {"+id+"}.MP4"))
	assert.Equal(t, id, restoreArg("trash/GX010294 {"+id+"}.MP4"))
	assert.Equal(t, "not-an-id", restoreArg("not-an-id"))
}

// newTestUploadFs builds a minimal *Fs with a real, empty upload/ dirtree -
// mirroring what NewFs seeds - for testing Mkdir/Rmdir/List over upload/
// without a live account.
func newTestUploadFs(root string) *Fs {
	f := &Fs{root: root, startTime: startTime, uploaded: dirtree.New()}
	if root == "" {
		f.uploaded["upload"] = nil
	}
	return f
}

func TestMkdirRmdirListUploads(t *testing.T) {
	f := newTestUploadFs("")
	ctx := context.Background()

	t.Run("the upload root lists empty from the start", func(t *testing.T) {
		entries, err := f.List(ctx, "upload")
		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("Mkdir creates an upload subdirectory, visible in its parent's listing", func(t *testing.T) {
		require.NoError(t, f.Mkdir(ctx, "upload/dir"))
		entries, err := f.List(ctx, "upload")
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, "upload/dir", entries[0].Remote())
	})

	t.Run("Mkdir outside upload/ is refused - every other directory is synthetic", func(t *testing.T) {
		assert.Equal(t, errCantMkdir, f.Mkdir(ctx, "media/all"))
	})

	t.Run("Mkdir is idempotent - calling it again doesn't duplicate the entry", func(t *testing.T) {
		require.NoError(t, f.Mkdir(ctx, "upload/dir"))
		entries, err := f.List(ctx, "upload")
		require.NoError(t, err)
		require.Len(t, entries, 1, "a second Mkdir on the same directory must not add a duplicate listing entry")
		assert.Equal(t, "upload/dir", entries[0].Remote())
	})

	t.Run("Rmdir removes an upload subdirectory again", func(t *testing.T) {
		require.NoError(t, f.Rmdir(ctx, "upload/dir"))
		entries, err := f.List(ctx, "upload")
		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("Rmdir outside upload/ is refused", func(t *testing.T) {
		assert.Equal(t, errCantRmdir, f.Rmdir(ctx, "media/all"))
	})

	t.Run("Rmdir refuses a non-empty upload directory", func(t *testing.T) {
		require.NoError(t, f.Mkdir(ctx, "upload/parent"))
		require.NoError(t, f.Mkdir(ctx, "upload/parent/child"))

		err := f.Rmdir(ctx, "upload/parent")
		assert.Equal(t, fs.ErrorDirectoryNotEmpty, err)

		entries, err := f.List(ctx, "upload/parent")
		require.NoError(t, err)
		require.Len(t, entries, 1, "the child must survive a refused Rmdir, not be silently pruned")
		assert.Equal(t, "upload/parent/child", entries[0].Remote())
	})

	t.Run("listing an unknown directory is ErrorDirNotFound", func(t *testing.T) {
		_, err := f.List(ctx, "not-a-real-directory")
		assert.Equal(t, fs.ErrorDirNotFound, err)
	})
}

func TestUploadRootDoesNotCreateNestedUploadDirectory(t *testing.T) {
	f := newTestUploadFs("upload")

	entries, err := f.List(context.Background(), "")
	require.NoError(t, err)
	assert.Empty(t, entries)

	_, err = f.List(context.Background(), "upload")
	assert.Equal(t, fs.ErrorDirNotFound, err)
}

// newTestMediaFs builds a minimal *Fs with items pre-seeded as the cached
// full library, so List/listDir can be exercised without a live account -
// f.srv is deliberately left nil, so a bug that fell through to the network
// would panic rather than silently pass.
func newTestMediaFs(items []api.Medium) *Fs {
	return &Fs{
		startTime: startTime,
		opt:       Options{AlwaysAddID: true},
		uploaded:  dirtree.New(),
		media:     cachedList(items),
	}
}

func TestListDirAndListMediaAll(t *testing.T) {
	size := int64(100)
	items := []api.Medium{
		{ID: "1", Filename: "GX010001.MP4", FileSize: &size, ItemCount: 1, CapturedAt: fstest.Time("2024-01-01T00:00:00Z")},
		{ID: "2", Filename: "GX010001.MP4", FileSize: &size, ItemCount: 1, CapturedAt: fstest.Time("2024-01-02T00:00:00Z")}, // filename collision with id 1
		{ID: "3", Filename: "GPAA0001.JPG", FileSize: &size, ItemCount: 3, CapturedAt: fstest.Time("2024-01-03T00:00:00Z")}, // burst - file_size is the total across all 3, still non-null
		{ID: "4", Filename: "broken.mp4", ItemCount: 1, CapturedAt: fstest.Time("2024-01-04T00:00:00Z")},                    // ready but null file_size - skipped unless show_all
	}

	t.Run("defaults skip the null file_size item and suffix every entry with always_add_id", func(t *testing.T) {
		f := newTestMediaFs(items)
		entries, err := f.List(context.Background(), "media/all")
		require.NoError(t, err)
		var names []string
		for _, e := range entries {
			names = append(names, e.Remote())
		}
		assert.ElementsMatch(t, []string{
			"media/all/GX010001 {1}.MP4",
			"media/all/GX010001 {2}.MP4",
			"media/all/GPAA0001-1 {3}.JPG",
			"media/all/GPAA0001-2 {3}.JPG",
			"media/all/GPAA0001-3 {3}.JPG",
		}, names)
	})

	t.Run("show_all lists the null file_size item too", func(t *testing.T) {
		f := newTestMediaFs(items)
		f.opt.ShowAll = true
		entries, err := f.List(context.Background(), "media/all")
		require.NoError(t, err)
		var names []string
		for _, e := range entries {
			names = append(names, e.Remote())
		}
		assert.Contains(t, names, "media/all/broken {4}.mp4")
	})
}

func TestListDispatchesRootAndMedia(t *testing.T) {
	f := newTestMediaFs(nil)
	ctx := context.Background()

	root, err := f.List(ctx, "")
	require.NoError(t, err)
	var rootNames []string
	for _, e := range root {
		rootNames = append(rootNames, e.Remote())
	}
	assert.ElementsMatch(t, []string{"media", "upload"}, rootNames)

	media, err := f.List(ctx, "media")
	require.NoError(t, err)
	var mediaNames []string
	for _, e := range media {
		mediaNames = append(mediaNames, e.Remote())
	}
	assert.ElementsMatch(t, []string{"media/all", "media/by-year", "media/by-month", "media/by-day"}, mediaNames)

	_, err = f.List(ctx, "not-a-real-directory")
	assert.Equal(t, fs.ErrorDirNotFound, err)
}

func TestNewFsWithStaticAccessToken(t *testing.T) {
	// A static access_token skips the OAuth machinery entirely, and root=""
	// never triggers NewFs's own "is the root a file" probe - so this
	// exercises NewFs's setup (upload/ root seeding, in particular) without
	// any network access or live account.
	// verify_size has no Go zero value default, so it must be supplied
	// explicitly here - configmap.Simple bypasses the registered-option
	// Default filling that a real config section gets in production.
	m := configmap.Simple{"access_token": "test-token", "verify_size": verifySizeReprocessed}
	fsIface, err := NewFs(context.Background(), "test", "", m)
	require.NoError(t, err)
	f, ok := fsIface.(*Fs)
	require.True(t, ok)
	assert.Equal(t, "test", f.Name())
	assert.Equal(t, "", f.Root())

	entries, err := f.List(context.Background(), "upload")
	require.NoError(t, err, "the upload root must be seeded so listing it doesn't return ErrorDirNotFound before anything has ever been uploaded")
	assert.Empty(t, entries)
}

func TestNewFsWithUploadRoot(t *testing.T) {
	m := configmap.Simple{"access_token": "test-token", "verify_size": verifySizeReprocessed}
	fsIface, err := NewFs(context.Background(), "test", "upload", m)
	require.NoError(t, err)
	f := fsIface.(*Fs)

	entries, err := f.List(context.Background(), "")
	require.NoError(t, err)
	assert.Empty(t, entries)

	_, err = f.List(context.Background(), "upload")
	assert.Equal(t, fs.ErrorDirNotFound, err)
}

func TestCurrentAccessToken(t *testing.T) {
	f := &Fs{opt: Options{AccessToken: "static-token"}}
	tok, err := f.currentAccessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "static-token", tok)
}

func TestGetResourceOwnerIDCacheHit(t *testing.T) {
	// f.srv is left nil: a cache hit must never reach the network.
	f := &Fs{resourceOwnerID: "cached-id"}
	rid, err := f.getResourceOwnerID(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "cached-id", rid)
}

func TestGetResourceOwnerIDFallsBackToUserInfo(t *testing.T) {
	f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/media/user", r.URL.Path)
		writeJSON(t, w, api.UserInfo{ID: "user-from-api"})
	}))
	defer srv.Close()

	rid, err := f.getResourceOwnerID(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "user-from-api", rid)
	assert.Equal(t, "user-from-api", f.resourceOwnerID, "the result must be cached for next time")
}

func TestGetUserInfoAndAbout(t *testing.T) {
	f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, api.UserInfo{
			ID:                    "u1",
			NonExemptStorageLimit: 1000,
			TotalStorage:          1500,
			NonExempt:             api.StorageBucket{TotalStorage: 400},
		})
	}))
	defer srv.Close()

	usage, err := f.About(context.Background())
	require.NoError(t, err)
	require.NotNil(t, usage.Used)
	require.NotNil(t, usage.Total)
	require.NotNil(t, usage.Free)
	// Used must share NonExempt's base with Total/Free (1000, both
	// non-exempt-only) rather than the combined 1500 TotalStorage, or
	// usage reports as "used more than total".
	assert.Equal(t, int64(400), *usage.Used)
	assert.Equal(t, int64(1000), *usage.Total)
	assert.Equal(t, int64(600), *usage.Free)
}

func TestAboutClampsNegativeFreeToZero(t *testing.T) {
	f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, api.UserInfo{
			NonExemptStorageLimit: 100,
			NonExempt:             api.StorageBucket{TotalStorage: 500}, // over quota
		})
	}))
	defer srv.Close()

	usage, err := f.About(context.Background())
	require.NoError(t, err)
	require.NotNil(t, usage.Free)
	assert.Equal(t, int64(0), *usage.Free)
}

func TestGetMedium(t *testing.T) {
	f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/media/abc123", r.URL.Path)
		assert.Equal(t, mediaFields, r.URL.Query().Get("fields"))
		writeJSON(t, w, api.Medium{ID: "abc123", Filename: "x.mp4"})
	}))
	defer srv.Close()

	item, err := f.getMedium(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "x.mp4", item.Filename)
}

func TestUpdateMediumInvalidatesMediaCache(t *testing.T) {
	f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/media/abc123", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	f.media = cachedList([]api.Medium{{ID: "stale"}})

	name := "new-name"
	err := f.updateMedium(context.Background(), "abc123", api.MediumUpdate{Filename: &name})
	require.NoError(t, err)
	assert.Nil(t, f.media.items, "a change to a medium must invalidate the cached listing")
}

func TestCreateCollectionAddToCollectionAndPublicLink(t *testing.T) {
	var gotCreate api.CollectionCreate
	var gotAdd api.CollectionMediaUpdate
	mux := http.NewServeMux()
	mux.HandleFunc("POST /collections", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotCreate))
		writeJSON(t, w, api.Collection{ID: "col1"})
	})
	mux.HandleFunc("PUT /collections/col1", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotAdd))
		w.WriteHeader(http.StatusNoContent)
	})
	f, srv := newTestAPIFs(mux)
	defer srv.Close()

	t.Run("createCollection and addToCollection send the right bodies", func(t *testing.T) {
		id, err := f.createCollection(context.Background(), "my title", true)
		require.NoError(t, err)
		assert.Equal(t, "col1", id)
		assert.Equal(t, "my title", gotCreate.Title)
		assert.True(t, gotCreate.Cloneable)

		require.NoError(t, f.addToCollection(context.Background(), "col1", "med1"))
		assert.Equal(t, []string{"med1"}, gotAdd.MediaIDs)
	})

	t.Run("PublicLink orchestrates both calls and defaults the title to the file's own name", func(t *testing.T) {
		size := int64(100)
		f.media = cachedList([]api.Medium{{ID: "med1", Filename: "clip.mp4", FileSize: &size, ItemCount: 1, CapturedAt: startTime}})
		link, err := f.PublicLink(context.Background(), "media/all/clip.mp4", fs.Duration(0), false)
		require.NoError(t, err)
		assert.Equal(t, "https://gopro.com/v/col1", link)
		assert.Equal(t, "clip.mp4", gotCreate.Title)
	})

	t.Run("PublicLink titles an unnamed medium with its listed name, not a bare extension", func(t *testing.T) {
		id := "6a9362c0b7d89053ceb33de9"
		f.media = cachedList(append(f.media.items, api.Medium{ID: id, Filename: "", FileExtension: "json", Type: "MultiClipEdit", ItemCount: 1, CapturedAt: startTime}))
		_, err := f.PublicLink(context.Background(), "media/all/{"+id+"}.mp4", fs.Duration(0), false)
		require.NoError(t, err)
		assert.Equal(t, "{"+id+"}.mp4", gotCreate.Title)
	})

	t.Run("PublicLink prefers an explicit link_title over the file's own name", func(t *testing.T) {
		f.opt.LinkTitle = "custom title"
		_, err := f.PublicLink(context.Background(), "media/all/clip.mp4", fs.Duration(0), false)
		require.NoError(t, err)
		assert.Equal(t, "custom title", gotCreate.Title)
	})
}

func TestGetDownloadCachesResult(t *testing.T) {
	var calls int
	f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		assert.Equal(t, "/media/abc123/download", r.URL.Path)
		writeJSON(t, w, api.DownloadResponse{Filename: "x.mp4"})
	}))
	defer srv.Close()

	dl, err := f.getDownload(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "x.mp4", dl.Filename)

	dl2, err := f.getDownload(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "x.mp4", dl2.Filename)
	assert.Equal(t, 1, calls, "a second call within the TTL must be served from cache")
}

func TestGetDownloadPrunesExpiredEntries(t *testing.T) {
	f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, api.DownloadResponse{Filename: "x.mp4"})
	}))
	defer srv.Close()
	f.dlCache["expired"] = &dlCacheEntry{resp: &api.DownloadResponse{}, fetched: time.Now().Add(-2 * dlCacheTTL)}
	f.dlCache["fresh"] = &dlCacheEntry{resp: &api.DownloadResponse{}, fetched: time.Now()}

	_, err := f.getDownload(context.Background(), "new")
	require.NoError(t, err)
	assert.NotContains(t, f.dlCache, "expired", "expired entries must not accumulate for the Fs's lifetime")
	assert.Contains(t, f.dlCache, "fresh")
	assert.Contains(t, f.dlCache, "new")
}

func TestDoDeleteMediumInvalidatesCachesAndReportsAPIErrors(t *testing.T) {
	t.Run("success invalidates both caches", func(t *testing.T) {
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "DELETE", r.Method)
			assert.Equal(t, "abc123", r.URL.Query().Get("ids"))
			writeJSON(t, w, api.DeleteResponse{})
		}))
		defer srv.Close()
		f.media = cachedList([]api.Medium{{ID: "stale"}})
		f.trash = cachedList([]api.Medium{{ID: "stale"}})

		require.NoError(t, f.doDeleteMedium(context.Background(), "abc123"))
		assert.Nil(t, f.media.items)
		assert.Nil(t, f.trash.items)
	})

	t.Run("an error embedded in a 200 response is surfaced as a Go error", func(t *testing.T) {
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, api.DeleteResponse{Embedded: struct {
				Errors []api.EmbeddedError `json:"errors"`
			}{Errors: []api.EmbeddedError{{Description: "not found or inaccessible"}}}})
		}))
		defer srv.Close()

		err := f.doDeleteMedium(context.Background(), "abc123")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found or inaccessible")
	})

	t.Run("extra query parameters (e.g. permanent=true) are passed through", func(t *testing.T) {
		var gotPermanent string
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPermanent = r.URL.Query().Get("permanent")
			writeJSON(t, w, api.DeleteResponse{})
		}))
		defer srv.Close()

		require.NoError(t, f.doDeleteMedium(context.Background(), "abc123", "permanent", "true"))
		assert.Equal(t, "true", gotPermanent)
	})
}

func TestDeleteMediumPermanent(t *testing.T) {
	old := deletePermanentDelay
	deletePermanentDelay = time.Millisecond
	defer func() { deletePermanentDelay = old }()

	var calls []string
	f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Query().Get("permanent"))
		writeJSON(t, w, api.DeleteResponse{})
	}))
	defer srv.Close()

	require.NoError(t, f.deleteMedium(context.Background(), "abc123", true))
	require.Len(t, calls, 2, "a permanent delete must issue a plain delete, then a second finalising one")
	assert.Equal(t, "", calls[0])
	assert.Equal(t, "true", calls[1])
}

func TestDeleteMediumPermanentRespectsContextCancellation(t *testing.T) {
	old := deletePermanentDelay
	deletePermanentDelay = time.Hour // long enough that only cancellation ends the wait
	defer func() { deletePermanentDelay = old }()

	f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, api.DeleteResponse{})
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	err := f.deleteMedium(ctx, "abc123", true)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRemoveDispatchesByTrashedOnly(t *testing.T) {
	t.Run("trashed_only purges directly, skipping the plain-delete step", func(t *testing.T) {
		var gotPermanent string
		var calls int
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			gotPermanent = r.URL.Query().Get("permanent")
			writeJSON(t, w, api.DeleteResponse{})
		}))
		defer srv.Close()
		f.opt.TrashedOnly = true

		o := &Object{fs: f, id: "abc123"}
		require.NoError(t, o.Remove(context.Background()))
		assert.Equal(t, 1, calls, "trashed_only must issue exactly one DELETE, not the plain-then-permanent pair")
		assert.Equal(t, "true", gotPermanent)
	})

	t.Run("a normal remove with use_trash goes through the plain delete only", func(t *testing.T) {
		var calls int
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			assert.Empty(t, r.URL.Query().Get("permanent"))
			writeJSON(t, w, api.DeleteResponse{})
		}))
		defer srv.Close()
		f.opt.UseTrash = true

		o := &Object{fs: f, id: "abc123"}
		require.NoError(t, o.Remove(context.Background()))
		assert.Equal(t, 1, calls)
	})
}

func TestRemoveClearsTheUploadedTreeEntry(t *testing.T) {
	// Upload listings come solely from the in-memory f.uploaded tree, so
	// a successful delete that doesn't also remove the tree entry would
	// leave the object listed and resolvable for this Fs's lifetime.
	f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, api.DeleteResponse{})
	}))
	defer srv.Close()
	f.uploaded = dirtree.New()
	f.opt.UseTrash = true // skips deletePermanentDelay's real-time wait

	o := &Object{fs: f, id: "abc123", remote: "upload/x.mp4"}
	f.uploaded.Add(o)

	entries, err := f.List(context.Background(), "upload")
	require.NoError(t, err)
	require.Len(t, entries, 1)

	require.NoError(t, o.Remove(context.Background()))

	entries, err = f.List(context.Background(), "upload")
	require.NoError(t, err)
	assert.Empty(t, entries, "a deleted upload object must be removed from the in-memory upload tree, not remain listed")
}

func TestRestoreCommand(t *testing.T) {
	ctx := context.Background()

	t.Run("explicit ids are restored and both caches invalidated", func(t *testing.T) {
		var gotIDs []string
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body api.RestoreRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			gotIDs = body.IDs
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()
		f.media = cachedList([]api.Medium{{ID: "stale"}})
		f.trash = cachedList([]api.Medium{{ID: "stale"}})

		id := "68b22325df3cf752557ac6d7"
		result, err := f.Command(ctx, "restore", []string{"GX010294 {" + id + "}.MP4"}, nil)
		require.NoError(t, err)
		assert.Equal(t, &restoreResult{Restored: 1}, result)
		assert.Equal(t, []string{id}, gotIDs)
		assert.Nil(t, f.media.items)
		assert.Nil(t, f.trash.items)
	})

	t.Run("no arguments restores everything currently in the trash", func(t *testing.T) {
		var gotIDs []string
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body api.RestoreRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			gotIDs = body.IDs
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()
		f.trash = cachedList([]api.Medium{{ID: "t1"}, {ID: "t2"}})

		result, err := f.restore(ctx, nil)
		require.NoError(t, err)
		assert.Equal(t, &restoreResult{Restored: 2}, result)
		assert.ElementsMatch(t, []string{"t1", "t2"}, gotIDs)
	})

	t.Run("an empty trash with no arguments is a no-op, not an error", func(t *testing.T) {
		f := &Fs{trash: cachedList([]api.Medium{})}
		result, err := f.restore(ctx, nil)
		require.NoError(t, err)
		assert.Equal(t, &restoreResult{}, result)
	})

	t.Run("dry-run reports intent without calling restoreMedia", func(t *testing.T) {
		var called bool
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}))
		defer srv.Close()

		dryCtx, ci := fs.AddConfig(ctx)
		ci.DryRun = true
		result, err := f.restore(dryCtx, []string{"abc123"})
		require.NoError(t, err)
		assert.Equal(t, &restoreResult{}, result)
		assert.False(t, called, "dry-run must not issue the restore request")
	})

	t.Run("an unknown command name is rejected", func(t *testing.T) {
		f := &Fs{}
		_, err := f.Command(ctx, "not-a-real-command", nil, nil)
		assert.Equal(t, fs.ErrorCommandNotFound, err)
	})
}

// newTestUploadFlowFs builds an *Fs ready to drive a full OpenChunkWriter ->
// WriteChunk -> Close upload against handler, with the access-token and
// resource-owner-id fast paths pre-seeded so the test only has to implement
// the actual upload protocol endpoints, not the auth ones.
func newTestUploadFlowFs(handler http.Handler) (*Fs, *httptest.Server) {
	f, srv := newTestAPIFs(handler)
	f.opt.AccessToken = "test-token"
	f.resourceOwnerID = "test-user"
	return f, srv
}

func TestOpenChunkWriterWriteChunkCloseRegistersUpload(t *testing.T) {
	content := []byte("hello chunked gopro upload")
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("POST /media", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]string{"id": "med1"})
	})
	mux.HandleFunc("POST /derivatives", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]string{"id": "der1"})
	})
	mux.HandleFunc("POST /user-uploads", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]string{"id": "up1"})
	})
	mux.HandleFunc("GET /user-uploads/der1", func(w http.ResponseWriter, r *http.Request) {
		resp := api.UserUploadsResponse{}
		resp.Embedded.Authorizations = []api.UploadAuthorization{{URL: srv.URL + "/chunk/1", Part: 1}}
		writeJSON(t, w, resp)
	})
	var gotChunk []byte
	mux.HandleFunc("PUT /chunk/1", func(w http.ResponseWriter, r *http.Request) {
		var err error
		gotChunk, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("PUT /user-uploads/der1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("PUT /derivatives/der1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("PUT /media/med1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	f, srv := newTestUploadFlowFs(mux)
	defer srv.Close()
	f.media = cachedList([]api.Medium{{ID: "stale"}})

	src := mockobject.New("upload/GX010001.MP4").WithContent(content, mockobject.SeekModeRegular)
	ctx := context.Background()
	info, writer, err := f.OpenChunkWriter(ctx, "upload/GX010001.MP4", src)
	require.NoError(t, err)
	assert.Equal(t, f.opt.UploadConcurrency, info.Concurrency)

	n, err := writer.WriteChunk(ctx, 0, bytes.NewReader(content))
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), n)
	assert.Equal(t, content, gotChunk)

	require.NoError(t, writer.Close(ctx))
	assert.Nil(t, f.media.items, "a completed upload must invalidate the cached library listing")

	entries, err := f.List(ctx, "upload")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "upload/GX010001.MP4", entries[0].Remote())
	uploaded, ok := entries[0].(*Object)
	require.True(t, ok)
	assert.Equal(t, "med1", uploaded.id)
	assert.Equal(t, int64(len(content)), uploaded.bytes)
}

// TestOpenChunkWriterNestedUploadPathUsesLeafFilename exercises uploading
// into a virtual subdirectory under upload/. GoPro's API is flat - it has
// no concept of directories - so the filename sent to createMedium must be
// just the leaf, not the whole match[1] sub-path, which still contains a
// "/" and can't match this backend's single-leaf patterns once listed back.
func TestOpenChunkWriterNestedUploadPathUsesLeafFilename(t *testing.T) {
	content := []byte("hello nested gopro upload")
	var srv *httptest.Server
	var gotFilename string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /media", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		gotFilename, _ = body["filename"].(string)
		writeJSON(t, w, map[string]string{"id": "med1"})
	})
	mux.HandleFunc("POST /derivatives", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]string{"id": "der1"})
	})
	mux.HandleFunc("POST /user-uploads", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]string{"id": "up1"})
	})
	mux.HandleFunc("GET /user-uploads/der1", func(w http.ResponseWriter, r *http.Request) {
		resp := api.UserUploadsResponse{}
		resp.Embedded.Authorizations = []api.UploadAuthorization{{URL: srv.URL + "/chunk/1", Part: 1}}
		writeJSON(t, w, resp)
	})
	mux.HandleFunc("PUT /chunk/1", func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("PUT /user-uploads/der1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("PUT /derivatives/der1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("PUT /media/med1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	f, srv := newTestUploadFlowFs(mux)
	defer srv.Close()

	src := mockobject.New("upload/dir/GX010001.MP4").WithContent(content, mockobject.SeekModeRegular)
	ctx := context.Background()
	_, writer, err := f.OpenChunkWriter(ctx, "upload/dir/GX010001.MP4", src)
	require.NoError(t, err)
	assert.Equal(t, "GX010001.MP4", gotFilename, "the filename sent to GoPro must be the leaf name, not the whole virtual sub-path")

	_, err = writer.WriteChunk(ctx, 0, bytes.NewReader(content))
	require.NoError(t, err)
	require.NoError(t, writer.Close(ctx))
}

func TestOpenChunkWriterRejectsUnknownSize(t *testing.T) {
	f := &Fs{}
	src := mockobject.New("upload/x.mp4").WithContent(nil, mockobject.SeekModeRegular)
	src.SetUnknownSize(true)
	_, _, err := f.OpenChunkWriter(context.Background(), "upload/x.mp4", src)
	assert.Error(t, err)
}

func TestOpenChunkWriterRejectsNonUploadPaths(t *testing.T) {
	f := &Fs{}
	src := mockobject.New("media/all/x.mp4").WithContent([]byte("x"), mockobject.SeekModeRegular)
	_, _, err := f.OpenChunkWriter(context.Background(), "media/all/x.mp4", src)
	assert.Equal(t, errCantUpload, err)
}

func TestAbortDeletesTheMedium(t *testing.T) {
	old := deletePermanentDelay
	deletePermanentDelay = time.Millisecond
	defer func() { deletePermanentDelay = old }()

	var gotIDs []string
	f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIDs = append(gotIDs, r.URL.Query().Get("ids"))
		writeJSON(t, w, api.DeleteResponse{})
	}))
	defer srv.Close()

	w := &gpChunkWriter{f: f, mediumID: "med1"}
	require.NoError(t, w.Abort(context.Background()))
	assert.Equal(t, []string{"med1", "med1"}, gotIDs, "Abort always finalises permanently, regardless of --gopro-use-trash")
}

func TestSizeVerifiesAndCorrectsViaHead(t *testing.T) {
	var srv *httptest.Server
	var headCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("GET /media/abc123/download", func(w http.ResponseWriter, r *http.Request) {
		dl := makeDownloadResponse(nil, []testFile{{url: srv.URL + "/original.mp4", label: "source"}})
		writeJSON(t, w, dl)
	})
	mux.HandleFunc("HEAD /original.mp4", func(w http.ResponseWriter, r *http.Request) {
		headCalls++
		w.Header().Set("Content-Length", "12345")
		w.WriteHeader(http.StatusOK)
	})
	f, s := newTestAPIFs(mux)
	srv = s
	defer srv.Close()
	f.opt.VerifySize = verifySizeAlways

	o := &Object{fs: f, id: "abc123", bytes: 999, itemNumber: 1}
	assert.Equal(t, int64(12345), o.Size())
	assert.True(t, o.sizeChecked)
	assert.Equal(t, 1, headCalls)

	// A resolved size must be cached for this Object's lifetime - a second
	// call must not re-issue the HEAD.
	assert.Equal(t, int64(12345), o.Size())
	assert.Equal(t, 1, headCalls)
}

// closeTrackingBody wraps a response body to record whether the caller
// closed it, for TestSizeClosesHeadResponseBody.
type closeTrackingBody struct {
	io.Reader
	closed *bool
}

func (b *closeTrackingBody) Close() error {
	*b.closed = true
	return nil
}

// closeTrackingTransport wraps every response body it sees in a
// closeTrackingBody.
type closeTrackingTransport struct {
	closed *bool
}

func (t *closeTrackingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	resp.Body = &closeTrackingBody{Reader: resp.Body, closed: t.closed}
	return resp, nil
}

func TestSizeClosesHeadResponseBody(t *testing.T) {
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("GET /media/abc123/download", func(w http.ResponseWriter, r *http.Request) {
		dl := makeDownloadResponse(nil, []testFile{{url: srv.URL + "/original.mp4", label: "source"}})
		writeJSON(t, w, dl)
	})
	mux.HandleFunc("HEAD /original.mp4", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "12345")
		w.WriteHeader(http.StatusOK)
	})
	f, s := newTestAPIFs(mux)
	srv = s
	defer srv.Close()
	f.opt.VerifySize = verifySizeAlways

	var closed bool
	f.unAuth = rest.NewClient(&http.Client{Transport: &closeTrackingTransport{closed: &closed}})
	f.unAuth.SetErrorHandler(errorHandler)

	o := &Object{fs: f, id: "abc123", bytes: 999, itemNumber: 1}
	assert.Equal(t, int64(12345), o.Size())
	assert.True(t, closed, "the HEAD response body must be closed after a successful call, or transport connections leak")
}

func TestSizeConcurrentCallsShareVerification(t *testing.T) {
	var srv *httptest.Server
	var headMu sync.Mutex
	headCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("GET /media/abc123/download", func(w http.ResponseWriter, r *http.Request) {
		dl := makeDownloadResponse(nil, []testFile{{url: srv.URL + "/original.mp4", label: "source"}})
		writeJSON(t, w, dl)
	})
	mux.HandleFunc("HEAD /original.mp4", func(w http.ResponseWriter, r *http.Request) {
		headMu.Lock()
		headCalls++
		headMu.Unlock()
		w.Header().Set("Content-Length", "12345")
		w.WriteHeader(http.StatusOK)
	})
	f, s := newTestAPIFs(mux)
	srv = s
	defer srv.Close()
	f.opt.VerifySize = verifySizeAlways

	o := &Object{fs: f, id: "abc123", bytes: 999, itemNumber: 1}
	results := make(chan int64, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- o.Size()
		}()
	}
	wg.Wait()
	close(results)
	for size := range results {
		assert.Equal(t, int64(12345), size)
	}
	headMu.Lock()
	assert.Equal(t, 1, headCalls)
	headMu.Unlock()
}

func TestSizeVerifyOffNeverChecks(t *testing.T) {
	// f.srv/unAuth are left nil: verify_size "off" must return without
	// ever reaching the network, for a known size.
	o := &Object{fs: &Fs{opt: Options{VerifySize: verifySizeOff}}, bytes: 999}
	assert.Equal(t, int64(999), o.Size())
}

func TestOpenDownloadsContentAndFixesSize(t *testing.T) {
	var srv *httptest.Server
	want := []byte("the actual file contents")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /media/abc123/download", func(w http.ResponseWriter, r *http.Request) {
		dl := makeDownloadResponse(nil, []testFile{{url: srv.URL + "/original.mp4", label: "source"}})
		writeJSON(t, w, dl)
	})
	mux.HandleFunc("GET /original.mp4", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(want)))
		_, err := w.Write(want)
		require.NoError(t, err)
	})
	f, s := newTestAPIFs(mux)
	srv = s
	defer srv.Close()

	o := &Object{fs: f, id: "abc123", itemNumber: 1, bytes: -1, modTime: startTime}
	rc, err := o.Open(context.Background())
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	assert.Equal(t, want, got)
	assert.Equal(t, int64(len(want)), o.bytes, "Open must resolve an unknown size from the response it already has")
	assert.True(t, o.sizeChecked)
}

// rawPhotoLabels are the available_labels of a photo shot with RAW, as
// returned live by /media/search
var rawPhotoLabels = []string{"source", "raw_photo", "mediainfo"}

func TestListDirListsRawFilesNextToTheirPhotos(t *testing.T) {
	const single, series, plain = "6a306a51ececc5d9c2b55749", "6abac128b02933d265c1f3d8", "6a1af2c79e010fb1373a5700"
	size := int64(100)
	f := newTestMediaFs([]api.Medium{
		{ID: single, Filename: "GP012013.JPG", FileExtension: "jpg", Type: "Photo", FileSize: &size, ItemCount: 1, CapturedAt: startTime, AvailableLabels: rawPhotoLabels},
		{ID: series, Filename: "GPAA2158.JPG", FileExtension: "jpg", Type: "TimeLapse", FileSize: &size, ItemCount: 2, CapturedAt: startTime, AvailableLabels: append([]string{"zip"}, rawPhotoLabels...)},
		{ID: plain, Filename: "GP012002.JPG", FileExtension: "jpg", Type: "Photo", FileSize: &size, ItemCount: 1, CapturedAt: startTime, AvailableLabels: []string{"source"}},
	})
	entries, err := f.listDir(context.Background(), "media/all/", mediaFilter{})
	require.NoError(t, err)
	got := map[string]*Object{}
	for _, e := range entries {
		got[e.Remote()] = e.(*Object)
	}
	var names []string
	for name := range got {
		names = append(names, name)
	}
	assert.ElementsMatch(t, []string{
		"media/all/GP012013 {" + single + "}.JPG",
		"media/all/GP012013 {" + single + "}.GPR",
		"media/all/GPAA2158-1 {" + series + "}.JPG",
		"media/all/GPAA2158-1 {" + series + "}.GPR",
		"media/all/GPAA2158-2 {" + series + "}.JPG",
		"media/all/GPAA2158-2 {" + series + "}.GPR",
		"media/all/GP012002 {" + plain + "}.JPG",
	}, names)

	raw := got["media/all/GPAA2158-2 {"+series+"}.GPR"]
	assert.True(t, raw.raw)
	assert.Equal(t, 2, raw.itemNumber)
	assert.Equal(t, int64(-1), raw.bytes, "file_size doesn't cover RAW files")
	assert.Equal(t, rawMimeType, raw.mimeType)
	photo := got["media/all/GP012013 {"+single+"}.JPG"]
	assert.False(t, photo.raw)
	assert.Equal(t, int64(100), photo.bytes)
}

func TestListDirPhotoFormat(t *testing.T) {
	const pair, jpegOnly, rawOnly = "6a306a51ececc5d9c2b55749", "6a1af2c79e010fb1373a5700", "6a1af2c79e010fb1373a5711"
	size := int64(100)
	items := []api.Medium{
		{ID: pair, Filename: "GP012013.JPG", FileExtension: "jpg", Type: "Photo", FileSize: &size, ItemCount: 1, CapturedAt: startTime, AvailableLabels: rawPhotoLabels},
		{ID: jpegOnly, Filename: "GP012002.JPG", FileExtension: "jpg", Type: "Photo", FileSize: &size, ItemCount: 1, CapturedAt: startTime, AvailableLabels: []string{"source"}},
		// e.g. a RAW file uploaded on its own - it is the photo itself
		{ID: rawOnly, Filename: "GP019999.GPR", FileExtension: "gpr", Type: "Photo", FileSize: &size, ItemCount: 1, CapturedAt: startTime, AvailableLabels: []string{"source"}},
	}
	for _, tc := range []struct {
		format string
		want   []string
	}{
		{photoFormatBoth, []string{"GP012013 {" + pair + "}.JPG", "GP012013 {" + pair + "}.GPR", "GP012002 {" + jpegOnly + "}.JPG", "GP019999 {" + rawOnly + "}.GPR"}},
		{photoFormatJPEG, []string{"GP012013 {" + pair + "}.JPG", "GP012002 {" + jpegOnly + "}.JPG", "GP019999 {" + rawOnly + "}.GPR"}},
		{photoFormatRaw, []string{"GP012013 {" + pair + "}.GPR", "GP012002 {" + jpegOnly + "}.JPG", "GP019999 {" + rawOnly + "}.GPR"}},
	} {
		t.Run(tc.format, func(t *testing.T) {
			f := newTestMediaFs(items)
			f.opt.PhotoFormat = tc.format
			entries, err := f.listDir(context.Background(), "", mediaFilter{})
			require.NoError(t, err)
			var names []string
			for _, e := range entries {
				names = append(names, e.Remote())
			}
			assert.ElementsMatch(t, tc.want, names, "a photo with only one format must always be listed")
		})
	}
}

func TestCheckPhotoFormat(t *testing.T) {
	for _, ok := range []string{photoFormatBoth, photoFormatJPEG, photoFormatRaw} {
		assert.NoError(t, checkPhotoFormat(ok))
	}
	assert.Error(t, checkPhotoFormat("png"))
	_, err := NewFs(context.Background(), "photo-format-test", "", configmap.Simple{
		"access_token": "test-token", "verify_size": verifySizeReprocessed, "photo_format": "png",
	})
	assert.Error(t, err)
}

func TestReadMetaDataFastPathFollowsPhotoFormat(t *testing.T) {
	const id = "6a306a51ececc5d9c2b55749"
	for _, tc := range []struct {
		format, leaf string
		found        bool
	}{
		{photoFormatJPEG, "GP012013 {" + id + "}.JPG", true},
		{photoFormatJPEG, "GP012013 {" + id + "}.GPR", false},
		{photoFormatRaw, "GP012013 {" + id + "}.JPG", false},
		{photoFormatRaw, "GP012013 {" + id + "}.GPR", true},
	} {
		t.Run(tc.format+" "+tc.leaf, func(t *testing.T) {
			f, srv, _ := newTestRawFs(t, id, false)
			defer srv.Close()
			f.opt.PhotoFormat = tc.format
			f.media = cachedList([]api.Medium{})
			_, err := f.NewObject(context.Background(), "media/all/"+tc.leaf)
			if tc.found {
				assert.NoError(t, err)
			} else {
				assert.Equal(t, fs.ErrorObjectNotFound, err)
			}
		})
	}
}

func TestRawLeaf(t *testing.T) {
	assert.Equal(t, "GP012013.GPR", rawLeaf("GP012013.JPG"))
	assert.Equal(t, "GPAA2158-1 {abc}.GPR", rawLeaf("GPAA2158-1 {abc}.JPG"))
	assert.Equal(t, "holiday.gpr", rawLeaf("holiday.jpg"))
}

func TestSelectRaw(t *testing.T) {
	sidecar := func(url string, item int) api.SidecarFile {
		return api.SidecarFile{URL: url, Head: url + "?head", Label: "raw_photo", Type: "gpr", ItemNumber: item}
	}

	t.Run("a single photo's RAW has no item_number", func(t *testing.T) {
		dl := &api.DownloadResponse{}
		dl.Embedded.SidecarFiles = []api.SidecarFile{{URL: "https://cdn/1.json", Label: "mediainfo"}, sidecar("https://cdn/1.gpr", 0)}
		u, head, err := selectRaw(dl, 1)
		require.NoError(t, err)
		assert.Equal(t, "https://cdn/1.gpr", u)
		assert.Equal(t, "https://cdn/1.gpr?head", head)
	})

	t.Run("a photo series has one RAW per item_number", func(t *testing.T) {
		dl := &api.DownloadResponse{}
		dl.Embedded.SidecarFiles = []api.SidecarFile{{URL: "https://cdn/1.zip", Label: "zip"}, sidecar("https://cdn/1.gpr", 1), sidecar("https://cdn/2.gpr", 2)}
		u, _, err := selectRaw(dl, 2)
		require.NoError(t, err)
		assert.Equal(t, "https://cdn/2.gpr", u)
	})

	t.Run("no RAW is an error", func(t *testing.T) {
		_, _, err := selectRaw(&api.DownloadResponse{}, 1)
		assert.Error(t, err)
	})
}

// newTestRawFs serves medium id as a single photo with a RAW file, and
// records DELETE /media requests. gone makes GET /media/{id} 404.
func newTestRawFs(t *testing.T, id string, gone bool) (f *Fs, srv *httptest.Server, deletes *int) {
	deletes = new(int)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /media/"+id, func(w http.ResponseWriter, r *http.Request) {
		if gone {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		writeJSON(t, w, api.Medium{ID: id, Filename: "GP012013.JPG", FileExtension: "jpg", Type: "Photo", ItemCount: 1, AvailableLabels: rawPhotoLabels})
	})
	mux.HandleFunc("GET /media/"+id+"/download", func(w http.ResponseWriter, r *http.Request) {
		dl := makeDownloadResponse([]testFile{{url: srv.URL + "/1.jpg", itemNumber: 1}}, []testFile{{url: srv.URL + "/1.jpg", label: "source"}})
		dl.Embedded.SidecarFiles = []api.SidecarFile{{URL: srv.URL + "/1.gpr", Head: srv.URL + "/1.gpr", Label: "raw_photo"}}
		writeJSON(t, w, dl)
	})
	for name, body := range map[string]string{"/1.jpg": "jpeg bytes", "/1.gpr": "raw bytes, rather more of them"} {
		mux.HandleFunc(name, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			if r.Method != http.MethodHead {
				_, err := w.Write([]byte(body))
				require.NoError(t, err)
			}
		})
	}
	mux.HandleFunc("DELETE /media", func(w http.ResponseWriter, r *http.Request) {
		*deletes++
		writeJSON(t, w, api.DeleteResponse{})
	})
	f, srv = newTestAPIFs(mux)
	f.opt.AlwaysAddID = true
	return f, srv, deletes
}

func TestRawObjects(t *testing.T) {
	ctx := context.Background()
	const id = "6a306a51ececc5d9c2b55749"
	rawRemote := "media/all/GP012013 {" + id + "}.GPR"

	t.Run("a RAW file resolves by path and downloads the RAW", func(t *testing.T) {
		f, srv, _ := newTestRawFs(t, id, false)
		defer srv.Close()
		o, err := f.NewObject(ctx, rawRemote)
		require.NoError(t, err)
		obj := o.(*Object)
		assert.True(t, obj.raw)
		assert.Equal(t, rawMimeType, obj.MimeType(ctx))

		rc, err := o.Open(ctx)
		require.NoError(t, err)
		got, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		assert.Equal(t, "raw bytes, rather more of them", string(got))
		assert.Equal(t, int64(len(got)), o.Size())
	})

	t.Run("read_size reads the RAW's own size", func(t *testing.T) {
		f, srv, _ := newTestRawFs(t, id, false)
		defer srv.Close()
		f.opt.ReadSize = true
		o := &Object{fs: f, remote: rawRemote, raw: true}
		o.setMetaData(&api.Medium{ID: id, Filename: "GP012013.JPG", ItemCount: 1, CapturedAt: startTime, AvailableLabels: rawPhotoLabels}, 1)
		assert.Equal(t, int64(len("raw bytes, rather more of them")), o.Size())
	})

	t.Run("deleting a RAW on its own is refused, since it would delete the photo too", func(t *testing.T) {
		f, srv, deletes := newTestRawFs(t, id, false)
		defer srv.Close()
		o := &Object{fs: f, remote: rawRemote, id: id, raw: true, itemCount: 1, modTime: startTime}
		assert.Error(t, o.Remove(ctx))
		assert.Equal(t, 0, *deletes)
	})

	t.Run("deleting a RAW whose photo is already deleted succeeds", func(t *testing.T) {
		f, srv, deletes := newTestRawFs(t, id, true)
		defer srv.Close()
		o := &Object{fs: f, remote: rawRemote, id: id, raw: true, itemCount: 1, modTime: startTime}
		require.NoError(t, o.Remove(ctx))
		assert.Equal(t, 0, *deletes)
	})

	t.Run("renaming a RAW on its own is refused", func(t *testing.T) {
		f := &Fs{}
		src := &Object{fs: f, id: id, raw: true, itemCount: 1, remote: "media/all/GP012013 {" + id + "}.GPR", modTime: startTime}
		_, err := f.Move(ctx, src, "media/all/renamed {"+id+"}.GPR")
		assert.Error(t, err)
	})

	t.Run("moving a RAW to another date keeps its name", func(t *testing.T) {
		var gotUpdate api.MediumUpdate
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotUpdate))
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()
		src := &Object{fs: f, id: id, raw: true, itemCount: 1, remote: "media/by-day/2025/2025-03-14/GP012013 {" + id + "}.GPR", modTime: time.Date(2025, 3, 14, 9, 0, 0, 0, time.UTC)}
		dst, err := f.Move(ctx, src, "media/by-day/2026/2026-07-04/GP012013 {"+id+"}.GPR")
		require.NoError(t, err)
		assert.Nil(t, gotUpdate.Filename)
		require.NotNil(t, gotUpdate.CapturedAt)
		assert.True(t, dst.(*Object).raw)
	})
}

func TestReadMetaDataAlreadyResolvedIsANoOp(t *testing.T) {
	// f.fs is left nil: a resolved Object must return without touching it.
	o := &Object{remote: "media/all/x.mp4", modTime: startTime}
	require.NoError(t, o.readMetaData(context.Background()))
}

func TestReadMetaDataRejectsAnUnmatchedPath(t *testing.T) {
	// patterns.match is always called with isFile=true here, which by
	// construction only ever matches a file pattern (or nothing) - see
	// dirPatterns.match - so "media/all" (a directory-only pattern) simply
	// fails to match at all, the same as any other nonsense path.
	o := &Object{fs: &Fs{}, remote: "media/all"}
	err := o.readMetaData(context.Background())
	assert.Equal(t, fs.ErrorObjectNotFound, err)
}

func TestReadMetaDataIDFastPath(t *testing.T) {
	id := "68b22325df3cf752557ac6d7"

	t.Run("a name that reconstructs exactly is trusted without listing", func(t *testing.T) {
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/media/"+id, r.URL.Path)
			writeJSON(t, w, api.Medium{ID: id, Filename: "GX010294.MP4", ItemCount: 1})
		}))
		defer srv.Close()
		f.opt.AlwaysAddID = true

		o := &Object{fs: f, remote: "media/all/GX010294 {" + id + "}.MP4"}
		require.NoError(t, o.readMetaData(context.Background()))
		assert.Equal(t, id, o.id)
	})

	t.Run("a mismatched reconstruction falls through to the listing lookup instead of trusting the id", func(t *testing.T) {
		var getMediumCalls int
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			getMediumCalls++
			writeJSON(t, w, api.Medium{ID: id, Filename: "totally-different-name.mp4", ItemCount: 1})
		}))
		defer srv.Close()
		f.opt.AlwaysAddID = true
		f.media = cachedList([]api.Medium{}) // an empty, but cached (non-nil), library

		o := &Object{fs: f, remote: "media/all/some-renamed-file {" + id + "}.mp4"}
		err := o.readMetaData(context.Background())
		assert.Equal(t, fs.ErrorObjectNotFound, err, "a fabricated {id} suffix on an unrelated name must never resolve to that id's medium")
		assert.Equal(t, 1, getMediumCalls)
	})

	t.Run("only the trailing generated suffix is trusted, not an id-shaped substring earlier in the name", func(t *testing.T) {
		unrelatedID := "aaaaaaaaaaaaaaaaaaaaaaaa"
		var requestedIDs []string
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotID := strings.TrimPrefix(r.URL.Path, "/media/")
			requestedIDs = append(requestedIDs, gotID)
			if gotID != id {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			writeJSON(t, w, api.Medium{ID: id, Filename: "note {" + unrelatedID + "} clip.mp4", ItemCount: 1})
		}))
		defer srv.Close()
		f.opt.AlwaysAddID = true

		o := &Object{fs: f, remote: "media/all/note {" + unrelatedID + "} clip {" + id + "}.mp4"}
		require.NoError(t, o.readMetaData(context.Background()), "the real trailing suffix must still resolve even though an unrelated id-shaped substring appears earlier")
		assert.Equal(t, id, o.id)
		assert.NotContains(t, requestedIDs, unrelatedID, "the unrelated id-shaped substring earlier in the name must never be looked up")
	})

	t.Run("a 404 from the fast path means not found, not an error", func(t *testing.T) {
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "not found", http.StatusNotFound)
		}))
		defer srv.Close()
		f.opt.AlwaysAddID = true
		f.media = cachedList([]api.Medium{})

		o := &Object{fs: f, remote: "media/all/deleted {" + id + "}.mp4"}
		err := o.readMetaData(context.Background())
		assert.Equal(t, fs.ErrorObjectNotFound, err, "a medium deleted (or trashed) since listing must read as not found")
	})

	t.Run("trashed_only always skips the fast path, since GET /media/{id} 404s for trashed items", func(t *testing.T) {
		var getMediumCalls int
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			getMediumCalls++
		}))
		defer srv.Close()
		f.opt.AlwaysAddID = true
		f.opt.TrashedOnly = true
		f.trash = cachedList([]api.Medium{})

		o := &Object{fs: f, remote: "media/all/x {" + id + "}.mp4"}
		err := o.readMetaData(context.Background())
		assert.Equal(t, fs.ErrorObjectNotFound, err)
		assert.Equal(t, 0, getMediumCalls)
	})
}

func TestModTimeAndSetModTime(t *testing.T) {
	t.Run("ModTime returns the already-resolved time without a network call", func(t *testing.T) {
		o := &Object{fs: &Fs{}, remote: "media/all/x.mp4", modTime: startTime}
		assert.True(t, startTime.Equal(o.ModTime(context.Background())))
	})

	t.Run("SetModTime updates captured_at and the Object's own cached modTime", func(t *testing.T) {
		var gotUpdate api.MediumUpdate
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotUpdate))
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		newTime := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
		o := &Object{fs: f, id: "abc123", modTime: startTime}
		require.NoError(t, o.SetModTime(context.Background(), newTime))
		assert.True(t, newTime.Equal(o.modTime))
		require.NotNil(t, gotUpdate.CapturedAt)
		assert.True(t, newTime.Equal(*gotUpdate.CapturedAt))
	})
}

func TestPutAndUpdateDriveTheFullUploadProtocol(t *testing.T) {
	content := []byte("put/update integration through the real multipart uploader")
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("POST /media", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]string{"id": "med1"})
	})
	mux.HandleFunc("POST /derivatives", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]string{"id": "der1"})
	})
	mux.HandleFunc("POST /user-uploads", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]string{"id": "up1"})
	})
	mux.HandleFunc("GET /user-uploads/der1", func(w http.ResponseWriter, r *http.Request) {
		resp := api.UserUploadsResponse{}
		resp.Embedded.Authorizations = []api.UploadAuthorization{{URL: srv.URL + "/chunk/1", Part: 1}}
		writeJSON(t, w, resp)
	})
	mux.HandleFunc("PUT /chunk/1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("PUT /user-uploads/der1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("PUT /derivatives/der1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("PUT /media/med1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	f, s := newTestUploadFlowFs(mux)
	srv = s
	defer srv.Close()

	src := mockobject.New("upload/GX010001.MP4").WithContent(content, mockobject.SeekModeRegular)
	dstObj, err := f.Put(context.Background(), bytes.NewReader(content), src)
	require.NoError(t, err)
	assert.Equal(t, "upload/GX010001.MP4", dstObj.Remote())

	gpObj, ok := dstObj.(*Object)
	require.True(t, ok)
	assert.Equal(t, "med1", gpObj.id)
	assert.Equal(t, int64(len(content)), gpObj.bytes)

	entries, err := f.List(context.Background(), "upload")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "upload/GX010001.MP4", entries[0].Remote())
}

// newTestReplaceFs builds an Fs whose mocked upload protocol always
// creates medium "new", recording every DELETE /media request's query.
func newTestReplaceFs(t *testing.T) (f *Fs, srv *httptest.Server, deletes *[]url.Values) {
	deletes = &[]url.Values{}
	var mu sync.Mutex
	mux := http.NewServeMux()
	mux.HandleFunc("POST /media", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]string{"id": "new"})
	})
	mux.HandleFunc("POST /derivatives", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]string{"id": "der1"})
	})
	mux.HandleFunc("POST /user-uploads", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]string{"id": "up1"})
	})
	mux.HandleFunc("GET /user-uploads/der1", func(w http.ResponseWriter, r *http.Request) {
		resp := api.UserUploadsResponse{}
		resp.Embedded.Authorizations = []api.UploadAuthorization{{URL: srv.URL + "/chunk/1", Part: 1}}
		writeJSON(t, w, resp)
	})
	noContent := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }
	mux.HandleFunc("PUT /chunk/1", noContent)
	mux.HandleFunc("PUT /user-uploads/der1", noContent)
	mux.HandleFunc("PUT /derivatives/der1", noContent)
	mux.HandleFunc("PUT /media/new", noContent)
	mux.HandleFunc("DELETE /media", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		*deletes = append(*deletes, r.URL.Query())
		mu.Unlock()
		writeJSON(t, w, api.DeleteResponse{})
	})
	f, srv = newTestUploadFlowFs(mux)
	f.opt.UseTrash = true
	return f, srv, deletes
}

func TestUpdateReplacesTheExistingUpload(t *testing.T) {
	ctx := context.Background()
	content := []byte("new content")
	const remote = "upload/GX010001.MP4"
	src := mockobject.New(remote).WithContent(content, mockobject.SeekModeRegular)

	t.Run("Update deletes the medium it replaces and keeps one entry", func(t *testing.T) {
		f, srv, deletes := newTestReplaceFs(t)
		defer srv.Close()
		old := &Object{fs: f, remote: remote, id: "old", itemCount: 1, bytes: 3}
		f.uploaded.AddEntry(old)

		require.NoError(t, old.Update(ctx, bytes.NewReader(content), src))
		assert.Equal(t, "new", old.id)
		require.Len(t, *deletes, 1)
		assert.Equal(t, "old", (*deletes)[0].Get("ids"))
		assert.Empty(t, (*deletes)[0].Get("permanent"), "use_trash sends the replaced medium to the trash")

		entries, err := f.List(ctx, "upload")
		require.NoError(t, err)
		require.Len(t, entries, 1, "the replaced entry must not be listed alongside the new one")
		assert.Equal(t, "new", entries[0].(*Object).id)
	})

	t.Run("an object not in the upload tree is replaced too", func(t *testing.T) {
		f, srv, deletes := newTestReplaceFs(t)
		defer srv.Close()
		old := &Object{fs: f, remote: remote, id: "old", itemCount: 1}

		require.NoError(t, old.Update(ctx, bytes.NewReader(content), src))
		require.Len(t, *deletes, 1)
		assert.Equal(t, "old", (*deletes)[0].Get("ids"))
	})

	t.Run("without use_trash the replaced medium is deleted permanently", func(t *testing.T) {
		defer func(d time.Duration) { deletePermanentDelay = d }(deletePermanentDelay)
		deletePermanentDelay = time.Millisecond
		f, srv, deletes := newTestReplaceFs(t)
		defer srv.Close()
		f.opt.UseTrash = false
		old := &Object{fs: f, remote: remote, id: "old", itemCount: 1}

		require.NoError(t, old.Update(ctx, bytes.NewReader(content), src))
		require.Len(t, *deletes, 2)
		assert.Equal(t, "true", (*deletes)[1].Get("permanent"))
	})

	t.Run("a multi-thread copy over an existing upload replaces it", func(t *testing.T) {
		f, srv, deletes := newTestReplaceFs(t)
		defer srv.Close()
		f.uploaded.AddEntry(&Object{fs: f, remote: remote, id: "old", itemCount: 1})

		_, writer, err := f.OpenChunkWriter(ctx, remote, src)
		require.NoError(t, err)
		_, err = writer.WriteChunk(ctx, 0, bytes.NewReader(content))
		require.NoError(t, err)
		require.NoError(t, writer.Close(ctx))

		require.Len(t, *deletes, 1)
		assert.Equal(t, "old", (*deletes)[0].Get("ids"))
		entries, err := f.List(ctx, "upload")
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, "new", entries[0].(*Object).id)
	})

	t.Run("a new upload deletes nothing", func(t *testing.T) {
		f, srv, deletes := newTestReplaceFs(t)
		defer srv.Close()
		_, err := f.Put(ctx, bytes.NewReader(content), src)
		require.NoError(t, err)
		assert.Empty(t, *deletes)
	})
}

func TestMoveSucceeds(t *testing.T) {
	t.Run("a same-bucket rename only renames, captured_at untouched", func(t *testing.T) {
		var gotUpdate api.MediumUpdate
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/media/abc123", r.URL.Path)
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotUpdate))
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		modTime := time.Date(2025, 3, 14, 9, 30, 15, 0, time.UTC)
		src := &Object{fs: f, id: "abc123", itemCount: 1, remote: "media/all/old.mp4", modTime: modTime}
		dst, err := f.Move(context.Background(), src, "media/all/new.mp4")
		require.NoError(t, err)
		require.NotNil(t, gotUpdate.Filename)
		assert.Equal(t, "new.mp4", *gotUpdate.Filename)
		assert.Nil(t, gotUpdate.CapturedAt, "media/all implies no date change")

		dstObj, ok := dst.(*Object)
		require.True(t, ok)
		assert.Equal(t, "media/all/new.mp4", dstObj.Remote())
		assert.Equal(t, f, dstObj.Fs())
		assert.True(t, modTime.Equal(dstObj.modTime))
	})

	t.Run("a move to a different by-day bucket also repins captured_at", func(t *testing.T) {
		var gotUpdate api.MediumUpdate
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotUpdate))
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		modTime := time.Date(2025, 3, 14, 9, 30, 15, 0, time.UTC)
		src := &Object{fs: f, id: "abc123", itemCount: 1, remote: "media/by-day/2025/2025-03-14/old.mp4", modTime: modTime}
		dst, err := f.Move(context.Background(), src, "media/by-day/2026/2026-07-04/new.mp4")
		require.NoError(t, err)
		require.NotNil(t, gotUpdate.CapturedAt)
		assert.Equal(t, time.Date(2026, 7, 4, 9, 30, 15, 0, time.UTC), *gotUpdate.CapturedAt)

		dstObj, ok := dst.(*Object)
		require.True(t, ok)
		assert.True(t, gotUpdate.CapturedAt.Equal(dstObj.modTime))
	})

	id := "68b22325df3cf752557ac6d7"
	modTime := time.Date(2025, 3, 14, 9, 30, 15, 0, time.UTC)

	t.Run("a date-only move leaves filename and content_title alone", func(t *testing.T) {
		var gotUpdate api.MediumUpdate
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotUpdate))
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		src := &Object{fs: f, id: id, itemCount: 1, remote: "media/by-day/2025/2025-03-14/clip {" + id + "}.mp4", modTime: modTime}
		_, err := f.Move(context.Background(), src, "media/by-day/2026/2026-07-04/clip {"+id+"}.mp4")
		require.NoError(t, err)
		assert.Nil(t, gotUpdate.Filename, "an unchanged name must not be resent")
		assert.Nil(t, gotUpdate.ContentTitle, "an unchanged name must not overwrite a custom title")
		require.NotNil(t, gotUpdate.CapturedAt)
	})

	t.Run("an unnamed medium moved to another day keeps its empty filename", func(t *testing.T) {
		var gotUpdate api.MediumUpdate
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotUpdate))
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		src := &Object{fs: f, id: id, itemCount: 1, remote: "media/by-day/2025/2025-03-14/{" + id + "}.mp4", modTime: modTime}
		_, err := f.Move(context.Background(), src, "media/by-day/2026/2026-07-04/{"+id+"}.mp4")
		require.NoError(t, err)
		assert.Nil(t, gotUpdate.Filename, "an unnamed medium must not be renamed to \".mp4\"")
		assert.Nil(t, gotUpdate.ContentTitle)
		require.NotNil(t, gotUpdate.CapturedAt)
	})

	t.Run("renaming to an unnamed leaf is refused", func(t *testing.T) {
		var calls int
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		src := &Object{fs: f, id: id, itemCount: 1, remote: "media/all/old.mp4", modTime: modTime}
		_, err := f.Move(context.Background(), src, "media/all/{"+id+"}.mp4")
		assert.Error(t, err)
		assert.Equal(t, 0, calls)
	})

	t.Run("a move that changes nothing makes no request", func(t *testing.T) {
		var calls int
		f, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		src := &Object{fs: f, id: id, itemCount: 1, remote: "media/all/clip {" + id + "}.mp4", modTime: modTime}
		dst, err := f.Move(context.Background(), src, "media/all/clip {"+id+"}.mp4")
		require.NoError(t, err)
		assert.Equal(t, "media/all/clip {"+id+"}.mp4", dst.Remote())
		assert.Equal(t, 0, calls)
	})

	t.Run("the destination Fs's cache is invalidated as well as the source's", func(t *testing.T) {
		srcFs, srv := newTestAPIFs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()
		dstFs := &Fs{root: "media/by-day/2026/2026-07-04"}
		dstFs.media = cachedList([]api.Medium{})

		src := &Object{fs: srcFs, id: id, itemCount: 1, remote: "old.mp4", modTime: modTime}
		_, err := dstFs.Move(context.Background(), src, "new.mp4")
		require.NoError(t, err)
		assert.Nil(t, dstFs.media.items)
	})

	t.Run("a move into upload/ is refused - there's no medium to rename until an upload completes", func(t *testing.T) {
		f := &Fs{}
		src := &Object{fs: f, itemCount: 1, remote: "media/all/x.mp4"}
		_, err := f.Move(context.Background(), src, "upload/x.mp4")
		assert.Equal(t, fs.ErrorCantMove, err)
	})
}

func TestShouldRetry(t *testing.T) {
	t.Run("a cancelled context is never retried", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		retry, err := shouldRetry(ctx, nil, errors.New("boom"))
		assert.False(t, retry)
		assert.Error(t, err)
	})

	t.Run("a cancelled context supplies its own error when none was given", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		retry, err := shouldRetry(ctx, nil, nil)
		assert.False(t, retry)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("a retryable HTTP status is retried", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusServiceUnavailable}
		retry, _ := shouldRetry(context.Background(), resp, errors.New("boom"))
		assert.True(t, retry)
	})

	t.Run("a non-retryable status with a plain error is not retried", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusNotFound}
		retry, _ := shouldRetry(context.Background(), resp, errors.New("boom"))
		assert.False(t, retry)
	})

	t.Run("no error at all is not retried", func(t *testing.T) {
		retry, err := shouldRetry(context.Background(), &http.Response{StatusCode: http.StatusOK}, nil)
		assert.False(t, retry)
		assert.NoError(t, err)
	})
}

func TestErrorHandler(t *testing.T) {
	t.Run("a GoPro/OAuth-shaped JSON error body decodes into api.Error's fields", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 400,
			Body:       io.NopCloser(strings.NewReader(`{"error":"invalid_grant","error_description":"bad token"}`)),
		}
		err := errorHandler(resp)
		apiErr, ok := err.(*api.Error)
		require.True(t, ok)
		assert.Equal(t, "invalid_grant", apiErr.ErrorCode)
		assert.Equal(t, "bad token", apiErr.ErrorDescription)
		assert.Equal(t, 400, apiErr.Status)
	})

	t.Run("a non-JSON body is preserved verbatim rather than dropped", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 500,
			Body:       io.NopCloser(strings.NewReader("internal server error")),
		}
		err := errorHandler(resp)
		apiErr, ok := err.(*api.Error)
		require.True(t, ok)
		assert.Equal(t, "internal server error", apiErr.Body)
		assert.Equal(t, 500, apiErr.Status)
	})
}

func TestNewObjectWithInfoUsesProvidedInfoWithoutAnyNetworkCall(t *testing.T) {
	// f has no srv: readMetaData's own network fallback would panic if this
	// ever reached it, proving the provided info is what's actually used.
	f := &Fs{}
	size := int64(42)
	item := &api.Medium{ID: "abc123", Filename: "x.mp4", FileSize: &size, ItemCount: 1, CapturedAt: startTime}
	o, err := f.newObjectWithInfo(context.Background(), "media/all/x.mp4", item)
	require.NoError(t, err)
	gpObj, ok := o.(*Object)
	require.True(t, ok)
	assert.Equal(t, "abc123", gpObj.id)
	assert.Equal(t, int64(42), gpObj.bytes)
}

func TestOpenChunkWriterPropagatesEachProtocolStepsFailure(t *testing.T) {
	old := deletePermanentDelay
	deletePermanentDelay = time.Millisecond // rollback uses a permanent delete
	defer func() { deletePermanentDelay = old }()

	src := mockobject.New("upload/x.mp4").WithContent([]byte("hello"), mockobject.SeekModeRegular)

	// A ServeMux request for a path with no registered handler gets Go's
	// default 404, which isn't in retryErrorCodes - an instant, cheap way
	// to simulate "this step's endpoint failed" without waiting through the
	// pacer's retry backoff.
	newFsWith := func(handlers map[string]http.HandlerFunc) (*Fs, *httptest.Server) {
		mux := http.NewServeMux()
		for pattern, h := range handlers {
			mux.HandleFunc(pattern, h)
		}
		return newTestUploadFlowFs(mux)
	}
	idJSON := func(id string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { writeJSON(t, w, map[string]string{"id": id}) }
	}

	t.Run("createMedium failing stops immediately", func(t *testing.T) {
		f, srv := newFsWith(nil)
		defer srv.Close()
		_, _, err := f.OpenChunkWriter(context.Background(), "upload/x.mp4", src)
		assert.Error(t, err)
	})

	t.Run("createMedium returning an empty id is an error", func(t *testing.T) {
		f, srv := newFsWith(map[string]http.HandlerFunc{"POST /media": idJSON("")})
		defer srv.Close()
		_, _, err := f.OpenChunkWriter(context.Background(), "upload/x.mp4", src)
		assert.Error(t, err)
	})

	// deleteHandler records every DELETE /media call's "ids" query value,
	// for asserting that a later setup-step failure rolls back the medium
	// createMedium already created. deleteMedium's rollback always
	// finalises permanently (see TestAbortDeletesTheMedium, the same
	// shape this reuses): one plain delete followed, after
	// deletePermanentDelay, by one with permanent=true.
	deleteHandler := func(calls *[]url.Values) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			*calls = append(*calls, r.URL.Query())
			writeJSON(t, w, api.DeleteResponse{})
		}
	}

	t.Run("createDerivative failing stops after createMedium succeeds, and rolls the medium back", func(t *testing.T) {
		var deleteCalls []url.Values
		f, srv := newFsWith(map[string]http.HandlerFunc{
			"POST /media":   idJSON("med1"),
			"DELETE /media": deleteHandler(&deleteCalls),
		})
		defer srv.Close()
		_, _, err := f.OpenChunkWriter(context.Background(), "upload/x.mp4", src)
		assert.Error(t, err)
		require.Len(t, deleteCalls, 2, "the orphaned medium must be rolled back, permanently")
		assert.Equal(t, "med1", deleteCalls[0].Get("ids"))
		assert.Equal(t, "med1", deleteCalls[1].Get("ids"))
		assert.Equal(t, "true", deleteCalls[1].Get("permanent"))
	})

	t.Run("createUpload failing stops after the first two steps succeed, and rolls the medium back", func(t *testing.T) {
		var deleteCalls []url.Values
		f, srv := newFsWith(map[string]http.HandlerFunc{
			"POST /media":       idJSON("med1"),
			"POST /derivatives": idJSON("der1"),
			"DELETE /media":     deleteHandler(&deleteCalls),
		})
		defer srv.Close()
		_, _, err := f.OpenChunkWriter(context.Background(), "upload/x.mp4", src)
		assert.Error(t, err)
		require.Len(t, deleteCalls, 2, "the orphaned medium must be rolled back, permanently")
		assert.Equal(t, "med1", deleteCalls[0].Get("ids"))
	})

	t.Run("getUploadParts failing stops after the first three steps succeed, and rolls the medium back", func(t *testing.T) {
		var deleteCalls []url.Values
		f, srv := newFsWith(map[string]http.HandlerFunc{
			"POST /media":        idJSON("med1"),
			"POST /derivatives":  idJSON("der1"),
			"POST /user-uploads": idJSON("up1"),
			"DELETE /media":      deleteHandler(&deleteCalls),
		})
		defer srv.Close()
		_, _, err := f.OpenChunkWriter(context.Background(), "upload/x.mp4", src)
		assert.Error(t, err)
		require.Len(t, deleteCalls, 2, "the orphaned medium must be rolled back, permanently")
		assert.Equal(t, "med1", deleteCalls[0].Get("ids"))
	})

	t.Run("getUploadParts returning no authorizations at all is an error, and rolls the medium back", func(t *testing.T) {
		var deleteCalls []url.Values
		f, srv := newFsWith(map[string]http.HandlerFunc{
			"POST /media":            idJSON("med1"),
			"POST /derivatives":      idJSON("der1"),
			"POST /user-uploads":     idJSON("up1"),
			"GET /user-uploads/der1": func(w http.ResponseWriter, r *http.Request) { writeJSON(t, w, api.UserUploadsResponse{}) },
			"DELETE /media":          deleteHandler(&deleteCalls),
		})
		defer srv.Close()
		_, _, err := f.OpenChunkWriter(context.Background(), "upload/x.mp4", src)
		assert.Error(t, err)
		require.Len(t, deleteCalls, 2, "the orphaned medium must be rolled back, permanently")
		assert.Equal(t, "med1", deleteCalls[0].Get("ids"))
	})
}

func TestCloseFinalizationFailurePreventsRegistration(t *testing.T) {
	// Close's three finalising calls run in order; a failure at any of
	// them must stop before the upload is registered under upload/, since
	// it hasn't actually finished.
	newFsWith := func(handlers map[string]http.HandlerFunc) (*Fs, *httptest.Server) {
		mux := http.NewServeMux()
		for pattern, h := range handlers {
			mux.HandleFunc(pattern, h)
		}
		return newTestUploadFlowFs(mux)
	}
	noContent := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }

	t.Run("completeUpload failing stops immediately", func(t *testing.T) {
		f, srv := newFsWith(nil)
		defer srv.Close()
		w := &gpChunkWriter{f: f, derivativeID: "der1", uploadID: "up1", mediumID: "med1", remote: "upload/x.mp4"}
		assert.Error(t, w.Close(context.Background()))
		entries, err := f.listUploads(context.Background(), "")
		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("markDerivativeAvailable failing stops after completeUpload succeeds", func(t *testing.T) {
		f, srv := newFsWith(map[string]http.HandlerFunc{"PUT /user-uploads/der1": noContent})
		defer srv.Close()
		w := &gpChunkWriter{f: f, derivativeID: "der1", uploadID: "up1", mediumID: "med1", remote: "upload/x.mp4"}
		assert.Error(t, w.Close(context.Background()))
	})

	t.Run("markMediumAvailable failing stops after the first two finalising calls succeed", func(t *testing.T) {
		f, srv := newFsWith(map[string]http.HandlerFunc{
			"PUT /user-uploads/der1": noContent,
			"PUT /derivatives/der1":  noContent,
		})
		defer srv.Close()
		w := &gpChunkWriter{f: f, derivativeID: "der1", uploadID: "up1", mediumID: "med1", remote: "upload/x.mp4"}
		assert.Error(t, w.Close(context.Background()))
	})
}
