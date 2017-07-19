// Internal tests for operations

package fs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterAndSortIncludeAll(t *testing.T) {
	da := newDir("a")
	oA := mockObject("A")
	db := newDir("b")
	oB := mockObject("B")
	dc := newDir("c")
	oC := mockObject("C")
	dd := newDir("d")
	oD := mockObject("D")
	entries := DirEntries{da, oA, db, oB, dc, oC, dd, oD}
	includeObject := func(o Object) bool {
		return o != oB
	}
	includeDirectory := func(remote string) bool {
		return remote != "c"
	}
	// no filter
	newEntries, err := filterAndSortDir(entries, true, "", includeObject, includeDirectory)
	require.NoError(t, err)
	assert.Equal(t,
		newEntries,
		DirEntries{oA, oB, oC, oD, da, db, dc, dd},
	)
	// filter
	newEntries, err = filterAndSortDir(entries, false, "", includeObject, includeDirectory)
	require.NoError(t, err)
	assert.Equal(t,
		newEntries,
		DirEntries{oA, oC, oD, da, db, dd},
	)
}

func TestFilterAndSortCheckDir(t *testing.T) {
	// Check the different kinds of error when listing "dir"
	da := newDir("dir/")
	oA := mockObject("diR/a")
	db := newDir("dir/b")
	oB := mockObject("dir/B/sub")
	dc := newDir("dir/c")
	oC := mockObject("dir/C")
	dd := newDir("dir/d")
	oD := mockObject("dir/D")
	entries := DirEntries{da, oA, db, oB, dc, oC, dd, oD}
	newEntries, err := filterAndSortDir(entries, true, "dir", nil, nil)
	require.NoError(t, err)
	assert.Equal(t,
		newEntries,
		DirEntries{oC, oD, db, dc, dd},
	)
}

func TestFilterAndSortCheckDirRoot(t *testing.T) {
	// Check the different kinds of error when listing the root ""
	da := newDir("")
	oA := mockObject("A")
	db := newDir("b")
	oB := mockObject("B/sub")
	dc := newDir("c")
	oC := mockObject("C")
	dd := newDir("d")
	oD := mockObject("D")
	entries := DirEntries{da, oA, db, oB, dc, oC, dd, oD}
	newEntries, err := filterAndSortDir(entries, true, "", nil, nil)
	require.NoError(t, err)
	assert.Equal(t,
		newEntries,
		DirEntries{oA, oC, oD, db, dc, dd},
	)
}

func TestFilterAndSortUnknown(t *testing.T) {
	// Check that an unknown entry produces an error
	da := newDir("")
	oA := mockObject("A")
	ub := unknownDirEntry("b")
	oB := mockObject("B/sub")
	entries := DirEntries{da, oA, ub, oB}
	newEntries, err := filterAndSortDir(entries, true, "", nil, nil)
	assert.Error(t, err, "error")
	assert.Nil(t, newEntries)
}
