// Package fstests provides generic integration tests for the Fs and
// Object interfaces
package fstests

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config"
	"github.com/ncw/rclone/fs/fserrors"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/fs/object"
	"github.com/ncw/rclone/fs/operations"
	"github.com/ncw/rclone/fs/walk"
	"github.com/ncw/rclone/fstest"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// InternalTester is an optional interface for Fs which allows to execute internal tests
//
// This interface should be implemented in 'backend'_internal_test.go and not in 'backend'.go
type InternalTester interface {
	InternalTest(*testing.T)
}

// winPath converts a path into a windows safe path
func winPath(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '<', '>', '"', '|', '?', '*', ':':
			return '_'
		}
		return r
	}, s)
}

// dirsToNames returns a sorted list of names
func dirsToNames(dirs []fs.Directory) []string {
	names := []string{}
	for _, dir := range dirs {
		names = append(names, winPath(fstest.Normalize(dir.Remote())))
	}
	sort.Strings(names)
	return names
}

// objsToNames returns a sorted list of object names
func objsToNames(objs []fs.Object) []string {
	names := []string{}
	for _, obj := range objs {
		names = append(names, winPath(fstest.Normalize(obj.Remote())))
	}
	sort.Strings(names)
	return names
}

// findObject finds the object on the remote
func findObject(t *testing.T, f fs.Fs, Name string) fs.Object {
	var obj fs.Object
	var err error
	for i := 1; i <= *fstest.ListRetries; i++ {
		obj, err = f.NewObject(Name)
		if err == nil {
			break
		}
		t.Logf("Sleeping for 1 second for findObject eventual consistency: %d/%d (%v)", i, *fstest.ListRetries, err)
		time.Sleep(1 * time.Second)
	}
	require.NoError(t, err)
	return obj
}

// testPut puts file to the remote
func testPut(t *testing.T, f fs.Fs, file *fstest.Item) string {
	tries := 1
	const maxTries = 10
again:
	contents := fstest.RandomString(100)
	buf := bytes.NewBufferString(contents)
	hash := hash.NewMultiHasher()
	in := io.TeeReader(buf, hash)

	file.Size = int64(buf.Len())
	obji := object.NewStaticObjectInfo(file.Path, file.ModTime, file.Size, true, nil, nil)
	obj, err := f.Put(in, obji)
	if err != nil {
		// Retry if err returned a retry error
		if fserrors.IsRetryError(err) && tries < maxTries {
			t.Logf("Put error: %v - low level retry %d/%d", err, tries, maxTries)
			time.Sleep(2 * time.Second)

			tries++
			goto again
		}
		require.NoError(t, err, fmt.Sprintf("Put error: %v", err))
	}
	file.Hashes = hash.Sums()
	file.Check(t, obj, f.Precision())
	// Re-read the object and check again
	obj = findObject(t, f, file.Path)
	file.Check(t, obj, f.Precision())
	return contents
}

// errorReader just returne an error on Read
type errorReader struct {
	err error
}

// Read returns an error immediately
func (er errorReader) Read(p []byte) (n int, err error) {
	return 0, er.err
}

// read the contents of an object as a string
func readObject(t *testing.T, obj fs.Object, limit int64, options ...fs.OpenOption) string {
	what := fmt.Sprintf("readObject(%q) limit=%d, options=%+v", obj, limit, options)
	in, err := obj.Open(options...)
	require.NoError(t, err, what)
	var r io.Reader = in
	if limit >= 0 {
		r = &io.LimitedReader{R: r, N: limit}
	}
	contents, err := ioutil.ReadAll(r)
	require.NoError(t, err, what)
	err = in.Close()
	require.NoError(t, err, what)
	return string(contents)
}

// ExtraConfigItem describes a config item for the tests
type ExtraConfigItem struct{ Name, Key, Value string }

// Opt is options for Run
type Opt struct {
	RemoteName  string
	NilObject   fs.Object
	ExtraConfig []ExtraConfigItem
	// SkipBadWindowsCharacters skips unusable characters for windows if set
	SkipBadWindowsCharacters bool
}

// Run runs the basic integration tests for a remote using the remote
// name passed in and the nil object
func Run(t *testing.T, opt *Opt) {
	var (
		remote        fs.Fs
		remoteName    = opt.RemoteName
		subRemoteName string
		subRemoteLeaf string
		file1         = fstest.Item{
			ModTime: fstest.Time("2001-02-03T04:05:06.499999999Z"),
			Path:    "file name.txt",
		}
		file1Contents string
		file2         = fstest.Item{
			ModTime: fstest.Time("2001-02-03T04:05:10.123123123Z"),
			Path:    `hello? sausage/êé/Hello, 世界/ " ' @ < > & ? + ≠/z.txt`,
			WinPath: `hello_ sausage/êé/Hello, 世界/ _ ' @ _ _ & _ + ≠/z.txt`,
		}
		isLocalRemote bool
	)

	// Make the Fs we are testing with, initialising the global variables
	// subRemoteName - name of the remote after the TestRemote:
	// subRemoteLeaf - a subdirectory to use under that
	// remote - the result of  fs.NewFs(TestRemote:subRemoteName)
	newFs := func(t *testing.T) {
		var err error
		subRemoteName, subRemoteLeaf, err = fstest.RandomRemoteName(remoteName)
		require.NoError(t, err)
		remote, err = fs.NewFs(subRemoteName)
		if err == fs.ErrorNotFoundInConfigFile {
			t.Logf("Didn't find %q in config file - skipping tests", remoteName)
			return
		}
		require.NoError(t, err, fmt.Sprintf("unexpected error: %v", err))
	}

	// Skip the test if the remote isn't configured
	skipIfNotOk := func(t *testing.T) {
		if remote == nil {
			t.Skipf("WARN: %q not configured", remoteName)
		}
	}

	// Skip if remote is not ListR capable, otherwise set the useListR
	// flag, returning a function to restore its value
	skipIfNotListR := func(t *testing.T) func() {
		skipIfNotOk(t)
		if remote.Features().ListR == nil {
			t.Skip("FS has no ListR interface")
		}
		previous := fs.Config.UseListR
		fs.Config.UseListR = true
		return func() {
			fs.Config.UseListR = previous
		}
	}

	// TestInit tests basic intitialisation
	t.Run("TestInit", func(t *testing.T) {
		var err error

		// Remove bad characters from Windows file name if set
		if opt.SkipBadWindowsCharacters {
			t.Logf("Removing bad windows characters from test file")
			file2.Path = winPath(file2.Path)
		}

		fstest.Initialise()

		// Set extra config if supplied
		for _, item := range opt.ExtraConfig {
			config.FileSet(item.Name, item.Key, item.Value)
		}
		if *fstest.RemoteName != "" {
			remoteName = *fstest.RemoteName
		}
		t.Logf("Using remote %q", remoteName)
		if remoteName == "" {
			remoteName, err = fstest.LocalRemote()
			require.NoError(t, err)
			isLocalRemote = true
		}

		newFs(t)

		skipIfNotOk(t)

		err = remote.Mkdir("")
		require.NoError(t, err)
		fstest.CheckListing(t, remote, []fstest.Item{})
	})

	// TestFsString tests the String method
	t.Run("TestFsString", func(t *testing.T) {
		skipIfNotOk(t)
		str := remote.String()
		require.NotEqual(t, "", str)
	})

	// TestFsName tests the Name method
	t.Run("TestFsName", func(t *testing.T) {
		skipIfNotOk(t)
		got := remote.Name()
		want := remoteName
		if isLocalRemote {
			want = "local:"
		}
		require.Equal(t, want, got+":")
	})

	// TestFsRoot tests the Root method
	t.Run("TestFsRoot", func(t *testing.T) {
		skipIfNotOk(t)
		name := remote.Name() + ":"
		root := remote.Root()
		if isLocalRemote {
			// only check last path element on local
			require.Equal(t, filepath.Base(subRemoteName), filepath.Base(root))
		} else {
			require.Equal(t, subRemoteName, name+root)
		}
	})

	// TestFsRmdirEmpty tests deleting an empty directory
	t.Run("TestFsRmdirEmpty", func(t *testing.T) {
		skipIfNotOk(t)
		err := remote.Rmdir("")
		require.NoError(t, err)
	})

	// TestFsRmdirNotFound tests deleting a non existent directory
	t.Run("TestFsRmdirNotFound", func(t *testing.T) {
		skipIfNotOk(t)
		err := remote.Rmdir("")
		assert.Error(t, err, "Expecting error on Rmdir non existent")
	})

	// TestFsMkdir tests making a directory
	t.Run("TestFsMkdir", func(t *testing.T) {
		skipIfNotOk(t)

		// Use a new directory here.  This is for the container based
		// remotes which take time to create and destroy a container
		// (eg azure blob)
		newFs(t)

		err := remote.Mkdir("")
		require.NoError(t, err)
		fstest.CheckListing(t, remote, []fstest.Item{})

		err = remote.Mkdir("")
		require.NoError(t, err)
	})

	// TestFsMkdirRmdirSubdir tests making and removing a sub directory
	t.Run("TestFsMkdirRmdirSubdir", func(t *testing.T) {
		skipIfNotOk(t)
		dir := "dir/subdir"
		err := operations.Mkdir(remote, dir)
		require.NoError(t, err)
		fstest.CheckListingWithPrecision(t, remote, []fstest.Item{}, []string{"dir", "dir/subdir"}, fs.Config.ModifyWindow)

		err = operations.Rmdir(remote, dir)
		require.NoError(t, err)
		fstest.CheckListingWithPrecision(t, remote, []fstest.Item{}, []string{"dir"}, fs.Config.ModifyWindow)

		err = operations.Rmdir(remote, "dir")
		require.NoError(t, err)
		fstest.CheckListingWithPrecision(t, remote, []fstest.Item{}, []string{}, fs.Config.ModifyWindow)
	})

	// TestFsListEmpty tests listing an empty directory
	t.Run("TestFsListEmpty", func(t *testing.T) {
		skipIfNotOk(t)
		fstest.CheckListing(t, remote, []fstest.Item{})
	})

	// TestFsListDirEmpty tests listing the directories from an empty directory
	TestFsListDirEmpty := func(t *testing.T) {
		skipIfNotOk(t)
		objs, dirs, err := walk.GetAll(remote, "", true, 1)
		require.NoError(t, err)
		assert.Equal(t, []string{}, objsToNames(objs))
		assert.Equal(t, []string{}, dirsToNames(dirs))
	}
	t.Run("TestFsListDirEmpty", TestFsListDirEmpty)

	// TestFsListRDirEmpty tests listing the directories from an empty directory using ListR
	t.Run("TestFsListRDirEmpty", func(t *testing.T) {
		defer skipIfNotListR(t)()
		TestFsListDirEmpty(t)
	})

	// TestFsNewObjectNotFound tests not finding a object
	t.Run("TestFsNewObjectNotFound", func(t *testing.T) {
		skipIfNotOk(t)
		// Object in an existing directory
		o, err := remote.NewObject("potato")
		assert.Nil(t, o)
		assert.Equal(t, fs.ErrorObjectNotFound, err)
		// Now try an object in a non existing directory
		o, err = remote.NewObject("directory/not/found/potato")
		assert.Nil(t, o)
		assert.Equal(t, fs.ErrorObjectNotFound, err)
	})

	// TestFsPutFile1 tests putting a file
	t.Run("TestFsPutFile1", func(t *testing.T) {
		skipIfNotOk(t)
		file1Contents = testPut(t, remote, &file1)
	})

	// TestFsPutError tests uploading a file where there is an error
	//
	// It makes sure that aborting a file half way through does not create
	// a file on the remote.
	t.Run("TestFsPutError", func(t *testing.T) {
		skipIfNotOk(t)

		// Read 50 bytes then produce an error
		contents := fstest.RandomString(50)
		buf := bytes.NewBufferString(contents)
		er := &errorReader{errors.New("potato")}
		in := io.MultiReader(buf, er)

		obji := object.NewStaticObjectInfo(file2.Path, file2.ModTime, 100, true, nil, nil)
		_, err := remote.Put(in, obji)
		// assert.Nil(t, obj) - FIXME some remotes return the object even on nil
		assert.NotNil(t, err)

		obj, err := remote.NewObject(file2.Path)
		assert.Nil(t, obj)
		assert.Equal(t, fs.ErrorObjectNotFound, err)
	})

	// TestFsPutFile2 tests putting a file into a subdirectory
	t.Run("TestFsPutFile2", func(t *testing.T) {
		skipIfNotOk(t)
		/* file2Contents = */ testPut(t, remote, &file2)
	})

	// TestFsUpdateFile1 tests updating file1 with new contents
	t.Run("TestFsUpdateFile1", func(t *testing.T) {
		skipIfNotOk(t)
		file1Contents = testPut(t, remote, &file1)
		// Note that the next test will check there are no duplicated file names
	})

	// TestFsListDirFile2 tests the files are correctly uploaded by doing
	// Depth 1 directory listings
	TestFsListDirFile2 := func(t *testing.T) {
		skipIfNotOk(t)
		list := func(dir string, expectedDirNames, expectedObjNames []string) {
			var objNames, dirNames []string
			for i := 1; i <= *fstest.ListRetries; i++ {
				objs, dirs, err := walk.GetAll(remote, dir, true, 1)
				if errors.Cause(err) == fs.ErrorDirNotFound {
					objs, dirs, err = walk.GetAll(remote, winPath(dir), true, 1)
				}
				require.NoError(t, err)
				objNames = objsToNames(objs)
				dirNames = dirsToNames(dirs)
				if len(objNames) >= len(expectedObjNames) && len(dirNames) >= len(expectedDirNames) {
					break
				}
				t.Logf("Sleeping for 1 second for TestFsListDirFile2 eventual consistency: %d/%d", i, *fstest.ListRetries)
				time.Sleep(1 * time.Second)
			}
			assert.Equal(t, expectedDirNames, dirNames)
			assert.Equal(t, expectedObjNames, objNames)
		}
		dir := file2.Path
		deepest := true
		for dir != "" {
			expectedObjNames := []string{}
			expectedDirNames := []string{}
			child := dir
			dir = path.Dir(dir)
			if dir == "." {
				dir = ""
				expectedObjNames = append(expectedObjNames, winPath(file1.Path))
			}
			if deepest {
				expectedObjNames = append(expectedObjNames, winPath(file2.Path))
				deepest = false
			} else {
				expectedDirNames = append(expectedDirNames, winPath(child))
			}
			list(dir, expectedDirNames, expectedObjNames)
		}
	}
	t.Run("TestFsListDirFile2", TestFsListDirFile2)

	// TestFsListRDirFile2 tests the files are correctly uploaded by doing
	// Depth 1 directory listings using ListR
	t.Run("TestFsListRDirFile2", func(t *testing.T) {
		defer skipIfNotListR(t)()
		TestFsListDirFile2(t)
	})

	// TestFsListDirRoot tests that DirList works in the root
	TestFsListDirRoot := func(t *testing.T) {
		skipIfNotOk(t)
		rootRemote, err := fs.NewFs(remoteName)
		require.NoError(t, err)
		_, dirs, err := walk.GetAll(rootRemote, "", true, 1)
		require.NoError(t, err)
		assert.Contains(t, dirsToNames(dirs), subRemoteLeaf, "Remote leaf not found")
	}
	t.Run("TestFsListDirRoot", TestFsListDirRoot)

	// TestFsListRDirRoot tests that DirList works in the root using ListR
	t.Run("TestFsListRDirRoot", func(t *testing.T) {
		defer skipIfNotListR(t)()
		TestFsListDirRoot(t)
	})

	// TestFsListSubdir tests List works for a subdirectory
	TestFsListSubdir := func(t *testing.T) {
		skipIfNotOk(t)
		fileName := file2.Path
		var err error
		var objs []fs.Object
		var dirs []fs.Directory
		for i := 0; i < 2; i++ {
			dir, _ := path.Split(fileName)
			dir = dir[:len(dir)-1]
			objs, dirs, err = walk.GetAll(remote, dir, true, -1)
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
	t.Run("TestFsListSubdir", TestFsListSubdir)

	// TestFsListRSubdir tests List works for a subdirectory using ListR
	t.Run("TestFsListRSubdir", func(t *testing.T) {
		defer skipIfNotListR(t)()
		TestFsListSubdir(t)
	})

	// TestFsListLevel2 tests List works for 2 levels
	TestFsListLevel2 := func(t *testing.T) {
		skipIfNotOk(t)
		objs, dirs, err := walk.GetAll(remote, "", true, 2)
		if err == fs.ErrorLevelNotSupported {
			return
		}
		require.NoError(t, err)
		assert.Equal(t, []string{file1.Path}, objsToNames(objs))
		assert.Equal(t, []string{`hello_ sausage`, `hello_ sausage/êé`}, dirsToNames(dirs))
	}
	t.Run("TestFsListLevel2", TestFsListLevel2)

	// TestFsListRLevel2 tests List works for 2 levels using ListR
	t.Run("TestFsListRLevel2", func(t *testing.T) {
		defer skipIfNotListR(t)()
		TestFsListLevel2(t)
	})

	// TestFsListFile1 tests file present
	t.Run("TestFsListFile1", func(t *testing.T) {
		skipIfNotOk(t)
		fstest.CheckListing(t, remote, []fstest.Item{file1, file2})
	})

	// TestFsNewObject tests NewObject
	t.Run("TestFsNewObject", func(t *testing.T) {
		skipIfNotOk(t)
		obj := findObject(t, remote, file1.Path)
		file1.Check(t, obj, remote.Precision())
	})

	// TestFsListFile1and2 tests two files present
	t.Run("TestFsListFile1and2", func(t *testing.T) {
		skipIfNotOk(t)
		fstest.CheckListing(t, remote, []fstest.Item{file1, file2})
	})

	// TestFsNewObjectDir tests NewObject on a directory which should produce an error
	t.Run("TestFsNewObjectDir", func(t *testing.T) {
		skipIfNotOk(t)
		dir := path.Dir(file2.Path)
		obj, err := remote.NewObject(dir)
		assert.Nil(t, obj)
		assert.NotNil(t, err)
	})

	// TestFsCopy tests Copy
	t.Run("TestFsCopy", func(t *testing.T) {
		skipIfNotOk(t)

		// Check have Copy
		doCopy := remote.Features().Copy
		if doCopy == nil {
			t.Skip("FS has no Copier interface")
		}

		// Test with file2 so have + and ' ' in file name
		var file2Copy = file2
		file2Copy.Path += "-copy"

		// do the copy
		src := findObject(t, remote, file2.Path)
		dst, err := doCopy(src, file2Copy.Path)
		if err == fs.ErrorCantCopy {
			t.Skip("FS can't copy")
		}
		require.NoError(t, err, fmt.Sprintf("Error: %#v", err))

		// check file exists in new listing
		fstest.CheckListing(t, remote, []fstest.Item{file1, file2, file2Copy})

		// Check dst lightly - list above has checked ModTime/Hashes
		assert.Equal(t, file2Copy.Path, dst.Remote())

		// Delete copy
		err = dst.Remove()
		require.NoError(t, err)

	})

	// TestFsMove tests Move
	t.Run("TestFsMove", func(t *testing.T) {
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
		src := findObject(t, remote, file2.Path)
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
		src = findObject(t, remote, file1.Path)
		_, err = doMove(src, file1Move.Path)
		require.NoError(t, err)
		fstest.CheckListing(t, remote, []fstest.Item{file1Move, file2Move})
		// 1: moveTest/other.txt
		// 2: other.txt

		// Check conflict on "move, then rename"
		src = findObject(t, remote, file1Move.Path)
		_, err = doMove(src, file1.Path)
		require.NoError(t, err)
		fstest.CheckListing(t, remote, []fstest.Item{file1, file2Move})
		// 1: file name.txt
		// 2: other.txt

		src = findObject(t, remote, file2Move.Path)
		_, err = doMove(src, file2.Path)
		require.NoError(t, err)
		fstest.CheckListing(t, remote, []fstest.Item{file1, file2})
		// 1: file name.txt
		// 2: hello sausage?/../z.txt
	})

	// Move src to this remote using server side move operations.
	//
	// Will only be called if src.Fs().Name() == f.Name()
	//
	// If it isn't possible then return fs.ErrorCantDirMove
	//
	// If destination exists then return fs.ErrorDirExists

	// TestFsDirMove tests DirMove
	//
	// go test -v -run '^Test(Setup|Init|FsMkdir|FsPutFile1|FsPutFile2|FsUpdateFile1|FsDirMove)$
	t.Run("TestFsDirMove", func(t *testing.T) {
		skipIfNotOk(t)

		// Check have DirMove
		doDirMove := remote.Features().DirMove
		if doDirMove == nil {
			t.Skip("FS has no DirMover interface")
		}

		// Check it can't move onto itself
		err := doDirMove(remote, "", "")
		require.Equal(t, fs.ErrorDirExists, err)

		// new remote
		newRemote, _, removeNewRemote, err := fstest.RandomRemote(remoteName, false)
		require.NoError(t, err)
		defer removeNewRemote()

		const newName = "new_name/sub_new_name"
		// try the move
		err = newRemote.Features().DirMove(remote, "", newName)
		require.NoError(t, err)

		// check remotes
		// FIXME: Prints errors.
		fstest.CheckListing(t, remote, []fstest.Item{})
		file1Copy := file1
		file1Copy.Path = path.Join(newName, file1.Path)
		file2Copy := file2
		file2Copy.Path = path.Join(newName, file2.Path)
		file2Copy.WinPath = path.Join(newName, file2.WinPath)
		fstest.CheckListing(t, newRemote, []fstest.Item{file2Copy, file1Copy})

		// move it back
		err = doDirMove(newRemote, newName, "")
		require.NoError(t, err)

		// check remotes
		fstest.CheckListing(t, remote, []fstest.Item{file2, file1})
		fstest.CheckListing(t, newRemote, []fstest.Item{})
	})

	// TestFsRmdirFull tests removing a non empty directory
	t.Run("TestFsRmdirFull", func(t *testing.T) {
		skipIfNotOk(t)
		err := remote.Rmdir("")
		require.Error(t, err, "Expecting error on RMdir on non empty remote")
	})

	// TestFsPrecision tests the Precision of the Fs
	t.Run("TestFsPrecision", func(t *testing.T) {
		skipIfNotOk(t)
		precision := remote.Precision()
		if precision == fs.ModTimeNotSupported {
			return
		}
		if precision > time.Second || precision < 0 {
			t.Fatalf("Precision out of range %v", precision)
		}
		// FIXME check expected precision
	})

	// TestFsChangeNotify tests that changes are properly
	// propagated
	//
	// go test -v -remote TestDrive: -run '^Test(Setup|Init|FsChangeNotify)$' -verbose
	t.Run("TestFsChangeNotify", func(t *testing.T) {
		skipIfNotOk(t)

		// Check have ChangeNotify
		doChangeNotify := remote.Features().ChangeNotify
		if doChangeNotify == nil {
			t.Skip("FS has no ChangeNotify interface")
		}

		err := operations.Mkdir(remote, "dir")
		require.NoError(t, err)

		dirChanges := []string{}
		objChanges := []string{}
		quitChannel := doChangeNotify(func(x string, e fs.EntryType) {
			fs.Debugf(nil, "doChangeNotify(%q, %+v)", x, e)
			if strings.HasPrefix(x, file1.Path[:5]) || strings.HasPrefix(x, file2.Path[:5]) {
				fs.Debugf(nil, "Ignoring notify for file1 or file2: %q, %v", x, e)
				return
			}
			if e == fs.EntryDirectory {
				dirChanges = append(dirChanges, x)
			} else if e == fs.EntryObject {
				objChanges = append(objChanges, x)
			}
		}, time.Second)
		defer func() { close(quitChannel) }()

		var dirs []string
		for _, idx := range []int{1, 3, 2} {
			dir := fmt.Sprintf("dir/subdir%d", idx)
			err = operations.Mkdir(remote, dir)
			require.NoError(t, err)
			dirs = append(dirs, dir)
		}

		contents := fstest.RandomString(100)
		buf := bytes.NewBufferString(contents)

		var objs []fs.Object
		for _, idx := range []int{2, 4, 3} {
			obji := object.NewStaticObjectInfo(fmt.Sprintf("dir/file%d", idx), time.Now(), int64(buf.Len()), true, nil, nil)
			o, err := remote.Put(buf, obji)
			require.NoError(t, err)
			objs = append(objs, o)
		}

		time.Sleep(3 * time.Second)

		assert.Equal(t, []string{"dir/subdir1", "dir/subdir3", "dir/subdir2"}, dirChanges)
		assert.Equal(t, []string{"dir/file2", "dir/file4", "dir/file3"}, objChanges)

		// tidy up afterwards
		for _, o := range objs {
			assert.NoError(t, o.Remove())
		}
		dirs = append(dirs, "dir")
		for _, dir := range dirs {
			assert.NoError(t, remote.Rmdir(dir))
		}
	})

	// TestObjectString tests the Object String method
	t.Run("TestObjectString", func(t *testing.T) {
		skipIfNotOk(t)
		obj := findObject(t, remote, file1.Path)
		assert.Equal(t, file1.Path, obj.String())
		assert.Equal(t, "<nil>", opt.NilObject.String())
	})

	// TestObjectFs tests the object can be found
	t.Run("TestObjectFs", func(t *testing.T) {
		skipIfNotOk(t)
		obj := findObject(t, remote, file1.Path)
		testRemote := remote
		if obj.Fs() != testRemote {
			// Check to see if this wraps something else
			if doUnWrap := testRemote.Features().UnWrap; doUnWrap != nil {
				testRemote = doUnWrap()
			}
		}
		assert.Equal(t, obj.Fs(), testRemote)
	})

	// TestObjectRemote tests the Remote is correct
	t.Run("TestObjectRemote", func(t *testing.T) {
		skipIfNotOk(t)
		obj := findObject(t, remote, file1.Path)
		assert.Equal(t, file1.Path, obj.Remote())
	})

	// TestObjectHashes checks all the hashes the object supports
	t.Run("TestObjectHashes", func(t *testing.T) {
		skipIfNotOk(t)
		obj := findObject(t, remote, file1.Path)
		file1.CheckHashes(t, obj)
	})

	// TestObjectModTime tests the ModTime of the object is correct
	TestObjectModTime := func(t *testing.T) {
		skipIfNotOk(t)
		obj := findObject(t, remote, file1.Path)
		file1.CheckModTime(t, obj, obj.ModTime(), remote.Precision())
	}
	t.Run("TestObjectModTime", TestObjectModTime)

	// TestObjectMimeType tests the MimeType of the object is correct
	t.Run("TestObjectMimeType", func(t *testing.T) {
		skipIfNotOk(t)
		obj := findObject(t, remote, file1.Path)
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
	})

	// TestObjectSetModTime tests that SetModTime works
	t.Run("TestObjectSetModTime", func(t *testing.T) {
		skipIfNotOk(t)
		newModTime := fstest.Time("2011-12-13T14:15:16.999999999Z")
		obj := findObject(t, remote, file1.Path)
		err := obj.SetModTime(newModTime)
		if err == fs.ErrorCantSetModTime || err == fs.ErrorCantSetModTimeWithoutDelete {
			t.Log(err)
			return
		}
		require.NoError(t, err)
		file1.ModTime = newModTime
		file1.CheckModTime(t, obj, obj.ModTime(), remote.Precision())
		// And make a new object and read it from there too
		TestObjectModTime(t)
	})

	// TestObjectSize tests that Size works
	t.Run("TestObjectSize", func(t *testing.T) {
		skipIfNotOk(t)
		obj := findObject(t, remote, file1.Path)
		assert.Equal(t, file1.Size, obj.Size())
	})

	// TestObjectOpen tests that Open works
	t.Run("TestObjectOpen", func(t *testing.T) {
		skipIfNotOk(t)
		obj := findObject(t, remote, file1.Path)
		assert.Equal(t, file1Contents, readObject(t, obj, -1), "contents of file1 differ")
	})

	// TestObjectOpenSeek tests that Open works with SeekOption
	t.Run("TestObjectOpenSeek", func(t *testing.T) {
		skipIfNotOk(t)
		obj := findObject(t, remote, file1.Path)
		assert.Equal(t, file1Contents[50:], readObject(t, obj, -1, &fs.SeekOption{Offset: 50}), "contents of file1 differ after seek")
	})

	// TestObjectOpenRange tests that Open works with RangeOption
	t.Run("TestObjectOpenRange", func(t *testing.T) {
		skipIfNotOk(t)
		obj := findObject(t, remote, file1.Path)
		for _, test := range []struct {
			ro                 fs.RangeOption
			wantStart, wantEnd int
		}{
			{fs.RangeOption{Start: 5, End: 15}, 5, 16},
			{fs.RangeOption{Start: 80, End: -1}, 80, 100},
			{fs.RangeOption{Start: 81, End: 100000}, 81, 100},
			{fs.RangeOption{Start: -1, End: 20}, 80, 100}, // if start is omitted this means get the final bytes
			// {fs.RangeOption{Start: -1, End: -1}, 0, 100}, - this seems to work but the RFC doesn't define it
		} {
			got := readObject(t, obj, -1, &test.ro)
			foundAt := strings.Index(file1Contents, got)
			help := fmt.Sprintf("%#v failed want [%d:%d] got [%d:%d]", test.ro, test.wantStart, test.wantEnd, foundAt, foundAt+len(got))
			assert.Equal(t, file1Contents[test.wantStart:test.wantEnd], got, help)
		}
	})

	// TestObjectPartialRead tests that reading only part of the object does the correct thing
	t.Run("TestObjectPartialRead", func(t *testing.T) {
		skipIfNotOk(t)
		obj := findObject(t, remote, file1.Path)
		assert.Equal(t, file1Contents[:50], readObject(t, obj, 50), "contents of file1 differ after limited read")
	})

	// TestObjectUpdate tests that Update works
	t.Run("TestObjectUpdate", func(t *testing.T) {
		skipIfNotOk(t)
		contents := fstest.RandomString(200)
		buf := bytes.NewBufferString(contents)
		hash := hash.NewMultiHasher()
		in := io.TeeReader(buf, hash)

		file1.Size = int64(buf.Len())
		obj := findObject(t, remote, file1.Path)
		obji := object.NewStaticObjectInfo(file1.Path, file1.ModTime, int64(len(contents)), true, nil, obj.Fs())
		err := obj.Update(in, obji)
		require.NoError(t, err)
		file1.Hashes = hash.Sums()

		// check the object has been updated
		file1.Check(t, obj, remote.Precision())

		// Re-read the object and check again
		obj = findObject(t, remote, file1.Path)
		file1.Check(t, obj, remote.Precision())

		// check contents correct
		assert.Equal(t, contents, readObject(t, obj, -1), "contents of updated file1 differ")
		file1Contents = contents
	})

	// TestObjectStorable tests that Storable works
	t.Run("TestObjectStorable", func(t *testing.T) {
		skipIfNotOk(t)
		obj := findObject(t, remote, file1.Path)
		require.NotNil(t, !obj.Storable(), "Expecting object to be storable")
	})

	// TestFsIsFile tests that an error is returned along with a valid fs
	// which points to the parent directory.
	t.Run("TestFsIsFile", func(t *testing.T) {
		skipIfNotOk(t)
		remoteName := subRemoteName + "/" + file2.Path
		file2Copy := file2
		file2Copy.Path = "z.txt"
		file2Copy.WinPath = ""
		fileRemote, err := fs.NewFs(remoteName)
		assert.Equal(t, fs.ErrorIsFile, err)
		fstest.CheckListing(t, fileRemote, []fstest.Item{file2Copy})
	})

	// TestFsIsFileNotFound tests that an error is not returned if no object is found
	t.Run("TestFsIsFileNotFound", func(t *testing.T) {
		skipIfNotOk(t)
		remoteName := subRemoteName + "/not found.txt"
		fileRemote, err := fs.NewFs(remoteName)
		require.NoError(t, err)
		fstest.CheckListing(t, fileRemote, []fstest.Item{})
	})

	// TestPublicLink tests creation of sharable, public links
	t.Run("TestPublicLink", func(t *testing.T) {
		skipIfNotOk(t)

		doPublicLink := remote.Features().PublicLink
		if doPublicLink == nil {
			t.Skip("FS has no PublicLinker interface")
		}

		// if object not found
		link, err := doPublicLink(file1.Path + "_does_not_exist")
		require.Error(t, err, "Expected to get error when file doesn't exist")
		require.Equal(t, "", link, "Expected link to be empty on error")

		// sharing file for the first time
		link1, err := doPublicLink(file1.Path)
		require.NoError(t, err)
		require.NotEqual(t, "", link1, "Link should not be empty")

		link2, err := doPublicLink(file2.Path)
		require.NoError(t, err)
		require.NotEqual(t, "", link2, "Link should not be empty")

		require.NotEqual(t, link1, link2, "Links to different files should differ")

		// sharing file for the 2nd time
		link1, err = doPublicLink(file1.Path)
		require.NoError(t, err)
		require.NotEqual(t, "", link1, "Link should not be empty")

		// sharing directory for the first time
		path := path.Dir(file2.Path)
		link3, err := doPublicLink(path)
		require.NoError(t, err)
		require.NotEqual(t, "", link3, "Link should not be empty")

		// sharing directory for the second time
		link3, err = doPublicLink(path)
		require.NoError(t, err)
		require.NotEqual(t, "", link3, "Link should not be empty")

		// sharing the "root" directory in a subremote
		subRemote, _, removeSubRemote, err := fstest.RandomRemote(remoteName, false)
		require.NoError(t, err)
		defer removeSubRemote()
		// ensure sub remote isn't empty
		buf := bytes.NewBufferString("somecontent")
		obji := object.NewStaticObjectInfo("somefile", time.Now(), int64(buf.Len()), true, nil, nil)
		_, err = subRemote.Put(buf, obji)
		require.NoError(t, err)

		link4, err := subRemote.Features().PublicLink("")
		require.NoError(t, err, "Sharing root in a sub-remote should work")
		require.NotEqual(t, "", link4, "Link should not be empty")
	})

	// TestObjectRemove tests Remove
	t.Run("TestObjectRemove", func(t *testing.T) {
		skipIfNotOk(t)
		obj := findObject(t, remote, file1.Path)
		err := obj.Remove()
		require.NoError(t, err)
		// check listing without modtime as TestPublicLink may change the modtime
		fstest.CheckListingWithPrecision(t, remote, []fstest.Item{file2}, nil, fs.ModTimeNotSupported)
	})

	// TestFsPutStream tests uploading files when size is not known in advance
	t.Run("TestFsPutStream", func(t *testing.T) {
		skipIfNotOk(t)
		if remote.Features().PutStream == nil {
			t.Skip("FS has no PutStream interface")
		}

		file := fstest.Item{
			ModTime: fstest.Time("2001-02-03T04:05:06.499999999Z"),
			Path:    "piped data.txt",
			Size:    -1, // use unknown size during upload
		}

		tries := 1
		const maxTries = 10
	again:
		contentSize := 100
		contents := fstest.RandomString(contentSize)
		buf := bytes.NewBufferString(contents)
		hash := hash.NewMultiHasher()
		in := io.TeeReader(buf, hash)

		file.Size = -1
		obji := object.NewStaticObjectInfo(file.Path, file.ModTime, file.Size, true, nil, nil)
		obj, err := remote.Features().PutStream(in, obji)
		if err != nil {
			// Retry if err returned a retry error
			if fserrors.IsRetryError(err) && tries < maxTries {
				t.Logf("Put error: %v - low level retry %d/%d", err, tries, maxTries)
				time.Sleep(2 * time.Second)

				tries++
				goto again
			}
			require.NoError(t, err, fmt.Sprintf("PutStream Unknown Length error: %v", err))
		}
		file.Hashes = hash.Sums()
		file.Size = int64(contentSize) // use correct size when checking
		file.Check(t, obj, remote.Precision())
		// Re-read the object and check again
		obj = findObject(t, remote, file.Path)
		file.Check(t, obj, remote.Precision())
	})

	// TestObjectPurge tests Purge
	t.Run("TestObjectPurge", func(t *testing.T) {
		skipIfNotOk(t)

		err := operations.Purge(remote, "")
		require.NoError(t, err)
		fstest.CheckListing(t, remote, []fstest.Item{})

		err = operations.Purge(remote, "")
		assert.Error(t, err, "Expecting error after on second purge")
	})

	// TestInternal calls InternalTest() on the Fs
	t.Run("TestInternal", func(t *testing.T) {
		skipIfNotOk(t)
		if it, ok := remote.(InternalTester); ok {
			it.InternalTest(t)
		} else {
			t.Skipf("%T does not implement InternalTester", remote)
		}
	})

	// TestFinalise tidies up after the previous tests
	t.Run("TestFinalise", func(t *testing.T) {
		skipIfNotOk(t)
		if strings.HasPrefix(remoteName, "/") {
			// Remove temp directory
			err := os.Remove(remoteName)
			require.NoError(t, err)
		}
	})
}
