package union

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MakeTestDirs makes directories in /tmp for testing
func MakeTestDirs(t *testing.T, n int) (dirs []string) {
	for i := 1; i <= n; i++ {
		dir := t.TempDir()
		dirs = append(dirs, dir)
	}
	return dirs
}

func (f *Fs) TestInternalReadOnly(t *testing.T) {
	if f.name != "TestUnionRO" {
		t.Skip("Only on RO union")
	}
	dir := "TestInternalReadOnly"
	ctx := context.Background()
	rofs := f.upstreams[len(f.upstreams)-1]
	assert.False(t, rofs.IsWritable())

	// Put a file onto the read only fs
	contents := random.String(50)
	file1 := fstest.NewItem(dir+"/file.txt", contents, time.Now())
	_, obj1 := fstests.PutTestContents(ctx, t, rofs, &file1, contents, true)

	// Check read from readonly fs via union
	o, err := f.NewObject(ctx, file1.Path)
	require.NoError(t, err)
	assert.Equal(t, int64(50), o.Size())

	// Now call Update on the union Object with new data
	contents2 := random.String(100)
	file2 := fstest.NewItem(dir+"/file.txt", contents2, time.Now())
	in := bytes.NewBufferString(contents2)
	src := object.NewStaticObjectInfo(file2.Path, file2.ModTime, file2.Size, true, nil, nil)
	err = o.Update(ctx, in, src)
	require.NoError(t, err)
	assert.Equal(t, int64(100), o.Size())

	// Check we read the new object via the union
	o, err = f.NewObject(ctx, file1.Path)
	require.NoError(t, err)
	assert.Equal(t, int64(100), o.Size())

	// Remove the object
	assert.NoError(t, o.Remove(ctx))

	// Check we read the old object in the read only layer now
	o, err = f.NewObject(ctx, file1.Path)
	require.NoError(t, err)
	assert.Equal(t, int64(50), o.Size())

	// Remove file and dir from read only fs
	assert.NoError(t, obj1.Remove(ctx))
	assert.NoError(t, rofs.Rmdir(ctx, dir))
}

func (f *Fs) InternalTest(t *testing.T) {
	t.Run("ReadOnly", f.TestInternalReadOnly)
}

var _ fstests.InternalTester = (*Fs)(nil)

// This specifically tests a union of local which can Move but not
// Copy and :memory: which can Copy but not Move to makes sure that
// the resulting union can Move
func TestMoveCopy(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	ctx := context.Background()
	dirs := MakeTestDirs(t, 1)
	fsString := fmt.Sprintf(":union,upstreams='%s :memory:bucket':", dirs[0])
	f, err := fs.NewFs(ctx, fsString)
	require.NoError(t, err)

	unionFs := f.(*Fs)
	fLocal := unionFs.upstreams[0].Fs
	fMemory := unionFs.upstreams[1].Fs

	t.Run("Features", func(t *testing.T) {
		assert.NotNil(t, f.Features().Move)
		assert.Nil(t, f.Features().Copy)

		// Check underlying are as we are expect
		assert.NotNil(t, fLocal.Features().Move)
		assert.Nil(t, fLocal.Features().Copy)
		assert.Nil(t, fMemory.Features().Move)
		assert.NotNil(t, fMemory.Features().Copy)
	})

	// Put a file onto the local fs
	contentsLocal := random.String(50)
	fileLocal := fstest.NewItem("local.txt", contentsLocal, time.Now())
	_, _ = fstests.PutTestContents(ctx, t, fLocal, &fileLocal, contentsLocal, true)
	objLocal, err := f.NewObject(ctx, fileLocal.Path)
	require.NoError(t, err)

	// Put a file onto the memory fs
	contentsMemory := random.String(60)
	fileMemory := fstest.NewItem("memory.txt", contentsMemory, time.Now())
	_, _ = fstests.PutTestContents(ctx, t, fMemory, &fileMemory, contentsMemory, true)
	objMemory, err := f.NewObject(ctx, fileMemory.Path)
	require.NoError(t, err)

	fstest.CheckListing(t, f, []fstest.Item{fileLocal, fileMemory})

	t.Run("MoveLocal", func(t *testing.T) {
		fileLocal.Path = "local-renamed.txt"
		_, err := operations.Move(ctx, f, nil, fileLocal.Path, objLocal)
		require.NoError(t, err)
		fstest.CheckListing(t, f, []fstest.Item{fileLocal, fileMemory})

		// Check can retrieve object from union
		obj, err := f.NewObject(ctx, fileLocal.Path)
		require.NoError(t, err)
		assert.Equal(t, fileLocal.Size, obj.Size())

		// Check can retrieve object from underlying
		obj, err = fLocal.NewObject(ctx, fileLocal.Path)
		require.NoError(t, err)
		assert.Equal(t, fileLocal.Size, obj.Size())

		t.Run("MoveMemory", func(t *testing.T) {
			fileMemory.Path = "memory-renamed.txt"
			_, err := operations.Move(ctx, f, nil, fileMemory.Path, objMemory)
			require.NoError(t, err)
			fstest.CheckListing(t, f, []fstest.Item{fileLocal, fileMemory})

			// Check can retrieve object from union
			obj, err := f.NewObject(ctx, fileMemory.Path)
			require.NoError(t, err)
			assert.Equal(t, fileMemory.Size, obj.Size())

			// Check can retrieve object from underlying
			obj, err = fMemory.NewObject(ctx, fileMemory.Path)
			require.NoError(t, err)
			assert.Equal(t, fileMemory.Size, obj.Size())
		})
	})
}
