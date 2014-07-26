// Test rclone by doing real transactions to a storage provider to and
// from the local disk

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest"
	"github.com/ogier/pflag"

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
	version               = pflag.BoolP("version", "V", false, "Print the version number")
	subDir                = pflag.BoolP("subdir", "S", false, "Test with a sub directory")
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

func TestCopy(flocal, fremote fs.Fs) {
	WriteFile("sub dir/hello world", "hello world", t1)

	// Check dry run is working
	log.Printf("Copy with --dry-run")
	fs.Config.DryRun = true
	err := fs.Sync(fremote, flocal, false)
	fs.Config.DryRun = false
	if err != nil {
		log.Fatalf("Copy failed: %v", err)
	}

	items := []fstest.Item{
		{Path: "sub dir/hello world", Size: 11, ModTime: t1, Md5sum: "5eb63bbbe01eeed093cb22bb8f5acdc3"},
	}

	fstest.CheckListing(flocal, items)
	fstest.CheckListing(fremote, []fstest.Item{})

	// Now without dry run

	log.Printf("Copy")
	err = fs.Sync(fremote, flocal, false)
	if err != nil {
		log.Fatalf("Copy failed: %v", err)
	}

	fstest.CheckListing(flocal, items)
	fstest.CheckListing(fremote, items)

	// Now delete the local file and download it

	err = os.Remove(localName + "/sub dir/hello world")
	if err != nil {
		log.Fatalf("Remove failed: %v", err)
	}

	fstest.CheckListing(flocal, []fstest.Item{})
	fstest.CheckListing(fremote, items)

	log.Printf("Copy - redownload")
	err = fs.Sync(flocal, fremote, false)
	if err != nil {
		log.Fatalf("Copy failed: %v", err)
	}

	fstest.CheckListing(flocal, items)
	fstest.CheckListing(fremote, items)

	// Clean the directory
	cleanTempDir()
}

func TestSync(flocal, fremote fs.Fs) {
	WriteFile("empty space", "", t1)

	log.Printf("Sync after changing file modtime only")
	err := os.Chtimes(localName+"/empty space", t2, t2)
	if err != nil {
		log.Fatalf("Chtimes failed: %v", err)
	}
	err = fs.Sync(fremote, flocal, true)
	if err != nil {
		log.Fatalf("Sync failed: %v", err)
	}
	items := []fstest.Item{
		{Path: "empty space", Size: 0, ModTime: t2, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
	}
	fstest.CheckListing(flocal, items)
	fstest.CheckListing(fremote, items)

	// ------------------------------------------------------------

	log.Printf("Sync after adding a file")
	WriteFile("potato", "------------------------------------------------------------", t3)
	err = fs.Sync(fremote, flocal, true)
	if err != nil {
		log.Fatalf("Sync failed: %v", err)
	}
	items = []fstest.Item{
		{Path: "empty space", Size: 0, ModTime: t2, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
		{Path: "potato", Size: 60, ModTime: t3, Md5sum: "d6548b156ea68a4e003e786df99eee76"},
	}
	fstest.CheckListing(flocal, items)
	fstest.CheckListing(fremote, items)

	// ------------------------------------------------------------

	log.Printf("Sync after changing a file's size only")
	WriteFile("potato", "smaller but same date", t3)
	err = fs.Sync(fremote, flocal, true)
	if err != nil {
		log.Fatalf("Sync failed: %v", err)
	}
	items = []fstest.Item{
		{Path: "empty space", Size: 0, ModTime: t2, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
		{Path: "potato", Size: 21, ModTime: t3, Md5sum: "100defcf18c42a1e0dc42a789b107cd2"},
	}
	fstest.CheckListing(flocal, items)
	fstest.CheckListing(fremote, items)

	// ------------------------------------------------------------

	log.Printf("Sync after changing a file's contents, modtime but not length")
	WriteFile("potato", "SMALLER BUT SAME DATE", t2)
	err = fs.Sync(fremote, flocal, true)
	if err != nil {
		log.Fatalf("Sync failed: %v", err)
	}
	items = []fstest.Item{
		{Path: "empty space", Size: 0, ModTime: t2, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
		{Path: "potato", Size: 21, ModTime: t2, Md5sum: "e4cb6955d9106df6263c45fcfc10f163"},
	}
	fstest.CheckListing(flocal, items)
	fstest.CheckListing(fremote, items)

	// ------------------------------------------------------------

	log.Printf("Sync after removing a file and adding a file --dry-run")
	WriteFile("potato2", "------------------------------------------------------------", t1)
	err = os.Remove(localName + "/potato")
	if err != nil {
		log.Fatalf("Remove failed: %v", err)
	}
	fs.Config.DryRun = true
	err = fs.Sync(fremote, flocal, true)
	fs.Config.DryRun = false
	if err != nil {
		log.Fatalf("Sync failed: %v", err)
	}

	before := []fstest.Item{
		{Path: "empty space", Size: 0, ModTime: t2, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
		{Path: "potato", Size: 21, ModTime: t2, Md5sum: "e4cb6955d9106df6263c45fcfc10f163"},
	}
	items = []fstest.Item{
		{Path: "empty space", Size: 0, ModTime: t2, Md5sum: "d41d8cd98f00b204e9800998ecf8427e"},
		{Path: "potato2", Size: 60, ModTime: t1, Md5sum: "d6548b156ea68a4e003e786df99eee76"},
	}
	fstest.CheckListing(flocal, items)
	fstest.CheckListing(fremote, before)

	log.Printf("Sync after removing a file and adding a file")
	err = fs.Sync(fremote, flocal, true)
	if err != nil {
		log.Fatalf("Sync failed: %v", err)
	}
	fstest.CheckListing(flocal, items)
	fstest.CheckListing(fremote, items)
}

func TestLs(flocal, fremote fs.Fs) {
	// Underlying List has been tested above, so we just make sure it runs
	err := fs.List(fremote)
	if err != nil {
		log.Fatalf("List failed: %v", err)
	}
}

func TestLsd(flocal, fremote fs.Fs) {
}

func TestCheck(flocal, fremote fs.Fs) {
}

func syntaxError() {
	fmt.Fprintf(os.Stderr, `Test rclone with a remote to find bugs in either - %s.

Syntax: [options] remote:

Need a remote: as argument.  This will create a random container or
directory under it and perform tests on it, deleting it at the end.

Options:

`, fs.Version)
	pflag.PrintDefaults()
}

// Clean the temporary directory
func cleanTempDir() {
	log.Printf("Cleaning temporary directory: %q", localName)
	err := os.RemoveAll(localName)
	if err != nil {
		log.Printf("Failed to remove %q: %v", localName, err)
	}
}

func main() {
	pflag.Usage = syntaxError
	pflag.Parse()
	if *version {
		fmt.Printf("rclonetest %s\n", fs.Version)
		os.Exit(0)
	}
	fs.LoadConfig()
	args := pflag.Args()

	if len(args) != 1 {
		syntaxError()
		os.Exit(1)
	}

	fremote, finalise := fstest.RandomRemote(args[0], *subDir)
	log.Printf("Testing with remote %v", fremote)

	var err error
	localName, err = ioutil.TempDir("", "rclone")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	log.Printf("Testing with local %q", localName)
	flocal, err := fs.NewFs(localName)
	if err != nil {
		log.Fatalf("Failed to make %q: %v", remoteName, err)
	}

	fs.CalculateModifyWindow(fremote, flocal)

	fstest.TestMkdir(fremote)
	TestCopy(flocal, fremote)
	TestSync(flocal, fremote)
	TestLs(flocal, fremote)
	TestLsd(flocal, fremote)
	TestCheck(flocal, fremote)
	//TestRmdir(flocal, fremote)

	finalise()

	cleanTempDir()
	log.Printf("Tests OK")
}
