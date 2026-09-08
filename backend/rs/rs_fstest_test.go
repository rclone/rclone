package rs_test

import (
	"strings"
	"testing"

	_ "github.com/rclone/rclone/backend/all"
	rs "github.com/rclone/rclone/backend/rs"
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
		NilObject:                    (*rs.Object)(nil),
		UnimplementableFsMethods:     integrationUnimplementableFsMethods,
		UnimplementableObjectMethods: integrationUnimplementableObjectMethods,
	})
}

// TestStandard runs the fstest suite against an rs remote over four temporary
// local shard directories (k=3, m=1), configured in-process like the chunker,
// crypt and union standard tests. No external test server is needed, so it
// runs on all platforms including Windows.
//
// The remote name must NOT match a script in fstest/testserver/init.d
// (i.e. not TestRsLocal), otherwise the fstest harness executes that
// script, which fails on Windows.
func TestStandard(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	dirs := make([]string, 4)
	for i := range dirs {
		dirs[i] = t.TempDir()
	}
	name := "TestRsStandard"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		NilObject:  (*rs.Object)(nil),
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "rs"},
			{Name: name, Key: "remotes", Value: strings.Join(dirs, ",")},
			{Name: name, Key: "data_shards", Value: "3"},
			{Name: name, Key: "parity_shards", Value: "1"},
			{Name: name, Key: "use_spooling", Value: "false"},
			{Name: name, Key: "rollback", Value: "true"},
		},
		UnimplementableFsMethods:     integrationUnimplementableFsMethods,
		UnimplementableObjectMethods: integrationUnimplementableObjectMethods,
		QuickTestOK:                  true,
	})
}
