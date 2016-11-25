// Test GoogleCloudStorage filesystem interface
//
// Automatically generated - DO NOT EDIT
// Regenerate with: make gen_tests
package googlecloudstorage_test

import (
	"testing"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest/fstests"
	"github.com/ncw/rclone/googlecloudstorage"
)

func TestSetup(t *testing.T) {
	fstests.NilObject = fs.Object((*googlecloudstorage.Object)(nil))
	fstests.RemoteName = "TestGoogleCloudStorage:"
}

// Generic tests for the Fs
func TestInit(t *testing.T)                { fstests.TestInit(t) }
func TestFsString(t *testing.T)            { fstests.TestFsString(t) }
func TestFsRmdirEmpty(t *testing.T)        { fstests.TestFsRmdirEmpty(t) }
func TestFsRmdirNotFound(t *testing.T)     { fstests.TestFsRmdirNotFound(t) }
func TestFsMkdir(t *testing.T)             { fstests.TestFsMkdir(t) }
func TestFsMkdirRmdirSubdir(t *testing.T)  { fstests.TestFsMkdirRmdirSubdir(t) }
func TestFsListEmpty(t *testing.T)         { fstests.TestFsListEmpty(t) }
func TestFsListDirEmpty(t *testing.T)      { fstests.TestFsListDirEmpty(t) }
func TestFsNewObjectNotFound(t *testing.T) { fstests.TestFsNewObjectNotFound(t) }
func TestFsPutFile1(t *testing.T)          { fstests.TestFsPutFile1(t) }
func TestFsPutError(t *testing.T)          { fstests.TestFsPutError(t) }
func TestFsPutFile2(t *testing.T)          { fstests.TestFsPutFile2(t) }
func TestFsUpdateFile1(t *testing.T)       { fstests.TestFsUpdateFile1(t) }
func TestFsListDirFile2(t *testing.T)      { fstests.TestFsListDirFile2(t) }
func TestFsListDirRoot(t *testing.T)       { fstests.TestFsListDirRoot(t) }
func TestFsListSubdir(t *testing.T)        { fstests.TestFsListSubdir(t) }
func TestFsListLevel2(t *testing.T)        { fstests.TestFsListLevel2(t) }
func TestFsListFile1(t *testing.T)         { fstests.TestFsListFile1(t) }
func TestFsNewObject(t *testing.T)         { fstests.TestFsNewObject(t) }
func TestFsListFile1and2(t *testing.T)     { fstests.TestFsListFile1and2(t) }
func TestFsCopy(t *testing.T)              { fstests.TestFsCopy(t) }
func TestFsMove(t *testing.T)              { fstests.TestFsMove(t) }
func TestFsDirMove(t *testing.T)           { fstests.TestFsDirMove(t) }
func TestFsRmdirFull(t *testing.T)         { fstests.TestFsRmdirFull(t) }
func TestFsPrecision(t *testing.T)         { fstests.TestFsPrecision(t) }
func TestObjectString(t *testing.T)        { fstests.TestObjectString(t) }
func TestObjectFs(t *testing.T)            { fstests.TestObjectFs(t) }
func TestObjectRemote(t *testing.T)        { fstests.TestObjectRemote(t) }
func TestObjectHashes(t *testing.T)        { fstests.TestObjectHashes(t) }
func TestObjectModTime(t *testing.T)       { fstests.TestObjectModTime(t) }
func TestObjectMimeType(t *testing.T)      { fstests.TestObjectMimeType(t) }
func TestObjectSetModTime(t *testing.T)    { fstests.TestObjectSetModTime(t) }
func TestObjectSize(t *testing.T)          { fstests.TestObjectSize(t) }
func TestObjectOpen(t *testing.T)          { fstests.TestObjectOpen(t) }
func TestObjectOpenSeek(t *testing.T)      { fstests.TestObjectOpenSeek(t) }
func TestObjectUpdate(t *testing.T)        { fstests.TestObjectUpdate(t) }
func TestObjectStorable(t *testing.T)      { fstests.TestObjectStorable(t) }
func TestFsIsFile(t *testing.T)            { fstests.TestFsIsFile(t) }
func TestFsIsFileNotFound(t *testing.T)    { fstests.TestFsIsFileNotFound(t) }
func TestObjectRemove(t *testing.T)        { fstests.TestObjectRemove(t) }
func TestObjectPurge(t *testing.T)         { fstests.TestObjectPurge(t) }
func TestFinalise(t *testing.T)            { fstests.TestFinalise(t) }
