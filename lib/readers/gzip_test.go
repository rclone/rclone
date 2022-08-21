package readers

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type checkClose struct {
	io.Reader
	closed bool
}

func (cc *checkClose) Close() error {
	cc.closed = true
	return nil
}

func TestGzipReader(t *testing.T) {
	// Create some compressed data
	data := random.String(1000)
	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
	_, err := io.Copy(zw, bytes.NewBufferString(data))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	gzData := out.Bytes()

	// Check we can decompress it
	cc := &checkClose{Reader: bytes.NewBuffer(gzData)}
	var decompressed bytes.Buffer
	zr, err := NewGzipReader(cc)
	require.NoError(t, err)
	_, err = io.Copy(&decompressed, zr)
	require.NoError(t, err)
	assert.Equal(t, data, decompressed.String())

	// Check the underlying close gets called
	assert.False(t, cc.closed)
	require.NoError(t, zr.Close())
	assert.True(t, cc.closed)
}
