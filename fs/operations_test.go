// Integration tests - test rclone by doing real transactions to a
// storage provider to and from the local disk.
//
// By default it will use a local fs, however you can provide a
// -remote option to use a different remote.  The test_all.go script
// is a wrapper to call this for all the test remotes.
//
// FIXME not safe for concurrent running of tests until fs.Config is
// no longer a global
//
// NB When writing tests
//
// Make sure every series of writes to the remote has a
// fstest.CheckItems() before use.  This make sure the directory
// listing is now consistent and stops cascading errors.
//
// Call fs.Stats.ResetCounters() before every fs.Sync() as it uses the
// error count internally.

package fs_test

import (
	"bytes"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ncw/rclone/fs"
	_ "github.com/ncw/rclone/fs/all" // import all fs
	"github.com/ncw/rclone/fstest"
)

// Globals
var (
	RemoteName      = flag.String("remote", "", "Remote to test with, defaults to local filesystem")
	SubDir          = flag.Bool("subdir", false, "Set to test with a sub directory")
	Verbose         = flag.Bool("verbose", false, "Set to enable logging")
	DumpHeaders     = flag.Bool("dump-headers", false, "Set to dump headers (needs -verbose)")
	DumpBodies      = flag.Bool("dump-bodies", false, "Set to dump bodies (needs -verbose)")
	Individual      = flag.Bool("individual", false, "Make individual bucket/container/directory for each test - much slower")
	LowLevelRetries = flag.Int("low-level-retries", 10, "Number of low level retries")
)

// Some times used in the tests
var (
	t1 = fstest.Time("2001-02-03T04:05:06.499999999Z")
	t2 = fstest.Time("2011-12-25T12:59:59.123456789Z")
	t3 = fstest.Time("2011-12-30T12:59:59.000000000Z")
)

// TestMain drives the tests
func TestMain(m *testing.M) {
	flag.Parse()
	if !*Individual {
		oneRun = newRun()
	}
	rc := m.Run()
	if !*Individual {
		oneRun.Finalise()
	}
	os.Exit(rc)
}

// Run holds the remotes for a test run
type Run struct {
	localName    string
	flocal       fs.Fs
	fremote      fs.Fs
	cleanRemote  func()
	mkdir        map[string]bool // whether the remote has been made yet for the fs name
	Logf, Fatalf func(text string, args ...interface{})
}

// oneRun holds the master run data if individual is not set
var oneRun *Run

// newRun initialise the remote and local for testing and returns a
// run object.
//
// r.flocal is an empty local Fs
// r.fremote is an empty remote Fs
//
// Finalise() will tidy them away when done.
func newRun() *Run {
	r := &Run{
		Logf:   log.Printf,
		Fatalf: log.Fatalf,
		mkdir:  make(map[string]bool),
	}

	// Never ask for passwords, fail instead.
	// If your local config is encrypted set environment variable
	// "RCLONE_CONFIG_PASS=hunter2" (or your password)
	*fs.AskPassword = false
	fs.LoadConfig()
	fs.Config.Verbose = *Verbose
	fs.Config.Quiet = !*Verbose
	fs.Config.DumpHeaders = *DumpHeaders
	fs.Config.DumpBodies = *DumpBodies
	fs.Config.LowLevelRetries = *LowLevelRetries
	var err error
	r.fremote, r.cleanRemote, err = fstest.RandomRemote(*RemoteName, *SubDir)
	if err != nil {
		r.Fatalf("Failed to open remote %q: %v", *RemoteName, err)
	}

	r.localName, err = ioutil.TempDir("", "rclone")
	if err != nil {
		r.Fatalf("Failed to create temp dir: %v", err)
	}
	r.localName = filepath.ToSlash(r.localName)
	r.flocal, err = fs.NewFs(r.localName)
	if err != nil {
		r.Fatalf("Failed to make %q: %v", r.localName, err)
	}
	fs.CalculateModifyWindow(r.fremote, r.flocal)
	return r
}

// NewRun initialise the remote and local for testing and returns a
// run object.  Call this from the tests.
//
// r.flocal is an empty local Fs
// r.fremote is an empty remote Fs
//
// Finalise() will tidy them away when done.
func NewRun(t *testing.T) *Run {
	var r *Run
	if *Individual {
		r = newRun()
	} else {
		// If not individual, use the global one with the clean method overridden
		r = new(Run)
		*r = *oneRun
		r.cleanRemote = func() {
			oldErrors := fs.Stats.GetErrors()
			fs.DeleteFiles(r.fremote.List())
			errors := fs.Stats.GetErrors() - oldErrors
			if errors != 0 {
				t.Fatalf("%d errors while cleaning remote %v", errors, r.fremote)
			}
			// Check remote is empty
			fstest.CheckItems(t, r.fremote)
		}
	}
	r.Logf = t.Logf
	r.Fatalf = t.Fatalf
	r.Logf("Remote %q, Local %q, Modify Window %q", r.fremote, r.flocal, fs.Config.ModifyWindow)
	return r
}

// Write a file to local
func (r *Run) WriteFile(filePath, content string, t time.Time) fstest.Item {
	item := fstest.NewItem(filePath, content, t)
	// FIXME make directories?
	filePath = path.Join(r.localName, filePath)
	dirPath := path.Dir(filePath)
	err := os.MkdirAll(dirPath, 0770)
	if err != nil {
		r.Fatalf("Failed to make directories %q: %v", dirPath, err)
	}
	err = ioutil.WriteFile(filePath, []byte(content), 0600)
	if err != nil {
		r.Fatalf("Failed to write file %q: %v", filePath, err)
	}
	err = os.Chtimes(filePath, t, t)
	if err != nil {
		r.Fatalf("Failed to chtimes file %q: %v", filePath, err)
	}
	return item
}

// WriteObjectTo writes an object to the fs, remote passed in
func (r *Run) WriteObjectTo(f fs.Fs, remote, content string, modTime time.Time) fstest.Item {
	const maxTries = 5
	if !r.mkdir[f.String()] {
		err := f.Mkdir()
		if err != nil {
			r.Fatalf("Failed to mkdir %q: %v", f, err)
		}
		r.mkdir[f.String()] = true
	}
	for tries := 1; ; tries++ {
		in := bytes.NewBufferString(content)
		objinfo := fs.NewStaticObjectInfo(remote, modTime, int64(len(content)), true, nil, nil)
		_, err := f.Put(in, objinfo)
		if err == nil {
			break
		}
		// Retry if err returned a retry error
		if retry, ok := err.(fs.Retry); ok && retry.Retry() && tries < maxTries {
			r.Logf("Retry Put of %q to %v: %d/%d (%v)", remote, f, tries, maxTries, err)
			continue
		}
		r.Fatalf("Failed to put %q to %q: %v", remote, f, err)
	}
	return fstest.NewItem(remote, content, modTime)
}

// WriteObject writes an object to the remote
func (r *Run) WriteObject(remote, content string, modTime time.Time) fstest.Item {
	return r.WriteObjectTo(r.fremote, remote, content, modTime)
}

// WriteBoth calls WriteObject and WriteFile with the same arguments
func (r *Run) WriteBoth(remote, content string, modTime time.Time) fstest.Item {
	r.WriteFile(remote, content, modTime)
	return r.WriteObject(remote, content, modTime)
}

// Clean the temporary directory
func (r *Run) cleanTempDir() {
	err := os.RemoveAll(r.localName)
	if err != nil {
		r.Logf("Failed to clean temporary directory %q: %v", r.localName, err)
	}
}

// finalise cleans the remote and local
func (r *Run) Finalise() {
	// r.Logf("Cleaning remote %q", r.fremote)
	r.cleanRemote()
	// r.Logf("Cleaning local %q", r.localName)
	r.cleanTempDir()
}

// ------------------------------------------------------------

func TestMkdir(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	fstest.TestMkdir(t, r.fremote)
}

// Check dry run is working
func TestCopyWithDryRun(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)

	fs.Config.DryRun = true
	err := fs.CopyDir(r.fremote, r.flocal)
	fs.Config.DryRun = false
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	fstest.CheckItems(t, r.flocal, file1)
	fstest.CheckItems(t, r.fremote)
}

// Now without dry run
func TestCopy(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("sub dir/hello world", "hello world", t1)

	err := fs.CopyDir(r.fremote, r.flocal)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	fstest.CheckItems(t, r.flocal, file1)
	fstest.CheckItems(t, r.fremote, file1)
}

// Test a server side copy if possible, or the backup path if not
func TestServerSideCopy(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject("sub dir/hello world", "hello world", t1)
	fstest.CheckItems(t, r.fremote, file1)

	fremoteCopy, finaliseCopy, err := fstest.RandomRemote(*RemoteName, *SubDir)
	if err != nil {
		t.Fatalf("Failed to open remote copy %q: %v", *RemoteName, err)
	}
	defer finaliseCopy()
	t.Logf("Server side copy (if possible) %v -> %v", r.fremote, fremoteCopy)

	err = fs.CopyDir(fremoteCopy, r.fremote)
	if err != nil {
		t.Fatalf("Server Side Copy failed: %v", err)
	}

	fstest.CheckItems(t, fremoteCopy, file1)
}

func TestLsd(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject("sub dir/hello world", "hello world", t1)

	fstest.CheckItems(t, r.fremote, file1)

	var buf bytes.Buffer
	err := fs.ListDir(r.fremote, &buf)
	if err != nil {
		t.Fatalf("ListDir failed: %v", err)
	}
	res := buf.String()
	if !strings.Contains(res, "sub dir\n") {
		t.Fatalf("Result wrong %q", res)
	}
}

// Check that if the local file doesn't exist when we copy it up,
// nothing happens to the remote file
func TestCopyAfterDelete(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject("sub dir/hello world", "hello world", t1)
	fstest.CheckItems(t, r.flocal)
	fstest.CheckItems(t, r.fremote, file1)

	err := fs.CopyDir(r.fremote, r.flocal)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

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
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

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
	if err != nil {
		t.Fatalf("Initial sync failed: %v", err)
	}

	// We should have transferred exactly one file.
	if fs.Stats.GetTransfers() != 1 {
		t.Fatalf("Sync 1: want 1 transfer, got %d", fs.Stats.GetTransfers())
	}

	fstest.CheckItems(t, r.fremote, file1)

	// Change last modified date only
	file2 := r.WriteFile("check sum", "", t2)
	fstest.CheckItems(t, r.flocal, file2)

	fs.Stats.ResetCounters()
	err = fs.Sync(r.fremote, r.flocal)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// We should have transferred no files
	if fs.Stats.GetTransfers() != 0 {
		t.Fatalf("Sync 2: want 0 transfers, got %d", fs.Stats.GetTransfers())
	}

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
	if err != nil {
		t.Fatalf("Initial sync failed: %v", err)
	}

	// We should have transferred exactly one file.
	if fs.Stats.GetTransfers() != 1 {
		t.Fatalf("Sync 1: want 1 transfer, got %d", fs.Stats.GetTransfers())
	}

	fstest.CheckItems(t, r.fremote, file1)

	// Update mtime, md5sum but not length of file
	file2 := r.WriteFile("sizeonly", "POTATO", t2)
	fstest.CheckItems(t, r.flocal, file2)

	fs.Stats.ResetCounters()
	err = fs.Sync(r.fremote, r.flocal)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// We should have transferred no files
	if fs.Stats.GetTransfers() != 0 {
		t.Fatalf("Sync 2: want 0 transfers, got %d", fs.Stats.GetTransfers())
	}

	fstest.CheckItems(t, r.flocal, file2)
	fstest.CheckItems(t, r.fremote, file1)
}

func TestSyncIgnoreTimes(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("existing", "potato", t1)

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// We should have transferred exactly 0 files because the
	// files were identical.
	if fs.Stats.GetTransfers() != 0 {
		t.Fatalf("Sync 1: want 0 transfer, got %d", fs.Stats.GetTransfers())
	}

	fs.Config.IgnoreTimes = true
	defer func() { fs.Config.IgnoreTimes = false }()

	fs.Stats.ResetCounters()
	err = fs.Sync(r.fremote, r.flocal)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// We should have transferred exactly one file even though the
	// files were identical.
	if fs.Stats.GetTransfers() != 1 {
		t.Fatalf("Sync 2: want 1 transfer, got %d", fs.Stats.GetTransfers())
	}

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
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	fstest.CheckItems(t, r.flocal, file1)
	fstest.CheckItems(t, r.fremote, file1)

	// Change everything
	r.WriteFile("existing", "newpotatoes", t2)
	fs.Stats.ResetCounters()
	err = fs.Sync(r.fremote, r.flocal)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	// Items should not change
	fstest.CheckItems(t, r.fremote, file1)
}

func TestSyncAfterChangingModtimeOnly(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("empty space", "", t2)
	r.WriteObject("empty space", "", t1)

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	fstest.CheckItems(t, r.flocal, file1)
	fstest.CheckItems(t, r.fremote, file1)
}

func TestSyncAfterAddingAFile(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("empty space", "", t2)
	file2 := r.WriteFile("potato", "------------------------------------------------------------", t3)

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
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
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
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
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
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
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

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
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
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
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
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

	fs.Config.Filter.MaxSize = 40
	defer func() {
		fs.Config.Filter.MaxSize = 0
	}()

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	fstest.CheckItems(t, r.fremote, file2, file1)

	// Now sync the other way round and check enormous doesn't get
	// deleted as it is excluded from the sync
	fs.Stats.ResetCounters()
	err = fs.Sync(r.flocal, r.fremote)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
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
		fs.Config.Filter.MaxSize = 0
		fs.Config.Filter.DeleteExcluded = false
	}()

	fs.Stats.ResetCounters()
	err := fs.Sync(r.fremote, r.flocal)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	fstest.CheckItems(t, r.fremote, file2)

	// Check sync the other way round to make sure enormous gets
	// deleted even though it is excluded
	fs.Stats.ResetCounters()
	err = fs.Sync(r.flocal, r.fremote)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
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
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	fstest.CheckItems(t, r.fremote, oneO, twoF, threeO, fourF, fiveF)
}

// Test a server side move if possible, or the backup path if not
func TestServerSideMove(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth("empty space", "", t2)

	fstest.CheckItems(t, r.fremote, file2, file1)

	fremoteMove, finaliseMove, err := fstest.RandomRemote(*RemoteName, *SubDir)
	if err != nil {
		t.Fatalf("Failed to open remote move %q: %v", *RemoteName, err)
	}
	defer finaliseMove()
	t.Logf("Server side move (if possible) %v -> %v", r.fremote, fremoteMove)

	// Write just one file in the new remote
	r.WriteObjectTo(fremoteMove, "empty space", "", t2)
	fstest.CheckItems(t, fremoteMove, file2)

	// Do server side move
	fs.Stats.ResetCounters()
	err = fs.MoveDir(fremoteMove, r.fremote)
	if err != nil {
		t.Fatalf("Server Side Move failed: %v", err)
	}

	fstest.CheckItems(t, r.fremote)
	fstest.CheckItems(t, fremoteMove, file2, file1)

	// Move it back again, dst does not exist this time
	fs.Stats.ResetCounters()
	err = fs.MoveDir(r.fremote, fremoteMove)
	if err != nil {
		t.Fatalf("Server Side Move 2 failed: %v", err)
	}

	fstest.CheckItems(t, r.fremote, file2, file1)
	fstest.CheckItems(t, fremoteMove)
}

func TestLs(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth("empty space", "", t2)

	fstest.CheckItems(t, r.fremote, file1, file2)

	var buf bytes.Buffer
	err := fs.List(r.fremote, &buf)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	res := buf.String()
	if !strings.Contains(res, "        0 empty space\n") {
		t.Errorf("empty space missing: %q", res)
	}
	if !strings.Contains(res, "       60 potato2\n") {
		t.Errorf("potato2 missing: %q", res)
	}
}

func TestLsLong(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth("empty space", "", t2)

	fstest.CheckItems(t, r.fremote, file1, file2)

	var buf bytes.Buffer
	err := fs.ListLong(r.fremote, &buf)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	res := buf.String()
	lines := strings.Split(strings.Trim(res, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("Wrong number of lines in list: %q", lines)
	}

	timeFormat := "2006-01-02 15:04:05.000000000"
	precision := r.fremote.Precision()
	location := time.Now().Location()
	checkTime := func(m, filename string, expected time.Time) {
		modTime, err := time.ParseInLocation(timeFormat, m, location) // parse as localtime
		if err != nil {
			t.Errorf("Error parsing %q: %v", m, err)
		} else {
			dt, ok := fstest.CheckTimeEqualWithPrecision(expected, modTime, precision)
			if !ok {
				t.Errorf("%s: Modification time difference too big |%s| > %s (%s vs %s) (precision %s)", filename, dt, precision, modTime, expected, precision)
			}
		}
	}

	m1 := regexp.MustCompile(`(?m)^        0 (\d{4}-\d\d-\d\d \d\d:\d\d:\d\d\.\d{9}) empty space$`)
	if ms := m1.FindStringSubmatch(res); ms == nil {
		t.Errorf("empty space missing: %q", res)
	} else {
		checkTime(ms[1], "empty space", t2.Local())
	}

	m2 := regexp.MustCompile(`(?m)^       60 (\d{4}-\d\d-\d\d \d\d:\d\d:\d\d\.\d{9}) potato2$`)
	if ms := m2.FindStringSubmatch(res); ms == nil {
		t.Errorf("potato2 missing: %q", res)
	} else {
		checkTime(ms[1], "potato2", t1.Local())
	}
}

func TestMd5sum(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth("empty space", "", t2)

	fstest.CheckItems(t, r.fremote, file1, file2)

	var buf bytes.Buffer
	err := fs.Md5sum(r.fremote, &buf)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	res := buf.String()
	if !strings.Contains(res, "d41d8cd98f00b204e9800998ecf8427e  empty space\n") &&
		!strings.Contains(res, "                     UNSUPPORTED  empty space\n") &&
		!strings.Contains(res, "                                  empty space\n") {
		t.Errorf("empty space missing: %q", res)
	}
	if !strings.Contains(res, "6548b156ea68a4e003e786df99eee76  potato2\n") &&
		!strings.Contains(res, "                     UNSUPPORTED  potato2\n") &&
		!strings.Contains(res, "                                  potato2\n") {
		t.Errorf("potato2 missing: %q", res)
	}
}

func TestSha1sum(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth("empty space", "", t2)

	fstest.CheckItems(t, r.fremote, file1, file2)

	var buf bytes.Buffer
	err := fs.Sha1sum(r.fremote, &buf)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	res := buf.String()
	if !strings.Contains(res, "da39a3ee5e6b4b0d3255bfef95601890afd80709  empty space\n") &&
		!strings.Contains(res, "                             UNSUPPORTED  empty space\n") &&
		!strings.Contains(res, "                                          empty space\n") {
		t.Errorf("empty space missing: %q", res)
	}
	if !strings.Contains(res, "9dc7f7d3279715991a22853f5981df582b7f9f6d  potato2\n") &&
		!strings.Contains(res, "                             UNSUPPORTED  potato2\n") &&
		!strings.Contains(res, "                                          potato2\n") {
		t.Errorf("potato2 missing: %q", res)
	}
}

func TestCount(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth("empty space", "", t2)

	fstest.CheckItems(t, r.fremote, file1, file2)

	objects, size, err := fs.Count(r.fremote)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if objects != 2 {
		t.Errorf("want 2 objects got %d", objects)
	}
	if size != 60 {
		t.Errorf("want size 60 got %d", size)
	}
}

func TestDelete(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject("small", "1234567890", t2)                                                                                           // 10 bytes
	file2 := r.WriteObject("medium", "------------------------------------------------------------", t1)                                        // 60 bytes
	file3 := r.WriteObject("large", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", t1) // 100 bytes
	fstest.CheckItems(t, r.fremote, file1, file2, file3)

	fs.Config.Filter.MaxSize = 60
	defer func() {
		fs.Config.Filter.MaxSize = 0
	}()

	err := fs.Delete(r.fremote)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	fstest.CheckItems(t, r.fremote, file3)
}

func TestCheck(t *testing.T) {
	r := NewRun(t)
	defer r.Finalise()

	check := func(i int, wantErrors int64) {
		fs.Debug(r.fremote, "%d: Starting check test", i)
		oldErrors := fs.Stats.GetErrors()
		err := fs.Check(r.flocal, r.fremote)
		gotErrors := fs.Stats.GetErrors() - oldErrors
		if wantErrors == 0 && err != nil {
			t.Errorf("%d: Got error when not expecting one: %v", i, err)
		}
		if wantErrors != 0 && err == nil {
			t.Errorf("%d: No error when expecting one", i)
		}
		if wantErrors != gotErrors {
			t.Errorf("%d: Expecting %d errors but got %d", i, wantErrors, gotErrors)
		}
		fs.Debug(r.fremote, "%d: Ending check test", i)
	}

	file1 := r.WriteBoth("rutabaga", "is tasty", t3)
	fstest.CheckItems(t, r.fremote, file1)
	fstest.CheckItems(t, r.flocal, file1)
	check(1, 0)

	file2 := r.WriteFile("potato2", "------------------------------------------------------------", t1)
	fstest.CheckItems(t, r.flocal, file1, file2)
	check(2, 1)

	file3 := r.WriteObject("empty space", "", t2)
	fstest.CheckItems(t, r.fremote, file1, file3)
	check(3, 2)

	r.WriteObject("potato2", "------------------------------------------------------------", t1)
	fstest.CheckItems(t, r.fremote, file1, file2, file3)
	check(4, 1)

	r.WriteFile("empty space", "", t2)
	fstest.CheckItems(t, r.flocal, file1, file2, file3)
	check(5, 0)
}

func (r *Run) checkWithDuplicates(t *testing.T, items ...fstest.Item) {
	objects, size, err := fs.Count(r.fremote)
	if err != nil {
		t.Fatalf("Error listing: %v", err)
	}
	if objects != int64(len(items)) {
		t.Fatalf("Error listing want %d objects, got %d", len(items), objects)
	}
	wantSize := int64(0)
	for _, item := range items {
		wantSize += item.Size
	}
	if wantSize != size {
		t.Fatalf("Error listing want %d size, got %d", wantSize, size)
	}
}

func TestDeduplicateInteractive(t *testing.T) {
	if *RemoteName != "TestDrive:" {
		t.Skip("Can only test deduplicate on google drive")
	}
	r := NewRun(t)
	defer r.Finalise()

	file1 := r.WriteObject("one", "This is one", t1)
	file2 := r.WriteObject("one", "This is one", t1)
	file3 := r.WriteObject("one", "This is one", t1)
	r.checkWithDuplicates(t, file1, file2, file3)

	err := fs.Deduplicate(r.fremote, fs.DeduplicateInteractive)
	if err != nil {
		t.Fatalf("fs.Deduplicate returned error: %v", err)
	}

	fstest.CheckItems(t, r.fremote, file1)
}

func TestDeduplicateSkip(t *testing.T) {
	if *RemoteName != "TestDrive:" {
		t.Skip("Can only test deduplicate on google drive")
	}
	r := NewRun(t)
	defer r.Finalise()

	file1 := r.WriteObject("one", "This is one", t1)
	file2 := r.WriteObject("one", "This is one", t1)
	file3 := r.WriteObject("one", "This is another one", t1)
	r.checkWithDuplicates(t, file1, file2, file3)

	err := fs.Deduplicate(r.fremote, fs.DeduplicateSkip)
	if err != nil {
		t.Fatalf("fs.Deduplicate returned error: %v", err)
	}

	r.checkWithDuplicates(t, file1, file3)
}

func TestDeduplicateFirst(t *testing.T) {
	if *RemoteName != "TestDrive:" {
		t.Skip("Can only test deduplicate on google drive")
	}
	r := NewRun(t)
	defer r.Finalise()

	file1 := r.WriteObject("one", "This is one", t1)
	file2 := r.WriteObject("one", "This is one A", t1)
	file3 := r.WriteObject("one", "This is one BB", t1)
	r.checkWithDuplicates(t, file1, file2, file3)

	err := fs.Deduplicate(r.fremote, fs.DeduplicateFirst)
	if err != nil {
		t.Fatalf("fs.Deduplicate returned error: %v", err)
	}

	objects, size, err := fs.Count(r.fremote)
	if err != nil {
		t.Fatalf("Error listing: %v", err)
	}
	if objects != 1 {
		t.Errorf("Expecting 1 object got %v", objects)
	}
	if size != file1.Size && size != file2.Size && size != file3.Size {
		t.Errorf("Size not one of the object sizes %d", size)
	}
}

func TestDeduplicateNewest(t *testing.T) {
	if *RemoteName != "TestDrive:" {
		t.Skip("Can only test deduplicate on google drive")
	}
	r := NewRun(t)
	defer r.Finalise()

	file1 := r.WriteObject("one", "This is one", t1)
	file2 := r.WriteObject("one", "This is one too", t2)
	file3 := r.WriteObject("one", "This is another one", t3)
	r.checkWithDuplicates(t, file1, file2, file3)

	err := fs.Deduplicate(r.fremote, fs.DeduplicateNewest)
	if err != nil {
		t.Fatalf("fs.Deduplicate returned error: %v", err)
	}

	fstest.CheckItems(t, r.fremote, file3)
}

func TestDeduplicateOldest(t *testing.T) {
	if *RemoteName != "TestDrive:" {
		t.Skip("Can only test deduplicate on google drive")
	}
	r := NewRun(t)
	defer r.Finalise()

	file1 := r.WriteObject("one", "This is one", t1)
	file2 := r.WriteObject("one", "This is one too", t2)
	file3 := r.WriteObject("one", "This is another one", t3)
	r.checkWithDuplicates(t, file1, file2, file3)

	err := fs.Deduplicate(r.fremote, fs.DeduplicateOldest)
	if err != nil {
		t.Fatalf("fs.Deduplicate returned error: %v", err)
	}

	fstest.CheckItems(t, r.fremote, file1)
}

func TestDeduplicateRename(t *testing.T) {
	if *RemoteName != "TestDrive:" {
		t.Skip("Can only test deduplicate on google drive")
	}
	r := NewRun(t)
	defer r.Finalise()

	file1 := r.WriteObject("one.txt", "This is one", t1)
	file2 := r.WriteObject("one.txt", "This is one too", t2)
	file3 := r.WriteObject("one.txt", "This is another one", t3)
	r.checkWithDuplicates(t, file1, file2, file3)

	err := fs.Deduplicate(r.fremote, fs.DeduplicateRename)
	if err != nil {
		t.Fatalf("fs.Deduplicate returned error: %v", err)
	}

	for o := range r.fremote.List() {
		remote := o.Remote()
		if remote != "one-1.txt" &&
			remote != "one-2.txt" &&
			remote != "one-3.txt" {
			t.Errorf("Bad file name after rename %q", remote)
		}
		size := o.Size()
		if size != file1.Size && size != file2.Size && size != file3.Size {
			t.Errorf("Size not one of the object sizes %d", size)
		}
	}
}
