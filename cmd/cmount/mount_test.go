// +build cgo
// +build linux darwin freebsd windows

package cmount

import (
	"testing"

	"github.com/ncw/rclone/cmd/mountlib/mounttest"
)

func TestMain(m *testing.M)                       { mounttest.TestMain(m, mount, dirPerms, filePerms) }
func TestDirLs(t *testing.T)                      { mounttest.TestDirLs(t) }
func TestDirCreateAndRemoveDir(t *testing.T)      { mounttest.TestDirCreateAndRemoveDir(t) }
func TestDirCreateAndRemoveFile(t *testing.T)     { mounttest.TestDirCreateAndRemoveFile(t) }
func TestDirRenameFile(t *testing.T)              { mounttest.TestDirRenameFile(t) }
func TestDirRenameEmptyDir(t *testing.T)          { mounttest.TestDirRenameEmptyDir(t) }
func TestDirRenameFullDir(t *testing.T)           { mounttest.TestDirRenameFullDir(t) }
func TestDirModTime(t *testing.T)                 { mounttest.TestDirModTime(t) }
func TestDirCacheFlush(t *testing.T)              { mounttest.TestDirCacheFlush(t) }
func TestDirCacheFlushOnDirRename(t *testing.T)   { mounttest.TestDirCacheFlushOnDirRename(t) }
func TestFileModTime(t *testing.T)                { mounttest.TestFileModTime(t) }
func TestFileModTimeWithOpenWriters(t *testing.T) {} // FIXME mounttest.TestFileModTimeWithOpenWriters(t)
func TestMount(t *testing.T)                      { mounttest.TestMount(t) }
func TestRoot(t *testing.T)                       { mounttest.TestRoot(t) }
func TestReadByByte(t *testing.T)                 { mounttest.TestReadByByte(t) }
func TestReadChecksum(t *testing.T)               { mounttest.TestReadChecksum(t) }
func TestReadFileDoubleClose(t *testing.T)        { mounttest.TestReadFileDoubleClose(t) }
func TestReadSeek(t *testing.T)                   { mounttest.TestReadSeek(t) }
func TestWriteFileNoWrite(t *testing.T)           { mounttest.TestWriteFileNoWrite(t) }
func TestWriteFileWrite(t *testing.T)             { mounttest.TestWriteFileWrite(t) }
func TestWriteFileOverwrite(t *testing.T)         { mounttest.TestWriteFileOverwrite(t) }
func TestWriteFileDoubleClose(t *testing.T)       { mounttest.TestWriteFileDoubleClose(t) }
func TestWriteFileFsync(t *testing.T)             { mounttest.TestWriteFileFsync(t) }
