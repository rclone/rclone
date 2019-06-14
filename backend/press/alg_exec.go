package press

// This file implements shell exec algorithms that require binaries.
import (
	"bytes"
	"io"
	"os/exec"
)

// XZ command
const xzcommand = "xz" // Name of xz binary (if available)

// ExecHeader - Header we add to an exec file. We don't need this.
var ExecHeader = []byte{}

// Function that checks whether XZ is present in the system
func checkXZ() bool {
	_, err := exec.LookPath("xz")
	if err != nil {
		return false
	}
	return true
}

// Function that gets binary paths if needed
func getBinPaths(c *Compression, mode int) (err error) {
	err = nil
	if mode == XZMin || mode == XZDefault {
		c.BinPath, err = exec.LookPath(xzcommand)
	}
	return err
}

// Function that compresses a block using a shell command without wrapping in gzip. Requires an binary corresponding with the command.
func (c *Compression) compressBlockExec(in []byte, out io.Writer, binaryPath string, args []string) (compressedSize uint32, uncompressedSize int64, err error) {
	// Initialize compression subprocess
	subprocess := exec.Command(binaryPath, args...)
	stdin, err := subprocess.StdinPipe()
	if err != nil {
		return 0, 0, err
	}

	// Run subprocess that creates compressed file
	stdinError := make(chan error)
	go func() {
		_, err := stdin.Write(in)
		_ = stdin.Close()
		stdinError <- err
	}()

	// Get output
	output, err := subprocess.Output()
	if err != nil {
		return 0, 0, err
	}

	// Copy over
	n, err := io.Copy(out, bytes.NewReader(output))
	if err != nil {
		return uint32(n), int64(len(in)), err
	}

	// Check if there was an error and return
	err = <-stdinError

	return uint32(n), int64(len(in)), err
}

// Utility function to decompress a block range using a shell command which wasn't wrapped in gzip
func decompressBlockRangeExec(in io.Reader, out io.Writer, binaryPath string, args []string) (n int, err error) {
	// Decompress actual compression
	// Initialize decompression subprocess
	subprocess := exec.Command(binaryPath, args...)
	stdin, err := subprocess.StdinPipe()
	if err != nil {
		return 0, err
	}

	// Run subprocess that copies over compressed block
	stdinError := make(chan error)
	go func() {
		_, err := io.Copy(stdin, in)
		_ = stdin.Close()
		stdinError <- err
	}()

	// Get output, copy, and return
	output, err := subprocess.Output()
	if err != nil {
		return 0, err
	}
	n64, err := io.Copy(out, bytes.NewReader(output))
	if err != nil {
		return int(n64), err
	}
	err = <-stdinError
	return int(n64), err
}
