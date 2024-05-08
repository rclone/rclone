// Test the Chunker filesystem interface
package chunker_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/rclone/rclone/backend/all" // for integration tests
	"github.com/rclone/rclone/backend/chunker"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
)

// Command line flags
var (
	// Invalid characters are not supported by some remotes, e.g. Mailru.
	// We enable testing with invalid characters when -remote is not set, so
	// chunker overlays a local directory, but invalid characters are disabled
	// by default when -remote is set, e.g. when test_all runs backend tests.
	// You can still test with invalid characters using the below flag.
	UseBadChars = flag.Bool("bad-chars", false, "Set to test bad characters in file names when -remote is set")
)

// TestIntegration runs integration tests against a concrete remote
// set by the -remote flag. If the flag is not set, it creates a
// dynamic chunker overlay wrapping a local temporary directory.
func TestIntegration(t *testing.T) {
	opt := fstests.Opt{
		RemoteName:               *fstest.RemoteName,
		NilObject:                (*chunker.Object)(nil),
		SkipBadWindowsCharacters: !*UseBadChars,
		UnimplementableObjectMethods: []string{
			"MimeType",
			"GetTier",
			"SetTier",
			"Metadata",
			"SetMetadata",
		},
		UnimplementableFsMethods: []string{
			"PublicLink",
			"OpenWriterAt",
			"OpenChunkWriter",
			"MergeDirs",
			"DirCacheFlush",
			"UserInfo",
			"Disconnect",
		},
	}
	if *fstest.RemoteName == "" {
		name := "TestChunker"
		opt.RemoteName = name + ":"
		tempDir := filepath.Join(os.TempDir(), "rclone-chunker-test-standard")
		opt.ExtraConfig = []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "chunker"},
			{Name: name, Key: "remote", Value: tempDir},
		}
		opt.QuickTestOK = true
	}
	fstests.Run(t, &opt)
}
