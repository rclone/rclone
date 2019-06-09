package chunker

import (
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
func (f *Fs) InternalTestPutLarge(t *testing.T, kilobytes int) {
	t.Run(fmt.Sprintf("PutLarge%dk", kilobytes), func(t *testing.T) {
		fstests.TestPutLarge(t, f, &fstest.Item{
			ModTime: fstest.Time("2001-02-03T04:05:06.499999999Z"),
			Path:    fmt.Sprintf("chunker-upload-%dk", kilobytes),
			Size:    int64(kilobytes) * int64(fs.KibiByte),
		})
	})
}

func (f *Fs) InternalTestChunkNameFormat(t *testing.T) {
	savedNameFormat := f.opt.NameFormat
	savedStartFrom := f.opt.StartFrom
	defer func() {
		// restore original settings
		_ = f.parseNameFormat(savedNameFormat)
		f.opt.StartFrom = savedStartFrom
	}()
	var err error

	err = f.parseNameFormat("*.rclone_chunk.###")
	assert.NoError(t, err)
	assert.Equal(t, `%s.rclone_chunk.%03d`, f.nameFormat)
	assert.Equal(t, `^(.+)\.rclone_chunk\.([0-9]{3,})$`, f.nameRegexp.String())

	err = f.parseNameFormat("*.rclone_chunk.#")
	assert.NoError(t, err)
	assert.Equal(t, `%s.rclone_chunk.%d`, f.nameFormat)
	assert.Equal(t, `^(.+)\.rclone_chunk\.([0-9]+)$`, f.nameRegexp.String())

	err = f.parseNameFormat("*_chunk_#####")
	assert.NoError(t, err)
	assert.Equal(t, `%s_chunk_%05d`, f.nameFormat)
	assert.Equal(t, `^(.+)_chunk_([0-9]{5,})$`, f.nameRegexp.String())

	err = f.parseNameFormat("*-chunk-#")
	assert.NoError(t, err)
	assert.Equal(t, `%s-chunk-%d`, f.nameFormat)
	assert.Equal(t, `^(.+)-chunk-([0-9]+)$`, f.nameRegexp.String())

	err = f.parseNameFormat("_*-chunk-##,")
	assert.NoError(t, err)
	assert.Equal(t, `_%s-chunk-%02d,`, f.nameFormat)
	assert.Equal(t, `^_(.+)-chunk-([0-9]{2,}),$`, f.nameRegexp.String())

	err = f.parseNameFormat(`*-chunk-#-%^$()[]{}.+-!?:\/`)
	assert.NoError(t, err)
	assert.Equal(t, `%s-chunk-%d-%%^$()[]{}.+-!?:\/`, f.nameFormat)
	assert.Equal(t, `^(.+)-chunk-([0-9]+)-%\^\$\(\)\[\]\{\}\.\+-!\?:\\/$`, f.nameRegexp.String())

	err = f.parseNameFormat("chunk-#")
	assert.Error(t, err)

	err = f.parseNameFormat("*-chunk")
	assert.Error(t, err)

	err = f.parseNameFormat("*-*-chunk-#")
	assert.Error(t, err)

	err = f.parseNameFormat("*-chunk-#-#")
	assert.Error(t, err)

	err = f.parseNameFormat("#-chunk-*")
	assert.Error(t, err)

	err = f.parseNameFormat("*#")
	assert.NoError(t, err)

	err = f.parseNameFormat("**#")
	assert.Error(t, err)
	err = f.parseNameFormat("#*")
	assert.Error(t, err)
	err = f.parseNameFormat("")
	assert.Error(t, err)
	err = f.parseNameFormat("-")
	assert.Error(t, err)

	f.opt.StartFrom = 2
	err = f.parseNameFormat("*.chunk.###")
	assert.NoError(t, err)
	assert.Equal(t, `%s.chunk.%03d`, f.nameFormat)
	assert.Equal(t, `^(.+)\.chunk\.([0-9]{3,})$`, f.nameRegexp.String())

	assert.Equal(t, "fish.chunk.003", f.makeChunkName("fish", 1, -1))
	assert.Equal(t, "fish.chunk.011..tmp_0000054321", f.makeChunkName("fish", 9, 54321))
	assert.Equal(t, "fish.chunk.011..tmp_1234567890", f.makeChunkName("fish", 9, 1234567890))
	assert.Equal(t, "fish.chunk.1916..tmp_123456789012345", f.makeChunkName("fish", 1914, 123456789012345))

	name, chunkNo, tempNo := f.parseChunkName("fish.chunk.003")
	assert.True(t, name == "fish" && chunkNo == 1 && tempNo == -1)
	name, chunkNo, tempNo = f.parseChunkName("fish.chunk.004..tmp_0000000021")
	assert.True(t, name == "fish" && chunkNo == 2 && tempNo == 21)
	name, chunkNo, tempNo = f.parseChunkName("fish.chunk.021")
	assert.True(t, name == "fish" && chunkNo == 19 && tempNo == -1)
	name, chunkNo, tempNo = f.parseChunkName("fish.chunk.323..tmp_1234567890123456789")
	assert.True(t, name == "fish" && chunkNo == 321 && tempNo == 1234567890123456789)
	name, chunkNo, tempNo = f.parseChunkName("fish.chunk.3")
	assert.True(t, name == "" && chunkNo == -1 && tempNo == -1)
	name, chunkNo, tempNo = f.parseChunkName("fish.chunk.001")
	assert.True(t, name == "" && chunkNo == -1 && tempNo == -1)
	name, chunkNo, tempNo = f.parseChunkName("fish.chunk.21")
	assert.True(t, name == "" && chunkNo == -1 && tempNo == -1)
	name, chunkNo, tempNo = f.parseChunkName("fish.chunk.-21")
	assert.True(t, name == "" && chunkNo == -1 && tempNo == -1)
	name, chunkNo, tempNo = f.parseChunkName("fish.chunk.004.tmp_0000000021")
	assert.True(t, name == "" && chunkNo == -1 && tempNo == -1)
	name, chunkNo, tempNo = f.parseChunkName("fish.chunk.003..tmp_123456789")
	assert.True(t, name == "" && chunkNo == -1 && tempNo == -1)
	name, chunkNo, tempNo = f.parseChunkName("fish.chunk.003..tmp_012345678901234567890123456789")
	assert.True(t, name == "" && chunkNo == -1 && tempNo == -1)
	name, chunkNo, tempNo = f.parseChunkName("fish.chunk.003..tmp_-1")
	assert.True(t, name == "" && chunkNo == -1 && tempNo == -1)
}

func (f *Fs) InternalTest(t *testing.T) {
	t.Run("PutLarge", func(t *testing.T) {
		if *UploadKilobytes <= 0 {
			t.Skip("-upload-kilobytes is not set")
		}
		f.InternalTestPutLarge(t, *UploadKilobytes)
	})
	t.Run("ChunkNameFormat", func(t *testing.T) {
		f.InternalTestChunkNameFormat(t)
	})
}

var _ fstests.InternalTester = (*Fs)(nil)
