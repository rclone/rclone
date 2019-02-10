package operations

import (
	"io"
	"io/ioutil"
	"testing"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/fstest/mockobject"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

// check interface
var _ io.ReadCloser = (*reOpen)(nil)

var errorTestError = errors.New("test error")

// this is a wrapper for an mockobject with a custom Open function
//
// breaks indicate the number of bytes to read before returning an
// error
type reOpenTestObject struct {
	fs.Object
	breaks []int64
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
//
// This will break after reading the number of bytes in breaks
func (o *reOpenTestObject) Open(options ...fs.OpenOption) (io.ReadCloser, error) {
	rc, err := o.Object.Open(options...)
	if err != nil {
		return nil, err
	}
	if len(o.breaks) > 0 {
		// Pop a breakpoint off
		N := o.breaks[0]
		o.breaks = o.breaks[1:]
		// If 0 then return an error immediately
		if N == 0 {
			return nil, errorTestError
		}
		// Read N bytes then an error
		r := io.MultiReader(&io.LimitedReader{R: rc, N: N}, errorReader{errorTestError})
		// Wrap with Close in a new readCloser
		rc = readCloser{Reader: r, Closer: rc}
	}
	return rc, nil
}

// Return an error only
type errorReader struct {
	err error
}

// Read returning an error
func (er errorReader) Read(p []byte) (n int, err error) {
	return 0, er.err
}

// Contents for the mock object
var reOpenTestcontents = []byte("0123456789")

// Start the test with the given breaks
func testReOpen(breaks []int64, maxRetries int) (io.ReadCloser, error) {
	srcOrig := mockobject.New("potato").WithContent(reOpenTestcontents, mockobject.SeekModeRegular)
	src := &reOpenTestObject{
		Object: srcOrig,
		breaks: breaks,
	}
	hashOption := &fs.HashesOption{Hashes: hash.NewHashSet(hash.MD5)}
	return newReOpen(src, hashOption, maxRetries)
}

func TestReOpenBasics(t *testing.T) {
	// open
	h, err := testReOpen(nil, 10)
	assert.NoError(t, err)

	// Check contents read correctly
	got, err := ioutil.ReadAll(h)
	assert.NoError(t, err)
	assert.Equal(t, reOpenTestcontents, got)

	// Check read after end
	var buf = make([]byte, 1)
	n, err := h.Read(buf)
	assert.Equal(t, 0, n)
	assert.Equal(t, io.EOF, err)

	// Check close
	assert.NoError(t, h.Close())

	// Check double close
	assert.Equal(t, errorFileClosed, h.Close())

	// Check read after close
	n, err = h.Read(buf)
	assert.Equal(t, 0, n)
	assert.Equal(t, errorFileClosed, err)
}

func TestReOpenErrorAtStart(t *testing.T) {
	// open with immediate breaking
	h, err := testReOpen([]int64{0}, 10)
	assert.Equal(t, errorTestError, err)
	assert.Nil(t, h)
}

func TestReOpenError(t *testing.T) {
	// open with a few break points but less than the max
	h, err := testReOpen([]int64{2, 1, 3}, 10)
	assert.NoError(t, err)

	// check contents
	got, err := ioutil.ReadAll(h)
	assert.NoError(t, err)
	assert.Equal(t, reOpenTestcontents, got)

	// check close
	assert.NoError(t, h.Close())
}

func TestReOpenFail(t *testing.T) {
	// open with a few break points but >= the max
	h, err := testReOpen([]int64{2, 1, 3}, 3)
	assert.NoError(t, err)

	// check contents
	got, err := ioutil.ReadAll(h)
	assert.Equal(t, errorTestError, err)
	assert.Equal(t, reOpenTestcontents[:6], got)

	// check old error is returned
	var buf = make([]byte, 1)
	n, err := h.Read(buf)
	assert.Equal(t, 0, n)
	assert.Equal(t, errorTooManyTries, err)

	// Check close
	assert.Equal(t, errorFileClosed, h.Close())
}
