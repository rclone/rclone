package raid3

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigToFooterCompression(t *testing.T) {
	none, err := ConfigToFooterCompression("none")
	require.NoError(t, err)
	assert.Equal(t, CompressionNone, none)

	empty, err := ConfigToFooterCompression("")
	require.NoError(t, err)
	assert.Equal(t, CompressionNone, empty)

	snappy, err := ConfigToFooterCompression("snappy")
	require.NoError(t, err)
	assert.Equal(t, CompressionSnappy, snappy)

	zstdVal, err := ConfigToFooterCompression("zstd")
	require.NoError(t, err)
	assert.Equal(t, CompressionZstd, zstdVal)

	_, err = ConfigToFooterCompression("lz4")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid compression")

	_, err = ConfigToFooterCompression("invalid")
	assert.Error(t, err)
}

func TestCompressingDecompressingRoundTrip(t *testing.T) {
	input := []byte("hello world hello world hello world")
	src := bytes.NewReader(input)

	cr, err := newCompressingReader(src, "snappy")
	require.NoError(t, err)
	compressed, err := io.ReadAll(cr)
	require.NoError(t, err)
	require.NotEmpty(t, compressed)

	rc := io.NopCloser(bytes.NewReader(compressed))
	out, err := newDecompressingReadCloser(rc, CompressionSnappy)
	require.NoError(t, err)
	decompressed, err := io.ReadAll(out)
	require.NoError(t, err)
	_ = out.Close()
	assert.Equal(t, input, decompressed)
}

func TestCompressingDecompressingRoundTripZstd(t *testing.T) {
	input := []byte("hello world hello world hello world")
	src := bytes.NewReader(input)

	cr, err := newCompressingReader(src, "zstd")
	require.NoError(t, err)
	compressed, err := io.ReadAll(cr)
	require.NoError(t, err)
	require.NotEmpty(t, compressed)

	rc := io.NopCloser(bytes.NewReader(compressed))
	out, err := newDecompressingReadCloser(rc, CompressionZstd)
	require.NoError(t, err)
	decompressed, err := io.ReadAll(out)
	require.NoError(t, err)
	_ = out.Close()
	assert.Equal(t, input, decompressed)
}

func TestNewCompressingReaderNone(t *testing.T) {
	r := bytes.NewReader([]byte("x"))
	out, err := newCompressingReader(r, "none")
	require.NoError(t, err)
	assert.Equal(t, r, out)

	out, err = newCompressingReader(r, "")
	require.NoError(t, err)
	assert.Equal(t, r, out)
}

func TestNewDecompressingReadCloserNone(t *testing.T) {
	r := io.NopCloser(bytes.NewReader([]byte("x")))
	out, err := newDecompressingReadCloser(r, CompressionNone)
	require.NoError(t, err)
	assert.Equal(t, r, out)
}

func TestNewDecompressingReadCloserUnsupported(t *testing.T) {
	r := io.NopCloser(bytes.NewReader(nil))
	out, err := newDecompressingReadCloser(r, CompressionLZ4)
	assert.Error(t, err)
	assert.Nil(t, out)
	assert.ErrorIs(t, err, errUnsupportedCompression)
}

func TestNewCompressingReaderZstd(t *testing.T) {
	r := bytes.NewReader([]byte("test data for zstd"))
	out, err := newCompressingReader(r, "zstd")
	require.NoError(t, err)
	require.NotNil(t, out)
	data, err := io.ReadAll(out)
	require.NoError(t, err)
	require.NotEmpty(t, data)
	// zstd compressed output should be different from input and typically smaller for repetitive data
	assert.NotEqual(t, []byte("test data for zstd"), data)
}
