package chunker

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Command line flags
var (
	UploadKilobytes = flag.Int("upload-kilobytes", 0, "Upload size in Kilobytes, set this to test large uploads")
)

// test that chunking does not break large uploads
func testPutLarge(t *testing.T, f *Fs, kilobytes int) {
	t.Run(fmt.Sprintf("PutLarge%dk", kilobytes), func(t *testing.T) {
		fstests.TestPutLarge(context.Background(), t, f, &fstest.Item{
			ModTime: fstest.Time("2001-02-03T04:05:06.499999999Z"),
			Path:    fmt.Sprintf("chunker-upload-%dk", kilobytes),
			Size:    int64(kilobytes) * int64(fs.KibiByte),
		})
	})
}

// test chunk name parser
func testChunkNameFormat(t *testing.T, f *Fs) {
	saveOpt := f.opt
	defer func() {
		// restore original settings (f is pointer, f.opt is struct)
		f.opt = saveOpt
		_ = f.setChunkNameFormat(f.opt.NameFormat)
	}()

	assertFormat := func(pattern, wantDataFormat, wantCtrlFormat, wantNameRegexp string) {
		err := f.setChunkNameFormat(pattern)
		assert.NoError(t, err)
		assert.Equal(t, wantDataFormat, f.dataNameFmt)
		assert.Equal(t, wantCtrlFormat, f.ctrlNameFmt)
		assert.Equal(t, wantNameRegexp, f.nameRegexp.String())
	}

	assertFormatValid := func(pattern string) {
		err := f.setChunkNameFormat(pattern)
		assert.NoError(t, err)
	}

	assertFormatInvalid := func(pattern string) {
		err := f.setChunkNameFormat(pattern)
		assert.Error(t, err)
	}

	assertMakeName := func(wantChunkName, mainName string, chunkNo int, ctrlType, xactID string) {
		gotChunkName := ""
		assert.NotPanics(t, func() {
			gotChunkName = f.makeChunkName(mainName, chunkNo, ctrlType, xactID)
		}, "makeChunkName(%q,%d,%q,%q) must not panic", mainName, chunkNo, ctrlType, xactID)
		if gotChunkName != "" {
			assert.Equal(t, wantChunkName, gotChunkName)
		}
	}

	assertMakeNamePanics := func(mainName string, chunkNo int, ctrlType, xactID string) {
		assert.Panics(t, func() {
			_ = f.makeChunkName(mainName, chunkNo, ctrlType, xactID)
		}, "makeChunkName(%q,%d,%q,%q) should panic", mainName, chunkNo, ctrlType, xactID)
	}

	assertParseName := func(fileName, wantMainName string, wantChunkNo int, wantCtrlType, wantXactID string) {
		gotMainName, gotChunkNo, gotCtrlType, gotXactID := f.parseChunkName(fileName)
		assert.Equal(t, wantMainName, gotMainName)
		assert.Equal(t, wantChunkNo, gotChunkNo)
		assert.Equal(t, wantCtrlType, gotCtrlType)
		assert.Equal(t, wantXactID, gotXactID)
	}

	const newFormatSupported = false // support for patterns not starting with base name (*)

	// valid formats
	assertFormat(`*.rclone_chunk.###`, `%s.rclone_chunk.%03d`, `%s.rclone_chunk._%s`, `^(.+?)\.rclone_chunk\.(?:([0-9]{3,})|_([a-z][a-z0-9]{2,6}))(?:_([0-9a-z]{4,9})|\.\.tmp_([0-9]{10,13}))?$`)
	assertFormat(`*.rclone_chunk.#`, `%s.rclone_chunk.%d`, `%s.rclone_chunk._%s`, `^(.+?)\.rclone_chunk\.(?:([0-9]+)|_([a-z][a-z0-9]{2,6}))(?:_([0-9a-z]{4,9})|\.\.tmp_([0-9]{10,13}))?$`)
	assertFormat(`*_chunk_#####`, `%s_chunk_%05d`, `%s_chunk__%s`, `^(.+?)_chunk_(?:([0-9]{5,})|_([a-z][a-z0-9]{2,6}))(?:_([0-9a-z]{4,9})|\.\.tmp_([0-9]{10,13}))?$`)
	assertFormat(`*-chunk-#`, `%s-chunk-%d`, `%s-chunk-_%s`, `^(.+?)-chunk-(?:([0-9]+)|_([a-z][a-z0-9]{2,6}))(?:_([0-9a-z]{4,9})|\.\.tmp_([0-9]{10,13}))?$`)
	assertFormat(`*-chunk-#-%^$()[]{}.+-!?:\`, `%s-chunk-%d-%%^$()[]{}.+-!?:\`, `%s-chunk-_%s-%%^$()[]{}.+-!?:\`, `^(.+?)-chunk-(?:([0-9]+)|_([a-z][a-z0-9]{2,6}))-%\^\$\(\)\[\]\{\}\.\+-!\?:\\(?:_([0-9a-z]{4,9})|\.\.tmp_([0-9]{10,13}))?$`)
	if newFormatSupported {
		assertFormat(`_*-chunk-##,`, `_%s-chunk-%02d,`, `_%s-chunk-_%s,`, `^_(.+?)-chunk-(?:([0-9]{2,})|_([a-z][a-z0-9]{2,6})),(?:_([0-9a-z]{4,9})|\.\.tmp_([0-9]{10,13}))?$`)
	}

	// invalid formats
	assertFormatInvalid(`chunk-#`)
	assertFormatInvalid(`*-chunk`)
	assertFormatInvalid(`*-*-chunk-#`)
	assertFormatInvalid(`*-chunk-#-#`)
	assertFormatInvalid(`#-chunk-*`)
	assertFormatInvalid(`*/#`)

	assertFormatValid(`*#`)
	assertFormatInvalid(`**#`)
	assertFormatInvalid(`#*`)
	assertFormatInvalid(``)
	assertFormatInvalid(`-`)

	// quick tests
	if newFormatSupported {
		assertFormat(`part_*_#`, `part_%s_%d`, `part_%s__%s`, `^part_(.+?)_(?:([0-9]+)|_([a-z][a-z0-9]{2,6}))(?:_([0-9][0-9a-z]{3,8})\.\.tmp_([0-9]{10,13}))?$`)
		f.opt.StartFrom = 1

		assertMakeName(`part_fish_1`, "fish", 0, "", "")
		assertParseName(`part_fish_43`, "fish", 42, "", "")
		assertMakeName(`part_fish__locks`, "fish", -2, "locks", "")
		assertParseName(`part_fish__locks`, "fish", -1, "locks", "")
		assertMakeName(`part_fish__x2y`, "fish", -2, "x2y", "")
		assertParseName(`part_fish__x2y`, "fish", -1, "x2y", "")
		assertMakeName(`part_fish_3_0004`, "fish", 2, "", "4")
		assertParseName(`part_fish_4_0005`, "fish", 3, "", "0005")
		assertMakeName(`part_fish__blkinfo_jj5fvo3wr`, "fish", -3, "blkinfo", "jj5fvo3wr")
		assertParseName(`part_fish__blkinfo_zz9fvo3wr`, "fish", -1, "blkinfo", "zz9fvo3wr")

		// old-style temporary suffix (parse only)
		assertParseName(`part_fish_4..tmp_0000000011`, "fish", 3, "", "000b")
		assertParseName(`part_fish__blkinfo_jj5fvo3wr`, "fish", -1, "blkinfo", "jj5fvo3wr")
	}

	// prepare format for long tests
	assertFormat(`*.chunk.###`, `%s.chunk.%03d`, `%s.chunk._%s`, `^(.+?)\.chunk\.(?:([0-9]{3,})|_([a-z][a-z0-9]{2,6}))(?:_([0-9a-z]{4,9})|\.\.tmp_([0-9]{10,13}))?$`)
	f.opt.StartFrom = 2

	// valid data chunks
	assertMakeName(`fish.chunk.003`, "fish", 1, "", "")
	assertParseName(`fish.chunk.003`, "fish", 1, "", "")
	assertMakeName(`fish.chunk.021`, "fish", 19, "", "")
	assertParseName(`fish.chunk.021`, "fish", 19, "", "")

	// valid temporary data chunks
	assertMakeName(`fish.chunk.011_4321`, "fish", 9, "", "4321")
	assertParseName(`fish.chunk.011_4321`, "fish", 9, "", "4321")
	assertMakeName(`fish.chunk.011_00bc`, "fish", 9, "", "00bc")
	assertParseName(`fish.chunk.011_00bc`, "fish", 9, "", "00bc")
	assertMakeName(`fish.chunk.1916_5jjfvo3wr`, "fish", 1914, "", "5jjfvo3wr")
	assertParseName(`fish.chunk.1916_5jjfvo3wr`, "fish", 1914, "", "5jjfvo3wr")
	assertMakeName(`fish.chunk.1917_zz9fvo3wr`, "fish", 1915, "", "zz9fvo3wr")
	assertParseName(`fish.chunk.1917_zz9fvo3wr`, "fish", 1915, "", "zz9fvo3wr")

	// valid temporary data chunks (old temporary suffix, only parse)
	assertParseName(`fish.chunk.004..tmp_0000000047`, "fish", 2, "", "001b")
	assertParseName(`fish.chunk.323..tmp_9994567890123`, "fish", 321, "", "3jjfvo3wr")

	// parsing invalid data chunk names
	assertParseName(`fish.chunk.3`, "", -1, "", "")
	assertParseName(`fish.chunk.001`, "", -1, "", "")
	assertParseName(`fish.chunk.21`, "", -1, "", "")
	assertParseName(`fish.chunk.-21`, "", -1, "", "")

	assertParseName(`fish.chunk.004abcd`, "", -1, "", "")        // missing underscore delimiter
	assertParseName(`fish.chunk.004__1234`, "", -1, "", "")      // extra underscore delimiter
	assertParseName(`fish.chunk.004_123`, "", -1, "", "")        // too short temporary suffix
	assertParseName(`fish.chunk.004_1234567890`, "", -1, "", "") // too long temporary suffix
	assertParseName(`fish.chunk.004_-1234`, "", -1, "", "")      // temporary suffix must be positive
	assertParseName(`fish.chunk.004_123E`, "", -1, "", "")       // uppercase not allowed
	assertParseName(`fish.chunk.004_12.3`, "", -1, "", "")       // punctuation not allowed

	// parsing invalid data chunk names (old temporary suffix)
	assertParseName(`fish.chunk.004.tmp_0000000021`, "", -1, "", "")
	assertParseName(`fish.chunk.003..tmp_123456789`, "", -1, "", "")
	assertParseName(`fish.chunk.003..tmp_012345678901234567890123456789`, "", -1, "", "")
	assertParseName(`fish.chunk.323..tmp_12345678901234`, "", -1, "", "")
	assertParseName(`fish.chunk.003..tmp_-1`, "", -1, "", "")

	// valid control chunks
	assertMakeName(`fish.chunk._info`, "fish", -1, "info", "")
	assertMakeName(`fish.chunk._locks`, "fish", -2, "locks", "")
	assertMakeName(`fish.chunk._blkinfo`, "fish", -3, "blkinfo", "")
	assertMakeName(`fish.chunk._x2y`, "fish", -4, "x2y", "")

	assertParseName(`fish.chunk._info`, "fish", -1, "info", "")
	assertParseName(`fish.chunk._locks`, "fish", -1, "locks", "")
	assertParseName(`fish.chunk._blkinfo`, "fish", -1, "blkinfo", "")
	assertParseName(`fish.chunk._x2y`, "fish", -1, "x2y", "")

	// valid temporary control chunks
	assertMakeName(`fish.chunk._info_0001`, "fish", -1, "info", "1")
	assertMakeName(`fish.chunk._locks_4321`, "fish", -2, "locks", "4321")
	assertMakeName(`fish.chunk._uploads_abcd`, "fish", -3, "uploads", "abcd")
	assertMakeName(`fish.chunk._blkinfo_xyzabcdef`, "fish", -4, "blkinfo", "xyzabcdef")
	assertMakeName(`fish.chunk._x2y_1aaa`, "fish", -5, "x2y", "1aaa")

	assertParseName(`fish.chunk._info_0001`, "fish", -1, "info", "0001")
	assertParseName(`fish.chunk._locks_4321`, "fish", -1, "locks", "4321")
	assertParseName(`fish.chunk._uploads_9abc`, "fish", -1, "uploads", "9abc")
	assertParseName(`fish.chunk._blkinfo_xyzabcdef`, "fish", -1, "blkinfo", "xyzabcdef")
	assertParseName(`fish.chunk._x2y_1aaa`, "fish", -1, "x2y", "1aaa")

	// valid temporary control chunks (old temporary suffix, parse only)
	assertParseName(`fish.chunk._info..tmp_0000000047`, "fish", -1, "info", "001b")
	assertParseName(`fish.chunk._locks..tmp_0000054321`, "fish", -1, "locks", "15wx")
	assertParseName(`fish.chunk._uploads..tmp_0000000000`, "fish", -1, "uploads", "0000")
	assertParseName(`fish.chunk._blkinfo..tmp_9994567890123`, "fish", -1, "blkinfo", "3jjfvo3wr")
	assertParseName(`fish.chunk._x2y..tmp_0000000000`, "fish", -1, "x2y", "0000")

	// parsing invalid control chunk names
	assertParseName(`fish.chunk.metadata`, "", -1, "", "") // must be prepended by underscore
	assertParseName(`fish.chunk.info`, "", -1, "", "")
	assertParseName(`fish.chunk.locks`, "", -1, "", "")
	assertParseName(`fish.chunk.uploads`, "", -1, "", "")

	assertParseName(`fish.chunk._os`, "", -1, "", "")        // too short
	assertParseName(`fish.chunk._metadata`, "", -1, "", "")  // too long
	assertParseName(`fish.chunk._blockinfo`, "", -1, "", "") // way too long
	assertParseName(`fish.chunk._4me`, "", -1, "", "")       // cannot start with digit
	assertParseName(`fish.chunk._567`, "", -1, "", "")       // cannot be all digits
	assertParseName(`fish.chunk._me_ta`, "", -1, "", "")     // punctuation not allowed
	assertParseName(`fish.chunk._in-fo`, "", -1, "", "")
	assertParseName(`fish.chunk._.bin`, "", -1, "", "")
	assertParseName(`fish.chunk._.2xy`, "", -1, "", "")

	// parsing invalid temporary control chunks
	assertParseName(`fish.chunk._blkinfo1234`, "", -1, "", "")     // missing underscore delimiter
	assertParseName(`fish.chunk._info__1234`, "", -1, "", "")      // extra underscore delimiter
	assertParseName(`fish.chunk._info_123`, "", -1, "", "")        // too short temporary suffix
	assertParseName(`fish.chunk._info_1234567890`, "", -1, "", "") // too long temporary suffix
	assertParseName(`fish.chunk._info_-1234`, "", -1, "", "")      // temporary suffix must be positive
	assertParseName(`fish.chunk._info_123E`, "", -1, "", "")       // uppercase not allowed
	assertParseName(`fish.chunk._info_12.3`, "", -1, "", "")       // punctuation not allowed

	assertParseName(`fish.chunk._locks..tmp_123456789`, "", -1, "", "")
	assertParseName(`fish.chunk._meta..tmp_-1`, "", -1, "", "")
	assertParseName(`fish.chunk._blockinfo..tmp_012345678901234567890123456789`, "", -1, "", "")

	// short control chunk names: 3 letters ok, 1-2 letters not allowed
	assertMakeName(`fish.chunk._ext`, "fish", -1, "ext", "")
	assertParseName(`fish.chunk._int`, "fish", -1, "int", "")

	assertMakeNamePanics("fish", -1, "in", "")
	assertMakeNamePanics("fish", -1, "up", "4")
	assertMakeNamePanics("fish", -1, "x", "")
	assertMakeNamePanics("fish", -1, "c", "1z")

	assertMakeName(`fish.chunk._ext_0000`, "fish", -1, "ext", "0")
	assertMakeName(`fish.chunk._ext_0026`, "fish", -1, "ext", "26")
	assertMakeName(`fish.chunk._int_0abc`, "fish", -1, "int", "abc")
	assertMakeName(`fish.chunk._int_9xyz`, "fish", -1, "int", "9xyz")
	assertMakeName(`fish.chunk._out_jj5fvo3wr`, "fish", -1, "out", "jj5fvo3wr")
	assertMakeName(`fish.chunk._out_jj5fvo3wr`, "fish", -1, "out", "jj5fvo3wr")

	assertParseName(`fish.chunk._ext_0000`, "fish", -1, "ext", "0000")
	assertParseName(`fish.chunk._ext_0026`, "fish", -1, "ext", "0026")
	assertParseName(`fish.chunk._int_0abc`, "fish", -1, "int", "0abc")
	assertParseName(`fish.chunk._int_9xyz`, "fish", -1, "int", "9xyz")
	assertParseName(`fish.chunk._out_jj5fvo3wr`, "fish", -1, "out", "jj5fvo3wr")
	assertParseName(`fish.chunk._out_jj5fvo3wr`, "fish", -1, "out", "jj5fvo3wr")

	// base file name can sometimes look like a valid chunk name
	assertParseName(`fish.chunk.003.chunk.004`, "fish.chunk.003", 2, "", "")
	assertParseName(`fish.chunk.003.chunk._info`, "fish.chunk.003", -1, "info", "")
	assertParseName(`fish.chunk.003.chunk._Meta`, "", -1, "", "")

	assertParseName(`fish.chunk._info.chunk.004`, "fish.chunk._info", 2, "", "")
	assertParseName(`fish.chunk._info.chunk._info`, "fish.chunk._info", -1, "info", "")
	assertParseName(`fish.chunk._info.chunk._info.chunk._Meta`, "", -1, "", "")

	// base file name looking like a valid chunk name (old temporary suffix)
	assertParseName(`fish.chunk.003.chunk.005..tmp_0000000022`, "fish.chunk.003", 3, "", "000m")
	assertParseName(`fish.chunk.003.chunk._x..tmp_0000054321`, "", -1, "", "")
	assertParseName(`fish.chunk._info.chunk.005..tmp_0000000023`, "fish.chunk._info", 3, "", "000n")
	assertParseName(`fish.chunk._info.chunk._info.chunk._x..tmp_0000054321`, "", -1, "", "")

	assertParseName(`fish.chunk.003.chunk._blkinfo..tmp_9994567890123`, "fish.chunk.003", -1, "blkinfo", "3jjfvo3wr")
	assertParseName(`fish.chunk._info.chunk._blkinfo..tmp_9994567890123`, "fish.chunk._info", -1, "blkinfo", "3jjfvo3wr")

	assertParseName(`fish.chunk.004..tmp_0000000021.chunk.004`, "fish.chunk.004..tmp_0000000021", 2, "", "")
	assertParseName(`fish.chunk.004..tmp_0000000021.chunk.005..tmp_0000000025`, "fish.chunk.004..tmp_0000000021", 3, "", "000p")
	assertParseName(`fish.chunk.004..tmp_0000000021.chunk._info`, "fish.chunk.004..tmp_0000000021", -1, "info", "")
	assertParseName(`fish.chunk.004..tmp_0000000021.chunk._blkinfo..tmp_9994567890123`, "fish.chunk.004..tmp_0000000021", -1, "blkinfo", "3jjfvo3wr")
	assertParseName(`fish.chunk.004..tmp_0000000021.chunk._Meta`, "", -1, "", "")
	assertParseName(`fish.chunk.004..tmp_0000000021.chunk._x..tmp_0000054321`, "", -1, "", "")

	assertParseName(`fish.chunk._blkinfo..tmp_9994567890123.chunk.004`, "fish.chunk._blkinfo..tmp_9994567890123", 2, "", "")
	assertParseName(`fish.chunk._blkinfo..tmp_9994567890123.chunk.005..tmp_0000000026`, "fish.chunk._blkinfo..tmp_9994567890123", 3, "", "000q")
	assertParseName(`fish.chunk._blkinfo..tmp_9994567890123.chunk._info`, "fish.chunk._blkinfo..tmp_9994567890123", -1, "info", "")
	assertParseName(`fish.chunk._blkinfo..tmp_9994567890123.chunk._blkinfo..tmp_9994567890123`, "fish.chunk._blkinfo..tmp_9994567890123", -1, "blkinfo", "3jjfvo3wr")
	assertParseName(`fish.chunk._blkinfo..tmp_9994567890123.chunk._info.chunk._Meta`, "", -1, "", "")
	assertParseName(`fish.chunk._blkinfo..tmp_9994567890123.chunk._info.chunk._x..tmp_0000054321`, "", -1, "", "")

	assertParseName(`fish.chunk._blkinfo..tmp_1234567890123456789.chunk.004`, "fish.chunk._blkinfo..tmp_1234567890123456789", 2, "", "")
	assertParseName(`fish.chunk._blkinfo..tmp_1234567890123456789.chunk.005..tmp_0000000022`, "fish.chunk._blkinfo..tmp_1234567890123456789", 3, "", "000m")
	assertParseName(`fish.chunk._blkinfo..tmp_1234567890123456789.chunk._info`, "fish.chunk._blkinfo..tmp_1234567890123456789", -1, "info", "")
	assertParseName(`fish.chunk._blkinfo..tmp_1234567890123456789.chunk._blkinfo..tmp_9994567890123`, "fish.chunk._blkinfo..tmp_1234567890123456789", -1, "blkinfo", "3jjfvo3wr")
	assertParseName(`fish.chunk._blkinfo..tmp_1234567890123456789.chunk._info.chunk._Meta`, "", -1, "", "")
	assertParseName(`fish.chunk._blkinfo..tmp_1234567890123456789.chunk._info.chunk._x..tmp_0000054321`, "", -1, "", "")

	// attempts to make invalid chunk names
	assertMakeNamePanics("fish", -1, "", "")          // neither data nor control
	assertMakeNamePanics("fish", 0, "info", "")       // both data and control
	assertMakeNamePanics("fish", -1, "metadata", "")  // control type too long
	assertMakeNamePanics("fish", -1, "blockinfo", "") // control type way too long
	assertMakeNamePanics("fish", -1, "2xy", "")       // first digit not allowed
	assertMakeNamePanics("fish", -1, "123", "")       // all digits not allowed
	assertMakeNamePanics("fish", -1, "Meta", "")      // only lower case letters allowed
	assertMakeNamePanics("fish", -1, "in-fo", "")     // punctuation not allowed
	assertMakeNamePanics("fish", -1, "_info", "")
	assertMakeNamePanics("fish", -1, "info_", "")
	assertMakeNamePanics("fish", -2, ".bind", "")
	assertMakeNamePanics("fish", -2, "bind.", "")

	assertMakeNamePanics("fish", -1, "", "1")          // neither data nor control
	assertMakeNamePanics("fish", 0, "info", "23")      // both data and control
	assertMakeNamePanics("fish", -1, "metadata", "45") // control type too long
	assertMakeNamePanics("fish", -1, "blockinfo", "7") // control type way too long
	assertMakeNamePanics("fish", -1, "2xy", "abc")     // first digit not allowed
	assertMakeNamePanics("fish", -1, "123", "def")     // all digits not allowed
	assertMakeNamePanics("fish", -1, "Meta", "mnk")    // only lower case letters allowed
	assertMakeNamePanics("fish", -1, "in-fo", "xyz")   // punctuation not allowed
	assertMakeNamePanics("fish", -1, "_info", "5678")
	assertMakeNamePanics("fish", -1, "info_", "999")
	assertMakeNamePanics("fish", -2, ".bind", "0")
	assertMakeNamePanics("fish", -2, "bind.", "0")

	assertMakeNamePanics("fish", 0, "", "1234567890") // temporary suffix too long
	assertMakeNamePanics("fish", 0, "", "123F4")      // uppercase not allowed
	assertMakeNamePanics("fish", 0, "", "123.")       // punctuation not allowed
	assertMakeNamePanics("fish", 0, "", "_123")
}

func testSmallFileInternals(t *testing.T, f *Fs) {
	const dir = "small"
	ctx := context.Background()
	saveOpt := f.opt
	defer func() {
		f.opt.FailHard = false
		_ = operations.Purge(ctx, f.base, dir)
		f.opt = saveOpt
	}()
	f.opt.FailHard = false

	modTime := fstest.Time("2001-02-03T04:05:06.499999999Z")

	checkSmallFileInternals := func(obj fs.Object) {
		assert.NotNil(t, obj)
		o, ok := obj.(*Object)
		assert.True(t, ok)
		assert.NotNil(t, o)
		if o == nil {
			return
		}
		switch {
		case !f.useMeta:
			// If meta format is "none", non-chunked file (even empty)
			// internally is a single chunk without meta object.
			assert.Nil(t, o.main)
			assert.True(t, o.isComposite()) // sorry, sometimes a name is misleading
			assert.Equal(t, 1, len(o.chunks))
		case f.hashAll:
			// Consistent hashing forces meta object on small files too
			assert.NotNil(t, o.main)
			assert.True(t, o.isComposite())
			assert.Equal(t, 1, len(o.chunks))
		default:
			// normally non-chunked file is kept in the Object's main field
			assert.NotNil(t, o.main)
			assert.False(t, o.isComposite())
			assert.Equal(t, 0, len(o.chunks))
		}
	}

	checkContents := func(obj fs.Object, contents string) {
		assert.NotNil(t, obj)
		assert.Equal(t, int64(len(contents)), obj.Size())

		r, err := obj.Open(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, r)
		if r == nil {
			return
		}
		data, err := ioutil.ReadAll(r)
		assert.NoError(t, err)
		assert.Equal(t, contents, string(data))
		_ = r.Close()
	}

	checkHashsum := func(obj fs.Object) {
		var ht hash.Type
		switch {
		case !f.hashAll:
			return
		case f.useMD5:
			ht = hash.MD5
		case f.useSHA1:
			ht = hash.SHA1
		default:
			return
		}
		// even empty files must have hashsum in consistent mode
		sum, err := obj.Hash(ctx, ht)
		assert.NoError(t, err)
		assert.NotEqual(t, sum, "")
	}

	checkSmallFile := func(name, contents string) {
		filename := path.Join(dir, name)
		item := fstest.Item{Path: filename, ModTime: modTime}
		_, put := fstests.PutTestContents(ctx, t, f, &item, contents, false)
		assert.NotNil(t, put)
		checkSmallFileInternals(put)
		checkContents(put, contents)
		checkHashsum(put)

		// objects returned by Put and NewObject must have similar structure
		obj, err := f.NewObject(ctx, filename)
		assert.NoError(t, err)
		assert.NotNil(t, obj)
		checkSmallFileInternals(obj)
		checkContents(obj, contents)
		checkHashsum(obj)

		_ = obj.Remove(ctx)
		_ = put.Remove(ctx) // for good
	}

	checkSmallFile("emptyfile", "")
	checkSmallFile("smallfile", "Ok")
}

func testPreventCorruption(t *testing.T, f *Fs) {
	if f.opt.ChunkSize > 50 {
		t.Skip("this test requires small chunks")
	}
	const dir = "corrupted"
	ctx := context.Background()
	saveOpt := f.opt
	defer func() {
		f.opt.FailHard = false
		_ = operations.Purge(ctx, f.base, dir)
		f.opt = saveOpt
	}()
	f.opt.FailHard = true

	contents := random.String(250)
	modTime := fstest.Time("2001-02-03T04:05:06.499999999Z")
	const overlapMessage = "chunk overlap"

	assertOverlapError := func(err error) {
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), overlapMessage)
		}
	}

	newFile := func(name string) fs.Object {
		item := fstest.Item{Path: path.Join(dir, name), ModTime: modTime}
		_, obj := fstests.PutTestContents(ctx, t, f, &item, contents, true)
		require.NotNil(t, obj)
		return obj
	}
	billyObj := newFile("billy")
	billyTxn := billyObj.(*Object).xactID
	if f.useNoRename {
		require.True(t, billyTxn != "")
	} else {
		require.True(t, billyTxn == "")
	}

	billyChunkName := func(chunkNo int) string {
		return f.makeChunkName(billyObj.Remote(), chunkNo, "", billyTxn)
	}

	err := f.Mkdir(ctx, billyChunkName(1))
	assertOverlapError(err)

	_, err = f.Move(ctx, newFile("silly1"), billyChunkName(2))
	assert.Error(t, err)
	assert.True(t, err == fs.ErrorCantMove || (err != nil && strings.Contains(err.Error(), overlapMessage)))

	_, err = f.Copy(ctx, newFile("silly2"), billyChunkName(3))
	assert.Error(t, err)
	assert.True(t, err == fs.ErrorCantCopy || (err != nil && strings.Contains(err.Error(), overlapMessage)))

	// accessing chunks in strict mode is prohibited
	f.opt.FailHard = true
	billyChunk4Name := billyChunkName(4)
	_, err = f.base.NewObject(ctx, billyChunk4Name)
	require.NoError(t, err)
	_, err = f.NewObject(ctx, billyChunk4Name)
	assertOverlapError(err)

	f.opt.FailHard = false
	billyChunk4, err := f.NewObject(ctx, billyChunk4Name)
	assert.NoError(t, err)
	require.NotNil(t, billyChunk4)

	f.opt.FailHard = true
	_, err = f.Put(ctx, bytes.NewBufferString(contents), billyChunk4)
	assertOverlapError(err)

	// you can freely read chunks (if you have an object)
	r, err := billyChunk4.Open(ctx)
	assert.NoError(t, err)
	var chunkContents []byte
	assert.NotPanics(t, func() {
		chunkContents, err = ioutil.ReadAll(r)
		_ = r.Close()
	})
	assert.NoError(t, err)
	assert.NotEqual(t, contents, string(chunkContents))

	// but you can't change them
	err = billyChunk4.Update(ctx, bytes.NewBufferString(contents), newFile("silly3"))
	assertOverlapError(err)

	// Remove isn't special, you can't corrupt files even if you have an object
	err = billyChunk4.Remove(ctx)
	assertOverlapError(err)

	// recreate billy in case it was anyhow corrupted
	willyObj := newFile("willy")
	willyTxn := willyObj.(*Object).xactID
	willyChunkName := f.makeChunkName(willyObj.Remote(), 1, "", willyTxn)
	f.opt.FailHard = false
	willyChunk, err := f.NewObject(ctx, willyChunkName)
	f.opt.FailHard = true
	assert.NoError(t, err)
	require.NotNil(t, willyChunk)

	_, err = operations.Copy(ctx, f, willyChunk, willyChunkName, newFile("silly4"))
	assertOverlapError(err)

	// operations.Move will return error when chunker's Move refused
	// to corrupt target file, but reverts to copy/delete method
	// still trying to delete target chunk. Chunker must come to rescue.
	_, err = operations.Move(ctx, f, willyChunk, willyChunkName, newFile("silly5"))
	assertOverlapError(err)
	r, err = willyChunk.Open(ctx)
	assert.NoError(t, err)
	assert.NotPanics(t, func() {
		_, err = ioutil.ReadAll(r)
		_ = r.Close()
	})
	assert.NoError(t, err)
}

func testChunkNumberOverflow(t *testing.T, f *Fs) {
	if f.opt.ChunkSize > 50 {
		t.Skip("this test requires small chunks")
	}
	const dir = "wreaked"
	const wreakNumber = 10200300
	ctx := context.Background()
	saveOpt := f.opt
	defer func() {
		f.opt.FailHard = false
		_ = operations.Purge(ctx, f.base, dir)
		f.opt = saveOpt
	}()

	modTime := fstest.Time("2001-02-03T04:05:06.499999999Z")
	contents := random.String(100)

	newFile := func(f fs.Fs, name string) (obj fs.Object, filename string, txnID string) {
		filename = path.Join(dir, name)
		item := fstest.Item{Path: filename, ModTime: modTime}
		_, obj = fstests.PutTestContents(ctx, t, f, &item, contents, true)
		require.NotNil(t, obj)
		if chunkObj, isChunkObj := obj.(*Object); isChunkObj {
			txnID = chunkObj.xactID
		}
		return
	}

	f.opt.FailHard = false
	file, fileName, fileTxn := newFile(f, "wreaker")
	wreak, _, _ := newFile(f.base, f.makeChunkName("wreaker", wreakNumber, "", fileTxn))

	f.opt.FailHard = false
	fstest.CheckListingWithRoot(t, f, dir, nil, nil, f.Precision())
	_, err := f.NewObject(ctx, fileName)
	assert.Error(t, err)

	f.opt.FailHard = true
	_, err = f.List(ctx, dir)
	assert.Error(t, err)
	_, err = f.NewObject(ctx, fileName)
	assert.Error(t, err)

	f.opt.FailHard = false
	_ = wreak.Remove(ctx)
	_ = file.Remove(ctx)
}

func testMetadataInput(t *testing.T, f *Fs) {
	const minChunkForTest = 50
	if f.opt.ChunkSize < minChunkForTest {
		t.Skip("this test requires chunks that fit metadata")
	}

	const dir = "usermeta"
	ctx := context.Background()
	saveOpt := f.opt
	defer func() {
		f.opt.FailHard = false
		_ = operations.Purge(ctx, f.base, dir)
		f.opt = saveOpt
	}()
	f.opt.FailHard = false

	modTime := fstest.Time("2001-02-03T04:05:06.499999999Z")

	putFile := func(f fs.Fs, name, contents, message string, check bool) fs.Object {
		item := fstest.Item{Path: name, ModTime: modTime}
		_, obj := fstests.PutTestContents(ctx, t, f, &item, contents, check)
		assert.NotNil(t, obj, message)
		return obj
	}

	runSubtest := func(contents, name string) {
		description := fmt.Sprintf("file with %s metadata", name)
		filename := path.Join(dir, name)
		require.True(t, len(contents) > 2 && len(contents) < minChunkForTest, description+" test data is correct")

		part := putFile(f.base, f.makeChunkName(filename, 0, "", ""), "oops", "", true)
		_ = putFile(f, filename, contents, "upload "+description, false)

		obj, err := f.NewObject(ctx, filename)
		assert.NoError(t, err, "access "+description)
		assert.NotNil(t, obj)
		assert.Equal(t, int64(len(contents)), obj.Size(), "size "+description)

		o, ok := obj.(*Object)
		assert.NotNil(t, ok)
		if o != nil {
			assert.True(t, o.isComposite() && len(o.chunks) == 1, description+" is forced composite")
			o = nil
		}

		defer func() {
			_ = obj.Remove(ctx)
			_ = part.Remove(ctx)
		}()

		r, err := obj.Open(ctx)
		assert.NoError(t, err, "open "+description)
		assert.NotNil(t, r, "open stream of "+description)
		if err == nil && r != nil {
			data, err := ioutil.ReadAll(r)
			assert.NoError(t, err, "read all of "+description)
			assert.Equal(t, contents, string(data), description+" contents is ok")
			_ = r.Close()
		}
	}

	metaData, err := marshalSimpleJSON(ctx, 3, 1, "", "", "")
	require.NoError(t, err)
	todaysMeta := string(metaData)
	runSubtest(todaysMeta, "today")

	pastMeta := regexp.MustCompile(`"ver":[0-9]+`).ReplaceAllLiteralString(todaysMeta, `"ver":1`)
	pastMeta = regexp.MustCompile(`"size":[0-9]+`).ReplaceAllLiteralString(pastMeta, `"size":0`)
	runSubtest(pastMeta, "past")

	futureMeta := regexp.MustCompile(`"ver":[0-9]+`).ReplaceAllLiteralString(todaysMeta, `"ver":999`)
	futureMeta = regexp.MustCompile(`"nchunks":[0-9]+`).ReplaceAllLiteralString(futureMeta, `"nchunks":0,"x":"y"`)
	runSubtest(futureMeta, "future")
}

// Test that chunker refuses to change on objects with future/unknown metadata
func testFutureProof(t *testing.T, f *Fs) {
	if f.opt.MetaFormat == "none" {
		t.Skip("this test requires metadata support")
	}

	saveOpt := f.opt
	ctx := context.Background()
	f.opt.FailHard = true
	const dir = "future"
	const file = dir + "/test"
	defer func() {
		f.opt.FailHard = false
		_ = operations.Purge(ctx, f.base, dir)
		f.opt = saveOpt
	}()

	modTime := fstest.Time("2001-02-03T04:05:06.499999999Z")
	putPart := func(name string, part int, data, msg string) {
		if part > 0 {
			name = f.makeChunkName(name, part-1, "", "")
		}
		item := fstest.Item{Path: name, ModTime: modTime}
		_, obj := fstests.PutTestContents(ctx, t, f.base, &item, data, true)
		assert.NotNil(t, obj, msg)
	}

	// simulate chunked object from future
	meta := `{"ver":999,"nchunks":3,"size":9,"garbage":"litter","sha1":"0707f2970043f9f7c22029482db27733deaec029"}`
	putPart(file, 0, meta, "metaobject")
	putPart(file, 1, "abc", "chunk1")
	putPart(file, 2, "def", "chunk2")
	putPart(file, 3, "ghi", "chunk3")

	// List should succeed
	ls, err := f.List(ctx, dir)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ls))
	assert.Equal(t, int64(9), ls[0].Size())

	// NewObject should succeed
	obj, err := f.NewObject(ctx, file)
	assert.NoError(t, err)
	assert.Equal(t, file, obj.Remote())
	assert.Equal(t, int64(9), obj.Size())

	// Hash must fail
	_, err = obj.Hash(ctx, hash.SHA1)
	assert.Equal(t, ErrMetaUnknown, err)

	// Move must fail
	mobj, err := operations.Move(ctx, f, nil, file+"2", obj)
	assert.Nil(t, mobj)
	assert.Error(t, err)
	if err != nil {
		assert.Contains(t, err.Error(), "please upgrade rclone")
	}

	// Put must fail
	oi := object.NewStaticObjectInfo(file, modTime, 3, true, nil, nil)
	buf := bytes.NewBufferString("abc")
	_, err = f.Put(ctx, buf, oi)
	assert.Error(t, err)

	// Rcat must fail
	in := ioutil.NopCloser(bytes.NewBufferString("abc"))
	robj, err := operations.Rcat(ctx, f, file, in, modTime)
	assert.Nil(t, robj)
	assert.NotNil(t, err)
	if err != nil {
		assert.Contains(t, err.Error(), "please upgrade rclone")
	}
}

// The newer method of doing transactions without renaming should still be able to correctly process chunks that were created with renaming
// If you attempt to do the inverse, however, the data chunks will be ignored causing commands to perform incorrectly
func testBackwardsCompatibility(t *testing.T, f *Fs) {
	if !f.useMeta {
		t.Skip("Can't do norename transactions without metadata")
	}
	const dir = "backcomp"
	ctx := context.Background()
	saveOpt := f.opt
	saveUseNoRename := f.useNoRename
	defer func() {
		f.opt.FailHard = false
		_ = operations.Purge(ctx, f.base, dir)
		f.opt = saveOpt
		f.useNoRename = saveUseNoRename
	}()
	f.opt.ChunkSize = fs.SizeSuffix(10)

	modTime := fstest.Time("2001-02-03T04:05:06.499999999Z")
	contents := random.String(250)
	newFile := func(f fs.Fs, name string) (fs.Object, string) {
		filename := path.Join(dir, name)
		item := fstest.Item{Path: filename, ModTime: modTime}
		_, obj := fstests.PutTestContents(ctx, t, f, &item, contents, true)
		require.NotNil(t, obj)
		return obj, filename
	}

	f.opt.FailHard = false
	f.useNoRename = false
	file, fileName := newFile(f, "renamefile")

	f.opt.FailHard = false
	item := fstest.NewItem(fileName, contents, modTime)

	var items []fstest.Item
	items = append(items, item)

	f.useNoRename = true
	fstest.CheckListingWithRoot(t, f, dir, items, nil, f.Precision())
	_, err := f.NewObject(ctx, fileName)
	assert.NoError(t, err)

	f.opt.FailHard = true
	_, err = f.List(ctx, dir)
	assert.NoError(t, err)

	f.opt.FailHard = false
	_ = file.Remove(ctx)
}

func testChunkerServerSideMove(t *testing.T, f *Fs) {
	if !f.useMeta {
		t.Skip("Can't test norename transactions without metadata")
	}

	ctx := context.Background()
	const dir = "servermovetest"
	subRemote := fmt.Sprintf("%s:%s/%s", f.Name(), f.Root(), dir)

	subFs1, err := fs.NewFs(ctx, subRemote+"/subdir1")
	assert.NoError(t, err)
	fs1, isChunkerFs := subFs1.(*Fs)
	assert.True(t, isChunkerFs)
	fs1.useNoRename = false
	fs1.opt.ChunkSize = fs.SizeSuffix(3)

	subFs2, err := fs.NewFs(ctx, subRemote+"/subdir2")
	assert.NoError(t, err)
	fs2, isChunkerFs := subFs2.(*Fs)
	assert.True(t, isChunkerFs)
	fs2.useNoRename = true
	fs2.opt.ChunkSize = fs.SizeSuffix(3)

	modTime := fstest.Time("2001-02-03T04:05:06.499999999Z")
	item := fstest.Item{Path: "movefile", ModTime: modTime}
	contents := "abcdef"
	_, file := fstests.PutTestContents(ctx, t, fs1, &item, contents, true)

	dstOverwritten, _ := fs2.NewObject(ctx, "movefile")
	dstFile, err := operations.Move(ctx, fs2, dstOverwritten, "movefile", file)
	assert.NoError(t, err)
	assert.Equal(t, int64(len(contents)), dstFile.Size())

	r, err := dstFile.Open(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, r)
	data, err := ioutil.ReadAll(r)
	assert.NoError(t, err)
	assert.Equal(t, contents, string(data))
	_ = r.Close()
	_ = operations.Purge(ctx, f.base, dir)
}

// InternalTest dispatches all internal tests
func (f *Fs) InternalTest(t *testing.T) {
	t.Run("PutLarge", func(t *testing.T) {
		if *UploadKilobytes <= 0 {
			t.Skip("-upload-kilobytes is not set")
		}
		testPutLarge(t, f, *UploadKilobytes)
	})
	t.Run("ChunkNameFormat", func(t *testing.T) {
		testChunkNameFormat(t, f)
	})
	t.Run("SmallFileInternals", func(t *testing.T) {
		testSmallFileInternals(t, f)
	})
	t.Run("PreventCorruption", func(t *testing.T) {
		testPreventCorruption(t, f)
	})
	t.Run("ChunkNumberOverflow", func(t *testing.T) {
		testChunkNumberOverflow(t, f)
	})
	t.Run("MetadataInput", func(t *testing.T) {
		testMetadataInput(t, f)
	})
	t.Run("FutureProof", func(t *testing.T) {
		testFutureProof(t, f)
	})
	t.Run("BackwardsCompatibility", func(t *testing.T) {
		testBackwardsCompatibility(t, f)
	})
	t.Run("ChunkerServerSideMove", func(t *testing.T) {
		testChunkerServerSideMove(t, f)
	})
}

var _ fstests.InternalTester = (*Fs)(nil)
