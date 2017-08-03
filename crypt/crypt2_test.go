// Test Crypt filesystem interface
//
// Automatically generated - DO NOT EDIT
// Regenerate with: make gen_tests
package crypt_test

import (
	"testing"

	"github.com/ncw/rclone/crypt"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest/fstests"
	_ "github.com/ncw/rclone/local"
)

func TestSetup2(t *testing.T) {
	fstests.NilObject = fs.Object((*crypt.Object)(nil))
	fstests.RemoteName = "TestCrypt2:"
}

// Generic tests for the Fs
func TestInit2(t *testing.T)                   { fstests.TestInit(t) }
func TestFsString2(t *testing.T)               { fstests.TestFsString(t) }
func TestFsName2(t *testing.T)                 { fstests.TestFsName(t) }
func TestFsRoot2(t *testing.T)                 { fstests.TestFsRoot(t) }
func TestFsRmdirEmpty2(t *testing.T)           { fstests.TestFsRmdirEmpty(t) }
func TestFsRmdirNotFound2(t *testing.T)        { fstests.TestFsRmdirNotFound(t) }
func TestFsMkdir2(t *testing.T)                { fstests.TestFsMkdir(t) }
func TestFsMkdirRmdirSubdir2(t *testing.T)     { fstests.TestFsMkdirRmdirSubdir(t) }
func TestFsListEmpty2(t *testing.T)            { fstests.TestFsListEmpty(t) }
func TestFsListDirEmpty2(t *testing.T)         { fstests.TestFsListDirEmpty(t) }
func TestFsListRDirEmpty2(t *testing.T)        { fstests.TestFsListRDirEmpty(t) }
func TestFsNewObjectNotFound2(t *testing.T)    { fstests.TestFsNewObjectNotFound(t) }
func TestFsPutFile12(t *testing.T)             { fstests.TestFsPutFile1(t) }
func TestFsPutError2(t *testing.T)             { fstests.TestFsPutError(t) }
func TestFsPutFile22(t *testing.T)             { fstests.TestFsPutFile2(t) }
func TestFsUpdateFile12(t *testing.T)          { fstests.TestFsUpdateFile1(t) }
func TestFsListDirFile22(t *testing.T)         { fstests.TestFsListDirFile2(t) }
func TestFsListRDirFile22(t *testing.T)        { fstests.TestFsListRDirFile2(t) }
func TestFsListDirRoot2(t *testing.T)          { fstests.TestFsListDirRoot(t) }
func TestFsListRDirRoot2(t *testing.T)         { fstests.TestFsListRDirRoot(t) }
func TestFsListSubdir2(t *testing.T)           { fstests.TestFsListSubdir(t) }
func TestFsListRSubdir2(t *testing.T)          { fstests.TestFsListRSubdir(t) }
func TestFsListLevel22(t *testing.T)           { fstests.TestFsListLevel2(t) }
func TestFsListRLevel22(t *testing.T)          { fstests.TestFsListRLevel2(t) }
func TestFsListFile12(t *testing.T)            { fstests.TestFsListFile1(t) }
func TestFsNewObject2(t *testing.T)            { fstests.TestFsNewObject(t) }
func TestFsListFile1and22(t *testing.T)        { fstests.TestFsListFile1and2(t) }
func TestFsNewObjectDir2(t *testing.T)         { fstests.TestFsNewObjectDir(t) }
func TestFsCopy2(t *testing.T)                 { fstests.TestFsCopy(t) }
func TestFsMove2(t *testing.T)                 { fstests.TestFsMove(t) }
func TestFsDirMove2(t *testing.T)              { fstests.TestFsDirMove(t) }
func TestFsRmdirFull2(t *testing.T)            { fstests.TestFsRmdirFull(t) }
func TestFsPrecision2(t *testing.T)            { fstests.TestFsPrecision(t) }
func TestFsDirChangeNotify2(t *testing.T)      { fstests.TestFsDirChangeNotify(t) }
func TestObjectString2(t *testing.T)           { fstests.TestObjectString(t) }
func TestObjectFs2(t *testing.T)               { fstests.TestObjectFs(t) }
func TestObjectRemote2(t *testing.T)           { fstests.TestObjectRemote(t) }
func TestObjectHashes2(t *testing.T)           { fstests.TestObjectHashes(t) }
func TestObjectModTime2(t *testing.T)          { fstests.TestObjectModTime(t) }
func TestObjectMimeType2(t *testing.T)         { fstests.TestObjectMimeType(t) }
func TestObjectSetModTime2(t *testing.T)       { fstests.TestObjectSetModTime(t) }
func TestObjectSize2(t *testing.T)             { fstests.TestObjectSize(t) }
func TestObjectOpen2(t *testing.T)             { fstests.TestObjectOpen(t) }
func TestObjectOpenSeek2(t *testing.T)         { fstests.TestObjectOpenSeek(t) }
func TestObjectPartialRead2(t *testing.T)      { fstests.TestObjectPartialRead(t) }
func TestObjectUpdate2(t *testing.T)           { fstests.TestObjectUpdate(t) }
func TestObjectStorable2(t *testing.T)         { fstests.TestObjectStorable(t) }
func TestFsIsFile2(t *testing.T)               { fstests.TestFsIsFile(t) }
func TestFsIsFileNotFound2(t *testing.T)       { fstests.TestFsIsFileNotFound(t) }
func TestObjectRemove2(t *testing.T)           { fstests.TestObjectRemove(t) }
func TestFsPutUnknownLengthFile2(t *testing.T) { fstests.TestFsPutUnknownLengthFile(t) }
func TestObjectPurge2(t *testing.T)            { fstests.TestObjectPurge(t) }
func TestFinalise2(t *testing.T)               { fstests.TestFinalise(t) }
