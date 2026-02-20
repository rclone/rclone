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

func TestBlockCompressDecompressRoundTrip(t *testing.T) {
	block := make([]byte, BlockSize)
	for i := range block {
		block[i] = byte(i % 256)
	}
	for _, comp := range []struct {
		name [4]byte
	}{
		{CompressionSnappy},
		{CompressionZstd},
	} {
		t.Run(string(comp.name[:3]), func(t *testing.T) {
			compressed, err := compressBlock(block, comp.name)
			require.NoError(t, err)
			require.NotEmpty(t, compressed)
			decompressed, err := decompressBlock(compressed, comp.name)
			require.NoError(t, err)
			assert.Equal(t, block, decompressed)
		})
	}
}

func TestBlockCompressDecompressPartialLastBlock(t *testing.T) {
	// Last block smaller than BlockSize
	smallBlock := make([]byte, 1000)
	for i := range smallBlock {
		smallBlock[i] = byte(i % 256)
	}
	compressed, err := compressBlock(smallBlock, CompressionSnappy)
	require.NoError(t, err)
	require.NotEmpty(t, compressed)
	decompressed, err := decompressBlock(compressed, CompressionSnappy)
	require.NoError(t, err)
	assert.Equal(t, smallBlock, decompressed)
}

func TestBlockHelpers(t *testing.T) {
	assert.Equal(t, 8, inventoryLength(2))
	assert.Equal(t, 0, blockIndex(0))
	assert.Equal(t, 0, blockIndex(BlockSize-1))
	assert.Equal(t, 1, blockIndex(BlockSize))
	assert.Equal(t, BlockSize, lastBlockUncompressedSize(BlockSize*3))
	assert.Equal(t, 100, lastBlockUncompressedSize(BlockSize*2+100))
	assert.Equal(t, BlockSize, lastBlockUncompressedSize(BlockSize))
	assert.Equal(t, 0, lastBlockUncompressedSize(0))
}

func TestBuildParseInventory(t *testing.T) {
	sizes := []uint32{100, 200, 150}
	b := buildInventory(sizes)
	require.Len(t, b, 12)
	parsed := parseInventory(b)
	assert.Equal(t, sizes, parsed)
}

func TestBlockDecompressReadCloser(t *testing.T) {
	// Simulate block-compressed payload: 2 blocks, merge even+odd into single stream
	block1 := make([]byte, BlockSize)
	for i := range block1 {
		block1[i] = byte(i % 256)
	}
	block2 := []byte("trailing data")
	comp1, err := compressBlock(block1, CompressionSnappy)
	require.NoError(t, err)
	comp2, err := compressBlock(block2, CompressionSnappy)
	require.NoError(t, err)
	inventory := buildInventory([]uint32{uint32(len(comp1)), uint32(len(comp2))})

	// Merge comp1 and comp2 (simulating StreamMerger output: interleaved even+odd)
	merged := append(comp1, comp2...)
	src := io.NopCloser(bytes.NewReader(merged))

	dec := newBlockDecompressReadCloser(src, parseInventory(inventory), CompressionSnappy)
	got, err := io.ReadAll(dec)
	require.NoError(t, err)
	_ = dec.Close()

	want := append(block1, block2...)
	assert.Equal(t, want, got)
}
