package cmd

import (
	"os"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/config"
)

// TestPrintProgressOutputTarget verifies that progress output written by
// printProgress goes to config.PasswordPromptOutput (the terminal attached
// to stderr, preserved even when --log-file redirects stderr) and NOT to
// stdout, so data written to stdout by commands like "cat" is never
// intermixed with the progress display.
func TestPrintProgressOutputTarget(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	defer func() {
		_ = r.Close()
		_ = w.Close()
	}()

	old := config.PasswordPromptOutput
	config.PasswordPromptOutput = w
	defer func() { config.PasswordPromptOutput = old }()

	readDone := make(chan int, 1)
	go func() {
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		readDone <- n
	}()

	printProgress("some log message")

	select {
	case n := <-readDone:
		if n == 0 {
			t.Errorf("printProgress wrote no output to PasswordPromptOutput")
		}
	case <-time.After(2 * time.Second):
		t.Errorf("printProgress wrote nothing to PasswordPromptOutput (still writing progress to stdout?)")
	}
}
