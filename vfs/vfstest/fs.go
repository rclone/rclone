// Test suite for rclonefs

package vfstest

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/all" // import all the backends
	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfsflags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	mountFn mountlib.MountFn
)

// RunTests runs all the tests against all the VFS cache modes
//
// If useVFS is set then it runs the tests against a VFS rather than amount
func RunTests(t *testing.T, useVFS bool, fn mountlib.MountFn) {
	mountFn = fn
	flag.Parse()
	tests := []struct {
		cacheMode vfscommon.CacheMode
		writeBack time.Duration
	}{
		{cacheMode: vfscommon.CacheModeOff},
		{cacheMode: vfscommon.CacheModeMinimal},
		{cacheMode: vfscommon.CacheModeWrites},
		{cacheMode: vfscommon.CacheModeFull},
		{cacheMode: vfscommon.CacheModeFull, writeBack: 100 * time.Millisecond},
	}
	run = newRun(useVFS)
	for _, test := range tests {
		run.cacheMode(test.cacheMode, test.writeBack)
		what := fmt.Sprintf("CacheMode=%v", test.cacheMode)
		if test.writeBack > 0 {
			what += fmt.Sprintf(",WriteBack=%v", test.writeBack)
		}
		log.Printf("Starting test run with %s", what)
		ok := t.Run(what, func(t *testing.T) {
			t.Run("TestTouchAndDelete", TestTouchAndDelete)
			t.Run("TestRenameOpenHandle", TestRenameOpenHandle)
			t.Run("TestDirLs", TestDirLs)
			t.Run("TestDirCreateAndRemoveDir", TestDirCreateAndRemoveDir)
			t.Run("TestDirCreateAndRemoveFile", TestDirCreateAndRemoveFile)
			t.Run("TestDirRenameFile", TestDirRenameFile)
			t.Run("TestDirRenameEmptyDir", TestDirRenameEmptyDir)
			t.Run("TestDirRenameFullDir", TestDirRenameFullDir)
			t.Run("TestDirModTime", TestDirModTime)
			t.Run("TestDirCacheFlush", TestDirCacheFlush)
			t.Run("TestDirCacheFlushOnDirRename", TestDirCacheFlushOnDirRename)
			t.Run("TestFileModTime", TestFileModTime)
			t.Run("TestFileModTimeWithOpenWriters", TestFileModTimeWithOpenWriters)
			t.Run("TestMount", TestMount)
			t.Run("TestRoot", TestRoot)
			t.Run("TestReadByByte", TestReadByByte)
			t.Run("TestReadChecksum", TestReadChecksum)
			t.Run("TestReadFileDoubleClose", TestReadFileDoubleClose)
			t.Run("TestReadSeek", TestReadSeek)
			t.Run("TestWriteFileNoWrite", TestWriteFileNoWrite)
			t.Run("TestWriteFileWrite", TestWriteFileWrite)
			t.Run("TestWriteFileOverwrite", TestWriteFileOverwrite)
			t.Run("TestWriteFileDoubleClose", TestWriteFileDoubleClose)
			t.Run("TestWriteFileFsync", TestWriteFileFsync)
			t.Run("TestWriteFileDup", TestWriteFileDup)
			t.Run("TestWriteFileAppend", TestWriteFileAppend)
		})
		log.Printf("Finished test run with %s (ok=%v)", what, ok)
		if !ok {
			break
		}
	}
	run.Finalise()
}

// Run holds the remotes for a test run
type Run struct {
	os           Oser
	vfs          *vfs.VFS
	useVFS       bool // set if we are testing a VFS not a mount
	mountPath    string
	fremote      fs.Fs
	fremoteName  string
	cleanRemote  func()
	umountResult <-chan error
	umountFn     mountlib.UnmountFn
	skip         bool
}

// run holds the master Run data
var run *Run

// newRun initialise the remote mount for testing and returns a run
// object.
//
// r.fremote is an empty remote Fs
//
// Finalise() will tidy them away when done.
func newRun(useVFS bool) *Run {
	r := &Run{
		useVFS:       useVFS,
		umountResult: make(chan error, 1),
	}
	fstest.Initialise()

	var err error
	r.fremote, r.fremoteName, r.cleanRemote, err = fstest.RandomRemote()
	if err != nil {
		log.Fatalf("Failed to open remote %q: %v", *fstest.RemoteName, err)
	}

	err = r.fremote.Mkdir(context.Background(), "")
	if err != nil {
		log.Fatalf("Failed to open mkdir %q: %v", *fstest.RemoteName, err)
	}

	if !r.useVFS {
		r.mountPath = findMountPath()
	}
	// Mount it up
	r.mount()

	return r
}

func findMountPath() string {
	if runtime.GOOS != "windows" {
		mountPath, err := ioutil.TempDir("", "rclonefs-mount")
		if err != nil {
			log.Fatalf("Failed to create mount dir: %v", err)
		}
		return mountPath
	}

	// Find a free drive letter
	drive := ""
	for letter := 'E'; letter <= 'Z'; letter++ {
		drive = string(letter) + ":"
		_, err := os.Stat(drive + "\\")
		if os.IsNotExist(err) {
			goto found
		}
	}
	log.Fatalf("Couldn't find free drive letter for test")
found:
	return drive
}

func (r *Run) mount() {
	log.Printf("mount %q %q", r.fremote, r.mountPath)
	var err error
	r.vfs = vfs.New(r.fremote, &vfsflags.Opt)
	r.umountResult, r.umountFn, err = mountFn(r.vfs, r.mountPath, &mountlib.Opt)
	if err != nil {
		log.Printf("mount FAILED: %v", err)
		r.skip = true
	} else {
		log.Printf("mount OK")
	}
	if r.useVFS {
		r.os = vfsOs{r.vfs}
	} else {
		r.os = realOs{}
	}

}

func (r *Run) umount() {
	if r.skip {
		log.Printf("FUSE not found so skipping umount")
		return
	}
	/*
		log.Printf("Calling fusermount -u %q", r.mountPath)
		err := exec.Command("fusermount", "-u", r.mountPath).Run()
		if err != nil {
			log.Printf("fusermount failed: %v", err)
		}
	*/
	log.Printf("Unmounting %q", r.mountPath)
	err := r.umountFn()
	if err != nil {
		log.Printf("signal to umount failed - retrying: %v", err)
		time.Sleep(3 * time.Second)
		err = r.umountFn()
	}
	if err != nil {
		log.Fatalf("signal to umount failed: %v", err)
	}
	log.Printf("Waiting for umount")
	err = <-r.umountResult
	if err != nil {
		log.Fatalf("umount failed: %v", err)
	}

	// Cleanup the VFS cache - umount has called Shutdown
	err = r.vfs.CleanUp()
	if err != nil {
		log.Printf("Failed to cleanup the VFS cache: %v", err)
	}
}

// cacheMode flushes the VFS and changes the CacheMode and the writeBack time
func (r *Run) cacheMode(cacheMode vfscommon.CacheMode, writeBack time.Duration) {
	if r.skip {
		log.Printf("FUSE not found so skipping cacheMode")
		return
	}
	// Wait for writers to finish
	r.vfs.WaitForWriters(30 * time.Second)
	// Empty and remake the remote
	r.cleanRemote()
	err := r.fremote.Mkdir(context.Background(), "")
	if err != nil {
		log.Fatalf("Failed to open mkdir %q: %v", *fstest.RemoteName, err)
	}
	// Empty the cache
	err = r.vfs.CleanUp()
	if err != nil {
		log.Printf("Failed to cleanup the VFS cache: %v", err)
	}
	// Reset the cache mode
	r.vfs.SetCacheMode(cacheMode)
	r.vfs.Opt.WriteBack = writeBack
	// Flush the directory cache
	r.vfs.FlushDirCache()

}

func (r *Run) skipIfNoFUSE(t *testing.T) {
	if r.skip {
		t.Skip("FUSE not found so skipping test")
	}
}

func (r *Run) skipIfVFS(t *testing.T) {
	if r.useVFS {
		t.Skip("Not running under VFS")
	}
}

// Finalise cleans the remote and unmounts
func (r *Run) Finalise() {
	r.umount()
	r.cleanRemote()
	if r.useVFS {
		// FIXME
	} else {
		err := os.RemoveAll(r.mountPath)
		if err != nil {
			log.Printf("Failed to clean mountPath %q: %v", r.mountPath, err)
		}
	}
}

// path returns an OS local path for filepath
func (r *Run) path(filePath string) string {
	if r.useVFS {
		return filePath
	}
	// return windows drive letter root as E:\
	if filePath == "" && runtime.GOOS == "windows" {
		return run.mountPath + `\`
	}
	return filepath.Join(run.mountPath, filepath.FromSlash(filePath))
}

type dirMap map[string]struct{}

// Create a dirMap from a string
func newDirMap(dirString string) (dm dirMap) {
	dm = make(dirMap)
	for _, entry := range strings.Split(dirString, "|") {
		if entry != "" {
			dm[entry] = struct{}{}
		}
	}
	return dm
}

// Returns a dirmap with only the files in
func (dm dirMap) filesOnly() dirMap {
	newDm := make(dirMap)
	for name := range dm {
		if !strings.HasSuffix(name, "/") {
			newDm[name] = struct{}{}
		}
	}
	return newDm
}

// reads the local tree into dir
func (r *Run) readLocal(t *testing.T, dir dirMap, filePath string) {
	realPath := r.path(filePath)
	files, err := r.os.ReadDir(realPath)
	require.NoError(t, err)
	for _, fi := range files {
		name := path.Join(filePath, fi.Name())
		if fi.IsDir() {
			dir[name+"/"] = struct{}{}
			r.readLocal(t, dir, name)
			assert.Equal(t, run.vfs.Opt.DirPerms&os.ModePerm, fi.Mode().Perm())
		} else {
			dir[fmt.Sprintf("%s %d", name, fi.Size())] = struct{}{}
			assert.Equal(t, run.vfs.Opt.FilePerms&os.ModePerm, fi.Mode().Perm())
		}
	}
}

// reads the remote tree into dir
func (r *Run) readRemote(t *testing.T, dir dirMap, filepath string) {
	objs, dirs, err := walk.GetAll(context.Background(), r.fremote, filepath, true, 1)
	if err == fs.ErrorDirNotFound {
		return
	}
	require.NoError(t, err)
	for _, obj := range objs {
		dir[fmt.Sprintf("%s %d", obj.Remote(), obj.Size())] = struct{}{}
	}
	for _, d := range dirs {
		name := d.Remote()
		dir[name+"/"] = struct{}{}
		r.readRemote(t, dir, name)
	}
}

// checkDir checks the local and remote against the string passed in
func (r *Run) checkDir(t *testing.T, dirString string) {
	var retries = *fstest.ListRetries
	sleep := time.Second / 5
	var remoteOK, fuseOK bool
	var dm, localDm, remoteDm dirMap
	for i := 1; i <= retries; i++ {
		dm = newDirMap(dirString)
		localDm = make(dirMap)
		r.readLocal(t, localDm, "")
		remoteDm = make(dirMap)
		r.readRemote(t, remoteDm, "")
		// Ignore directories for remote compare
		remoteOK = reflect.DeepEqual(dm.filesOnly(), remoteDm.filesOnly())
		fuseOK = reflect.DeepEqual(dm, localDm)
		if remoteOK && fuseOK {
			return
		}
		sleep *= 2
		t.Logf("Sleeping for %v for list eventual consistency: %d/%d", sleep, i, retries)
		time.Sleep(sleep)
	}
	assert.Equal(t, dm.filesOnly(), remoteDm.filesOnly(), "expected vs remote")
	assert.Equal(t, dm, localDm, "expected vs fuse mount")
}

// wait for any files being written to be released by fuse
func (r *Run) waitForWriters() {
	run.vfs.WaitForWriters(10 * time.Second)
}

// writeFile writes data to a file named by filename.
// If the file does not exist, WriteFile creates it with permissions perm;
// otherwise writeFile truncates it before writing.
// If there is an error writing then writeFile
// deletes it an existing file and tries again.
func writeFile(filename string, data []byte, perm os.FileMode) error {
	f, err := run.os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		err = run.os.Remove(filename)
		if err != nil {
			return err
		}
		f, err = run.os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, perm)
		if err != nil {
			return err
		}
	}
	n, err := f.Write(data)
	if err == nil && n < len(data) {
		err = io.ErrShortWrite
	}
	if err1 := f.Close(); err == nil {
		err = err1
	}
	return err
}

func (r *Run) createFile(t *testing.T, filepath string, contents string) {
	filepath = r.path(filepath)
	err := writeFile(filepath, []byte(contents), 0600)
	require.NoError(t, err)
	r.waitForWriters()
}

func (r *Run) readFile(t *testing.T, filepath string) string {
	filepath = r.path(filepath)
	result, err := run.os.ReadFile(filepath)
	require.NoError(t, err)
	return string(result)
}

func (r *Run) mkdir(t *testing.T, filepath string) {
	filepath = r.path(filepath)
	err := run.os.Mkdir(filepath, 0700)
	require.NoError(t, err)
}

func (r *Run) rm(t *testing.T, filepath string) {
	filepath = r.path(filepath)
	err := run.os.Remove(filepath)
	require.NoError(t, err)

	// Wait for file to disappear from listing
	for i := 0; i < 100; i++ {
		_, err := run.os.Stat(filepath)
		if os.IsNotExist(err) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	assert.Fail(t, "failed to delete file", filepath)
}

func (r *Run) rmdir(t *testing.T, filepath string) {
	filepath = r.path(filepath)
	err := run.os.Remove(filepath)
	require.NoError(t, err)
}

// TestMount checks that the Fs is mounted by seeing if the mountpoint
// is in the mount output
func TestMount(t *testing.T) {
	run.skipIfVFS(t)
	run.skipIfNoFUSE(t)
	if runtime.GOOS == "windows" {
		t.Skip("not running on windows")
	}

	out, err := exec.Command("mount").Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), run.mountPath)
}

// TestRoot checks root directory is present and correct
func TestRoot(t *testing.T) {
	run.skipIfVFS(t)
	run.skipIfNoFUSE(t)

	fi, err := os.Lstat(run.mountPath)
	require.NoError(t, err)
	assert.True(t, fi.IsDir())
	assert.Equal(t, run.vfs.Opt.DirPerms&os.ModePerm, fi.Mode().Perm())
}
