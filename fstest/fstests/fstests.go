// Generic tests for testing the Fs and Object interfaces
package fstests

// FIXME need to check the limited file system

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
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
	}
)

func TestInit(t *testing.T) {
	var err error
	fs.LoadConfig()
	fs.Config.Verbose = false
	fs.Config.Quiet = true
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
	fstest.Fatalf = t.Fatalf
	fstest.TestMkdir(remote)
}

func skipIfNotOk(t *testing.T) {
	fstest.Fatalf = t.Fatalf
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
	fstest.TestRmdir(remote)
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
	fstest.TestMkdir(remote)
	fstest.TestMkdir(remote)
}

func TestFsListEmpty(t *testing.T) {
	skipIfNotOk(t)
	fstest.CheckListing(remote, []fstest.Item{})
}

func TestFsListDirEmpty(t *testing.T) {
	skipIfNotOk(t)
	for obj := range remote.ListDir() {
		t.Error("Found unexpected item %q", obj.Name)
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
	file.Check(obj)
	// Re-read the object and check again
	obj = findObject(t, file.Path)
	file.Check(obj)
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
		if obj.Name != `hello? sausage` {
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
		t.Fatal("Failed to make remote %q: %v", RemoteName, err)
	}
	found := false
	for obj := range rootRemote.ListDir() {
		fmt.Printf("obj = %q\n", obj.Name)
		if obj.Name == subRemoteLeaf {
			found = true
		}
	}
	if !found {
		t.Errorf("Didn't find %q", subRemoteLeaf)
	}
}

func TestFsListFile1(t *testing.T) {
	skipIfNotOk(t)
	fstest.CheckListing(remote, []fstest.Item{file1, file2})
}

func TestFsNewFsObject(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	file1.Check(obj)
}

func TestFsListFile1and2(t *testing.T) {
	skipIfNotOk(t)
	fstest.CheckListing(remote, []fstest.Item{file1, file2})
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
	if Md5sum != file1.Md5sum {
		t.Errorf("Md5sum is wrong %v != %v", Md5sum, file1.Md5sum)
	}
}

func TestObjectModTime(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	file1.CheckModTime(obj, obj.ModTime())
}

func TestObjectSetModTime(t *testing.T) {
	skipIfNotOk(t)
	newModTime := fstest.Time("2011-12-13T14:15:16.999999999Z")
	obj := findObject(t, file1.Path)
	obj.SetModTime(newModTime)
	file1.ModTime = newModTime
	file1.CheckModTime(obj, newModTime)
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
	if Md5sum != file1.Md5sum {
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
	file1.Check(obj)
	// Re-read the object and check again
	obj = findObject(t, file1.Path)
	file1.Check(obj)
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
		t.Fatal("Failed to make remote %q: %v", remoteName, err)
	}
	fstest.CheckListing(fileRemote, []fstest.Item{file2Copy})
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
		t.Fatal("Failed to make remote %q: %v", remoteName, err)
	}
	fstest.CheckListing(fileRemote, []fstest.Item{})
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
	fstest.CheckListing(remote, []fstest.Item{file2})
}

func TestObjectPurge(t *testing.T) {
	skipIfNotOk(t)
	fstest.TestPurge(remote)
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
