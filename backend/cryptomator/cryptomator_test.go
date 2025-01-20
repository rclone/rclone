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
	UnimplementableFsMethods = []string{
		// TODO: implement these:
		// It's not possible to complete this in one call, but Purge could still be implemented more efficiently than the fallback by
		// recursing and deleting a full directory at a time (instead of each file individually.)
		"Purge",
		// MergeDirs could be implemented by merging the underlying directories, while taking care to leave the dirid.c9r alone.
		"MergeDirs",
		// OpenWriterAt could be implemented by a strategy such as: to write to a chunk, read and decrypt it, handle all writes, then reencrypt and upload.
		"OpenWriterAt",
		// OpenChunkWriter could be implemented, at least if the backend's chunk size is a multiple of Cryptomator's chunk size.
		"OpenChunkWriter",

		// Having ListR on the backend doesn't help at all for implementing it in Cryptomator.
		"ListR",
		// ChangeNotify would have to undo the dir to dir ID conversion, which is lossy. It can be done, but not without scanning and caching the full hierarchy.
		"ChangeNotify",
	}
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	if *fstest.RemoteName == "" {
		t.Skip("Skipping as -remote not set")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName:               *fstest.RemoteName,
		NilObject:                (*cryptomator.DecryptingObject)(nil),
		TiersToTest:              []string{"REDUCED_REDUNDANCY", "STANDARD"},
		UnimplementableFsMethods: UnimplementableFsMethods,
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
		NilObject:  (*cryptomator.DecryptingObject)(nil),
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "cryptomator"},
			{Name: name, Key: "remote", Value: tempdir},
			{Name: name, Key: "password", Value: obscure.MustObscure("potato")},
		},
		QuickTestOK:              true,
		UnimplementableFsMethods: UnimplementableFsMethods,
	})
}
