// Test Cryptomator filesystem interface
package cryptomator_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rclone/rclone/backend/cryptomator"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"

	_ "github.com/rclone/rclone/backend/alias"
	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/backend/s3"
)

var (
	UnimplementableFsMethods        = []string{"Purge", "ChangeNotify", "MergeDirs", "ListR", "OpenWriterAt", "MkdirMetadata", "DirSetModTime", "OpenChunkWriter"}
	UnimplementableObjectMethods    = []string{"MimeType"}
	UnimplementableDirectoryMethods = []string{"Metadata", "SetMetadata", "SetModTime"}
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	if *fstest.RemoteName == "" {
		t.Skip("Skipping as -remote not set")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName:                      *fstest.RemoteName,
		NilObject:                       (*cryptomator.Object)(nil),
		TiersToTest:                     []string{"REDUCED_REDUNDANCY", "STANDARD"},
		UnimplementableFsMethods:        UnimplementableFsMethods,
		UnimplementableObjectMethods:    UnimplementableObjectMethods,
		UnimplementableDirectoryMethods: UnimplementableDirectoryMethods,
	})
}

// TestStandard runs integration tests against the remote
func TestStandard(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	tempdir := filepath.Join(os.TempDir(), "rclone-cryptomator-test-standard")
	name := "TestCryptomator"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		NilObject:  (*cryptomator.Object)(nil),
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "cryptomator"},
			{Name: name, Key: "remote", Value: tempdir},
			{Name: name, Key: "password", Value: obscure.MustObscure("potato")},
		},
		QuickTestOK:                     true,
		UnimplementableFsMethods:        UnimplementableFsMethods,
		UnimplementableObjectMethods:    UnimplementableObjectMethods,
		UnimplementableDirectoryMethods: UnimplementableDirectoryMethods,
	})
}
