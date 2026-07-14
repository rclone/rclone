//go:build windows

package gui

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func desktopLauncherTestNames(t *testing.T) (string, string) {
	t.Helper()
	id := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
	return `Local\rclone-gui-test-` + id, `\\.\pipe\rclone-gui-test-` + id
}

func TestDesktopLauncherReopensRunningGUI(t *testing.T) {
	mutexName, pipeName := desktopLauncherTestNames(t)
	opened := make(chan string, 1)
	oldOpenBrowser := openBrowser
	openBrowser = func(loginURL string) error {
		opened <- loginURL
		return nil
	}
	t.Cleanup(func() { openBrowser = oldOpenBrowser })

	first, alreadyRunning, err := startDesktopLauncher(mutexName, pipeName, "")
	require.NoError(t, err)
	require.False(t, alreadyRunning)
	t.Cleanup(first.Close)
	first.publishURL("http://localhost/login?token=secret")

	second, alreadyRunning, err := startDesktopLauncher(mutexName, pipeName, "")
	require.NoError(t, err)
	assert.Nil(t, second)
	assert.True(t, alreadyRunning)
	assert.Equal(t, "http://localhost/login?token=secret", <-opened)
}

func TestDesktopLauncherWaitsForURL(t *testing.T) {
	mutexName, pipeName := desktopLauncherTestNames(t)
	opened := make(chan string, 1)
	oldOpenBrowser := openBrowser
	openBrowser = func(loginURL string) error {
		opened <- loginURL
		return nil
	}
	t.Cleanup(func() { openBrowser = oldOpenBrowser })

	first, _, err := startDesktopLauncher(mutexName, pipeName, "")
	require.NoError(t, err)
	t.Cleanup(first.Close)
	result := make(chan error, 1)
	go func() {
		_, _, err := startDesktopLauncher(mutexName, pipeName, "")
		result <- err
	}()
	select {
	case err := <-result:
		t.Fatalf("second launch returned before the GUI URL was ready: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	first.publishURL("http://localhost/ready")
	require.NoError(t, <-result)
	assert.Equal(t, "http://localhost/ready", <-opened)
}

func TestDesktopLauncherReportsBrowserFailure(t *testing.T) {
	mutexName, pipeName := desktopLauncherTestNames(t)
	oldOpenBrowser := openBrowser
	openBrowser = func(string) error { return errors.New("browser unavailable") }
	t.Cleanup(func() { openBrowser = oldOpenBrowser })

	first, _, err := startDesktopLauncher(mutexName, pipeName, "")
	require.NoError(t, err)
	t.Cleanup(first.Close)
	first.publishURL("http://localhost/login")

	second, alreadyRunning, err := startDesktopLauncher(mutexName, pipeName, "")
	assert.Nil(t, second)
	assert.True(t, alreadyRunning)
	assert.EqualError(t, err, "browser unavailable")
}

func TestDesktopLauncherReleasesOwnership(t *testing.T) {
	mutexName, pipeName := desktopLauncherTestNames(t)
	first, _, err := startDesktopLauncher(mutexName, pipeName, "")
	require.NoError(t, err)
	first.Close()
	first.Close()

	next, alreadyRunning, err := startDesktopLauncher(mutexName, pipeName, "")
	require.NoError(t, err)
	require.False(t, alreadyRunning)
	require.NotNil(t, next)
	next.Close()
}

func TestDesktopLauncherConcurrentAcquisition(t *testing.T) {
	mutexName, pipeName := desktopLauncherTestNames(t)
	oldOpenBrowser := openBrowser
	openBrowser = func(string) error { return nil }
	t.Cleanup(func() { openBrowser = oldOpenBrowser })

	results := make(chan struct {
		launcher       *desktopLauncher
		alreadyRunning bool
		err            error
	}, 2)
	for range 2 {
		go func() {
			launcher, alreadyRunning, err := startDesktopLauncher(mutexName, pipeName, "")
			results <- struct {
				launcher       *desktopLauncher
				alreadyRunning bool
				err            error
			}{launcher, alreadyRunning, err}
		}()
	}
	owner := <-results
	require.NoError(t, owner.err)
	require.NotNil(t, owner.launcher)
	require.False(t, owner.alreadyRunning)
	owner.launcher.publishURL("http://localhost/login")
	t.Cleanup(owner.launcher.Close)

	reopened := <-results
	require.NoError(t, reopened.err)
	assert.Nil(t, reopened.launcher)
	assert.True(t, reopened.alreadyRunning)
}
