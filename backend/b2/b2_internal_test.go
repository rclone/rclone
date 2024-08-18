package b2

import (
	"context"
	"crypto/sha1"
	"fmt"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/b2/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/bucket"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test b2 string encoding
// https://www.backblaze.com/docs/cloud-storage-native-api-string-encoding

var encodeTest = []struct {
	fullyEncoded     string
	minimallyEncoded string
	plainText        string
}{
	{fullyEncoded: "%20", minimallyEncoded: "+", plainText: " "},
	{fullyEncoded: "%21", minimallyEncoded: "!", plainText: "!"},
	{fullyEncoded: "%22", minimallyEncoded: "%22", plainText: "\""},
	{fullyEncoded: "%23", minimallyEncoded: "%23", plainText: "#"},
	{fullyEncoded: "%24", minimallyEncoded: "$", plainText: "$"},
	{fullyEncoded: "%25", minimallyEncoded: "%25", plainText: "%"},
	{fullyEncoded: "%26", minimallyEncoded: "%26", plainText: "&"},
	{fullyEncoded: "%27", minimallyEncoded: "'", plainText: "'"},
	{fullyEncoded: "%28", minimallyEncoded: "(", plainText: "("},
	{fullyEncoded: "%29", minimallyEncoded: ")", plainText: ")"},
	{fullyEncoded: "%2A", minimallyEncoded: "*", plainText: "*"},
	{fullyEncoded: "%2B", minimallyEncoded: "%2B", plainText: "+"},
	{fullyEncoded: "%2C", minimallyEncoded: "%2C", plainText: ","},
	{fullyEncoded: "%2D", minimallyEncoded: "-", plainText: "-"},
	{fullyEncoded: "%2E", minimallyEncoded: ".", plainText: "."},
	{fullyEncoded: "%2F", minimallyEncoded: "/", plainText: "/"},
	{fullyEncoded: "%30", minimallyEncoded: "0", plainText: "0"},
	{fullyEncoded: "%31", minimallyEncoded: "1", plainText: "1"},
	{fullyEncoded: "%32", minimallyEncoded: "2", plainText: "2"},
	{fullyEncoded: "%33", minimallyEncoded: "3", plainText: "3"},
	{fullyEncoded: "%34", minimallyEncoded: "4", plainText: "4"},
	{fullyEncoded: "%35", minimallyEncoded: "5", plainText: "5"},
	{fullyEncoded: "%36", minimallyEncoded: "6", plainText: "6"},
	{fullyEncoded: "%37", minimallyEncoded: "7", plainText: "7"},
	{fullyEncoded: "%38", minimallyEncoded: "8", plainText: "8"},
	{fullyEncoded: "%39", minimallyEncoded: "9", plainText: "9"},
	{fullyEncoded: "%3A", minimallyEncoded: ":", plainText: ":"},
	{fullyEncoded: "%3B", minimallyEncoded: ";", plainText: ";"},
	{fullyEncoded: "%3C", minimallyEncoded: "%3C", plainText: "<"},
	{fullyEncoded: "%3D", minimallyEncoded: "=", plainText: "="},
	{fullyEncoded: "%3E", minimallyEncoded: "%3E", plainText: ">"},
	{fullyEncoded: "%3F", minimallyEncoded: "%3F", plainText: "?"},
	{fullyEncoded: "%40", minimallyEncoded: "@", plainText: "@"},
	{fullyEncoded: "%41", minimallyEncoded: "A", plainText: "A"},
	{fullyEncoded: "%42", minimallyEncoded: "B", plainText: "B"},
	{fullyEncoded: "%43", minimallyEncoded: "C", plainText: "C"},
	{fullyEncoded: "%44", minimallyEncoded: "D", plainText: "D"},
	{fullyEncoded: "%45", minimallyEncoded: "E", plainText: "E"},
	{fullyEncoded: "%46", minimallyEncoded: "F", plainText: "F"},
	{fullyEncoded: "%47", minimallyEncoded: "G", plainText: "G"},
	{fullyEncoded: "%48", minimallyEncoded: "H", plainText: "H"},
	{fullyEncoded: "%49", minimallyEncoded: "I", plainText: "I"},
	{fullyEncoded: "%4A", minimallyEncoded: "J", plainText: "J"},
	{fullyEncoded: "%4B", minimallyEncoded: "K", plainText: "K"},
	{fullyEncoded: "%4C", minimallyEncoded: "L", plainText: "L"},
	{fullyEncoded: "%4D", minimallyEncoded: "M", plainText: "M"},
	{fullyEncoded: "%4E", minimallyEncoded: "N", plainText: "N"},
	{fullyEncoded: "%4F", minimallyEncoded: "O", plainText: "O"},
	{fullyEncoded: "%50", minimallyEncoded: "P", plainText: "P"},
	{fullyEncoded: "%51", minimallyEncoded: "Q", plainText: "Q"},
	{fullyEncoded: "%52", minimallyEncoded: "R", plainText: "R"},
	{fullyEncoded: "%53", minimallyEncoded: "S", plainText: "S"},
	{fullyEncoded: "%54", minimallyEncoded: "T", plainText: "T"},
	{fullyEncoded: "%55", minimallyEncoded: "U", plainText: "U"},
	{fullyEncoded: "%56", minimallyEncoded: "V", plainText: "V"},
	{fullyEncoded: "%57", minimallyEncoded: "W", plainText: "W"},
	{fullyEncoded: "%58", minimallyEncoded: "X", plainText: "X"},
	{fullyEncoded: "%59", minimallyEncoded: "Y", plainText: "Y"},
	{fullyEncoded: "%5A", minimallyEncoded: "Z", plainText: "Z"},
	{fullyEncoded: "%5B", minimallyEncoded: "%5B", plainText: "["},
	{fullyEncoded: "%5C", minimallyEncoded: "%5C", plainText: "\\"},
	{fullyEncoded: "%5D", minimallyEncoded: "%5D", plainText: "]"},
	{fullyEncoded: "%5E", minimallyEncoded: "%5E", plainText: "^"},
	{fullyEncoded: "%5F", minimallyEncoded: "_", plainText: "_"},
	{fullyEncoded: "%60", minimallyEncoded: "%60", plainText: "`"},
	{fullyEncoded: "%61", minimallyEncoded: "a", plainText: "a"},
	{fullyEncoded: "%62", minimallyEncoded: "b", plainText: "b"},
	{fullyEncoded: "%63", minimallyEncoded: "c", plainText: "c"},
	{fullyEncoded: "%64", minimallyEncoded: "d", plainText: "d"},
	{fullyEncoded: "%65", minimallyEncoded: "e", plainText: "e"},
	{fullyEncoded: "%66", minimallyEncoded: "f", plainText: "f"},
	{fullyEncoded: "%67", minimallyEncoded: "g", plainText: "g"},
	{fullyEncoded: "%68", minimallyEncoded: "h", plainText: "h"},
	{fullyEncoded: "%69", minimallyEncoded: "i", plainText: "i"},
	{fullyEncoded: "%6A", minimallyEncoded: "j", plainText: "j"},
	{fullyEncoded: "%6B", minimallyEncoded: "k", plainText: "k"},
	{fullyEncoded: "%6C", minimallyEncoded: "l", plainText: "l"},
	{fullyEncoded: "%6D", minimallyEncoded: "m", plainText: "m"},
	{fullyEncoded: "%6E", minimallyEncoded: "n", plainText: "n"},
	{fullyEncoded: "%6F", minimallyEncoded: "o", plainText: "o"},
	{fullyEncoded: "%70", minimallyEncoded: "p", plainText: "p"},
	{fullyEncoded: "%71", minimallyEncoded: "q", plainText: "q"},
	{fullyEncoded: "%72", minimallyEncoded: "r", plainText: "r"},
	{fullyEncoded: "%73", minimallyEncoded: "s", plainText: "s"},
	{fullyEncoded: "%74", minimallyEncoded: "t", plainText: "t"},
	{fullyEncoded: "%75", minimallyEncoded: "u", plainText: "u"},
	{fullyEncoded: "%76", minimallyEncoded: "v", plainText: "v"},
	{fullyEncoded: "%77", minimallyEncoded: "w", plainText: "w"},
	{fullyEncoded: "%78", minimallyEncoded: "x", plainText: "x"},
	{fullyEncoded: "%79", minimallyEncoded: "y", plainText: "y"},
	{fullyEncoded: "%7A", minimallyEncoded: "z", plainText: "z"},
	{fullyEncoded: "%7B", minimallyEncoded: "%7B", plainText: "{"},
	{fullyEncoded: "%7C", minimallyEncoded: "%7C", plainText: "|"},
	{fullyEncoded: "%7D", minimallyEncoded: "%7D", plainText: "}"},
	{fullyEncoded: "%7E", minimallyEncoded: "~", plainText: "~"},
	{fullyEncoded: "%7F", minimallyEncoded: "%7F", plainText: "\u007f"},
	{fullyEncoded: "%E8%87%AA%E7%94%B1", minimallyEncoded: "%E8%87%AA%E7%94%B1", plainText: "自由"},
	{fullyEncoded: "%F0%90%90%80", minimallyEncoded: "%F0%90%90%80", plainText: "𐐀"},
}

func TestUrlEncode(t *testing.T) {
	for _, test := range encodeTest {
		got := urlEncode(test.plainText)
		if got != test.minimallyEncoded && got != test.fullyEncoded {
			t.Errorf("urlEncode(%q) got %q wanted %q or %q", test.plainText, got, test.minimallyEncoded, test.fullyEncoded)
		}
	}
}

func TestTimeString(t *testing.T) {
	for _, test := range []struct {
		in   time.Time
		want string
	}{
		{fstest.Time("1970-01-01T00:00:00.000000000Z"), "0"},
		{fstest.Time("2001-02-03T04:05:10.123123123Z"), "981173110123"},
		{fstest.Time("2001-02-03T05:05:10.123123123+01:00"), "981173110123"},
	} {
		got := timeString(test.in)
		if test.want != got {
			t.Logf("%v: want %v got %v", test.in, test.want, got)
		}
	}

}

func TestParseTimeString(t *testing.T) {
	for _, test := range []struct {
		in        string
		want      time.Time
		wantError string
	}{
		{"0", fstest.Time("1970-01-01T00:00:00.000000000Z"), ""},
		{"981173110123", fstest.Time("2001-02-03T04:05:10.123000000Z"), ""},
		{"", time.Time{}, ""},
		{"potato", time.Time{}, `strconv.ParseInt: parsing "potato": invalid syntax`},
	} {
		o := Object{}
		err := o.parseTimeString(test.in)
		got := o.modTime
		var gotError string
		if err != nil {
			gotError = err.Error()
		}
		if test.want != got {
			t.Logf("%v: want %v got %v", test.in, test.want, got)
		}
		if test.wantError != gotError {
			t.Logf("%v: want error %v got error %v", test.in, test.wantError, gotError)
		}
	}

}

// Return a map of the headers in the options with keys stripped of the "x-bz-info-" prefix
func OpenOptionToMetaData(options []fs.OpenOption) map[string]string {
	var headers = make(map[string]string)
	for _, option := range options {
		k, v := option.Header()
		k = strings.ToLower(k)
		if strings.HasPrefix(k, headerPrefix) {
			headers[k[len(headerPrefix):]] = v
		}
	}

	return headers
}

func (f *Fs) internalTestMetadata(t *testing.T, size string, uploadCutoff string, chunkSize string) {
	what := fmt.Sprintf("Size%s/UploadCutoff%s/ChunkSize%s", size, uploadCutoff, chunkSize)
	t.Run(what, func(t *testing.T) {
		ctx := context.Background()

		ss := fs.SizeSuffix(0)
		err := ss.Set(size)
		require.NoError(t, err)
		original := random.String(int(ss))

		contents := fstest.Gz(t, original)
		mimeType := "text/html"

		if chunkSize != "" {
			ss := fs.SizeSuffix(0)
			err := ss.Set(chunkSize)
			require.NoError(t, err)
			_, err = f.SetUploadChunkSize(ss)
			require.NoError(t, err)
		}

		if uploadCutoff != "" {
			ss := fs.SizeSuffix(0)
			err := ss.Set(uploadCutoff)
			require.NoError(t, err)
			_, err = f.SetUploadCutoff(ss)
			require.NoError(t, err)
		}

		item := fstest.NewItem("test-metadata", contents, fstest.Time("2001-05-06T04:05:06.499Z"))
		btime := time.Now()
		metadata := fs.Metadata{
			// Just mtime for now - limit to milliseconds since x-bz-info-src_last_modified_millis can't support any

			"mtime": "2009-05-06T04:05:06.499Z",
		}

		// Need to specify HTTP options with the header prefix since they are passed as-is
		options := []fs.OpenOption{
			&fs.HTTPOption{Key: "X-Bz-Info-a", Value: "1"},
			&fs.HTTPOption{Key: "X-Bz-Info-b", Value: "2"},
		}

		obj := fstests.PutTestContentsMetadata(ctx, t, f, &item, true, contents, true, mimeType, metadata, options...)
		defer func() {
			assert.NoError(t, obj.Remove(ctx))
		}()
		o := obj.(*Object)
		gotMetadata, err := o.getMetaData(ctx)
		require.NoError(t, err)

		// X-Bz-Info-a & X-Bz-Info-b
		optMetadata := OpenOptionToMetaData(options)
		for k, v := range optMetadata {
			got := gotMetadata.Info[k]
			assert.Equal(t, v, got, k)
		}

		// mtime
		for k, v := range metadata {
			got := o.meta[k]
			assert.Equal(t, v, got, k)
		}

		assert.Equal(t, mimeType, gotMetadata.ContentType, "Content-Type")

		// Modification time from the x-bz-info-src_last_modified_millis header
		var mtime api.Timestamp
		err = mtime.UnmarshalJSON([]byte(gotMetadata.Info[timeKey]))
		if err != nil {
			fs.Debugf(o, "Bad "+timeHeader+" header: %v", err)
		}
		assert.Equal(t, item.ModTime, time.Time(mtime), "Modification time")

		// Upload time
		gotBtime := time.Time(gotMetadata.UploadTimestamp)
		dt := gotBtime.Sub(btime)
		assert.True(t, dt < time.Minute && dt > -time.Minute, fmt.Sprintf("btime more than 1 minute out want %v got %v delta %v", btime, gotBtime, dt))

		t.Run("GzipEncoding", func(t *testing.T) {
			// Test that the gzipped file we uploaded can be
			// downloaded
			checkDownload := func(wantContents string, wantSize int64, wantHash string) {
				gotContents := fstests.ReadObject(ctx, t, o, -1)
				assert.Equal(t, wantContents, gotContents)
				assert.Equal(t, wantSize, o.Size())
				gotHash, err := o.Hash(ctx, hash.SHA1)
				require.NoError(t, err)
				assert.Equal(t, wantHash, gotHash)
			}

			t.Run("NoDecompress", func(t *testing.T) {
				checkDownload(contents, int64(len(contents)), sha1Sum(t, contents))
			})
		})
	})
}

func (f *Fs) InternalTestMetadata(t *testing.T) {
	// 1 kB regular file
	f.internalTestMetadata(t, "1kiB", "", "")

	// 10 MiB large file
	f.internalTestMetadata(t, "10MiB", "6MiB", "6MiB")
}

func sha1Sum(t *testing.T, s string) string {
	hash := sha1.Sum([]byte(s))
	return fmt.Sprintf("%x", hash)
}

// This is adapted from the s3 equivalent.
func (f *Fs) InternalTestVersions(t *testing.T) {
	ctx := context.Background()

	// Small pause to make the LastModified different since AWS
	// only seems to track them to 1 second granularity
	time.Sleep(2 * time.Second)

	// Create an object
	const dirName = "versions"
	const fileName = dirName + "/" + "test-versions.txt"
	contents := random.String(100)
	item := fstest.NewItem(fileName, contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
	obj := fstests.PutTestContents(ctx, t, f, &item, contents, true)
	defer func() {
		assert.NoError(t, obj.Remove(ctx))
	}()
	objMetadata, err := obj.(*Object).getMetaData(ctx)
	require.NoError(t, err)

	// Small pause
	time.Sleep(2 * time.Second)

	// Remove it
	assert.NoError(t, obj.Remove(ctx))

	// Small pause to make the LastModified different since AWS only seems to track them to 1 second granularity
	time.Sleep(2 * time.Second)

	// And create it with different size and contents
	newContents := random.String(101)
	newItem := fstest.NewItem(fileName, newContents, fstest.Time("2002-05-06T04:05:06.499999999Z"))
	newObj := fstests.PutTestContents(ctx, t, f, &newItem, newContents, true)
	newObjMetadata, err := newObj.(*Object).getMetaData(ctx)
	require.NoError(t, err)

	t.Run("Versions", func(t *testing.T) {
		// Set --b2-versions for this test
		f.opt.Versions = true
		defer func() {
			f.opt.Versions = false
		}()

		// Read the contents
		entries, err := f.List(ctx, dirName)
		require.NoError(t, err)
		tests := 0
		var fileNameVersion string
		for _, entry := range entries {
			t.Log(entry)
			remote := entry.Remote()
			if remote == fileName {
				t.Run("ReadCurrent", func(t *testing.T) {
					assert.Equal(t, newContents, fstests.ReadObject(ctx, t, entry.(fs.Object), -1))
				})
				tests++
			} else if versionTime, p := version.Remove(remote); !versionTime.IsZero() && p == fileName {
				t.Run("ReadVersion", func(t *testing.T) {
					assert.Equal(t, contents, fstests.ReadObject(ctx, t, entry.(fs.Object), -1))
				})
				assert.WithinDuration(t, time.Time(objMetadata.UploadTimestamp), versionTime, time.Second, "object time must be with 1 second of version time")
				fileNameVersion = remote
				tests++
			}
		}
		assert.Equal(t, 2, tests, "object missing from listing")

		// Check we can read the object with a version suffix
		t.Run("NewObject", func(t *testing.T) {
			o, err := f.NewObject(ctx, fileNameVersion)
			require.NoError(t, err)
			require.NotNil(t, o)
			assert.Equal(t, int64(100), o.Size(), o.Remote())
		})

		// Check we can make a NewFs from that object with a version suffix
		t.Run("NewFs", func(t *testing.T) {
			newPath := bucket.Join(fs.ConfigStringFull(f), fileNameVersion)
			// Make sure --b2-versions is set in the config of the new remote
			fs.Debugf(nil, "oldPath = %q", newPath)
			lastColon := strings.LastIndex(newPath, ":")
			require.True(t, lastColon >= 0)
			newPath = newPath[:lastColon] + ",versions" + newPath[lastColon:]
			fs.Debugf(nil, "newPath = %q", newPath)
			fNew, err := cache.Get(ctx, newPath)
			// This should return pointing to a file
			require.Equal(t, fs.ErrorIsFile, err)
			require.NotNil(t, fNew)
			// With the directory above
			assert.Equal(t, dirName, path.Base(fs.ConfigStringFull(fNew)))
		})
	})

	t.Run("VersionAt", func(t *testing.T) {
		// We set --b2-version-at for this test so make sure we reset it at the end
		defer func() {
			f.opt.VersionAt = fs.Time{}
		}()

		var (
			firstObjectTime  = time.Time(objMetadata.UploadTimestamp)
			secondObjectTime = time.Time(newObjMetadata.UploadTimestamp)
		)

		for _, test := range []struct {
			what     string
			at       time.Time
			want     []fstest.Item
			wantErr  error
			wantSize int64
		}{
			{
				what:    "Before",
				at:      firstObjectTime.Add(-time.Second),
				want:    fstests.InternalTestFiles,
				wantErr: fs.ErrorObjectNotFound,
			},
			{
				what:     "AfterOne",
				at:       firstObjectTime.Add(time.Second),
				want:     append([]fstest.Item{item}, fstests.InternalTestFiles...),
				wantSize: 100,
			},
			{
				what:    "AfterDelete",
				at:      secondObjectTime.Add(-time.Second),
				want:    fstests.InternalTestFiles,
				wantErr: fs.ErrorObjectNotFound,
			},
			{
				what:     "AfterTwo",
				at:       secondObjectTime.Add(time.Second),
				want:     append([]fstest.Item{newItem}, fstests.InternalTestFiles...),
				wantSize: 101,
			},
		} {
			t.Run(test.what, func(t *testing.T) {
				f.opt.VersionAt = fs.Time(test.at)
				t.Run("List", func(t *testing.T) {
					fstest.CheckListing(t, f, test.want)
				})
				// b2 NewObject doesn't work with VersionAt
				//t.Run("NewObject", func(t *testing.T) {
				//	gotObj, gotErr := f.NewObject(ctx, fileName)
				//	assert.Equal(t, test.wantErr, gotErr)
				//	if gotErr == nil {
				//		assert.Equal(t, test.wantSize, gotObj.Size())
				//	}
				//})
			})
		}
	})

	t.Run("Cleanup", func(t *testing.T) {
		require.NoError(t, f.cleanUp(ctx, true, false, 0))
		items := append([]fstest.Item{newItem}, fstests.InternalTestFiles...)
		fstest.CheckListing(t, f, items)
		// Set --b2-versions for this test
		f.opt.Versions = true
		defer func() {
			f.opt.Versions = false
		}()
		fstest.CheckListing(t, f, items)
	})

	// Purge gets tested later
}

// -run TestIntegration/FsMkdir/FsPutFiles/Internal
func (f *Fs) InternalTest(t *testing.T) {
	t.Run("Metadata", f.InternalTestMetadata)
	t.Run("Versions", f.InternalTestVersions)
}

var _ fstests.InternalTester = (*Fs)(nil)
