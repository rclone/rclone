package raid3

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFooterMarshalParseRoundTrip(t *testing.T) {
	contentLength := int64(12345)
	md5 := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	sha256 := [32]byte{}
	for i := range sha256 {
		sha256[i] = byte(i)
	}
	mtime := time.Unix(1600000000, 0)

	ft := FooterFromReconstructed(contentLength, md5[:], sha256[:], mtime, CompressionNone, ShardEven)
	b, err := ft.MarshalBinary()
	require.NoError(t, err)
	assert.Len(t, b, FooterSize)

	parsed, err := ParseFooter(b)
	require.NoError(t, err)
	assert.Equal(t, contentLength, parsed.ContentLength)
	assert.Equal(t, md5, parsed.MD5)
	assert.Equal(t, sha256, parsed.SHA256)
	assert.Equal(t, mtime.Unix(), parsed.Mtime)
	assert.Equal(t, uint8(ShardEven), parsed.CurrentShard)
	assert.Equal(t, uint8(2), parsed.DataShards)
	assert.Equal(t, uint8(1), parsed.ParityShards)
	assert.Equal(t, AlgorithmR3, parsed.Algorithm)
}

func TestParseFooterInvalidMagic(t *testing.T) {
	buf := make([]byte, FooterSize)
	copy(buf, "BADMAGIC!")
	_, err := ParseFooter(buf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "magic")
}

func TestParseFooterWrongLength(t *testing.T) {
	_, err := ParseFooter([]byte("short"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "90")

	_, err = ParseFooter(make([]byte, 100))
	assert.Error(t, err)
}

func TestParseFooterInvalidVersion(t *testing.T) {
	buf := make([]byte, FooterSize)
	copy(buf[0:9], FooterMagic)
	// Version at 9:11 - set to 99
	buf[9] = 99
	buf[10] = 0
	_, err := ParseFooter(buf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "version")
}

func TestFooterShards(t *testing.T) {
	for shard := 0; shard < 3; shard++ {
		ft := FooterFromReconstructed(100, nil, nil, time.Now(), CompressionNone, shard)
		b, err := ft.MarshalBinary()
		require.NoError(t, err)
		parsed, err := ParseFooter(b)
		require.NoError(t, err)
		assert.Equal(t, uint8(shard), parsed.CurrentShard)
	}
}

func TestFooterReservedZeros(t *testing.T) {
	ft := FooterFromReconstructed(0, nil, nil, time.Time{}, CompressionNone, ShardParity)
	b, err := ft.MarshalBinary()
	require.NoError(t, err)
	assert.Equal(t, []byte{0, 0, 0, 0}, b[86:90])
}

func TestFooterAllThreeShardsMarshal(t *testing.T) {
	contentLength := int64(42)
	mtime := time.Now()
	md5 := [16]byte{0xaa}
	sha256 := [32]byte{0xbb}

	for _, shard := range []int{ShardEven, ShardOdd, ShardParity} {
		ft := FooterFromReconstructed(contentLength, md5[:], sha256[:], mtime, CompressionNone, shard)
		b, err := ft.MarshalBinary()
		require.NoError(t, err)
		require.Len(t, b, FooterSize)
		parsed, err := ParseFooter(b)
		require.NoError(t, err)
		assert.Equal(t, contentLength, parsed.ContentLength)
		assert.Equal(t, uint8(shard), parsed.CurrentShard)
	}
	// Only CurrentShard should differ between the three
	fe := FooterFromReconstructed(1, nil, nil, mtime, CompressionNone, ShardEven)
	fo := FooterFromReconstructed(1, nil, nil, mtime, CompressionNone, ShardOdd)
	fp := FooterFromReconstructed(1, nil, nil, mtime, CompressionNone, ShardParity)
	be, _ := fe.MarshalBinary()
	bo, _ := fo.MarshalBinary()
	bp, _ := fp.MarshalBinary()
	assert.False(t, bytes.Equal(be, bo))
	assert.False(t, bytes.Equal(bo, bp))
	assert.False(t, bytes.Equal(be, bp))
	// Only byte 85 (CurrentShard) should differ
	for i := 0; i < FooterSize; i++ {
		if i == 85 {
			assert.Equal(t, byte(0), be[85])
			assert.Equal(t, byte(1), bo[85])
			assert.Equal(t, byte(2), bp[85])
		} else {
			assert.Equal(t, be[i], bo[i], "index %d", i)
			assert.Equal(t, be[i], bp[i], "index %d", i)
		}
	}
}
