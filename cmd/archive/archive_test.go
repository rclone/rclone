package archive_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mholt/archives"
	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/backend/memory"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rclone/rclone/cmd/archive/create"
	"github.com/rclone/rclone/cmd/archive/extract"
	"github.com/rclone/rclone/cmd/archive/list"
)

var (
	t1 = fstest.Time("2017-02-03T04:05:06.499999999Z")
)

// TestMain drives the tests
func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

func TestCheckValidDestination(t *testing.T) {
	var err error

	ctx := context.Background()
	r := fstest.NewRun(t)

	// create file
	r.WriteObject(ctx, "file1.txt", "111", t1)

	// test checkValidDestination when file exists
	err = create.CheckValidDestination(ctx, r.Fremote, "file1.txt")
	require.NoError(t, err)

	// test checkValidDestination when file does not exist
	err = create.CheckValidDestination(ctx, r.Fremote, "file2.txt")
	require.NoError(t, err)

	// test checkValidDestination when dest is a directory
	if r.Fremote.Features().CanHaveEmptyDirectories {
		err = create.CheckValidDestination(ctx, r.Fremote, "")
		require.ErrorIs(t, err, fs.ErrorIsDir)
	}

	// test checkValidDestination when dest does not exists
	err = create.CheckValidDestination(ctx, r.Fremote, "dir/file.txt")
	require.NoError(t, err)
}

// test archiving to the remote
func testArchiveRemote(t *testing.T, fromLocal bool, subDir string, extension string) {
	var err error
	ctx := context.Background()
	r := fstest.NewRun(t)
	var src, dst fs.Fs
	var f1, f2, f3 fstest.Item

	// create files to archive on src
	if fromLocal {
		// create files to archive on local
		src = r.Flocal
		dst = r.Fremote
		f1 = r.WriteFile("file1.txt", "content 1", t1)
		f2 = r.WriteFile("dir1/sub1.txt", "sub content 1", t1)
		f3 = r.WriteFile("dir2/sub2a.txt", "sub content 2a", t1)
	} else {
		// create files to archive on remote
		src = r.Fremote
		dst = r.Flocal
		f1 = r.WriteObject(ctx, "file1.txt", "content 1", t1)
		f2 = r.WriteObject(ctx, "dir1/sub1.txt", "sub content 1", t1)
		f3 = r.WriteObject(ctx, "dir2/sub2a.txt", "sub content 2a", t1)
	}
	fstest.CheckItems(t, src, f1, f2, f3)

	// create archive on dst
	archiveName := "test." + extension
	err = create.ArchiveCreate(ctx, dst, archiveName, src, "", "")
	require.NoError(t, err)

	// list archive on dst
	expected := map[string]int64{
		"file1.txt":      9,
		"dir1/":          0,
		"dir1/sub1.txt":  13,
		"dir2/":          0,
		"dir2/sub2a.txt": 14,
	}
	listFile := func(ctx context.Context, f archives.FileInfo) error {
		name := f.NameInArchive
		gotSize := f.Size()
		if f.IsDir() && !strings.HasSuffix(name, "/") {
			name += "/"
			gotSize = 0
		}
		wantSize, found := expected[name]
		assert.True(t, found, name)
		assert.Equal(t, wantSize, gotSize)
		delete(expected, name)
		return nil
	}
	err = list.ArchiveList(ctx, dst, archiveName, listFile)
	require.NoError(t, err)
	assert.Equal(t, 0, len(expected), expected)

	// clear the src
	require.NoError(t, operations.Purge(ctx, src, ""))
	require.NoError(t, src.Mkdir(ctx, ""))
	fstest.CheckItems(t, src)

	// extract dst archive back to src
	err = extract.ArchiveExtract(ctx, src, subDir, dst, archiveName)
	require.NoError(t, err)

	// check files on src are restored from the archive on dst
	items := []fstest.Item{f1, f2, f3}
	if subDir != "" {
		for i := range items {
			item := &items[i]
			item.Path = subDir + "/" + item.Path
		}
	}
	fstest.CheckListingWithPrecision(t, src, items, nil, fs.ModTimeNotSupported)
}

func testArchive(t *testing.T) {
	var extensions = []string{
		"zip",
		"tar",
		"tar.gz",
		"tar.bz2",
		"tar.lz",
		"tar.lz4",
		"tar.xz",
		"tar.zst",
		"tar.br",
		"tar.sz",
		"tar.mz",
	}
	for _, extension := range extensions {
		t.Run(extension, func(t *testing.T) {
			for _, subDir := range []string{"", "subdir"} {
				name := subDir
				if name == "" {
					name = "root"
				}
				t.Run(name, func(t *testing.T) {
					t.Run("local", func(t *testing.T) {
						testArchiveRemote(t, true, name, extension)
					})
					t.Run("remote", func(t *testing.T) {
						testArchiveRemote(t, false, name, extension)
					})
				})
			}
		})
	}
}

func TestIntegration(t *testing.T) {
	testArchive(t)
}

func TestMemory(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("skipping as -remote is set")
	}

	// Reset -remote to point to :memory:
	oldFstestRemoteName := fstest.RemoteName
	remoteName := ":memory:"
	fstest.RemoteName = &remoteName
	defer func() {
		fstest.RemoteName = oldFstestRemoteName
	}()
	fstest.ResetRun()

	testArchive(t)
}
