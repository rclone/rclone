package chunker

import (
	"context"
	"flag"
	"fmt"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/stretchr/testify/assert"
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

	assertMakeName := func(wantChunkName, mainName string, chunkNo int, ctrlType string, xactNo int64) {
		gotChunkName := f.makeChunkName(mainName, chunkNo, ctrlType, xactNo)
		assert.Equal(t, wantChunkName, gotChunkName)
	}

	assertMakeNamePanics := func(mainName string, chunkNo int, ctrlType string, xactNo int64) {
		assert.Panics(t, func() {
			_ = f.makeChunkName(mainName, chunkNo, ctrlType, xactNo)
		}, "makeChunkName(%q,%d,%q,%d) should panic", mainName, chunkNo, ctrlType, xactNo)
	}

	assertParseName := func(fileName, wantMainName string, wantChunkNo int, wantCtrlType string, wantXactNo int64) {
		gotMainName, gotChunkNo, gotCtrlType, gotXactNo := f.parseChunkName(fileName)
		assert.Equal(t, wantMainName, gotMainName)
		assert.Equal(t, wantChunkNo, gotChunkNo)
		assert.Equal(t, wantCtrlType, gotCtrlType)
		assert.Equal(t, wantXactNo, gotXactNo)
	}

	const newFormatSupported = false // support for patterns not starting with base name (*)

	// valid formats
	assertFormat(`*.rclone_chunk.###`, `%s.rclone_chunk.%03d`, `%s.rclone_chunk._%s`, `^(.+?)\.rclone_chunk\.(?:([0-9]{3,})|_([a-z]{3,9}))(?:\.\.tmp_([0-9]{10,19}))?$`)
	assertFormat(`*.rclone_chunk.#`, `%s.rclone_chunk.%d`, `%s.rclone_chunk._%s`, `^(.+?)\.rclone_chunk\.(?:([0-9]+)|_([a-z]{3,9}))(?:\.\.tmp_([0-9]{10,19}))?$`)
	assertFormat(`*_chunk_#####`, `%s_chunk_%05d`, `%s_chunk__%s`, `^(.+?)_chunk_(?:([0-9]{5,})|_([a-z]{3,9}))(?:\.\.tmp_([0-9]{10,19}))?$`)
	assertFormat(`*-chunk-#`, `%s-chunk-%d`, `%s-chunk-_%s`, `^(.+?)-chunk-(?:([0-9]+)|_([a-z]{3,9}))(?:\.\.tmp_([0-9]{10,19}))?$`)
	assertFormat(`*-chunk-#-%^$()[]{}.+-!?:\`, `%s-chunk-%d-%%^$()[]{}.+-!?:\`, `%s-chunk-_%s-%%^$()[]{}.+-!?:\`, `^(.+?)-chunk-(?:([0-9]+)|_([a-z]{3,9}))-%\^\$\(\)\[\]\{\}\.\+-!\?:\\(?:\.\.tmp_([0-9]{10,19}))?$`)
	if newFormatSupported {
		assertFormat(`_*-chunk-##,`, `_%s-chunk-%02d,`, `_%s-chunk-_%s,`, `^_(.+?)-chunk-(?:([0-9]{2,})|_([a-z]{3,9})),(?:\.\.tmp_([0-9]{10,19}))?$`)
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
		assertFormat(`part_*_#`, `part_%s_%d`, `part_%s__%s`, `^part_(.+?)_(?:([0-9]+)|_([a-z]{3,9}))(?:\.\.tmp_([0-9]{10,19}))?$`)
		f.opt.StartFrom = 1

		assertMakeName(`part_fish_1`, "fish", 0, "", -1)
		assertParseName(`part_fish_43`, "fish", 42, "", -1)
		assertMakeName(`part_fish_3..tmp_0000000004`, "fish", 2, "", 4)
		assertParseName(`part_fish_4..tmp_0000000005`, "fish", 3, "", 5)
		assertMakeName(`part_fish__locks`, "fish", -2, "locks", -3)
		assertParseName(`part_fish__locks`, "fish", -1, "locks", -1)
		assertMakeName(`part_fish__blockinfo..tmp_1234567890123456789`, "fish", -3, "blockinfo", 1234567890123456789)
		assertParseName(`part_fish__blockinfo..tmp_1234567890123456789`, "fish", -1, "blockinfo", 1234567890123456789)
	}

	// prepare format for long tests
	assertFormat(`*.chunk.###`, `%s.chunk.%03d`, `%s.chunk._%s`, `^(.+?)\.chunk\.(?:([0-9]{3,})|_([a-z]{3,9}))(?:\.\.tmp_([0-9]{10,19}))?$`)
	f.opt.StartFrom = 2

	// valid data chunks
	assertMakeName(`fish.chunk.003`, "fish", 1, "", -1)
	assertMakeName(`fish.chunk.011..tmp_0000054321`, "fish", 9, "", 54321)
	assertMakeName(`fish.chunk.011..tmp_1234567890`, "fish", 9, "", 1234567890)
	assertMakeName(`fish.chunk.1916..tmp_123456789012345`, "fish", 1914, "", 123456789012345)

	assertParseName(`fish.chunk.003`, "fish", 1, "", -1)
	assertParseName(`fish.chunk.004..tmp_0000000021`, "fish", 2, "", 21)
	assertParseName(`fish.chunk.021`, "fish", 19, "", -1)
	assertParseName(`fish.chunk.323..tmp_1234567890123456789`, "fish", 321, "", 1234567890123456789)

	// parsing invalid data chunk names
	assertParseName(`fish.chunk.3`, "", -1, "", -1)
	assertParseName(`fish.chunk.001`, "", -1, "", -1)
	assertParseName(`fish.chunk.21`, "", -1, "", -1)
	assertParseName(`fish.chunk.-21`, "", -1, "", -1)

	assertParseName(`fish.chunk.004.tmp_0000000021`, "", -1, "", -1)
	assertParseName(`fish.chunk.003..tmp_123456789`, "", -1, "", -1)
	assertParseName(`fish.chunk.003..tmp_012345678901234567890123456789`, "", -1, "", -1)
	assertParseName(`fish.chunk.003..tmp_-1`, "", -1, "", -1)

	// valid control chunks
	assertMakeName(`fish.chunk._info`, "fish", -1, "info", -1)
	assertMakeName(`fish.chunk._locks`, "fish", -2, "locks", -1)
	assertMakeName(`fish.chunk._blockinfo`, "fish", -3, "blockinfo", -1)

	assertParseName(`fish.chunk._info`, "fish", -1, "info", -1)
	assertParseName(`fish.chunk._locks`, "fish", -1, "locks", -1)
	assertParseName(`fish.chunk._blockinfo`, "fish", -1, "blockinfo", -1)

	// valid temporary control chunks
	assertMakeName(`fish.chunk._info..tmp_0000000021`, "fish", -1, "info", 21)
	assertMakeName(`fish.chunk._locks..tmp_0000054321`, "fish", -2, "locks", 54321)
	assertMakeName(`fish.chunk._uploads..tmp_0000000000`, "fish", -3, "uploads", 0)
	assertMakeName(`fish.chunk._blockinfo..tmp_1234567890123456789`, "fish", -4, "blockinfo", 1234567890123456789)

	assertParseName(`fish.chunk._info..tmp_0000000021`, "fish", -1, "info", 21)
	assertParseName(`fish.chunk._locks..tmp_0000054321`, "fish", -1, "locks", 54321)
	assertParseName(`fish.chunk._uploads..tmp_0000000000`, "fish", -1, "uploads", 0)
	assertParseName(`fish.chunk._blockinfo..tmp_1234567890123456789`, "fish", -1, "blockinfo", 1234567890123456789)

	// parsing invalid control chunk names
	assertParseName(`fish.chunk.info`, "", -1, "", -1)
	assertParseName(`fish.chunk.locks`, "", -1, "", -1)
	assertParseName(`fish.chunk.uploads`, "", -1, "", -1)
	assertParseName(`fish.chunk.blockinfo`, "", -1, "", -1)

	assertParseName(`fish.chunk._os`, "", -1, "", -1)
	assertParseName(`fish.chunk._futuredata`, "", -1, "", -1)
	assertParseName(`fish.chunk._me_ta`, "", -1, "", -1)
	assertParseName(`fish.chunk._in-fo`, "", -1, "", -1)
	assertParseName(`fish.chunk._.bin`, "", -1, "", -1)

	assertParseName(`fish.chunk._locks..tmp_123456789`, "", -1, "", -1)
	assertParseName(`fish.chunk._meta..tmp_-1`, "", -1, "", -1)
	assertParseName(`fish.chunk._blockinfo..tmp_012345678901234567890123456789`, "", -1, "", -1)

	// short control chunk names: 3 letters ok, 1-2 letters not allowed
	assertMakeName(`fish.chunk._ext`, "fish", -1, "ext", -1)
	assertMakeName(`fish.chunk._ext..tmp_0000000021`, "fish", -1, "ext", 21)
	assertParseName(`fish.chunk._int`, "fish", -1, "int", -1)
	assertParseName(`fish.chunk._int..tmp_0000000021`, "fish", -1, "int", 21)
	assertMakeNamePanics("fish", -1, "in", -1)
	assertMakeNamePanics("fish", -1, "up", 4)
	assertMakeNamePanics("fish", -1, "x", -1)
	assertMakeNamePanics("fish", -1, "c", 4)

	// base file name can sometimes look like a valid chunk name
	assertParseName(`fish.chunk.003.chunk.004`, "fish.chunk.003", 2, "", -1)
	assertParseName(`fish.chunk.003.chunk.005..tmp_0000000021`, "fish.chunk.003", 3, "", 21)
	assertParseName(`fish.chunk.003.chunk._info`, "fish.chunk.003", -1, "info", -1)
	assertParseName(`fish.chunk.003.chunk._blockinfo..tmp_1234567890123456789`, "fish.chunk.003", -1, "blockinfo", 1234567890123456789)
	assertParseName(`fish.chunk.003.chunk._Meta`, "", -1, "", -1)
	assertParseName(`fish.chunk.003.chunk._x..tmp_0000054321`, "", -1, "", -1)

	assertParseName(`fish.chunk.004..tmp_0000000021.chunk.004`, "fish.chunk.004..tmp_0000000021", 2, "", -1)
	assertParseName(`fish.chunk.004..tmp_0000000021.chunk.005..tmp_0000000021`, "fish.chunk.004..tmp_0000000021", 3, "", 21)
	assertParseName(`fish.chunk.004..tmp_0000000021.chunk._info`, "fish.chunk.004..tmp_0000000021", -1, "info", -1)
	assertParseName(`fish.chunk.004..tmp_0000000021.chunk._blockinfo..tmp_1234567890123456789`, "fish.chunk.004..tmp_0000000021", -1, "blockinfo", 1234567890123456789)
	assertParseName(`fish.chunk.004..tmp_0000000021.chunk._Meta`, "", -1, "", -1)
	assertParseName(`fish.chunk.004..tmp_0000000021.chunk._x..tmp_0000054321`, "", -1, "", -1)

	assertParseName(`fish.chunk._info.chunk.004`, "fish.chunk._info", 2, "", -1)
	assertParseName(`fish.chunk._info.chunk.005..tmp_0000000021`, "fish.chunk._info", 3, "", 21)
	assertParseName(`fish.chunk._info.chunk._info`, "fish.chunk._info", -1, "info", -1)
	assertParseName(`fish.chunk._info.chunk._blockinfo..tmp_1234567890123456789`, "fish.chunk._info", -1, "blockinfo", 1234567890123456789)
	assertParseName(`fish.chunk._info.chunk._info.chunk._Meta`, "", -1, "", -1)
	assertParseName(`fish.chunk._info.chunk._info.chunk._x..tmp_0000054321`, "", -1, "", -1)

	assertParseName(`fish.chunk._blockinfo..tmp_1234567890123456789.chunk.004`, "fish.chunk._blockinfo..tmp_1234567890123456789", 2, "", -1)
	assertParseName(`fish.chunk._blockinfo..tmp_1234567890123456789.chunk.005..tmp_0000000021`, "fish.chunk._blockinfo..tmp_1234567890123456789", 3, "", 21)
	assertParseName(`fish.chunk._blockinfo..tmp_1234567890123456789.chunk._info`, "fish.chunk._blockinfo..tmp_1234567890123456789", -1, "info", -1)
	assertParseName(`fish.chunk._blockinfo..tmp_1234567890123456789.chunk._blockinfo..tmp_1234567890123456789`, "fish.chunk._blockinfo..tmp_1234567890123456789", -1, "blockinfo", 1234567890123456789)
	assertParseName(`fish.chunk._blockinfo..tmp_1234567890123456789.chunk._info.chunk._Meta`, "", -1, "", -1)
	assertParseName(`fish.chunk._blockinfo..tmp_1234567890123456789.chunk._info.chunk._x..tmp_0000054321`, "", -1, "", -1)

	// attempts to make invalid chunk names
	assertMakeNamePanics("fish", -1, "", -1)           // neither data nor control
	assertMakeNamePanics("fish", 0, "info", -1)        // both data and control
	assertMakeNamePanics("fish", -1, "futuredata", -1) // control type too long
	assertMakeNamePanics("fish", -1, "123", -1)        // digits not allowed
	assertMakeNamePanics("fish", -1, "Meta", -1)       // only lower case letters allowed
	assertMakeNamePanics("fish", -1, "in-fo", -1)      // punctuation not allowed
	assertMakeNamePanics("fish", -1, "_info", -1)
	assertMakeNamePanics("fish", -1, "info_", -1)
	assertMakeNamePanics("fish", -2, ".bind", -3)
	assertMakeNamePanics("fish", -2, "bind.", -3)

	assertMakeNamePanics("fish", -1, "", 1)            // neither data nor control
	assertMakeNamePanics("fish", 0, "info", 12)        // both data and control
	assertMakeNamePanics("fish", -1, "futuredata", 45) // control type too long
	assertMakeNamePanics("fish", -1, "123", 123)       // digits not allowed
	assertMakeNamePanics("fish", -1, "Meta", 456)      // only lower case letters allowed
	assertMakeNamePanics("fish", -1, "in-fo", 321)     // punctuation not allowed
	assertMakeNamePanics("fish", -1, "_info", 15678)
	assertMakeNamePanics("fish", -1, "info_", 999)
	assertMakeNamePanics("fish", -2, ".bind", 0)
	assertMakeNamePanics("fish", -2, "bind.", 0)
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
}

var _ fstests.InternalTester = (*Fs)(nil)
