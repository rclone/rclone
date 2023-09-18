// Test Combine filesystem interface
package combine_test

import (
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/backend/memory"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
)

var (
	unimplementableFsMethods     = []string{"UnWrap", "WrapFs", "SetWrapper", "UserInfo", "Disconnect", "OpenChunkWriter"}
	unimplementableObjectMethods = []string{}
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	if *fstest.RemoteName == "" {
		t.Skip("Skipping as -remote not set")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName:                   *fstest.RemoteName,
		UnimplementableFsMethods:     unimplementableFsMethods,
		UnimplementableObjectMethods: unimplementableObjectMethods,
	})
}

func TestLocal(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	dirs := MakeTestDirs(t, 3)
	upstreams := "dir1=" + dirs[0] + " dir2=" + dirs[1] + " dir3=" + dirs[2]
	name := "TestCombineLocal"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":dir1",
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "combine"},
			{Name: name, Key: "upstreams", Value: upstreams},
		},
		QuickTestOK:                  true,
		UnimplementableFsMethods:     unimplementableFsMethods,
		UnimplementableObjectMethods: unimplementableObjectMethods,
	})
}

func TestMemory(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	upstreams := "dir1=:memory:dir1 dir2=:memory:dir2 dir3=:memory:dir3"
	name := "TestCombineMemory"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":dir1",
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "combine"},
			{Name: name, Key: "upstreams", Value: upstreams},
		},
		QuickTestOK:                  true,
		UnimplementableFsMethods:     unimplementableFsMethods,
		UnimplementableObjectMethods: unimplementableObjectMethods,
	})
}

func TestMixed(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	dirs := MakeTestDirs(t, 2)
	upstreams := "dir1=" + dirs[0] + " dir2=" + dirs[1] + " dir3=:memory:dir3"
	name := "TestCombineMixed"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":dir1",
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "combine"},
			{Name: name, Key: "upstreams", Value: upstreams},
		},
		UnimplementableFsMethods:     unimplementableFsMethods,
		UnimplementableObjectMethods: unimplementableObjectMethods,
	})
}

// MakeTestDirs makes directories in /tmp for testing
func MakeTestDirs(t *testing.T, n int) (dirs []string) {
	for i := 1; i <= n; i++ {
		dir := t.TempDir()
		dirs = append(dirs, dir)
	}
	return dirs
}
