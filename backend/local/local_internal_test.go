package local

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/file"
	"github.com/rclone/rclone/lib/readers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain drives the tests
func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

// Test copy with source file that's updating
func TestUpdatingCheck(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	filePath := "sub dir/local test"
	r.WriteFile(filePath, "content", time.Now())

	fd, err := file.Open(path.Join(r.LocalName, filePath))
	if err != nil {
		t.Fatalf("failed opening file %q: %v", filePath, err)
	}
	defer func() {
		require.NoError(t, fd.Close())
	}()

	fi, err := fd.Stat()
	require.NoError(t, err)
	o := &Object{size: fi.Size(), modTime: fi.ModTime(), fs: &Fs{}}
	wrappedFd := readers.NewLimitedReadCloser(fd, -1)
	hash, err := hash.NewMultiHasherTypes(hash.Supported())
	require.NoError(t, err)
	in := localOpenFile{
		o:    o,
		in:   wrappedFd,
		hash: hash,
		fd:   fd,
	}

	buf := make([]byte, 1)
	_, err = in.Read(buf)
	require.NoError(t, err)

	r.WriteFile(filePath, "content updated", time.Now())
	_, err = in.Read(buf)
	require.Errorf(t, err, "can't copy - source file is being updated")

	// turn the checking off and try again
	in.o.fs.opt.NoCheckUpdated = true

	r.WriteFile(filePath, "content updated", time.Now())
	_, err = in.Read(buf)
	require.NoError(t, err)

}

func TestSymlink(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	f := r.Flocal.(*Fs)
	dir := f.root

	// Write a file
	modTime1 := fstest.Time("2001-02-03T04:05:10.123123123Z")
	file1 := r.WriteFile("file.txt", "hello", modTime1)

	// Write a symlink
	modTime2 := fstest.Time("2002-02-03T04:05:10.123123123Z")
	symlinkPath := filepath.Join(dir, "symlink.txt")
	require.NoError(t, os.Symlink("file.txt", symlinkPath))
	require.NoError(t, lChtimes(symlinkPath, modTime2, modTime2))

	// Object viewed as symlink
	file2 := fstest.NewItem("symlink.txt"+linkSuffix, "file.txt", modTime2)
	if runtime.GOOS == "windows" {
		file2.Size = 0 // symlinks are 0 length under Windows
	}

	// Object viewed as destination
	file2d := fstest.NewItem("symlink.txt", "hello", modTime1)

	// Check with no symlink flags
	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote)

	// Set fs into "-L" mode
	f.opt.FollowSymlinks = true
	f.opt.TranslateSymlinks = false
	f.lstat = os.Stat

	fstest.CheckItems(t, r.Flocal, file1, file2d)
	fstest.CheckItems(t, r.Fremote)

	// Set fs into "-l" mode
	f.opt.FollowSymlinks = false
	f.opt.TranslateSymlinks = true
	f.lstat = os.Lstat

	fstest.CheckListingWithPrecision(t, r.Flocal, []fstest.Item{file1, file2}, nil, fs.ModTimeNotSupported)
	if haveLChtimes {
		fstest.CheckItems(t, r.Flocal, file1, file2)
	}

	// Create a symlink
	modTime3 := fstest.Time("2002-03-03T04:05:10.123123123Z")
	file3 := r.WriteObjectTo(ctx, r.Flocal, "symlink2.txt"+linkSuffix, "file.txt", modTime3, false)
	if runtime.GOOS == "windows" {
		file3.Size = 0 // symlinks are 0 length under Windows
	}
	fstest.CheckListingWithPrecision(t, r.Flocal, []fstest.Item{file1, file2, file3}, nil, fs.ModTimeNotSupported)
	if haveLChtimes {
		fstest.CheckItems(t, r.Flocal, file1, file2, file3)
	}

	// Check it got the correct contents
	symlinkPath = filepath.Join(dir, "symlink2.txt")
	fi, err := os.Lstat(symlinkPath)
	require.NoError(t, err)
	assert.False(t, fi.Mode().IsRegular())
	linkText, err := os.Readlink(symlinkPath)
	require.NoError(t, err)
	assert.Equal(t, "file.txt", linkText)

	// Check that NewObject gets the correct object
	o, err := r.Flocal.NewObject(ctx, "symlink2.txt"+linkSuffix)
	require.NoError(t, err)
	assert.Equal(t, "symlink2.txt"+linkSuffix, o.Remote())
	if runtime.GOOS != "windows" {
		assert.Equal(t, int64(8), o.Size())
	}

	// Check that NewObject doesn't see the non suffixed version
	_, err = r.Flocal.NewObject(ctx, "symlink2.txt")
	require.Equal(t, fs.ErrorObjectNotFound, err)

	// Check reading the object
	in, err := o.Open(ctx)
	require.NoError(t, err)
	contents, err := ioutil.ReadAll(in)
	require.NoError(t, err)
	require.Equal(t, "file.txt", string(contents))
	require.NoError(t, in.Close())

	// Check reading the object with range
	in, err = o.Open(ctx, &fs.RangeOption{Start: 2, End: 5})
	require.NoError(t, err)
	contents, err = ioutil.ReadAll(in)
	require.NoError(t, err)
	require.Equal(t, "file.txt"[2:5+1], string(contents))
	require.NoError(t, in.Close())
}

func TestSymlinkError(t *testing.T) {
	m := configmap.Simple{
		"links":      "true",
		"copy_links": "true",
	}
	_, err := NewFs("local", "/", m)
	assert.Equal(t, errLinksAndCopyLinks, err)
}
