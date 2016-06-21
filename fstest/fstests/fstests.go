// Package fstests provides generic tests for testing the Fs and Object interfaces
//
// Run go generate to write the tests for the remotes
package fstests

//go:generate go run gen_tests.go

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	remote fs.Fs
	// RemoteName should be set to the name of the remote for testing
	RemoteName    = ""
	subRemoteName = ""
	subRemoteLeaf = ""
	// NilObject should be set to a nil Object from the Fs under test
	NilObject fs.Object
	file1     = fstest.Item{
		ModTime: fstest.Time("2001-02-03T04:05:06.499999999Z"),
		Path:    "file name.txt",
	}
	file2 = fstest.Item{
		ModTime: fstest.Time("2001-02-03T04:05:10.123123123Z"),
		Path:    `hello? sausage/êé/Hello, 世界/ " ' @ < > & ?/z.txt`,
		WinPath: `hello_ sausage/êé/Hello, 世界/ _ ' @ _ _ & _/z.txt`,
	}
	verbose     = flag.Bool("verbose", false, "Set to enable logging")
	dumpHeaders = flag.Bool("dump-headers", false, "Dump HTTP headers - may contain sensitive info")
	dumpBodies  = flag.Bool("dump-bodies", false, "Dump HTTP headers and bodies - may contain sensitive info")
)

const eventualConsistencyRetries = 10

func init() {
	flag.StringVar(&RemoteName, "remote", "", "Set this to override the default remote name (eg s3:)")
}

// TestInit tests basic intitialisation
func TestInit(t *testing.T) {
	var err error

	// Never ask for passwords, fail instead.
	// If your local config is encrypted set environment variable
	// "RCLONE_CONFIG_PASS=hunter2" (or your password)
	*fs.AskPassword = false
	fs.LoadConfig()
	fs.Config.Verbose = *verbose
	fs.Config.Quiet = !*verbose
	fs.Config.DumpHeaders = *dumpHeaders
	fs.Config.DumpBodies = *dumpBodies
	t.Logf("Using remote %q", RemoteName)
	if RemoteName == "" {
		RemoteName, err = fstest.LocalRemote()
		if err != nil {
			t.Fatalf("Failed to create tmp dir: %v", err)
		}
	}
	subRemoteName, subRemoteLeaf, err = fstest.RandomRemoteName(RemoteName)
	if err != nil {
		t.Fatalf("Couldn't make remote name: %v", err)
	}

	remote, err = fs.NewFs(subRemoteName)
	if err == fs.ErrorNotFoundInConfigFile {
		t.Logf("Didn't find %q in config file - skipping tests", RemoteName)
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

// TestFsString tests the String method
func TestFsString(t *testing.T) {
	skipIfNotOk(t)
	str := remote.String()
	if str == "" {
		t.Fatal("Bad fs.String()")
	}
}

// TestFsRmdirEmpty tests deleting an empty directory
func TestFsRmdirEmpty(t *testing.T) {
	skipIfNotOk(t)
	fstest.TestRmdir(t, remote)
}

// TestFsRmdirNotFound tests deleting a non existent directory
func TestFsRmdirNotFound(t *testing.T) {
	skipIfNotOk(t)
	err := remote.Rmdir()
	if err == nil {
		t.Fatalf("Expecting error on Rmdir non existent")
	}
}

// TestFsMkdir tests tests making a directory
func TestFsMkdir(t *testing.T) {
	skipIfNotOk(t)
	fstest.TestMkdir(t, remote)
	fstest.TestMkdir(t, remote)
}

// TestFsListEmpty tests listing an empty directory
func TestFsListEmpty(t *testing.T) {
	skipIfNotOk(t)
	fstest.CheckListing(t, remote, []fstest.Item{})
}

// winPath converts a path into a windows safe path
func winPath(s string) string {
	s = strings.Replace(s, "?", "_", -1)
	s = strings.Replace(s, `"`, "_", -1)
	s = strings.Replace(s, "<", "_", -1)
	s = strings.Replace(s, ">", "_", -1)
	return s
}

// dirsToNames returns a sorted list of names
func dirsToNames(dirs []*fs.Dir) []string {
	names := []string{}
	for _, dir := range dirs {
		names = append(names, winPath(dir.Name))
	}
	sort.Strings(names)
	return names
}

// objsToNames returns a sorted list of object names
func objsToNames(objs []fs.Object) []string {
	names := []string{}
	for _, obj := range objs {
		names = append(names, winPath(obj.Remote()))
	}
	sort.Strings(names)
	return names
}

// TestFsListDirEmpty tests listing the directories from an empty directory
func TestFsListDirEmpty(t *testing.T) {
	skipIfNotOk(t)
	objs, dirs, err := fs.NewLister().SetLevel(1).Start(remote, "").GetAll()
	require.NoError(t, err)
	assert.Equal(t, []string{}, objsToNames(objs))
	assert.Equal(t, []string{}, dirsToNames(dirs))
}

// TestFsNewFsObjectNotFound tests not finding a object
func TestFsNewFsObjectNotFound(t *testing.T) {
	skipIfNotOk(t)
	if remote.NewFsObject("potato") != nil {
		t.Fatal("Didn't expect to find object")
	}
}

func findObject(t *testing.T, Name string) fs.Object {
	var obj fs.Object
	for i := 1; i <= eventualConsistencyRetries; i++ {
		obj = remote.NewFsObject(Name)
		if obj != nil {
			break
		}
		t.Logf("Sleeping for 1 second for findObject eventual consistency: %d/%d", i, eventualConsistencyRetries)
		time.Sleep(1 * time.Second)
	}
	if obj == nil {
		t.Fatalf("Object not found: %q", Name)
	}
	return obj
}

func testPut(t *testing.T, file *fstest.Item) {
again:
	buf := bytes.NewBufferString(fstest.RandomString(100))
	hash := fs.NewMultiHasher()
	in := io.TeeReader(buf, hash)

	tries := 1
	const maxTries = 10
	file.Size = int64(buf.Len())
	obji := fs.NewStaticObjectInfo(file.Path, file.ModTime, file.Size, true, nil, nil)
	obj, err := remote.Put(in, obji)
	if err != nil {
		// Retry if err returned a retry error
		if r, ok := err.(fs.Retry); ok && r.Retry() && tries < maxTries {
			t.Logf("Put error: %v - low level retry %d/%d", err, tries, maxTries)

			tries++
			goto again
		}
		t.Fatal("Put error", err)
	}
	file.Hashes = hash.Sums()
	file.Check(t, obj, remote.Precision())
	// Re-read the object and check again
	obj = findObject(t, file.Path)
	file.Check(t, obj, remote.Precision())
}

// TestFsPutFile1 tests putting a file
func TestFsPutFile1(t *testing.T) {
	skipIfNotOk(t)
	testPut(t, &file1)
}

// TestFsPutFile2 tests putting a file into a subdirectory
func TestFsPutFile2(t *testing.T) {
	skipIfNotOk(t)
	testPut(t, &file2)
}

// TestFsUpdateFile1 tests updating file1 with new contents
func TestFsUpdateFile1(t *testing.T) {
	skipIfNotOk(t)
	testPut(t, &file1)
	// Note that the next test will check there are no duplicated file names
}

// TestFsListDirFile2 tests the files are correctly uploaded
func TestFsListDirFile2(t *testing.T) {
	skipIfNotOk(t)
	var objNames, dirNames []string
	for i := 1; i <= eventualConsistencyRetries; i++ {
		objs, dirs, err := fs.NewLister().SetLevel(1).Start(remote, "").GetAll()
		require.NoError(t, err)
		objNames = objsToNames(objs)
		dirNames = dirsToNames(dirs)
		if len(objNames) >= 1 && len(dirNames) >= 1 {
			break
		}
		t.Logf("Sleeping for 1 second for TestFsListDirFile2 eventual consistency: %d/%d", i, eventualConsistencyRetries)
		time.Sleep(1 * time.Second)
	}
	assert.Equal(t, []string{`hello_ sausage`}, dirNames)
	assert.Equal(t, []string{file1.Path}, objNames)
}

// TestFsListDirRoot tests that DirList works in the root
func TestFsListDirRoot(t *testing.T) {
	skipIfNotOk(t)
	rootRemote, err := fs.NewFs(RemoteName)
	if err != nil {
		t.Fatalf("Failed to make remote %q: %v", RemoteName, err)
	}
	dirs, err := fs.NewLister().SetLevel(1).Start(rootRemote, "").GetDirs()
	require.NoError(t, err)
	assert.Contains(t, dirsToNames(dirs), subRemoteLeaf, "Remote leaf not found")
}

// TestFsListSubdir tests List works for a subdirectory
func TestFsListSubdir(t *testing.T) {
	skipIfNotOk(t)
	fileName := file2.Path
	if runtime.GOOS == "windows" {
		fileName = file2.WinPath
	}
	dir, _ := path.Split(fileName)
	dir = dir[:len(dir)-1]
	objs, dirs, err := fs.NewLister().Start(remote, dir).GetAll()
	require.NoError(t, err)
	require.Len(t, objs, 1)
	assert.Equal(t, fileName, objs[0].Remote())
	require.Len(t, dirs, 0)
}

// TestFsListLevel2 tests List works for 2 levels
func TestFsListLevel2(t *testing.T) {
	skipIfNotOk(t)
	objs, dirs, err := fs.NewLister().SetLevel(2).Start(remote, "").GetAll()
	if err == fs.ErrorLevelNotSupported {
		return
	}
	require.NoError(t, err)
	assert.Equal(t, []string{file1.Path}, objsToNames(objs))
	assert.Equal(t, []string{`hello_ sausage`, `hello_ sausage/êé`}, dirsToNames(dirs))
}

// TestFsListFile1 tests file present
func TestFsListFile1(t *testing.T) {
	skipIfNotOk(t)
	fstest.CheckListing(t, remote, []fstest.Item{file1, file2})
}

// TestFsNewFsObject tests NewFsObject
func TestFsNewFsObject(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	file1.Check(t, obj, remote.Precision())
}

// TestFsListFile1and2 tests two files present
func TestFsListFile1and2(t *testing.T) {
	skipIfNotOk(t)
	fstest.CheckListing(t, remote, []fstest.Item{file1, file2})
}

// TestFsCopy tests Copy
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
		t.Fatalf("Copy failed: %v (%#v)", err, err)
	}

	// check file exists in new listing
	fstest.CheckListing(t, remote, []fstest.Item{file1, file2, file1Copy})

	// Check dst lightly - list above has checked ModTime/Hashes
	if dst.Remote() != file1Copy.Path {
		t.Errorf("object path: want %q got %q", file1Copy.Path, dst.Remote())
	}

	// Delete copy
	err = dst.Remove()
	if err != nil {
		t.Fatal("Remove copy error", err)
	}

}

// TestFsMove tests Move
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

	// Check dst lightly - list above has checked ModTime/Hashes
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

// TestFsDirMove tests DirMove
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

// TestFsRmdirFull tests removing a non empty directory
func TestFsRmdirFull(t *testing.T) {
	skipIfNotOk(t)
	err := remote.Rmdir()
	if err == nil {
		t.Fatalf("Expecting error on RMdir on non empty remote")
	}
}

// TestFsPrecision tests the Precision of the Fs
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

// TestObjectString tests the Object String method
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

// TestObjectFs tests the object can be found
func TestObjectFs(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	equal := obj.Fs() == remote
	if !equal {
		// Check to see if this wraps something else
		if unwrap, ok := remote.(fs.UnWrapper); ok {
			equal = obj.Fs() == unwrap.UnWrap()
		}
	}
	if !equal {
		t.Errorf("Fs is wrong %v != %v", obj.Fs(), remote)
	}
}

// TestObjectRemote tests the Remote is correct
func TestObjectRemote(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	if obj.Remote() != file1.Path {
		t.Errorf("Remote is wrong %v != %v", obj.Remote(), file1.Path)
	}
}

// TestObjectHashes checks all the hashes the object supports
func TestObjectHashes(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	file1.CheckHashes(t, obj)
}

// TestObjectModTime tests the ModTime of the object is correct
func TestObjectModTime(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	file1.CheckModTime(t, obj, obj.ModTime(), remote.Precision())
}

// TestObjectSetModTime tests that SetModTime works
func TestObjectSetModTime(t *testing.T) {
	skipIfNotOk(t)
	newModTime := fstest.Time("2011-12-13T14:15:16.999999999Z")
	obj := findObject(t, file1.Path)
	err := obj.SetModTime(newModTime)
	if err == fs.ErrorCantSetModTime {
		t.Log(err)
		return
	} else if err != nil {
		t.Fatal(err)
	}
	file1.ModTime = newModTime
	file1.CheckModTime(t, obj, obj.ModTime(), remote.Precision())
	// And make a new object and read it from there too
	TestObjectModTime(t)
}

// TestObjectSize tests that Size works
func TestObjectSize(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	if obj.Size() != file1.Size {
		t.Errorf("Size is wrong %v != %v", obj.Size(), file1.Size)
	}
}

// TestObjectOpen tests that Open works
func TestObjectOpen(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	in, err := obj.Open()
	if err != nil {
		t.Fatalf("Open() return error: %v", err)
	}
	hasher := fs.NewMultiHasher()
	n, err := io.Copy(hasher, in)
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
	// Check content of file by comparing the calculated hashes
	for hashType, got := range hasher.Sums() {
		want := file1.Hashes[hashType]
		if want != got {
			t.Errorf("%v is wrong %v != %v", hashType, want, got)
		}
	}

}

// TestObjectUpdate tests that Update works
func TestObjectUpdate(t *testing.T) {
	skipIfNotOk(t)
	buf := bytes.NewBufferString(fstest.RandomString(200))
	hash := fs.NewMultiHasher()
	in := io.TeeReader(buf, hash)

	file1.Size = int64(buf.Len())
	obj := findObject(t, file1.Path)
	obji := fs.NewStaticObjectInfo("", file1.ModTime, file1.Size, true, nil, obj.Fs())
	err := obj.Update(in, obji)
	if err != nil {
		t.Fatal("Update error", err)
	}
	file1.Hashes = hash.Sums()
	file1.Check(t, obj, remote.Precision())
	// Re-read the object and check again
	obj = findObject(t, file1.Path)
	file1.Check(t, obj, remote.Precision())
}

// TestObjectStorable tests that Storable works
func TestObjectStorable(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	if !obj.Storable() {
		t.Fatalf("Expecting %v to be storable", obj)
	}
}

// TestFsIsFile tests that an error is returned along with a valid fs
// which points to the parent directory.
func TestFsIsFile(t *testing.T) {
	skipIfNotOk(t)
	remoteName := subRemoteName + "/" + file2.Path
	file2Copy := file2
	file2Copy.Path = "z.txt"
	fileRemote, err := fs.NewFs(remoteName)
	assert.Equal(t, fs.ErrorIsFile, err)
	fstest.CheckListing(t, fileRemote, []fstest.Item{file2Copy})
}

// TestFsIsFileNotFound tests that an error is not returned if no object is found
func TestFsIsFileNotFound(t *testing.T) {
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

// TestObjectRemove tests Remove
func TestObjectRemove(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	err := obj.Remove()
	if err != nil {
		t.Fatal("Remove error", err)
	}
	fstest.CheckListing(t, remote, []fstest.Item{file2})
}

// TestObjectPurge tests Purge
func TestObjectPurge(t *testing.T) {
	skipIfNotOk(t)
	fstest.TestPurge(t, remote)
	err := fs.Purge(remote)
	if err == nil {
		t.Fatal("Expecting error after on second purge")
	}
}

// TestFinalise tidies up after the previous tests
func TestFinalise(t *testing.T) {
	skipIfNotOk(t)
	if strings.HasPrefix(RemoteName, "/") {
		// Remove temp directory
		err := os.Remove(RemoteName)
		if err != nil {
			t.Logf("Failed to remove %q: %v\n", RemoteName, err)
		}
	}
}
