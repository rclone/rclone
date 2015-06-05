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
	_ "github.com/ncw/rclone/drive"
	_ "github.com/ncw/rclone/dropbox"
	_ "github.com/ncw/rclone/googlecloudstorage"
	_ "github.com/ncw/rclone/local"
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
	err := fs.Sync(fremote, flocal, false)
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
	err := fs.Sync(fremote, flocal, false)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	items := []fstest.Item{
		{Path: "sub dir/hello world", Size: 11, ModTime: t1, Md5sum: "5eb63bbbe01eeed093cb22bb8f5acdc3"},
	}

	fstest.CheckListingWithPrecision(t, flocal, items, fs.Config.ModifyWindow)
	fstest.CheckListingWithPrecision(t, fremote, items, fs.Config.ModifyWindow)
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
	err := fs.Sync(flocal, fremote, false)
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

	WriteFile("check sum", "", t1)

	transfers_before := fs.Stats.GetTransfers()
	err := fs.Sync(fremote, flocal, true)
	if err != nil {
		t.Fatalf("Initial sync failed: %v", err)
	}
	transfers_after := fs.Stats.GetTransfers()

	//We should have transferred exactly one file.
	if transfers_after-1 != transfers_before {
		t.Fatalf("Initial sync didn't do what we wanted.")
	}

	err = os.Chtimes(localName+"/check sum", t2, t2)
	if err != nil {
		t.Fatalf("Chtimes failed: %v", err)
	}

	transfers_before = fs.Stats.GetTransfers()
	err = fs.Sync(fremote, flocal, true)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	transfers_after = fs.Stats.GetTransfers()

	//We should have transferred no files
	if transfers_after != transfers_before {
		t.Fatalf("We synced, though we shouldn't have.")
	}

	remote_items := []fstest.Item{
		{Path: "check sum", Size: 0, ModTime: t1, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
	}
	local_items := []fstest.Item{
		{Path: "check sum", Size: 0, ModTime: t2, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
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
	err = fs.Sync(fremote, flocal, true)
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
	err := fs.Sync(fremote, flocal, true)
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
	err := fs.Sync(fremote, flocal, true)
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
	WriteFile("potato", "SMALLER BUT SAME DATE", t2)
	err := fs.Sync(fremote, flocal, true)
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
	err = fs.Sync(fremote, flocal, true)
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
	err := fs.Sync(fremote, flocal, true)
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
	m1 := regexp.MustCompile(`(?m)^        0 2011-12-25 12:59:59\.\d{9} empty space$`)
	if !m1.MatchString(res) {
		t.Errorf("empty space missing: %q", res)
	}
	m2 := regexp.MustCompile(`(?m)^       60 2001-02-03 04:05:06\.\d{9} potato2$`)
	if !m2.MatchString(res) {
		t.Errorf("potato2 missing: %q", res)
	}
}

func TestMd5sum(t *testing.T) {
	var buf bytes.Buffer
	err := fs.Md5sum(fremote, &buf)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	res := buf.String()
	if !strings.Contains(res, "d41d8cd98f00b204e9800998ecf8427e  empty space\n") {
		t.Errorf("empty space missing: %q", res)
	}
	if !strings.Contains(res, "6548b156ea68a4e003e786df99eee76  potato2\n") {
		t.Errorf("potato2 missing: %q", res)
	}
}

func TestCheck(t *testing.T) {
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
