package readers

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

var errRead = errors.New("read error")

type readOnly struct{}

func (readOnly) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

type readClose struct{}

func (readClose) Read(p []byte) (n int, err error) {
	return 0, errRead
}

func (readClose) Close() (err error) {
	return io.EOF
}

type readCloseWriteTo struct {
	readClose
}

func (readCloseWriteTo) WriteTo(w io.Writer) (n int64, err error) {
	return 42, errRead
}

func TestNoCloser(t *testing.T) {
	assert.Equal(t, nil, NoCloser(nil))

	ro := readOnly{}
	assert.Equal(t, ro, NoCloser(ro))

	rc := readClose{}
	nc := NoCloser(rc)
	assert.NotEqual(t, nc, rc)

	_, hasClose := nc.(io.Closer)
	assert.False(t, hasClose)

	_, err := nc.Read(nil)
	assert.Equal(t, errRead, err)

	_, hasWriteTo := nc.(io.WriterTo)
	assert.False(t, hasWriteTo)

	rcw := readCloseWriteTo{}
	ncw := NoCloser(rcw)
	assert.NotEqual(t, ncw, rcw)

	_, hasClose = ncw.(io.Closer)
	assert.False(t, hasClose)

	wt, hasWriteTo := ncw.(io.WriterTo)
	assert.True(t, hasWriteTo)
	n, err := wt.WriteTo(nil)
	assert.Equal(t, int64(42), n)
	assert.Equal(t, errRead, err)
}
