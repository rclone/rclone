//go:build windows

package sync

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

// Helper function that only runs in a separate child process to lock a file for testing purposes
func lockFileExclusive(t *testing.T, filePath string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("exclusive file locking is only supported on Windows")
	}

	pathp, err := syscall.UTF16PtrFromString(filePath)
	if err != nil {
		os.Exit(1)
	}

	handle, err := syscall.CreateFile(
		pathp,
		syscall.GENERIC_READ,
		0, // exclusive lock
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		os.Exit(1)
	}
	defer syscall.CloseHandle(handle)
	fmt.Println("locked") // signal to the parent that lock is ready

	// Wait for release signal on stdin or process termination
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "release") {
			break
		}
	}
	// Lock is released by deferred CloseHandle when we exit
}
