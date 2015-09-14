// Generic tests for testing the Fs and Object interfaces
//
// Run go generate to write the tests for the remotes

//go:generate go run gen_tests.go
package fstests

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest"
)

var (
	remote        fs.Fs
	RemoteName    = ""
	subRemoteName = ""
	subRemoteLeaf = ""
	NilObject     fs.Object
	file1         = fstest.Item{
		ModTime: fstest.Time("2001-02-03T04:05:06.499999999Z"),
		Path:    "file name.txt",
	}
	file2 = fstest.Item{
		ModTime: fstest.Time("2001-02-03T04:05:10.123123123Z"),
		Path:    `hello? sausage/êé/Hello, 世界/ " ' @ < > & ?/z.txt`,
		WinPath: `hello_ sausage/êé/Hello, 世界/ _ ' @ _ _ _ _/z.txt`,
	}
)

func init() {
	flag.StringVar(&RemoteName, "remote", "", "Set this to override the default remote name (eg s3:)")
}

func TestInit(t *testing.T) {
	var err error
	fs.LoadConfig()
	fs.Config.Verbose = false
	fs.Config.Quiet = true
	t.Logf("Using remote %q", RemoteName)
	if RemoteName == "" {
		RemoteName, err = fstest.LocalRemote()
		if err != nil {
			log.Fatalf("Failed to create tmp dir: %v", err)
		}
	}
	subRemoteName, subRemoteLeaf, err = fstest.RandomRemoteName(RemoteName)
	if err != nil {
		t.Fatalf("Couldn't make remote name: %v", err)
	}

	remote, err = fs.NewFs(subRemoteName)
	if err == fs.NotFoundInConfigFile {
		log.Printf("Didn't find %q in config file - skipping tests", RemoteName)
		return
	}
	if err != nil {
		t.Fatalf("Couldn't start FS: %v", err)
	}
	fstest.TestMkdir(t, remote)
}

func skipIfNotOk(t *testing.T) {
	if remote == nil {
		t.Skip("FS not configured")
	}
}

// String returns a description of the FS

func TestFsString(t *testing.T) {
	skipIfNotOk(t)
	str := remote.String()
	if str == "" {
		t.Fatal("Bad fs.String()")
	}
}

type TestFile struct {
	ModTime time.Time
	Path    string
	Size    int64
	Md5sum  string
}

func TestFsRmdirEmpty(t *testing.T) {
	skipIfNotOk(t)
	fstest.TestRmdir(t, remote)
}

func TestFsRmdirNotFound(t *testing.T) {
	skipIfNotOk(t)
	err := remote.Rmdir()
	if err == nil {
		t.Fatalf("Expecting error on Rmdir non existent")
	}
}

func TestFsMkdir(t *testing.T) {
	skipIfNotOk(t)
	fstest.TestMkdir(t, remote)
	fstest.TestMkdir(t, remote)
}

func TestFsListEmpty(t *testing.T) {
	skipIfNotOk(t)
	fstest.CheckListing(t, remote, []fstest.Item{})
}

func TestFsListDirEmpty(t *testing.T) {
	skipIfNotOk(t)
	for obj := range remote.ListDir() {
		t.Errorf("Found unexpected item %q", obj.Name)
	}
}

func TestFsNewFsObjectNotFound(t *testing.T) {
	skipIfNotOk(t)
	if remote.NewFsObject("potato") != nil {
		t.Fatal("Didn't expect to find object")
	}
}

func findObject(t *testing.T, Name string) fs.Object {
	obj := remote.NewFsObject(Name)
	if obj == nil {
		t.Fatalf("Object not found: %q", Name)
	}
	return obj
}

func testPut(t *testing.T, file *fstest.Item) {
	buf := bytes.NewBufferString(fstest.RandomString(100))
	hash := md5.New()
	in := io.TeeReader(buf, hash)

	file.Size = int64(buf.Len())
	obj, err := remote.Put(in, file.Path, file.ModTime, file.Size)
	if err != nil {
		t.Fatal("Put error", err)
	}
	file.Md5sum = hex.EncodeToString(hash.Sum(nil))
	file.Check(t, obj, remote.Precision())
	// Re-read the object and check again
	obj = findObject(t, file.Path)
	file.Check(t, obj, remote.Precision())
}

func TestFsPutFile1(t *testing.T) {
	skipIfNotOk(t)
	testPut(t, &file1)
}

func TestFsPutFile2(t *testing.T) {
	skipIfNotOk(t)
	testPut(t, &file2)
}

func TestFsListDirFile2(t *testing.T) {
	skipIfNotOk(t)
	found := false
	for obj := range remote.ListDir() {
		if obj.Name != `hello? sausage` && obj.Name != `hello_ sausage` {
			t.Errorf("Found unexpected item %q", obj.Name)
		} else {
			found = true
		}
	}
	if !found {
		t.Errorf("Didn't find %q", `hello? sausage`)
	}
}

func TestFsListDirRoot(t *testing.T) {
	skipIfNotOk(t)
	rootRemote, err := fs.NewFs(RemoteName)
	if err != nil {
		t.Fatalf("Failed to make remote %q: %v", RemoteName, err)
	}
	found := false
	for obj := range rootRemote.ListDir() {
		if obj.Name == subRemoteLeaf {
			found = true
		}
	}
	if !found {
		t.Errorf("Didn't find %q", subRemoteLeaf)
	}
}

func TestFsListRoot(t *testing.T) {
	skipIfNotOk(t)
	rootRemote, err := fs.NewFs(RemoteName)
	if err != nil {
		t.Fatalf("Failed to make remote %q: %v", RemoteName, err)
	}
	// Should either find file1 and file2 or nothing
	found1 := false
	f1 := subRemoteLeaf + "/" + file1.Path
	found2 := false
	f2 := subRemoteLeaf + "/" + file2.Path
	f2Alt := subRemoteLeaf + "/" + file2.WinPath
	count := 0
	errors := fs.Stats.GetErrors()
	for obj := range rootRemote.List() {
		count++
		if obj.Remote() == f1 {
			found1 = true
		}
		if obj.Remote() == f2 || obj.Remote() == f2Alt {
			found2 = true
		}
	}
	errors -= fs.Stats.GetErrors()
	if count == 0 {
		if errors == 0 {
			t.Error("Expecting error if count==0")
		}
		return
	}
	if found1 && found2 {
		if errors != 0 {
			t.Error("Not expecting error if found")
		}
		return
	}
	t.Errorf("Didn't find %q (%v) and %q (%v) or no files (count %d)", f1, found1, f2, found2, count)
}

func TestFsListFile1(t *testing.T) {
	skipIfNotOk(t)
	fstest.CheckListing(t, remote, []fstest.Item{file1, file2})
}

func TestFsNewFsObject(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	file1.Check(t, obj, remote.Precision())
}

func TestFsListFile1and2(t *testing.T) {
	skipIfNotOk(t)
	fstest.CheckListing(t, remote, []fstest.Item{file1, file2})
}

func TestFsCopy(t *testing.T) {
	skipIfNotOk(t)

	// Check have Copy
	_, ok := remote.(fs.Copier)
	if !ok {
		t.Skip("FS has no Copier interface")
	}

	var file1Copy = file1
	file1Copy.Path += "-copy"

	// do the copy
	src := findObject(t, file1.Path)
	dst, err := remote.(fs.Copier).Copy(src, file1Copy.Path)
	if err != nil {
		t.Errorf("Copy failed: %v", err)
	}

	// check file exists in new listing
	fstest.CheckListing(t, remote, []fstest.Item{file1, file2, file1Copy})

	// Check dst lightly - list above has checked ModTime/Md5sum
	if dst.Remote() != file1Copy.Path {
		t.Errorf("object path: want %q got %q", file1Copy.Path, dst.Remote())
	}

	// Delete copy
	err = dst.Remove()
	if err != nil {
		t.Fatal("Remove copy error", err)
	}

}

func TestFsMove(t *testing.T) {
	skipIfNotOk(t)

	// Check have Move
	_, ok := remote.(fs.Mover)
	if !ok {
		t.Skip("FS has no Mover interface")
	}

	var file1Move = file1
	file1Move.Path += "-move"

	// do the move
	src := findObject(t, file1.Path)
	dst, err := remote.(fs.Mover).Move(src, file1Move.Path)
	if err != nil {
		t.Fatalf("Move failed: %v", err)
	}

	// check file exists in new listing
	fstest.CheckListing(t, remote, []fstest.Item{file2, file1Move})

	// Check dst lightly - list above has checked ModTime/Md5sum
	if dst.Remote() != file1Move.Path {
		t.Errorf("object path: want %q got %q", file1Move.Path, dst.Remote())
	}

	// move it back
	src = findObject(t, file1Move.Path)
	_, err = remote.(fs.Mover).Move(src, file1.Path)
	if err != nil {
		t.Errorf("Move failed: %v", err)
	}

	// check file exists in new listing
	fstest.CheckListing(t, remote, []fstest.Item{file2, file1})
}

// Move src to this remote using server side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func TestFsDirMove(t *testing.T) {
	skipIfNotOk(t)

	// Check have DirMove
	_, ok := remote.(fs.DirMover)
	if !ok {
		t.Skip("FS has no DirMover interface")
	}

	// Check it can't move onto itself
	err := remote.(fs.DirMover).DirMove(remote)
	if err != fs.ErrorDirExists {
		t.Errorf("Expecting fs.ErrorDirExists got: %v", err)
	}

	// new remote
	newRemote, removeNewRemote, err := fstest.RandomRemote(RemoteName, false)
	if err != nil {
		t.Fatalf("Failed to create remote: %v", err)
	}
	defer removeNewRemote()

	// try the move
	err = newRemote.(fs.DirMover).DirMove(remote)
	if err != nil {
		t.Errorf("Failed to DirMove: %v", err)
	}

	// check remotes
	// FIXME: Prints errors.
	fstest.CheckListing(t, remote, []fstest.Item{})
	fstest.CheckListing(t, newRemote, []fstest.Item{file2, file1})

	// move it back
	err = remote.(fs.DirMover).DirMove(newRemote)
	if err != nil {
		t.Errorf("Failed to DirMove: %v", err)
	}

	// check remotes
	fstest.CheckListing(t, remote, []fstest.Item{file2, file1})
	fstest.CheckListing(t, newRemote, []fstest.Item{})
}

func TestFsRmdirFull(t *testing.T) {
	skipIfNotOk(t)
	err := remote.Rmdir()
	if err == nil {
		t.Fatalf("Expecting error on RMdir on non empty remote")
	}
}

func TestFsPrecision(t *testing.T) {
	skipIfNotOk(t)
	precision := remote.Precision()
	if precision == fs.ModTimeNotSupported {
		return
	}
	if precision > time.Second || precision < 0 {
		t.Fatalf("Precision out of range %v", precision)
	}
	// FIXME check expected precision
}

func TestObjectString(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	s := obj.String()
	if s != file1.Path {
		t.Errorf("String() wrong %v != %v", s, file1.Path)
	}
	obj = NilObject
	s = obj.String()
	if s != "<nil>" {
		t.Errorf("String() wrong %v != %v", s, "<nil>")
	}
}

func TestObjectFs(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	if obj.Fs() != remote {
		t.Errorf("Fs is wrong %v != %v", obj.Fs(), remote)
	}
}

func TestObjectRemote(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	if obj.Remote() != file1.Path {
		t.Errorf("Remote is wrong %v != %v", obj.Remote(), file1.Path)
	}
}

func TestObjectMd5sum(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	Md5sum, err := obj.Md5sum()
	if err != nil {
		t.Errorf("Error in Md5sum: %v", err)
	}
	if !fs.Md5sumsEqual(Md5sum, file1.Md5sum) {
		t.Errorf("Md5sum is wrong %v != %v", Md5sum, file1.Md5sum)
	}
}

func TestObjectModTime(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	file1.CheckModTime(t, obj, obj.ModTime(), remote.Precision())
}

func TestObjectSetModTime(t *testing.T) {
	skipIfNotOk(t)
	newModTime := fstest.Time("2011-12-13T14:15:16.999999999Z")
	obj := findObject(t, file1.Path)
	obj.SetModTime(newModTime)
	file1.ModTime = newModTime
	file1.CheckModTime(t, obj, obj.ModTime(), remote.Precision())
	// And make a new object and read it from there too
	TestObjectModTime(t)
}

func TestObjectSize(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	if obj.Size() != file1.Size {
		t.Errorf("Size is wrong %v != %v", obj.Size(), file1.Size)
	}
}

func TestObjectOpen(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	in, err := obj.Open()
	if err != nil {
		t.Fatalf("Open() return error: %v", err)
	}
	hash := md5.New()
	n, err := io.Copy(hash, in)
	if err != nil {
		t.Fatalf("io.Copy() return error: %v", err)
	}
	if n != file1.Size {
		t.Fatalf("Read wrong number of bytes %d != %d", n, file1.Size)
	}
	err = in.Close()
	if err != nil {
		t.Fatalf("in.Close() return error: %v", err)
	}
	Md5sum := hex.EncodeToString(hash.Sum(nil))
	if !fs.Md5sumsEqual(Md5sum, file1.Md5sum) {
		t.Errorf("Md5sum is wrong %v != %v", Md5sum, file1.Md5sum)
	}
}

func TestObjectUpdate(t *testing.T) {
	skipIfNotOk(t)
	buf := bytes.NewBufferString(fstest.RandomString(200))
	hash := md5.New()
	in := io.TeeReader(buf, hash)

	file1.Size = int64(buf.Len())
	obj := findObject(t, file1.Path)
	err := obj.Update(in, file1.ModTime, file1.Size)
	if err != nil {
		t.Fatal("Update error", err)
	}
	file1.Md5sum = hex.EncodeToString(hash.Sum(nil))
	file1.Check(t, obj, remote.Precision())
	// Re-read the object and check again
	obj = findObject(t, file1.Path)
	file1.Check(t, obj, remote.Precision())
}

func TestObjectStorable(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	if !obj.Storable() {
		t.Fatalf("Expecting %v to be storable", obj)
	}
}

func TestLimitedFs(t *testing.T) {
	skipIfNotOk(t)
	remoteName := subRemoteName + "/" + file2.Path
	file2Copy := file2
	file2Copy.Path = "z.txt"
	fileRemote, err := fs.NewFs(remoteName)
	if err != nil {
		t.Fatalf("Failed to make remote %q: %v", remoteName, err)
	}
	fstest.CheckListing(t, fileRemote, []fstest.Item{file2Copy})
	_, ok := fileRemote.(*fs.Limited)
	if !ok {
		t.Errorf("%v is not a fs.Limited", fileRemote)
	}
}

func TestLimitedFsNotFound(t *testing.T) {
	skipIfNotOk(t)
	remoteName := subRemoteName + "/not found.txt"
	fileRemote, err := fs.NewFs(remoteName)
	if err != nil {
		t.Fatalf("Failed to make remote %q: %v", remoteName, err)
	}
	fstest.CheckListing(t, fileRemote, []fstest.Item{})
	_, ok := fileRemote.(*fs.Limited)
	if ok {
		t.Errorf("%v is is a fs.Limited", fileRemote)
	}
}

func TestObjectRemove(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	err := obj.Remove()
	if err != nil {
		t.Fatal("Remove error", err)
	}
	fstest.CheckListing(t, remote, []fstest.Item{file2})
}

func TestObjectPurge(t *testing.T) {
	skipIfNotOk(t)
	fstest.TestPurge(t, remote)
	err := fs.Purge(remote)
	if err == nil {
		t.Fatal("Expecting error after on second purge")
	}
}

func TestFinalise(t *testing.T) {
	skipIfNotOk(t)
	if strings.HasPrefix(RemoteName, "/") {
		// Remove temp directory
		err := os.Remove(RemoteName)
		if err != nil {
			log.Printf("Failed to remove %q: %v\n", RemoteName, err)
		}
	}
}
