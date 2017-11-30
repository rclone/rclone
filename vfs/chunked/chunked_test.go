package chunked

import (
	"context"
	"errors"
	"fmt"
	"io"
	stdfs "io/fs"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/all" // import all the file systems
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain drives the tests
func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

func TestChunkFileName(t *testing.T) {
	cf := &File{
		dir: "path/to/dir",
	}

	for _, test := range []struct {
		bits  uint
		off   int64
		want  string
		panic bool
	}{
		{
			8,
			0,
			"path/to/dir/00/00/00/00/00/00/00000000000000.bin",
			false,
		},
		{
			8,
			0x123456789ABCDE00,
			"path/to/dir/12/34/56/78/9A/BC/123456789ABCDE.bin",
			false,
		},
		{
			8,
			0x123456789ABCDE80,
			"",
			true,
		},
		{
			12,
			0,
			"path/to/dir/00/00/00/00/00/00/00000000000000.bin",
			false,
		},
		{
			12,
			0x123456789ABCD000,
			"path/to/dir/01/23/45/67/89/AB/0123456789ABCD.bin",
			false,
		},
		{
			15,
			0,
			"path/to/dir/00/00/00/00/00/00/00000000000000.bin",
			false,
		},
		{
			15,
			0x123456789ABCC000,
			"",
			true,
		},
		{
			15,
			0x123456789ABC0000,
			"path/to/dir/00/24/68/AC/F1/35/002468ACF13578.bin",
			false,
		},
		{
			16,
			0,
			"path/to/dir/00/00/00/00/00/000000000000.bin",
			false,
		},
		{
			16,
			0x123456789ABC8000,
			"",
			true,
		},
		{
			16,
			0x123456789ABC0000,
			"path/to/dir/12/34/56/78/9A/123456789ABC.bin",
			false,
		},
		{
			20,
			0,
			"path/to/dir/00/00/00/00/00/000000000000.bin",
			false,
		},
		{
			23,
			0,
			"path/to/dir/00/00/00/00/00/000000000000.bin",
			false,
		},
		{
			24,
			0,
			"path/to/dir/00/00/00/00/0000000000.bin",
			false,
		},
		{
			24,
			0x7EFDFCFBFA000000,
			"path/to/dir/7E/FD/FC/FB/7EFDFCFBFA.bin",
			false,
		},
		{
			28,
			0x7EFDFCFBF0000000,
			"path/to/dir/07/EF/DF/CF/07EFDFCFBF.bin",
			false,
		},
	} {
		cf.info.ChunkBits = test.bits
		cf._updateChunkBits()
		what := fmt.Sprintf("bits=%d, off=0x%X, panic=%v", test.bits, test.off, test.panic)
		if !test.panic {
			got := cf.makeChunkFileName(test.off)
			assert.Equal(t, test.want, got, what)
		} else {
			assert.Panics(t, func() {
				cf.makeChunkFileName(test.off)
			}, what)
		}
	}
}

// check that the object exists and has the contents
func checkObject(t *testing.T, f fs.Fs, remote string, want string) {
	o, err := f.NewObject(context.TODO(), remote)
	require.NoError(t, err)
	dst := object.NewMemoryObject(remote, time.Now(), nil)
	_, err = operations.Copy(context.TODO(), object.MemoryFs, dst, "", o)
	require.NoError(t, err)
	assert.Equal(t, want, string(dst.Content()))
}

// Constants uses in the tests
const (
	writeBackDelay      = 100 * time.Millisecond // A short writeback delay for testing
	waitForWritersDelay = 30 * time.Second       // time to wait for existing writers
)

// Clean up a test VFS
func cleanupVFS(t *testing.T, vfs *vfs.VFS) {
	vfs.WaitForWriters(waitForWritersDelay)
	err := vfs.CleanUp()
	require.NoError(t, err)
	vfs.Shutdown()
}

// Create a new VFS
func newTestVFSOpt(t *testing.T, opt *vfscommon.Options) (r *fstest.Run, VFS *vfs.VFS) {
	r = fstest.NewRun(t)
	VFS = vfs.New(r.Fremote, opt)
	t.Cleanup(func() {
		cleanupVFS(t, VFS)
	})
	return r, VFS
}

func TestNew(t *testing.T) {
	_, VFS := newTestVFSOpt(t, nil)

	// check default open
	cf := New(VFS, "")
	assert.Equal(t, 0, cf.opens)
	err := cf.Open(true, defaultChunkBits)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), cf.info.Size)
	assert.Equal(t, uint(defaultChunkBits), cf.info.ChunkBits)
	assert.Equal(t, 0x10000, cf.chunkSize)
	assert.Equal(t, int64(0xFFFF), cf.chunkMask)
	assert.Equal(t, ^int64(0xFFFF), cf.mask)
	assert.Equal(t, 1, cf.opens)

	// check the close
	err = cf.Close()
	assert.NoError(t, err)
	assert.Equal(t, 0, cf.opens)

	// check the double close
	err = cf.Close()
	assert.Error(t, err)
	assert.Equal(t, 0, cf.opens)

	// check that the info got written
	checkObject(t, VFS.Fs(), cf.infoRemote, `{"Version":1,"Comment":"rclone chunked file","Size":0,"ChunkBits":16,"ChunkSize":65536}`)

	// change the info
	cf.info.Size = 100
	cf.info.ChunkBits = 20
	cf._updateChunkBits()
	err = cf._writeInfo()
	assert.NoError(t, err)

	// read it back in
	cf = New(VFS, "")
	err = cf.Open(false, 0)
	assert.NoError(t, err)
	assert.Equal(t, int64(100), cf.info.Size)
	assert.Equal(t, uint(20), cf.info.ChunkBits)
	assert.Equal(t, 0x100000, cf.chunkSize)
	assert.Equal(t, int64(0xFFFFF), cf.chunkMask)
	assert.Equal(t, ^int64(0xFFFFF), cf.mask)

	// check opens

	// test limits for readInfo
	for _, test := range []struct {
		info  string
		error string
	}{
		{
			`{"Version":1,"Comment":"rclone chunked file","Size":0,"ChunkBits":16,"ChunkSize":65536`,
			"failed to decode chunk info file",
		},
		{
			`{"Version":99,"Comment":"rclone chunked file","Size":0,"ChunkBits":16,"ChunkSize":65536}`,
			"don't understand version 99 info files",
		},
		{
			`{"Version":1,"Comment":"rclone chunked file","Size":0,"ChunkBits":1,"ChunkSize":65536}`,
			"chunk bits 1 too small",
		},
		{
			`{"Version":1,"Comment":"rclone chunked file","Size":0,"ChunkBits":99,"ChunkSize":65536}`,
			"chunk bits 99 too large",
		},
	} {
		require.NoError(t, VFS.WriteFile(cf.infoRemote, []byte(test.info), 0600))
		err = cf._readInfo()
		require.Error(t, err)
		assert.Contains(t, err.Error(), test.error)
	}
}

func newTestFile(t *testing.T) (*vfs.VFS, *File) {
	opt := vfscommon.Opt
	opt.CacheMode = vfscommon.CacheModeFull
	opt.WriteBack = 0 // make writeback synchronous
	_, VFS := newTestVFSOpt(t, &opt)

	cf := New(VFS, "")
	err := cf.Open(true, 4)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, cf.Close())
	})

	return VFS, cf
}

func TestReadWriteChunk(t *testing.T) {
	VFS, cf := newTestFile(t)

	const (
		off        = 0x123456789ABCDEF0
		wantRemote = "01/23/45/67/89/AB/CD/0123456789ABCDEF.bin"
	)

	// pretend the file is big
	require.NoError(t, cf.Truncate(2*off))

	// check reading non existent chunk gives 0
	var zero = make([]byte, 16)
	var b = make([]byte, 16)
	n, err := cf.ReadAt(b, off)
	require.NoError(t, err)
	assert.Equal(t, 16, n)
	assert.Equal(t, zero, b)

	// create a new chunk and write some data
	n, err = cf.WriteAt([]byte("0123456789abcdef"), off)
	require.NoError(t, err)
	assert.Equal(t, 16, n)

	// check the chunk on disk
	checkObject(t, VFS.Fs(), wantRemote, "0123456789abcdef")

	// read the chunk off disk and check it
	n, err = cf.ReadAt(b, off)
	require.NoError(t, err)
	assert.Equal(t, 16, n)
	assert.Equal(t, "0123456789abcdef", string(b))
}

func TestZeroBytes(t *testing.T) {
	b := []byte{1, 2, 3, 4}
	zeroBytes(b, 2)
	assert.Equal(t, []byte{0, 0, 3, 4}, b)

	b = []byte{1, 2, 3, 4}
	zeroBytes(b, 17)
	assert.Equal(t, []byte{0, 0, 0, 0}, b)
}

func TestReadAt(t *testing.T) {
	_, cf := newTestFile(t)

	// make a new chunk and write it to disk as chunk 1
	zero := make([]byte, 16)
	middle := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	n, err := cf.WriteAt(middle, 16)
	require.NoError(t, err)
	assert.Equal(t, 16, n)

	// set the size to 0
	cf.info.Size = 0

	// check reading
	b := make([]byte, 40)
	n, err = cf.ReadAt(b, 0)
	assert.Equal(t, 0, n)
	assert.Equal(t, io.EOF, err)

	// set the size to 38
	cf.info.Size = 38

	// read to end
	n, err = cf.ReadAt(b, 0)
	assert.Equal(t, 38, n)
	assert.Equal(t, io.EOF, err)
	expected := append([]byte(nil), zero...)
	expected = append(expected, middle...)
	expected = append(expected, zero[:6]...)
	assert.Equal(t, expected, b[:n])

	// read not to end
	b = make([]byte, 16)
	n, err = cf.ReadAt(b, 10)
	assert.Equal(t, 16, n)
	assert.NoError(t, err)
	expected = append([]byte(nil), zero[10:]...)
	expected = append(expected, middle[:10]...)
	assert.Equal(t, expected, b[:n])

	// read at end
	n, err = cf.ReadAt(b, 38)
	assert.Equal(t, 0, n)
	assert.Equal(t, io.EOF, err)

	// read past end
	n, err = cf.ReadAt(b, 99)
	assert.Equal(t, 0, n)
	assert.Equal(t, io.EOF, err)
}

func TestWriteAt(t *testing.T) {
	VFS, cf := newTestFile(t)
	f := VFS.Fs()

	// Make test buffer
	b := []byte{}
	for i := byte(0); i < 30; i++ {
		b = append(b, '0'+i)
	}

	t.Run("SizeZero", func(t *testing.T) {
		assert.Equal(t, int64(0), cf.Size())
	})

	const (
		wantRemote1 = "00/00/00/00/00/00/00/0000000000000000.bin"
		wantRemote2 = "00/00/00/00/00/00/00/0000000000000001.bin"
		wantRemote3 = "00/00/00/00/00/00/00/0000000000000002.bin"
	)

	t.Run("Extended", func(t *testing.T) {
		// write it and check file is extended
		n, err := cf.WriteAt(b, 8)
		assert.Equal(t, 30, n)
		assert.NoError(t, err)
		assert.Equal(t, int64(38), cf.info.Size)

		// flush the parts to disk
		require.NoError(t, cf.Sync())

		// check the parts on disk
		checkObject(t, f, wantRemote1, "\x00\x00\x00\x00\x00\x00\x00\x0001234567")
		checkObject(t, f, wantRemote2, "89:;<=>?@ABCDEFG")
		checkObject(t, f, wantRemote3, "HIJKLM\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00")
	})

	t.Run("Size", func(t *testing.T) {
		assert.Equal(t, int64(38), cf.Size())
	})

	t.Run("Overwrite", func(t *testing.T) {
		// overwrite a part
		n, err := cf.WriteAt([]byte("abcdefgh"), 12)
		assert.Equal(t, 8, n)
		assert.NoError(t, err)
		assert.Equal(t, int64(38), cf.info.Size)

		// flush the parts to disk
		require.NoError(t, cf.Sync())

		// check the parts on disk
		checkObject(t, f, wantRemote1, "\x00\x00\x00\x00\x00\x00\x00\x000123abcd")
		checkObject(t, f, wantRemote2, "efgh<=>?@ABCDEFG")
		checkObject(t, f, wantRemote3, "HIJKLM\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00")
	})

	t.Run("Remove", func(t *testing.T) {
		require.Error(t, cf.Remove())
		require.NoError(t, cf.Close())

		// Check files are there
		fis, err := VFS.ReadDir(cf.dir)
		require.NoError(t, err)
		assert.True(t, len(fis) > 0)

		// Remove the file
		require.NoError(t, cf.Remove())

		// Check files have gone
		fis, err = VFS.ReadDir(cf.dir)
		what := fmt.Sprintf("err=%v, fis=%v", err, fis)
		if err == nil {
			assert.Equal(t, 0, len(fis), what)
		} else {
			require.True(t, errors.Is(err, stdfs.ErrNotExist), what)
		}

		// Reopen for cleanup
		require.NoError(t, cf.Open(true, 0))
	})
}
