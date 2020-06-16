package operations_test

import (
	"context"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/fstest"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check flag satisfies the interface
var _ pflag.Value = (*operations.DeduplicateMode)(nil)

func skipIfCantDedupe(t *testing.T, f fs.Fs) {
	if !f.Features().DuplicateFiles {
		t.Skip("Can't test deduplicate - no duplicate files possible")
	}
	if f.Features().PutUnchecked == nil {
		t.Skip("Can't test deduplicate - no PutUnchecked")
	}
	if f.Features().MergeDirs == nil {
		t.Skip("Can't test deduplicate - no MergeDirs")
	}
}

func skipIfNoHash(t *testing.T, f fs.Fs) {
	if f.Hashes().GetOne() == hash.None {
		t.Skip("Can't run this test without a hash")
	}
}

func TestDeduplicateInteractive(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	skipIfCantDedupe(t, r.Fremote)
	skipIfNoHash(t, r.Fremote)

	file1 := r.WriteUncheckedObject(context.Background(), "one", "This is one", t1)
	file2 := r.WriteUncheckedObject(context.Background(), "one", "This is one", t1)
	file3 := r.WriteUncheckedObject(context.Background(), "one", "This is one", t1)
	r.CheckWithDuplicates(t, file1, file2, file3)

	err := operations.Deduplicate(context.Background(), r.Fremote, operations.DeduplicateInteractive)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Fremote, file1)
}

func TestDeduplicateSkip(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	skipIfCantDedupe(t, r.Fremote)
	haveHash := r.Fremote.Hashes().GetOne() != hash.None

	file1 := r.WriteUncheckedObject(context.Background(), "one", "This is one", t1)
	files := []fstest.Item{file1}
	if haveHash {
		file2 := r.WriteUncheckedObject(context.Background(), "one", "This is one", t1)
		files = append(files, file2)
	}
	file3 := r.WriteUncheckedObject(context.Background(), "one", "This is another one", t1)
	files = append(files, file3)
	r.CheckWithDuplicates(t, files...)

	err := operations.Deduplicate(context.Background(), r.Fremote, operations.DeduplicateSkip)
	require.NoError(t, err)

	r.CheckWithDuplicates(t, file1, file3)
}

func TestDeduplicateSizeOnly(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	skipIfCantDedupe(t, r.Fremote)

	file1 := r.WriteUncheckedObject(context.Background(), "one", "This is one", t1)
	file2 := r.WriteUncheckedObject(context.Background(), "one", "THIS IS ONE", t1)
	file3 := r.WriteUncheckedObject(context.Background(), "one", "This is another one", t1)
	r.CheckWithDuplicates(t, file1, file2, file3)

	fs.Config.SizeOnly = true
	defer func() {
		fs.Config.SizeOnly = false
	}()

	err := operations.Deduplicate(context.Background(), r.Fremote, operations.DeduplicateSkip)
	require.NoError(t, err)

	r.CheckWithDuplicates(t, file1, file3)
}

func TestDeduplicateFirst(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	skipIfCantDedupe(t, r.Fremote)

	file1 := r.WriteUncheckedObject(context.Background(), "one", "This is one", t1)
	file2 := r.WriteUncheckedObject(context.Background(), "one", "This is one A", t1)
	file3 := r.WriteUncheckedObject(context.Background(), "one", "This is one BB", t1)
	r.CheckWithDuplicates(t, file1, file2, file3)

	err := operations.Deduplicate(context.Background(), r.Fremote, operations.DeduplicateFirst)
	require.NoError(t, err)

	// list until we get one object
	var objects, size int64
	for try := 1; try <= *fstest.ListRetries; try++ {
		objects, size, err = operations.Count(context.Background(), r.Fremote)
		require.NoError(t, err)
		if objects == 1 {
			break
		}
		time.Sleep(time.Second)
	}
	assert.Equal(t, int64(1), objects)
	if size != file1.Size && size != file2.Size && size != file3.Size {
		t.Errorf("Size not one of the object sizes %d", size)
	}
}

func TestDeduplicateNewest(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	skipIfCantDedupe(t, r.Fremote)

	file1 := r.WriteUncheckedObject(context.Background(), "one", "This is one", t1)
	file2 := r.WriteUncheckedObject(context.Background(), "one", "This is one too", t2)
	file3 := r.WriteUncheckedObject(context.Background(), "one", "This is another one", t3)
	r.CheckWithDuplicates(t, file1, file2, file3)

	err := operations.Deduplicate(context.Background(), r.Fremote, operations.DeduplicateNewest)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Fremote, file3)
}

func TestDeduplicateOldest(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	skipIfCantDedupe(t, r.Fremote)

	file1 := r.WriteUncheckedObject(context.Background(), "one", "This is one", t1)
	file2 := r.WriteUncheckedObject(context.Background(), "one", "This is one too", t2)
	file3 := r.WriteUncheckedObject(context.Background(), "one", "This is another one", t3)
	r.CheckWithDuplicates(t, file1, file2, file3)

	err := operations.Deduplicate(context.Background(), r.Fremote, operations.DeduplicateOldest)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Fremote, file1)
}

func TestDeduplicateLargest(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	skipIfCantDedupe(t, r.Fremote)

	file1 := r.WriteUncheckedObject(context.Background(), "one", "This is one", t1)
	file2 := r.WriteUncheckedObject(context.Background(), "one", "This is one too", t2)
	file3 := r.WriteUncheckedObject(context.Background(), "one", "This is another one", t3)
	r.CheckWithDuplicates(t, file1, file2, file3)

	err := operations.Deduplicate(context.Background(), r.Fremote, operations.DeduplicateLargest)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Fremote, file3)
}

func TestDeduplicateSmallest(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	skipIfCantDedupe(t, r.Fremote)

	file1 := r.WriteUncheckedObject(context.Background(), "one", "This is one", t1)
	file2 := r.WriteUncheckedObject(context.Background(), "one", "This is one too", t2)
	file3 := r.WriteUncheckedObject(context.Background(), "one", "This is another one", t3)
	r.CheckWithDuplicates(t, file1, file2, file3)

	err := operations.Deduplicate(context.Background(), r.Fremote, operations.DeduplicateSmallest)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Fremote, file1)
}

func TestDeduplicateRename(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	skipIfCantDedupe(t, r.Fremote)

	file1 := r.WriteUncheckedObject(context.Background(), "one.txt", "This is one", t1)
	file2 := r.WriteUncheckedObject(context.Background(), "one.txt", "This is one too", t2)
	file3 := r.WriteUncheckedObject(context.Background(), "one.txt", "This is another one", t3)
	file4 := r.WriteUncheckedObject(context.Background(), "one-1.txt", "This is not a duplicate", t1)
	r.CheckWithDuplicates(t, file1, file2, file3, file4)

	err := operations.Deduplicate(context.Background(), r.Fremote, operations.DeduplicateRename)
	require.NoError(t, err)

	require.NoError(t, walk.ListR(context.Background(), r.Fremote, "", true, -1, walk.ListObjects, func(entries fs.DirEntries) error {
		entries.ForObject(func(o fs.Object) {
			remote := o.Remote()
			if remote != "one-1.txt" &&
				remote != "one-2.txt" &&
				remote != "one-3.txt" &&
				remote != "one-4.txt" {
				t.Errorf("Bad file name after rename %q", remote)
			}
			size := o.Size()
			if size != file1.Size &&
				size != file2.Size &&
				size != file3.Size &&
				size != file4.Size {
				t.Errorf("Size not one of the object sizes %d", size)
			}
			if remote == "one-1.txt" && size != file4.Size {
				t.Errorf("Existing non-duplicate file modified %q", remote)
			}
		})
		return nil
	}))
}

// This should really be a unit test, but the test framework there
// doesn't have enough tools to make it easy
func TestMergeDirs(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	mergeDirs := r.Fremote.Features().MergeDirs
	if mergeDirs == nil {
		t.Skip("Can't merge directories")
	}

	file1 := r.WriteObject(context.Background(), "dupe1/one.txt", "This is one", t1)
	file2 := r.WriteObject(context.Background(), "dupe2/two.txt", "This is one too", t2)
	file3 := r.WriteObject(context.Background(), "dupe3/three.txt", "This is another one", t3)

	objs, dirs, err := walk.GetAll(context.Background(), r.Fremote, "", true, 1)
	require.NoError(t, err)
	assert.Equal(t, 3, len(dirs))
	assert.Equal(t, 0, len(objs))

	err = mergeDirs(context.Background(), dirs)
	require.NoError(t, err)

	file2.Path = "dupe1/two.txt"
	file3.Path = "dupe1/three.txt"
	fstest.CheckItems(t, r.Fremote, file1, file2, file3)

	objs, dirs, err = walk.GetAll(context.Background(), r.Fremote, "", true, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, len(dirs))
	assert.Equal(t, 0, len(objs))
	assert.Equal(t, "dupe1", dirs[0].Remote())
}
