package touch

import (
	"testing"
	"time"

	"github.com/ncw/rclone/fs/walk"
	"github.com/ncw/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/ncw/rclone/backend/local"
)

var (
	t1 = fstest.Time("2017-02-03T04:05:06.499999999Z")
)

// TestMain drives the tests
func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

func TestTouchOneFile(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	err := Touch(r.Fremote, "newFile")
	require.NoError(t, err)
	_, err = r.Fremote.NewObject("newFile")
	require.NoError(t, err)
}

func TestTouchWithNoCreateFlag(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	notCreateNewFile = true
	err := Touch(r.Fremote, "newFile")
	require.NoError(t, err)
	_, err = r.Fremote.NewObject("newFile")
	require.Error(t, err)
	notCreateNewFile = false
}

func TestTouchWithTimestamp(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	timeAsArgument = "060102"
	err := Touch(r.Fremote, "oldFile")
	require.NoError(t, err)
	file, err := r.Fremote.NewObject("oldFile")
	require.NoError(t, err)
	curretTime := time.Now()
	assert.Equal(t, true, file.ModTime().Year() < curretTime.Year())
}

func TestTouchWithLognerTimestamp(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	timeAsArgument = "2006-01-02T15:04:05"
	err := Touch(r.Fremote, "oldFile")
	require.NoError(t, err)
	file, err := r.Fremote.NewObject("oldFile")
	require.NoError(t, err)
	curretTime := time.Now()
	assert.Equal(t, true, file.ModTime().Year() < curretTime.Year())
}

func TestTouchUpdateTimestamp(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	file1 := r.WriteObject("a", "aaa", t1)
	fstest.CheckItems(t, r.Fremote, file1)

	file, err := r.Fremote.NewObject("a")
	require.NoError(t, err)
	aSize := file.Size()
	timeAsArgument = "121212"
	err = Touch(r.Fremote, "a")
	require.NoError(t, err)
	file, err = r.Fremote.NewObject("a")
	require.NoError(t, err)
	assert.Equal(t, aSize, file.Size())
	assert.Equal(t, 2012, file.ModTime().Year())
}

func TestTouchUpdateTimestampWithCFlag(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	file1 := r.WriteObject("a", "aaa", t1)
	fstest.CheckItems(t, r.Fremote, file1)

	notCreateNewFile = true
	timeAsArgument = "121212"
	err := Touch(r.Fremote, "a")
	require.NoError(t, err)
	file, err := r.Fremote.NewObject("a")
	require.NoError(t, err)
	assert.Equal(t, 2012, file.ModTime().Year())
	notCreateNewFile = false
}

func TestTouchCreateMultipleDirAndFile(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	longPath := "a/b/c/d/e/f/g/h.txt"
	err := Touch(r.Fremote, longPath)
	require.NoError(t, err)
	file, err := r.Fremote.NewObject(longPath)
	require.NoError(t, err)
	assert.Equal(t, longPath, file.Remote())

	objs, dirs, err := walk.GetAll(r.Fremote, "", true, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, len(dirs))
	assert.Equal(t, 0, len(objs))
}
