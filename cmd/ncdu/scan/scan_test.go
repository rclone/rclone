package scan

import (
	"context"
	"fmt"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/mockdir"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScan(t *testing.T) {
	fstest.Initialise()

	// testfiles (from cmd/tree) contains: file1, file2, file3 (empty), subdir/{file4,file5} (empty)
	f, err := fs.NewFs(context.Background(), "../../tree/testfiles")
	require.NoError(t, err)

	rootChan, errChan, updatedChan := Scan(context.Background(), f)

	root := <-rootChan
	require.NotNil(t, root)
	require.NoError(t, <-errChan)

	assert.Equal(t, "", root.Path())
	assert.Nil(t, root.Parent())

	size, count := root.Attr()
	assert.Equal(t, int64(0), size)  // all files are empty
	assert.Equal(t, int64(5), count) // 3 root files + 2 subdir files

	// Entries are sorted: file1, file2, file3, subdir
	require.Len(t, root.Entries(), 4)
	i := indexByName(root.Entries(), "subdir")
	require.NotEqual(t, -1, i, "subdir not found")
	subDir, _ := root.GetDir(i)
	require.NotNil(t, subDir)

	assert.Equal(t, "subdir", subDir.Path())
	assert.Equal(t, root, subDir.Parent())

	subSize, subCount := subDir.Attr()
	assert.Equal(t, int64(0), subSize)
	assert.Equal(t, int64(2), subCount)

	select {
	case <-updatedChan:
		// at least one update was signalled during the scan
	default:
		t.Error("expected at least one update signal from Scan")
	}
}

// indexByName returns the index of the entry with the given name, or -1 if not found.
func indexByName(entries fs.DirEntries, name string) int {
	for i, e := range entries {
		if e.Remote() == name {
			return i
		}
	}
	return -1
}

func TestAttrsAverageSize(t *testing.T) {
	for _, tc := range []struct {
		name string
		a    Attrs
		want float64
	}{
		{"no files", Attrs{Count: 0}, 0},
		{"all sizes known", Attrs{Count: 4, Size: 100}, 25},
		{"some sizes unknown", Attrs{Count: 4, CountUnknownSize: 2, Size: 100}, 50},
		{"all sizes unknown", Attrs{Count: 3, CountUnknownSize: 3}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.a.AverageSize())
		})
	}
}

// fileEntry returns a mock fs.Object with the given name and content.
// The size of the object equals len(content).
func fileEntry(name, content string) fs.Object {
	return mockobject.New(name).WithContent([]byte(content), mockobject.SeekModeNone)
}

func TestNewDirSizeAndCount(t *testing.T) {
	entries := fs.DirEntries{
		fileEntry("file1", "hello"), // 5 bytes
		fileEntry("file2", "hi"),    // 2 bytes
		mockdir.New("subdir"),
	}
	d := newDir(nil, "", entries, nil)

	size, count := d.Attr()
	assert.Equal(t, int64(7), size)
	assert.Equal(t, int64(2), count) // dirs are not counted, only files
}

func TestNewDirUnknownSize(t *testing.T) {
	obj := mockobject.New("unknown").WithContent([]byte("some content"), mockobject.SeekModeNone)
	obj.SetUnknownSize(true) // simulate a backend that doesn't know the object size

	d := newDir(nil, "", fs.DirEntries{obj}, nil)

	size, count := d.Attr()
	assert.Equal(t, int64(0), size)  // unknown size treated as 0
	assert.Equal(t, int64(1), count) // still counted as a file
}

func TestNewDirReadErrorSetsEntriesHaveErrors(t *testing.T) {
	// AttrI returns subDir.entriesHaveErrors (not subDir.readError directly).
	// entriesHaveErrors is set on a dir when one of *its* children has a readError.
	// So we need three levels: root → child → grandchild(readError),
	// then root.AttrI(child) reflects child.entriesHaveErrors == true.
	root := newDir(nil, "", fs.DirEntries{mockdir.New("child")}, nil)
	child := newDir(root, "child", fs.DirEntries{mockdir.New("child/grand")}, nil)
	newDir(child, "child/grand", fs.DirEntries{}, fmt.Errorf("permission denied"))

	i := indexByName(root.Entries(), "child")
	require.NotEqual(t, -1, i)
	attrs, err := root.AttrI(i)
	assert.NoError(t, err)
	assert.True(t, attrs.EntriesHaveErrors)
}

func TestAttrIFile(t *testing.T) {
	d := newDir(nil, "", fs.DirEntries{fileEntry("f", "hello")}, nil)
	attrs, err := d.AttrI(0)
	assert.NoError(t, err)
	assert.False(t, attrs.IsDir)
	assert.True(t, attrs.Readable)
	assert.Equal(t, int64(5), attrs.Size)
}

func TestAttrIDirLoaded(t *testing.T) {
	root := newDir(nil, "", fs.DirEntries{mockdir.New("child")}, nil)
	newDir(root, "child", fs.DirEntries{fileEntry("child/f", "hello")}, nil)

	i := indexByName(root.Entries(), "child")
	require.NotEqual(t, -1, i, "child not found")
	attrs, err := root.AttrI(i)
	assert.NoError(t, err)
	assert.True(t, attrs.IsDir)
	assert.True(t, attrs.Readable)
	assert.Equal(t, int64(5), attrs.Size)
	assert.Equal(t, int64(1), attrs.Count)
}

func TestAttrIDirUnloaded(t *testing.T) {
	// Directory entry present in entries but no child Dir created yet.
	d := newDir(nil, "", fs.DirEntries{mockdir.New("missing")}, nil)
	attrs, err := d.AttrI(0)
	assert.NoError(t, err)
	assert.True(t, attrs.IsDir)
	assert.False(t, attrs.Readable)
	assert.Equal(t, int64(0), attrs.Size)
}

func TestAttrWithModTimeI(t *testing.T) {
	obj := mockobject.New("file").WithContent([]byte("abc"), mockobject.SeekModeNone)
	d := newDir(nil, "", fs.DirEntries{obj}, nil)
	attrs, err := d.AttrWithModTimeI(context.Background(), 0)
	assert.NoError(t, err)
	assert.False(t, attrs.IsDir)
	assert.Equal(t, int64(3), attrs.Size)
	assert.Equal(t, obj.ModTime(context.Background()), attrs.ModTime)
}

func TestRemoveFileUpdatesDir(t *testing.T) {
	entries := fs.DirEntries{
		fileEntry("file1", "hello"), // 5 bytes
		fileEntry("file2", "hi"),    // 2 bytes
	}
	d := newDir(nil, "", entries, nil)

	i := indexByName(d.Entries(), "file1")
	require.NotEqual(t, -1, i)
	d.Remove(i)

	size, count := d.Attr()
	assert.Equal(t, int64(2), size)
	assert.Equal(t, int64(1), count)
	assert.Len(t, d.Entries(), 1)
}

func TestRemoveUnknownSizeFile(t *testing.T) {
	obj := mockobject.New("unknown").WithContent([]byte("data"), mockobject.SeekModeNone)
	obj.SetUnknownSize(true)

	d := newDir(nil, "", fs.DirEntries{obj}, nil)
	d.Remove(0)

	size, count := d.Attr()
	assert.Equal(t, int64(0), size)
	assert.Equal(t, int64(0), count)
}

func TestRemoveDirUpdatesParent(t *testing.T) {
	root := newDir(nil, "", fs.DirEntries{mockdir.New("child")}, nil)
	newDir(root, "child", fs.DirEntries{
		fileEntry("child/file1", "hello"), // 5 bytes
		fileEntry("child/file2", "world"), // 5 bytes
	}, nil)

	rootSize, rootCount := root.Attr()
	assert.Equal(t, int64(10), rootSize)
	assert.Equal(t, int64(2), rootCount)

	i := indexByName(root.Entries(), "child")
	require.NotEqual(t, -1, i)
	root.Remove(i)

	rootSize, rootCount = root.Attr()
	assert.Equal(t, int64(0), rootSize)
	assert.Equal(t, int64(0), rootCount)
	assert.Empty(t, root.Entries())
}

func TestRemoveFilePropagatesCountsToParent(t *testing.T) {
	root := newDir(nil, "", fs.DirEntries{mockdir.New("child")}, nil)

	childEntries := fs.DirEntries{
		fileEntry("child/file1", "hello"), // 5 bytes
		fileEntry("child/file2", "world"), // 5 bytes
	}
	child := newDir(root, "child", childEntries, nil)

	// After building the tree: root accumulates child's files (10 bytes, 2 files)
	rootSize, rootCount := root.Attr()
	assert.Equal(t, int64(10), rootSize)
	assert.Equal(t, int64(2), rootCount)

	i := indexByName(child.Entries(), "child/file1")
	require.NotEqual(t, -1, i)
	child.Remove(i)

	// After removing child/file1: root should reflect the remaining 5 bytes and 1 file
	rootSize, rootCount = root.Attr()
	assert.Equal(t, int64(5), rootSize)
	assert.Equal(t, int64(1), rootCount)
}
