package lsf

import (
	"bytes"
	"context"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/list"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultLsf(t *testing.T) {
	fstest.Initialise()
	buf := new(bytes.Buffer)

	f, err := fs.NewFs(context.Background(), "testfiles")
	require.NoError(t, err)

	err = Lsf(context.Background(), f, buf)
	require.NoError(t, err)
	assert.Equal(t, `file1
file2
file3
subdir/
`, buf.String())
}

func TestRecurseFlag(t *testing.T) {
	fstest.Initialise()
	buf := new(bytes.Buffer)

	f, err := fs.NewFs(context.Background(), "testfiles")
	require.NoError(t, err)

	recurse = true
	err = Lsf(context.Background(), f, buf)
	require.NoError(t, err)
	assert.Equal(t, `file1
file2
file3
subdir/
subdir/file1
subdir/file2
subdir/file3
`, buf.String())
	recurse = false
}

func TestDirSlashFlag(t *testing.T) {
	fstest.Initialise()
	buf := new(bytes.Buffer)

	f, err := fs.NewFs(context.Background(), "testfiles")
	require.NoError(t, err)

	dirSlash = true
	format = "p"
	err = Lsf(context.Background(), f, buf)
	require.NoError(t, err)
	assert.Equal(t, `file1
file2
file3
subdir/
`, buf.String())

	buf = new(bytes.Buffer)
	dirSlash = false
	err = Lsf(context.Background(), f, buf)
	require.NoError(t, err)
	assert.Equal(t, `file1
file2
file3
subdir
`, buf.String())
}

func TestFormat(t *testing.T) {
	fstest.Initialise()
	f, err := fs.NewFs(context.Background(), "testfiles")
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	format = "p"
	err = Lsf(context.Background(), f, buf)
	require.NoError(t, err)
	assert.Equal(t, `file1
file2
file3
subdir
`, buf.String())

	buf = new(bytes.Buffer)
	format = "s"
	err = Lsf(context.Background(), f, buf)
	require.NoError(t, err)
	assert.Equal(t, `0
321
1234
-1
`, buf.String())

	buf = new(bytes.Buffer)
	format = "hp"
	err = Lsf(context.Background(), f, buf)
	require.NoError(t, err)
	assert.Equal(t, `d41d8cd98f00b204e9800998ecf8427e;file1
409d6c19451dd39d4a94e42d2ff2c834;file2
9b4c8a5e36d3be7e2c4b1d75ded8c8a1;file3
;subdir
`, buf.String())

	buf = new(bytes.Buffer)
	format = "p"
	filesOnly = true
	err = Lsf(context.Background(), f, buf)
	require.NoError(t, err)
	assert.Equal(t, `file1
file2
file3
`, buf.String())
	filesOnly = false

	buf = new(bytes.Buffer)
	format = "p"
	dirsOnly = true
	err = Lsf(context.Background(), f, buf)
	require.NoError(t, err)
	assert.Equal(t, `subdir
`, buf.String())
	dirsOnly = false

	buf = new(bytes.Buffer)
	format = "t"
	err = Lsf(context.Background(), f, buf)
	require.NoError(t, err)

	items, _ := list.DirSorted(context.Background(), f, true, "")
	var expectedOutput string
	for _, item := range items {
		expectedOutput += item.ModTime(context.Background()).Format("2006-01-02 15:04:05") + "\n"
	}

	assert.Equal(t, expectedOutput, buf.String())

	buf = new(bytes.Buffer)
	format = "sp"
	err = Lsf(context.Background(), f, buf)
	require.NoError(t, err)
	assert.Equal(t, `0;file1
321;file2
1234;file3
-1;subdir
`, buf.String())
	format = ""
}

func TestSeparator(t *testing.T) {
	fstest.Initialise()
	f, err := fs.NewFs(context.Background(), "testfiles")
	require.NoError(t, err)
	format = "ps"

	buf := new(bytes.Buffer)
	err = Lsf(context.Background(), f, buf)
	require.NoError(t, err)
	assert.Equal(t, `file1;0
file2;321
file3;1234
subdir;-1
`, buf.String())

	separator = "__SEP__"
	buf = new(bytes.Buffer)
	err = Lsf(context.Background(), f, buf)
	require.NoError(t, err)
	assert.Equal(t, `file1__SEP__0
file2__SEP__321
file3__SEP__1234
subdir__SEP__-1
`, buf.String())
	format = ""
	separator = ""
}

func TestWholeLsf(t *testing.T) {
	fstest.Initialise()
	f, err := fs.NewFs(context.Background(), "testfiles")
	require.NoError(t, err)
	format = "pst"
	separator = "_+_"
	recurse = true
	dirSlash = true

	buf := new(bytes.Buffer)
	err = Lsf(context.Background(), f, buf)
	require.NoError(t, err)

	items, _ := list.DirSorted(context.Background(), f, true, "")
	itemsInSubdir, _ := list.DirSorted(context.Background(), f, true, "subdir")
	var expectedOutput []string
	for _, item := range items {
		expectedOutput = append(expectedOutput, item.ModTime(context.Background()).Format("2006-01-02 15:04:05"))
	}
	for _, item := range itemsInSubdir {
		expectedOutput = append(expectedOutput, item.ModTime(context.Background()).Format("2006-01-02 15:04:05"))
	}

	assert.Equal(t, `file1_+_0_+_`+expectedOutput[0]+`
file2_+_321_+_`+expectedOutput[1]+`
file3_+_1234_+_`+expectedOutput[2]+`
subdir/_+_-1_+_`+expectedOutput[3]+`
subdir/file1_+_0_+_`+expectedOutput[4]+`
subdir/file2_+_1_+_`+expectedOutput[5]+`
subdir/file3_+_111_+_`+expectedOutput[6]+`
`, buf.String())

	format = ""
	separator = ""
	recurse = false
	dirSlash = false
}

func TestTimeFormat(t *testing.T) {
	fstest.Initialise()
	f, err := fs.NewFs(context.Background(), "testfiles")
	require.NoError(t, err)
	format = "pst"
	separator = "_+_"
	recurse = true
	dirSlash = true
	timeFormat = "Jan 2, 2006 at 3:04pm (MST)"

	buf := new(bytes.Buffer)
	err = Lsf(context.Background(), f, buf)
	require.NoError(t, err)

	items, _ := list.DirSorted(context.Background(), f, true, "")
	itemsInSubdir, _ := list.DirSorted(context.Background(), f, true, "subdir")
	var expectedOutput []string
	for _, item := range items {
		expectedOutput = append(expectedOutput, item.ModTime(context.Background()).Format(timeFormat))
	}
	for _, item := range itemsInSubdir {
		expectedOutput = append(expectedOutput, item.ModTime(context.Background()).Format(timeFormat))
	}

	assert.Equal(t, `file1_+_0_+_`+expectedOutput[0]+`
file2_+_321_+_`+expectedOutput[1]+`
file3_+_1234_+_`+expectedOutput[2]+`
subdir/_+_-1_+_`+expectedOutput[3]+`
subdir/file1_+_0_+_`+expectedOutput[4]+`
subdir/file2_+_1_+_`+expectedOutput[5]+`
subdir/file3_+_111_+_`+expectedOutput[6]+`
`, buf.String())

	format = ""
	separator = ""
	recurse = false
	dirSlash = false
}

func TestTimeFormatMax(t *testing.T) {
	fstest.Initialise()
	f, err := fs.NewFs(context.Background(), "testfiles")
	require.NoError(t, err)
	format = "pst"
	separator = "_+_"
	recurse = true
	dirSlash = true
	timeFormat = "max"
	precision := operations.FormatForLSFPrecision(f.Precision())

	buf := new(bytes.Buffer)
	err = Lsf(context.Background(), f, buf)
	require.NoError(t, err)

	items, _ := list.DirSorted(context.Background(), f, true, "")
	itemsInSubdir, _ := list.DirSorted(context.Background(), f, true, "subdir")
	var expectedOutput []string
	for _, item := range items {
		expectedOutput = append(expectedOutput, item.ModTime(context.Background()).Format(precision))
	}
	for _, item := range itemsInSubdir {
		expectedOutput = append(expectedOutput, item.ModTime(context.Background()).Format(precision))
	}

	assert.Equal(t, `file1_+_0_+_`+expectedOutput[0]+`
file2_+_321_+_`+expectedOutput[1]+`
file3_+_1234_+_`+expectedOutput[2]+`
subdir/_+_-1_+_`+expectedOutput[3]+`
subdir/file1_+_0_+_`+expectedOutput[4]+`
subdir/file2_+_1_+_`+expectedOutput[5]+`
subdir/file3_+_111_+_`+expectedOutput[6]+`
`, buf.String())

	format = ""
	separator = ""
	recurse = false
	dirSlash = false
}
