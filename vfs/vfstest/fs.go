// Test suite for rclonefs

package vfstest

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/all" // import all the backends
	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfsflags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	waitForWritersDelay = 30 * time.Second // time to wait for existing writers
)

// RunTests runs all the tests against all the VFS cache modes
//
// If useVFS is set then it runs the tests against a VFS rather than a
// mount
//
// If useVFS is not set then it runs the mount in a subprocess in
// order to avoid kernel deadlocks.
func RunTests(t *testing.T, useVFS bool, mountFn mountlib.MountFn) {
	flag.Parse()
	if isSubProcess() {
		startMount(mountFn, useVFS, *runMount)
		return
	}
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
	for _, test := range tests {
		vfsOpt := vfsflags.Opt
		vfsOpt.CacheMode = test.cacheMode
		vfsOpt.WriteBack = test.writeBack
		run = newRun(useVFS, &vfsOpt, mountFn)
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
		run.Finalise()
		if !ok {
			break
		}
	}
}

// Run holds the remotes for a test run
type Run struct {
	os          Oser
	vfsOpt      *vfscommon.Options
	useVFS      bool // set if we are testing a VFS not a mount
	mountPath   string
	fremote     fs.Fs
	fremoteName string
	cleanRemote func()
	skip        bool
	// For controlling the subprocess running the mount
	cmdMu   sync.Mutex
	cmd     *exec.Cmd
	in      io.ReadCloser
	out     io.WriteCloser
	scanner *bufio.Scanner
}

// run holds the master Run data
var run *Run

// newRun initialise the remote mount for testing and returns a run
// object.
//
// r.fremote is an empty remote Fs
//
// Finalise() will tidy them away when done.
func newRun(useVFS bool, vfsOpt *vfscommon.Options, mountFn mountlib.MountFn) *Run {
	r := &Run{
		useVFS: useVFS,
		vfsOpt: vfsOpt,
	}
	r.vfsOpt.Init()
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

	r.startMountSubProcess()
	return r
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
	if !r.useVFS {
		r.sendMountCommand("exit")
		_, err := r.cmd.Process.Wait()
		if err != nil {
			log.Fatalf("mount sub process failed: %v", err)
		}
	}
	r.cleanRemote()
	if !r.useVFS {
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
		return r.mountPath + `\`
	}
	return filepath.Join(r.mountPath, filepath.FromSlash(filePath))
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
			assert.Equal(t, r.vfsOpt.DirPerms&os.ModePerm, fi.Mode().Perm())
		} else {
			dir[fmt.Sprintf("%s %d", name, fi.Size())] = struct{}{}
			assert.Equal(t, r.vfsOpt.FilePerms&os.ModePerm, fi.Mode().Perm())
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
	result, err := r.os.ReadFile(filepath)
	require.NoError(t, err)
	return string(result)
}

func (r *Run) mkdir(t *testing.T, filepath string) {
	filepath = r.path(filepath)
	err := r.os.Mkdir(filepath, 0700)
	require.NoError(t, err)
}

func (r *Run) rm(t *testing.T, filepath string) {
	filepath = r.path(filepath)
	err := r.os.Remove(filepath)
	require.NoError(t, err)

	// Wait for file to disappear from listing
	for i := 0; i < 100; i++ {
		_, err := r.os.Stat(filepath)
		if os.IsNotExist(err) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	assert.Fail(t, "failed to delete file", filepath)
}

func (r *Run) rmdir(t *testing.T, filepath string) {
	filepath = r.path(filepath)
	err := r.os.Remove(filepath)
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
	assert.Equal(t, run.vfsOpt.DirPerms&os.ModePerm, fi.Mode().Perm())
}
