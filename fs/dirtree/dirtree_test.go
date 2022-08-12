package dirtree

import (
	"testing"

	"github.com/rclone/rclone/fstest/mockdir"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	dt := New()
	assert.Equal(t, "", dt.String())
}

func TestParentDir(t *testing.T) {
	assert.Equal(t, "root/parent", parentDir("root/parent/file"))
	assert.Equal(t, "parent", parentDir("parent/file"))
	assert.Equal(t, "", parentDir("parent"))
	assert.Equal(t, "", parentDir(""))
}

func TestDirTreeAdd(t *testing.T) {
	dt := New()
	o := mockobject.New("potato")
	dt.Add(o)
	assert.Equal(t, `/
  potato
`, dt.String())
	o = mockobject.New("dir/subdir/sausage")
	dt.Add(o)
	assert.Equal(t, `/
  potato
dir/subdir/
  sausage
`, dt.String())
}

func TestDirTreeAddDir(t *testing.T) {
	dt := New()
	d := mockdir.New("potato")
	dt.Add(d)
	assert.Equal(t, `/
  potato/
`, dt.String())
	d = mockdir.New("dir/subdir/sausage")
	dt.AddDir(d)
	assert.Equal(t, `/
  potato/
dir/subdir/
  sausage/
dir/subdir/sausage/
`, dt.String())
	d = mockdir.New("")
	dt.AddDir(d)
	assert.Equal(t, `/
  potato/
dir/subdir/
  sausage/
dir/subdir/sausage/
`, dt.String())
}

func TestDirTreeAddEntry(t *testing.T) {
	dt := New()

	d := mockdir.New("dir/subdir/sausagedir")
	dt.AddEntry(d)
	o := mockobject.New("dir/subdir2/sausage2")
	dt.AddEntry(o)

	assert.Equal(t, `/
  dir/
dir/
  subdir/
  subdir2/
dir/subdir/
  sausagedir/
dir/subdir/sausagedir/
dir/subdir2/
  sausage2
`, dt.String())
}

func TestDirTreeFind(t *testing.T) {
	dt := New()

	parent, foundObj := dt.Find("dir/subdir/sausage")
	assert.Equal(t, "dir/subdir", parent)
	assert.Nil(t, foundObj)

	o := mockobject.New("dir/subdir/sausage")
	dt.Add(o)

	parent, foundObj = dt.Find("dir/subdir/sausage")
	assert.Equal(t, "dir/subdir", parent)
	assert.Equal(t, o, foundObj)
}

func TestDirTreeCheckParent(t *testing.T) {
	dt := New()

	o := mockobject.New("dir/subdir/sausage")
	dt.Add(o)

	assert.Equal(t, `dir/subdir/
  sausage
`, dt.String())

	dt.CheckParent("", "dir/subdir")

	assert.Equal(t, `/
  dir/
dir/
  subdir/
dir/subdir/
  sausage
`, dt.String())

}

func TestDirTreeCheckParents(t *testing.T) {
	dt := New()

	dt.Add(mockobject.New("dir/subdir/sausage"))
	dt.Add(mockobject.New("dir/subdir2/sausage2"))

	dt.CheckParents("")
	dt.Sort() // sort since the exact order of adding parents is not defined

	assert.Equal(t, `/
  dir/
dir/
  subdir/
  subdir2/
dir/subdir/
  sausage
dir/subdir2/
  sausage2
`, dt.String())
}

func TestDirTreeSort(t *testing.T) {
	dt := New()

	dt.Add(mockobject.New("dir/subdir/B"))
	dt.Add(mockobject.New("dir/subdir/A"))

	assert.Equal(t, `dir/subdir/
  B
  A
`, dt.String())

	dt.Sort()

	assert.Equal(t, `dir/subdir/
  A
  B
`, dt.String())
}

func TestDirTreeDirs(t *testing.T) {
	dt := New()

	dt.Add(mockobject.New("dir/subdir/sausage"))
	dt.Add(mockobject.New("dir/subdir2/sausage2"))

	dt.CheckParents("")

	assert.Equal(t, []string{
		"",
		"dir",
		"dir/subdir",
		"dir/subdir2",
	}, dt.Dirs())
}

func TestDirTreePrune(t *testing.T) {
	dt := New()

	dt.Add(mockobject.New("file"))
	dt.Add(mockobject.New("dir/subdir/sausage"))
	dt.Add(mockobject.New("dir/subdir2/sausage2"))
	dt.Add(mockobject.New("dir/file"))
	dt.Add(mockobject.New("dir2/file"))

	dt.CheckParents("")

	err := dt.Prune(map[string]bool{
		"dir": true,
	})
	require.NoError(t, err)

	assert.Equal(t, `/
  file
  dir2/
dir2/
  file
`, dt.String())

}
