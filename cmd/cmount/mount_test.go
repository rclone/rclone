// +build cmount
// +build cgo
// +build linux darwin freebsd windows

package cmount

import (
	"runtime"
	"testing"

	"github.com/ncw/rclone/cmd/mountlib/mounttest"
)

func notWin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("not running on windows")
	}
}

func TestMain(m *testing.M)                       { mounttest.TestMain(m, mount, dirPerms, filePerms) }
func TestDirLs(t *testing.T)                      { mounttest.TestDirLs(t) }
func TestDirCreateAndRemoveDir(t *testing.T)      { notWin(t); mounttest.TestDirCreateAndRemoveDir(t) }
func TestDirCreateAndRemoveFile(t *testing.T)     { notWin(t); mounttest.TestDirCreateAndRemoveFile(t) }
func TestDirRenameFile(t *testing.T)              { notWin(t); mounttest.TestDirRenameFile(t) }
func TestDirRenameEmptyDir(t *testing.T)          { notWin(t); mounttest.TestDirRenameEmptyDir(t) }
func TestDirRenameFullDir(t *testing.T)           { notWin(t); mounttest.TestDirRenameFullDir(t) }
func TestDirModTime(t *testing.T)                 { notWin(t); mounttest.TestDirModTime(t) }
func TestDirCacheFlush(t *testing.T)              { notWin(t); mounttest.TestDirCacheFlush(t) }
func TestDirCacheFlushOnDirRename(t *testing.T)   { notWin(t); mounttest.TestDirCacheFlushOnDirRename(t) }
func TestFileModTime(t *testing.T)                { notWin(t); mounttest.TestFileModTime(t) }
func TestFileModTimeWithOpenWriters(t *testing.T) {} // FIXME mounttest.TestFileModTimeWithOpenWriters(t)
func TestMount(t *testing.T)                      { notWin(t); mounttest.TestMount(t) }
func TestRoot(t *testing.T)                       { notWin(t); mounttest.TestRoot(t) }
func TestReadByByte(t *testing.T)                 { notWin(t); mounttest.TestReadByByte(t) }
func TestReadChecksum(t *testing.T)               { notWin(t); mounttest.TestReadChecksum(t) }
func TestReadFileDoubleClose(t *testing.T)        { notWin(t); mounttest.TestReadFileDoubleClose(t) }
func TestReadSeek(t *testing.T)                   { notWin(t); mounttest.TestReadSeek(t) }
func TestWriteFileNoWrite(t *testing.T)           { notWin(t); mounttest.TestWriteFileNoWrite(t) }
func TestWriteFileWrite(t *testing.T)             { notWin(t); mounttest.TestWriteFileWrite(t) }
func TestWriteFileOverwrite(t *testing.T)         { notWin(t); mounttest.TestWriteFileOverwrite(t) }
func TestWriteFileDoubleClose(t *testing.T)       { notWin(t); mounttest.TestWriteFileDoubleClose(t) }
func TestWriteFileFsync(t *testing.T)             { notWin(t); mounttest.TestWriteFileFsync(t) }
