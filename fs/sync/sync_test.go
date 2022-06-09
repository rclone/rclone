// Test sync/copy/move

package sync

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/all" // import all backends
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/unicode/norm"
)

// Some times used in the tests
var (
	t1 = fstest.Time("2001-02-03T04:05:06.499999999Z")
	t2 = fstest.Time("2011-12-25T12:59:59.123456789Z")
	t3 = fstest.Time("2011-12-30T12:59:59.000000000Z")
)

// TestMain drives the tests
func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

// Check dry run is working
func TestCopyWithDryRun(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	r.Mkdir(ctx, r.Fremote)

	ci.DryRun = true
	err := CopyDir(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t)
}

// Now without dry run
func TestCopy(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	r.Mkdir(ctx, r.Fremote)

	err := CopyDir(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file1)
}

func TestCopyMissingDirectory(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	r.Mkdir(ctx, r.Fremote)

	nonExistingFs, err := fs.NewFs(ctx, "/non-existing")
	if err != nil {
		t.Fatal(err)
	}

	err = CopyDir(ctx, r.Fremote, nonExistingFs, false)
	require.Error(t, err)
}

// Now with --no-traverse
func TestCopyNoTraverse(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()

	ci.NoTraverse = true

	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)

	err := CopyDir(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file1)
}

// Now with --check-first
func TestCopyCheckFirst(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()

	ci.CheckFirst = true

	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)

	err := CopyDir(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file1)
}

// Now with --no-traverse
func TestSyncNoTraverse(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()

	ci.NoTraverse = true

	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)

	accounting.GlobalStats().ResetCounters()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file1)
}

// Test copy with depth
func TestCopyWithDepth(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	file2 := r.WriteFile("hello world2", "hello world2", t2)

	// Check the MaxDepth too
	ci.MaxDepth = 1

	err := CopyDir(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	r.CheckLocalItems(t, file1, file2)
	r.CheckRemoteItems(t, file2)
}

// Test copy with files from
func testCopyWithFilesFrom(t *testing.T, noTraverse bool) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("potato2", "hello world", t1)
	file2 := r.WriteFile("hello world2", "hello world2", t2)

	// Set the --files-from equivalent
	f, err := filter.NewFilter(nil)
	require.NoError(t, err)
	require.NoError(t, f.AddFile("potato2"))
	require.NoError(t, f.AddFile("notfound"))

	// Change the active filter
	ctx = filter.ReplaceConfig(ctx, f)

	ci.NoTraverse = noTraverse

	err = CopyDir(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	r.CheckLocalItems(t, file1, file2)
	r.CheckRemoteItems(t, file1)
}
func TestCopyWithFilesFrom(t *testing.T)              { testCopyWithFilesFrom(t, false) }
func TestCopyWithFilesFromAndNoTraverse(t *testing.T) { testCopyWithFilesFrom(t, true) }

// Test copy empty directories
func TestCopyEmptyDirectories(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	err := operations.Mkdir(ctx, r.Flocal, "sub dir2")
	require.NoError(t, err)
	r.Mkdir(ctx, r.Fremote)

	err = CopyDir(ctx, r.Fremote, r.Flocal, true)
	require.NoError(t, err)

	r.CheckRemoteListing(
		t,
		[]fstest.Item{
			file1,
		},
		[]string{
			"sub dir",
			"sub dir2",
		},
	)
}

// Test move empty directories
func TestMoveEmptyDirectories(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	err := operations.Mkdir(ctx, r.Flocal, "sub dir2")
	require.NoError(t, err)
	r.Mkdir(ctx, r.Fremote)

	err = MoveDir(ctx, r.Fremote, r.Flocal, false, true)
	require.NoError(t, err)

	r.CheckRemoteListing(
		t,
		[]fstest.Item{
			file1,
		},
		[]string{
			"sub dir",
			"sub dir2",
		},
	)
}

// Test sync empty directories
func TestSyncEmptyDirectories(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	err := operations.Mkdir(ctx, r.Flocal, "sub dir2")
	require.NoError(t, err)
	r.Mkdir(ctx, r.Fremote)

	err = Sync(ctx, r.Fremote, r.Flocal, true)
	require.NoError(t, err)

	r.CheckRemoteListing(
		t,
		[]fstest.Item{
			file1,
		},
		[]string{
			"sub dir",
			"sub dir2",
		},
	)
}

// Test a server-side copy if possible, or the backup path if not
func TestServerSideCopy(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject(ctx, "sub dir/hello world", "hello world", t1)
	r.CheckRemoteItems(t, file1)

	FremoteCopy, _, finaliseCopy, err := fstest.RandomRemote()
	require.NoError(t, err)
	defer finaliseCopy()
	t.Logf("Server side copy (if possible) %v -> %v", r.Fremote, FremoteCopy)

	err = CopyDir(ctx, FremoteCopy, r.Fremote, false)
	require.NoError(t, err)

	fstest.CheckItems(t, FremoteCopy, file1)
}

// Check that if the local file doesn't exist when we copy it up,
// nothing happens to the remote file
func TestCopyAfterDelete(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject(ctx, "sub dir/hello world", "hello world", t1)
	r.CheckLocalItems(t)
	r.CheckRemoteItems(t, file1)

	err := operations.Mkdir(ctx, r.Flocal, "")
	require.NoError(t, err)

	err = CopyDir(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	r.CheckLocalItems(t)
	r.CheckRemoteItems(t, file1)
}

// Check the copy downloading a file
func TestCopyRedownload(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject(ctx, "sub dir/hello world", "hello world", t1)
	r.CheckRemoteItems(t, file1)

	err := CopyDir(ctx, r.Flocal, r.Fremote, false)
	require.NoError(t, err)

	// Test with combined precision of local and remote as we copied it there and back
	r.CheckLocalListing(t, []fstest.Item{file1}, nil)
}

// Create a file and sync it. Change the last modified date and resync.
// If we're only doing sync by size and checksum, we expect nothing to
// to be transferred on the second sync.
func TestSyncBasedOnCheckSum(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()
	ci.CheckSum = true

	file1 := r.WriteFile("check sum", "-", t1)
	r.CheckLocalItems(t, file1)

	accounting.GlobalStats().ResetCounters()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	// We should have transferred exactly one file.
	assert.Equal(t, toyFileTransfers(r), accounting.GlobalStats().GetTransfers())
	r.CheckRemoteItems(t, file1)

	// Change last modified date only
	file2 := r.WriteFile("check sum", "-", t2)
	r.CheckLocalItems(t, file2)

	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	// We should have transferred no files
	assert.Equal(t, int64(0), accounting.GlobalStats().GetTransfers())
	r.CheckLocalItems(t, file2)
	r.CheckRemoteItems(t, file1)
}

// Create a file and sync it. Change the last modified date and the
// file contents but not the size.  If we're only doing sync by size
// only, we expect nothing to to be transferred on the second sync.
func TestSyncSizeOnly(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()
	ci.SizeOnly = true

	file1 := r.WriteFile("sizeonly", "potato", t1)
	r.CheckLocalItems(t, file1)

	accounting.GlobalStats().ResetCounters()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	// We should have transferred exactly one file.
	assert.Equal(t, toyFileTransfers(r), accounting.GlobalStats().GetTransfers())
	r.CheckRemoteItems(t, file1)

	// Update mtime, md5sum but not length of file
	file2 := r.WriteFile("sizeonly", "POTATO", t2)
	r.CheckLocalItems(t, file2)

	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	// We should have transferred no files
	assert.Equal(t, int64(0), accounting.GlobalStats().GetTransfers())
	r.CheckLocalItems(t, file2)
	r.CheckRemoteItems(t, file1)
}

// Create a file and sync it. Keep the last modified date but change
// the size.  With --ignore-size we expect nothing to to be
// transferred on the second sync.
func TestSyncIgnoreSize(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()
	ci.IgnoreSize = true

	file1 := r.WriteFile("ignore-size", "contents", t1)
	r.CheckLocalItems(t, file1)

	accounting.GlobalStats().ResetCounters()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	// We should have transferred exactly one file.
	assert.Equal(t, toyFileTransfers(r), accounting.GlobalStats().GetTransfers())
	r.CheckRemoteItems(t, file1)

	// Update size but not date of file
	file2 := r.WriteFile("ignore-size", "longer contents but same date", t1)
	r.CheckLocalItems(t, file2)

	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	// We should have transferred no files
	assert.Equal(t, int64(0), accounting.GlobalStats().GetTransfers())
	r.CheckLocalItems(t, file2)
	r.CheckRemoteItems(t, file1)
}

func TestSyncIgnoreTimes(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth(ctx, "existing", "potato", t1)
	r.CheckRemoteItems(t, file1)

	accounting.GlobalStats().ResetCounters()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	// We should have transferred exactly 0 files because the
	// files were identical.
	assert.Equal(t, int64(0), accounting.GlobalStats().GetTransfers())

	ci.IgnoreTimes = true

	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	// We should have transferred exactly one file even though the
	// files were identical.
	assert.Equal(t, toyFileTransfers(r), accounting.GlobalStats().GetTransfers())

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file1)
}

func TestSyncIgnoreExisting(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("existing", "potato", t1)

	ci.IgnoreExisting = true

	accounting.GlobalStats().ResetCounters()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file1)

	// Change everything
	r.WriteFile("existing", "newpotatoes", t2)
	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	// Items should not change
	r.CheckRemoteItems(t, file1)
}

func TestSyncIgnoreErrors(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	ci.IgnoreErrors = true
	defer r.Finalise()
	file1 := r.WriteFile("a/potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteObject(ctx, "b/potato", "SMALLER BUT SAME DATE", t2)
	file3 := r.WriteBoth(ctx, "c/non empty space", "AhHa!", t2)
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "d"))

	r.CheckLocalListing(
		t,
		[]fstest.Item{
			file1,
			file3,
		},
		[]string{
			"a",
			"c",
		},
	)
	r.CheckRemoteListing(
		t,
		[]fstest.Item{
			file2,
			file3,
		},
		[]string{
			"b",
			"c",
			"d",
		},
	)

	accounting.GlobalStats().ResetCounters()
	_ = fs.CountError(errors.New("boom"))
	assert.NoError(t, Sync(ctx, r.Fremote, r.Flocal, false))

	r.CheckLocalListing(
		t,
		[]fstest.Item{
			file1,
			file3,
		},
		[]string{
			"a",
			"c",
		},
	)
	r.CheckRemoteListing(
		t,
		[]fstest.Item{
			file1,
			file3,
		},
		[]string{
			"a",
			"c",
		},
	)
}

func TestSyncAfterChangingModtimeOnly(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("empty space", "-", t2)
	file2 := r.WriteObject(ctx, "empty space", "-", t1)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file2)

	ci.DryRun = true

	accounting.GlobalStats().ResetCounters()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file2)

	ci.DryRun = false

	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file1)
}

func TestSyncAfterChangingModtimeOnlyWithNoUpdateModTime(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()

	if r.Fremote.Hashes().Count() == 0 {
		t.Logf("Can't check this if no hashes supported")
		return
	}

	ci.NoUpdateModTime = true

	file1 := r.WriteFile("empty space", "-", t2)
	file2 := r.WriteObject(ctx, "empty space", "-", t1)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file2)

	accounting.GlobalStats().ResetCounters()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file2)
}

func TestSyncDoesntUpdateModtime(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	if fs.GetModifyWindow(ctx, r.Fremote) == fs.ModTimeNotSupported {
		t.Skip("Can't run this test on fs which doesn't support mod time")
	}

	file1 := r.WriteFile("foo", "foo", t2)
	file2 := r.WriteObject(ctx, "foo", "bar", t1)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file2)

	accounting.GlobalStats().ResetCounters()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file1)

	// We should have transferred exactly one file, not set the mod time
	assert.Equal(t, toyFileTransfers(r), accounting.GlobalStats().GetTransfers())
}

func TestSyncAfterAddingAFile(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth(ctx, "empty space", "-", t2)
	file2 := r.WriteFile("potato", "------------------------------------------------------------", t3)

	r.CheckLocalItems(t, file1, file2)
	r.CheckRemoteItems(t, file1)

	accounting.GlobalStats().ResetCounters()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	r.CheckLocalItems(t, file1, file2)
	r.CheckRemoteItems(t, file1, file2)
}

func TestSyncAfterChangingFilesSizeOnly(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject(ctx, "potato", "------------------------------------------------------------", t3)
	file2 := r.WriteFile("potato", "smaller but same date", t3)
	r.CheckRemoteItems(t, file1)
	r.CheckLocalItems(t, file2)

	accounting.GlobalStats().ResetCounters()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	r.CheckLocalItems(t, file2)
	r.CheckRemoteItems(t, file2)
}

// Sync after changing a file's contents, changing modtime but length
// remaining the same
func TestSyncAfterChangingContentsOnly(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	var file1 fstest.Item
	if r.Fremote.Precision() == fs.ModTimeNotSupported {
		t.Logf("ModTimeNotSupported so forcing file to be a different size")
		file1 = r.WriteObject(ctx, "potato", "different size to make sure it syncs", t3)
	} else {
		file1 = r.WriteObject(ctx, "potato", "smaller but same date", t3)
	}
	file2 := r.WriteFile("potato", "SMALLER BUT SAME DATE", t2)
	r.CheckRemoteItems(t, file1)
	r.CheckLocalItems(t, file2)

	accounting.GlobalStats().ResetCounters()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	r.CheckLocalItems(t, file2)
	r.CheckRemoteItems(t, file2)
}

// Sync after removing a file and adding a file --dry-run
func TestSyncAfterRemovingAFileAndAddingAFileDryRun(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteObject(ctx, "potato", "SMALLER BUT SAME DATE", t2)
	file3 := r.WriteBoth(ctx, "empty space", "-", t2)

	ci.DryRun = true
	accounting.GlobalStats().ResetCounters()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	ci.DryRun = false
	require.NoError(t, err)

	r.CheckLocalItems(t, file3, file1)
	r.CheckRemoteItems(t, file3, file2)
}

// Sync after removing a file and adding a file
func testSyncAfterRemovingAFileAndAddingAFile(ctx context.Context, t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteObject(ctx, "potato", "SMALLER BUT SAME DATE", t2)
	file3 := r.WriteBoth(ctx, "empty space", "-", t2)
	r.CheckRemoteItems(t, file2, file3)
	r.CheckLocalItems(t, file1, file3)

	accounting.GlobalStats().ResetCounters()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	r.CheckLocalItems(t, file1, file3)
	r.CheckRemoteItems(t, file1, file3)
}

func TestSyncAfterRemovingAFileAndAddingAFile(t *testing.T) {
	testSyncAfterRemovingAFileAndAddingAFile(context.Background(), t)
}

// Sync after removing a file and adding a file
func testSyncAfterRemovingAFileAndAddingAFileSubDir(ctx context.Context, t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("a/potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteObject(ctx, "b/potato", "SMALLER BUT SAME DATE", t2)
	file3 := r.WriteBoth(ctx, "c/non empty space", "AhHa!", t2)
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "d"))
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "d/e"))

	r.CheckLocalListing(
		t,
		[]fstest.Item{
			file1,
			file3,
		},
		[]string{
			"a",
			"c",
		},
	)
	r.CheckRemoteListing(
		t,
		[]fstest.Item{
			file2,
			file3,
		},
		[]string{
			"b",
			"c",
			"d",
			"d/e",
		},
	)

	accounting.GlobalStats().ResetCounters()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	r.CheckLocalListing(
		t,
		[]fstest.Item{
			file1,
			file3,
		},
		[]string{
			"a",
			"c",
		},
	)
	r.CheckRemoteListing(
		t,
		[]fstest.Item{
			file1,
			file3,
		},
		[]string{
			"a",
			"c",
		},
	)
}

func TestSyncAfterRemovingAFileAndAddingAFileSubDir(t *testing.T) {
	testSyncAfterRemovingAFileAndAddingAFileSubDir(context.Background(), t)
}

// Sync after removing a file and adding a file with IO Errors
func TestSyncAfterRemovingAFileAndAddingAFileSubDirWithErrors(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("a/potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteObject(ctx, "b/potato", "SMALLER BUT SAME DATE", t2)
	file3 := r.WriteBoth(ctx, "c/non empty space", "AhHa!", t2)
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "d"))

	r.CheckLocalListing(
		t,
		[]fstest.Item{
			file1,
			file3,
		},
		[]string{
			"a",
			"c",
		},
	)
	r.CheckRemoteListing(
		t,
		[]fstest.Item{
			file2,
			file3,
		},
		[]string{
			"b",
			"c",
			"d",
		},
	)

	accounting.GlobalStats().ResetCounters()
	_ = fs.CountError(errors.New("boom"))
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	assert.Equal(t, fs.ErrorNotDeleting, err)

	r.CheckLocalListing(
		t,
		[]fstest.Item{
			file1,
			file3,
		},
		[]string{
			"a",
			"c",
		},
	)
	r.CheckRemoteListing(
		t,
		[]fstest.Item{
			file1,
			file2,
			file3,
		},
		[]string{
			"a",
			"b",
			"c",
			"d",
		},
	)
}

// Sync test delete after
func TestSyncDeleteAfter(t *testing.T) {
	ctx := context.Background()
	ci := fs.GetConfig(ctx)
	// This is the default so we've checked this already
	// check it is the default
	require.Equal(t, ci.DeleteMode, fs.DeleteModeAfter, "Didn't default to --delete-after")
}

// Sync test delete during
func TestSyncDeleteDuring(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	ci.DeleteMode = fs.DeleteModeDuring

	testSyncAfterRemovingAFileAndAddingAFile(ctx, t)
}

// Sync test delete before
func TestSyncDeleteBefore(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	ci.DeleteMode = fs.DeleteModeBefore

	testSyncAfterRemovingAFileAndAddingAFile(ctx, t)
}

// Copy test delete before - shouldn't delete anything
func TestCopyDeleteBefore(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()

	ci.DeleteMode = fs.DeleteModeBefore

	file1 := r.WriteObject(ctx, "potato", "hopefully not deleted", t1)
	file2 := r.WriteFile("potato2", "hopefully copied in", t1)
	r.CheckRemoteItems(t, file1)
	r.CheckLocalItems(t, file2)

	accounting.GlobalStats().ResetCounters()
	err := CopyDir(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	r.CheckRemoteItems(t, file1, file2)
	r.CheckLocalItems(t, file2)
}

// Test with exclude
func TestSyncWithExclude(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth(ctx, "potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth(ctx, "empty space", "-", t2)
	file3 := r.WriteFile("enormous", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", t1) // 100 bytes
	r.CheckRemoteItems(t, file1, file2)
	r.CheckLocalItems(t, file1, file2, file3)

	fi, err := filter.NewFilter(nil)
	require.NoError(t, err)
	fi.Opt.MaxSize = 40
	ctx = filter.ReplaceConfig(ctx, fi)

	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	r.CheckRemoteItems(t, file2, file1)

	// Now sync the other way round and check enormous doesn't get
	// deleted as it is excluded from the sync
	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, r.Flocal, r.Fremote, false)
	require.NoError(t, err)
	r.CheckLocalItems(t, file2, file1, file3)
}

// Test with exclude and delete excluded
func TestSyncWithExcludeAndDeleteExcluded(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth(ctx, "potato2", "------------------------------------------------------------", t1) // 60 bytes
	file2 := r.WriteBoth(ctx, "empty space", "-", t2)
	file3 := r.WriteBoth(ctx, "enormous", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", t1) // 100 bytes
	r.CheckRemoteItems(t, file1, file2, file3)
	r.CheckLocalItems(t, file1, file2, file3)

	fi, err := filter.NewFilter(nil)
	require.NoError(t, err)
	fi.Opt.MaxSize = 40
	fi.Opt.DeleteExcluded = true
	ctx = filter.ReplaceConfig(ctx, fi)

	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	r.CheckRemoteItems(t, file2)

	// Check sync the other way round to make sure enormous gets
	// deleted even though it is excluded
	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, r.Flocal, r.Fremote, false)
	require.NoError(t, err)
	r.CheckLocalItems(t, file2)
}

// Test with UpdateOlder set
func TestSyncWithUpdateOlder(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()
	if fs.GetModifyWindow(ctx, r.Fremote) == fs.ModTimeNotSupported {
		t.Skip("Can't run this test on fs which doesn't support mod time")
	}
	t2plus := t2.Add(time.Second / 2)
	t2minus := t2.Add(time.Second / 2)
	oneF := r.WriteFile("one", "one", t1)
	twoF := r.WriteFile("two", "two", t3)
	threeF := r.WriteFile("three", "three", t2)
	fourF := r.WriteFile("four", "four", t2)
	fiveF := r.WriteFile("five", "five", t2)
	r.CheckLocalItems(t, oneF, twoF, threeF, fourF, fiveF)
	oneO := r.WriteObject(ctx, "one", "ONE", t2)
	twoO := r.WriteObject(ctx, "two", "TWO", t2)
	threeO := r.WriteObject(ctx, "three", "THREE", t2plus)
	fourO := r.WriteObject(ctx, "four", "FOURFOUR", t2minus)
	r.CheckRemoteItems(t, oneO, twoO, threeO, fourO)

	ci.UpdateOlder = true
	ci.ModifyWindow = fs.ModTimeNotSupported

	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	r.CheckRemoteItems(t, oneO, twoF, threeO, fourF, fiveF)

	if r.Fremote.Hashes().Count() == 0 {
		t.Logf("Skip test with --checksum as no hashes supported")
		return
	}

	// now enable checksum
	ci.CheckSum = true

	err = Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	r.CheckRemoteItems(t, oneO, twoF, threeF, fourF, fiveF)
}

// Test with a max transfer duration
func TestSyncWithMaxDuration(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	if *fstest.RemoteName != "" {
		t.Skip("Skipping test on non local remote")
	}
	r := fstest.NewRun(t)
	defer r.Finalise()

	maxDuration := 250 * time.Millisecond
	ci.MaxDuration = maxDuration
	bytesPerSecond := 300
	accounting.TokenBucket.SetBwLimit(fs.BwPair{Tx: fs.SizeSuffix(bytesPerSecond), Rx: fs.SizeSuffix(bytesPerSecond)})
	ci.Transfers = 1
	defer accounting.TokenBucket.SetBwLimit(fs.BwPair{Tx: -1, Rx: -1})

	// 5 files of 60 bytes at 60 Byte/s 5 seconds
	testFiles := make([]fstest.Item, 5)
	for i := 0; i < len(testFiles); i++ {
		testFiles[i] = r.WriteFile(fmt.Sprintf("file%d", i), "------------------------------------------------------------", t1)
	}

	fstest.CheckListing(t, r.Flocal, testFiles)

	accounting.GlobalStats().ResetCounters()
	startTime := time.Now()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.True(t, errors.Is(err, errorMaxDurationReached))

	elapsed := time.Since(startTime)
	maxTransferTime := (time.Duration(len(testFiles)) * 60 * time.Second) / time.Duration(bytesPerSecond)

	what := fmt.Sprintf("expecting elapsed time %v between %v and %v", elapsed, maxDuration, maxTransferTime)
	require.True(t, elapsed >= maxDuration, what)
	require.True(t, elapsed < 5*time.Second, what)
	// we must not have transferred all files during the session
	require.True(t, accounting.GlobalStats().GetTransfers() < int64(len(testFiles)))
}

// Test with TrackRenames set
func TestSyncWithTrackRenames(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()

	ci.TrackRenames = true
	defer func() {
		ci.TrackRenames = false
	}()

	haveHash := r.Fremote.Hashes().Overlap(r.Flocal.Hashes()).GetOne() != hash.None
	canTrackRenames := haveHash && operations.CanServerSideMove(r.Fremote)
	t.Logf("Can track renames: %v", canTrackRenames)

	f1 := r.WriteFile("potato", "Potato Content", t1)
	f2 := r.WriteFile("yam", "Yam Content", t2)

	accounting.GlobalStats().ResetCounters()
	require.NoError(t, Sync(ctx, r.Fremote, r.Flocal, false))

	r.CheckRemoteItems(t, f1, f2)
	r.CheckLocalItems(t, f1, f2)

	// Now rename locally.
	f2 = r.RenameFile(f2, "yaml")

	accounting.GlobalStats().ResetCounters()
	require.NoError(t, Sync(ctx, r.Fremote, r.Flocal, false))

	r.CheckRemoteItems(t, f1, f2)

	// Check we renamed something if we should have
	if canTrackRenames {
		renames := accounting.GlobalStats().Renames(0)
		assert.Equal(t, canTrackRenames, renames != 0, fmt.Sprintf("canTrackRenames=%v, renames=%d", canTrackRenames, renames))
	}
}

func TestParseRenamesStrategyModtime(t *testing.T) {
	for _, test := range []struct {
		in      string
		want    trackRenamesStrategy
		wantErr bool
	}{
		{"", 0, false},
		{"modtime", trackRenamesStrategyModtime, false},
		{"hash", trackRenamesStrategyHash, false},
		{"size", 0, false},
		{"modtime,hash", trackRenamesStrategyModtime | trackRenamesStrategyHash, false},
		{"hash,modtime,size", trackRenamesStrategyModtime | trackRenamesStrategyHash, false},
		{"size,boom", 0, true},
	} {
		got, err := parseTrackRenamesStrategy(test.in)
		assert.Equal(t, test.want, got, test.in)
		assert.Equal(t, test.wantErr, err != nil, test.in)
	}
}

func TestRenamesStrategyModtime(t *testing.T) {
	both := trackRenamesStrategyHash | trackRenamesStrategyModtime
	hash := trackRenamesStrategyHash
	modTime := trackRenamesStrategyModtime

	assert.True(t, both.hash())
	assert.True(t, both.modTime())
	assert.True(t, hash.hash())
	assert.False(t, hash.modTime())
	assert.False(t, modTime.hash())
	assert.True(t, modTime.modTime())
}

func TestSyncWithTrackRenamesStrategyModtime(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()

	ci.TrackRenames = true
	ci.TrackRenamesStrategy = "modtime"

	canTrackRenames := operations.CanServerSideMove(r.Fremote) && r.Fremote.Precision() != fs.ModTimeNotSupported
	t.Logf("Can track renames: %v", canTrackRenames)

	f1 := r.WriteFile("potato", "Potato Content", t1)
	f2 := r.WriteFile("yam", "Yam Content", t2)

	accounting.GlobalStats().ResetCounters()
	require.NoError(t, Sync(ctx, r.Fremote, r.Flocal, false))

	r.CheckRemoteItems(t, f1, f2)
	r.CheckLocalItems(t, f1, f2)

	// Now rename locally.
	f2 = r.RenameFile(f2, "yaml")

	accounting.GlobalStats().ResetCounters()
	require.NoError(t, Sync(ctx, r.Fremote, r.Flocal, false))

	r.CheckRemoteItems(t, f1, f2)

	// Check we renamed something if we should have
	if canTrackRenames {
		renames := accounting.GlobalStats().Renames(0)
		assert.Equal(t, canTrackRenames, renames != 0, fmt.Sprintf("canTrackRenames=%v, renames=%d", canTrackRenames, renames))
	}
}

func TestSyncWithTrackRenamesStrategyLeaf(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()

	ci.TrackRenames = true
	ci.TrackRenamesStrategy = "leaf"

	canTrackRenames := operations.CanServerSideMove(r.Fremote) && r.Fremote.Precision() != fs.ModTimeNotSupported
	t.Logf("Can track renames: %v", canTrackRenames)

	f1 := r.WriteFile("potato", "Potato Content", t1)
	f2 := r.WriteFile("sub/yam", "Yam Content", t2)

	accounting.GlobalStats().ResetCounters()
	require.NoError(t, Sync(ctx, r.Fremote, r.Flocal, false))

	r.CheckRemoteItems(t, f1, f2)
	r.CheckLocalItems(t, f1, f2)

	// Now rename locally.
	f2 = r.RenameFile(f2, "yam")

	accounting.GlobalStats().ResetCounters()
	require.NoError(t, Sync(ctx, r.Fremote, r.Flocal, false))

	r.CheckRemoteItems(t, f1, f2)

	// Check we renamed something if we should have
	if canTrackRenames {
		renames := accounting.GlobalStats().Renames(0)
		assert.Equal(t, canTrackRenames, renames != 0, fmt.Sprintf("canTrackRenames=%v, renames=%d", canTrackRenames, renames))
	}
}

func toyFileTransfers(r *fstest.Run) int64 {
	remote := r.Fremote.Name()
	transfers := 1
	if strings.HasPrefix(remote, "TestChunker") && strings.HasSuffix(remote, "S3") {
		transfers++ // Extra Copy because S3 emulates Move as Copy+Delete.
	}
	return int64(transfers)
}

// Test a server-side move if possible, or the backup path if not
func testServerSideMove(ctx context.Context, t *testing.T, r *fstest.Run, withFilter, testDeleteEmptyDirs bool) {
	FremoteMove, _, finaliseMove, err := fstest.RandomRemote()
	require.NoError(t, err)
	defer finaliseMove()

	file1 := r.WriteBoth(ctx, "potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth(ctx, "empty space", "-", t2)
	file3u := r.WriteBoth(ctx, "potato3", "------------------------------------------------------------ UPDATED", t2)

	if testDeleteEmptyDirs {
		err := operations.Mkdir(ctx, r.Fremote, "tomatoDir")
		require.NoError(t, err)
	}

	r.CheckRemoteItems(t, file2, file1, file3u)

	t.Logf("Server side move (if possible) %v -> %v", r.Fremote, FremoteMove)

	// Write just one file in the new remote
	r.WriteObjectTo(ctx, FremoteMove, "empty space", "-", t2, false)
	file3 := r.WriteObjectTo(ctx, FremoteMove, "potato3", "------------------------------------------------------------", t1, false)
	fstest.CheckItems(t, FremoteMove, file2, file3)

	// Do server-side move
	accounting.GlobalStats().ResetCounters()
	err = MoveDir(ctx, FremoteMove, r.Fremote, testDeleteEmptyDirs, false)
	require.NoError(t, err)

	if withFilter {
		r.CheckRemoteItems(t, file2)
	} else {
		r.CheckRemoteItems(t)
	}

	if testDeleteEmptyDirs {
		r.CheckRemoteListing(t, nil, []string{})
	}

	fstest.CheckItems(t, FremoteMove, file2, file1, file3u)

	// Create a new empty remote for stuff to be moved into
	FremoteMove2, _, finaliseMove2, err := fstest.RandomRemote()
	require.NoError(t, err)
	defer finaliseMove2()

	if testDeleteEmptyDirs {
		err := operations.Mkdir(ctx, FremoteMove, "tomatoDir")
		require.NoError(t, err)
	}

	// Move it back to a new empty remote, dst does not exist this time
	accounting.GlobalStats().ResetCounters()
	err = MoveDir(ctx, FremoteMove2, FremoteMove, testDeleteEmptyDirs, false)
	require.NoError(t, err)

	if withFilter {
		fstest.CheckItems(t, FremoteMove2, file1, file3u)
		fstest.CheckItems(t, FremoteMove, file2)
	} else {
		fstest.CheckItems(t, FremoteMove2, file2, file1, file3u)
		fstest.CheckItems(t, FremoteMove)
	}

	if testDeleteEmptyDirs {
		fstest.CheckListingWithPrecision(t, FremoteMove, nil, []string{}, fs.GetModifyWindow(ctx, r.Fremote))
	}
}

// Test move
func TestMoveWithDeleteEmptySrcDirs(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	file2 := r.WriteFile("nested/sub dir/file", "nested", t1)
	r.Mkdir(ctx, r.Fremote)

	// run move with --delete-empty-src-dirs
	err := MoveDir(ctx, r.Fremote, r.Flocal, true, false)
	require.NoError(t, err)

	r.CheckLocalListing(
		t,
		nil,
		[]string{},
	)
	r.CheckRemoteItems(t, file1, file2)
}

func TestMoveWithoutDeleteEmptySrcDirs(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	file2 := r.WriteFile("nested/sub dir/file", "nested", t1)
	r.Mkdir(ctx, r.Fremote)

	err := MoveDir(ctx, r.Fremote, r.Flocal, false, false)
	require.NoError(t, err)

	r.CheckLocalListing(
		t,
		nil,
		[]string{
			"sub dir",
			"nested",
			"nested/sub dir",
		},
	)
	r.CheckRemoteItems(t, file1, file2)
}

func TestMoveWithIgnoreExisting(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("existing", "potato", t1)
	file2 := r.WriteFile("existing-b", "tomato", t1)

	ci.IgnoreExisting = true

	accounting.GlobalStats().ResetCounters()
	err := MoveDir(ctx, r.Fremote, r.Flocal, false, false)
	require.NoError(t, err)
	r.CheckLocalListing(
		t,
		[]fstest.Item{},
		[]string{},
	)
	r.CheckRemoteListing(
		t,
		[]fstest.Item{
			file1,
			file2,
		},
		[]string{},
	)

	// Recreate first file with modified content
	file1b := r.WriteFile("existing", "newpotatoes", t2)
	accounting.GlobalStats().ResetCounters()
	err = MoveDir(ctx, r.Fremote, r.Flocal, false, false)
	require.NoError(t, err)
	// Source items should still exist in modified state
	r.CheckLocalListing(
		t,
		[]fstest.Item{
			file1b,
		},
		[]string{},
	)
	// Dest items should not have changed
	r.CheckRemoteListing(
		t,
		[]fstest.Item{
			file1,
			file2,
		},
		[]string{},
	)
}

// Test a server-side move if possible, or the backup path if not
func TestServerSideMove(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	testServerSideMove(ctx, t, r, false, false)
}

// Test a server-side move if possible, or the backup path if not
func TestServerSideMoveWithFilter(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()

	fi, err := filter.NewFilter(nil)
	require.NoError(t, err)
	fi.Opt.MinSize = 40
	ctx = filter.ReplaceConfig(ctx, fi)

	testServerSideMove(ctx, t, r, true, false)
}

// Test a server-side move if possible
func TestServerSideMoveDeleteEmptySourceDirs(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	testServerSideMove(ctx, t, r, false, true)
}

// Test a server-side move with overlap
func TestServerSideMoveOverlap(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()

	if r.Fremote.Features().DirMove != nil {
		t.Skip("Skipping test as remote supports DirMove")
	}

	subRemoteName := r.FremoteName + "/rclone-move-test"
	FremoteMove, err := fs.NewFs(ctx, subRemoteName)
	require.NoError(t, err)

	file1 := r.WriteObject(ctx, "potato2", "------------------------------------------------------------", t1)
	r.CheckRemoteItems(t, file1)

	// Subdir move with no filters should return ErrorCantMoveOverlapping
	err = MoveDir(ctx, FremoteMove, r.Fremote, false, false)
	assert.EqualError(t, err, fs.ErrorOverlapping.Error())

	// Now try with a filter which should also fail with ErrorCantMoveOverlapping
	fi, err := filter.NewFilter(nil)
	require.NoError(t, err)
	fi.Opt.MinSize = 40
	ctx = filter.ReplaceConfig(ctx, fi)

	err = MoveDir(ctx, FremoteMove, r.Fremote, false, false)
	assert.EqualError(t, err, fs.ErrorOverlapping.Error())
}

// Test a sync with overlap
func TestSyncOverlap(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()

	subRemoteName := r.FremoteName + "/rclone-sync-test"
	FremoteSync, err := fs.NewFs(ctx, subRemoteName)
	require.NoError(t, err)

	checkErr := func(err error) {
		require.Error(t, err)
		assert.True(t, fserrors.IsFatalError(err))
		assert.Equal(t, fs.ErrorOverlapping.Error(), err.Error())
	}

	checkErr(Sync(ctx, FremoteSync, r.Fremote, false))
	checkErr(Sync(ctx, r.Fremote, FremoteSync, false))
	checkErr(Sync(ctx, r.Fremote, r.Fremote, false))
	checkErr(Sync(ctx, FremoteSync, FremoteSync, false))
}

// Test a sync with filtered overlap
func TestSyncOverlapWithFilter(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()

	fi, err := filter.NewFilter(nil)
	require.NoError(t, err)
	require.NoError(t, fi.Add(false, "/rclone-sync-test/"))
	require.NoError(t, fi.Add(false, "*/layer2/"))
	fi.Opt.ExcludeFile = []string{".ignore"}
	ctx = filter.ReplaceConfig(ctx, fi)

	subRemoteName := r.FremoteName + "/rclone-sync-test"
	FremoteSync, err := fs.NewFs(ctx, subRemoteName)
	require.NoError(t, FremoteSync.Mkdir(ctx, ""))
	require.NoError(t, err)

	subRemoteName2 := r.FremoteName + "/rclone-sync-test-include/layer2"
	FremoteSync2, err := fs.NewFs(ctx, subRemoteName2)
	require.NoError(t, FremoteSync2.Mkdir(ctx, ""))
	require.NoError(t, err)

	subRemoteName3 := r.FremoteName + "/rclone-sync-test-ignore-file"
	FremoteSync3, err := fs.NewFs(ctx, subRemoteName3)
	require.NoError(t, FremoteSync3.Mkdir(ctx, ""))
	require.NoError(t, err)
	r.WriteObject(context.Background(), "rclone-sync-test-ignore-file/.ignore", "-", t1)

	checkErr := func(err error) {
		require.Error(t, err)
		assert.True(t, fserrors.IsFatalError(err))
		assert.Equal(t, fs.ErrorOverlapping.Error(), err.Error())
		accounting.GlobalStats().ResetCounters()
	}

	checkNoErr := func(err error) {
		require.NoError(t, err)
	}

	accounting.GlobalStats().ResetCounters()
	checkNoErr(Sync(ctx, FremoteSync, r.Fremote, false))
	checkErr(Sync(ctx, r.Fremote, FremoteSync, false))
	checkErr(Sync(ctx, r.Fremote, r.Fremote, false))
	checkErr(Sync(ctx, FremoteSync, FremoteSync, false))

	checkNoErr(Sync(ctx, FremoteSync2, r.Fremote, false))
	checkErr(Sync(ctx, r.Fremote, FremoteSync2, false))
	checkErr(Sync(ctx, r.Fremote, r.Fremote, false))
	checkErr(Sync(ctx, FremoteSync2, FremoteSync2, false))

	checkNoErr(Sync(ctx, FremoteSync3, r.Fremote, false))
	checkErr(Sync(ctx, r.Fremote, FremoteSync3, false))
	checkErr(Sync(ctx, r.Fremote, r.Fremote, false))
	checkErr(Sync(ctx, FremoteSync3, FremoteSync3, false))
}

// Test with CompareDest set
func TestSyncCompareDest(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()

	ci.CompareDest = []string{r.FremoteName + "/CompareDest"}

	fdst, err := fs.NewFs(ctx, r.FremoteName+"/dst")
	require.NoError(t, err)

	// check empty dest, empty compare
	file1 := r.WriteFile("one", "one", t1)
	r.CheckLocalItems(t, file1)

	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, fdst, r.Flocal, false)
	require.NoError(t, err)

	file1dst := file1
	file1dst.Path = "dst/one"

	r.CheckRemoteItems(t, file1dst)

	// check old dest, empty compare
	file1b := r.WriteFile("one", "onet2", t2)
	r.CheckRemoteItems(t, file1dst)
	r.CheckLocalItems(t, file1b)

	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, fdst, r.Flocal, false)
	require.NoError(t, err)

	file1bdst := file1b
	file1bdst.Path = "dst/one"

	r.CheckRemoteItems(t, file1bdst)

	// check old dest, new compare
	file3 := r.WriteObject(ctx, "dst/one", "one", t1)
	file2 := r.WriteObject(ctx, "CompareDest/one", "onet2", t2)
	file1c := r.WriteFile("one", "onet2", t2)
	r.CheckRemoteItems(t, file2, file3)
	r.CheckLocalItems(t, file1c)

	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, fdst, r.Flocal, false)
	require.NoError(t, err)

	r.CheckRemoteItems(t, file2, file3)

	// check empty dest, new compare
	file4 := r.WriteObject(ctx, "CompareDest/two", "two", t2)
	file5 := r.WriteFile("two", "two", t2)
	r.CheckRemoteItems(t, file2, file3, file4)
	r.CheckLocalItems(t, file1c, file5)

	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, fdst, r.Flocal, false)
	require.NoError(t, err)

	r.CheckRemoteItems(t, file2, file3, file4)

	// check new dest, new compare
	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, fdst, r.Flocal, false)
	require.NoError(t, err)

	r.CheckRemoteItems(t, file2, file3, file4)

	// Work out if we actually have hashes for uploaded files
	haveHash := false
	if ht := fdst.Hashes().GetOne(); ht != hash.None {
		file2obj, err := fdst.NewObject(ctx, "one")
		if err == nil {
			file2objHash, err := file2obj.Hash(ctx, ht)
			if err == nil {
				haveHash = file2objHash != ""
			}
		}
	}

	// check new dest, new compare, src timestamp differs
	//
	// we only check this if we the file we uploaded previously
	// actually has a hash otherwise the differing timestamp is
	// always copied.
	if haveHash {
		file5b := r.WriteFile("two", "two", t3)
		r.CheckLocalItems(t, file1c, file5b)

		accounting.GlobalStats().ResetCounters()
		err = Sync(ctx, fdst, r.Flocal, false)
		require.NoError(t, err)

		r.CheckRemoteItems(t, file2, file3, file4)
	} else {
		t.Log("No hash on uploaded file so skipping compare timestamp test")
	}

	// check empty dest, old compare
	file5c := r.WriteFile("two", "twot3", t3)
	r.CheckRemoteItems(t, file2, file3, file4)
	r.CheckLocalItems(t, file1c, file5c)

	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, fdst, r.Flocal, false)
	require.NoError(t, err)

	file5cdst := file5c
	file5cdst.Path = "dst/two"

	r.CheckRemoteItems(t, file2, file3, file4, file5cdst)
}

// Test with multiple CompareDest
func TestSyncMultipleCompareDest(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()
	precision := fs.GetModifyWindow(ctx, r.Fremote, r.Flocal)

	ci.CompareDest = []string{r.FremoteName + "/pre-dest1", r.FremoteName + "/pre-dest2"}

	// check empty dest, new compare
	fsrc1 := r.WriteFile("1", "1", t1)
	fsrc2 := r.WriteFile("2", "2", t1)
	fsrc3 := r.WriteFile("3", "3", t1)
	r.CheckLocalItems(t, fsrc1, fsrc2, fsrc3)

	fdest1 := r.WriteObject(ctx, "pre-dest1/1", "1", t1)
	fdest2 := r.WriteObject(ctx, "pre-dest2/2", "2", t1)
	r.CheckRemoteItems(t, fdest1, fdest2)

	accounting.GlobalStats().ResetCounters()
	fdst, err := fs.NewFs(ctx, r.FremoteName+"/dest")
	require.NoError(t, err)
	require.NoError(t, Sync(ctx, fdst, r.Flocal, false))

	fdest3 := fsrc3
	fdest3.Path = "dest/3"

	fstest.CheckItemsWithPrecision(t, fdst, precision, fsrc3)
	r.CheckRemoteItems(t, fdest1, fdest2, fdest3)
}

// Test with CopyDest set
func TestSyncCopyDest(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()

	if r.Fremote.Features().Copy == nil {
		t.Skip("Skipping test as remote does not support server-side copy")
	}

	ci.CopyDest = []string{r.FremoteName + "/CopyDest"}

	fdst, err := fs.NewFs(ctx, r.FremoteName+"/dst")
	require.NoError(t, err)

	// check empty dest, empty copy
	file1 := r.WriteFile("one", "one", t1)
	r.CheckLocalItems(t, file1)

	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, fdst, r.Flocal, false)
	require.NoError(t, err)

	file1dst := file1
	file1dst.Path = "dst/one"

	r.CheckRemoteItems(t, file1dst)

	// check old dest, empty copy
	file1b := r.WriteFile("one", "onet2", t2)
	r.CheckRemoteItems(t, file1dst)
	r.CheckLocalItems(t, file1b)

	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, fdst, r.Flocal, false)
	require.NoError(t, err)

	file1bdst := file1b
	file1bdst.Path = "dst/one"

	r.CheckRemoteItems(t, file1bdst)

	// check old dest, new copy, backup-dir

	ci.BackupDir = r.FremoteName + "/BackupDir"

	file3 := r.WriteObject(ctx, "dst/one", "one", t1)
	file2 := r.WriteObject(ctx, "CopyDest/one", "onet2", t2)
	file1c := r.WriteFile("one", "onet2", t2)
	r.CheckRemoteItems(t, file2, file3)
	r.CheckLocalItems(t, file1c)

	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, fdst, r.Flocal, false)
	require.NoError(t, err)

	file2dst := file2
	file2dst.Path = "dst/one"
	file3.Path = "BackupDir/one"

	r.CheckRemoteItems(t, file2, file2dst, file3)
	ci.BackupDir = ""

	// check empty dest, new copy
	file4 := r.WriteObject(ctx, "CopyDest/two", "two", t2)
	file5 := r.WriteFile("two", "two", t2)
	r.CheckRemoteItems(t, file2, file2dst, file3, file4)
	r.CheckLocalItems(t, file1c, file5)

	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, fdst, r.Flocal, false)
	require.NoError(t, err)

	file4dst := file4
	file4dst.Path = "dst/two"

	r.CheckRemoteItems(t, file2, file2dst, file3, file4, file4dst)

	// check new dest, new copy
	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, fdst, r.Flocal, false)
	require.NoError(t, err)

	r.CheckRemoteItems(t, file2, file2dst, file3, file4, file4dst)

	// check empty dest, old copy
	file6 := r.WriteObject(ctx, "CopyDest/three", "three", t2)
	file7 := r.WriteFile("three", "threet3", t3)
	r.CheckRemoteItems(t, file2, file2dst, file3, file4, file4dst, file6)
	r.CheckLocalItems(t, file1c, file5, file7)

	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, fdst, r.Flocal, false)
	require.NoError(t, err)

	file7dst := file7
	file7dst.Path = "dst/three"

	r.CheckRemoteItems(t, file2, file2dst, file3, file4, file4dst, file6, file7dst)
}

// Test with BackupDir set
func testSyncBackupDir(t *testing.T, backupDir string, suffix string, suffixKeepExtension bool) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()

	if !operations.CanServerSideMove(r.Fremote) {
		t.Skip("Skipping test as remote does not support server-side move")
	}
	r.Mkdir(ctx, r.Fremote)

	if backupDir != "" {
		ci.BackupDir = r.FremoteName + "/" + backupDir
		backupDir += "/"
	} else {
		ci.BackupDir = ""
		backupDir = "dst/"
		// Exclude the suffix from the sync otherwise the sync
		// deletes the old backup files
		flt, err := filter.NewFilter(nil)
		require.NoError(t, err)
		require.NoError(t, flt.AddRule("- *"+suffix))
		// Change the active filter
		ctx = filter.ReplaceConfig(ctx, flt)
	}
	ci.Suffix = suffix
	ci.SuffixKeepExtension = suffixKeepExtension

	// Make the setup so we have one, two, three in the dest
	// and one (different), two (same) in the source
	file1 := r.WriteObject(ctx, "dst/one", "one", t1)
	file2 := r.WriteObject(ctx, "dst/two", "two", t1)
	file3 := r.WriteObject(ctx, "dst/three.txt", "three", t1)
	file2a := r.WriteFile("two", "two", t1)
	file1a := r.WriteFile("one", "oneA", t2)

	r.CheckRemoteItems(t, file1, file2, file3)
	r.CheckLocalItems(t, file1a, file2a)

	fdst, err := fs.NewFs(ctx, r.FremoteName+"/dst")
	require.NoError(t, err)

	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, fdst, r.Flocal, false)
	require.NoError(t, err)

	// one should be moved to the backup dir and the new one installed
	file1.Path = backupDir + "one" + suffix
	file1a.Path = "dst/one"
	// two should be unchanged
	// three should be moved to the backup dir
	if suffixKeepExtension {
		file3.Path = backupDir + "three" + suffix + ".txt"
	} else {
		file3.Path = backupDir + "three.txt" + suffix
	}

	r.CheckRemoteItems(t, file1, file2, file3, file1a)

	// Now check what happens if we do it again
	// Restore a different three and update one in the source
	file3a := r.WriteObject(ctx, "dst/three.txt", "threeA", t2)
	file1b := r.WriteFile("one", "oneBB", t3)
	r.CheckRemoteItems(t, file1, file2, file3, file1a, file3a)

	// This should delete three and overwrite one again, checking
	// the files got overwritten correctly in backup-dir
	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, fdst, r.Flocal, false)
	require.NoError(t, err)

	// one should be moved to the backup dir and the new one installed
	file1a.Path = backupDir + "one" + suffix
	file1b.Path = "dst/one"
	// two should be unchanged
	// three should be moved to the backup dir
	if suffixKeepExtension {
		file3a.Path = backupDir + "three" + suffix + ".txt"
	} else {
		file3a.Path = backupDir + "three.txt" + suffix
	}

	r.CheckRemoteItems(t, file1b, file2, file3a, file1a)
}
func TestSyncBackupDir(t *testing.T) {
	testSyncBackupDir(t, "backup", "", false)
}
func TestSyncBackupDirWithSuffix(t *testing.T) {
	testSyncBackupDir(t, "backup", ".bak", false)
}
func TestSyncBackupDirWithSuffixKeepExtension(t *testing.T) {
	testSyncBackupDir(t, "backup", "-2019-01-01", true)
}
func TestSyncBackupDirSuffixOnly(t *testing.T) {
	testSyncBackupDir(t, "", ".bak", false)
}

// Test with Suffix set
func testSyncSuffix(t *testing.T, suffix string, suffixKeepExtension bool) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()

	if !operations.CanServerSideMove(r.Fremote) {
		t.Skip("Skipping test as remote does not support server-side move")
	}
	r.Mkdir(ctx, r.Fremote)

	ci.Suffix = suffix
	ci.SuffixKeepExtension = suffixKeepExtension

	// Make the setup so we have one, two, three in the dest
	// and one (different), two (same) in the source
	file1 := r.WriteObject(ctx, "dst/one", "one", t1)
	file2 := r.WriteObject(ctx, "dst/two", "two", t1)
	file3 := r.WriteObject(ctx, "dst/three.txt", "three", t1)
	file2a := r.WriteFile("two", "two", t1)
	file1a := r.WriteFile("one", "oneA", t2)
	file3a := r.WriteFile("three.txt", "threeA", t1)

	r.CheckRemoteItems(t, file1, file2, file3)
	r.CheckLocalItems(t, file1a, file2a, file3a)

	fdst, err := fs.NewFs(ctx, r.FremoteName+"/dst")
	require.NoError(t, err)

	accounting.GlobalStats().ResetCounters()
	err = operations.CopyFile(ctx, fdst, r.Flocal, "one", "one")
	require.NoError(t, err)
	err = operations.CopyFile(ctx, fdst, r.Flocal, "two", "two")
	require.NoError(t, err)
	err = operations.CopyFile(ctx, fdst, r.Flocal, "three.txt", "three.txt")
	require.NoError(t, err)

	// one should be moved to the backup dir and the new one installed
	file1.Path = "dst/one" + suffix
	file1a.Path = "dst/one"
	// two should be unchanged
	// three should be moved to the backup dir
	if suffixKeepExtension {
		file3.Path = "dst/three" + suffix + ".txt"
	} else {
		file3.Path = "dst/three.txt" + suffix
	}
	file3a.Path = "dst/three.txt"

	r.CheckRemoteItems(t, file1, file2, file3, file1a, file3a)

	// Now check what happens if we do it again
	// Restore a different three and update one in the source
	file3b := r.WriteFile("three.txt", "threeBDifferentSize", t3)
	file1b := r.WriteFile("one", "oneBB", t3)
	r.CheckRemoteItems(t, file1, file2, file3, file1a, file3a)

	// This should delete three and overwrite one again, checking
	// the files got overwritten correctly in backup-dir
	accounting.GlobalStats().ResetCounters()
	err = operations.CopyFile(ctx, fdst, r.Flocal, "one", "one")
	require.NoError(t, err)
	err = operations.CopyFile(ctx, fdst, r.Flocal, "two", "two")
	require.NoError(t, err)
	err = operations.CopyFile(ctx, fdst, r.Flocal, "three.txt", "three.txt")
	require.NoError(t, err)

	// one should be moved to the backup dir and the new one installed
	file1a.Path = "dst/one" + suffix
	file1b.Path = "dst/one"
	// two should be unchanged
	// three should be moved to the backup dir
	if suffixKeepExtension {
		file3a.Path = "dst/three" + suffix + ".txt"
	} else {
		file3a.Path = "dst/three.txt" + suffix
	}
	file3b.Path = "dst/three.txt"

	r.CheckRemoteItems(t, file1b, file3b, file2, file3a, file1a)
}
func TestSyncSuffix(t *testing.T)              { testSyncSuffix(t, ".bak", false) }
func TestSyncSuffixKeepExtension(t *testing.T) { testSyncSuffix(t, "-2019-01-01", true) }

// Check we can sync two files with differing UTF-8 representations
func TestSyncUTFNorm(t *testing.T) {
	ctx := context.Background()
	if runtime.GOOS == "darwin" {
		t.Skip("Can't test UTF normalization on OS X")
	}

	r := fstest.NewRun(t)
	defer r.Finalise()

	// Two strings with different unicode normalization (from OS X)
	Encoding1 := "Testêé"
	Encoding2 := "Testêé"
	assert.NotEqual(t, Encoding1, Encoding2)
	assert.Equal(t, norm.NFC.String(Encoding1), norm.NFC.String(Encoding2))

	file1 := r.WriteFile(Encoding1, "This is a test", t1)
	r.CheckLocalItems(t, file1)

	file2 := r.WriteObject(ctx, Encoding2, "This is a old test", t2)
	r.CheckRemoteItems(t, file2)

	accounting.GlobalStats().ResetCounters()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	// We should have transferred exactly one file, but kept the
	// normalized state of the file.
	assert.Equal(t, toyFileTransfers(r), accounting.GlobalStats().GetTransfers())
	r.CheckLocalItems(t, file1)
	file1.Path = file2.Path
	r.CheckRemoteItems(t, file1)
}

// Test --immutable
func TestSyncImmutable(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()

	ci.Immutable = true

	// Create file on source
	file1 := r.WriteFile("existing", "potato", t1)
	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t)

	// Should succeed
	accounting.GlobalStats().ResetCounters()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file1)

	// Modify file data and timestamp on source
	file2 := r.WriteFile("existing", "tomatoes", t2)
	r.CheckLocalItems(t, file2)
	r.CheckRemoteItems(t, file1)

	// Should fail with ErrorImmutableModified and not modify local or remote files
	accounting.GlobalStats().ResetCounters()
	err = Sync(ctx, r.Fremote, r.Flocal, false)
	assert.EqualError(t, err, fs.ErrorImmutableModified.Error())
	r.CheckLocalItems(t, file2)
	r.CheckRemoteItems(t, file1)
}

// Test --ignore-case-sync
func TestSyncIgnoreCase(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()

	// Only test if filesystems are case sensitive
	if r.Fremote.Features().CaseInsensitive || r.Flocal.Features().CaseInsensitive {
		t.Skip("Skipping test as local or remote are case-insensitive")
	}

	ci.IgnoreCaseSync = true

	// Create files with different filename casing
	file1 := r.WriteFile("existing", "potato", t1)
	r.CheckLocalItems(t, file1)
	file2 := r.WriteObject(ctx, "EXISTING", "potato", t1)
	r.CheckRemoteItems(t, file2)

	// Should not copy files that are differently-cased but otherwise identical
	accounting.GlobalStats().ResetCounters()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file2)
}

// Test that aborting on --max-transfer works
func TestMaxTransfer(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	ci.MaxTransfer = 3 * 1024
	ci.Transfers = 1
	ci.Checkers = 1
	ci.CutoffMode = fs.CutoffModeHard

	test := func(t *testing.T, cutoff fs.CutoffMode) {
		r := fstest.NewRun(t)
		defer r.Finalise()
		ci.CutoffMode = cutoff

		if r.Fremote.Name() != "local" {
			t.Skip("This test only runs on local")
		}

		// Create file on source
		file1 := r.WriteFile("file1", string(make([]byte, 5*1024)), t1)
		file2 := r.WriteFile("file2", string(make([]byte, 2*1024)), t1)
		file3 := r.WriteFile("file3", string(make([]byte, 3*1024)), t1)
		r.CheckLocalItems(t, file1, file2, file3)
		r.CheckRemoteItems(t)

		accounting.GlobalStats().ResetCounters()

		err := Sync(ctx, r.Fremote, r.Flocal, false)
		expectedErr := fserrors.FsError(accounting.ErrorMaxTransferLimitReachedFatal)
		if cutoff != fs.CutoffModeHard {
			expectedErr = accounting.ErrorMaxTransferLimitReachedGraceful
		}
		fserrors.Count(expectedErr)
		assert.Equal(t, expectedErr, err)
	}

	t.Run("Hard", func(t *testing.T) { test(t, fs.CutoffModeHard) })
	t.Run("Soft", func(t *testing.T) { test(t, fs.CutoffModeSoft) })
	t.Run("Cautious", func(t *testing.T) { test(t, fs.CutoffModeCautious) })
}

func testSyncConcurrent(t *testing.T, subtest string) {
	const (
		NFILES     = 20
		NCHECKERS  = 4
		NTRANSFERS = 4
	)

	ctx, ci := fs.AddConfig(context.Background())
	ci.Checkers = NCHECKERS
	ci.Transfers = NTRANSFERS

	r := fstest.NewRun(t)
	defer r.Finalise()
	stats := accounting.GlobalStats()

	itemsBefore := []fstest.Item{}
	itemsAfter := []fstest.Item{}
	for i := 0; i < NFILES; i++ {
		nameBoth := fmt.Sprintf("both%d", i)
		nameOnly := fmt.Sprintf("only%d", i)
		switch subtest {
		case "delete":
			fileBoth := r.WriteBoth(ctx, nameBoth, "potato", t1)
			fileOnly := r.WriteObject(ctx, nameOnly, "potato", t1)
			itemsBefore = append(itemsBefore, fileBoth, fileOnly)
			itemsAfter = append(itemsAfter, fileBoth)
		case "truncate":
			fileBoth := r.WriteBoth(ctx, nameBoth, "potato", t1)
			fileFull := r.WriteObject(ctx, nameOnly, "potato", t1)
			fileEmpty := r.WriteFile(nameOnly, "", t1)
			itemsBefore = append(itemsBefore, fileBoth, fileFull)
			itemsAfter = append(itemsAfter, fileBoth, fileEmpty)
		}
	}

	r.CheckRemoteItems(t, itemsBefore...)
	stats.ResetErrors()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	if errors.Is(err, fs.ErrorCantUploadEmptyFiles) {
		t.Skipf("Skip test because remote cannot upload empty files")
	}
	assert.NoError(t, err, "Sync must not return a error")
	assert.False(t, stats.Errored(), "Low level errors must not have happened")
	r.CheckRemoteItems(t, itemsAfter...)
}

func TestSyncConcurrentDelete(t *testing.T) {
	testSyncConcurrent(t, "delete")
}

func TestSyncConcurrentTruncate(t *testing.T) {
	testSyncConcurrent(t, "truncate")
}
