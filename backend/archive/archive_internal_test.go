//go:build !plan9

package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rclone/rclone/backend/archive/archiver"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FIXME need to test Open with seek

// run - run a shell command
func run(t *testing.T, args ...string) {
	cmd := exec.Command(args[0], args[1:]...)
	fs.Debugf(nil, "run args = %v", args)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf(`
----------------------------
Failed to run %v: %v
Command output was:
%s
----------------------------
`, args, err, out)
	}
}

// check the dst and src are identical
func checkTree(ctx context.Context, name string, t *testing.T, dstArchive, src string, expectedCount int) {
	t.Run(name, func(t *testing.T) {
		fs.Debugf(nil, "check %q vs %q", dstArchive, src)
		Farchive, err := cache.Get(ctx, dstArchive)
		if err != fs.ErrorIsFile {
			require.NoError(t, err)
		}
		Fsrc, err := cache.Get(ctx, src)
		if err != fs.ErrorIsFile {
			require.NoError(t, err)
		}

		var matches bytes.Buffer
		opt := operations.CheckOpt{
			Fdst:  Farchive,
			Fsrc:  Fsrc,
			Match: &matches,
		}

		for _, action := range []string{"Check", "Download"} {
			t.Run(action, func(t *testing.T) {
				matches.Reset()
				if action == "Download" {
					assert.NoError(t, operations.CheckDownload(ctx, &opt))
				} else {
					assert.NoError(t, operations.Check(ctx, &opt))
				}
				if expectedCount > 0 {
					assert.Equal(t, expectedCount, strings.Count(matches.String(), "\n"))
				}
			})
		}

		t.Run("NewObject", func(t *testing.T) {
			// Check we can run NewObject on all files and read them
			assert.NoError(t, operations.ListFn(ctx, Fsrc, func(srcObj fs.Object) {
				if t.Failed() {
					return
				}
				remote := srcObj.Remote()
				archiveObj, err := Farchive.NewObject(ctx, remote)
				require.NoError(t, err, remote)
				assert.Equal(t, remote, archiveObj.Remote(), remote)

				// Test that the contents are the same
				archiveBuf := fstests.ReadObject(ctx, t, archiveObj, -1)
				srcBuf := fstests.ReadObject(ctx, t, srcObj, -1)
				assert.Equal(t, srcBuf, archiveBuf)

				if len(srcBuf) < 81 {
					return
				}

				// Tests that Open works with SeekOption
				assert.Equal(t, srcBuf[50:], fstests.ReadObject(ctx, t, archiveObj, -1, &fs.SeekOption{Offset: 50}), "contents differ after seek")

				// Tests that Open works with RangeOption
				for _, test := range []struct {
					ro                 fs.RangeOption
					wantStart, wantEnd int
				}{
					{fs.RangeOption{Start: 5, End: 15}, 5, 16},
					{fs.RangeOption{Start: 80, End: -1}, 80, len(srcBuf)},
					{fs.RangeOption{Start: 81, End: 100000}, 81, len(srcBuf)},
					{fs.RangeOption{Start: -1, End: 20}, len(srcBuf) - 20, len(srcBuf)}, // if start is omitted this means get the final bytes
					// {fs.RangeOption{Start: -1, End: -1}, 0, len(srcBuf)}, - this seems to work but the RFC doesn't define it
				} {
					got := fstests.ReadObject(ctx, t, archiveObj, -1, &test.ro)
					foundAt := strings.Index(srcBuf, got)
					help := fmt.Sprintf("%#v failed want [%d:%d] got [%d:%d]", test.ro, test.wantStart, test.wantEnd, foundAt, foundAt+len(got))
					assert.Equal(t, srcBuf[test.wantStart:test.wantEnd], got, help)
				}

				// Test that the modtimes are correct
				fstest.AssertTimeEqualWithPrecision(t, remote, srcObj.ModTime(ctx), archiveObj.ModTime(ctx), Farchive.Precision())

				// Test that the sizes are correct
				assert.Equal(t, srcObj.Size(), archiveObj.Size())

				// Test that Strings are OK
				assert.Equal(t, srcObj.String(), archiveObj.String())
			}))
		})

		// t.Logf("Fdst ------------- %v", Fdst)
		// operations.List(ctx, Fdst, os.Stdout)
		// t.Logf("Fsrc ------------- %v", Fsrc)
		// operations.List(ctx, Fsrc, os.Stdout)
	})

}

// test creating and reading back some archives
//
// Note that this uses rclone and zip as external binaries.
func testArchive(t *testing.T, archiveName string, archiveFn func(t *testing.T, output, input string)) {
	ctx := context.Background()
	checkFiles := 1000

	// create random test input files
	inputRoot := t.TempDir()
	input := filepath.Join(inputRoot, archiveName)
	require.NoError(t, os.Mkdir(input, 0777))
	run(t, "rclone", "test", "makefiles", "--files", strconv.Itoa(checkFiles), "--ascii", input)

	// Create the archive
	output := t.TempDir()
	zipFile := path.Join(output, archiveName)
	archiveFn(t, zipFile, input)

	// Check the archive itself
	checkTree(ctx, "Archive", t, ":archive:"+zipFile, input, checkFiles)

	// Now check a subdirectory
	fis, err := os.ReadDir(input)
	require.NoError(t, err)
	subDir := "NOT FOUND"
	aFile := "NOT FOUND"
	for _, fi := range fis {
		if fi.IsDir() {
			subDir = fi.Name()
		} else {
			aFile = fi.Name()
		}
	}
	checkTree(ctx, "SubDir", t, ":archive:"+zipFile+"/"+subDir, filepath.Join(input, subDir), 0)

	// Now check a single file
	fiCtx, fi := filter.AddConfig(ctx)
	require.NoError(t, fi.AddRule("+ "+aFile))
	require.NoError(t, fi.AddRule("- *"))
	checkTree(fiCtx, "SingleFile", t, ":archive:"+zipFile+"/"+aFile, filepath.Join(input, aFile), 0)

	// Now check the level above
	checkTree(ctx, "Root", t, ":archive:"+output, inputRoot, checkFiles)
	// run(t, "cp", "-a", inputRoot, output, "/tmp/test-"+archiveName)
}

// Make sure we have the executable named
func skipIfNoExe(t *testing.T, exeName string) {
	_, err := exec.LookPath(exeName)
	if err != nil {
		t.Skipf("%s executable not installed", exeName)
	}
}

// Test creating and reading back some archives
//
// Note that this uses rclone and zip as external binaries.
func TestArchiveZip(t *testing.T) {
	fstest.Initialise()
	skipIfNoExe(t, "zip")
	skipIfNoExe(t, "rclone")
	testArchive(t, "test.zip", func(t *testing.T, output, input string) {
		oldcwd, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(input))
		defer func() {
			require.NoError(t, os.Chdir(oldcwd))
		}()
		run(t, "zip", "-9r", output, ".")
	})
}

// Test creating and reading back some archives
//
// Note that this uses rclone and squashfs as external binaries.
func TestArchiveSquashfs(t *testing.T) {
	fstest.Initialise()
	skipIfNoExe(t, "mksquashfs")
	skipIfNoExe(t, "rclone")
	testArchive(t, "test.sqfs", func(t *testing.T, output, input string) {
		run(t, "mksquashfs", input, output)
	})
}

// TestArchiveSquashfsIssue9004 lists and reads squashfs images that exercise
// two layouts go-diskfs used to choke on (fixed in go-diskfs v1.9.4):
//
//   - 1.sqfs: a single empty directory, so the image has no fragment table
//     (its fragment-table start holds the "not present" sentinel).
//   - 2.sqfs: a small tree whose superblock has the NO_XATTRS flag set while
//     inodes still carry a (non-sentinel) xattr index - the shape squashfs-
//     tools-ng can emit. Built by packing a two-file tree with xattrs via
//     `gensquashfs -x`, then setting the NO_XATTRS superblock flag; the tree
//     content is trivial placeholder data.
//
// Both images used to fail to list. Regression test for #9004.
func TestArchiveSquashfsIssue9004(t *testing.T) {
	fstest.Initialise()
	ctx := context.Background()

	testdata, err := filepath.Abs(filepath.Join("squashfs", "testdata"))
	require.NoError(t, err)

	archiveFor := func(t *testing.T, name string) fs.Fs {
		f, err := cache.Get(ctx, ":archive:"+filepath.Join(testdata, name))
		require.NoError(t, err)
		return f
	}

	t.Run("EmptyDir", func(t *testing.T) {
		// 1.sqfs is a single empty directory - it must list without error.
		entries, err := archiveFor(t, "1.sqfs").List(ctx, "")
		require.NoError(t, err)
		assert.Equal(t, 0, len(entries))
	})

	t.Run("NoXattrTree", func(t *testing.T) {
		f := archiveFor(t, "2.sqfs")
		entries, err := f.List(ctx, "")
		require.NoError(t, err)
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, path.Base(e.Remote()))
		}
		assert.Contains(t, names, "alpha")
		assert.Contains(t, names, "beta")

		// A file in the tree must be readable with its real content.
		obj, err := f.NewObject(ctx, "beta/sample.xml")
		require.NoError(t, err)
		assert.Greater(t, obj.Size(), int64(0))
		rc, err := obj.Open(ctx)
		require.NoError(t, err)
		data, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		assert.Equal(t, int(obj.Size()), len(data))
		assert.True(t, bytes.HasPrefix(data, []byte("<?xml")))
	})
}

// TestArchiveUncleanRoot checks that a path into an archive which isn't
// in canonical form (with "./" or doubled slashes) still finds its
// directory.
func TestArchiveUncleanRoot(t *testing.T) {
	fstest.Initialise()
	ctx := context.Background()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("sub/dir/a.txt")
	require.NoError(t, err)
	_, err = w.Write([]byte("data"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	zipPath := filepath.Join(t.TempDir(), "test.zip")
	require.NoError(t, os.WriteFile(zipPath, buf.Bytes(), 0600))

	for _, root := range []string{"sub/dir", "sub/./dir", "sub//dir", "./sub/dir/", "sub/dir/."} {
		t.Run(root, func(t *testing.T) {
			f, err := cache.Get(ctx, ":archive:"+zipPath+"/"+root)
			require.NoError(t, err)
			entries, err := f.List(ctx, "")
			require.NoError(t, err)
			require.Len(t, entries, 1)
			assert.Equal(t, "a.txt", entries[0].Remote())
		})
	}
}

// escapingObject is an object whose remote is not where it was asked for.
type escapingObject struct {
	fs.Object
	remote string
}

func (o *escapingObject) Remote() string { return o.remote }
func (o *escapingObject) String() string { return o.remote }

// escapingFs stands in for a badly behaved archiver which exposes entry
// names outside the directory being listed.
type escapingFs struct {
	fs.Fs
	prefix string
}

func (f *escapingFs) Name() string           { return "escaping" }
func (f *escapingFs) Root() string           { return "" }
func (f *escapingFs) String() string         { return "escaping" }
func (f *escapingFs) Features() *fs.Features { return &fs.Features{} }

func (f *escapingFs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	return fs.DirEntries{
		&escapingObject{remote: path.Join(dir, "good.txt")},
		fs.NewDir(path.Join(dir, "gooddir"), fstest.Time("2001-02-03T04:05:06.499999999Z")),
		&escapingObject{remote: path.Join(dir, "../escape.txt")},
		&escapingObject{remote: "../../escape.txt"},
		&escapingObject{remote: path.Join(dir, "sub/notachild.txt")},
		fs.NewDir("../escapedir", fstest.Time("2001-02-03T04:05:06.499999999Z")),
	}, nil
}

func (f *escapingFs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return &escapingObject{remote: "../escape.txt"}, nil
}

// TestArchiveEscapingArchiver checks that the archive backend does not
// pass on entries from an archiver which escape the directory being
// listed, whatever the archiver does.
func TestArchiveEscapingArchiver(t *testing.T) {
	fstest.Initialise()
	ctx := context.Background()

	archiver.Register(archiver.Archiver{
		New: func(ctx context.Context, f fs.Fs, remote, prefix, root string) (fs.Fs, error) {
			return &escapingFs{prefix: prefix}, nil
		},
		Extension: ".escaping",
	})

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.escaping"), []byte("x"), 0600))
	f, err := cache.Get(ctx, ":archive:"+dir)
	require.NoError(t, err)

	// Archives are discovered when their parent directory is listed
	_, err = f.List(ctx, "")
	require.NoError(t, err)

	entries, err := f.List(ctx, "test.escaping")
	require.NoError(t, err)
	var remotes []string
	for _, entry := range entries {
		remotes = append(remotes, entry.Remote())
	}
	assert.ElementsMatch(t, []string{"test.escaping/good.txt", "test.escaping/gooddir"}, remotes)

	_, err = f.NewObject(ctx, "test.escaping/file.txt")
	assert.ErrorIs(t, err, fs.ErrorObjectNotFound)
}
