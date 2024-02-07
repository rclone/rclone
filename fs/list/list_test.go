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
	da := mockdir.New("dir/")
	oA := mockobject.Object("diR/a")
	db := mockdir.New("dir/b")
	oB := mockobject.Object("dir/B/sub")
	dc := mockdir.New("dir/c")
	oC := mockobject.Object("dir/C")
	dd := mockdir.New("dir/d")
	oD := mockobject.Object("dir/D")
	entries := fs.DirEntries{da, oA, db, oB, dc, oC, dd, oD}
	newEntries, err := filterAndSortDir(context.Background(), entries, true, "dir", nil, nil)
	require.NoError(t, err)
	assert.Equal(t,
		newEntries,
		fs.DirEntries{oC, oD, db, dc, dd},
	)
}

func TestFilterAndSortCheckDirRoot(t *testing.T) {
	// Check the different kinds of error when listing the root ""
	da := mockdir.New("")
	oA := mockobject.Object("A")
	db := mockdir.New("b")
	oB := mockobject.Object("B/sub")
	dc := mockdir.New("c")
	oC := mockobject.Object("C")
	dd := mockdir.New("d")
	oD := mockobject.Object("D")
	entries := fs.DirEntries{da, oA, db, oB, dc, oC, dd, oD}
	newEntries, err := filterAndSortDir(context.Background(), entries, true, "", nil, nil)
	require.NoError(t, err)
	assert.Equal(t,
		newEntries,
		fs.DirEntries{oA, oC, oD, db, dc, dd},
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
