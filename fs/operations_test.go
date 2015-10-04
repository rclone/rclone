// Test rclone by doing real transactions to a storage provider to and
// from the local disk

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
	"github.com/ncw/rclone/fstest"

	// Active file systems
	_ "github.com/ncw/rclone/amazonclouddrive"
	_ "github.com/ncw/rclone/drive"
	_ "github.com/ncw/rclone/dropbox"
	_ "github.com/ncw/rclone/googlecloudstorage"
	_ "github.com/ncw/rclone/local"
	_ "github.com/ncw/rclone/onedrive"
	_ "github.com/ncw/rclone/s3"
	_ "github.com/ncw/rclone/swift"
)

// Globals
var (
	localName, remoteName string
	flocal, fremote       fs.Fs
	RemoteName            = flag.String("remote", "", "Remote to test with, defaults to local filesystem")
	SubDir                = flag.Bool("subdir", false, "Set to test with a sub directory")
	Verbose               = flag.Bool("verbose", false, "Set to enable logging")
	finalise              func()
)

// Write a file
func WriteFile(filePath, content string, t time.Time) {
	// FIXME make directories?
	filePath = path.Join(localName, filePath)
	dirPath := path.Dir(filePath)
	err := os.MkdirAll(dirPath, 0770)
	if err != nil {
		log.Fatalf("Failed to make directories %q: %v", dirPath, err)
	}
	err = ioutil.WriteFile(filePath, []byte(content), 0600)
	if err != nil {
		log.Fatalf("Failed to write file %q: %v", filePath, err)
	}
	err = os.Chtimes(filePath, t, t)
	if err != nil {
		log.Fatalf("Failed to chtimes file %q: %v", filePath, err)
	}
}

var t1 = fstest.Time("2001-02-03T04:05:06.499999999Z")
var t2 = fstest.Time("2011-12-25T12:59:59.123456789Z")
var t3 = fstest.Time("2011-12-30T12:59:59.000000000Z")

func TestInit(t *testing.T) {
	fs.LoadConfig()
	fs.Config.Verbose = *Verbose
	fs.Config.Quiet = !*Verbose
	var err error
	fremote, finalise, err = fstest.RandomRemote(*RemoteName, *SubDir)
	if err != nil {
		t.Fatalf("Failed to open remote %q: %v", *RemoteName, err)
	}
	t.Logf("Testing with remote %v", fremote)

	localName, err = ioutil.TempDir("", "rclone")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	localName = filepath.ToSlash(localName)
	t.Logf("Testing with local %q", localName)
	flocal, err = fs.NewFs(localName)
	if err != nil {
		t.Fatalf("Failed to make %q: %v", remoteName, err)
	}

}
func TestCalculateModifyWindow(t *testing.T) {
	fs.CalculateModifyWindow(fremote, flocal)
	t.Logf("ModifyWindow is %q", fs.Config.ModifyWindow)
}

func TestMkdir(t *testing.T) {
	fstest.TestMkdir(t, fremote)
}

// Check dry run is working
func TestCopyWithDryRun(t *testing.T) {
	WriteFile("sub dir/hello world", "hello world", t1)

	fs.Config.DryRun = true
	err := fs.CopyDir(fremote, flocal)
	fs.Config.DryRun = false
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	items := []fstest.Item{
		{Path: "sub dir/hello world", Size: 11, ModTime: t1, Md5sum: "5eb63bbbe01eeed093cb22bb8f5acdc3"},
	}

	fstest.CheckListingWithPrecision(t, flocal, items, fs.Config.ModifyWindow)
	fstest.CheckListingWithPrecision(t, fremote, []fstest.Item{}, fs.Config.ModifyWindow)
}

// Now without dry run
func TestCopy(t *testing.T) {
	err := fs.CopyDir(fremote, flocal)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	items := []fstest.Item{
		{Path: "sub dir/hello world", Size: 11, ModTime: t1, Md5sum: "5eb63bbbe01eeed093cb22bb8f5acdc3"},
	}

	fstest.CheckListingWithPrecision(t, flocal, items, fs.Config.ModifyWindow)
	fstest.CheckListingWithPrecision(t, fremote, items, fs.Config.ModifyWindow)
}

// Test a server side copy if possible, or the backup path if not
func TestServerSideCopy(t *testing.T) {
	fremoteCopy, finaliseCopy, err := fstest.RandomRemote(*RemoteName, *SubDir)
	if err != nil {
		t.Fatalf("Failed to open remote copy %q: %v", *RemoteName, err)
	}
	defer finaliseCopy()
	t.Logf("Server side copy (if possible) %v -> %v", fremote, fremoteCopy)

	err = fs.CopyDir(fremoteCopy, fremote)
	if err != nil {
		t.Fatalf("Server Side Copy failed: %v", err)
	}

	items := []fstest.Item{
		{Path: "sub dir/hello world", Size: 11, ModTime: t1, Md5sum: "5eb63bbbe01eeed093cb22bb8f5acdc3"},
	}

	fstest.CheckListingWithPrecision(t, fremote, items, fs.Config.ModifyWindow)
	fstest.CheckListingWithPrecision(t, fremoteCopy, items, fs.Config.ModifyWindow)
}

func TestLsd(t *testing.T) {
	var buf bytes.Buffer
	err := fs.ListDir(fremote, &buf)
	if err != nil {
		t.Fatalf("ListDir failed: %v", err)
	}
	res := buf.String()
	if !strings.Contains(res, "sub dir\n") {
		t.Fatalf("Result wrong %q", res)
	}
}

// Now delete the local file and download it
func TestCopyAfterDelete(t *testing.T) {
	err := os.Remove(localName + "/sub dir/hello world")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	items := []fstest.Item{
		{Path: "sub dir/hello world", Size: 11, ModTime: t1, Md5sum: "5eb63bbbe01eeed093cb22bb8f5acdc3"},
	}
	fstest.CheckListingWithPrecision(t, flocal, []fstest.Item{}, fs.Config.ModifyWindow)
	fstest.CheckListingWithPrecision(t, fremote, items, fs.Config.ModifyWindow)
}

func TestCopyRedownload(t *testing.T) {
	err := fs.CopyDir(flocal, fremote)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	items := []fstest.Item{
		{Path: "sub dir/hello world", Size: 11, ModTime: t1, Md5sum: "5eb63bbbe01eeed093cb22bb8f5acdc3"},
	}
	fstest.CheckListingWithPrecision(t, flocal, items, fs.Config.ModifyWindow)
	fstest.CheckListingWithPrecision(t, fremote, items, fs.Config.ModifyWindow)

	// Clean the directory
	cleanTempDir(t)
}

// Create a file and sync it. Change the last modified date and resync.
// If we're only doing sync by size and checksum, we expect nothing to
// to be transferred on the second sync.
func TestSyncBasedOnCheckSum(t *testing.T) {
	cleanTempDir(t)
	fs.Config.CheckSum = true
	defer func() { fs.Config.CheckSum = false }()

	WriteFile("check sum", "", t1)
	local_items := []fstest.Item{
		{Path: "check sum", Size: 0, ModTime: t1, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
	}
	fstest.CheckListingWithPrecision(t, flocal, local_items, fs.Config.ModifyWindow)

	fs.Stats.ResetCounters()
	err := fs.Sync(fremote, flocal)
	if err != nil {
		t.Fatalf("Initial sync failed: %v", err)
	}

	// We should have transferred exactly one file.
	if fs.Stats.GetTransfers() != 1 {
		t.Fatalf("Sync 1: want 1 transfer, got %d", fs.Stats.GetTransfers())
	}

	remote_items := local_items
	fstest.CheckListingWithPrecision(t, fremote, remote_items, fs.Config.ModifyWindow)

	err = os.Chtimes(localName+"/check sum", t2, t2)
	if err != nil {
		t.Fatalf("Chtimes failed: %v", err)
	}
	local_items = []fstest.Item{
		{Path: "check sum", Size: 0, ModTime: t2, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
	}
	fstest.CheckListingWithPrecision(t, flocal, local_items, fs.Config.ModifyWindow)

	fs.Stats.ResetCounters()
	err = fs.Sync(fremote, flocal)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// We should have transferred no files
	if fs.Stats.GetTransfers() != 0 {
		t.Fatalf("Sync 2: want 0 transfers, got %d", fs.Stats.GetTransfers())
	}

	fstest.CheckListingWithPrecision(t, flocal, local_items, fs.Config.ModifyWindow)
	fstest.CheckListingWithPrecision(t, fremote, remote_items, fs.Config.ModifyWindow)

	cleanTempDir(t)
}

// Create a file and sync it. Change the last modified date and the
// file contents but not the size.  If we're only doing sync by size
// only, we expect nothing to to be transferred on the second sync.
func TestSyncSizeOnly(t *testing.T) {
	cleanTempDir(t)
	fs.Config.SizeOnly = true
	defer func() { fs.Config.SizeOnly = false }()

	WriteFile("sizeonly", "potato", t1)
	local_items := []fstest.Item{
		{Path: "sizeonly", Size: 6, ModTime: t1, Md5sum: "8ee2027983915ec78acc45027d874316"},
	}
	fstest.CheckListingWithPrecision(t, flocal, local_items, fs.Config.ModifyWindow)

	fs.Stats.ResetCounters()
	err := fs.Sync(fremote, flocal)
	if err != nil {
		t.Fatalf("Initial sync failed: %v", err)
	}

	// We should have transferred exactly one file.
	if fs.Stats.GetTransfers() != 1 {
		t.Fatalf("Sync 1: want 1 transfer, got %d", fs.Stats.GetTransfers())
	}

	remote_items := local_items
	fstest.CheckListingWithPrecision(t, fremote, remote_items, fs.Config.ModifyWindow)

	// Update mtime, md5sum but not length of file
	WriteFile("sizeonly", "POTATO", t2)
	local_items = []fstest.Item{
		{Path: "sizeonly", Size: 6, ModTime: t2, Md5sum: "8ac6f27a282e4938125482607ccfb55f"},
	}
	fstest.CheckListingWithPrecision(t, flocal, local_items, fs.Config.ModifyWindow)

	fs.Stats.ResetCounters()
	err = fs.Sync(fremote, flocal)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// We should have transferred no files
	if fs.Stats.GetTransfers() != 0 {
		t.Fatalf("Sync 2: want 0 transfers, got %d", fs.Stats.GetTransfers())
	}

	fstest.CheckListingWithPrecision(t, flocal, local_items, fs.Config.ModifyWindow)
	fstest.CheckListingWithPrecision(t, fremote, remote_items, fs.Config.ModifyWindow)

	cleanTempDir(t)
}

func TestSyncAfterChangingModtimeOnly(t *testing.T) {
	WriteFile("empty space", "", t1)

	err := os.Chtimes(localName+"/empty space", t2, t2)
	if err != nil {
		t.Fatalf("Chtimes failed: %v", err)
	}
	err = fs.Sync(fremote, flocal)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	items := []fstest.Item{
		{Path: "empty space", Size: 0, ModTime: t2, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
	}
	fstest.CheckListingWithPrecision(t, flocal, items, fs.Config.ModifyWindow)
	fstest.CheckListingWithPrecision(t, fremote, items, fs.Config.ModifyWindow)
}

func TestSyncAfterAddingAFile(t *testing.T) {
	WriteFile("potato", "------------------------------------------------------------", t3)
	err := fs.Sync(fremote, flocal)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	items := []fstest.Item{
		{Path: "empty space", Size: 0, ModTime: t2, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
		{Path: "potato", Size: 60, ModTime: t3, Md5sum: "d6548b156ea68a4e003e786df99eee76"},
	}
	fstest.CheckListingWithPrecision(t, flocal, items, fs.Config.ModifyWindow)
	fstest.CheckListingWithPrecision(t, fremote, items, fs.Config.ModifyWindow)
}

func TestSyncAfterChangingFilesSizeOnly(t *testing.T) {
	WriteFile("potato", "smaller but same date", t3)
	err := fs.Sync(fremote, flocal)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	items := []fstest.Item{
		{Path: "empty space", Size: 0, ModTime: t2, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
		{Path: "potato", Size: 21, ModTime: t3, Md5sum: "100defcf18c42a1e0dc42a789b107cd2"},
	}
	fstest.CheckListingWithPrecision(t, flocal, items, fs.Config.ModifyWindow)
	fstest.CheckListingWithPrecision(t, fremote, items, fs.Config.ModifyWindow)
}

// Sync after changing a file's contents, modtime but not length
func TestSyncAfterChangingContentsOnly(t *testing.T) {
	if fremote.Precision() == fs.ModTimeNotSupported {
		t.Logf("ModTimeNotSupported so forcing file to be a different size")
		WriteFile("potato", "different size to make sure it syncs", t2)
		err := fs.Sync(fremote, flocal)
		if err != nil {
			t.Fatalf("Sync failed: %v", err)
		}
	}
	WriteFile("potato", "SMALLER BUT SAME DATE", t2)
	err := fs.Sync(fremote, flocal)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	items := []fstest.Item{
		{Path: "empty space", Size: 0, ModTime: t2, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
		{Path: "potato", Size: 21, ModTime: t2, Md5sum: "e4cb6955d9106df6263c45fcfc10f163"},
	}
	fstest.CheckListingWithPrecision(t, flocal, items, fs.Config.ModifyWindow)
	fstest.CheckListingWithPrecision(t, fremote, items, fs.Config.ModifyWindow)
}

// Sync after removing a file and adding a file --dry-run
func TestSyncAfterRemovingAFileAndAddingAFileDryRun(t *testing.T) {
	WriteFile("potato2", "------------------------------------------------------------", t1)
	err := os.Remove(localName + "/potato")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	fs.Config.DryRun = true
	err = fs.Sync(fremote, flocal)
	fs.Config.DryRun = false
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	before := []fstest.Item{
		{Path: "empty space", Size: 0, ModTime: t2, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
		{Path: "potato", Size: 21, ModTime: t2, Md5sum: "e4cb6955d9106df6263c45fcfc10f163"},
	}
	items := []fstest.Item{
		{Path: "empty space", Size: 0, ModTime: t2, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
		{Path: "potato2", Size: 60, ModTime: t1, Md5sum: "d6548b156ea68a4e003e786df99eee76"},
	}
	fstest.CheckListingWithPrecision(t, flocal, items, fs.Config.ModifyWindow)
	fstest.CheckListingWithPrecision(t, fremote, before, fs.Config.ModifyWindow)
}

// Sync after removing a file and adding a file
func TestSyncAfterRemovingAFileAndAddingAFile(t *testing.T) {
	err := fs.Sync(fremote, flocal)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	items := []fstest.Item{
		{Path: "empty space", Size: 0, ModTime: t2, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
		{Path: "potato2", Size: 60, ModTime: t1, Md5sum: "d6548b156ea68a4e003e786df99eee76"},
	}
	fstest.CheckListingWithPrecision(t, flocal, items, fs.Config.ModifyWindow)
	fstest.CheckListingWithPrecision(t, fremote, items, fs.Config.ModifyWindow)
}

// Test with exclude
func TestSyncWithExclude(t *testing.T) {
	WriteFile("enormous", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", t1) // 100 bytes
	fs.Config.Filter.MaxSize = 80
	defer func() {
		fs.Config.Filter.MaxSize = 0
	}()
	err := fs.Sync(fremote, flocal)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	items := []fstest.Item{
		{Path: "empty space", Size: 0, ModTime: t2, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
		{Path: "potato2", Size: 60, ModTime: t1, Md5sum: "d6548b156ea68a4e003e786df99eee76"},
	}
	fstest.CheckListingWithPrecision(t, fremote, items, fs.Config.ModifyWindow)
}

// Test with exclude and delete excluded
func TestSyncWithExcludeAndDeleleteExcluded(t *testing.T) {
	fs.Config.Filter.MaxSize = 40
	fs.Config.Filter.DeleteExcluded = true
	reset := func() {
		fs.Config.Filter.MaxSize = 0
		fs.Config.Filter.DeleteExcluded = false
	}
	defer reset()
	err := fs.Sync(fremote, flocal)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	items := []fstest.Item{
		{Path: "empty space", Size: 0, ModTime: t2, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
	}
	fstest.CheckListingWithPrecision(t, fremote, items, fs.Config.ModifyWindow)

	// Tidy up
	reset()
	err = os.Remove(localName + "/enormous")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	err = fs.Sync(fremote, flocal)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	items = []fstest.Item{
		{Path: "empty space", Size: 0, ModTime: t2, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
		{Path: "potato2", Size: 60, ModTime: t1, Md5sum: "d6548b156ea68a4e003e786df99eee76"},
	}
	fstest.CheckListingWithPrecision(t, fremote, items, fs.Config.ModifyWindow)
}

// Test a server side move if possible, or the backup path if not
func TestServerSideMove(t *testing.T) {
	fremoteMove, finaliseMove, err := fstest.RandomRemote(*RemoteName, *SubDir)
	if err != nil {
		t.Fatalf("Failed to open remote move %q: %v", *RemoteName, err)
	}
	defer finaliseMove()
	t.Logf("Server side move (if possible) %v -> %v", fremote, fremoteMove)

	// Start with a copy
	err = fs.CopyDir(fremoteMove, fremote)
	if err != nil {
		t.Fatalf("Server Side Copy failed: %v", err)
	}

	// Remove one file
	obj := fremoteMove.NewFsObject("potato2")
	if obj == nil {
		t.Fatalf("Failed to find potato2")
	}
	err = obj.Remove()
	if err != nil {
		t.Fatalf("Failed to remove object: %v", err)
	}

	// Do server side move
	err = fs.MoveDir(fremoteMove, fremote)
	if err != nil {
		t.Fatalf("Server Side Move failed: %v", err)
	}

	items := []fstest.Item{
		{Path: "empty space", Size: 0, ModTime: t2, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
		{Path: "potato2", Size: 60, ModTime: t1, Md5sum: "d6548b156ea68a4e003e786df99eee76"},
	}

	fstest.CheckListingWithPrecision(t, fremote, items[:0], fs.Config.ModifyWindow)
	fstest.CheckListingWithPrecision(t, fremoteMove, items, fs.Config.ModifyWindow)

	// Move it back again, dst does not exist this time
	err = fs.MoveDir(fremote, fremoteMove)
	if err != nil {
		t.Fatalf("Server Side Move 2 failed: %v", err)
	}

	fstest.CheckListingWithPrecision(t, fremote, items, fs.Config.ModifyWindow)
	fstest.CheckListingWithPrecision(t, fremoteMove, items[:0], fs.Config.ModifyWindow)
}

func TestLs(t *testing.T) {
	var buf bytes.Buffer
	err := fs.List(fremote, &buf)
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
	var buf bytes.Buffer
	err := fs.ListLong(fremote, &buf)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	res := buf.String()
	lines := strings.Split(strings.Trim(res, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("Wrong number of lines in list: %q", lines)
	}

	timeFormat := "2006-01-02 15:04:05.000000000"
	precision := fremote.Precision()
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
	var buf bytes.Buffer
	err := fs.Md5sum(fremote, &buf)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	res := buf.String()
	if !strings.Contains(res, "d41d8cd98f00b204e9800998ecf8427e  empty space\n") &&
		!strings.Contains(res, "                                  empty space\n") {
		t.Errorf("empty space missing: %q", res)
	}
	if !strings.Contains(res, "6548b156ea68a4e003e786df99eee76  potato2\n") &&
		!strings.Contains(res, "                                  potato2\n") {
		t.Errorf("potato2 missing: %q", res)
	}
}

func TestCount(t *testing.T) {
	objects, size, err := fs.Count(fremote)
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

func TestCheck(t *testing.T) {
	// FIXME
}

// Clean the temporary directory
func cleanTempDir(t *testing.T) {
	t.Logf("Cleaning temporary directory: %q", localName)
	err := os.RemoveAll(localName)
	if err != nil {
		t.Logf("Failed to remove %q: %v", localName, err)
	}
}

func TestFinalise(t *testing.T) {
	finalise()

	cleanTempDir(t)
}
