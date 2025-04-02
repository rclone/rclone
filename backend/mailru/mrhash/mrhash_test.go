package mrhash_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/rclone/rclone/backend/mailru/mrhash"
	"github.com/stretchr/testify/assert"
)

func testChunk(t *testing.T, chunk int) {
	data := make([]byte, chunk)
	for i := range chunk {
		data[i] = 'A'
	}
	for _, test := range []struct {
		n    int
		want string
	}{
		{0, "0000000000000000000000000000000000000000"},
		{1, "4100000000000000000000000000000000000000"},
		{2, "4141000000000000000000000000000000000000"},
		{19, "4141414141414141414141414141414141414100"},
		{20, "4141414141414141414141414141414141414141"},
		{21, "eb1d05e78a18691a5aa196a6c2b60cd40b5faafb"},
		{22, "037e6d960601118a0639afbeff30fe716c66ed2d"},
		{4096, "45a16aa192502b010280fb5b44274c601a91fd9f"},
		{4194303, "fa019d5bd26498cf6abe35e0d61801bf19bf704b"},
		{4194304, "5ed0e07aa6ea5c1beb9402b4d807258f27d40773"},
		{4194305, "67bd0b9247db92e0e7d7e29a0947a50fedcb5452"},
		{8388607, "41a8e2eb044c2e242971b5445d7be2a13fc0dd84"},
		{8388608, "267a970917c624c11fe624276ec60233a66dc2c0"},
		{8388609, "37b60b308d553d2732aefb62b3ea88f74acfa13f"},
	} {
		d := mrhash.New()
		var toWrite int
		for toWrite = test.n; toWrite >= chunk; toWrite -= chunk {
			n, err := d.Write(data)
			assert.Nil(t, err)
			assert.Equal(t, chunk, n)
		}
		n, err := d.Write(data[:toWrite])
		assert.Nil(t, err)
		assert.Equal(t, toWrite, n)
		got1 := hex.EncodeToString(d.Sum(nil))
		assert.Equal(t, test.want, got1, fmt.Sprintf("when testing length %d", n))
		got2 := hex.EncodeToString(d.Sum(nil))
		assert.Equal(t, test.want, got2, fmt.Sprintf("when testing length %d (2nd sum)", n))
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
	d := mrhash.New()
	assert.NotPanics(t, func() { d.Sum(nil) })
	d.Reset()
	assert.NotPanics(t, func() { d.Sum(nil) })
	assert.NotPanics(t, func() { d.Sum(nil) })
	_, _ = d.Write([]byte{1})
	assert.NotPanics(t, func() { d.Sum(nil) })
}

func TestSize(t *testing.T) {
	d := mrhash.New()
	assert.Equal(t, 20, d.Size())
}

func TestBlockSize(t *testing.T) {
	d := mrhash.New()
	assert.Equal(t, 64, d.BlockSize())
}
