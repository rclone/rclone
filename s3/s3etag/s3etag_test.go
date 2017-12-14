package s3etag_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/ncw/rclone/s3/s3etag"
	"github.com/stretchr/testify/assert"
)

func testChunk(t *testing.T, chunk int) {
	data := make([]byte, chunk)
	for i := 0; i < chunk; i++ {
		data[i] = 'A'
	}
	for _, test := range []struct {
		n    int
		want string
	}{
		{0, "d41d8cd98f00b204e9800998ecf8427e0001"},
		{1, "7fc56270e7a70fa81a5935b72eacbe290001"},
		{2, "3b98e2dffc6cb06a89dcb0d5c60a02060001"},
		{4096, "82a7348c2e03731109d0cf45a7325b880001"},
		{4194303, "641df9678a8c9a1527879c6852fe91730001"},
		{4194304, "4a31ac3594cb245c08e134ec06b3057e0001"},
		{4194305, "66816f4e9f5e61a7925026675e63ad4f0001"},
		{8388607, "bc229508beb212045c0708355548ee720002"},
		{8388608, "6ebd5a9ae5b0bccec7a57598aa48b6af0002"},
		{8388609, "c1c497519ddb7926871193810877832b0002"},
	} {
		s := s3etag.New(int64(test.n))
		var toWrite int
		for toWrite = test.n; toWrite >= chunk; toWrite -= chunk {
			n, err := s.Write(data)
			assert.Nil(t, err)
			assert.Equal(t, chunk, n)
		}
		n, err := s.Write(data[:toWrite])
		assert.Nil(t, err)
		assert.Equal(t, toWrite, n)
		got := hex.EncodeToString(s.Sum(nil))
		assert.Equal(t, test.want, got, fmt.Sprintf("when testing length %d", n))
	}
}

func TestHashChunk16M(t *testing.T)  { testChunk(t, 16*1024*1024) }
func TestHashChunk8M(t *testing.T)   { testChunk(t, 8*1024*1024) }
func TestHashChunk4M(t *testing.T)   { testChunk(t, 4*1024*1024) }
func TestHashChunk2M(t *testing.T)   { testChunk(t, 2*1024*1024) }
func TestHashChunk1M(t *testing.T)   { testChunk(t, 1*1024*1024) }
func TestHashChunk64k(t *testing.T)  { testChunk(t, 64*1024) }
func TestHashChunk32k(t *testing.T)  { testChunk(t, 32*1024) }
func TestHashChunk2048(t *testing.T) { testChunk(t, 2048) }
func TestHashChunk2047(t *testing.T) { testChunk(t, 2047) }

func TestSumCalledTwice(t *testing.T) {
	s := s3etag.New(-1)
	assert.NotPanics(t, func() { s.Sum(nil) })
	s.Reset()
	assert.NotPanics(t, func() { s.Sum(nil) })
	assert.NotPanics(t, func() { s.Sum(nil) })
	_, _ = s.Write([]byte{1})
	assert.Panics(t, func() { s.Sum(nil) })
}

func TestSize(t *testing.T) {
	s := s3etag.New(-1)
	assert.Equal(t, 36, s.Size())
}

func TestBlockSize(t *testing.T) {
	s := s3etag.New(-1)
	assert.Equal(t, 64, s.BlockSize())
}

func TestSum(t *testing.T) {
	assert.Equal(t,
		[36]byte{
			0x7f, 0xc5, 0x62, 0x70, 0xe7, 0xa7, 0x0f, 0xa8,
			0x1a, 0x59, 0x35, 0xb7, 0x2e, 0xac, 0xbe, 0x29,
			0x00, 0x01,
		},
		s3etag.Sum([]byte{'A'}),
	)
}
