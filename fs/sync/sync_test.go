// Test sync/copy/move

package sync

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
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
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	r.Mkdir(context.Background(), r.Fremote)

	fs.Config.DryRun = true
	err := CopyDir(context.Background(), r.Fremote, r.Flocal, false)
	fs.Config.DryRun = false
	require.NoError(t, err)

	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote)
}

// Now without dry run
func TestCopy(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	r.Mkdir(context.Background(), r.Fremote)

	err := CopyDir(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file1)
}

func TestCopyMissingDirectory(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	r.Mkdir(context.Background(), r.Fremote)

	nonExistingFs, err := fs.NewFs("/non-existing")
	if err != nil {
		t.Fatal(err)
	}

	err = CopyDir(context.Background(), r.Fremote, nonExistingFs, false)
	require.Error(t, err)
}

// Now with --no-traverse
func TestCopyNoTraverse(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	fs.Config.NoTraverse = true
	defer func() { fs.Config.NoTraverse = false }()

	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)

	err := CopyDir(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file1)
}

// Now with --check-first
func TestCopyCheckFirst(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	fs.Config.CheckFirst = true
	defer func() { fs.Config.CheckFirst = false }()

	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)

	err := CopyDir(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file1)
}

// Now with --no-traverse
func TestSyncNoTraverse(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	fs.Config.NoTraverse = true
	defer func() { fs.Config.NoTraverse = false }()

	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)

	accounting.GlobalStats().ResetCounters()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file1)
}

// Test copy with depth
func TestCopyWithDepth(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	file2 := r.WriteFile("hello world2", "hello world2", t2)

	// Check the MaxDepth too
	fs.Config.MaxDepth = 1
	defer func() { fs.Config.MaxDepth = -1 }()

	err := CopyDir(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Flocal, file1, file2)
	fstest.CheckItems(t, r.Fremote, file2)
}

// Test copy with files from
func testCopyWithFilesFrom(t *testing.T, noTraverse bool) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("potato2", "hello world", t1)
	file2 := r.WriteFile("hello world2", "hello world2", t2)

	// Set the --files-from equivalent
	f, err := filter.NewFilter(nil)
	require.NoError(t, err)
	require.NoError(t, f.AddFile("potato2"))
	require.NoError(t, f.AddFile("notfound"))

	// Monkey patch the active filter
	oldFilter := filter.Active
	oldNoTraverse := fs.Config.NoTraverse
	filter.Active = f
	fs.Config.NoTraverse = noTraverse
	unpatch := func() {
		filter.Active = oldFilter
		fs.Config.NoTraverse = oldNoTraverse
	}
	defer unpatch()

	err = CopyDir(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	unpatch()

	fstest.CheckItems(t, r.Flocal, file1, file2)
	fstest.CheckItems(t, r.Fremote, file1)
}
func TestCopyWithFilesFrom(t *testing.T)              { testCopyWithFilesFrom(t, false) }
func TestCopyWithFilesFromAndNoTraverse(t *testing.T) { testCopyWithFilesFrom(t, true) }

// Test copy empty directories
func TestCopyEmptyDirectories(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	err := operations.Mkdir(context.Background(), r.Flocal, "sub dir2")
	require.NoError(t, err)
	r.Mkdir(context.Background(), r.Fremote)

	err = CopyDir(context.Background(), r.Fremote, r.Flocal, true)
	require.NoError(t, err)

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{
			file1,
		},
		[]string{
			"sub dir",
			"sub dir2",
		},
		fs.GetModifyWindow(r.Fremote),
	)
}

// Test move empty directories
func TestMoveEmptyDirectories(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	err := operations.Mkdir(context.Background(), r.Flocal, "sub dir2")
	require.NoError(t, err)
	r.Mkdir(context.Background(), r.Fremote)

	err = MoveDir(context.Background(), r.Fremote, r.Flocal, false, true)
	require.NoError(t, err)

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{
			file1,
		},
		[]string{
			"sub dir",
			"sub dir2",
		},
		fs.GetModifyWindow(r.Fremote),
	)
}

// Test sync empty directories
func TestSyncEmptyDirectories(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	err := operations.Mkdir(context.Background(), r.Flocal, "sub dir2")
	require.NoError(t, err)
	r.Mkdir(context.Background(), r.Fremote)

	err = Sync(context.Background(), r.Fremote, r.Flocal, true)
	require.NoError(t, err)

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{
			file1,
		},
		[]string{
			"sub dir",
			"sub dir2",
		},
		fs.GetModifyWindow(r.Fremote),
	)
}

// Test a server side copy if possible, or the backup path if not
func TestServerSideCopy(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject(context.Background(), "sub dir/hello world", "hello world", t1)
	fstest.CheckItems(t, r.Fremote, file1)

	FremoteCopy, _, finaliseCopy, err := fstest.RandomRemote()
	require.NoError(t, err)
	defer finaliseCopy()
	t.Logf("Server side copy (if possible) %v -> %v", r.Fremote, FremoteCopy)

	err = CopyDir(context.Background(), FremoteCopy, r.Fremote, false)
	require.NoError(t, err)

	fstest.CheckItems(t, FremoteCopy, file1)
}

// Check that if the local file doesn't exist when we copy it up,
// nothing happens to the remote file
func TestCopyAfterDelete(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject(context.Background(), "sub dir/hello world", "hello world", t1)
	fstest.CheckItems(t, r.Flocal)
	fstest.CheckItems(t, r.Fremote, file1)

	err := operations.Mkdir(context.Background(), r.Flocal, "")
	require.NoError(t, err)

	err = CopyDir(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Flocal)
	fstest.CheckItems(t, r.Fremote, file1)
}

// Check the copy downloading a file
func TestCopyRedownload(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject(context.Background(), "sub dir/hello world", "hello world", t1)
	fstest.CheckItems(t, r.Fremote, file1)

	err := CopyDir(context.Background(), r.Flocal, r.Fremote, false)
	require.NoError(t, err)

	// Test with combined precision of local and remote as we copied it there and back
	fstest.CheckListingWithPrecision(t, r.Flocal, []fstest.Item{file1}, nil, fs.GetModifyWindow(r.Flocal, r.Fremote))
}

// Create a file and sync it. Change the last modified date and resync.
// If we're only doing sync by size and checksum, we expect nothing to
// to be transferred on the second sync.
func TestSyncBasedOnCheckSum(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	fs.Config.CheckSum = true
	defer func() { fs.Config.CheckSum = false }()

	file1 := r.WriteFile("check sum", "-", t1)
	fstest.CheckItems(t, r.Flocal, file1)

	accounting.GlobalStats().ResetCounters()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	// We should have transferred exactly one file.
	assert.Equal(t, toyFileTransfers(r), accounting.GlobalStats().GetTransfers())
	fstest.CheckItems(t, r.Fremote, file1)

	// Change last modified date only
	file2 := r.WriteFile("check sum", "-", t2)
	fstest.CheckItems(t, r.Flocal, file2)

	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	// We should have transferred no files
	assert.Equal(t, int64(0), accounting.GlobalStats().GetTransfers())
	fstest.CheckItems(t, r.Flocal, file2)
	fstest.CheckItems(t, r.Fremote, file1)
}

// Create a file and sync it. Change the last modified date and the
// file contents but not the size.  If we're only doing sync by size
// only, we expect nothing to to be transferred on the second sync.
func TestSyncSizeOnly(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	fs.Config.SizeOnly = true
	defer func() { fs.Config.SizeOnly = false }()

	file1 := r.WriteFile("sizeonly", "potato", t1)
	fstest.CheckItems(t, r.Flocal, file1)

	accounting.GlobalStats().ResetCounters()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	// We should have transferred exactly one file.
	assert.Equal(t, toyFileTransfers(r), accounting.GlobalStats().GetTransfers())
	fstest.CheckItems(t, r.Fremote, file1)

	// Update mtime, md5sum but not length of file
	file2 := r.WriteFile("sizeonly", "POTATO", t2)
	fstest.CheckItems(t, r.Flocal, file2)

	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	// We should have transferred no files
	assert.Equal(t, int64(0), accounting.GlobalStats().GetTransfers())
	fstest.CheckItems(t, r.Flocal, file2)
	fstest.CheckItems(t, r.Fremote, file1)
}

// Create a file and sync it. Keep the last modified date but change
// the size.  With --ignore-size we expect nothing to to be
// transferred on the second sync.
func TestSyncIgnoreSize(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	fs.Config.IgnoreSize = true
	defer func() { fs.Config.IgnoreSize = false }()

	file1 := r.WriteFile("ignore-size", "contents", t1)
	fstest.CheckItems(t, r.Flocal, file1)

	accounting.GlobalStats().ResetCounters()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	// We should have transferred exactly one file.
	assert.Equal(t, toyFileTransfers(r), accounting.GlobalStats().GetTransfers())
	fstest.CheckItems(t, r.Fremote, file1)

	// Update size but not date of file
	file2 := r.WriteFile("ignore-size", "longer contents but same date", t1)
	fstest.CheckItems(t, r.Flocal, file2)

	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	// We should have transferred no files
	assert.Equal(t, int64(0), accounting.GlobalStats().GetTransfers())
	fstest.CheckItems(t, r.Flocal, file2)
	fstest.CheckItems(t, r.Fremote, file1)
}

func TestSyncIgnoreTimes(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth(context.Background(), "existing", "potato", t1)
	fstest.CheckItems(t, r.Fremote, file1)

	accounting.GlobalStats().ResetCounters()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	// We should have transferred exactly 0 files because the
	// files were identical.
	assert.Equal(t, int64(0), accounting.GlobalStats().GetTransfers())

	fs.Config.IgnoreTimes = true
	defer func() { fs.Config.IgnoreTimes = false }()

	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	// We should have transferred exactly one file even though the
	// files were identical.
	assert.Equal(t, toyFileTransfers(r), accounting.GlobalStats().GetTransfers())

	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file1)
}

func TestSyncIgnoreExisting(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("existing", "potato", t1)

	fs.Config.IgnoreExisting = true
	defer func() { fs.Config.IgnoreExisting = false }()

	accounting.GlobalStats().ResetCounters()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file1)

	// Change everything
	r.WriteFile("existing", "newpotatoes", t2)
	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	// Items should not change
	fstest.CheckItems(t, r.Fremote, file1)
}

func TestSyncIgnoreErrors(t *testing.T) {
	r := fstest.NewRun(t)
	fs.Config.IgnoreErrors = true
	defer func() {
		fs.Config.IgnoreErrors = false
		r.Finalise()
	}()
	file1 := r.WriteFile("a/potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteObject(context.Background(), "b/potato", "SMALLER BUT SAME DATE", t2)
	file3 := r.WriteBoth(context.Background(), "c/non empty space", "AhHa!", t2)
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "d"))

	fstest.CheckListingWithPrecision(
		t,
		r.Flocal,
		[]fstest.Item{
			file1,
			file3,
		},
		[]string{
			"a",
			"c",
		},
		fs.GetModifyWindow(r.Fremote),
	)
	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{
			file2,
			file3,
		},
		[]string{
			"b",
			"c",
			"d",
		},
		fs.GetModifyWindow(r.Fremote),
	)

	accounting.GlobalStats().ResetCounters()
	_ = fs.CountError(errors.New("boom"))
	assert.NoError(t, Sync(context.Background(), r.Fremote, r.Flocal, false))

	fstest.CheckListingWithPrecision(
		t,
		r.Flocal,
		[]fstest.Item{
			file1,
			file3,
		},
		[]string{
			"a",
			"c",
		},
		fs.GetModifyWindow(r.Fremote),
	)
	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{
			file1,
			file3,
		},
		[]string{
			"a",
			"c",
		},
		fs.GetModifyWindow(r.Fremote),
	)
}

func TestSyncAfterChangingModtimeOnly(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("empty space", "-", t2)
	file2 := r.WriteObject(context.Background(), "empty space", "-", t1)

	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file2)

	fs.Config.DryRun = true
	defer func() { fs.Config.DryRun = false }()

	accounting.GlobalStats().ResetCounters()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file2)

	fs.Config.DryRun = false

	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file1)
}

func TestSyncAfterChangingModtimeOnlyWithNoUpdateModTime(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	if r.Fremote.Hashes().Count() == 0 {
		t.Logf("Can't check this if no hashes supported")
		return
	}

	fs.Config.NoUpdateModTime = true
	defer func() {
		fs.Config.NoUpdateModTime = false
	}()

	file1 := r.WriteFile("empty space", "-", t2)
	file2 := r.WriteObject(context.Background(), "empty space", "-", t1)

	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file2)

	accounting.GlobalStats().ResetCounters()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file2)
}

func TestSyncDoesntUpdateModtime(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	if fs.GetModifyWindow(r.Fremote) == fs.ModTimeNotSupported {
		t.Skip("Can't run this test on fs which doesn't support mod time")
	}

	file1 := r.WriteFile("foo", "foo", t2)
	file2 := r.WriteObject(context.Background(), "foo", "bar", t1)

	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file2)

	accounting.GlobalStats().ResetCounters()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file1)

	// We should have transferred exactly one file, not set the mod time
	assert.Equal(t, toyFileTransfers(r), accounting.GlobalStats().GetTransfers())
}

func TestSyncAfterAddingAFile(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth(context.Background(), "empty space", "-", t2)
	file2 := r.WriteFile("potato", "------------------------------------------------------------", t3)

	fstest.CheckItems(t, r.Flocal, file1, file2)
	fstest.CheckItems(t, r.Fremote, file1)

	accounting.GlobalStats().ResetCounters()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file1, file2)
	fstest.CheckItems(t, r.Fremote, file1, file2)
}

func TestSyncAfterChangingFilesSizeOnly(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject(context.Background(), "potato", "------------------------------------------------------------", t3)
	file2 := r.WriteFile("potato", "smaller but same date", t3)
	fstest.CheckItems(t, r.Fremote, file1)
	fstest.CheckItems(t, r.Flocal, file2)

	accounting.GlobalStats().ResetCounters()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file2)
	fstest.CheckItems(t, r.Fremote, file2)
}

// Sync after changing a file's contents, changing modtime but length
// remaining the same
func TestSyncAfterChangingContentsOnly(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	var file1 fstest.Item
	if r.Fremote.Precision() == fs.ModTimeNotSupported {
		t.Logf("ModTimeNotSupported so forcing file to be a different size")
		file1 = r.WriteObject(context.Background(), "potato", "different size to make sure it syncs", t3)
	} else {
		file1 = r.WriteObject(context.Background(), "potato", "smaller but same date", t3)
	}
	file2 := r.WriteFile("potato", "SMALLER BUT SAME DATE", t2)
	fstest.CheckItems(t, r.Fremote, file1)
	fstest.CheckItems(t, r.Flocal, file2)

	accounting.GlobalStats().ResetCounters()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file2)
	fstest.CheckItems(t, r.Fremote, file2)
}

// Sync after removing a file and adding a file --dry-run
func TestSyncAfterRemovingAFileAndAddingAFileDryRun(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteObject(context.Background(), "potato", "SMALLER BUT SAME DATE", t2)
	file3 := r.WriteBoth(context.Background(), "empty space", "-", t2)

	fs.Config.DryRun = true
	accounting.GlobalStats().ResetCounters()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	fs.Config.DryRun = false
	require.NoError(t, err)

	fstest.CheckItems(t, r.Flocal, file3, file1)
	fstest.CheckItems(t, r.Fremote, file3, file2)
}

// Sync after removing a file and adding a file
func TestSyncAfterRemovingAFileAndAddingAFile(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteObject(context.Background(), "potato", "SMALLER BUT SAME DATE", t2)
	file3 := r.WriteBoth(context.Background(), "empty space", "-", t2)
	fstest.CheckItems(t, r.Fremote, file2, file3)
	fstest.CheckItems(t, r.Flocal, file1, file3)

	accounting.GlobalStats().ResetCounters()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file1, file3)
	fstest.CheckItems(t, r.Fremote, file1, file3)
}

// Sync after removing a file and adding a file
func TestSyncAfterRemovingAFileAndAddingAFileSubDir(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("a/potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteObject(context.Background(), "b/potato", "SMALLER BUT SAME DATE", t2)
	file3 := r.WriteBoth(context.Background(), "c/non empty space", "AhHa!", t2)
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "d"))
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "d/e"))

	fstest.CheckListingWithPrecision(
		t,
		r.Flocal,
		[]fstest.Item{
			file1,
			file3,
		},
		[]string{
			"a",
			"c",
		},
		fs.GetModifyWindow(r.Fremote),
	)
	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
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
		fs.GetModifyWindow(r.Fremote),
	)

	accounting.GlobalStats().ResetCounters()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	fstest.CheckListingWithPrecision(
		t,
		r.Flocal,
		[]fstest.Item{
			file1,
			file3,
		},
		[]string{
			"a",
			"c",
		},
		fs.GetModifyWindow(r.Fremote),
	)
	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{
			file1,
			file3,
		},
		[]string{
			"a",
			"c",
		},
		fs.GetModifyWindow(r.Fremote),
	)
}

// Sync after removing a file and adding a file with IO Errors
func TestSyncAfterRemovingAFileAndAddingAFileSubDirWithErrors(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("a/potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteObject(context.Background(), "b/potato", "SMALLER BUT SAME DATE", t2)
	file3 := r.WriteBoth(context.Background(), "c/non empty space", "AhHa!", t2)
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "d"))

	fstest.CheckListingWithPrecision(
		t,
		r.Flocal,
		[]fstest.Item{
			file1,
			file3,
		},
		[]string{
			"a",
			"c",
		},
		fs.GetModifyWindow(r.Fremote),
	)
	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{
			file2,
			file3,
		},
		[]string{
			"b",
			"c",
			"d",
		},
		fs.GetModifyWindow(r.Fremote),
	)

	accounting.GlobalStats().ResetCounters()
	_ = fs.CountError(errors.New("boom"))
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	assert.Equal(t, fs.ErrorNotDeleting, err)

	fstest.CheckListingWithPrecision(
		t,
		r.Flocal,
		[]fstest.Item{
			file1,
			file3,
		},
		[]string{
			"a",
			"c",
		},
		fs.GetModifyWindow(r.Fremote),
	)
	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
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
		fs.GetModifyWindow(r.Fremote),
	)
}

// Sync test delete after
func TestSyncDeleteAfter(t *testing.T) {
	// This is the default so we've checked this already
	// check it is the default
	require.Equal(t, fs.Config.DeleteMode, fs.DeleteModeAfter, "Didn't default to --delete-after")
}

// Sync test delete during
func TestSyncDeleteDuring(t *testing.T) {
	fs.Config.DeleteMode = fs.DeleteModeDuring
	defer func() {
		fs.Config.DeleteMode = fs.DeleteModeDefault
	}()

	TestSyncAfterRemovingAFileAndAddingAFile(t)
}

// Sync test delete before
func TestSyncDeleteBefore(t *testing.T) {
	fs.Config.DeleteMode = fs.DeleteModeBefore
	defer func() {
		fs.Config.DeleteMode = fs.DeleteModeDefault
	}()

	TestSyncAfterRemovingAFileAndAddingAFile(t)
}

// Copy test delete before - shouldn't delete anything
func TestCopyDeleteBefore(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	fs.Config.DeleteMode = fs.DeleteModeBefore
	defer func() {
		fs.Config.DeleteMode = fs.DeleteModeDefault
	}()

	file1 := r.WriteObject(context.Background(), "potato", "hopefully not deleted", t1)
	file2 := r.WriteFile("potato2", "hopefully copied in", t1)
	fstest.CheckItems(t, r.Fremote, file1)
	fstest.CheckItems(t, r.Flocal, file2)

	accounting.GlobalStats().ResetCounters()
	err := CopyDir(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Fremote, file1, file2)
	fstest.CheckItems(t, r.Flocal, file2)
}

// Test with exclude
func TestSyncWithExclude(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth(context.Background(), "potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth(context.Background(), "empty space", "-", t2)
	file3 := r.WriteFile("enormous", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", t1) // 100 bytes
	fstest.CheckItems(t, r.Fremote, file1, file2)
	fstest.CheckItems(t, r.Flocal, file1, file2, file3)

	filter.Active.Opt.MaxSize = 40
	defer func() {
		filter.Active.Opt.MaxSize = -1
	}()

	accounting.GlobalStats().ResetCounters()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Fremote, file2, file1)

	// Now sync the other way round and check enormous doesn't get
	// deleted as it is excluded from the sync
	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), r.Flocal, r.Fremote, false)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file2, file1, file3)
}

// Test with exclude and delete excluded
func TestSyncWithExcludeAndDeleteExcluded(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth(context.Background(), "potato2", "------------------------------------------------------------", t1) // 60 bytes
	file2 := r.WriteBoth(context.Background(), "empty space", "-", t2)
	file3 := r.WriteBoth(context.Background(), "enormous", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", t1) // 100 bytes
	fstest.CheckItems(t, r.Fremote, file1, file2, file3)
	fstest.CheckItems(t, r.Flocal, file1, file2, file3)

	filter.Active.Opt.MaxSize = 40
	filter.Active.Opt.DeleteExcluded = true
	defer func() {
		filter.Active.Opt.MaxSize = -1
		filter.Active.Opt.DeleteExcluded = false
	}()

	accounting.GlobalStats().ResetCounters()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Fremote, file2)

	// Check sync the other way round to make sure enormous gets
	// deleted even though it is excluded
	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), r.Flocal, r.Fremote, false)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file2)
}

// Test with UpdateOlder set
func TestSyncWithUpdateOlder(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	if fs.GetModifyWindow(r.Fremote) == fs.ModTimeNotSupported {
		t.Skip("Can't run this test on fs which doesn't support mod time")
	}
	t2plus := t2.Add(time.Second / 2)
	t2minus := t2.Add(time.Second / 2)
	oneF := r.WriteFile("one", "one", t1)
	twoF := r.WriteFile("two", "two", t3)
	threeF := r.WriteFile("three", "three", t2)
	fourF := r.WriteFile("four", "four", t2)
	fiveF := r.WriteFile("five", "five", t2)
	fstest.CheckItems(t, r.Flocal, oneF, twoF, threeF, fourF, fiveF)
	oneO := r.WriteObject(context.Background(), "one", "ONE", t2)
	twoO := r.WriteObject(context.Background(), "two", "TWO", t2)
	threeO := r.WriteObject(context.Background(), "three", "THREE", t2plus)
	fourO := r.WriteObject(context.Background(), "four", "FOURFOUR", t2minus)
	fstest.CheckItems(t, r.Fremote, oneO, twoO, threeO, fourO)

	fs.Config.UpdateOlder = true
	oldModifyWindow := fs.Config.ModifyWindow
	fs.Config.ModifyWindow = fs.ModTimeNotSupported
	defer func() {
		fs.Config.UpdateOlder = false
		fs.Config.ModifyWindow = oldModifyWindow
	}()

	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Fremote, oneO, twoF, threeO, fourF, fiveF)

	if r.Fremote.Hashes().Count() == 0 {
		t.Logf("Skip test with --checksum as no hashes supported")
		return
	}

	// now enable checksum
	fs.Config.CheckSum = true
	defer func() { fs.Config.CheckSum = false }()

	err = Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Fremote, oneO, twoF, threeF, fourF, fiveF)
}

// Test with a max transfer duration
func TestSyncWithMaxDuration(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping test on non local remote")
	}
	r := fstest.NewRun(t)
	defer r.Finalise()

	maxDuration := 250 * time.Millisecond
	fs.Config.MaxDuration = maxDuration
	bytesPerSecond := 300
	accounting.SetBwLimit(fs.SizeSuffix(bytesPerSecond))
	oldTransfers := fs.Config.Transfers
	fs.Config.Transfers = 1
	defer func() {
		fs.Config.MaxDuration = 0 // reset back to default
		fs.Config.Transfers = oldTransfers
		accounting.SetBwLimit(fs.SizeSuffix(0))
	}()

	// 5 files of 60 bytes at 60 bytes/s 5 seconds
	testFiles := make([]fstest.Item, 5)
	for i := 0; i < len(testFiles); i++ {
		testFiles[i] = r.WriteFile(fmt.Sprintf("file%d", i), "------------------------------------------------------------", t1)
	}

	fstest.CheckListing(t, r.Flocal, testFiles)

	accounting.GlobalStats().ResetCounters()
	startTime := time.Now()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.Equal(t, context.DeadlineExceeded, errors.Cause(err))

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
	r := fstest.NewRun(t)
	defer r.Finalise()

	fs.Config.TrackRenames = true
	defer func() {
		fs.Config.TrackRenames = false
	}()

	haveHash := r.Fremote.Hashes().Overlap(r.Flocal.Hashes()).GetOne() != hash.None
	canTrackRenames := haveHash && operations.CanServerSideMove(r.Fremote)
	t.Logf("Can track renames: %v", canTrackRenames)

	f1 := r.WriteFile("potato", "Potato Content", t1)
	f2 := r.WriteFile("yam", "Yam Content", t2)

	accounting.GlobalStats().ResetCounters()
	require.NoError(t, Sync(context.Background(), r.Fremote, r.Flocal, false))

	fstest.CheckItems(t, r.Fremote, f1, f2)
	fstest.CheckItems(t, r.Flocal, f1, f2)

	// Now rename locally.
	f2 = r.RenameFile(f2, "yaml")

	accounting.GlobalStats().ResetCounters()
	require.NoError(t, Sync(context.Background(), r.Fremote, r.Flocal, false))

	fstest.CheckItems(t, r.Fremote, f1, f2)

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
	r := fstest.NewRun(t)
	defer r.Finalise()

	fs.Config.TrackRenames = true
	fs.Config.TrackRenamesStrategy = "modtime"
	defer func() {
		fs.Config.TrackRenames = false
		fs.Config.TrackRenamesStrategy = "hash"
	}()

	canTrackRenames := operations.CanServerSideMove(r.Fremote) && r.Fremote.Precision() != fs.ModTimeNotSupported
	t.Logf("Can track renames: %v", canTrackRenames)

	f1 := r.WriteFile("potato", "Potato Content", t1)
	f2 := r.WriteFile("yam", "Yam Content", t2)

	accounting.GlobalStats().ResetCounters()
	require.NoError(t, Sync(context.Background(), r.Fremote, r.Flocal, false))

	fstest.CheckItems(t, r.Fremote, f1, f2)
	fstest.CheckItems(t, r.Flocal, f1, f2)

	// Now rename locally.
	f2 = r.RenameFile(f2, "yaml")

	accounting.GlobalStats().ResetCounters()
	require.NoError(t, Sync(context.Background(), r.Fremote, r.Flocal, false))

	fstest.CheckItems(t, r.Fremote, f1, f2)

	// Check we renamed something if we should have
	if canTrackRenames {
		renames := accounting.GlobalStats().Renames(0)
		assert.Equal(t, canTrackRenames, renames != 0, fmt.Sprintf("canTrackRenames=%v, renames=%d", canTrackRenames, renames))
	}
}

func TestSyncWithTrackRenamesStrategyLeaf(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	fs.Config.TrackRenames = true
	fs.Config.TrackRenamesStrategy = "leaf"
	defer func() {
		fs.Config.TrackRenames = false
		fs.Config.TrackRenamesStrategy = "hash"
	}()

	canTrackRenames := operations.CanServerSideMove(r.Fremote) && r.Fremote.Precision() != fs.ModTimeNotSupported
	t.Logf("Can track renames: %v", canTrackRenames)

	f1 := r.WriteFile("potato", "Potato Content", t1)
	f2 := r.WriteFile("sub/yam", "Yam Content", t2)

	accounting.GlobalStats().ResetCounters()
	require.NoError(t, Sync(context.Background(), r.Fremote, r.Flocal, false))

	fstest.CheckItems(t, r.Fremote, f1, f2)
	fstest.CheckItems(t, r.Flocal, f1, f2)

	// Now rename locally.
	f2 = r.RenameFile(f2, "yam")

	accounting.GlobalStats().ResetCounters()
	require.NoError(t, Sync(context.Background(), r.Fremote, r.Flocal, false))

	fstest.CheckItems(t, r.Fremote, f1, f2)

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

// Test a server side move if possible, or the backup path if not
func testServerSideMove(t *testing.T, r *fstest.Run, withFilter, testDeleteEmptyDirs bool) {
	FremoteMove, _, finaliseMove, err := fstest.RandomRemote()
	require.NoError(t, err)
	defer finaliseMove()

	file1 := r.WriteBoth(context.Background(), "potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth(context.Background(), "empty space", "-", t2)
	file3u := r.WriteBoth(context.Background(), "potato3", "------------------------------------------------------------ UPDATED", t2)

	if testDeleteEmptyDirs {
		err := operations.Mkdir(context.Background(), r.Fremote, "tomatoDir")
		require.NoError(t, err)
	}

	fstest.CheckItems(t, r.Fremote, file2, file1, file3u)

	t.Logf("Server side move (if possible) %v -> %v", r.Fremote, FremoteMove)

	// Write just one file in the new remote
	r.WriteObjectTo(context.Background(), FremoteMove, "empty space", "-", t2, false)
	file3 := r.WriteObjectTo(context.Background(), FremoteMove, "potato3", "------------------------------------------------------------", t1, false)
	fstest.CheckItems(t, FremoteMove, file2, file3)

	// Do server side move
	accounting.GlobalStats().ResetCounters()
	err = MoveDir(context.Background(), FremoteMove, r.Fremote, testDeleteEmptyDirs, false)
	require.NoError(t, err)

	if withFilter {
		fstest.CheckItems(t, r.Fremote, file2)
	} else {
		fstest.CheckItems(t, r.Fremote)
	}

	if testDeleteEmptyDirs {
		fstest.CheckListingWithPrecision(t, r.Fremote, nil, []string{}, fs.GetModifyWindow(r.Fremote))
	}

	fstest.CheckItems(t, FremoteMove, file2, file1, file3u)

	// Create a new empty remote for stuff to be moved into
	FremoteMove2, _, finaliseMove2, err := fstest.RandomRemote()
	require.NoError(t, err)
	defer finaliseMove2()

	if testDeleteEmptyDirs {
		err := operations.Mkdir(context.Background(), FremoteMove, "tomatoDir")
		require.NoError(t, err)
	}

	// Move it back to a new empty remote, dst does not exist this time
	accounting.GlobalStats().ResetCounters()
	err = MoveDir(context.Background(), FremoteMove2, FremoteMove, testDeleteEmptyDirs, false)
	require.NoError(t, err)

	if withFilter {
		fstest.CheckItems(t, FremoteMove2, file1, file3u)
		fstest.CheckItems(t, FremoteMove, file2)
	} else {
		fstest.CheckItems(t, FremoteMove2, file2, file1, file3u)
		fstest.CheckItems(t, FremoteMove)
	}

	if testDeleteEmptyDirs {
		fstest.CheckListingWithPrecision(t, FremoteMove, nil, []string{}, fs.GetModifyWindow(r.Fremote))
	}
}

// Test move
func TestMoveWithDeleteEmptySrcDirs(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	file2 := r.WriteFile("nested/sub dir/file", "nested", t1)
	r.Mkdir(context.Background(), r.Fremote)

	// run move with --delete-empty-src-dirs
	err := MoveDir(context.Background(), r.Fremote, r.Flocal, true, false)
	require.NoError(t, err)

	fstest.CheckListingWithPrecision(
		t,
		r.Flocal,
		nil,
		[]string{},
		fs.GetModifyWindow(r.Flocal),
	)
	fstest.CheckItems(t, r.Fremote, file1, file2)
}

func TestMoveWithoutDeleteEmptySrcDirs(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	file2 := r.WriteFile("nested/sub dir/file", "nested", t1)
	r.Mkdir(context.Background(), r.Fremote)

	err := MoveDir(context.Background(), r.Fremote, r.Flocal, false, false)
	require.NoError(t, err)

	fstest.CheckListingWithPrecision(
		t,
		r.Flocal,
		nil,
		[]string{
			"sub dir",
			"nested",
			"nested/sub dir",
		},
		fs.GetModifyWindow(r.Flocal),
	)
	fstest.CheckItems(t, r.Fremote, file1, file2)
}

// Test a server side move if possible, or the backup path if not
func TestServerSideMove(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	testServerSideMove(t, r, false, false)
}

// Test a server side move if possible, or the backup path if not
func TestServerSideMoveWithFilter(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	filter.Active.Opt.MinSize = 40
	defer func() {
		filter.Active.Opt.MinSize = -1
	}()

	testServerSideMove(t, r, true, false)
}

// Test a server side move if possible
func TestServerSideMoveDeleteEmptySourceDirs(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	testServerSideMove(t, r, false, true)
}

// Test a server side move with overlap
func TestServerSideMoveOverlap(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	if r.Fremote.Features().DirMove != nil {
		t.Skip("Skipping test as remote supports DirMove")
	}

	subRemoteName := r.FremoteName + "/rclone-move-test"
	FremoteMove, err := fs.NewFs(subRemoteName)
	require.NoError(t, err)

	file1 := r.WriteObject(context.Background(), "potato2", "------------------------------------------------------------", t1)
	fstest.CheckItems(t, r.Fremote, file1)

	// Subdir move with no filters should return ErrorCantMoveOverlapping
	err = MoveDir(context.Background(), FremoteMove, r.Fremote, false, false)
	assert.EqualError(t, err, fs.ErrorOverlapping.Error())

	// Now try with a filter which should also fail with ErrorCantMoveOverlapping
	filter.Active.Opt.MinSize = 40
	defer func() {
		filter.Active.Opt.MinSize = -1
	}()
	err = MoveDir(context.Background(), FremoteMove, r.Fremote, false, false)
	assert.EqualError(t, err, fs.ErrorOverlapping.Error())
}

// Test a sync with overlap
func TestSyncOverlap(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	subRemoteName := r.FremoteName + "/rclone-sync-test"
	FremoteSync, err := fs.NewFs(subRemoteName)
	require.NoError(t, err)

	checkErr := func(err error) {
		require.Error(t, err)
		assert.True(t, fserrors.IsFatalError(err))
		assert.Equal(t, fs.ErrorOverlapping.Error(), err.Error())
	}

	checkErr(Sync(context.Background(), FremoteSync, r.Fremote, false))
	checkErr(Sync(context.Background(), r.Fremote, FremoteSync, false))
	checkErr(Sync(context.Background(), r.Fremote, r.Fremote, false))
	checkErr(Sync(context.Background(), FremoteSync, FremoteSync, false))
}

// Test with CompareDest set
func TestSyncCompareDest(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	fs.Config.CompareDest = r.FremoteName + "/CompareDest"
	defer func() {
		fs.Config.CompareDest = ""
	}()

	fdst, err := fs.NewFs(r.FremoteName + "/dst")
	require.NoError(t, err)

	// check empty dest, empty compare
	file1 := r.WriteFile("one", "one", t1)
	fstest.CheckItems(t, r.Flocal, file1)

	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), fdst, r.Flocal, false)
	require.NoError(t, err)

	file1dst := file1
	file1dst.Path = "dst/one"

	fstest.CheckItems(t, r.Fremote, file1dst)

	// check old dest, empty compare
	file1b := r.WriteFile("one", "onet2", t2)
	fstest.CheckItems(t, r.Fremote, file1dst)
	fstest.CheckItems(t, r.Flocal, file1b)

	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), fdst, r.Flocal, false)
	require.NoError(t, err)

	file1bdst := file1b
	file1bdst.Path = "dst/one"

	fstest.CheckItems(t, r.Fremote, file1bdst)

	// check old dest, new compare
	file3 := r.WriteObject(context.Background(), "dst/one", "one", t1)
	file2 := r.WriteObject(context.Background(), "CompareDest/one", "onet2", t2)
	file1c := r.WriteFile("one", "onet2", t2)
	fstest.CheckItems(t, r.Fremote, file2, file3)
	fstest.CheckItems(t, r.Flocal, file1c)

	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), fdst, r.Flocal, false)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Fremote, file2, file3)

	// check empty dest, new compare
	file4 := r.WriteObject(context.Background(), "CompareDest/two", "two", t2)
	file5 := r.WriteFile("two", "two", t2)
	fstest.CheckItems(t, r.Fremote, file2, file3, file4)
	fstest.CheckItems(t, r.Flocal, file1c, file5)

	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), fdst, r.Flocal, false)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Fremote, file2, file3, file4)

	// check new dest, new compare
	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), fdst, r.Flocal, false)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Fremote, file2, file3, file4)

	// check empty dest, old compare
	file5b := r.WriteFile("two", "twot3", t3)
	fstest.CheckItems(t, r.Fremote, file2, file3, file4)
	fstest.CheckItems(t, r.Flocal, file1c, file5b)

	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), fdst, r.Flocal, false)
	require.NoError(t, err)

	file5bdst := file5b
	file5bdst.Path = "dst/two"

	fstest.CheckItems(t, r.Fremote, file2, file3, file4, file5bdst)
}

// Test with CopyDest set
func TestSyncCopyDest(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	if r.Fremote.Features().Copy == nil {
		t.Skip("Skipping test as remote does not support server side copy")
	}

	fs.Config.CopyDest = r.FremoteName + "/CopyDest"
	defer func() {
		fs.Config.CopyDest = ""
	}()

	fdst, err := fs.NewFs(r.FremoteName + "/dst")
	require.NoError(t, err)

	// check empty dest, empty copy
	file1 := r.WriteFile("one", "one", t1)
	fstest.CheckItems(t, r.Flocal, file1)

	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), fdst, r.Flocal, false)
	require.NoError(t, err)

	file1dst := file1
	file1dst.Path = "dst/one"

	fstest.CheckItems(t, r.Fremote, file1dst)

	// check old dest, empty copy
	file1b := r.WriteFile("one", "onet2", t2)
	fstest.CheckItems(t, r.Fremote, file1dst)
	fstest.CheckItems(t, r.Flocal, file1b)

	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), fdst, r.Flocal, false)
	require.NoError(t, err)

	file1bdst := file1b
	file1bdst.Path = "dst/one"

	fstest.CheckItems(t, r.Fremote, file1bdst)

	// check old dest, new copy, backup-dir

	fs.Config.BackupDir = r.FremoteName + "/BackupDir"

	file3 := r.WriteObject(context.Background(), "dst/one", "one", t1)
	file2 := r.WriteObject(context.Background(), "CopyDest/one", "onet2", t2)
	file1c := r.WriteFile("one", "onet2", t2)
	fstest.CheckItems(t, r.Fremote, file2, file3)
	fstest.CheckItems(t, r.Flocal, file1c)

	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), fdst, r.Flocal, false)
	require.NoError(t, err)

	file2dst := file2
	file2dst.Path = "dst/one"
	file3.Path = "BackupDir/one"

	fstest.CheckItems(t, r.Fremote, file2, file2dst, file3)
	fs.Config.BackupDir = ""

	// check empty dest, new copy
	file4 := r.WriteObject(context.Background(), "CopyDest/two", "two", t2)
	file5 := r.WriteFile("two", "two", t2)
	fstest.CheckItems(t, r.Fremote, file2, file2dst, file3, file4)
	fstest.CheckItems(t, r.Flocal, file1c, file5)

	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), fdst, r.Flocal, false)
	require.NoError(t, err)

	file4dst := file4
	file4dst.Path = "dst/two"

	fstest.CheckItems(t, r.Fremote, file2, file2dst, file3, file4, file4dst)

	// check new dest, new copy
	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), fdst, r.Flocal, false)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Fremote, file2, file2dst, file3, file4, file4dst)

	// check empty dest, old copy
	file6 := r.WriteObject(context.Background(), "CopyDest/three", "three", t2)
	file7 := r.WriteFile("three", "threet3", t3)
	fstest.CheckItems(t, r.Fremote, file2, file2dst, file3, file4, file4dst, file6)
	fstest.CheckItems(t, r.Flocal, file1c, file5, file7)

	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), fdst, r.Flocal, false)
	require.NoError(t, err)

	file7dst := file7
	file7dst.Path = "dst/three"

	fstest.CheckItems(t, r.Fremote, file2, file2dst, file3, file4, file4dst, file6, file7dst)
}

// Test with BackupDir set
func testSyncBackupDir(t *testing.T, suffix string, suffixKeepExtension bool) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	if !operations.CanServerSideMove(r.Fremote) {
		t.Skip("Skipping test as remote does not support server side move")
	}
	r.Mkdir(context.Background(), r.Fremote)

	fs.Config.BackupDir = r.FremoteName + "/backup"
	fs.Config.Suffix = suffix
	fs.Config.SuffixKeepExtension = suffixKeepExtension
	defer func() {
		fs.Config.BackupDir = ""
		fs.Config.Suffix = ""
		fs.Config.SuffixKeepExtension = false
	}()

	// Make the setup so we have one, two, three in the dest
	// and one (different), two (same) in the source
	file1 := r.WriteObject(context.Background(), "dst/one", "one", t1)
	file2 := r.WriteObject(context.Background(), "dst/two", "two", t1)
	file3 := r.WriteObject(context.Background(), "dst/three.txt", "three", t1)
	file2a := r.WriteFile("two", "two", t1)
	file1a := r.WriteFile("one", "oneA", t2)

	fstest.CheckItems(t, r.Fremote, file1, file2, file3)
	fstest.CheckItems(t, r.Flocal, file1a, file2a)

	fdst, err := fs.NewFs(r.FremoteName + "/dst")
	require.NoError(t, err)

	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), fdst, r.Flocal, false)
	require.NoError(t, err)

	// one should be moved to the backup dir and the new one installed
	file1.Path = "backup/one" + suffix
	file1a.Path = "dst/one"
	// two should be unchanged
	// three should be moved to the backup dir
	if suffixKeepExtension {
		file3.Path = "backup/three" + suffix + ".txt"
	} else {
		file3.Path = "backup/three.txt" + suffix
	}

	fstest.CheckItems(t, r.Fremote, file1, file2, file3, file1a)

	// Now check what happens if we do it again
	// Restore a different three and update one in the source
	file3a := r.WriteObject(context.Background(), "dst/three.txt", "threeA", t2)
	file1b := r.WriteFile("one", "oneBB", t3)
	fstest.CheckItems(t, r.Fremote, file1, file2, file3, file1a, file3a)

	// This should delete three and overwrite one again, checking
	// the files got overwritten correctly in backup-dir
	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), fdst, r.Flocal, false)
	require.NoError(t, err)

	// one should be moved to the backup dir and the new one installed
	file1a.Path = "backup/one" + suffix
	file1b.Path = "dst/one"
	// two should be unchanged
	// three should be moved to the backup dir
	if suffixKeepExtension {
		file3a.Path = "backup/three" + suffix + ".txt"
	} else {
		file3a.Path = "backup/three.txt" + suffix
	}

	fstest.CheckItems(t, r.Fremote, file1b, file2, file3a, file1a)
}
func TestSyncBackupDir(t *testing.T)           { testSyncBackupDir(t, "", false) }
func TestSyncBackupDirWithSuffix(t *testing.T) { testSyncBackupDir(t, ".bak", false) }
func TestSyncBackupDirWithSuffixKeepExtension(t *testing.T) {
	testSyncBackupDir(t, "-2019-01-01", true)
}

// Test with Suffix set
func testSyncSuffix(t *testing.T, suffix string, suffixKeepExtension bool) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	if !operations.CanServerSideMove(r.Fremote) {
		t.Skip("Skipping test as remote does not support server side move")
	}
	r.Mkdir(context.Background(), r.Fremote)

	fs.Config.Suffix = suffix
	fs.Config.SuffixKeepExtension = suffixKeepExtension
	defer func() {
		fs.Config.BackupDir = ""
		fs.Config.Suffix = ""
		fs.Config.SuffixKeepExtension = false
	}()

	// Make the setup so we have one, two, three in the dest
	// and one (different), two (same) in the source
	file1 := r.WriteObject(context.Background(), "dst/one", "one", t1)
	file2 := r.WriteObject(context.Background(), "dst/two", "two", t1)
	file3 := r.WriteObject(context.Background(), "dst/three.txt", "three", t1)
	file2a := r.WriteFile("two", "two", t1)
	file1a := r.WriteFile("one", "oneA", t2)
	file3a := r.WriteFile("three.txt", "threeA", t1)

	fstest.CheckItems(t, r.Fremote, file1, file2, file3)
	fstest.CheckItems(t, r.Flocal, file1a, file2a, file3a)

	fdst, err := fs.NewFs(r.FremoteName + "/dst")
	require.NoError(t, err)

	accounting.GlobalStats().ResetCounters()
	err = operations.CopyFile(context.Background(), fdst, r.Flocal, "one", "one")
	require.NoError(t, err)
	err = operations.CopyFile(context.Background(), fdst, r.Flocal, "two", "two")
	require.NoError(t, err)
	err = operations.CopyFile(context.Background(), fdst, r.Flocal, "three.txt", "three.txt")
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

	fstest.CheckItems(t, r.Fremote, file1, file2, file3, file1a, file3a)

	// Now check what happens if we do it again
	// Restore a different three and update one in the source
	file3b := r.WriteFile("three.txt", "threeBDifferentSize", t3)
	file1b := r.WriteFile("one", "oneBB", t3)
	fstest.CheckItems(t, r.Fremote, file1, file2, file3, file1a, file3a)

	// This should delete three and overwrite one again, checking
	// the files got overwritten correctly in backup-dir
	accounting.GlobalStats().ResetCounters()
	err = operations.CopyFile(context.Background(), fdst, r.Flocal, "one", "one")
	require.NoError(t, err)
	err = operations.CopyFile(context.Background(), fdst, r.Flocal, "two", "two")
	require.NoError(t, err)
	err = operations.CopyFile(context.Background(), fdst, r.Flocal, "three.txt", "three.txt")
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

	fstest.CheckItems(t, r.Fremote, file1b, file3b, file2, file3a, file1a)
}
func TestSyncSuffix(t *testing.T)              { testSyncSuffix(t, ".bak", false) }
func TestSyncSuffixKeepExtension(t *testing.T) { testSyncSuffix(t, "-2019-01-01", true) }

// Check we can sync two files with differing UTF-8 representations
func TestSyncUTFNorm(t *testing.T) {
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
	fstest.CheckItems(t, r.Flocal, file1)

	file2 := r.WriteObject(context.Background(), Encoding2, "This is a old test", t2)
	fstest.CheckItems(t, r.Fremote, file2)

	accounting.GlobalStats().ResetCounters()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	// We should have transferred exactly one file, but kept the
	// normalized state of the file.
	assert.Equal(t, toyFileTransfers(r), accounting.GlobalStats().GetTransfers())
	fstest.CheckItems(t, r.Flocal, file1)
	file1.Path = file2.Path
	fstest.CheckItems(t, r.Fremote, file1)
}

// Test --immutable
func TestSyncImmutable(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	fs.Config.Immutable = true
	defer func() { fs.Config.Immutable = false }()

	// Create file on source
	file1 := r.WriteFile("existing", "potato", t1)
	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote)

	// Should succeed
	accounting.GlobalStats().ResetCounters()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file1)

	// Modify file data and timestamp on source
	file2 := r.WriteFile("existing", "tomatoes", t2)
	fstest.CheckItems(t, r.Flocal, file2)
	fstest.CheckItems(t, r.Fremote, file1)

	// Should fail with ErrorImmutableModified and not modify local or remote files
	accounting.GlobalStats().ResetCounters()
	err = Sync(context.Background(), r.Fremote, r.Flocal, false)
	assert.EqualError(t, err, fs.ErrorImmutableModified.Error())
	fstest.CheckItems(t, r.Flocal, file2)
	fstest.CheckItems(t, r.Fremote, file1)
}

// Test --ignore-case-sync
func TestSyncIgnoreCase(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	// Only test if filesystems are case sensitive
	if r.Fremote.Features().CaseInsensitive || r.Flocal.Features().CaseInsensitive {
		t.Skip("Skipping test as local or remote are case-insensitive")
	}

	fs.Config.IgnoreCaseSync = true
	defer func() { fs.Config.IgnoreCaseSync = false }()

	// Create files with different filename casing
	file1 := r.WriteFile("existing", "potato", t1)
	fstest.CheckItems(t, r.Flocal, file1)
	file2 := r.WriteObject(context.Background(), "EXISTING", "potato", t1)
	fstest.CheckItems(t, r.Fremote, file2)

	// Should not copy files that are differently-cased but otherwise identical
	accounting.GlobalStats().ResetCounters()
	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file2)
}

// Test that aborting on max upload works
func TestAbort(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	if r.Fremote.Name() != "local" {
		t.Skip("This test only runs on local")
	}

	oldMaxTransfer := fs.Config.MaxTransfer
	oldTransfers := fs.Config.Transfers
	oldCheckers := fs.Config.Checkers
	fs.Config.MaxTransfer = 3 * 1024
	fs.Config.Transfers = 1
	fs.Config.Checkers = 1
	defer func() {
		fs.Config.MaxTransfer = oldMaxTransfer
		fs.Config.Transfers = oldTransfers
		fs.Config.Checkers = oldCheckers
	}()

	// Create file on source
	file1 := r.WriteFile("file1", string(make([]byte, 5*1024)), t1)
	file2 := r.WriteFile("file2", string(make([]byte, 2*1024)), t1)
	file3 := r.WriteFile("file3", string(make([]byte, 3*1024)), t1)
	fstest.CheckItems(t, r.Flocal, file1, file2, file3)
	fstest.CheckItems(t, r.Fremote)

	accounting.GlobalStats().ResetCounters()

	err := Sync(context.Background(), r.Fremote, r.Flocal, false)
	expectedErr := fserrors.FsError(accounting.ErrorMaxTransferLimitReachedFatal)
	fserrors.Count(expectedErr)
	assert.Equal(t, expectedErr, err)
}
