package readers

import (
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatternReader(t *testing.T) {
	b2 := make([]byte, 1)

	r := NewPatternReader(0)
	b, err := ioutil.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, []byte{}, b)
	n, err := r.Read(b2)
	require.Equal(t, io.EOF, err)
	require.Equal(t, 0, n)

	r = NewPatternReader(10)
	b, err = ioutil.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, b)
	n, err = r.Read(b2)
	require.Equal(t, io.EOF, err)
	require.Equal(t, 0, n)
}

func TestPatternReaderSeek(t *testing.T) {
	r := NewPatternReader(1024)
	b, err := ioutil.ReadAll(r)
	require.NoError(t, err)

	for i := range b {
		assert.Equal(t, byte(i%251), b[i])
	}

	n, err := r.Seek(1, io.SeekStart)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	// pos 1

	b2 := make([]byte, 10)
	nn, err := r.Read(b2)
	require.NoError(t, err)
	assert.Equal(t, 10, nn)
	assert.Equal(t, b[1:11], b2)

	// pos 11

	n, err = r.Seek(9, io.SeekCurrent)
	require.NoError(t, err)
	assert.Equal(t, int64(20), n)

	// pos 20

	nn, err = r.Read(b2)
	require.NoError(t, err)
	assert.Equal(t, 10, nn)
	assert.Equal(t, b[20:30], b2)

	n, err = r.Seek(-24, io.SeekEnd)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), n)

	// pos 1000

	nn, err = r.Read(b2)
	require.NoError(t, err)
	assert.Equal(t, 10, nn)
	assert.Equal(t, b[1000:1010], b2)

	// Now test errors

	n, err = r.Seek(1, 400)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid whence")
	assert.Equal(t, int64(0), n)

	n, err = r.Seek(-1, io.SeekStart)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "negative position")
	assert.Equal(t, int64(0), n)
}
