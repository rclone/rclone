package readers

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepeatableReader(t *testing.T) {
	var dst []byte
	var n int
	var pos int64
	var err error

	b := []byte("Testbuffer")
	buf := bytes.NewBuffer(b)
	r := NewRepeatableReader(buf)

	dst = make([]byte, 100)
	n, err = r.Read(dst)
	assert.Nil(t, err)
	assert.Equal(t, 10, n)
	require.Equal(t, b, dst[0:10])

	// Test read EOF
	n, err = r.Read(dst)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, n)

	// Test Seek Back to start
	dst = make([]byte, 10)
	pos, err = r.Seek(0, io.SeekStart)
	assert.Nil(t, err)
	require.Equal(t, 0, int(pos))

	n, err = r.Read(dst)
	assert.Nil(t, err)
	assert.Equal(t, 10, n)
	require.Equal(t, b, dst)

	// Test partial read
	buf = bytes.NewBuffer(b)
	r = NewRepeatableReader(buf)
	dst = make([]byte, 5)
	n, err = r.Read(dst)
	assert.Nil(t, err)
	assert.Equal(t, 5, n)
	require.Equal(t, b[0:5], dst)
	n, err = r.Read(dst)
	assert.Nil(t, err)
	assert.Equal(t, 5, n)
	require.Equal(t, b[5:], dst)

	// Test Seek
	buf = bytes.NewBuffer(b)
	r = NewRepeatableReader(buf)
	// Should not allow seek past cache index
	pos, err = r.Seek(5, io.SeekCurrent)
	assert.NotNil(t, err)
	assert.Equal(t, "fs.RepeatableReader.Seek: offset is unavailable", err.Error())
	assert.Equal(t, 0, int(pos))

	// Should not allow seek to negative position start
	pos, err = r.Seek(-1, io.SeekCurrent)
	assert.NotNil(t, err)
	assert.Equal(t, "fs.RepeatableReader.Seek: negative position", err.Error())
	assert.Equal(t, 0, int(pos))

	// Should not allow seek with invalid whence
	pos, err = r.Seek(0, 3)
	assert.NotNil(t, err)
	assert.Equal(t, "fs.RepeatableReader.Seek: invalid whence", err.Error())
	assert.Equal(t, 0, int(pos))

	// Should seek from index with io.SeekCurrent(1) whence
	dst = make([]byte, 5)
	_, _ = r.Read(dst)
	pos, err = r.Seek(-3, io.SeekCurrent)
	assert.Nil(t, err)
	require.Equal(t, 2, int(pos))
	pos, err = r.Seek(1, io.SeekCurrent)
	assert.Nil(t, err)
	require.Equal(t, 3, int(pos))

	// Should seek from cache end with io.SeekEnd(2) whence
	pos, err = r.Seek(-3, io.SeekEnd)
	assert.Nil(t, err)
	require.Equal(t, 2, int(pos))

	// Should read from seek position and past it
	dst = make([]byte, 5)
	n, err = io.ReadFull(r, dst)
	assert.Nil(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, b[2:7], dst)

}
