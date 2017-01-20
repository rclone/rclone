// Package fstests provides generic tests for testing the Fs and Object interfaces
//
// Run go generate to write the tests for the remotes
package fstests

//go:generate go run gen_tests.go

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
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
	// ExtraConfig is for adding config to a remote
	ExtraConfig = []ExtraConfigItem{}
	file1       = fstest.Item{
		ModTime: fstest.Time("2001-02-03T04:05:06.499999999Z"),
		Path:    "file name.txt",
	}
	file1Contents = ""
	file2         = fstest.Item{
		ModTime: fstest.Time("2001-02-03T04:05:10.123123123Z"),
		Path:    `hello? sausage/êé/Hello, 世界/ " ' @ < > & ? + ≠/z.txt`,
		WinPath: `hello_ sausage/êé/Hello, 世界/ _ ' @ _ _ & _ + ≠/z.txt`,
	}
	file2Contents = ""
	verbose       = flag.Bool("verbose", false, "Set to enable logging")
	dumpHeaders   = flag.Bool("dump-headers", false, "Dump HTTP headers - may contain sensitive info")
	dumpBodies    = flag.Bool("dump-bodies", false, "Dump HTTP headers and bodies - may contain sensitive info")
)

// ExtraConfigItem describes a config item added on the fly while testing
type ExtraConfigItem struct{ Name, Key, Value string }

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
	// Set extra config if supplied
	for _, item := range ExtraConfig {
		fs.ConfigFileSet(item.Name, item.Key, item.Value)
	}
	fs.Config.Verbose = *verbose
	fs.Config.Quiet = !*verbose
	fs.Config.DumpHeaders = *dumpHeaders
	fs.Config.DumpBodies = *dumpBodies
	t.Logf("Using remote %q", RemoteName)
	if RemoteName == "" {
		RemoteName, err = fstest.LocalRemote()
		require.NoError(t, err)
	}
	subRemoteName, subRemoteLeaf, err = fstest.RandomRemoteName(RemoteName)
	require.NoError(t, err)

	remote, err = fs.NewFs(subRemoteName)
	if err == fs.ErrorNotFoundInConfigFile {
		t.Logf("Didn't find %q in config file - skipping tests", RemoteName)
		return
	}
	require.NoError(t, err, fmt.Sprintf("unexpected error: %v", err))
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
	require.NotEqual(t, str, "")
}

// TestFsRmdirEmpty tests deleting an empty directory
func TestFsRmdirEmpty(t *testing.T) {
	skipIfNotOk(t)
	fstest.TestRmdir(t, remote)
}

// TestFsRmdirNotFound tests deleting a non existent directory
func TestFsRmdirNotFound(t *testing.T) {
	skipIfNotOk(t)
	err := remote.Rmdir("")
	assert.Error(t, err, "Expecting error on Rmdir non existent")
}

// TestFsMkdir tests tests making a directory
func TestFsMkdir(t *testing.T) {
	skipIfNotOk(t)
	fstest.TestMkdir(t, remote)
	fstest.TestMkdir(t, remote)
}

// TestFsMkdirRmdirSubdir tests making and removing a sub directory
func TestFsMkdirRmdirSubdir(t *testing.T) {
	skipIfNotOk(t)
	dir := "dir/subdir"
	err := fs.Mkdir(remote, dir)
	require.NoError(t, err)
	fstest.CheckListingWithPrecision(t, remote, []fstest.Item{}, []string{"dir", "dir/subdir"}, fs.Config.ModifyWindow)

	err = fs.Rmdir(remote, dir)
	require.NoError(t, err)
	fstest.CheckListingWithPrecision(t, remote, []fstest.Item{}, []string{"dir"}, fs.Config.ModifyWindow)

	err = fs.Rmdir(remote, "dir")
	require.NoError(t, err)
	fstest.CheckListingWithPrecision(t, remote, []fstest.Item{}, []string{}, fs.Config.ModifyWindow)
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

// TestFsNewObjectNotFound tests not finding a object
func TestFsNewObjectNotFound(t *testing.T) {
	skipIfNotOk(t)
	// Object in an existing directory
	o, err := remote.NewObject("potato")
	assert.Nil(t, o)
	assert.Equal(t, fs.ErrorObjectNotFound, err)
	// Now try an object in a non existing directory
	o, err = remote.NewObject("directory/not/found/potato")
	assert.Nil(t, o)
	assert.Equal(t, fs.ErrorObjectNotFound, err)
}

func findObject(t *testing.T, Name string) fs.Object {
	var obj fs.Object
	var err error
	for i := 1; i <= eventualConsistencyRetries; i++ {
		obj, err = remote.NewObject(Name)
		if err == nil {
			break
		}
		t.Logf("Sleeping for 1 second for findObject eventual consistency: %d/%d (%v)", i, eventualConsistencyRetries, err)
		time.Sleep(1 * time.Second)
	}
	require.NoError(t, err)
	return obj
}

func testPut(t *testing.T, file *fstest.Item) string {
again:
	contents := fstest.RandomString(100)
	buf := bytes.NewBufferString(contents)
	hash := fs.NewMultiHasher()
	in := io.TeeReader(buf, hash)

	tries := 1
	const maxTries = 10
	file.Size = int64(buf.Len())
	obji := fs.NewStaticObjectInfo(file.Path, file.ModTime, file.Size, true, nil, nil)
	obj, err := remote.Put(in, obji)
	if err != nil {
		// Retry if err returned a retry error
		if fs.IsRetryError(err) && tries < maxTries {
			t.Logf("Put error: %v - low level retry %d/%d", err, tries, maxTries)
			time.Sleep(2 * time.Second)

			tries++
			goto again
		}
		require.NoError(t, err, fmt.Sprintf("Put error: %v", err))
	}
	file.Hashes = hash.Sums()
	file.Check(t, obj, remote.Precision())
	// Re-read the object and check again
	obj = findObject(t, file.Path)
	file.Check(t, obj, remote.Precision())
	return contents
}

// TestFsPutFile1 tests putting a file
func TestFsPutFile1(t *testing.T) {
	skipIfNotOk(t)
	file1Contents = testPut(t, &file1)
}

type errorReader struct {
	err error
}

func (er errorReader) Read(p []byte) (n int, err error) {
	return 0, er.err
}

// TestFsPutError tests uploading a file where there is an error
//
// It makes sure that aborting a file half way through does not create
// a file on the remote.
func TestFsPutError(t *testing.T) {
	skipIfNotOk(t)

	// Read 50 bytes then produce an error
	contents := fstest.RandomString(50)
	buf := bytes.NewBufferString(contents)
	er := &errorReader{errors.New("potato")}
	in := io.MultiReader(buf, er)

	obji := fs.NewStaticObjectInfo(file2.Path, file2.ModTime, 100, true, nil, nil)
	obj, err := remote.Put(in, obji)
	// assert.Nil(t, obj) - FIXME some remotes return the object even on nil
	assert.NotNil(t, err)

	obj, err = remote.NewObject(file2.Path)
	assert.Nil(t, obj)
	assert.Equal(t, fs.ErrorObjectNotFound, err)
}

// TestFsPutFile2 tests putting a file into a subdirectory
func TestFsPutFile2(t *testing.T) {
	skipIfNotOk(t)
	file2Contents = testPut(t, &file2)
}

// TestFsUpdateFile1 tests updating file1 with new contents
func TestFsUpdateFile1(t *testing.T) {
	skipIfNotOk(t)
	file1Contents = testPut(t, &file1)
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
	require.NoError(t, err)
	dirs, err := fs.NewLister().SetLevel(1).Start(rootRemote, "").GetDirs()
	require.NoError(t, err)
	assert.Contains(t, dirsToNames(dirs), subRemoteLeaf, "Remote leaf not found")
}

// TestFsListSubdir tests List works for a subdirectory
func TestFsListSubdir(t *testing.T) {
	skipIfNotOk(t)
	fileName := file2.Path
	var err error
	var objs []fs.Object
	var dirs []*fs.Dir
	for i := 0; i < 2; i++ {
		dir, _ := path.Split(fileName)
		dir = dir[:len(dir)-1]
		objs, dirs, err = fs.NewLister().Start(remote, dir).GetAll()
		if err != fs.ErrorDirNotFound {
			break
		}
		fileName = file2.WinPath
	}
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

// TestFsNewObject tests NewObject
func TestFsNewObject(t *testing.T) {
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
	doCopy := remote.Features().Copy
	if doCopy == nil {
		t.Skip("FS has no Copier interface")
	}

	var file1Copy = file1
	file1Copy.Path += "-copy"

	// do the copy
	src := findObject(t, file1.Path)
	dst, err := doCopy(src, file1Copy.Path)
	if err == fs.ErrorCantCopy {
		t.Skip("FS can't copy")
	}
	require.NoError(t, err, fmt.Sprintf("Error: %#v", err))

	// check file exists in new listing
	fstest.CheckListing(t, remote, []fstest.Item{file1, file2, file1Copy})

	// Check dst lightly - list above has checked ModTime/Hashes
	assert.Equal(t, file1Copy.Path, dst.Remote())

	// Delete copy
	err = dst.Remove()
	require.NoError(t, err)

}

// TestFsMove tests Move
func TestFsMove(t *testing.T) {
	skipIfNotOk(t)

	// Check have Move
	doMove := remote.Features().Move
	if doMove == nil {
		t.Skip("FS has no Mover interface")
	}

	// state of files now:
	// 1: file name.txt
	// 2: hello sausage?/../z.txt

	var file1Move = file1
	var file2Move = file2

	// check happy path, i.e. no naming conflicts when rename and move are two
	// separate operations
	file2Move.Path = "other.txt"
	file2Move.WinPath = ""
	src := findObject(t, file2.Path)
	dst, err := doMove(src, file2Move.Path)
	if err == fs.ErrorCantMove {
		t.Skip("FS can't move")
	}
	require.NoError(t, err)
	// check file exists in new listing
	fstest.CheckListing(t, remote, []fstest.Item{file1, file2Move})
	// Check dst lightly - list above has checked ModTime/Hashes
	assert.Equal(t, file2Move.Path, dst.Remote())
	// 1: file name.txt
	// 2: other.txt

	// Check conflict on "rename, then move"
	file1Move.Path = "moveTest/other.txt"
	src = findObject(t, file1.Path)
	_, err = doMove(src, file1Move.Path)
	require.NoError(t, err)
	fstest.CheckListing(t, remote, []fstest.Item{file1Move, file2Move})
	// 1: moveTest/other.txt
	// 2: other.txt

	// Check conflict on "move, then rename"
	src = findObject(t, file1Move.Path)
	_, err = doMove(src, file1.Path)
	require.NoError(t, err)
	fstest.CheckListing(t, remote, []fstest.Item{file1, file2Move})
	// 1: file name.txt
	// 2: other.txt

	src = findObject(t, file2Move.Path)
	_, err = doMove(src, file2.Path)
	require.NoError(t, err)
	fstest.CheckListing(t, remote, []fstest.Item{file1, file2})
	// 1: file name.txt
	// 2: hello sausage?/../z.txt
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
	doDirMove := remote.Features().DirMove
	if doDirMove == nil {
		t.Skip("FS has no DirMover interface")
	}

	// Check it can't move onto itself
	err := doDirMove(remote)
	require.Equal(t, fs.ErrorDirExists, err)

	// new remote
	newRemote, _, removeNewRemote, err := fstest.RandomRemote(RemoteName, false)
	require.NoError(t, err)
	defer removeNewRemote()

	// try the move
	err = newRemote.Features().DirMove(remote)
	require.NoError(t, err)

	// check remotes
	// FIXME: Prints errors.
	fstest.CheckListing(t, remote, []fstest.Item{})
	fstest.CheckListing(t, newRemote, []fstest.Item{file2, file1})

	// move it back
	err = doDirMove(newRemote)
	require.NoError(t, err)

	// check remotes
	fstest.CheckListing(t, remote, []fstest.Item{file2, file1})
	fstest.CheckListing(t, newRemote, []fstest.Item{})
}

// TestFsRmdirFull tests removing a non empty directory
func TestFsRmdirFull(t *testing.T) {
	skipIfNotOk(t)
	err := remote.Rmdir("")
	require.Error(t, err, "Expecting error on RMdir on non empty remote")
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
	assert.Equal(t, file1.Path, obj.String())
	assert.Equal(t, "<nil>", NilObject.String())
}

// TestObjectFs tests the object can be found
func TestObjectFs(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	testRemote := remote
	if obj.Fs() != testRemote {
		// Check to see if this wraps something else
		if doUnWrap := testRemote.Features().UnWrap; doUnWrap != nil {
			testRemote = doUnWrap()
		}
	}
	assert.Equal(t, obj.Fs(), testRemote)
}

// TestObjectRemote tests the Remote is correct
func TestObjectRemote(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	assert.Equal(t, file1.Path, obj.Remote())
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

// TestObjectMimeType tests the MimeType of the object is correct
func TestObjectMimeType(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	do, ok := obj.(fs.MimeTyper)
	if !ok {
		t.Skip("MimeType method not supported")
	}
	mimeType := do.MimeType()
	if strings.ContainsRune(mimeType, ';') {
		assert.Equal(t, "text/plain; charset=utf-8", mimeType)
	} else {
		assert.Equal(t, "text/plain", mimeType)
	}
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
	}
	require.NoError(t, err)
	file1.ModTime = newModTime
	file1.CheckModTime(t, obj, obj.ModTime(), remote.Precision())
	// And make a new object and read it from there too
	TestObjectModTime(t)
}

// TestObjectSize tests that Size works
func TestObjectSize(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	assert.Equal(t, file1.Size, obj.Size())
}

// read the contents of an object as a string
func readObject(t *testing.T, obj fs.Object, options ...fs.OpenOption) string {
	in, err := obj.Open(options...)
	require.NoError(t, err)
	contents, err := ioutil.ReadAll(in)
	require.NoError(t, err)
	err = in.Close()
	require.NoError(t, err)
	return string(contents)
}

// TestObjectOpen tests that Open works
func TestObjectOpen(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	assert.Equal(t, file1Contents, readObject(t, obj), "contents of file1 differ")
}

// TestObjectOpenSeek tests that Open works with Seek
func TestObjectOpenSeek(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	assert.Equal(t, file1Contents[50:], readObject(t, obj, &fs.SeekOption{Offset: 50}), "contents of file1 differ after seek")
}

// TestObjectUpdate tests that Update works
func TestObjectUpdate(t *testing.T) {
	skipIfNotOk(t)
	contents := fstest.RandomString(200)
	buf := bytes.NewBufferString(contents)
	hash := fs.NewMultiHasher()
	in := io.TeeReader(buf, hash)

	file1.Size = int64(buf.Len())
	obj := findObject(t, file1.Path)
	obji := fs.NewStaticObjectInfo(file1.Path, file1.ModTime, int64(len(contents)), true, nil, obj.Fs())
	err := obj.Update(in, obji)
	require.NoError(t, err)
	file1.Hashes = hash.Sums()

	// check the object has been updated
	file1.Check(t, obj, remote.Precision())

	// Re-read the object and check again
	obj = findObject(t, file1.Path)
	file1.Check(t, obj, remote.Precision())

	// check contents correct
	assert.Equal(t, contents, readObject(t, obj), "contents of updated file1 differ")
	file1Contents = contents
}

// TestObjectStorable tests that Storable works
func TestObjectStorable(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	require.NotNil(t, !obj.Storable(), "Expecting object to be storable")
}

// TestFsIsFile tests that an error is returned along with a valid fs
// which points to the parent directory.
func TestFsIsFile(t *testing.T) {
	skipIfNotOk(t)
	remoteName := subRemoteName + "/" + file2.Path
	file2Copy := file2
	file2Copy.Path = "z.txt"
	file2Copy.WinPath = ""
	fileRemote, err := fs.NewFs(remoteName)
	assert.Equal(t, fs.ErrorIsFile, err)
	fstest.CheckListing(t, fileRemote, []fstest.Item{file2Copy})
}

// TestFsIsFileNotFound tests that an error is not returned if no object is found
func TestFsIsFileNotFound(t *testing.T) {
	skipIfNotOk(t)
	remoteName := subRemoteName + "/not found.txt"
	fileRemote, err := fs.NewFs(remoteName)
	require.NoError(t, err)
	fstest.CheckListing(t, fileRemote, []fstest.Item{})
}

// TestObjectRemove tests Remove
func TestObjectRemove(t *testing.T) {
	skipIfNotOk(t)
	obj := findObject(t, file1.Path)
	err := obj.Remove()
	require.NoError(t, err)
	fstest.CheckListing(t, remote, []fstest.Item{file2})
}

// TestObjectPurge tests Purge
func TestObjectPurge(t *testing.T) {
	skipIfNotOk(t)
	fstest.TestPurge(t, remote)
	err := fs.Purge(remote)
	assert.Error(t, err, "Expecting error after on second purge")
}

// TestFinalise tidies up after the previous tests
func TestFinalise(t *testing.T) {
	skipIfNotOk(t)
	if strings.HasPrefix(RemoteName, "/") {
		// Remove temp directory
		err := os.Remove(RemoteName)
		require.NoError(t, err)
	}
}
