package hash_test

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check it satisfies the interface
var _ pflag.Value = (*hash.Type)(nil)

func TestHashSet(t *testing.T) {
	var h hash.Set

	assert.Equal(t, 0, h.Count())

	a := h.Array()
	assert.Len(t, a, 0)

	h = h.Add(hash.MD5)
	fs.Log(nil, fmt.Sprint(h))
	assert.Equal(t, 1, h.Count())
	assert.Equal(t, hash.MD5, h.GetOne())
	a = h.Array()
	assert.Len(t, a, 1)
	assert.Equal(t, a[0], hash.MD5)

	// Test overlap, with all hashes
	h = h.Overlap(hash.Supported())
	assert.Equal(t, 1, h.Count())
	assert.Equal(t, hash.MD5, h.GetOne())
	assert.True(t, h.SubsetOf(hash.Supported()))
	assert.True(t, h.SubsetOf(hash.NewHashSet(hash.MD5)))

	h = h.Add(hash.SHA1)
	assert.Equal(t, 2, h.Count())
	one := h.GetOne()
	if !(one == hash.MD5 || one == hash.SHA1) {
		t.Fatalf("expected to be either MD5 or SHA1, got %v", one)
	}
	assert.True(t, h.SubsetOf(hash.Supported()))
	assert.False(t, h.SubsetOf(hash.NewHashSet(hash.MD5)))
	assert.False(t, h.SubsetOf(hash.NewHashSet(hash.SHA1)))
	assert.True(t, h.SubsetOf(hash.NewHashSet(hash.MD5, hash.SHA1)))
	a = h.Array()
	assert.Len(t, a, 2)

	ol := h.Overlap(hash.NewHashSet(hash.MD5))
	assert.Equal(t, 1, ol.Count())
	assert.True(t, ol.Contains(hash.MD5))
	assert.False(t, ol.Contains(hash.SHA1))

	ol = h.Overlap(hash.NewHashSet(hash.MD5, hash.SHA1))
	assert.Equal(t, 2, ol.Count())
	assert.True(t, ol.Contains(hash.MD5))
	assert.True(t, ol.Contains(hash.SHA1))
}

type hashTest struct {
	input  []byte
	output map[hash.Type]string
}

var hashTestSet = []hashTest{
	{
		input: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14},
		output: map[hash.Type]string{
			hash.MD5:       "bf13fc19e5151ac57d4252e0e0f87abe",
			hash.SHA1:      "3ab6543c08a75f292a5ecedac87ec41642d12166",
			hash.Whirlpool: "eddf52133d4566d763f716e853d6e4efbabd29e2c2e63f56747b1596172851d34c2df9944beb6640dbdbe3d9b4eb61180720a79e3d15baff31c91e43d63869a4",
			hash.CRC32:     "a6041d7e",
			hash.SHA256:    "c839e57675862af5c21bd0a15413c3ec579e0d5522dab600bc6c3489b05b8f54",
		},
	},
	// Empty data set
	{
		input: []byte{},
		output: map[hash.Type]string{
			hash.MD5:       "d41d8cd98f00b204e9800998ecf8427e",
			hash.SHA1:      "da39a3ee5e6b4b0d3255bfef95601890afd80709",
			hash.Whirlpool: "19fa61d75522a4669b44e39c1d2e1726c530232130d407f89afee0964997f7a73e83be698b288febcf88e3e03c4f0757ea8964e59b63d93708b138cc42a66eb3",
			hash.CRC32:     "00000000",
			hash.SHA256:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	},
}

func TestMultiHasher(t *testing.T) {
	for _, test := range hashTestSet {
		mh := hash.NewMultiHasher()
		n, err := io.Copy(mh, bytes.NewBuffer(test.input))
		require.NoError(t, err)
		assert.Len(t, test.input, int(n))
		sums := mh.Sums()
		for k, v := range sums {
			expect, ok := test.output[k]
			require.True(t, ok, "test output for hash not found")
			assert.Equal(t, expect, v)
		}
		// Test that all are present
		for k, v := range test.output {
			expect, ok := sums[k]
			require.True(t, ok, "test output for hash not found")
			assert.Equal(t, expect, v)
		}
	}
}

func TestMultiHasherTypes(t *testing.T) {
	h := hash.SHA1
	for _, test := range hashTestSet {
		mh, err := hash.NewMultiHasherTypes(hash.NewHashSet(h))
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
		sums, err := hash.Stream(bytes.NewBuffer(test.input))
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
	h := hash.SHA1
	for _, test := range hashTestSet {
		sums, err := hash.StreamTypes(bytes.NewBuffer(test.input), hash.NewHashSet(h))
		require.NoError(t, err)
		assert.Len(t, sums, 1)
		assert.Equal(t, sums[h], test.output[h])
	}
}

func TestHashSetStringer(t *testing.T) {
	h := hash.NewHashSet(hash.SHA1, hash.MD5)
	assert.Equal(t, "[md5, sha1]", h.String())
	h = hash.NewHashSet(hash.SHA1)
	assert.Equal(t, "[sha1]", h.String())
	h = hash.NewHashSet()
	assert.Equal(t, "[]", h.String())
}

func TestHashStringer(t *testing.T) {
	h := hash.MD5
	assert.Equal(t, "md5", h.String())
	h = hash.SHA1
	assert.Equal(t, "sha1", h.String())
	h = hash.None
	assert.Equal(t, "none", h.String())
}

func TestHashSetter(t *testing.T) {
	var ht hash.Type

	assert.NoError(t, ht.Set("none"))
	assert.Equal(t, hash.None, ht)
	assert.NoError(t, ht.Set("None"))
	assert.Equal(t, hash.None, ht)

	assert.NoError(t, ht.Set("md5"))
	assert.Equal(t, hash.MD5, ht)
	assert.NoError(t, ht.Set("MD5"))
	assert.Equal(t, hash.MD5, ht)

	assert.NoError(t, ht.Set("sha1"))
	assert.Equal(t, hash.SHA1, ht)
	assert.NoError(t, ht.Set("SHA-1"))
	assert.Equal(t, hash.SHA1, ht)

	assert.NoError(t, ht.Set("SHA1"))
	assert.Equal(t, hash.SHA1, ht)
	assert.NoError(t, ht.Set("Sha1"))
	assert.Equal(t, hash.SHA1, ht)
	assert.Error(t, ht.Set("Sha-1"))
}

func TestHashTypeStability(t *testing.T) {
	assert.Equal(t, hash.Type(0), hash.None)
	assert.Equal(t, hash.Type(1), hash.MD5)
	assert.Equal(t, hash.Type(2), hash.SHA1)

	assert.True(t, hash.Supported().Contains(hash.MD5))
	assert.True(t, hash.Supported().Contains(hash.SHA1))
	assert.False(t, hash.Supported().Contains(hash.None))
}
