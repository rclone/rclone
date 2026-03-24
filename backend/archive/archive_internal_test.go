//go:build !plan9

package archive

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/backend/archive/archiver"
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

// Test that findAllArchives returns the correct archive candidates
// sorted from longest match to shortest match.
func TestFindAllArchives(t *testing.T) {
	require.GreaterOrEqual(t, len(archiver.Archivers), 1, "need at least one registered archiver")
	ext1 := archiver.Archivers[0].Extension
	var ext2 string
	if len(archiver.Archivers) >= 2 {
		ext2 = archiver.Archivers[1].Extension
	}

	tests := []struct {
		name   string
		remote string
		want   []string // expected .remote values, in order (longest first)
		skip   bool
	}{
		{
			name:   "Empty",
			remote: "",
			want:   nil,
		},
		{
			name:   "NoExtension",
			remote: "path/to/dir",
			want:   nil,
		},
		{
			name:   "SingleAtEnd",
			remote: "dir/test" + ext1,
			want:   []string{"dir/test" + ext1},
		},
		{
			name:   "SingleMidPath",
			remote: "dir/test" + ext1 + "/inner",
			want:   []string{"dir/test" + ext1},
		},
		{
			name:   "NotAtBoundary",
			remote: "dir/test" + ext1 + "py",
			want:   nil,
		},
		{
			name:   "BoundaryAndNonBoundary",
			remote: "test" + ext1 + "py/real" + ext1,
			want:   []string{"test" + ext1 + "py/real" + ext1},
		},
		{
			name:   "TwoSameExtension",
			remote: "a" + ext1 + "/b" + ext1,
			want:   []string{"a" + ext1 + "/b" + ext1, "a" + ext1},
		},
		{
			name:   "TwoDifferentExtensions",
			remote: "a" + ext2 + "/b" + ext1,
			want:   []string{"a" + ext2 + "/b" + ext1, "a" + ext2},
			skip:   ext2 == "",
		},
		{
			name:   "ExtensionOnly",
			remote: ext1,
			want:   []string{ext1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip("not enough registered archivers")
			}
			got := findAllArchives(tt.remote)
			require.Equal(t, len(tt.want), len(got), "result count")
			for i, w := range tt.want {
				assert.Equal(t, w, got[i].remote, "remote[%d]", i)
				assert.Equal(t, w, got[i].prefix, "prefix[%d]", i)
				assert.Equal(t, "", got[i].root, "root[%d]", i)
			}
		})
	}
}

// Test that the probe loop in NewFs correctly resolves nested archive
// paths by falling back from the greedy match to a shorter candidate.
func TestNestedArchivePath(t *testing.T) {
	fstest.Initialise()
	skipIfNoExe(t, "zip")
	ctx := context.Background()

	// Create a file named "inner.zip" to put inside the archive
	inputDir := t.TempDir()
	innerContent := []byte("inner archive content")
	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "inner.zip"), innerContent, 0666))

	// Create outer.zip containing inner.zip using the zip command
	outputDir := t.TempDir()
	outerZip := filepath.Join(outputDir, "outer.zip")
	oldcwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(inputDir))
	defer func() { require.NoError(t, os.Chdir(oldcwd)) }()
	run(t, "zip", "-9r", outerZip, ".")

	// Access the nested path — probe should fall back to outer.zip
	f, err := cache.Get(ctx, ":archive:"+outerZip+"/inner.zip")
	if err == fs.ErrorIsFile {
		err = nil
	}
	require.NoError(t, err)
	require.NotNil(t, f)

	// Verify we can read inner.zip from inside the archive
	obj, err := f.NewObject(ctx, "inner.zip")
	require.NoError(t, err)
	assert.Equal(t, int64(len(innerContent)), obj.Size())
}
