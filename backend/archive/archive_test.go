//go:build !plan9

// Test Archive filesystem interface
package archive_test

import (
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/backend/memory"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
)

var (
	unimplementableFsMethods = []string{"ListR", "ListP", "MkdirMetadata", "DirSetModTime"}
	// In these tests we receive objects from the underlying remote which don't implement these methods
	unimplementableObjectMethods = []string{"GetTier", "ID", "Metadata", "MimeType", "SetTier", "UnWrap", "SetMetadata"}
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
	remote := t.TempDir()
	name := "TestArchiveLocal"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "archive"},
			{Name: name, Key: "remote", Value: remote},
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
	remote := ":memory:"
	name := "TestArchiveMemory"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "archive"},
			{Name: name, Key: "remote", Value: remote},
		},
		QuickTestOK:                  true,
		UnimplementableFsMethods:     unimplementableFsMethods,
		UnimplementableObjectMethods: unimplementableObjectMethods,
	})
}
