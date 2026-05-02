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
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Spawn a new process that holds an exclusive lock on the specified file.
// Blocks until the lock is acquired, then returns a cleanup function to release the lock (also blocking) when called.
func createExclusiveFileLock(t *testing.T, filePath string) func() {
	// Re-exec the same binary
	lockCmd := exec.Command(os.Args[0], "-test.run=^TestFileLockHelper$", "-test.v")
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "IS_LOCK_HOLDER=") || strings.HasPrefix(e, "FILE_TO_LOCK=") {
			t.Fatalf("Environment variable %q should not be set in parent process", e)
		}
	}
	lockCmd.Env = append(os.Environ(), "IS_LOCK_HOLDER=1", "FILE_TO_LOCK="+filePath)

	// Set up pipes and buffers for communicating with the helper proc
	stdout, err := lockCmd.StdoutPipe()
	require.NoError(t, err, "failed to create helper stdout pipe")
	stdoutReader := bufio.NewReader(stdout)
	lockStdin, err := lockCmd.StdinPipe()
	require.NoError(t, err, "failed to create helper stdin pipe")

	err = lockCmd.Start()
	require.NoError(t, err, "failed to start lock holder process")

	var once sync.Once
	cleanupLockHelper := func() {
		once.Do(func() {
			// todo(maxgreen01) remove
			// check if the helper terminated unexpectedly before we signal??
			if lockCmd.ProcessState != nil && lockCmd.ProcessState.Exited() {
				assert.Fail(t, "lock helper process should still be running when cleanup is called")
				return
			}
			
			// Signal to the helper to release the lock and exit
			if lockStdin != nil {
				_, _ = lockStdin.Write([]byte("release\n"))
				_ = lockStdin.Close()
			}

			// Wait for the helper to signal that it has released the lock
			// todo(maxgreen) comment out these logs
			t.Log("Waiting for file lock to be released...")
			awaitChildOutput(t, stdoutReader, "finished")

			// Wait for the helper process to exit
			err = lockCmd.Wait()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					assert.Failf(t, "lock helper process exited with error: %s", string(exitErr.Stderr))
				} else {
					assert.Fail(t, "lock helper process should exit cleanly")
				}
			}

			t.Log("lock should have been released")
			// Make sure the file is actually accessible again
			err := os.Rename(filePath, filePath)
			assert.NoError(t, err, "file should not be locked by helper process anymore")
		})
	}
	// Run cleanup in case of failure, even if it's already called manually later
	t.Cleanup(cleanupLockHelper)

	// Wait for lock to be acquired with timeout
	t.Log("Waiting for file lock to be acquired...")
	awaitChildOutput(t, stdoutReader, "locked")
	t.Log("lock should be acquired...")

	// Make sure the file is actually locked and the helper process is still alive
	err = os.Rename(filePath, filePath)
	assert.Error(t, err, "file should be locked by helper process")
	assert.Nil(t, lockCmd.ProcessState, "lock helper process should still be running")

	return cleanupLockHelper
}

// Block until the child process sends a signal by printing a newline-terminated string to its stdout
func awaitChildOutput(t *testing.T, stdoutReader *bufio.Reader, signal string) {
	t.Helper()
	if stdoutReader == nil {
		return
	}
	// Receive the signal from a separate goroutine
	outputChan := make(chan error, 1)
	go func() {
		for {
			line, readErr := stdoutReader.ReadString('\n')
			if readErr != nil {
				if readErr == io.EOF {
					outputChan <- fmt.Errorf("file locking process exited before signaling: %w", readErr)
				} else {
					outputChan <- fmt.Errorf("error reading output from file locking process: %w", readErr)
				}
				return
			}
			if strings.TrimSpace(line) == signal { // helper has sent the signal string
				outputChan <- nil
				return
			}
		}
	}()

	// Wait for the signal
	select {
	case err := <-outputChan:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for file locking process to send signal %q", signal)
	}
	// time.Sleep(1 * time.Second) // make sure its done //todo(maxgreen01) remove
}

// Helper function that only runs in a separate child process to hold an exclusive lock on a file until signaled to release it
func holdExclusiveFileLock(t *testing.T, filePath string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		fmt.Fprintln(os.Stderr, "Exclusive file locking is only supported on Windows")
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
		os.Exit(1)
	}
	fmt.Println("finished") // signal to the parent that lock is released
}
