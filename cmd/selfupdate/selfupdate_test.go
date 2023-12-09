//go:build !noselfupdate
// +build !noselfupdate

package selfupdate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	_ "github.com/rclone/rclone/fstest" // needed to run under integration tests
	"github.com/rclone/rclone/fstest/testy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetVersion(t *testing.T) {
	testy.SkipUnreliable(t)

	ctx := context.Background()

	// a beta version can only have "v" prepended
	resultVer, _, err := GetVersion(ctx, true, "1.2.3.4")
	assert.NoError(t, err)
	assert.Equal(t, "v1.2.3.4", resultVer)

	// but a stable version syntax should be checked
	_, _, err = GetVersion(ctx, false, "1")
	assert.Error(t, err)
	_, _, err = GetVersion(ctx, false, "1.")
	assert.Error(t, err)
	_, _, err = GetVersion(ctx, false, "1.2.")
	assert.Error(t, err)
	_, _, err = GetVersion(ctx, false, "1.2.3.4")
	assert.Error(t, err)

	// incomplete stable version should have micro release added
	resultVer, _, err = GetVersion(ctx, false, "1.52")
	assert.NoError(t, err)
	assert.Equal(t, "v1.52.3", resultVer)
}

func TestInstallOnLinux(t *testing.T) {
	testy.SkipUnreliable(t)
	if runtime.GOOS != "linux" {
		t.Skip("this is a Linux only test")
	}

	// Prepare for test
	ctx := context.Background()
	testDir := t.TempDir()
	path := filepath.Join(testDir, "rclone")

	regexVer := regexp.MustCompile(`v[0-9]\S+`)

	betaVer, _, err := GetVersion(ctx, true, "")
	assert.NoError(t, err)

	// Must do nothing if version isn't changing
	assert.NoError(t, InstallUpdate(ctx, &Options{Beta: true, Output: path, Version: fs.Version}))

	// Must fail on non-writable file
	assert.NoError(t, os.WriteFile(path, []byte("test"), 0644))
	assert.NoError(t, os.Chmod(path, 0000))
	defer func() {
		_ = os.Chmod(path, 0644)
	}()
	err = (InstallUpdate(ctx, &Options{Beta: true, Output: path}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "run self-update as root")

	// Must keep non-standard permissions
	assert.NoError(t, os.Chmod(path, 0644))
	require.NoError(t, InstallUpdate(ctx, &Options{Beta: true, Output: path}))

	info, err := os.Stat(path)
	assert.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info.Mode().Perm())

	// Must remove temporary files
	files, err := os.ReadDir(testDir)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(files))

	// Must contain valid executable
	assert.NoError(t, os.Chmod(path, 0755))
	cmd := exec.Command(path, "version")
	output, err := cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.True(t, cmd.ProcessState.Success())
	assert.Equal(t, betaVer, regexVer.FindString(string(output)))
}

func TestRenameOnWindows(t *testing.T) {
	testy.SkipUnreliable(t)
	if runtime.GOOS != "windows" {
		t.Skip("this is a Windows only test")
	}

	// Prepare for test
	ctx := context.Background()

	testDir := t.TempDir()

	path := filepath.Join(testDir, "rclone.exe")
	regexVer := regexp.MustCompile(`v[0-9]\S+`)

	stableVer, _, err := GetVersion(ctx, false, "")
	assert.NoError(t, err)

	betaVer, _, err := GetVersion(ctx, true, "")
	assert.NoError(t, err)

	// Must not create temporary files when target doesn't exist
	assert.NoError(t, InstallUpdate(ctx, &Options{Beta: true, Output: path}))

	files, err := os.ReadDir(testDir)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(files))

	// Must save running executable as the "old" file
	cmdWait := exec.Command(path, "config")
	stdinWait, err := cmdWait.StdinPipe() // Make it run waiting for input
	assert.NoError(t, err)
	assert.NoError(t, cmdWait.Start())

	assert.NoError(t, InstallUpdate(ctx, &Options{Beta: false, Output: path}))
	files, err = os.ReadDir(testDir)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(files))

	pathOld := filepath.Join(testDir, "rclone.old.exe")
	_, err = os.Stat(pathOld)
	assert.NoError(t, err)

	cmd := exec.Command(path, "version")
	output, err := cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.True(t, cmd.ProcessState.Success())
	assert.Equal(t, stableVer, regexVer.FindString(string(output)))

	cmdOld := exec.Command(pathOld, "version")
	output, err = cmdOld.CombinedOutput()
	assert.NoError(t, err)
	assert.True(t, cmdOld.ProcessState.Success())
	assert.Equal(t, betaVer, regexVer.FindString(string(output)))

	// Stop previous waiting executable, run new and saved executables
	_ = stdinWait.Close()
	_ = cmdWait.Wait()
	time.Sleep(100 * time.Millisecond)

	cmdWait = exec.Command(path, "config")
	stdinWait, err = cmdWait.StdinPipe()
	assert.NoError(t, err)
	assert.NoError(t, cmdWait.Start())

	cmdWaitOld := exec.Command(pathOld, "config")
	stdinWaitOld, err := cmdWaitOld.StdinPipe()
	assert.NoError(t, err)
	assert.NoError(t, cmdWaitOld.Start())

	// Updating when the "old" executable is running must produce a random "old" file
	assert.NoError(t, InstallUpdate(ctx, &Options{Beta: true, Output: path}))
	files, err = os.ReadDir(testDir)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(files))

	// Stop all waiting executables
	_ = stdinWait.Close()
	_ = cmdWait.Wait()
	_ = stdinWaitOld.Close()
	_ = cmdWaitOld.Wait()
	time.Sleep(100 * time.Millisecond)
}
