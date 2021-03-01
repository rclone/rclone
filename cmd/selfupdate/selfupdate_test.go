package selfupdate

import (
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
)

func TestInstallOnLinux(t *testing.T) {
	if testing.Short() {
		t.Skip("not running with -short")
	}
	if runtime.GOOS != "linux" {
		t.Skip("this is a Linux only test")
	}

	// Prepare for test
	file, err := ioutil.TempFile("", "rclone-test.out")
	assert.NoError(t, err)
	path := file.Name()
	assert.NoError(t, file.Close())
	defer func() {
		_ = os.Chmod(path, 0644)
		_ = os.Remove(path)
		_ = os.Remove(path + ".old")
		_ = os.Remove(path + ".new")
	}()

	regexVer := regexp.MustCompile(`v[0-9]\S+`)

	betaVer, _, err := GetVersion(true, "")
	assert.NoError(t, err)

	// Must do nothing if version isn't changing
	assert.NoError(t, InstallUpdate(&Options{Beta: true, Output: path, Version: fs.Version}))

	// Must fail on non-writable file
	assert.NoError(t, os.Chmod(path, 0000))
	err = (InstallUpdate(&Options{Beta: true, Output: path}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "run self-update as root")

	// Must keep non-standard permissions
	assert.NoError(t, os.Chmod(path, 0644))
	assert.NoError(t, InstallUpdate(&Options{Beta: true, Output: path}))

	info, err := os.Stat(path)
	assert.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info.Mode().Perm())

	// Must remove temporary files
	_, err = os.Stat(path + ".new")
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(path + ".old")
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	// Must contain valid executable
	assert.NoError(t, os.Chmod(path, 0755))
	cmd := exec.Command(path, "version")
	output, err := cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.True(t, cmd.ProcessState.Success())
	assert.Equal(t, betaVer, regexVer.FindString(string(output)))
}

func TestRenameOnWindows(t *testing.T) {
	if testing.Short() {
		t.Skip("not running with -short")
	}
	if runtime.GOOS != "windows" {
		t.Skip("this is a Windows only test")
	}

	// Prepare for test
	file, err := ioutil.TempFile("", "rclone-test.exe")
	assert.NoError(t, err)
	path := file.Name()
	assert.NoError(t, file.Close())
	assert.NoError(t, os.Remove(path))

	defer func() {
		_ = os.Remove(path)
		_ = os.Remove(path + ".old")
		_ = os.Remove(path + ".new")
	}()

	regexVer := regexp.MustCompile(`v[0-9]\S+`)

	stableVer, _, err := GetVersion(false, "")
	assert.NoError(t, err)

	betaVer, _, err := GetVersion(true, "")
	assert.NoError(t, err)

	// Must not create temporary files when target doesn't exist
	assert.NoError(t, InstallUpdate(&Options{Beta: true, Output: path}))

	_, err = os.Stat(path + ".new")
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(path + ".old")
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	// Must save running executable as the "old" file
	cmd := exec.Command(path, "config")
	stdin, err := cmd.StdinPipe() // Make it run waiting for input
	assert.NoError(t, err)
	assert.NoError(t, cmd.Start())

	assert.NoError(t, InstallUpdate(&Options{Beta: false, Output: path}))

	_, err = os.Stat(path + ".new")
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(path + ".old")
	assert.NoError(t, err)

	_ = stdin.Close() // End the wait
	_ = cmd.Wait()

	cmd = exec.Command(path, "version")
	output, err := cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.True(t, cmd.ProcessState.Success())
	assert.Equal(t, stableVer, regexVer.FindString(string(output)))

	assert.NoError(t, os.Remove(path))
	assert.NoError(t, os.Rename(path+".old", path))
	cmd = exec.Command(path, "version")
	output, err = cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.True(t, cmd.ProcessState.Success())
	assert.Equal(t, betaVer, regexVer.FindString(string(output)))
}
