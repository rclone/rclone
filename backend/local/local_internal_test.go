package local

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
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

// Test corrupted on transfer
// should error due to size/hash mismatch
func TestVerifyCopy(t *testing.T) {
	t.Skip("FIXME this test is unreliable")
	r := fstest.NewRun(t)
	filePath := "sub dir/local test"
	r.WriteFile(filePath, "some content", time.Now())
	src, err := r.Flocal.NewObject(context.Background(), filePath)
	require.NoError(t, err)
	src.(*Object).fs.opt.NoCheckUpdated = true

	for i := 0; i < 100; i++ {
		go r.WriteFile(src.Remote(), fmt.Sprintf("some new content %d", i), src.ModTime(context.Background()))
	}
	_, err = operations.Copy(context.Background(), r.Fremote, nil, filePath+"2", src)
	assert.Error(t, err)
}

func TestSymlink(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
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

	// Object viewed as destination
	file2d := fstest.NewItem("symlink.txt", "hello", modTime1)

	// Check with no symlink flags
	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t)

	// Set fs into "-L" mode
	f.opt.FollowSymlinks = true
	f.opt.TranslateSymlinks = false
	f.lstat = os.Stat

	r.CheckLocalItems(t, file1, file2d)
	r.CheckRemoteItems(t)

	// Set fs into "-l" mode
	f.opt.FollowSymlinks = false
	f.opt.TranslateSymlinks = true
	f.lstat = os.Lstat

	fstest.CheckListingWithPrecision(t, r.Flocal, []fstest.Item{file1, file2}, nil, fs.ModTimeNotSupported)
	if haveLChtimes {
		r.CheckLocalItems(t, file1, file2)
	}

	// Create a symlink
	modTime3 := fstest.Time("2002-03-03T04:05:10.123123123Z")
	file3 := r.WriteObjectTo(ctx, r.Flocal, "symlink2.txt"+linkSuffix, "file.txt", modTime3, false)
	fstest.CheckListingWithPrecision(t, r.Flocal, []fstest.Item{file1, file2, file3}, nil, fs.ModTimeNotSupported)
	if haveLChtimes {
		r.CheckLocalItems(t, file1, file2, file3)
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
	assert.Equal(t, int64(8), o.Size())

	// Check that NewObject doesn't see the non suffixed version
	_, err = r.Flocal.NewObject(ctx, "symlink2.txt")
	require.Equal(t, fs.ErrorObjectNotFound, err)

	// Check that NewFs works with the suffixed version and --links
	f2, err := NewFs(ctx, "local", filepath.Join(dir, "symlink2.txt"+linkSuffix), configmap.Simple{
		"links": "true",
	})
	require.Equal(t, fs.ErrorIsFile, err)
	require.Equal(t, dir, f2.(*Fs).root)

	// Check that NewFs doesn't see the non suffixed version with --links
	f2, err = NewFs(ctx, "local", filepath.Join(dir, "symlink2.txt"), configmap.Simple{
		"links": "true",
	})
	require.Equal(t, errLinksNeedsSuffix, err)
	require.Nil(t, f2)

	// Check reading the object
	in, err := o.Open(ctx)
	require.NoError(t, err)
	contents, err := io.ReadAll(in)
	require.NoError(t, err)
	require.Equal(t, "file.txt", string(contents))
	require.NoError(t, in.Close())

	// Check reading the object with range
	in, err = o.Open(ctx, &fs.RangeOption{Start: 2, End: 5})
	require.NoError(t, err)
	contents, err = io.ReadAll(in)
	require.NoError(t, err)
	require.Equal(t, "file.txt"[2:5+1], string(contents))
	require.NoError(t, in.Close())
}

func TestSymlinkError(t *testing.T) {
	m := configmap.Simple{
		"links":      "true",
		"copy_links": "true",
	}
	_, err := NewFs(context.Background(), "local", "/", m)
	assert.Equal(t, errLinksAndCopyLinks, err)
}

// Test hashes on updating an object
func TestHashOnUpdate(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	const filePath = "file.txt"
	when := time.Now()
	r.WriteFile(filePath, "content", when)
	f := r.Flocal.(*Fs)

	// Get the object
	o, err := f.NewObject(ctx, filePath)
	require.NoError(t, err)

	// Test the hash is as we expect
	md5, err := o.Hash(ctx, hash.MD5)
	require.NoError(t, err)
	assert.Equal(t, "9a0364b9e99bb480dd25e1f0284c8555", md5)

	// Reupload it with different contents but same size and timestamp
	var b = bytes.NewBufferString("CONTENT")
	src := object.NewStaticObjectInfo(filePath, when, int64(b.Len()), true, nil, f)
	err = o.Update(ctx, b, src)
	require.NoError(t, err)

	// Check the hash is as expected
	md5, err = o.Hash(ctx, hash.MD5)
	require.NoError(t, err)
	assert.Equal(t, "45685e95985e20822fb2538a522a5ccf", md5)
}

// Test hashes on deleting an object
func TestHashOnDelete(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	const filePath = "file.txt"
	when := time.Now()
	r.WriteFile(filePath, "content", when)
	f := r.Flocal.(*Fs)

	// Get the object
	o, err := f.NewObject(ctx, filePath)
	require.NoError(t, err)

	// Test the hash is as we expect
	md5, err := o.Hash(ctx, hash.MD5)
	require.NoError(t, err)
	assert.Equal(t, "9a0364b9e99bb480dd25e1f0284c8555", md5)

	// Delete the object
	require.NoError(t, o.Remove(ctx))

	// Test the hash cache is empty
	require.Nil(t, o.(*Object).hashes)

	// Test the hash returns an error
	_, err = o.Hash(ctx, hash.MD5)
	require.Error(t, err)
}

func TestMetadata(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	const filePath = "metafile.txt"
	when := time.Now()
	const dayLength = len("2001-01-01")
	whenRFC := when.Format(time.RFC3339Nano)
	r.WriteFile(filePath, "metadata file contents", when)
	f := r.Flocal.(*Fs)

	// Get the object
	obj, err := f.NewObject(ctx, filePath)
	require.NoError(t, err)
	o := obj.(*Object)

	features := f.Features()

	var hasXID, hasAtime, hasBtime bool
	switch runtime.GOOS {
	case "darwin", "freebsd", "netbsd", "linux":
		hasXID, hasAtime, hasBtime = true, true, true
	case "openbsd", "solaris":
		hasXID, hasAtime = true, true
	case "windows":
		hasAtime, hasBtime = true, true
	case "plan9", "js":
		// nada
	default:
		t.Errorf("No test cases for OS %q", runtime.GOOS)
	}

	assert.True(t, features.ReadMetadata)
	assert.True(t, features.WriteMetadata)
	assert.Equal(t, xattrSupported, features.UserMetadata)

	t.Run("Xattr", func(t *testing.T) {
		if !xattrSupported {
			t.Skip()
		}
		m, err := o.getXattr()
		require.NoError(t, err)
		assert.Nil(t, m)

		inM := fs.Metadata{
			"potato":  "chips",
			"cabbage": "soup",
		}
		err = o.setXattr(inM)
		require.NoError(t, err)

		m, err = o.getXattr()
		require.NoError(t, err)
		assert.NotNil(t, m)
		assert.Equal(t, inM, m)
	})

	checkTime := func(m fs.Metadata, key string, when time.Time) {
		mt, ok := o.parseMetadataTime(m, key)
		assert.True(t, ok)
		dt := mt.Sub(when)
		precision := time.Second
		assert.True(t, dt >= -precision && dt <= precision, fmt.Sprintf("%s: dt %v outside +/- precision %v", key, dt, precision))
	}

	checkInt := func(m fs.Metadata, key string, base int) int {
		value, ok := o.parseMetadataInt(m, key, base)
		assert.True(t, ok)
		return value
	}
	t.Run("Read", func(t *testing.T) {
		m, err := o.Metadata(ctx)
		require.NoError(t, err)
		assert.NotNil(t, m)

		// All OSes have these
		checkInt(m, "mode", 8)
		checkTime(m, "mtime", when)

		assert.Equal(t, len(whenRFC), len(m["mtime"]))
		assert.Equal(t, whenRFC[:dayLength], m["mtime"][:dayLength])

		if hasAtime {
			checkTime(m, "atime", when)
		}
		if hasBtime {
			checkTime(m, "btime", when)
		}
		if hasXID {
			checkInt(m, "uid", 10)
			checkInt(m, "gid", 10)
		}
	})

	t.Run("Write", func(t *testing.T) {
		newAtimeString := "2011-12-13T14:15:16.999999999Z"
		newAtime := fstest.Time(newAtimeString)
		newMtimeString := "2011-12-12T14:15:16.999999999Z"
		newMtime := fstest.Time(newMtimeString)
		newBtimeString := "2011-12-11T14:15:16.999999999Z"
		newBtime := fstest.Time(newBtimeString)
		newM := fs.Metadata{
			"mtime": newMtimeString,
			"atime": newAtimeString,
			"btime": newBtimeString,
			// Can't test uid, gid without being root
			"mode":   "0767",
			"potato": "wedges",
		}
		err := o.writeMetadata(newM)
		require.NoError(t, err)

		m, err := o.Metadata(ctx)
		require.NoError(t, err)
		assert.NotNil(t, m)

		mode := checkInt(m, "mode", 8)
		if runtime.GOOS != "windows" {
			assert.Equal(t, 0767, mode&0777, fmt.Sprintf("mode wrong - expecting 0767 got 0%o", mode&0777))
		}

		checkTime(m, "mtime", newMtime)
		if hasAtime {
			checkTime(m, "atime", newAtime)
		}
		if haveSetBTime {
			checkTime(m, "btime", newBtime)
		}
		if xattrSupported {
			assert.Equal(t, "wedges", m["potato"])
		}
	})

}

func TestFilter(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	when := time.Now()
	r.WriteFile("included", "included file", when)
	r.WriteFile("excluded", "excluded file", when)
	f := r.Flocal.(*Fs)

	// Check set up for filtering
	assert.True(t, f.Features().FilterAware)

	// Add a filter
	ctx, fi := filter.AddConfig(ctx)
	require.NoError(t, fi.AddRule("+ included"))
	require.NoError(t, fi.AddRule("- *"))

	// Check listing without use filter flag
	entries, err := f.List(ctx, "")
	require.NoError(t, err)
	sort.Sort(entries)
	require.Equal(t, "[excluded included]", fmt.Sprint(entries))

	// Add user filter flag
	ctx = filter.SetUseFilter(ctx, true)

	// Check listing with use filter flag
	entries, err = f.List(ctx, "")
	require.NoError(t, err)
	sort.Sort(entries)
	require.Equal(t, "[included]", fmt.Sprint(entries))
}

func testFilterSymlink(t *testing.T, copyLinks bool) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	when := time.Now()
	f := r.Flocal.(*Fs)

	// Create a file, a directory, a symlink to a file, a symlink to a directory and a dangling symlink
	r.WriteFile("included.file", "included file", when)
	r.WriteFile("included.dir/included.sub.file", "included sub file", when)
	require.NoError(t, os.Symlink("included.file", filepath.Join(r.LocalName, "included.file.link")))
	require.NoError(t, os.Symlink("included.dir", filepath.Join(r.LocalName, "included.dir.link")))
	require.NoError(t, os.Symlink("dangling", filepath.Join(r.LocalName, "dangling.link")))

	defer func() {
		// Reset -L/-l mode
		f.opt.FollowSymlinks = false
		f.opt.TranslateSymlinks = false
		f.lstat = os.Lstat
	}()
	if copyLinks {
		// Set fs into "-L" mode
		f.opt.FollowSymlinks = true
		f.opt.TranslateSymlinks = false
		f.lstat = os.Stat
	} else {
		// Set fs into "-l" mode
		f.opt.FollowSymlinks = false
		f.opt.TranslateSymlinks = true
		f.lstat = os.Lstat
	}

	// Check set up for filtering
	assert.True(t, f.Features().FilterAware)

	// Reset global error count
	accounting.Stats(ctx).ResetErrors()
	assert.Equal(t, int64(0), accounting.Stats(ctx).GetErrors(), "global errors found")

	// Add a filter
	ctx, fi := filter.AddConfig(ctx)
	require.NoError(t, fi.AddRule("+ included.file"))
	require.NoError(t, fi.AddRule("+ included.dir/**"))
	if copyLinks {
		require.NoError(t, fi.AddRule("+ included.file.link"))
		require.NoError(t, fi.AddRule("+ included.dir.link/**"))
	} else {
		require.NoError(t, fi.AddRule("+ included.file.link.rclonelink"))
		require.NoError(t, fi.AddRule("+ included.dir.link.rclonelink"))
	}
	require.NoError(t, fi.AddRule("- *"))

	// Check listing without use filter flag
	entries, err := f.List(ctx, "")
	require.NoError(t, err)

	if copyLinks {
		// Check 1 global errors one for each dangling symlink
		assert.Equal(t, int64(1), accounting.Stats(ctx).GetErrors(), "global errors found")
	} else {
		// Check 0 global errors as dangling symlink copied properly
		assert.Equal(t, int64(0), accounting.Stats(ctx).GetErrors(), "global errors found")
	}
	accounting.Stats(ctx).ResetErrors()

	sort.Sort(entries)
	if copyLinks {
		require.Equal(t, "[included.dir included.dir.link included.file included.file.link]", fmt.Sprint(entries))
	} else {
		require.Equal(t, "[dangling.link.rclonelink included.dir included.dir.link.rclonelink included.file included.file.link.rclonelink]", fmt.Sprint(entries))
	}

	// Add user filter flag
	ctx = filter.SetUseFilter(ctx, true)

	// Check listing with use filter flag
	entries, err = f.List(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, int64(0), accounting.Stats(ctx).GetErrors(), "global errors found")

	sort.Sort(entries)
	if copyLinks {
		require.Equal(t, "[included.dir included.dir.link included.file included.file.link]", fmt.Sprint(entries))
	} else {
		require.Equal(t, "[included.dir included.dir.link.rclonelink included.file included.file.link.rclonelink]", fmt.Sprint(entries))
	}

	// Check listing through a symlink still works
	entries, err = f.List(ctx, "included.dir")
	require.NoError(t, err)
	assert.Equal(t, int64(0), accounting.Stats(ctx).GetErrors(), "global errors found")

	sort.Sort(entries)
	require.Equal(t, "[included.dir/included.sub.file]", fmt.Sprint(entries))
}

func TestFilterSymlinkCopyLinks(t *testing.T) {
	testFilterSymlink(t, true)
}

func TestFilterSymlinkLinks(t *testing.T) {
	testFilterSymlink(t, false)
}

func TestCopySymlink(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	when := time.Now()
	f := r.Flocal.(*Fs)

	// Create a file and a symlink to it
	r.WriteFile("src/file.txt", "hello world", when)
	require.NoError(t, os.Symlink("file.txt", filepath.Join(r.LocalName, "src", "link.txt")))
	defer func() {
		// Reset -L/-l mode
		f.opt.FollowSymlinks = false
		f.opt.TranslateSymlinks = false
		f.lstat = os.Lstat
	}()

	// Set fs into "-l/--links" mode
	f.opt.FollowSymlinks = false
	f.opt.TranslateSymlinks = true
	f.lstat = os.Lstat

	// Create dst
	require.NoError(t, f.Mkdir(ctx, "dst"))

	// Do copy from src into dst
	src, err := f.NewObject(ctx, "src/link.txt.rclonelink")
	require.NoError(t, err)
	require.NotNil(t, src)
	dst, err := operations.Copy(ctx, f, nil, "dst/link.txt.rclonelink", src)
	require.NoError(t, err)
	require.NotNil(t, dst)

	// Test that we made a symlink and it has the right contents
	dstPath := filepath.Join(r.LocalName, "dst", "link.txt")
	linkContents, err := os.Readlink(dstPath)
	require.NoError(t, err)
	assert.Equal(t, "file.txt", linkContents)
}
