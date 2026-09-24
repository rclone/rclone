// Test the Chunker filesystem interface
package chunker_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/rclone/rclone/backend/all" // for integration tests
	"github.com/rclone/rclone/backend/chunker"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			"ListP",
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

func TestNewFsUnionSingleFile(t *testing.T) {
	fstest.Initialise()
	for _, meta := range []string{"simplejson", "none"} {
		for _, root := range []struct{ base, parent string }{{"", ""}, {"", "subdir"}, {"base", ""}, {"base", "subdir"}} {
			base, parent := root.base, root.parent
			t.Run(meta+"/"+path.Join(base, parent), func(t *testing.T) {
				ctx := context.Background()
				dirs := []string{t.TempDir(), t.TempDir(), t.TempDir()}
				const contents = "abcdefghijklmnop"
				const name = "file.txt"
				remote := path.Join(parent, name)
				upload, err := fs.NewFs(ctx, fmt.Sprintf(":chunker,remote=%q,chunk_size=3b,hash_type=none,meta_format=%s:", filepath.Join(dirs[0], base), meta))
				require.NoError(t, err)
				item := fstest.NewItem(remote, contents, fstest.Time("2001-02-03T04:05:06Z"))
				fstests.PutTestContents(ctx, t, upload, &item, contents, true)

				// Leave the metadata on its own upstream and split the chunks
				// between two others, independently of the create policy.
				chunks, err := filepath.Glob(filepath.Join(dirs[0], base, parent, name+".rclone_chunk.*"))
				require.NoError(t, err)
				require.Len(t, chunks, 6)
				for i, chunk := range chunks {
					dst := filepath.Join(dirs[1+i%2], base, parent, filepath.Base(chunk))
					require.NoError(t, os.MkdirAll(filepath.Dir(dst), 0700))
					require.NoError(t, os.Rename(chunk, dst))
				}

				t.Setenv("RCLONE_CONFIG_TESTCHUNKERUNION_TYPE", "union")
				t.Setenv("RCLONE_CONFIG_TESTCHUNKERUNION_UPSTREAMS", strings.Join(dirs, " "))
				fsString := fmt.Sprintf(":chunker,remote=%q,chunk_size=3b,hash_type=none,meta_format=%s:", "TestChunkerUnion:"+base, meta)
				// A cached directory Fs would hide single-file upstream selection.
				cache.Clear()
				t.Cleanup(cache.Clear)
				fileFs, err := fs.NewFs(ctx, fsString+remote)
				require.ErrorIs(t, err, fs.ErrorIsFile)
				assert.Equal(t, parent, fileFs.Root())
				obj, err := fileFs.NewObject(ctx, name)
				require.NoError(t, err)
				data, err := operations.ReadFile(ctx, obj)
				require.NoError(t, err)
				assert.Equal(t, contents, string(data))

				again, err := fs.NewFs(ctx, fsString+remote)
				require.ErrorIs(t, err, fs.ErrorIsFile)
				assert.Same(t, fileFs.(*chunker.Fs).UnWrap(), again.(*chunker.Fs).UnWrap())

				entries, err := fileFs.List(ctx, "")
				require.NoError(t, err)
				require.Len(t, entries, 1)
				assert.Equal(t, name, entries[0].Remote())

				dirFs, err := fs.NewFs(ctx, fsString+parent)
				require.NoError(t, err)
				obj, err = dirFs.NewObject(ctx, name)
				require.NoError(t, err)
				data, err = operations.ReadFile(ctx, obj)
				require.NoError(t, err)
				assert.Equal(t, contents, string(data))
			})
		}
	}
}
