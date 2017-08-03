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

func TestSetup3(t *testing.T) {
	fstests.NilObject = fs.Object((*crypt.Object)(nil))
	fstests.RemoteName = "TestCrypt3:"
}

// Generic tests for the Fs
func TestInit3(t *testing.T)                   { fstests.TestInit(t) }
func TestFsString3(t *testing.T)               { fstests.TestFsString(t) }
func TestFsName3(t *testing.T)                 { fstests.TestFsName(t) }
func TestFsRoot3(t *testing.T)                 { fstests.TestFsRoot(t) }
func TestFsRmdirEmpty3(t *testing.T)           { fstests.TestFsRmdirEmpty(t) }
func TestFsRmdirNotFound3(t *testing.T)        { fstests.TestFsRmdirNotFound(t) }
func TestFsMkdir3(t *testing.T)                { fstests.TestFsMkdir(t) }
func TestFsMkdirRmdirSubdir3(t *testing.T)     { fstests.TestFsMkdirRmdirSubdir(t) }
func TestFsListEmpty3(t *testing.T)            { fstests.TestFsListEmpty(t) }
func TestFsListDirEmpty3(t *testing.T)         { fstests.TestFsListDirEmpty(t) }
func TestFsListRDirEmpty3(t *testing.T)        { fstests.TestFsListRDirEmpty(t) }
func TestFsNewObjectNotFound3(t *testing.T)    { fstests.TestFsNewObjectNotFound(t) }
func TestFsPutFile13(t *testing.T)             { fstests.TestFsPutFile1(t) }
func TestFsPutError3(t *testing.T)             { fstests.TestFsPutError(t) }
func TestFsPutFile23(t *testing.T)             { fstests.TestFsPutFile2(t) }
func TestFsUpdateFile13(t *testing.T)          { fstests.TestFsUpdateFile1(t) }
func TestFsListDirFile23(t *testing.T)         { fstests.TestFsListDirFile2(t) }
func TestFsListRDirFile23(t *testing.T)        { fstests.TestFsListRDirFile2(t) }
func TestFsListDirRoot3(t *testing.T)          { fstests.TestFsListDirRoot(t) }
func TestFsListRDirRoot3(t *testing.T)         { fstests.TestFsListRDirRoot(t) }
func TestFsListSubdir3(t *testing.T)           { fstests.TestFsListSubdir(t) }
func TestFsListRSubdir3(t *testing.T)          { fstests.TestFsListRSubdir(t) }
func TestFsListLevel23(t *testing.T)           { fstests.TestFsListLevel2(t) }
func TestFsListRLevel23(t *testing.T)          { fstests.TestFsListRLevel2(t) }
func TestFsListFile13(t *testing.T)            { fstests.TestFsListFile1(t) }
func TestFsNewObject3(t *testing.T)            { fstests.TestFsNewObject(t) }
func TestFsListFile1and23(t *testing.T)        { fstests.TestFsListFile1and2(t) }
func TestFsNewObjectDir3(t *testing.T)         { fstests.TestFsNewObjectDir(t) }
func TestFsCopy3(t *testing.T)                 { fstests.TestFsCopy(t) }
func TestFsMove3(t *testing.T)                 { fstests.TestFsMove(t) }
func TestFsDirMove3(t *testing.T)              { fstests.TestFsDirMove(t) }
func TestFsRmdirFull3(t *testing.T)            { fstests.TestFsRmdirFull(t) }
func TestFsPrecision3(t *testing.T)            { fstests.TestFsPrecision(t) }
func TestFsDirChangeNotify3(t *testing.T)      { fstests.TestFsDirChangeNotify(t) }
func TestObjectString3(t *testing.T)           { fstests.TestObjectString(t) }
func TestObjectFs3(t *testing.T)               { fstests.TestObjectFs(t) }
func TestObjectRemote3(t *testing.T)           { fstests.TestObjectRemote(t) }
func TestObjectHashes3(t *testing.T)           { fstests.TestObjectHashes(t) }
func TestObjectModTime3(t *testing.T)          { fstests.TestObjectModTime(t) }
func TestObjectMimeType3(t *testing.T)         { fstests.TestObjectMimeType(t) }
func TestObjectSetModTime3(t *testing.T)       { fstests.TestObjectSetModTime(t) }
func TestObjectSize3(t *testing.T)             { fstests.TestObjectSize(t) }
func TestObjectOpen3(t *testing.T)             { fstests.TestObjectOpen(t) }
func TestObjectOpenSeek3(t *testing.T)         { fstests.TestObjectOpenSeek(t) }
func TestObjectPartialRead3(t *testing.T)      { fstests.TestObjectPartialRead(t) }
func TestObjectUpdate3(t *testing.T)           { fstests.TestObjectUpdate(t) }
func TestObjectStorable3(t *testing.T)         { fstests.TestObjectStorable(t) }
func TestFsIsFile3(t *testing.T)               { fstests.TestFsIsFile(t) }
func TestFsIsFileNotFound3(t *testing.T)       { fstests.TestFsIsFileNotFound(t) }
func TestObjectRemove3(t *testing.T)           { fstests.TestObjectRemove(t) }
func TestFsPutUnknownLengthFile3(t *testing.T) { fstests.TestFsPutUnknownLengthFile(t) }
func TestObjectPurge3(t *testing.T)            { fstests.TestObjectPurge(t) }
func TestFinalise3(t *testing.T)               { fstests.TestFinalise(t) }
