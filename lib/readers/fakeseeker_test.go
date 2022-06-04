package readers

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check interface
var _ io.ReadSeeker = &FakeSeeker{}

func TestFakeSeeker(t *testing.T) {
	// Test that passing in an io.ReadSeeker just passes it through
	bufReader := bytes.NewReader([]byte{1})
	r := NewFakeSeeker(bufReader, 5)
	assert.Equal(t, r, bufReader)

	in := bytes.NewBufferString("hello")
	buf := make([]byte, 16)
	r = NewFakeSeeker(in, 5)
	assert.NotEqual(t, r, in)

	// check the seek offset is as passed in
	checkPos := func(pos int64) {
		abs, err := r.Seek(0, io.SeekCurrent)
		require.NoError(t, err)
		assert.Equal(t, pos, abs)
	}

	// Test some seeking
	checkPos(0)

	abs, err := r.Seek(2, io.SeekStart)
	require.NoError(t, err)
	assert.Equal(t, int64(2), abs)
	checkPos(2)

	abs, err = r.Seek(-1, io.SeekEnd)
	require.NoError(t, err)
	assert.Equal(t, int64(4), abs)
	checkPos(4)

	// Check can't read if not at start
	_, err = r.Read(buf)
	require.ErrorContains(t, err, "not at start")

	// Seek back to start
	abs, err = r.Seek(-4, io.SeekCurrent)
	require.NoError(t, err)
	assert.Equal(t, int64(0), abs)
	checkPos(0)

	_, err = r.Seek(42, 17)
	require.ErrorContains(t, err, "invalid whence")

	_, err = r.Seek(-1, io.SeekStart)
	require.ErrorContains(t, err, "negative position")

	// Test reading now seeked back to the start
	n, err := r.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, []byte("hello"), buf[:5])

	// Seeking should give an error now
	_, err = r.Seek(-1, io.SeekEnd)
	require.ErrorContains(t, err, "after reading")
}

func TestFakeSeekerError(t *testing.T) {
	in := bytes.NewBufferString("hello")
	r := NewFakeSeeker(in, 5)
	assert.NotEqual(t, r, in)

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), buf)

	_, err = r.Read(buf)
	assert.Equal(t, io.EOF, err)

	_, err = r.Seek(0, io.SeekStart)
	assert.Equal(t, io.EOF, err)
}
