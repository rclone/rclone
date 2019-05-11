// This file implements shell exec algorithms that require binaries.
// Requires alg_gzip.go for gzip-wrapped files (cough cough lzma).
package press
import (
	"os/exec"
	"io"
	"bytes"
)

// Header we add to an exec file. We don't need this.
var EXEC_HEADER = []byte{}

// Function that gets binary paths if needed
func getBinPaths(c *Compression, mode int) (err error) {
	err = nil
	if mode == XZ_IN_GZ || mode == XZ_IN_GZ_MIN {
		c.BinPath, err = exec.LookPath(XZCommand)
	}
	return err
}

// Function that compresses a block using a shell command without wrapping in gzip. Requires an binary corresponding with the command.
func (c *Compression) compressBlockExecNogz(in []byte, out io.Writer, binaryPath string, args []string) (compressedSize uint32, uncompressedSize int64, err error) {
	// Initialize compression subprocess
	subprocess := exec.Command(binaryPath, args...)
	stdin, err := subprocess.StdinPipe()
	if err != nil {
		return 0, 0, err
	}

	// Run subprocess that creates compressed file
	go func() {
		stdin.Write(in)
		stdin.Close()
	}()

	// Get output
	output, err := subprocess.Output()
	if err != nil {
		return 0, 0, err
	}

	// Copy over and return
	n, err := io.Copy(out, bytes.NewReader(output))

	return uint32(n), int64(len(in)), err
}

// Function that compresses a block using a shell command. Requires an binary corresponding with the command.
func (c *Compression) compressBlockExecGz(in []byte, out io.Writer, binaryPath string, args []string) (compressedSize uint32, uncompressedSize int64, err error) {
	reachedEOF := false

	// Compress without gzip wrapper
	var b bytes.Buffer
	_, n, err := c.compressBlockExecNogz(in, &b, binaryPath, args)
	if err == io.EOF {
		reachedEOF = true
	} else if err != nil {
		return 0, n, err
	}

	// Store in gzip and return
	blockSize, _, err := c.compressBlockGz(b.Bytes(), out, 0)
	if reachedEOF == true && err == nil {
		err = io.EOF
	}
	return blockSize, n, err
}

// Utility function to decompress a block range using a shell command which wasn't wrapped in gzip
func decompressBlockRangeExecNogz(in io.Reader, out io.Writer, binaryPath string, args []string) (n int, err error) {
	// Decompress actual compression
	// Initialize decompression subprocess
	subprocess := exec.Command(binaryPath, args...)
	stdin, err := subprocess.StdinPipe()
	if err != nil {
		return 0, err
	}

	// Run subprocess that copies over compressed block
	go func() {
		defer stdin.Close()
		io.Copy(stdin, in)
	}()

	// Get output, copy, and return
	output, err := subprocess.Output()
	if err != nil {
		return 0, err
	}
	n64, err := io.Copy(out, bytes.NewReader(output))
	return int(n64), err
}

// Utility function to decompress a block range using a shell command
func decompressBlockRangeExecGz(in io.Reader, out io.Writer, binaryPath string, args []string) (n int, err error) {
	// "Decompress" gzip (this should be in store mode)
	var b bytes.Buffer
	_, err = decompressBlockRangeGz(in, &b)
	if err != nil {
		return 0, err
	}

	// Decompress actual compression
	return decompressBlockRangeExecNogz(&b, out, binaryPath, args)
}