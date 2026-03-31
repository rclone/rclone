package rs_test

import (
	"strings"
	"testing"

	_ "github.com/rclone/rclone/backend/all"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
)

// Optional Fs methods not implemented by rs; rs implements PutStream.
var integrationUnimplementableFsMethods = []string{
	"UnWrap", "WrapFs", "SetWrapper", "UserInfo", "Disconnect", "PublicLink",
	"PutUnchecked", "MergeDirs", "OpenWriterAt", "OpenChunkWriter", "ListP",
	"ChangeNotify", "DirCacheFlush",
}

var integrationUnimplementableObjectMethods = []string{}

// TestIntegration runs the fstest suite when -remote is set (e.g. CI test_all).
func TestIntegration(t *testing.T) {
	if *fstest.RemoteName == "" {
		t.Skip("Skipping as -remote not set")
	}
	if strings.HasPrefix(*fstest.RemoteName, "TestRsMinio") {
		t.Skip("Full integration for MinIO rs remote is not configured yet")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName:                   *fstest.RemoteName,
		UnimplementableFsMethods:     integrationUnimplementableFsMethods,
		UnimplementableObjectMethods: integrationUnimplementableObjectMethods,
	})
}

// TestStandard runs fstest against TestRsLocal from fstest/testserver/init.d/TestRsLocal.
func TestStandard(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName:                   "TestRsLocal:",
		UnimplementableFsMethods:     integrationUnimplementableFsMethods,
		UnimplementableObjectMethods: integrationUnimplementableObjectMethods,
		QuickTestOK:                  true,
	})
}
