package squashfs

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

// panickingReader panics on Read, standing in for the go-diskfs parser
// hitting an unchecked slice while reading a malformed image.
type panickingReader struct{}

func (panickingReader) Read([]byte) (int, error) { panic("boom") }
func (panickingReader) Close() error             { return nil }

// Test that a panic raised while reading file data becomes an error. The
// parser reads lazily, so an image can open cleanly and only panic here.
func TestRecoveringReadCloser(t *testing.T) {
	var rc io.ReadCloser = recoveringReadCloser{panickingReader{}}

	_, err := rc.Read(make([]byte, 16))
	assert.ErrorContains(t, err, "invalid or corrupt squashfs image")
	assert.ErrorContains(t, err, "boom")
}
