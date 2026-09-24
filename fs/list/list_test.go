package list

import (
	"context"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockdir"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NB integration tests for DirSorted are in
// fs/operations/listdirsorted_test.go

func TestRemoteEscapesRoot(t *testing.T) {
	for _, test := range []struct {
		in     string
		escape bool
	}{
		// non-escaping
		{"", false},
		{".", false},
		{"a", false},
		{"a/b", false},
		{"a/../b", false}, // cleans to "b" - does not climb
		{"foo/..", false}, // cleans back to root
		{"..foo", false},  // not a ".." segment
		{"a/..bar/c", false},
		{"...", false}, // three dots is an ordinary name
		{"/", false},   // absolute, but does not climb
		{"/a/b", false},
		{"//a", false},
		// climbing with a relative prefix
		{"..", true},
		{"../b", true},
		{"a/../../b", true}, // cleans to "../b"
		{"../../etc/passwd", true},
		// climbing hidden behind a leading slash - path.Clean would anchor
		// these as absolute and miss them, but path.Join(root, ...) escapes
		{"/..", true},
		{"/../x", true},
		{"//../../x", true},
		{"/../../etc/passwd", true},
		{"/./../x", true},
	} {
		assert.Equal(t, test.escape, RemoteEscapesRoot(test.in), test.in)
	}
}

func TestFilterAndSortConfinement(t *testing.T) {
	ok := mockobject.Object("ok.txt")
	dotdot := mockobject.Object("..")        // bare ".." - missed by the belongs-in-dir check
	up := mockobject.Object("../escape.txt") // climbs one level
	upDir := mockdir.New("../evildir")       // climbing directory
	deepUp := mockobject.Object("a/../../x") // cleans to "../x"
	entries := fs.DirEntries{ok, dotdot, up, upDir, deepUp}
	includeObject := func(ctx context.Context, o fs.Object) bool { return true }
	includeDirectory := func(remote string) (bool, error) { return true, nil }

	// Even with includeAll, entries that escape the root are dropped.
	newEntries, err := filterAndSortDir(context.Background(), entries, true, "", includeObject, includeDirectory)
	require.NoError(t, err)
	assert.Equal(t, fs.DirEntries{ok}, newEntries)
}

func TestFilterAndSortIncludeAll(t *testing.T) {
	da := mockdir.New("a")
	oA := mockobject.Object("A")
	db := mockdir.New("b")
	oB := mockobject.Object("B")
	dc := mockdir.New("c")
	oC := mockobject.Object("C")
	dd := mockdir.New("d")
	oD := mockobject.Object("D")
	entries := fs.DirEntries{da, oA, db, oB, dc, oC, dd, oD}
	includeObject := func(ctx context.Context, o fs.Object) bool {
		return o != oB
	}
	includeDirectory := func(remote string) (bool, error) {
		return remote != "c", nil
	}
	// no filter
	newEntries, err := filterAndSortDir(context.Background(), entries, true, "", includeObject, includeDirectory)
	require.NoError(t, err)
	assert.Equal(t,
		newEntries,
		fs.DirEntries{oA, oB, oC, oD, da, db, dc, dd},
	)
	// filter
	newEntries, err = filterAndSortDir(context.Background(), entries, false, "", includeObject, includeDirectory)
	require.NoError(t, err)
	assert.Equal(t,
		newEntries,
		fs.DirEntries{oA, oC, oD, da, db, dd},
	)
}

func TestFilterAndSortCheckDir(t *testing.T) {
	// Check the different kinds of error when listing "dir"
	da := mockdir.New("dir")
	da2 := mockdir.New("dir/") // double slash dir - allowed for bucket based remotes
	oA := mockobject.Object("diR/a")
	db := mockdir.New("dir/b")
	oB := mockobject.Object("dir/B/sub")
	dc := mockdir.New("dir/c")
	oC := mockobject.Object("dir/C")
	dd := mockdir.New("dir/d")
	oD := mockobject.Object("dir/D")
	entries := fs.DirEntries{da, da2, oA, db, oB, dc, oC, dd, oD}
	newEntries, err := filterAndSortDir(context.Background(), entries, true, "dir", nil, nil)
	require.NoError(t, err)
	assert.Equal(t,
		fs.DirEntries{da2, oC, oD, db, dc, dd},
		newEntries,
	)
}

func TestFilterAndSortCheckDirRoot(t *testing.T) {
	// Check the different kinds of error when listing the root ""
	da := mockdir.New("")
	da2 := mockdir.New("/") // doubleslash dir allowed on bucket based remotes
	oA := mockobject.Object("A")
	db := mockdir.New("b")
	oB := mockobject.Object("B/sub")
	dc := mockdir.New("c")
	oC := mockobject.Object("C")
	dd := mockdir.New("d")
	oD := mockobject.Object("D")
	entries := fs.DirEntries{da, da2, oA, db, oB, dc, oC, dd, oD}
	newEntries, err := filterAndSortDir(context.Background(), entries, true, "", nil, nil)
	require.NoError(t, err)
	assert.Equal(t,
		fs.DirEntries{da2, oA, oC, oD, db, dc, dd},
		newEntries,
	)
}

type unknownDirEntry string

func (o unknownDirEntry) Fs() fs.Info                               { return fs.Unknown }
func (o unknownDirEntry) String() string                            { return string(o) }
func (o unknownDirEntry) Remote() string                            { return string(o) }
func (o unknownDirEntry) ModTime(ctx context.Context) (t time.Time) { return t }
func (o unknownDirEntry) Size() int64                               { return 0 }

func TestFilterAndSortUnknown(t *testing.T) {
	// Check that an unknown entry produces an error
	da := mockdir.New("")
	oA := mockobject.Object("A")
	ub := unknownDirEntry("b")
	oB := mockobject.Object("B/sub")
	entries := fs.DirEntries{da, oA, ub, oB}
	newEntries, err := filterAndSortDir(context.Background(), entries, true, "", nil, nil)
	assert.Error(t, err, "error")
	assert.Nil(t, newEntries)
}
