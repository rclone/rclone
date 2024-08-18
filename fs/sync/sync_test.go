// Test sync/copy/move

package sync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	mutex "sync" // renamed as "sync" already in use

	_ "github.com/rclone/rclone/backend/all" // import all backends
	"github.com/rclone/rclone/cmd/bisync/bilib"
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
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	r.Mkdir(ctx, r.Fremote)

	ci.DryRun = true
	ctx = predictDstFromLogger(ctx)
	err := CopyDir(ctx, r.Fremote, r.Flocal, false)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t) // error expected here because dry-run
	require.NoError(t, err)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t)
}

// Now without dry run
func TestCopy(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	_, err := operations.SetDirModTime(ctx, r.Flocal, nil, "sub dir", t2)
	if err != nil && !errors.Is(err, fs.ErrorNotImplemented) {
		require.NoError(t, err)
	}
	r.Mkdir(ctx, r.Fremote)

	ctx = predictDstFromLogger(ctx)
	err = CopyDir(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file1)

	// Check that the modtimes of the directories are as expected
	r.CheckDirectoryModTimes(t, "sub dir")
}

func testCopyMetadata(t *testing.T, createEmptySrcDirs bool) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	ci.Metadata = true
	r := fstest.NewRun(t)
	features := r.Fremote.Features()

	if !features.ReadMetadata && !features.WriteMetadata && !features.UserMetadata &&
		!features.ReadDirMetadata && !features.WriteDirMetadata && !features.UserDirMetadata {
		t.Skip("Skipping as metadata not supported")
	}

	if createEmptySrcDirs && !features.CanHaveEmptyDirectories {
		t.Skip("Skipping as can't have empty directories")
	}

	const content = "hello metadata world!"
	const dirPath = "metadata sub dir"
	const emptyDirPath = "empty metadata sub dir"
	const filePath = dirPath + "/hello metadata world"

	fileMetadata := fs.Metadata{
		// System metadata supported by all backends
		"mtime": t1.Format(time.RFC3339Nano),
		// User metadata
		"potato": "jersey",
	}

	dirMetadata := fs.Metadata{
		// System metadata supported by all backends
		"mtime": t2.Format(time.RFC3339Nano),
		// User metadata
		"potato": "king edward",
	}

	// Make the directory with metadata - may fall back to Mkdir
	_, err := operations.MkdirMetadata(ctx, r.Flocal, dirPath, dirMetadata)
	require.NoError(t, err)

	// Make the empty directory with metadata - may fall back to Mkdir
	_, err = operations.MkdirMetadata(ctx, r.Flocal, emptyDirPath, dirMetadata)
	require.NoError(t, err)

	// Upload the file with metadata
	in := io.NopCloser(bytes.NewBufferString(content))
	_, err = operations.Rcat(ctx, r.Flocal, filePath, in, t1, fileMetadata)
	require.NoError(t, err)
	file1 := fstest.NewItem(filePath, content, t1)

	// Reset the time of the directory
	_, err = operations.SetDirModTime(ctx, r.Flocal, nil, dirPath, t2)
	if err != nil && !errors.Is(err, fs.ErrorNotImplemented) {
		require.NoError(t, err)
	}

	ctx = predictDstFromLogger(ctx)
	err = CopyDir(ctx, r.Fremote, r.Flocal, createEmptySrcDirs)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file1)

	// Check that the modtimes of the directories are as expected
	r.CheckDirectoryModTimes(t, dirPath)

	// Check that the metadata on the directory and file is correct
	if features.WriteMetadata && features.ReadMetadata {
		fstest.CheckEntryMetadata(ctx, t, r.Fremote, fstest.NewObject(ctx, t, r.Fremote, filePath), fileMetadata)
	}
	if features.WriteDirMetadata && features.ReadDirMetadata {
		fstest.CheckEntryMetadata(ctx, t, r.Fremote, fstest.NewDirectory(ctx, t, r.Fremote, dirPath), dirMetadata)
	}
	if !createEmptySrcDirs {
		// dir must not exist
		_, err := fstest.NewDirectoryRetries(ctx, t, r.Fremote, emptyDirPath, 1)
		assert.Error(t, err, "Not expecting to find empty directory")
		assert.True(t, errors.Is(err, fs.ErrorDirNotFound), fmt.Sprintf("expecting wrapped %#v not: %#v", fs.ErrorDirNotFound, err))
	} else {
		// dir must exist
		dir := fstest.NewDirectory(ctx, t, r.Fremote, emptyDirPath)
		if features.ReadDirMetadata {
			fstest.CheckEntryMetadata(ctx, t, r.Fremote, dir, dirMetadata)
		}
	}
}

func TestCopyMetadata(t *testing.T) {
	testCopyMetadata(t, true)
}

func TestCopyMetadataNoEmptyDirs(t *testing.T) {
	testCopyMetadata(t, false)
}

func TestCopyMissingDirectory(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	r.Mkdir(ctx, r.Fremote)

	nonExistingFs, err := fs.NewFs(ctx, "/non-existing")
	if err != nil {
		t.Fatal(err)
	}

	ctx = predictDstFromLogger(ctx)
	err = CopyDir(ctx, r.Fremote, nonExistingFs, false)
	require.Error(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
}

// Now with --no-traverse
func TestCopyNoTraverse(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)

	ci.NoTraverse = true

	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)

	ctx = predictDstFromLogger(ctx)
	err := CopyDir(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file1)
}

// Now with --check-first
func TestCopyCheckFirst(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)

	ci.CheckFirst = true

	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)

	ctx = predictDstFromLogger(ctx)
	err := CopyDir(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file1)
}

// Now with --no-traverse
func TestSyncNoTraverse(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)

	ci.NoTraverse = true

	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file1)
}

// Test copy with depth
func TestCopyWithDepth(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	file2 := r.WriteFile("hello world2", "hello world2", t2)

	// Check the MaxDepth too
	ci.MaxDepth = 1

	ctx = predictDstFromLogger(ctx)
	err := CopyDir(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	r.CheckLocalItems(t, file1, file2)
	r.CheckRemoteItems(t, file2)
}

// Test copy with files from
func testCopyWithFilesFrom(t *testing.T, noTraverse bool) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
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

	ctx = predictDstFromLogger(ctx)
	err = CopyDir(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	r.CheckLocalItems(t, file1, file2)
	r.CheckRemoteItems(t, file1)
}
func TestCopyWithFilesFrom(t *testing.T)              { testCopyWithFilesFrom(t, false) }
func TestCopyWithFilesFromAndNoTraverse(t *testing.T) { testCopyWithFilesFrom(t, true) }

// Test copy empty directories
func TestCopyEmptyDirectories(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	_, err := operations.MkdirModTime(ctx, r.Flocal, "sub dir2/sub sub dir2", t2)
	require.NoError(t, err)
	_, err = operations.SetDirModTime(ctx, r.Flocal, nil, "sub dir2", t2)
	require.NoError(t, err)
	r.Mkdir(ctx, r.Fremote)

	// Set the modtime on "sub dir" to something specific
	// Without this it fails on the CI and in VirtualBox with variances of up to 10mS
	_, err = operations.SetDirModTime(ctx, r.Flocal, nil, "sub dir", t1)
	require.NoError(t, err)

	ctx = predictDstFromLogger(ctx)
	err = CopyDir(ctx, r.Fremote, r.Flocal, true)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	r.CheckRemoteListing(
		t,
		[]fstest.Item{
			file1,
		},
		[]string{
			"sub dir",
			"sub dir2",
			"sub dir2/sub sub dir2",
		},
	)

	// Check that the modtimes of the directories are as expected
	r.CheckDirectoryModTimes(t, "sub dir", "sub dir2", "sub dir2/sub sub dir2")
}

// Test copy empty directories when we are configured not to create them
func TestCopyNoEmptyDirectories(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	err := operations.Mkdir(ctx, r.Flocal, "sub dir2")
	require.NoError(t, err)
	_, err = operations.MkdirModTime(ctx, r.Flocal, "sub dir2/sub sub dir2", t2)
	require.NoError(t, err)
	r.Mkdir(ctx, r.Fremote)

	err = CopyDir(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	r.CheckRemoteListing(
		t,
		[]fstest.Item{
			file1,
		},
		[]string{
			"sub dir",
		},
	)
}

// Test move empty directories
func TestMoveEmptyDirectories(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	_, err := operations.MkdirModTime(ctx, r.Flocal, "sub dir2", t2)
	require.NoError(t, err)
	subDir := fstest.NewDirectory(ctx, t, r.Flocal, "sub dir")
	subDirT := subDir.ModTime(ctx)
	r.Mkdir(ctx, r.Fremote)

	ctx = predictDstFromLogger(ctx)
	err = MoveDir(ctx, r.Fremote, r.Flocal, false, true)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

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

	// Check that the modtimes of the directories are as expected
	r.CheckDirectoryModTimes(t, "sub dir2")
	// Note that "sub dir" mod time is updated when file1 is deleted from it
	// So check it more manually
	got := fstest.NewDirectory(ctx, t, r.Fremote, "sub dir")
	fstest.CheckDirModTime(ctx, t, r.Fremote, got, subDirT)
}

// Test that --no-update-dir-modtime is working
func TestSyncNoUpdateDirModtime(t *testing.T) {
	r := fstest.NewRun(t)
	if r.Fremote.Features().DirSetModTime == nil {
		t.Skip("Skipping test as backend does not support DirSetModTime")
	}

	ctx, ci := fs.AddConfig(context.Background())
	ci.NoUpdateDirModTime = true
	const name = "sub dir no update dir modtime"

	// Set the modtime on name to something specific
	_, err := operations.MkdirModTime(ctx, r.Flocal, name, t1)
	require.NoError(t, err)

	// Create the remote directory with the current time
	require.NoError(t, r.Fremote.Mkdir(ctx, name))

	// Read its modification time
	wantT := fstest.NewDirectory(ctx, t, r.Fremote, name).ModTime(ctx)

	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, true)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	r.CheckRemoteListing(
		t,
		[]fstest.Item{},
		[]string{
			name,
		},
	)

	// Read the new directory modification time - it should not have changed
	gotT := fstest.NewDirectory(ctx, t, r.Fremote, name).ModTime(ctx)
	fstest.AssertTimeEqualWithPrecision(t, name, wantT, gotT, r.Fremote.Precision())
}

// Test move empty directories when we are not configured to create them
func TestMoveNoEmptyDirectories(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	err := operations.Mkdir(ctx, r.Flocal, "sub dir2")
	require.NoError(t, err)
	r.Mkdir(ctx, r.Fremote)

	err = MoveDir(ctx, r.Fremote, r.Flocal, false, false)
	require.NoError(t, err)

	r.CheckRemoteListing(
		t,
		[]fstest.Item{
			file1,
		},
		[]string{
			"sub dir",
		},
	)
}

// Test sync empty directories
func TestSyncEmptyDirectories(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	_, err := operations.MkdirModTime(ctx, r.Flocal, "sub dir2", t2)
	require.NoError(t, err)

	// Set the modtime on "sub dir" to something specific
	// Without this it fails on the CI and in VirtualBox with variances of up to 10mS
	_, err = operations.SetDirModTime(ctx, r.Flocal, nil, "sub dir", t1)
	require.NoError(t, err)

	r.Mkdir(ctx, r.Fremote)

	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, true)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

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

	// Check that the modtimes of the directories are as expected
	r.CheckDirectoryModTimes(t, "sub dir", "sub dir2")
}

// Test delayed mod time setting
func TestSyncSetDelayedModTimes(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)

	if !r.Fremote.Features().DirModTimeUpdatesOnWrite {
		t.Skip("Backend doesn't have DirModTimeUpdatesOnWrite set")
	}

	// Create directories without timestamps
	require.NoError(t, r.Flocal.Mkdir(ctx, "a1/b1/c1/d1/e1/f1"))
	require.NoError(t, r.Flocal.Mkdir(ctx, "a1/b2/c1/d1/e1/f1"))
	require.NoError(t, r.Flocal.Mkdir(ctx, "a1/b1/c1/d2/e1/f1"))
	require.NoError(t, r.Flocal.Mkdir(ctx, "a1/b1/c1/d2/e1/f2"))

	dirs := []string{
		"a1",
		"a1/b1",
		"a1/b1/c1",
		"a1/b1/c1/d1",
		"a1/b1/c1/d1/e1",
		"a1/b1/c1/d1/e1/f1",
		"a1/b1/c1/d2",
		"a1/b1/c1/d2/e1",
		"a1/b1/c1/d2/e1/f1",
		"a1/b1/c1/d2/e1/f2",
		"a1/b2",
		"a1/b2/c1",
		"a1/b2/c1/d1",
		"a1/b2/c1/d1/e1",
		"a1/b2/c1/d1/e1/f1",
	}
	r.CheckLocalListing(t, []fstest.Item{}, dirs)

	// Timestamp the directories in reverse order
	ts := t1
	for i := len(dirs) - 1; i >= 0; i-- {
		dir := dirs[i]
		_, err := operations.SetDirModTime(ctx, r.Flocal, nil, dir, ts)
		require.NoError(t, err)
		ts = ts.Add(time.Minute)
	}

	r.Mkdir(ctx, r.Fremote)

	ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, true)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	r.CheckRemoteListing(t, []fstest.Item{}, dirs)

	// Check that the modtimes of the directories are as expected
	r.CheckDirectoryModTimes(t, dirs...)
}

// Test sync empty directories when we are not configured to create them
func TestSyncNoEmptyDirectories(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	err := operations.Mkdir(ctx, r.Flocal, "sub dir2")
	require.NoError(t, err)
	r.Mkdir(ctx, r.Fremote)

	err = Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)

	r.CheckRemoteListing(
		t,
		[]fstest.Item{
			file1,
		},
		[]string{
			"sub dir",
		},
	)
}

// Test a server-side copy if possible, or the backup path if not
func TestServerSideCopy(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	file1 := r.WriteObject(ctx, "sub dir/hello world", "hello world", t1)
	r.CheckRemoteItems(t, file1)

	FremoteCopy, _, finaliseCopy, err := fstest.RandomRemote()
	require.NoError(t, err)
	defer finaliseCopy()
	t.Logf("Server side copy (if possible) %v -> %v", r.Fremote, FremoteCopy)

	ctx = predictDstFromLogger(ctx)
	err = CopyDir(ctx, FremoteCopy, r.Fremote, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	fstest.CheckItems(t, FremoteCopy, file1)
}

// Check that if the local file doesn't exist when we copy it up,
// nothing happens to the remote file
func TestCopyAfterDelete(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	file1 := r.WriteObject(ctx, "sub dir/hello world", "hello world", t1)
	r.CheckLocalItems(t)
	r.CheckRemoteItems(t, file1)

	err := operations.Mkdir(ctx, r.Flocal, "")
	require.NoError(t, err)

	ctx = predictDstFromLogger(ctx)
	err = CopyDir(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	r.CheckLocalItems(t)
	r.CheckRemoteItems(t, file1)
}

// Check the copy downloading a file
func TestCopyRedownload(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	file1 := r.WriteObject(ctx, "sub dir/hello world", "hello world", t1)
	r.CheckRemoteItems(t, file1)

	ctx = predictDstFromLogger(ctx)
	err := CopyDir(ctx, r.Flocal, r.Fremote, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

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
	ci.CheckSum = true

	file1 := r.WriteFile("check sum", "-", t1)
	r.CheckLocalItems(t, file1)

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	// We should have transferred exactly one file.
	assert.Equal(t, toyFileTransfers(r), accounting.GlobalStats().GetTransfers())
	r.CheckRemoteItems(t, file1)

	// Change last modified date only
	file2 := r.WriteFile("check sum", "-", t2)
	r.CheckLocalItems(t, file2)

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

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
	ci.SizeOnly = true

	file1 := r.WriteFile("sizeonly", "potato", t1)
	r.CheckLocalItems(t, file1)

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	// We should have transferred exactly one file.
	assert.Equal(t, toyFileTransfers(r), accounting.GlobalStats().GetTransfers())
	r.CheckRemoteItems(t, file1)

	// Update mtime, md5sum but not length of file
	file2 := r.WriteFile("sizeonly", "POTATO", t2)
	r.CheckLocalItems(t, file2)

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

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
	ci.IgnoreSize = true

	file1 := r.WriteFile("ignore-size", "contents", t1)
	r.CheckLocalItems(t, file1)

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	// We should have transferred exactly one file.
	assert.Equal(t, toyFileTransfers(r), accounting.GlobalStats().GetTransfers())
	r.CheckRemoteItems(t, file1)

	// Update size but not date of file
	file2 := r.WriteFile("ignore-size", "longer contents but same date", t1)
	r.CheckLocalItems(t, file2)

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	// We should have transferred no files
	assert.Equal(t, int64(0), accounting.GlobalStats().GetTransfers())
	r.CheckLocalItems(t, file2)
	r.CheckRemoteItems(t, file1)
}

func TestSyncIgnoreTimes(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	file1 := r.WriteBoth(ctx, "existing", "potato", t1)
	r.CheckRemoteItems(t, file1)

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	// We should have transferred exactly 0 files because the
	// files were identical.
	assert.Equal(t, int64(0), accounting.GlobalStats().GetTransfers())

	ci.IgnoreTimes = true

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

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
	file1 := r.WriteFile("existing", "potato", t1)

	ci.IgnoreExisting = true

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file1)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	// Change everything
	r.WriteFile("existing", "newpotatoes", t2)
	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	// Items should not change
	r.CheckRemoteItems(t, file1)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
}

func TestSyncIgnoreErrors(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	ci.IgnoreErrors = true
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
	ctx = predictDstFromLogger(ctx)
	_ = fs.CountError(errors.New("boom"))
	assert.NoError(t, Sync(ctx, r.Fremote, r.Flocal, false))
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

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
	file1 := r.WriteFile("empty space", "-", t2)
	file2 := r.WriteObject(ctx, "empty space", "-", t1)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file2)

	ci.DryRun = true

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file2)

	ci.DryRun = false

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file1)
}

func TestSyncAfterChangingModtimeOnlyWithNoUpdateModTime(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)

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
	ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file2)
}

func TestSyncDoesntUpdateModtime(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	if fs.GetModifyWindow(ctx, r.Fremote) == fs.ModTimeNotSupported {
		t.Skip("Can't run this test on fs which doesn't support mod time")
	}

	file1 := r.WriteFile("foo", "foo", t2)
	file2 := r.WriteObject(ctx, "foo", "bar", t1)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file2)

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file1)

	// We should have transferred exactly one file, not set the mod time
	assert.Equal(t, toyFileTransfers(r), accounting.GlobalStats().GetTransfers())
}

func TestSyncAfterAddingAFile(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	file1 := r.WriteBoth(ctx, "empty space", "-", t2)
	file2 := r.WriteFile("potato", "------------------------------------------------------------", t3)

	r.CheckLocalItems(t, file1, file2)
	r.CheckRemoteItems(t, file1)

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
	r.CheckLocalItems(t, file1, file2)
	r.CheckRemoteItems(t, file1, file2)
}

func TestSyncAfterChangingFilesSizeOnly(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	file1 := r.WriteObject(ctx, "potato", "------------------------------------------------------------", t3)
	file2 := r.WriteFile("potato", "smaller but same date", t3)
	r.CheckRemoteItems(t, file1)
	r.CheckLocalItems(t, file2)

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
	r.CheckLocalItems(t, file2)
	r.CheckRemoteItems(t, file2)
}

// Sync after changing a file's contents, changing modtime but length
// remaining the same
func TestSyncAfterChangingContentsOnly(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
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
	ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
	r.CheckLocalItems(t, file2)
	r.CheckRemoteItems(t, file2)
}

// Sync after removing a file and adding a file --dry-run
func TestSyncAfterRemovingAFileAndAddingAFileDryRun(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	file1 := r.WriteFile("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteObject(ctx, "potato", "SMALLER BUT SAME DATE", t2)
	file3 := r.WriteBoth(ctx, "empty space", "-", t2)

	ci.DryRun = true
	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	ci.DryRun = false
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	r.CheckLocalItems(t, file3, file1)
	r.CheckRemoteItems(t, file3, file2)
}

// Sync after removing a file and adding a file
func testSyncAfterRemovingAFileAndAddingAFile(ctx context.Context, t *testing.T) {
	r := fstest.NewRun(t)
	file1 := r.WriteFile("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteObject(ctx, "potato", "SMALLER BUT SAME DATE", t2)
	file3 := r.WriteBoth(ctx, "empty space", "-", t2)
	r.CheckRemoteItems(t, file2, file3)
	r.CheckLocalItems(t, file1, file3)

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
	r.CheckLocalItems(t, file1, file3)
	r.CheckRemoteItems(t, file1, file3)
}

func TestSyncAfterRemovingAFileAndAddingAFile(t *testing.T) {
	testSyncAfterRemovingAFileAndAddingAFile(context.Background(), t)
}

// Sync after removing a file and adding a file
func testSyncAfterRemovingAFileAndAddingAFileSubDir(ctx context.Context, t *testing.T) {
	r := fstest.NewRun(t)
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
	ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

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

	ctx = predictDstFromLogger(ctx)
	accounting.GlobalStats().ResetCounters()
	_ = fs.CountError(errors.New("boom"))
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	assert.Equal(t, fs.ErrorNotDeleting, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

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

	ci.DeleteMode = fs.DeleteModeBefore

	file1 := r.WriteObject(ctx, "potato", "hopefully not deleted", t1)
	file2 := r.WriteFile("potato2", "hopefully copied in", t1)
	r.CheckRemoteItems(t, file1)
	r.CheckLocalItems(t, file2)

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err := CopyDir(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	r.CheckRemoteItems(t, file1, file2)
	r.CheckLocalItems(t, file2)
}

// Test with exclude
func TestSyncWithExclude(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
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
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
	r.CheckRemoteItems(t, file2, file1)

	// Now sync the other way round and check enormous doesn't get
	// deleted as it is excluded from the sync
	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Flocal, r.Fremote, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
	r.CheckLocalItems(t, file2, file1, file3)
}

// Test with exclude and delete excluded
func TestSyncWithExcludeAndDeleteExcluded(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
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
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
	r.CheckRemoteItems(t, file2)

	// Check sync the other way round to make sure enormous gets
	// deleted even though it is excluded
	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Flocal, r.Fremote, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
	r.CheckLocalItems(t, file2)
}

// Test with UpdateOlder set
func TestSyncWithUpdateOlder(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
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

	ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	r.CheckRemoteItems(t, oneO, twoF, threeO, fourF, fiveF)
	// testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t) // no modtime

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
func testSyncWithMaxDuration(t *testing.T, cutoffMode fs.CutoffMode) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	if *fstest.RemoteName != "" {
		t.Skip("Skipping test on non local remote")
	}
	r := fstest.NewRun(t)

	maxDuration := 250 * time.Millisecond
	ci.MaxDuration = maxDuration
	ci.CutoffMode = cutoffMode
	ci.CheckFirst = true
	ci.OrderBy = "size"
	ci.Transfers = 1
	ci.Checkers = 1
	bytesPerSecond := 10 * 1024
	accounting.TokenBucket.SetBwLimit(fs.BwPair{Tx: fs.SizeSuffix(bytesPerSecond), Rx: fs.SizeSuffix(bytesPerSecond)})
	defer accounting.TokenBucket.SetBwLimit(fs.BwPair{Tx: -1, Rx: -1})

	// write one small file which we expect to transfer and one big one which we don't
	file1 := r.WriteFile("file1", string(make([]byte, 16)), t1)
	file2 := r.WriteFile("file2", string(make([]byte, 50*1024)), t1)
	r.CheckLocalItems(t, file1, file2)
	r.CheckRemoteItems(t)

	if runtime.GOOS == "darwin" {
		r.Flocal.Features().Disable("Copy") // macOS cloning is too fast for this test!
		if r.Fremote.Features().IsLocal {
			r.Fremote.Features().Disable("Copy") // macOS cloning is too fast for this test!
		}
	}
	accounting.GlobalStats().ResetCounters()
	// ctx = predictDstFromLogger(ctx) // not currently supported (but tests do pass for CutoffModeSoft)
	startTime := time.Now()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	// testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
	require.True(t, errors.Is(err, ErrorMaxDurationReached))

	if cutoffMode == fs.CutoffModeHard {
		r.CheckRemoteItems(t, file1)
		assert.Equal(t, int64(1), accounting.GlobalStats().GetTransfers())
	} else {
		r.CheckRemoteItems(t, file1, file2)
		assert.Equal(t, int64(2), accounting.GlobalStats().GetTransfers())
	}

	elapsed := time.Since(startTime)
	const maxTransferTime = 20 * time.Second

	what := fmt.Sprintf("expecting elapsed time %v between %v and %v", elapsed, maxDuration, maxTransferTime)
	assert.True(t, elapsed >= maxDuration, what)
	assert.True(t, elapsed < maxTransferTime, what)
}

func TestSyncWithMaxDuration(t *testing.T) {
	t.Run("Hard", func(t *testing.T) {
		testSyncWithMaxDuration(t, fs.CutoffModeHard)
	})
	t.Run("Soft", func(t *testing.T) {
		testSyncWithMaxDuration(t, fs.CutoffModeSoft)
	})
}

// Test with TrackRenames set
func TestSyncWithTrackRenames(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)

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
	ctx = predictDstFromLogger(ctx)
	require.NoError(t, Sync(ctx, r.Fremote, r.Flocal, false))
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	r.CheckRemoteItems(t, f1, f2)
	r.CheckLocalItems(t, f1, f2)

	// Now rename locally.
	f2 = r.RenameFile(f2, "yaml")

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	require.NoError(t, Sync(ctx, r.Fremote, r.Flocal, false))
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

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

	ci.TrackRenames = true
	ci.TrackRenamesStrategy = "modtime"

	canTrackRenames := operations.CanServerSideMove(r.Fremote) && r.Fremote.Precision() != fs.ModTimeNotSupported
	t.Logf("Can track renames: %v", canTrackRenames)

	f1 := r.WriteFile("potato", "Potato Content", t1)
	f2 := r.WriteFile("yam", "Yam Content", t2)

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	require.NoError(t, Sync(ctx, r.Fremote, r.Flocal, false))
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	r.CheckRemoteItems(t, f1, f2)
	r.CheckLocalItems(t, f1, f2)

	// Now rename locally.
	f2 = r.RenameFile(f2, "yaml")

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	require.NoError(t, Sync(ctx, r.Fremote, r.Flocal, false))
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

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

	ci.TrackRenames = true
	ci.TrackRenamesStrategy = "leaf"

	canTrackRenames := operations.CanServerSideMove(r.Fremote) && r.Fremote.Precision() != fs.ModTimeNotSupported
	t.Logf("Can track renames: %v", canTrackRenames)

	f1 := r.WriteFile("potato", "Potato Content", t1)
	f2 := r.WriteFile("sub/yam", "Yam Content", t2)

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	require.NoError(t, Sync(ctx, r.Fremote, r.Flocal, false))
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	r.CheckRemoteItems(t, f1, f2)
	r.CheckLocalItems(t, f1, f2)

	// Now rename locally.
	f2 = r.RenameFile(f2, "yam")

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	require.NoError(t, Sync(ctx, r.Fremote, r.Flocal, false))
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

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
	// ctx = predictDstFromLogger(ctx) // not currently supported -- doesn't list all contents of dir.
	err = MoveDir(ctx, FremoteMove, r.Fremote, testDeleteEmptyDirs, false)
	require.NoError(t, err)
	// testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

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
	// ctx = predictDstFromLogger(ctx)
	err = MoveDir(ctx, FremoteMove2, FremoteMove, testDeleteEmptyDirs, false)
	require.NoError(t, err)
	// testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

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

// Test MoveDir on Local
func TestServerSideMoveLocal(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	f1 := r.WriteFile("dir1/file1.txt", "hello", t1)
	f2 := r.WriteFile("dir2/file2.txt", "hello again", t2)
	r.CheckLocalItems(t, f1, f2)

	dir1, err := fs.NewFs(ctx, r.Flocal.Root()+"/dir1")
	require.NoError(t, err)
	dir2, err := fs.NewFs(ctx, r.Flocal.Root()+"/dir2")
	require.NoError(t, err)
	err = MoveDir(ctx, dir2, dir1, false, false)
	require.NoError(t, err)
}

// Test move
func TestMoveWithDeleteEmptySrcDirs(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	file2 := r.WriteFile("nested/sub dir/file", "nested", t1)
	r.Mkdir(ctx, r.Fremote)

	// run move with --delete-empty-src-dirs
	ctx = predictDstFromLogger(ctx)
	err := MoveDir(ctx, r.Fremote, r.Flocal, true, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

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
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	file2 := r.WriteFile("nested/sub dir/file", "nested", t1)
	r.Mkdir(ctx, r.Fremote)

	ctx = predictDstFromLogger(ctx)
	err := MoveDir(ctx, r.Fremote, r.Flocal, false, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

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
	file1 := r.WriteFile("existing", "potato", t1)
	file2 := r.WriteFile("existing-b", "tomato", t1)

	ci.IgnoreExisting = true

	accounting.GlobalStats().ResetCounters()
	// ctx = predictDstFromLogger(ctx)
	err := MoveDir(ctx, r.Fremote, r.Flocal, false, false)
	require.NoError(t, err)
	// testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
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
	ctx = predictDstFromLogger(ctx)
	err = MoveDir(ctx, r.Fremote, r.Flocal, false, false)
	require.NoError(t, err)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
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
	testServerSideMove(ctx, t, r, false, false)
}

// Test a server-side move if possible, or the backup path if not
func TestServerSideMoveWithFilter(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)

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
	testServerSideMove(ctx, t, r, false, true)
}

// Test a server-side move with overlap
func TestServerSideMoveOverlap(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)

	if r.Fremote.Features().DirMove != nil {
		t.Skip("Skipping test as remote supports DirMove")
	}

	subRemoteName := r.FremoteName + "/rclone-move-test"
	FremoteMove, err := fs.NewFs(ctx, subRemoteName)
	require.NoError(t, err)

	file1 := r.WriteObject(ctx, "potato2", "------------------------------------------------------------", t1)
	r.CheckRemoteItems(t, file1)

	// Subdir move with no filters should return ErrorCantMoveOverlapping
	// ctx = predictDstFromLogger(ctx)
	err = MoveDir(ctx, FremoteMove, r.Fremote, false, false)
	assert.EqualError(t, err, fs.ErrorOverlapping.Error())
	// testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)

	// Now try with a filter which should also fail with ErrorCantMoveOverlapping
	fi, err := filter.NewFilter(nil)
	require.NoError(t, err)
	fi.Opt.MinSize = 40
	ctx = filter.ReplaceConfig(ctx, fi)

	// ctx = predictDstFromLogger(ctx)
	err = MoveDir(ctx, FremoteMove, r.Fremote, false, false)
	assert.EqualError(t, err, fs.ErrorOverlapping.Error())
	// testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
}

// Test a sync with overlap
func TestSyncOverlap(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)

	subRemoteName := r.FremoteName + "/rclone-sync-test"
	FremoteSync, err := fs.NewFs(ctx, subRemoteName)
	require.NoError(t, err)

	checkErr := func(err error) {
		require.Error(t, err)
		assert.True(t, fserrors.IsFatalError(err))
		assert.Equal(t, fs.ErrorOverlapping.Error(), err.Error())
	}

	ctx = predictDstFromLogger(ctx)
	checkErr(Sync(ctx, FremoteSync, r.Fremote, false))
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
	ctx = predictDstFromLogger(ctx)
	checkErr(Sync(ctx, r.Fremote, FremoteSync, false))
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
	ctx = predictDstFromLogger(ctx)
	checkErr(Sync(ctx, r.Fremote, r.Fremote, false))
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
	ctx = predictDstFromLogger(ctx)
	checkErr(Sync(ctx, FremoteSync, FremoteSync, false))
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
}

// Test a sync with filtered overlap
func TestSyncOverlapWithFilter(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)

	fi, err := filter.NewFilter(nil)
	require.NoError(t, err)
	require.NoError(t, fi.Add(false, "/rclone-sync-test/"))
	require.NoError(t, fi.Add(false, "*/layer2/"))
	fi.Opt.ExcludeFile = []string{".ignore"}
	filterCtx := filter.ReplaceConfig(ctx, fi)

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
	filterCtx = predictDstFromLogger(filterCtx)
	checkNoErr(Sync(filterCtx, FremoteSync, r.Fremote, false))
	checkErr(Sync(ctx, FremoteSync, r.Fremote, false))
	checkNoErr(Sync(filterCtx, r.Fremote, FremoteSync, false))
	testLoggerVsLsf(filterCtx, r.Fremote, operations.GetLoggerOpt(filterCtx).JSON, t)
	filterCtx = predictDstFromLogger(filterCtx)
	checkErr(Sync(ctx, r.Fremote, FremoteSync, false))
	testLoggerVsLsf(filterCtx, r.Fremote, operations.GetLoggerOpt(filterCtx).JSON, t)
	filterCtx = predictDstFromLogger(filterCtx)
	checkErr(Sync(filterCtx, r.Fremote, r.Fremote, false))
	testLoggerVsLsf(filterCtx, r.Fremote, operations.GetLoggerOpt(filterCtx).JSON, t)
	filterCtx = predictDstFromLogger(filterCtx)
	checkErr(Sync(ctx, r.Fremote, r.Fremote, false))
	testLoggerVsLsf(filterCtx, r.Fremote, operations.GetLoggerOpt(filterCtx).JSON, t)
	filterCtx = predictDstFromLogger(filterCtx)
	checkErr(Sync(filterCtx, FremoteSync, FremoteSync, false))
	testLoggerVsLsf(filterCtx, r.Fremote, operations.GetLoggerOpt(filterCtx).JSON, t)
	filterCtx = predictDstFromLogger(filterCtx)
	checkErr(Sync(ctx, FremoteSync, FremoteSync, false))
	testLoggerVsLsf(filterCtx, r.Fremote, operations.GetLoggerOpt(filterCtx).JSON, t)
	filterCtx = predictDstFromLogger(filterCtx)

	checkNoErr(Sync(filterCtx, FremoteSync2, r.Fremote, false))
	testLoggerVsLsf(filterCtx, r.Fremote, operations.GetLoggerOpt(filterCtx).JSON, t)
	filterCtx = predictDstFromLogger(filterCtx)
	checkErr(Sync(ctx, FremoteSync2, r.Fremote, false))
	testLoggerVsLsf(filterCtx, r.Fremote, operations.GetLoggerOpt(filterCtx).JSON, t)
	filterCtx = predictDstFromLogger(filterCtx)
	checkNoErr(Sync(filterCtx, r.Fremote, FremoteSync2, false))
	testLoggerVsLsf(filterCtx, r.Fremote, operations.GetLoggerOpt(filterCtx).JSON, t)
	filterCtx = predictDstFromLogger(filterCtx)
	checkErr(Sync(ctx, r.Fremote, FremoteSync2, false))
	testLoggerVsLsf(filterCtx, r.Fremote, operations.GetLoggerOpt(filterCtx).JSON, t)
	filterCtx = predictDstFromLogger(filterCtx)
	checkErr(Sync(filterCtx, FremoteSync2, FremoteSync2, false))
	testLoggerVsLsf(filterCtx, r.Fremote, operations.GetLoggerOpt(filterCtx).JSON, t)
	filterCtx = predictDstFromLogger(filterCtx)
	checkErr(Sync(ctx, FremoteSync2, FremoteSync2, false))
	testLoggerVsLsf(filterCtx, r.Fremote, operations.GetLoggerOpt(filterCtx).JSON, t)
	filterCtx = predictDstFromLogger(filterCtx)

	checkNoErr(Sync(filterCtx, FremoteSync3, r.Fremote, false))
	testLoggerVsLsf(filterCtx, r.Fremote, operations.GetLoggerOpt(filterCtx).JSON, t)
	filterCtx = predictDstFromLogger(filterCtx)
	checkErr(Sync(ctx, FremoteSync3, r.Fremote, false))
	testLoggerVsLsf(filterCtx, r.Fremote, operations.GetLoggerOpt(filterCtx).JSON, t)
	filterCtx = predictDstFromLogger(filterCtx)
	// Destination is excluded so this test makes no sense
	// checkNoErr(Sync(filterCtx, r.Fremote, FremoteSync3, false))
	checkErr(Sync(ctx, r.Fremote, FremoteSync3, false))
	testLoggerVsLsf(filterCtx, r.Fremote, operations.GetLoggerOpt(filterCtx).JSON, t)
	filterCtx = predictDstFromLogger(filterCtx)
	checkErr(Sync(filterCtx, FremoteSync3, FremoteSync3, false))
	testLoggerVsLsf(filterCtx, r.Fremote, operations.GetLoggerOpt(filterCtx).JSON, t)
	filterCtx = predictDstFromLogger(filterCtx)
	checkErr(Sync(ctx, FremoteSync3, FremoteSync3, false))
	testLoggerVsLsf(filterCtx, r.Fremote, operations.GetLoggerOpt(filterCtx).JSON, t)
}

// Test with CompareDest set
func TestSyncCompareDest(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)

	ci.CompareDest = []string{r.FremoteName + "/CompareDest"}

	fdst, err := fs.NewFs(ctx, r.FremoteName+"/dst")
	require.NoError(t, err)

	// check empty dest, empty compare
	file1 := r.WriteFile("one", "one", t1)
	r.CheckLocalItems(t, file1)

	accounting.GlobalStats().ResetCounters()
	// ctx = predictDstFromLogger(ctx) // not currently supported due to duplicate equal() checks
	err = Sync(ctx, fdst, r.Flocal, false)
	// testLoggerVsLsf(ctx, fdst, operations.GetLoggerOpt(ctx).JSON, t)
	require.NoError(t, err)

	file1dst := file1
	file1dst.Path = "dst/one"

	r.CheckRemoteItems(t, file1dst)

	// check old dest, empty compare
	file1b := r.WriteFile("one", "onet2", t2)
	r.CheckRemoteItems(t, file1dst)
	r.CheckLocalItems(t, file1b)

	accounting.GlobalStats().ResetCounters()
	// ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, fdst, r.Flocal, false)
	// testLoggerVsLsf(ctx, fdst, operations.GetLoggerOpt(ctx).JSON, t)
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
	// ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, fdst, r.Flocal, false)
	// testLoggerVsLsf(ctx, fdst, operations.GetLoggerOpt(ctx).JSON, t)
	require.NoError(t, err)

	r.CheckRemoteItems(t, file2, file3)

	// check empty dest, new compare
	file4 := r.WriteObject(ctx, "CompareDest/two", "two", t2)
	file5 := r.WriteFile("two", "two", t2)
	r.CheckRemoteItems(t, file2, file3, file4)
	r.CheckLocalItems(t, file1c, file5)

	accounting.GlobalStats().ResetCounters()
	// ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, fdst, r.Flocal, false)
	// testLoggerVsLsf(ctx, fdst, operations.GetLoggerOpt(ctx).JSON, t)
	require.NoError(t, err)

	r.CheckRemoteItems(t, file2, file3, file4)

	// check new dest, new compare
	accounting.GlobalStats().ResetCounters()
	// ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, fdst, r.Flocal, false)
	// testLoggerVsLsf(ctx, fdst, operations.GetLoggerOpt(ctx).JSON, t)
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
		// ctx = predictDstFromLogger(ctx)
		err = Sync(ctx, fdst, r.Flocal, false)
		// testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
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
	// ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, fdst, r.Flocal, false)
	// testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
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
	// ctx = predictDstFromLogger(ctx)
	require.NoError(t, Sync(ctx, fdst, r.Flocal, false))
	// testLoggerVsLsf(ctx, fdst, operations.GetLoggerOpt(ctx).JSON, t)

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
	// ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, fdst, r.Flocal, false)
	// testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t) // not currently supported
	require.NoError(t, err)

	file1dst := file1
	file1dst.Path = "dst/one"

	r.CheckRemoteItems(t, file1dst)

	// check old dest, empty copy
	file1b := r.WriteFile("one", "onet2", t2)
	r.CheckRemoteItems(t, file1dst)
	r.CheckLocalItems(t, file1b)

	accounting.GlobalStats().ResetCounters()
	// ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, fdst, r.Flocal, false)
	// testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
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
	// ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, fdst, r.Flocal, false)
	// testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
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
	// ctx = predictDstFromLogger(ctx)
	// testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
	err = Sync(ctx, fdst, r.Flocal, false)
	require.NoError(t, err)

	file4dst := file4
	file4dst.Path = "dst/two"

	r.CheckRemoteItems(t, file2, file2dst, file3, file4, file4dst)

	// check new dest, new copy
	accounting.GlobalStats().ResetCounters()
	// ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, fdst, r.Flocal, false)
	// testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
	require.NoError(t, err)

	r.CheckRemoteItems(t, file2, file2dst, file3, file4, file4dst)

	// check empty dest, old copy
	file6 := r.WriteObject(ctx, "CopyDest/three", "three", t2)
	file7 := r.WriteFile("three", "threet3", t3)
	r.CheckRemoteItems(t, file2, file2dst, file3, file4, file4dst, file6)
	r.CheckLocalItems(t, file1c, file5, file7)

	accounting.GlobalStats().ResetCounters()
	// ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, fdst, r.Flocal, false)
	// testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
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
	// ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	// testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t) // can't test this on macOS
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

	ci.Immutable = true

	// Create file on source
	file1 := r.WriteFile("existing", "potato", t1)
	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t)

	// Should succeed
	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
	require.NoError(t, err)
	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file1)

	// Modify file data and timestamp on source
	file2 := r.WriteFile("existing", "tomatoes", t2)
	r.CheckLocalItems(t, file2)
	r.CheckRemoteItems(t, file1)

	// Should fail with ErrorImmutableModified and not modify local or remote files
	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, false)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
	assert.EqualError(t, err, fs.ErrorImmutableModified.Error())
	r.CheckLocalItems(t, file2)
	r.CheckRemoteItems(t, file1)
}

// Test --ignore-case-sync
func TestSyncIgnoreCase(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)

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
	// ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	// testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t) // can't test this on macOS
	require.NoError(t, err)
	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file2)
}

// Test --fix-case
func TestFixCase(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)

	// Only test if remote is case insensitive
	if !r.Fremote.Features().CaseInsensitive {
		t.Skip("Skipping test as local or remote are case-sensitive")
	}

	ci.FixCase = true

	// Create files with different filename casing
	file1a := r.WriteFile("existing", "potato", t1)
	file1b := r.WriteFile("existingbutdifferent", "donut", t1)
	file1c := r.WriteFile("subdira/subdirb/subdirc/hello", "donut", t1)
	file1d := r.WriteFile("subdira/subdirb/subdirc/subdird/filewithoutcasedifferences", "donut", t1)
	r.CheckLocalItems(t, file1a, file1b, file1c, file1d)
	file2a := r.WriteObject(ctx, "EXISTING", "potato", t1)
	file2b := r.WriteObject(ctx, "EXISTINGBUTDIFFERENT", "lemonade", t1)
	file2c := r.WriteObject(ctx, "SUBDIRA/subdirb/SUBDIRC/HELLO", "lemonade", t1)
	file2d := r.WriteObject(ctx, "SUBDIRA/subdirb/SUBDIRC/subdird/filewithoutcasedifferences", "lemonade", t1)
	r.CheckRemoteItems(t, file2a, file2b, file2c, file2d)

	// Should force rename of dest file that is differently-cased
	accounting.GlobalStats().ResetCounters()
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	require.NoError(t, err)
	r.CheckLocalItems(t, file1a, file1b, file1c, file1d)
	r.CheckRemoteItems(t, file1a, file1b, file1c, file1d)
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

		if runtime.GOOS == "darwin" {
			// disable server-side copies as they don't count towards transfer size stats
			r.Flocal.Features().Disable("Copy")
			if r.Fremote.Features().IsLocal {
				r.Fremote.Features().Disable("Copy")
			}
		}

		accounting.GlobalStats().ResetCounters()

		// ctx = predictDstFromLogger(ctx) // not currently supported
		err := Sync(ctx, r.Fremote, r.Flocal, false)
		// testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
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
	ctx = predictDstFromLogger(ctx)
	err := Sync(ctx, r.Fremote, r.Flocal, false)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
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

// Tests that nothing is transferred when src and dst already match
// Run the same sync twice, ensure no action is taken the second time
func testNothingToTransfer(t *testing.T, copyEmptySrcDirs bool) {
	accounting.GlobalStats().ResetCounters()
	ctx, _ := fs.AddConfig(context.Background())
	r := fstest.NewRun(t)
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)
	file2 := r.WriteFile("sub dir2/very/very/very/very/very/nested/subdir/hello world", "hello world", t1)
	r.CheckLocalItems(t, file1, file2)
	_, err := operations.SetDirModTime(ctx, r.Flocal, nil, "sub dir", t2)
	if err != nil && !errors.Is(err, fs.ErrorNotImplemented) {
		require.NoError(t, err)
	}
	r.Mkdir(ctx, r.Fremote)
	_, err = operations.MkdirModTime(ctx, r.Fremote, "sub dir", t3)
	require.NoError(t, err)

	// set logging
	// (this checks log output as DirModtime operations do not yet have stats, and r.CheckDirectoryModTimes also does not tell us what actions were taken)
	oldLogLevel := fs.GetConfig(context.Background()).LogLevel
	defer func() { fs.GetConfig(context.Background()).LogLevel = oldLogLevel }() // reset to old val after test
	// need to do this as fs.Infof only respects the globalConfig
	fs.GetConfig(context.Background()).LogLevel = fs.LogLevelInfo

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	output := bilib.CaptureOutput(func() {
		err = CopyDir(ctx, r.Fremote, r.Flocal, copyEmptySrcDirs)
		require.NoError(t, err)
	})
	require.NotNil(t, output)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
	r.CheckLocalItems(t, file1, file2)
	r.CheckRemoteItems(t, file1, file2)
	// Check that the modtimes of the directories are as expected
	r.CheckDirectoryModTimes(t, "sub dir", "sub dir2", "sub dir2/very", "sub dir2/very/very", "sub dir2/very/very/very/very/very/nested/subdir")

	// check that actions were taken
	assert.True(t, strings.Contains(string(output), "Copied"), `expected to find at least one "Copied" log: `+string(output))
	if r.Fremote.Features().DirSetModTime != nil || r.Fremote.Features().MkdirMetadata != nil {
		assert.True(t, strings.Contains(string(output), "Set directory modification time"), `expected to find at least one "Set directory modification time" log: `+string(output))
	}
	assert.False(t, strings.Contains(string(output), "There was nothing to transfer"), `expected to find no "There was nothing to transfer" logs, but found one: `+string(output))
	assert.True(t, accounting.GlobalStats().GetTransfers() >= 2)

	// run it again and make sure no actions were taken
	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	output = bilib.CaptureOutput(func() {
		err = CopyDir(ctx, r.Fremote, r.Flocal, copyEmptySrcDirs)
		require.NoError(t, err)
	})
	require.NotNil(t, output)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
	r.CheckLocalItems(t, file1, file2)
	r.CheckRemoteItems(t, file1, file2)
	// Check that the modtimes of the directories are as expected
	r.CheckDirectoryModTimes(t, "sub dir", "sub dir2", "sub dir2/very", "sub dir2/very/very", "sub dir2/very/very/very/very/very/nested/subdir")

	// check that actions were NOT taken
	assert.False(t, strings.Contains(string(output), "Copied"), `expected to find no "Copied" logs, but found one: `+string(output))
	if r.Fremote.Features().DirSetModTime != nil || r.Fremote.Features().MkdirMetadata != nil {
		assert.False(t, strings.Contains(string(output), "Set directory modification time"), `expected to find no "Set directory modification time" logs, but found one: `+string(output))
		assert.False(t, strings.Contains(string(output), "Updated directory metadata"), `expected to find no "Updated directory metadata" logs, but found one: `+string(output))
		assert.False(t, strings.Contains(string(output), "directory"), `expected to find no "directory"-related logs, but found one: `+string(output)) // catch-all
	}
	assert.True(t, strings.Contains(string(output), "There was nothing to transfer"), `expected to find a "There was nothing to transfer" log: `+string(output))
	assert.Equal(t, int64(0), accounting.GlobalStats().GetTransfers())

	// check nested empty dir behavior (FIXME: probably belongs in a separate test)
	if r.Fremote.Features().DirSetModTime == nil && r.Fremote.Features().MkdirMetadata == nil {
		return
	}
	file3 := r.WriteFile("sub dir2/sub dir3/hello world", "hello again, world", t1)
	_, err = operations.SetDirModTime(ctx, r.Flocal, nil, "sub dir2", t1)
	assert.NoError(t, err)
	_, err = operations.SetDirModTime(ctx, r.Fremote, nil, "sub dir2", t1)
	assert.NoError(t, err)
	_, err = operations.MkdirModTime(ctx, r.Flocal, "sub dirEmpty/sub dirEmpty2", t2)
	assert.NoError(t, err)
	_, err = operations.SetDirModTime(ctx, r.Flocal, nil, "sub dirEmpty", t2)
	assert.NoError(t, err)

	accounting.GlobalStats().ResetCounters()
	ctx = predictDstFromLogger(ctx)
	output = bilib.CaptureOutput(func() {
		err = CopyDir(ctx, r.Fremote, r.Flocal, copyEmptySrcDirs)
		require.NoError(t, err)
	})
	require.NotNil(t, output)
	testLoggerVsLsf(ctx, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
	r.CheckLocalItems(t, file1, file2, file3)
	r.CheckRemoteItems(t, file1, file2, file3)
	// Check that the modtimes of the directories are as expected
	r.CheckDirectoryModTimes(t, "sub dir", "sub dir2", "sub dir2/very", "sub dir2/very/very", "sub dir2/very/very/very/very/very/nested/subdir", "sub dir2/sub dir3")
	if copyEmptySrcDirs {
		r.CheckDirectoryModTimes(t, "sub dirEmpty", "sub dirEmpty/sub dirEmpty2")
		assert.True(t, strings.Contains(string(output), "sub dirEmpty:"), `expected to find at least one "sub dirEmpty:" log: `+string(output))
	} else {
		assert.False(t, strings.Contains(string(output), "sub dirEmpty:"), `expected to find no "sub dirEmpty:" logs, but found one (empty dir was synced and shouldn't have been): `+string(output))
	}
	assert.True(t, strings.Contains(string(output), "sub dir3:"), `expected to find at least one "sub dir3:" log: `+string(output))
	assert.False(t, strings.Contains(string(output), "sub dir2/very:"), `expected to find no "sub dir2/very:" logs, but found one (unmodified dir was marked modified): `+string(output))
}

func TestNothingToTransferWithEmptyDirs(t *testing.T) {
	testNothingToTransfer(t, true)
}

func TestNothingToTransferWithoutEmptyDirs(t *testing.T) {
	testNothingToTransfer(t, false)
}

// for testing logger:
func predictDstFromLogger(ctx context.Context) context.Context {
	opt := operations.NewLoggerOpt()
	var lock mutex.Mutex

	opt.LoggerFn = func(ctx context.Context, sigil operations.Sigil, src, dst fs.DirEntry, err error) {
		lock.Lock()
		defer lock.Unlock()

		// ignore dirs for our purposes here
		if err == fs.ErrorIsDir {
			return
		}
		winner := operations.WinningSide(ctx, sigil, src, dst, err)
		if winner.Obj != nil {
			file := winner.Obj
			obj, ok := file.(fs.ObjectInfo)
			checksum := ""
			timeFormat := "2006-01-02 15:04:05"
			if ok {
				if obj.Fs().Hashes().GetOne() == hash.MD5 {
					// skip if no MD5
					checksum, _ = obj.Hash(ctx, hash.MD5)
				}
				timeFormat = operations.FormatForLSFPrecision(obj.Fs().Precision())
			}
			errMsg := ""
			if winner.Err != nil {
				errMsg = ";" + winner.Err.Error()
			}
			operations.SyncFprintf(opt.JSON, "%s;%s;%v;%s%s\n", file.ModTime(ctx).Local().Format(timeFormat), checksum, file.Size(), file.Remote(), errMsg)
		}
	}
	return operations.WithSyncLogger(ctx, opt)
}

func DstLsf(ctx context.Context, Fremote fs.Fs) *bytes.Buffer {
	var opt = operations.ListJSONOpt{
		NoModTime:  false,
		NoMimeType: true,
		DirsOnly:   false,
		FilesOnly:  true,
		Recurse:    true,
		ShowHash:   true,
		HashTypes:  []string{"MD5"},
	}

	var list operations.ListFormat

	list.SetSeparator(";")
	timeFormat := operations.FormatForLSFPrecision(Fremote.Precision())
	list.AddModTime(timeFormat)
	list.AddHash(hash.MD5)
	list.AddSize()
	list.AddPath()

	out := new(bytes.Buffer)

	err := operations.ListJSON(ctx, Fremote, "", &opt, func(item *operations.ListJSONItem) error {
		_, _ = fmt.Fprintln(out, list.Format(item))
		return nil
	})
	if err != nil {
		fs.Errorf(Fremote, "ListJSON error: %v", err)
	}

	return out
}

func LoggerMatchesLsf(logger, lsf *bytes.Buffer) error {
	loggerSplit := bytes.Split(logger.Bytes(), []byte("\n"))
	sort.SliceStable(loggerSplit, func(i int, j int) bool { return string(loggerSplit[i]) < string(loggerSplit[j]) })
	lsfSplit := bytes.Split(lsf.Bytes(), []byte("\n"))
	sort.SliceStable(lsfSplit, func(i int, j int) bool { return string(lsfSplit[i]) < string(lsfSplit[j]) })

	loggerJoined := bytes.Join(loggerSplit, []byte("\n"))
	lsfJoined := bytes.Join(lsfSplit, []byte("\n"))

	if bytes.Equal(loggerJoined, lsfJoined) {
		return nil
	}
	Diff(string(loggerJoined), string(lsfJoined))
	return fmt.Errorf("logger does not match lsf! \nlogger: \n%s \nlsf: \n%s", loggerJoined, lsfJoined)
}

func Diff(rev1, rev2 string) {
	fmt.Printf("Diff of %q and %q\n", "logger", "lsf")
	cmd := exec.Command("bash", "-c", fmt.Sprintf(`diff <(echo "%s")  <(echo "%s")`, rev1, rev2))
	out, _ := cmd.Output()
	_, _ = os.Stdout.Write(out)
}

func testLoggerVsLsf(ctx context.Context, Fremote fs.Fs, logger *bytes.Buffer, t *testing.T) {
	var newlogger bytes.Buffer
	canTestModtime := fs.GetModifyWindow(ctx, Fremote) != fs.ModTimeNotSupported
	canTestHash := Fremote.Hashes().Contains(hash.MD5)
	if !canTestHash || !canTestModtime {
		loggerSplit := bytes.Split(logger.Bytes(), []byte("\n"))
		for i, line := range loggerSplit {
			elements := bytes.Split(line, []byte(";"))
			if len(elements) >= 2 {
				if !canTestModtime {
					elements[0] = []byte("")
				}
				if !canTestHash {
					elements[1] = []byte("")
				}
			}
			loggerSplit[i] = bytes.Join(elements, []byte(";"))
		}
		newlogger.Write(bytes.Join(loggerSplit, []byte("\n")))
	} else {
		newlogger.Write(logger.Bytes())
	}

	r := fstest.NewRun(t)
	if r.Flocal.Precision() == Fremote.Precision() && r.Flocal.Hashes().Contains(hash.MD5) && canTestHash {
		lsf := DstLsf(ctx, Fremote)
		err := LoggerMatchesLsf(&newlogger, lsf)
		require.NoError(t, err)
	}
}
