package version

import (
	"io/ioutil"
	"os"
	"runtime"
	"testing"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs/config"
	"github.com/stretchr/testify/assert"
)

func TestVersionWorksWithoutAccessibleConfigFile(t *testing.T) {
	// create temp config file
	tempFile, err := ioutil.TempFile("", "unreadable_config.conf")
	assert.NoError(t, err)
	path := tempFile.Name()
	defer func() {
		err := os.Remove(path)
		assert.NoError(t, err)
	}()
	assert.NoError(t, tempFile.Close())
	if runtime.GOOS != "windows" {
		assert.NoError(t, os.Chmod(path, 0000))
	}
	// re-wire
	oldOsStdout := os.Stdout
	oldConfigPath := config.ConfigPath
	config.ConfigPath = path
	os.Stdout = nil
	defer func() {
		os.Stdout = oldOsStdout
		config.ConfigPath = oldConfigPath
	}()

	cmd.Root.SetArgs([]string{"version"})
	assert.NotPanics(t, func() {
		assert.NoError(t, cmd.Root.Execute())
	})

	// This causes rclone to exit and the tests to stop!
	// cmd.Root.SetArgs([]string{"--version"})
	// assert.NotPanics(t, func() {
	// 	assert.NoError(t, cmd.Root.Execute())
	// })
}
