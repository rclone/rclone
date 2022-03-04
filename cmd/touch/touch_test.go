package touch

import (
	"context"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/require"
)

var (
	t1 = fstest.Time("2017-02-03T04:05:06.499999999Z")
)

func checkFile(t *testing.T, r fs.Fs, path string, content string) {
	timeAtrFromFlags, err := timeOfTouch()
	require.NoError(t, err)
	file1 := fstest.NewItem(path, content, timeAtrFromFlags)
	fstest.CheckItems(t, r, file1)
}

// TestMain drives the tests
func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

func TestTouchOneFile(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	err := Touch(context.Background(), r.Fremote, "newFile")
	require.NoError(t, err)
	_, err = r.Fremote.NewObject(context.Background(), "newFile")
	require.NoError(t, err)
}

func TestTouchWithNoCreateFlag(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	notCreateNewFile = true
	err := Touch(context.Background(), r.Fremote, "newFile")
	require.NoError(t, err)
	_, err = r.Fremote.NewObject(context.Background(), "newFile")
	require.Error(t, err)
	notCreateNewFile = false
}

func TestTouchWithTimestamp(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	timeAsArgument = "060102"
	srcFileName := "oldFile"
	err := Touch(context.Background(), r.Fremote, srcFileName)
	require.NoError(t, err)
	checkFile(t, r.Fremote, srcFileName, "")
}

func TestTouchWithLongerTimestamp(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	timeAsArgument = "2006-01-02T15:04:05"
	srcFileName := "oldFile"
	err := Touch(context.Background(), r.Fremote, srcFileName)
	require.NoError(t, err)
	checkFile(t, r.Fremote, srcFileName, "")
}

func TestTouchUpdateTimestamp(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	srcFileName := "a"
	content := "aaa"
	file1 := r.WriteObject(context.Background(), srcFileName, content, t1)
	r.CheckRemoteItems(t, file1)

	timeAsArgument = "121212"
	err := Touch(context.Background(), r.Fremote, "a")
	require.NoError(t, err)
	checkFile(t, r.Fremote, srcFileName, content)
}

func TestTouchUpdateTimestampWithCFlag(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	srcFileName := "a"
	content := "aaa"
	file1 := r.WriteObject(context.Background(), srcFileName, content, t1)
	r.CheckRemoteItems(t, file1)

	notCreateNewFile = true
	timeAsArgument = "121212"
	err := Touch(context.Background(), r.Fremote, "a")
	require.NoError(t, err)
	checkFile(t, r.Fremote, srcFileName, content)
	notCreateNewFile = false
}

func TestTouchCreateMultipleDirAndFile(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	longPath := "a/b/c.txt"
	err := Touch(context.Background(), r.Fremote, longPath)
	require.NoError(t, err)
	file1 := fstest.NewItem("a/b/c.txt", "", t1)
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, []string{"a", "a/b"}, fs.ModTimeNotSupported)
}

func TestTouchEmptyName(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	err := Touch(context.Background(), r.Fremote, "")
	require.NoError(t, err)
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{}, []string{}, fs.ModTimeNotSupported)
}

func TestTouchEmptyDir(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	err := r.Fremote.Mkdir(context.Background(), "a")
	require.NoError(t, err)
	err = Touch(context.Background(), r.Fremote, "a")
	require.NoError(t, err)
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{}, []string{"a"}, fs.ModTimeNotSupported)
}

func TestTouchDirWithFiles(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	err := r.Fremote.Mkdir(context.Background(), "a")
	require.NoError(t, err)
	file1 := r.WriteObject(context.Background(), "a/f1", "111", t1)
	file2 := r.WriteObject(context.Background(), "a/f2", "222", t1)
	err = Touch(context.Background(), r.Fremote, "a")
	require.NoError(t, err)
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1, file2}, []string{"a"}, fs.ModTimeNotSupported)
}

func TestRecursiveTouchDirWithFiles(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	err := r.Fremote.Mkdir(context.Background(), "a/b/c")
	require.NoError(t, err)
	file1 := r.WriteObject(context.Background(), "a/f1", "111", t1)
	file2 := r.WriteObject(context.Background(), "a/b/f2", "222", t1)
	file3 := r.WriteObject(context.Background(), "a/b/c/f3", "333", t1)
	recursive = true
	err = Touch(context.Background(), r.Fremote, "a")
	recursive = false
	require.NoError(t, err)
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1, file2, file3}, []string{"a", "a/b", "a/b/c"}, fs.ModTimeNotSupported)
}
