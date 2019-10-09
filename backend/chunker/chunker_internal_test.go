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

	billyChunkName := func(chunkNo int) string {
		return f.makeChunkName(billyObj.Remote(), chunkNo, "", -1)
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
	billyChunk4, err := f.NewObject(ctx, billyChunk4Name)
	assertOverlapError(err)

	f.opt.FailHard = false
	billyChunk4, err = f.NewObject(ctx, billyChunk4Name)
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
	willyChunkName := f.makeChunkName(willyObj.Remote(), 1, "", -1)
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

	newFile := func(f fs.Fs, name string) (fs.Object, string) {
		filename := path.Join(dir, name)
		item := fstest.Item{Path: filename, ModTime: modTime}
		_, obj := fstests.PutTestContents(ctx, t, f, &item, contents, true)
		require.NotNil(t, obj)
		return obj, filename
	}

	f.opt.FailHard = false
	file, fileName := newFile(f, "wreaker")
	wreak, _ := newFile(f.base, f.makeChunkName("wreaker", wreakNumber, "", -1))

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

		part := putFile(f.base, f.makeChunkName(filename, 0, "", -1), "oops", "", true)
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

	metaData, err := marshalSimpleJSON(ctx, 3, 1, "", "")
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
}

var _ fstests.InternalTester = (*Fs)(nil)
