package union

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	obj1 := fstests.PutTestContents(ctx, t, rofs, &file1, contents, true)

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

	if runtime.GOOS == "darwin" {
		// need to disable as this test specifically tests a local that can't Copy
		f.Features().Disable("Copy")
		fLocal.Features().Disable("Copy")
	}

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
	_ = fstests.PutTestContents(ctx, t, fLocal, &fileLocal, contentsLocal, true)
	objLocal, err := f.NewObject(ctx, fileLocal.Path)
	require.NoError(t, err)

	// Put a file onto the memory fs
	contentsMemory := random.String(60)
	fileMemory := fstest.NewItem("memory.txt", contentsMemory, time.Now())
	_ = fstests.PutTestContents(ctx, t, fMemory, &fileMemory, contentsMemory, true)
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

// TestHealWrites reproduces the scenario in issue #9647: with
// create_policy = all, if one upstream is missing a file (e.g. from a
// past partial failure), re-running the copy should repair it when
// heal_writes is set, but does not by default.
func TestHealWrites(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	ctx := context.Background()

	newUnion := func(t *testing.T, dirs []string, healWrites bool) (fs.Fs, string, string) {
		fsString := fmt.Sprintf(":union,upstreams='%s %s',create_policy=all,heal_writes=%v:", dirs[0], dirs[1], healWrites)
		f, err := fs.NewFs(ctx, fsString)
		require.NoError(t, err)
		return f, dirs[0], dirs[1]
	}

	t.Run("DefaultDoesNotHeal", func(t *testing.T) {
		dirs := MakeTestDirs(t, 2)
		f, u1, u2 := newUnion(t, dirs, false)

		contents := random.String(50)
		file := fstest.NewItem("file.txt", contents, time.Now())
		_ = fstests.PutTestContents(ctx, t, f, &file, contents, true)

		// Both upstreams should have the file after the initial put
		assert.FileExists(t, filepath.Join(u1, "file.txt"))
		assert.FileExists(t, filepath.Join(u2, "file.txt"))

		// Simulate a partial failure: the file goes missing on u1
		require.NoError(t, os.Remove(filepath.Join(u1, "file.txt")))

		// Re-running the same write via the union Object.Update should
		// only touch upstreams which already have the file
		o, err := f.NewObject(ctx, file.Path)
		require.NoError(t, err)
		newContents := random.String(60)
		in := bytes.NewBufferString(newContents)
		src := object.NewStaticObjectInfo(file.Path, time.Now(), int64(len(newContents)), true, nil, nil)
		require.NoError(t, o.Update(ctx, in, src))

		assert.NoFileExists(t, filepath.Join(u1, "file.txt"))
		assert.FileExists(t, filepath.Join(u2, "file.txt"))
	})

	t.Run("HealWritesRepairsMissingUpstream", func(t *testing.T) {
		dirs := MakeTestDirs(t, 2)
		f, u1, u2 := newUnion(t, dirs, true)

		contents := random.String(50)
		file := fstest.NewItem("file.txt", contents, time.Now())
		_ = fstests.PutTestContents(ctx, t, f, &file, contents, true)

		assert.FileExists(t, filepath.Join(u1, "file.txt"))
		assert.FileExists(t, filepath.Join(u2, "file.txt"))

		// Simulate a partial failure: the file goes missing on u1
		require.NoError(t, os.Remove(filepath.Join(u1, "file.txt")))

		o, err := f.NewObject(ctx, file.Path)
		require.NoError(t, err)
		newContents := random.String(60)
		in := bytes.NewBufferString(newContents)
		src := object.NewStaticObjectInfo(file.Path, time.Now(), int64(len(newContents)), true, nil, nil)
		require.NoError(t, o.Update(ctx, in, src))

		// heal_writes should have created the file on u1 as well as
		// updating it on u2
		assert.FileExists(t, filepath.Join(u1, "file.txt"))
		assert.FileExists(t, filepath.Join(u2, "file.txt"))

		b1, err := os.ReadFile(filepath.Join(u1, "file.txt"))
		require.NoError(t, err)
		b2, err := os.ReadFile(filepath.Join(u2, "file.txt"))
		require.NoError(t, err)
		assert.Equal(t, newContents, string(b1))
		assert.Equal(t, newContents, string(b2))

		// The union object itself should reflect the healed size
		assert.Equal(t, int64(len(newContents)), o.Size())
	})

	// With a narrowing action_policy such as epff, actionEntries only
	// returns a subset of the upstreams which actually hold the object.
	// heal_writes must compare against the object's full candidate list,
	// not that narrowed subset, or it will misclassify upstreams which
	// already have the file as missing and create duplicate candidates.
	t.Run("HealWritesDoesNotDuplicateWithNarrowingActionPolicy", func(t *testing.T) {
		dirs := MakeTestDirs(t, 2)
		fsString := fmt.Sprintf(":union,upstreams='%s %s',action_policy=epff,create_policy=all,heal_writes=true:", dirs[0], dirs[1])
		f, err := fs.NewFs(ctx, fsString)
		require.NoError(t, err)
		u1, u2 := dirs[0], dirs[1]

		contents := random.String(50)
		file := fstest.NewItem("file.txt", contents, time.Now())
		_ = fstests.PutTestContents(ctx, t, f, &file, contents, true)

		// Both upstreams genuinely have the file - nothing is missing
		assert.FileExists(t, filepath.Join(u1, "file.txt"))
		assert.FileExists(t, filepath.Join(u2, "file.txt"))

		o, err := f.NewObject(ctx, file.Path)
		require.NoError(t, err)
		uo := o.(*Object)
		require.Len(t, uo.candidates(), 2)

		// epff always selects the same (first) upstream for update, so the
		// other upstream's content is expected to go stale - that's the
		// policy working as intended, not something heal_writes should
		// override since the file isn't missing there, just outdated
		u2Before, err := os.ReadFile(filepath.Join(u2, "file.txt"))
		require.NoError(t, err)

		for i := range 3 {
			newContents := random.String(60 + i)
			in := bytes.NewBufferString(newContents)
			src := object.NewStaticObjectInfo(file.Path, time.Now(), int64(len(newContents)), true, nil, nil)
			require.NoError(t, uo.Update(ctx, in, src))

			// The candidate list must not grow: nothing was missing, so
			// epff's single action candidate must not be treated as the
			// full set and cause the other upstream to be re-created
			assert.Len(t, uo.candidates(), 2, "candidate list should not grow on update %d", i)

			b1, err := os.ReadFile(filepath.Join(u1, "file.txt"))
			require.NoError(t, err)
			assert.Equal(t, newContents, string(b1))
		}

		// u2 was never missing, so heal_writes must not have touched it
		u2After, err := os.ReadFile(filepath.Join(u2, "file.txt"))
		require.NoError(t, err)
		assert.Equal(t, string(u2Before), string(u2After))
	})
}
