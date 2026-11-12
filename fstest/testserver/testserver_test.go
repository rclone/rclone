package testserver

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupFakeServer points the testserver at a temporary init.d holding
// a TestFake server script which records when it is stopped.
func setupFakeServer(t *testing.T) (stopped string) {
	if runtime.GOOS == "windows" {
		t.Skip("init.d scripts need a POSIX shell")
	}
	dir := t.TempDir()
	stopped = filepath.Join(dir, "stopped")
	script := `#!/bin/sh
case "$1" in
start) echo type=memory ;;
stop) touch "` + stopped + `" ;;
esac
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "TestFake"), []byte(script), 0o755))
	findConfigOnce.Do(func() {})
	oldConfigDir := configDir
	configDir = dir
	t.Cleanup(func() { configDir = oldConfigDir })
	return stopped
}

func TestStartWrapped(t *testing.T) {
	stopped := setupFakeServer(t)
	t.Setenv("RCLONE_CONFIG_TESTFAKE_TYPE", "")
	t.Setenv("RCLONE_CONFIG_TESTWRAP_TYPE", "chunker")
	t.Setenv("RCLONE_CONFIG_TESTWRAP_REMOTE", "TestFake:")
	t.Setenv("RCLONE_CONFIG_TESTWRAPWRAP_TYPE", "crypt")
	t.Setenv("RCLONE_CONFIG_TESTWRAPWRAP_REMOTE", "TestWrap:sub/dir")

	finish, err := Start("TestWrapWrap:")
	require.NoError(t, err)
	assert.Equal(t, "memory", os.Getenv("RCLONE_CONFIG_TESTFAKE_TYPE"))
	assert.NoFileExists(t, stopped)

	finish()
	assert.FileExists(t, stopped)
}

func TestStartWrappedLoop(t *testing.T) {
	stopped := setupFakeServer(t)
	t.Setenv("RCLONE_CONFIG_TESTLOOP_TYPE", "chunker")
	t.Setenv("RCLONE_CONFIG_TESTLOOP_REMOTE", "TestLoop:")

	finish, err := Start("TestLoop:")
	require.NoError(t, err)
	finish()
	assert.NoFileExists(t, stopped)
}
