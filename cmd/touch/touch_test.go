package touch

import (
	"testing"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/ncw/rclone/backend/local"
)

func TestTouch(t *testing.T) {
	fstest.Initialise()
	f, err := fs.NewFs("testfiles")
	err = Touch(f, "newFile")
	require.NoError(t, err)
	file, errFile := f.NewObject("newFile")
	require.NoError(t, errFile)
	err = file.Remove()
	require.NoError(t, err)

	notCreateNewFile = true
	err = Touch(f, "fileWithCflag")
	require.NoError(t, err)
	file, errFile = f.NewObject("fileWithCflag")
	require.Error(t, errFile)
	notCreateNewFile = false

	timeAsArgument = "060102"
	err = Touch(f, "oldFile")
	require.NoError(t, err)
	file, err = f.NewObject("oldFile")
	require.NoError(t, err)
	curretTime := time.Now()
	require.NoError(t, err)
	print(file.ModTime().Year() < curretTime.Year())
	assert.Equal(t, true, file.ModTime().Year() < curretTime.Year())
	err = file.Remove()
	require.NoError(t, err)

	timeAsArgument = "2006-01-02T15:04:05"
	err = Touch(f, "oldFile")
	require.NoError(t, err)
	file, err = f.NewObject("oldFile")
	require.NoError(t, err)
	assert.Equal(t, true, file.ModTime().Year() < curretTime.Year())

	timeAsArgument = ""
	err = Touch(f, "oldFile")
	require.NoError(t, err)
	file, err = f.NewObject("oldFile")
	require.NoError(t, err)
	timeBetween2007YearAndCurrent, errTime := time.Parse("060102", "121212")
	require.NoError(t, errTime)
	assert.Equal(t, true, file.ModTime().Year() > timeBetween2007YearAndCurrent.Year())
	err = file.Remove()
	require.NoError(t, err)
}
