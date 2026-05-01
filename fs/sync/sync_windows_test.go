//go:build windows

package sync

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Spawn a new process that holds an exclusive lock on the specified file.
// Blocks until the lock is acquired, then returns a cleanup function to release the lock (also blocking) when called.
func createExclusiveFileLock(t *testing.T, filePath string) func() {
	// Re-exec the same binary
	lockCmd := exec.Command(os.Args[0], "-test.run=^TestFileLockHelper$", "-test.v")
	lockCmd.Env = append(os.Environ(), "IS_LOCK_HOLDER=1", "FILE_TO_LOCK="+filePath)

	// Set up pipes for communicating with the helper proc
	stdout, err := lockCmd.StdoutPipe()
	require.NoError(t, err, "failed to create helper stdout pipe")
	lockStdout, err := lockCmd.StdinPipe()
	require.NoError(t, err, "failed to create helper stdin pipe")

	err = lockCmd.Start()
	require.NoError(t, err, "failed to start lock holder process")
	cleanupLockHelper := func() {
		// Signal to the helper to release the lock, then wait for it to exit
		if lockStdout != nil {
			_, _ = lockStdout.Write([]byte("release\n"))
			_ = lockStdout.Close()
			lockStdout = nil // don't try to clean up twice
		}
		if lockCmd != nil && lockCmd.Process != nil {
			_ = lockCmd.Wait()
			lockCmd = nil // don't try to clean up twice
		}
		// Wait for the helper to signal that it has released the lock
		// todo(maxgreen) comment out these logs
		t.Log("Waiting for file lock to be released...")
		awaitChildOutput(t, stdout, "finished")
		_ = lockStdout.Close()
		lockStdout = nil // don't try to clean up twice
	}

	// Wait for lock to be acquired with timeout
	t.Log("Waiting for file lock to be acquired...")
	awaitChildOutput(t, stdout, "locked")

	// Make sure the file is actually locked
	_, err = os.OpenFile(filePath, os.O_RDONLY, 0)
	require.Error(t, err, "file should be locked by helper process")

	return cleanupLockHelper
}

// Block until the child process sends a signal by printing a newline-terminated string to its stdout
func awaitChildOutput(t *testing.T, stdout io.Reader, signal string) {
	t.Helper()
	if stdout == nil {
		return
	}
	outputChan := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(stdout)
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				if readErr == io.EOF {
					outputChan <- fmt.Errorf("file locking process exited before signaling: %w", readErr)
				} else {
					outputChan <- fmt.Errorf("error reading output from file locking process: %w", readErr)
				}
				return
			}
			if strings.Contains(line, signal) { // helper has sent the signal string
				outputChan <- nil
				return
			}
		}
	}()

	select {
	case err := <-outputChan:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for file locking process to send signal %q", signal)
	}
}

// Helper function that only runs in a separate child process to hold an exclusive lock on a file until signaled to release it
func holdExclusiveFileLock(t *testing.T, filePath string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		fmt.Fprint(os.Stderr, "Exclusive file locking is only supported on Windows")
		os.Exit(1)
	}

	pathp, err := syscall.UTF16PtrFromString(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while setting up file lock: %v\n", err)
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
		fmt.Fprintf(os.Stderr, "Error while acquiring file lock: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("locked") // signal to the parent that lock is ready

	// Wait for release signal on stdin or process termination
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "release") {
			break
		}
	}
	
	// Release the lock and exit
	if err := syscall.CloseHandle(handle); err != nil {
		fmt.Fprintf(os.Stderr, "Error while releasing file lock: %v\n", err)
	}
	fmt.Println("finished") // signal to the parent that lock is released
}
