// Test sync/copy/move

package fs_test

import (
	"testing"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check dry run is working
func TestCopyWithDryRun(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)

	fs.Config.DryRun = true
	err := fs.CopyDir(r.fremote, r.flocal)
	fs.Config.DryRun = false
	require.NoError(t, err)

	fstest.CheckItems(t, r.flocal, file1)
	fstest.CheckItems(t, r.fremote)
}

// Now without dry run
func TestCopy(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)

	err := fs.CopyDir(r.fremote, r.flocal)
	require.NoError(t, err)

	fstest.CheckItems(t, r.flocal, file1)
	fstest.CheckItems(t, r.fremote, file1)
}

// Now with --no-traverse
func TestCopyNoTraverse(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()

	fs.Config.NoTraverse = true
	defer func() { fs.Config.NoTraverse = false }()

	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)

	err := fs.CopyDir(r.fremote, r.flocal)
	require.NoError(t, err)

	fstest.CheckItems(t, r.flocal, file1)
	fstest.CheckItems(t, r.fremote, file1)
}

// Now with --no-traverse
func TestSyncNoTraverse(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()

	fs.Config.NoTraverse = true
	defer func() { fs.Config.NoTraverse = false }()

	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)

	fstest.CheckItems(t, r.flocal, file1)
	fstest.CheckItems(t, r.fremote, file1)
}

// Test copy with depth
func TestCopyWithDepth(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	file2 := r.WriteFile("hello world2", "hello world2", t2)

	// Check the MaxDepth too
	fs.Config.MaxDepth = 1
	defer func() { fs.Config.MaxDepth = -1 }()

	err := fs.CopyDir(r.fremote, r.flocal)
	require.NoError(t, err)

	fstest.CheckItems(t, r.flocal, file1, file2)
	fstest.CheckItems(t, r.fremote, file2)
}

// Test a server side copy if possible, or the backup path if not
func TestServerSideCopy(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject("sub dir/hello world", "hello world", t1)
	fstest.CheckItems(t, r.fremote, file1)

	fremoteCopy, _, finaliseCopy, err := fstest.RandomRemote(*RemoteName, *SubDir)
	require.NoError(t, err)
	defer finaliseCopy()
	t.Logf("Server side copy (if possible) %v -> %v", r.fremote, fremoteCopy)

	err = fs.CopyDir(fremoteCopy, r.fremote)
	require.NoError(t, err)

	fstest.CheckItems(t, fremoteCopy, file1)
}

// Check that if the local file doesn't exist when we copy it up,
// nothing happens to the remote file
func TestCopyAfterDelete(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject("sub dir/hello world", "hello world", t1)
	fstest.CheckItems(t, r.flocal)
	fstest.CheckItems(t, r.fremote, file1)

	err := fs.Mkdir(r.flocal, "")
	require.NoError(t, err)

	err = fs.CopyDir(r.fremote, r.flocal)
	require.NoError(t, err)

	fstest.CheckItems(t, r.flocal)
	fstest.CheckItems(t, r.fremote, file1)
}

// Check the copy downloading a file
func TestCopyRedownload(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject("sub dir/hello world", "hello world", t1)
	fstest.CheckItems(t, r.fremote, file1)

	err := fs.CopyDir(r.flocal, r.fremote)
	require.NoError(t, err)

	fstest.CheckItems(t, r.flocal, file1)
}

// Create a file and sync it. Change the last modified date and resync.
// If we're only doing sync by size and checksum, we expect nothing to
// to be transferred on the second sync.
func TestSyncBasedOnCheckSum(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	fs.Config.CheckSum = true
	defer func() { fs.Config.CheckSum = false }()

	file1 := r.WriteFile("check sum", "", t1)
	fstest.CheckItems(t, r.flocal, file1)

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)

	// We should have transferred exactly one file.
	assert.Equal(t, int64(1), fs.Stats.GetTransfers())
	fstest.CheckItems(t, r.fremote, file1)

	// Change last modified date only
	file2 := r.WriteFile("check sum", "", t2)
	fstest.CheckItems(t, r.flocal, file2)

	fs.Stats.ResetCounters()
	err = fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)

	// We should have transferred no files
	assert.Equal(t, int64(0), fs.Stats.GetTransfers())
	fstest.CheckItems(t, r.flocal, file2)
	fstest.CheckItems(t, r.fremote, file1)
}

// Create a file and sync it. Change the last modified date and the
// file contents but not the size.  If we're only doing sync by size
// only, we expect nothing to to be transferred on the second sync.
func TestSyncSizeOnly(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	fs.Config.SizeOnly = true
	defer func() { fs.Config.SizeOnly = false }()

	file1 := r.WriteFile("sizeonly", "potato", t1)
	fstest.CheckItems(t, r.flocal, file1)

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)

	// We should have transferred exactly one file.
	assert.Equal(t, int64(1), fs.Stats.GetTransfers())
	fstest.CheckItems(t, r.fremote, file1)

	// Update mtime, md5sum but not length of file
	file2 := r.WriteFile("sizeonly", "POTATO", t2)
	fstest.CheckItems(t, r.flocal, file2)

	fs.Stats.ResetCounters()
	err = fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)

	// We should have transferred no files
	assert.Equal(t, int64(0), fs.Stats.GetTransfers())
	fstest.CheckItems(t, r.flocal, file2)
	fstest.CheckItems(t, r.fremote, file1)
}

// Create a file and sync it. Keep the last modified date but change
// the size.  With --ignore-size we expect nothing to to be
// transferred on the second sync.
func TestSyncIgnoreSize(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	fs.Config.IgnoreSize = true
	defer func() { fs.Config.IgnoreSize = false }()

	file1 := r.WriteFile("ignore-size", "contents", t1)
	fstest.CheckItems(t, r.flocal, file1)

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)

	// We should have transferred exactly one file.
	assert.Equal(t, int64(1), fs.Stats.GetTransfers())
	fstest.CheckItems(t, r.fremote, file1)

	// Update size but not date of file
	file2 := r.WriteFile("ignore-size", "longer contents but same date", t1)
	fstest.CheckItems(t, r.flocal, file2)

	fs.Stats.ResetCounters()
	err = fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)

	// We should have transferred no files
	assert.Equal(t, int64(0), fs.Stats.GetTransfers())
	fstest.CheckItems(t, r.flocal, file2)
	fstest.CheckItems(t, r.fremote, file1)
}

func TestSyncIgnoreTimes(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("existing", "potato", t1)
	fstest.CheckItems(t, r.fremote, file1)

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)

	// We should have transferred exactly 0 files because the
	// files were identical.
	assert.Equal(t, int64(0), fs.Stats.GetTransfers())

	fs.Config.IgnoreTimes = true
	defer func() { fs.Config.IgnoreTimes = false }()

	fs.Stats.ResetCounters()
	err = fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)

	// We should have transferred exactly one file even though the
	// files were identical.
	assert.Equal(t, int64(1), fs.Stats.GetTransfers())

	fstest.CheckItems(t, r.flocal, file1)
	fstest.CheckItems(t, r.fremote, file1)
}

func TestSyncIgnoreExisting(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("existing", "potato", t1)

	fs.Config.IgnoreExisting = true
	defer func() { fs.Config.IgnoreExisting = false }()

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)
	fstest.CheckItems(t, r.flocal, file1)
	fstest.CheckItems(t, r.fremote, file1)

	// Change everything
	r.WriteFile("existing", "newpotatoes", t2)
	fs.Stats.ResetCounters()
	err = fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)
	// Items should not change
	fstest.CheckItems(t, r.fremote, file1)
}

func TestSyncAfterChangingModtimeOnly(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("empty space", "", t2)
	file2 := r.WriteObject("empty space", "", t1)

	fstest.CheckItems(t, r.flocal, file1)
	fstest.CheckItems(t, r.fremote, file2)

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)

	fstest.CheckItems(t, r.flocal, file1)
	fstest.CheckItems(t, r.fremote, file1)
}

func TestSyncAfterChangingModtimeOnlyWithNoUpdateModTime(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	fs.Config.NoUpdateModTime = true
	defer func() {
		fs.Config.NoUpdateModTime = false
	}()

	file1 := r.WriteFile("empty space", "", t2)
	file2 := r.WriteObject("empty space", "", t1)

	fstest.CheckItems(t, r.flocal, file1)
	fstest.CheckItems(t, r.fremote, file2)

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)

	fstest.CheckItems(t, r.flocal, file1)
	fstest.CheckItems(t, r.fremote, file2)
}

func TestSyncDoesntUpdateModtime(t *testing.T) {
	if fs.Config.ModifyWindow == fs.ModTimeNotSupported {
		t.Skip("Can't run this test on fs which doesn't support mod time")
	}
	r := NewRun(t)
	defer r.Finalise()

	file1 := r.WriteFile("foo", "foo", t2)
	file2 := r.WriteObject("foo", "bar", t1)

	fstest.CheckItems(t, r.flocal, file1)
	fstest.CheckItems(t, r.fremote, file2)

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)

	fstest.CheckItems(t, r.flocal, file1)
	fstest.CheckItems(t, r.fremote, file1)

	// We should have transferred exactly one file, not set the mod time
	assert.Equal(t, int64(1), fs.Stats.GetTransfers())
}

func TestSyncAfterAddingAFile(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("empty space", "", t2)
	file2 := r.WriteFile("potato", "------------------------------------------------------------", t3)

	fstest.CheckItems(t, r.flocal, file1, file2)
	fstest.CheckItems(t, r.fremote, file1)

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)
	fstest.CheckItems(t, r.flocal, file1, file2)
	fstest.CheckItems(t, r.fremote, file1, file2)
}

func TestSyncAfterChangingFilesSizeOnly(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject("potato", "------------------------------------------------------------", t3)
	file2 := r.WriteFile("potato", "smaller but same date", t3)
	fstest.CheckItems(t, r.fremote, file1)
	fstest.CheckItems(t, r.flocal, file2)

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)
	fstest.CheckItems(t, r.flocal, file2)
	fstest.CheckItems(t, r.fremote, file2)
}

// Sync after changing a file's contents, changing modtime but length
// remaining the same
func TestSyncAfterChangingContentsOnly(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	var file1 fstest.Item
	if r.fremote.Precision() == fs.ModTimeNotSupported {
		t.Logf("ModTimeNotSupported so forcing file to be a different size")
		file1 = r.WriteObject("potato", "different size to make sure it syncs", t3)
	} else {
		file1 = r.WriteObject("potato", "smaller but same date", t3)
	}
	file2 := r.WriteFile("potato", "SMALLER BUT SAME DATE", t2)
	fstest.CheckItems(t, r.fremote, file1)
	fstest.CheckItems(t, r.flocal, file2)

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)
	fstest.CheckItems(t, r.flocal, file2)
	fstest.CheckItems(t, r.fremote, file2)
}

// Sync after removing a file and adding a file --dry-run
func TestSyncAfterRemovingAFileAndAddingAFileDryRun(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteObject("potato", "SMALLER BUT SAME DATE", t2)
	file3 := r.WriteBoth("empty space", "", t2)

	fs.Config.DryRun = true
	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	fs.Config.DryRun = false
	require.NoError(t, err)

	fstest.CheckItems(t, r.flocal, file3, file1)
	fstest.CheckItems(t, r.fremote, file3, file2)
}

// Sync after removing a file and adding a file
func TestSyncAfterRemovingAFileAndAddingAFile(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteObject("potato", "SMALLER BUT SAME DATE", t2)
	file3 := r.WriteBoth("empty space", "", t2)
	fstest.CheckItems(t, r.fremote, file2, file3)
	fstest.CheckItems(t, r.flocal, file1, file3)

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)
	fstest.CheckItems(t, r.flocal, file1, file3)
	fstest.CheckItems(t, r.fremote, file1, file3)
}

// Sync after removing a file and adding a file with IO Errors
func TestSyncAfterRemovingAFileAndAddingAFileWithErrors(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteObject("potato", "SMALLER BUT SAME DATE", t2)
	file3 := r.WriteBoth("empty space", "", t2)
	fstest.CheckItems(t, r.fremote, file2, file3)
	fstest.CheckItems(t, r.flocal, file1, file3)

	fs.Stats.ResetCounters()
	fs.Stats.Error()
	err := fs.Sync(r.fremote, r.flocal)
	assert.Equal(t, fs.ErrorNotDeleting, err)
	fstest.CheckItems(t, r.flocal, file1, file3)
	fstest.CheckItems(t, r.fremote, file1, file2, file3)
}

// Sync test delete during
func TestSyncDeleteDuring(t *testing.T) {
	// This is the default so we've checked this already
	// check it is the default
	if !(!fs.Config.DeleteBefore && fs.Config.DeleteDuring && !fs.Config.DeleteAfter) {
		t.Fatalf("Didn't default to --delete-during")
	}
}

// Sync test delete before
func TestSyncDeleteBefore(t *testing.T) {
	fs.Config.DeleteBefore = true
	fs.Config.DeleteDuring = false
	fs.Config.DeleteAfter = false
	defer func() {
		fs.Config.DeleteBefore = false
		fs.Config.DeleteDuring = true
		fs.Config.DeleteAfter = false
	}()

	TestSyncAfterRemovingAFileAndAddingAFile(t)
}

// Sync test delete after
func TestSyncDeleteAfter(t *testing.T) {
	fs.Config.DeleteBefore = false
	fs.Config.DeleteDuring = false
	fs.Config.DeleteAfter = true
	defer func() {
		fs.Config.DeleteBefore = false
		fs.Config.DeleteDuring = true
		fs.Config.DeleteAfter = false
	}()

	TestSyncAfterRemovingAFileAndAddingAFile(t)
}

// Test with exclude
func TestSyncWithExclude(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth("empty space", "", t2)
	file3 := r.WriteFile("enormous", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", t1) // 100 bytes
	fstest.CheckItems(t, r.fremote, file1, file2)
	fstest.CheckItems(t, r.flocal, file1, file2, file3)

	fs.Config.Filter.MaxSize = 40
	defer func() {
		fs.Config.Filter.MaxSize = -1
	}()

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)
	fstest.CheckItems(t, r.fremote, file2, file1)

	// Now sync the other way round and check enormous doesn't get
	// deleted as it is excluded from the sync
	fs.Stats.ResetCounters()
	err = fs.Sync(r.flocal, r.fremote)
	require.NoError(t, err)
	fstest.CheckItems(t, r.flocal, file2, file1, file3)
}

// Test with exclude and delete excluded
func TestSyncWithExcludeAndDeleteExcluded(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("potato2", "------------------------------------------------------------", t1) // 60 bytes
	file2 := r.WriteBoth("empty space", "", t2)
	file3 := r.WriteBoth("enormous", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", t1) // 100 bytes
	fstest.CheckItems(t, r.fremote, file1, file2, file3)
	fstest.CheckItems(t, r.flocal, file1, file2, file3)

	fs.Config.Filter.MaxSize = 40
	fs.Config.Filter.DeleteExcluded = true
	defer func() {
		fs.Config.Filter.MaxSize = -1
		fs.Config.Filter.DeleteExcluded = false
	}()

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)
	fstest.CheckItems(t, r.fremote, file2)

	// Check sync the other way round to make sure enormous gets
	// deleted even though it is excluded
	fs.Stats.ResetCounters()
	err = fs.Sync(r.flocal, r.fremote)
	require.NoError(t, err)
	fstest.CheckItems(t, r.flocal, file2)
}

// Test with UpdateOlder set
func TestSyncWithUpdateOlder(t *testing.T) {
	if fs.Config.ModifyWindow == fs.ModTimeNotSupported {
		t.Skip("Can't run this test on fs which doesn't support mod time")
	}
	r := NewRun(t)
	defer r.Finalise()
	t2plus := t2.Add(time.Second / 2)
	t2minus := t2.Add(time.Second / 2)
	oneF := r.WriteFile("one", "one", t1)
	twoF := r.WriteFile("two", "two", t3)
	threeF := r.WriteFile("three", "three", t2)
	fourF := r.WriteFile("four", "four", t2)
	fiveF := r.WriteFile("five", "five", t2)
	fstest.CheckItems(t, r.flocal, oneF, twoF, threeF, fourF, fiveF)
	oneO := r.WriteObject("one", "ONE", t2)
	twoO := r.WriteObject("two", "TWO", t2)
	threeO := r.WriteObject("three", "THREE", t2plus)
	fourO := r.WriteObject("four", "FOURFOUR", t2minus)
	fstest.CheckItems(t, r.fremote, oneO, twoO, threeO, fourO)

	fs.Config.UpdateOlder = true
	oldModifyWindow := fs.Config.ModifyWindow
	fs.Config.ModifyWindow = fs.ModTimeNotSupported
	defer func() {
		fs.Config.UpdateOlder = false
		fs.Config.ModifyWindow = oldModifyWindow
	}()

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	require.NoError(t, err)
	fstest.CheckItems(t, r.fremote, oneO, twoF, threeO, fourF, fiveF)
}

// Test with TrackRenames set
func TestSyncWithTrackRenames(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()

	fs.Config.TrackRenames = true
	defer func() {
		fs.Config.TrackRenames = false

	}()

	f1 := r.WriteFile("potato", "Potato Content", t1)
	f2 := r.WriteFile("yam", "Yam Content", t2)

	fs.Stats.ResetCounters()
	require.NoError(t, fs.Sync(r.fremote, r.flocal))

	fstest.CheckItems(t, r.fremote, f1, f2)
	fstest.CheckItems(t, r.flocal, f1, f2)

	// Now rename locally.
	f2 = r.RenameFile(f2, "yaml")

	fs.Stats.ResetCounters()
	require.NoError(t, fs.Sync(r.fremote, r.flocal))

	fstest.CheckItems(t, r.fremote, f1, f2)

}

// Test a server side move if possible, or the backup path if not
func testServerSideMove(t *testing.T, r *Run, fremoteMove fs.Fs, withFilter bool) {
	file1 := r.WriteBoth("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth("empty space", "", t2)
	file3u := r.WriteBoth("potato3", "------------------------------------------------------------ UPDATED", t2)

	fstest.CheckItems(t, r.fremote, file2, file1, file3u)

	t.Logf("Server side move (if possible) %v -> %v", r.fremote, fremoteMove)

	// Write just one file in the new remote
	r.WriteObjectTo(fremoteMove, "empty space", "", t2, false)
	file3 := r.WriteObjectTo(fremoteMove, "potato3", "------------------------------------------------------------", t1, false)
	fstest.CheckItems(t, fremoteMove, file2, file3)

	// Do server side move
	fs.Stats.ResetCounters()
	err := fs.MoveDir(fremoteMove, r.fremote)
	require.NoError(t, err)

	if withFilter {
		fstest.CheckItems(t, r.fremote, file2)
	} else {
		fstest.CheckItems(t, r.fremote)
	}
	fstest.CheckItems(t, fremoteMove, file2, file1, file3u)

	// Purge the original before moving
	require.NoError(t, fs.Purge(r.fremote))

	// Move it back again, dst does not exist this time
	fs.Stats.ResetCounters()
	err = fs.MoveDir(r.fremote, fremoteMove)
	require.NoError(t, err)

	if withFilter {
		fstest.CheckItems(t, r.fremote, file1, file3u)
		fstest.CheckItems(t, fremoteMove, file2)
	} else {
		fstest.CheckItems(t, r.fremote, file2, file1, file3u)
		fstest.CheckItems(t, fremoteMove)
	}
}

// Test a server side move if possible, or the backup path if not
func TestServerSideMove(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	fremoteMove, _, finaliseMove, err := fstest.RandomRemote(*RemoteName, *SubDir)
	require.NoError(t, err)
	defer finaliseMove()
	testServerSideMove(t, r, fremoteMove, false)
}

// Test a server side move if possible, or the backup path if not
func TestServerSideMoveWithFilter(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()

	fs.Config.Filter.MinSize = 40
	defer func() {
		fs.Config.Filter.MinSize = -1
	}()

	fremoteMove, _, finaliseMove, err := fstest.RandomRemote(*RemoteName, *SubDir)
	require.NoError(t, err)
	defer finaliseMove()
	testServerSideMove(t, r, fremoteMove, true)
}

// Test a server side move with overlap
func TestServerSideMoveOverlap(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()

	if _, ok := r.fremote.(fs.DirMover); ok {
		t.Skip("Skipping test as remote supports DirMove")
	}

	subRemoteName := r.fremoteName + "/rclone-move-test"
	fremoteMove, err := fs.NewFs(subRemoteName)
	require.NoError(t, err)

	file1 := r.WriteObject("potato2", "------------------------------------------------------------", t1)
	fstest.CheckItems(t, r.fremote, file1)

	// Subdir move with no filters should return ErrorCantMoveOverlapping
	err = fs.MoveDir(fremoteMove, r.fremote)
	assert.EqualError(t, err, fs.ErrorCantMoveOverlapping.Error())

	// Now try with a filter which should also fail with ErrorCantMoveOverlapping
	fs.Config.Filter.MinSize = 40
	defer func() {
		fs.Config.Filter.MinSize = -1
	}()
	err = fs.MoveDir(fremoteMove, r.fremote)
	assert.EqualError(t, err, fs.ErrorCantMoveOverlapping.Error())
}
