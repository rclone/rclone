package operations

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type interruptReader struct {
	once sync.Once
	r    io.Reader
}

// Read sends an OS specific interrupt signal and then reads 1 byte at a time
func (r *interruptReader) Read(b []byte) (n int, err error) {
	r.once.Do(func() {
		_ = sendInterrupt()
	})
	buffer := make([]byte, 1)
	n, err = r.r.Read(buffer)
	b[0] = buffer[0]
	// Simulate duration of a larger read without needing to test with a large file
	// Allows for the interrupt to be handled before Copy completes
	time.Sleep(time.Microsecond * 10)
	return n, err
}

// this is a wrapper for a mockobject with a custom Open function
//
// n indicates the number of bytes to read before sending an
// interrupt signal
type resumeTestObject struct {
	fs.Object
	n int64
}

// Open opens the file for read. Call Close() on the returned io.ReadCloser
//
// The Reader will signal an interrupt after reading n bytes, then continue to read 1 byte at a time.
// If TestResume is successful, the interrupt will be processed and reads will be cancelled before running
// out of bytes to read
func (o *resumeTestObject) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	rc, err := o.Object.Open(ctx, options...)
	if err != nil {
		return nil, err
	}
	r := io.MultiReader(&io.LimitedReader{R: rc, N: o.n}, &interruptReader{r: rc})
	// Wrap with Close in a new readCloser
	rc = readCloser{Reader: r, Closer: rc}
	return rc, nil
}

func makeContent(t *testing.T, size int) []byte {
	content := make([]byte, size)
	r := rand.New(rand.NewSource(42))
	_, err := io.ReadFull(r, content)
	assert.NoError(t, err)
	return content
}

func TestResume(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	ci := fs.GetConfig(ctx)
	ci.ResumeLarger = 0

	// Contents for the mock object
	var (
		// Test contents must be large enough that io.Copy does not complete during the first Rclone Copy operation
		resumeTestContents = makeContent(t, 1024)
		expectedContents   = resumeTestContents
	)

	// Create mockobjects with given breaks
	createTestSrc := func(interrupt int64) (fs.Object, fs.Object) {
		srcOrig := mockobject.New("potato").WithContent(resumeTestContents, mockobject.SeekModeNone)
		srcOrig.SetFs(r.Flocal)
		src := &resumeTestObject{
			Object: srcOrig,
			n:      interrupt,
		}
		return src, srcOrig
	}

	checkContents := func(obj fs.Object, contents string) {
		assert.NotNil(t, obj)
		assert.Equal(t, int64(len(contents)), obj.Size())

		r, err := obj.Open(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, r)
		if r == nil {
			return
		}
		data, err := ioutil.ReadAll(r)
		assert.NoError(t, err)
		assert.Equal(t, contents, string(data))
		_ = r.Close()
	}

	srcBreak, srcNoBreak := createTestSrc(2)

	// Run first Copy only in a subprocess so that it can be interrupted without ending the test
	// adapted from: https://stackoverflow.com/questions/26225513/how-to-test-os-exit-scenarios-in-go
	if os.Getenv("RUNTEST") == "1" {
		remoteRoot := os.Getenv("REMOTEROOT")
		remoteFs, err := fs.NewFs(ctx, remoteRoot)
		require.NoError(t, err)
		_, _ = Copy(ctx, remoteFs, nil, "testdst", srcBreak)
		// This should never be reached as the subroutine should exit during Copy
		require.True(t, false, "Problem with test, first Copy operation should've been interrupted before completion")
		return
	}
	// Start the subprocess
	cmd := exec.Command(os.Args[0], "-test.run=TestResume")
	cmd.Env = append(os.Environ(), "RUNTEST=1", "REMOTEROOT="+r.Fremote.Root())
	cmd.Stdout = os.Stdout
	setupCmd(cmd)
	err := cmd.Run()

	e, ok := err.(*exec.ExitError)

	// Exit code after signal will be (128+signum) on Linux or (signum) on Windows
	expectedErrorString := "exit status 1"
	if runtime.GOOS == "windows" {
		expectedErrorString = "exit status 2"
	}
	assert.True(t, ok)
	assert.Contains(t, e.Error(), expectedErrorString)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(os.Stderr)
	}()

	// Start copy again, but with no breaks
	newDst, err := Copy(ctx, r.Fremote, nil, "testdst", srcNoBreak)
	assert.NoError(t, err)

	// Checks to see if a resume was initiated
	// Resumed byte position can vary slightly depending how long it takes atexit to process the interrupt
	assert.True(t, strings.Contains(buf.String(), "Resuming at byte position: "), "The upload did not resume when restarted. Message: %q", buf.String())

	checkContents(newDst, string(expectedContents))
}
