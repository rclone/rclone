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

func TestUncompressedInventory(t *testing.T) {
	assert.Nil(t, uncompressedInventory(0))
	assert.Nil(t, uncompressedInventory(-1))
	// Single block: content fits in one block
	inv := uncompressedInventory(100)
	require.Len(t, inv, 1)
	assert.Equal(t, uint32(100), inv[0])
	// Full blocks
	inv = uncompressedInventory(BlockSize * 3)
	require.Len(t, inv, 3)
	assert.Equal(t, uint32(BlockSize), inv[0])
	assert.Equal(t, uint32(BlockSize), inv[1])
	assert.Equal(t, uint32(BlockSize), inv[2])
	// Last block smaller
	inv = uncompressedInventory(BlockSize*2 + 100)
	require.Len(t, inv, 3)
	assert.Equal(t, uint32(BlockSize), inv[0])
	assert.Equal(t, uint32(BlockSize), inv[1])
	assert.Equal(t, uint32(100), inv[2])
}

func TestBuildParseInventory(t *testing.T) {
	sizes := []uint32{100, 200, 150}
	b := buildInventory(sizes)
	require.Len(t, b, 12)
	parsed := parseInventory(b)
	assert.Equal(t, sizes, parsed)
}

func TestFullStreamRangeForBlocks(t *testing.T) {
	inv := []uint32{10, 20, 30, 40} // blocks 0..3
	tests := []struct {
		first, last     int
		wantStart, wantLen int64
	}{
		{0, 0, 0, 10},
		{1, 1, 10, 20},
		{2, 2, 30, 30},
		{0, 1, 0, 30},
		{1, 2, 10, 50},
		{0, 3, 0, 100},
	}
	for _, tt := range tests {
		gotStart, gotLen := fullStreamRangeForBlocks(inv, tt.first, tt.last)
		assert.Equal(t, tt.wantStart, gotStart, "first=%d last=%d start", tt.first, tt.last)
		assert.Equal(t, tt.wantLen, gotLen, "first=%d last=%d len", tt.first, tt.last)
	}
}

func TestAlignFullStreamToPairs(t *testing.T) {
	tests := []struct {
		start, len int64
		wantStart, wantLen int64
	}{
		{0, 4, 0, 4},
		{1, 1, 0, 2},
		{2, 2, 2, 2},
		{1, 2, 0, 4},
		{5, 3, 4, 4},
	}
	for _, tt := range tests {
		gotStart, gotLen := alignFullStreamToPairs(tt.start, tt.len)
		assert.Equal(t, tt.wantStart, gotStart, "start")
		assert.Equal(t, tt.wantLen, gotLen, "len")
	}
}

func TestParticleRangesForFullStream(t *testing.T) {
	tests := []struct {
		name                    string
		fullStart, fullLen      int64
		evenStart, evenEnd      int64
		oddStart, oddEnd        int64
	}{
		{"[0,4) first 4 bytes", 0, 4, 0, 1, 0, 1},
		{"[0,1) first byte (even)", 0, 1, 0, 0, 0, -1},
		{"[1,2) second byte (odd) only", 1, 1, 1, 0, 0, 0},
		{"[2,5) bytes 2,3,4", 2, 3, 1, 2, 1, 1},
		{"[1,3) bytes 1,2", 1, 2, 1, 1, 0, 0},
		{"[0,2) bytes 0-1", 0, 2, 0, 0, 0, 0},
		{"[0,3) bytes 0-2", 0, 3, 0, 1, 0, 0},
		{"empty", 0, 0, 0, 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es, ee, os, oe := particleRangesForFullStream(tt.fullStart, tt.fullLen)
			assert.Equal(t, tt.evenStart, es, "evenStart")
			assert.Equal(t, tt.evenEnd, ee, "evenEnd")
			assert.Equal(t, tt.oddStart, os, "oddStart")
			assert.Equal(t, tt.oddEnd, oe, "oddEnd")
		})
	}
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
