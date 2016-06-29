package fs_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/ncw/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashSet(t *testing.T) {
	var h fs.HashSet

	assert.Equal(t, 0, h.Count())

	a := h.Array()
	assert.Len(t, a, 0)

	h = h.Add(fs.HashMD5)
	assert.Equal(t, 1, h.Count())
	assert.Equal(t, fs.HashMD5, h.GetOne())
	a = h.Array()
	assert.Len(t, a, 1)
	assert.Equal(t, a[0], fs.HashMD5)

	// Test overlap, with all hashes
	h = h.Overlap(fs.SupportedHashes)
	assert.Equal(t, 1, h.Count())
	assert.Equal(t, fs.HashMD5, h.GetOne())
	assert.True(t, h.SubsetOf(fs.SupportedHashes))
	assert.True(t, h.SubsetOf(fs.NewHashSet(fs.HashMD5)))

	h = h.Add(fs.HashSHA1)
	assert.Equal(t, 2, h.Count())
	one := h.GetOne()
	if !(one == fs.HashMD5 || one == fs.HashSHA1) {
		t.Fatalf("expected to be either MD5 or SHA1, got %v", one)
	}
	assert.True(t, h.SubsetOf(fs.SupportedHashes))
	assert.False(t, h.SubsetOf(fs.NewHashSet(fs.HashMD5)))
	assert.False(t, h.SubsetOf(fs.NewHashSet(fs.HashSHA1)))
	assert.True(t, h.SubsetOf(fs.NewHashSet(fs.HashMD5, fs.HashSHA1)))
	a = h.Array()
	assert.Len(t, a, 2)

	ol := h.Overlap(fs.NewHashSet(fs.HashMD5))
	assert.Equal(t, 1, ol.Count())
	assert.True(t, ol.Contains(fs.HashMD5))
	assert.False(t, ol.Contains(fs.HashSHA1))

	ol = h.Overlap(fs.NewHashSet(fs.HashMD5, fs.HashSHA1))
	assert.Equal(t, 2, ol.Count())
	assert.True(t, ol.Contains(fs.HashMD5))
	assert.True(t, ol.Contains(fs.HashSHA1))
}

type hashTest struct {
	input  []byte
	output map[fs.HashType]string
}

var hashTestSet = []hashTest{
	{
		input: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14},
		output: map[fs.HashType]string{
			fs.HashMD5:  "bf13fc19e5151ac57d4252e0e0f87abe",
			fs.HashSHA1: "3ab6543c08a75f292a5ecedac87ec41642d12166",
		},
	},
	// Empty data set
	{
		input: []byte{},
		output: map[fs.HashType]string{
			fs.HashMD5:  "d41d8cd98f00b204e9800998ecf8427e",
			fs.HashSHA1: "da39a3ee5e6b4b0d3255bfef95601890afd80709",
		},
	},
}

func TestMultiHasher(t *testing.T) {
	for _, test := range hashTestSet {
		mh := fs.NewMultiHasher()
		n, err := io.Copy(mh, bytes.NewBuffer(test.input))
		require.NoError(t, err)
		assert.Len(t, test.input, int(n))
		sums := mh.Sums()
		for k, v := range sums {
			expect, ok := test.output[k]
			require.True(t, ok)
			assert.Equal(t, v, expect)
		}
		// Test that all are present
		for k, v := range test.output {
			expect, ok := sums[k]
			require.True(t, ok)
			assert.Equal(t, v, expect)
		}
	}
}

func TestMultiHasherTypes(t *testing.T) {
	h := fs.HashSHA1
	for _, test := range hashTestSet {
		mh, err := fs.NewMultiHasherTypes(fs.NewHashSet(h))
		if err != nil {
			t.Fatal(err)
		}
		n, err := io.Copy(mh, bytes.NewBuffer(test.input))
		require.NoError(t, err)
		assert.Len(t, test.input, int(n))
		sums := mh.Sums()
		assert.Len(t, sums, 1)
		assert.Equal(t, sums[h], test.output[h])
	}
}

func TestHashStream(t *testing.T) {
	for _, test := range hashTestSet {
		sums, err := fs.HashStream(bytes.NewBuffer(test.input))
		require.NoError(t, err)
		for k, v := range sums {
			expect, ok := test.output[k]
			require.True(t, ok)
			assert.Equal(t, v, expect)
		}
		// Test that all are present
		for k, v := range test.output {
			expect, ok := sums[k]
			require.True(t, ok)
			assert.Equal(t, v, expect)
		}
	}
}

func TestHashStreamTypes(t *testing.T) {
	h := fs.HashSHA1
	for _, test := range hashTestSet {
		sums, err := fs.HashStreamTypes(bytes.NewBuffer(test.input), fs.NewHashSet(h))
		require.NoError(t, err)
		assert.Len(t, sums, 1)
		assert.Equal(t, sums[h], test.output[h])
	}
}

func TestHashSetStringer(t *testing.T) {
	h := fs.NewHashSet(fs.HashSHA1, fs.HashMD5)
	assert.Equal(t, h.String(), "[MD5, SHA-1]")
	h = fs.NewHashSet(fs.HashSHA1)
	assert.Equal(t, h.String(), "[SHA-1]")
	h = fs.NewHashSet()
	assert.Equal(t, h.String(), "[]")
}

func TestHashStringer(t *testing.T) {
	h := fs.HashMD5
	assert.Equal(t, h.String(), "MD5")
	h = fs.HashNone
	assert.Equal(t, h.String(), "None")
}
