package touch

import (
	"context"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/require"
)

var (
	t1 = fstest.Time("2017-02-03T04:05:06.499999999Z")
)

func checkFile(t *testing.T, r fs.Fs, path string, content string) {
	layout := defaultLayout
	if len(timeAsArgument) == len(layoutDateWithTime) {
		layout = layoutDateWithTime
	}
	timeAtrFromFlags, err := time.Parse(layout, timeAsArgument)
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

func TestTouchWithLognerTimestamp(t *testing.T) {
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
	fstest.CheckItems(t, r.Fremote, file1)

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
	fstest.CheckItems(t, r.Fremote, file1)

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
