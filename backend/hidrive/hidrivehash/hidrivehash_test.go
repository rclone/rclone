package hidrivehash_test

import (
	"crypto/sha1"
	"encoding"
	"encoding/hex"
	"fmt"
	"io"
	"testing"

	"github.com/rclone/rclone/backend/hidrive/hidrivehash"
	"github.com/rclone/rclone/backend/hidrive/hidrivehash/internal"
	"github.com/stretchr/testify/assert"
)

// helper functions to set up test-tables

func sha1ArrayAsSlice(sum [sha1.Size]byte) []byte {
	return sum[:]
}

func mustDecode(hexstring string) []byte {
	result, err := hex.DecodeString(hexstring)
	if err != nil {
		panic(err)
	}
	return result
}

// ------------------------------------------------------------

var testTableLevelPositionEmbedded = []struct {
	ins  [][]byte
	outs [][]byte
	name string
}{
	{
		[][]byte{
			sha1ArrayAsSlice([20]byte{245, 202, 195, 223, 121, 198, 189, 112, 138, 202, 222, 2, 146, 156, 127, 16, 208, 233, 98, 88}),
			sha1ArrayAsSlice([20]byte{78, 188, 156, 219, 173, 54, 81, 55, 47, 220, 222, 207, 201, 21, 57, 252, 255, 239, 251, 186}),
		},
		[][]byte{
			sha1ArrayAsSlice([20]byte{245, 202, 195, 223, 121, 198, 189, 112, 138, 202, 222, 2, 146, 156, 127, 16, 208, 233, 98, 88}),
			sha1ArrayAsSlice([20]byte{68, 135, 96, 187, 38, 253, 14, 167, 186, 167, 188, 210, 91, 177, 185, 13, 208, 217, 94, 18}),
		},
		"documentation-v3.2rev27-example L0 (position-embedded)",
	},
	{
		[][]byte{
			sha1ArrayAsSlice([20]byte{68, 254, 92, 166, 52, 37, 104, 180, 22, 123, 249, 144, 182, 78, 64, 74, 57, 117, 225, 195}),
			sha1ArrayAsSlice([20]byte{75, 211, 153, 190, 125, 179, 67, 49, 60, 149, 98, 246, 142, 20, 11, 254, 159, 162, 129, 237}),
			sha1ArrayAsSlice([20]byte{150, 2, 9, 153, 97, 153, 189, 104, 147, 14, 77, 203, 244, 243, 25, 212, 67, 48, 111, 107}),
		},
		[][]byte{
			sha1ArrayAsSlice([20]byte{68, 254, 92, 166, 52, 37, 104, 180, 22, 123, 249, 144, 182, 78, 64, 74, 57, 117, 225, 195}),
			sha1ArrayAsSlice([20]byte{144, 209, 246, 100, 177, 216, 171, 229, 83, 17, 92, 135, 68, 98, 76, 72, 217, 24, 99, 176}),
			sha1ArrayAsSlice([20]byte{38, 211, 255, 254, 19, 114, 105, 77, 230, 31, 170, 83, 57, 85, 102, 29, 28, 72, 211, 27}),
		},
		"documentation-example L0 (position-embedded)",
	},
	{
		[][]byte{
			sha1ArrayAsSlice([20]byte{173, 123, 132, 245, 176, 172, 43, 183, 121, 40, 66, 252, 101, 249, 188, 193, 160, 189, 2, 116}),
			sha1ArrayAsSlice([20]byte{40, 34, 8, 238, 37, 5, 237, 184, 79, 105, 10, 167, 171, 254, 13, 229, 132, 112, 254, 8}),
			sha1ArrayAsSlice([20]byte{39, 112, 26, 86, 190, 35, 100, 101, 28, 131, 122, 191, 254, 144, 239, 107, 253, 124, 104, 203}),
		},
		[][]byte{
			sha1ArrayAsSlice([20]byte{173, 123, 132, 245, 176, 172, 43, 183, 121, 40, 66, 252, 101, 249, 188, 193, 160, 189, 2, 116}),
			sha1ArrayAsSlice([20]byte{213, 157, 141, 227, 213, 178, 25, 111, 200, 145, 77, 164, 17, 247, 202, 167, 37, 46, 0, 124}),
			sha1ArrayAsSlice([20]byte{253, 13, 168, 58, 147, 213, 125, 212, 229, 20, 200, 100, 16, 136, 186, 19, 34, 170, 105, 71}),
		},
		"documentation-example L1 (position-embedded)",
	},
}

var testTableLevel = []struct {
	ins  [][]byte
	outs [][]byte
	name string
}{
	{
		[][]byte{
			mustDecode("09f077820a8a41f34a639f2172f1133b1eafe4e6"),
			mustDecode("09f077820a8a41f34a639f2172f1133b1eafe4e6"),
			mustDecode("09f077820a8a41f34a639f2172f1133b1eafe4e6"),
		},
		[][]byte{
			mustDecode("44fe5ca6342568b4167bf990b64e404a3975e1c3"),
			mustDecode("90d1f664b1d8abe553115c8744624c48d91863b0"),
			mustDecode("26d3fffe1372694de61faa533955661d1c48d31b"),
		},
		"documentation-example L0",
	},
	{
		[][]byte{
			mustDecode("75a9f88fb219ef1dd31adf41c93e2efaac8d0245"),
			mustDecode("daedc425199501b1e86b5eaba5649cbde205e6ae"),
			mustDecode("286ac5283f99c4e0f11683900a3e39661c375dd6"),
		},
		[][]byte{
			mustDecode("ad7b84f5b0ac2bb7792842fc65f9bcc1a0bd0274"),
			mustDecode("d59d8de3d5b2196fc8914da411f7caa7252e007c"),
			mustDecode("fd0da83a93d57dd4e514c8641088ba1322aa6947"),
		},
		"documentation-example L1",
	},
	{
		[][]byte{
			mustDecode("0000000000000000000000000000000000000000"),
			mustDecode("0000000000000000000000000000000000000000"),
			mustDecode("75a9f88fb219ef1dd31adf41c93e2efaac8d0245"),
			mustDecode("0000000000000000000000000000000000000000"),
			mustDecode("daedc425199501b1e86b5eaba5649cbde205e6ae"),
			mustDecode("0000000000000000000000000000000000000000"),
			mustDecode("0000000000000000000000000000000000000000"),
			mustDecode("0000000000000000000000000000000000000000"),
			mustDecode("286ac5283f99c4e0f11683900a3e39661c375dd6"),
			mustDecode("0000000000000000000000000000000000000000"),
		},
		[][]byte{
			mustDecode("0000000000000000000000000000000000000000"),
			mustDecode("0000000000000000000000000000000000000000"),
			mustDecode("a197464ec19f2b2b2bc6b21f6c939c7e57772843"),
			mustDecode("a197464ec19f2b2b2bc6b21f6c939c7e57772843"),
			mustDecode("b04769357aa4eb4b52cd5bec6935bc8f977fa3a1"),
			mustDecode("b04769357aa4eb4b52cd5bec6935bc8f977fa3a1"),
			mustDecode("b04769357aa4eb4b52cd5bec6935bc8f977fa3a1"),
			mustDecode("b04769357aa4eb4b52cd5bec6935bc8f977fa3a1"),
			mustDecode("8f56351897b4e1d100646fa122c924347721b2f5"),
			mustDecode("8f56351897b4e1d100646fa122c924347721b2f5"),
		},
		"mixed-with-empties",
	},
}

var testTable = []struct {
	data []byte
	// pattern describes how to use data to construct the hash-input.
	// For every entry n at even indices this repeats the data n times.
	// For every entry m at odd indices this repeats a null-byte m times.
	// The input-data is constructed by concatenating the results in order.
	pattern []int64
	out     []byte
	name    string
}{
	{
		[]byte("#ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyz\n"),
		[]int64{64},
		mustDecode("09f077820a8a41f34a639f2172f1133b1eafe4e6"),
		"documentation-example L0",
	},
	{
		[]byte("#ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyz\n"),
		[]int64{64 * 256},
		mustDecode("75a9f88fb219ef1dd31adf41c93e2efaac8d0245"),
		"documentation-example L1",
	},
	{
		[]byte("#ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyz\n"),
		[]int64{64 * 256, 0, 64 * 128, 4096 * 128, 64*2 + 32},
		mustDecode("fd0da83a93d57dd4e514c8641088ba1322aa6947"),
		"documentation-example L2",
	},
	{
		[]byte("hello rclone\n"),
		[]int64{316},
		mustDecode("72370f9c18a2c20b31d71f3f4cee7a3cd2703737"),
		"not-block-aligned",
	},
	{
		[]byte("hello rclone\n"),
		[]int64{13, 4096 * 3, 4},
		mustDecode("a6990b81791f0d2db750b38f046df321c975aa60"),
		"not-block-aligned-with-null-bytes",
	},
	{
		[]byte{},
		[]int64{},
		mustDecode("0000000000000000000000000000000000000000"),
		"empty",
	},
	{
		[]byte{},
		[]int64{0, 4096 * 256 * 256},
		mustDecode("0000000000000000000000000000000000000000"),
		"null-bytes",
	},
}

// ------------------------------------------------------------

func TestLevelAdd(t *testing.T) {
	for _, test := range testTableLevelPositionEmbedded {
		l := hidrivehash.NewLevel().(internal.LevelHash)
		t.Run(test.name, func(t *testing.T) {
			for i := range test.ins {
				l.Add(test.ins[i])
				assert.Equal(t, test.outs[i], l.Sum(nil))
			}
		})
	}
}

func TestLevelWrite(t *testing.T) {
	for _, test := range testTableLevel {
		l := hidrivehash.NewLevel()
		t.Run(test.name, func(t *testing.T) {
			for i := range test.ins {
				l.Write(test.ins[i])
				assert.Equal(t, test.outs[i], l.Sum(nil))
			}
		})
	}
}

func TestLevelIsFull(t *testing.T) {
	content := [hidrivehash.Size]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}
	l := hidrivehash.NewLevel()
	for range 256 {
		assert.False(t, l.(internal.LevelHash).IsFull())
		written, err := l.Write(content[:])
		assert.Equal(t, len(content), written)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
	}
	assert.True(t, l.(internal.LevelHash).IsFull())
	written, err := l.Write(content[:])
	assert.True(t, l.(internal.LevelHash).IsFull())
	assert.Equal(t, 0, written)
	assert.ErrorIs(t, err, hidrivehash.ErrorHashFull)
}

func TestLevelReset(t *testing.T) {
	l := hidrivehash.NewLevel()
	zeroHash := l.Sum(nil)
	_, err := l.Write([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19})
	if assert.NoError(t, err) {
		assert.NotEqual(t, zeroHash, l.Sum(nil))
		l.Reset()
		assert.Equal(t, zeroHash, l.Sum(nil))
	}
}

func TestLevelSize(t *testing.T) {
	l := hidrivehash.NewLevel()
	assert.Equal(t, 20, l.Size())
}

func TestLevelBlockSize(t *testing.T) {
	l := hidrivehash.NewLevel()
	assert.Equal(t, 20, l.BlockSize())
}

func TestLevelBinaryMarshaler(t *testing.T) {
	content := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}
	l := hidrivehash.NewLevel().(internal.LevelHash)
	l.Write(content[:10])
	encoded, err := l.MarshalBinary()
	if assert.NoError(t, err) {
		d := hidrivehash.NewLevel().(internal.LevelHash)
		err = d.UnmarshalBinary(encoded)
		if assert.NoError(t, err) {
			assert.Equal(t, l.Sum(nil), d.Sum(nil))
			l.Write(content[10:])
			d.Write(content[10:])
			assert.Equal(t, l.Sum(nil), d.Sum(nil))
		}
	}
}

func TestLevelInvalidEncoding(t *testing.T) {
	l := hidrivehash.NewLevel().(internal.LevelHash)
	err := l.UnmarshalBinary([]byte{})
	assert.ErrorIs(t, err, hidrivehash.ErrorInvalidEncoding)
}

// ------------------------------------------------------------

type infiniteReader struct {
	source []byte
	offset int
}

func (m *infiniteReader) Read(b []byte) (int, error) {
	count := copy(b, m.source[m.offset:])
	m.offset += count
	m.offset %= len(m.source)
	return count, nil
}

func writeInChunks(writer io.Writer, chunkSize int64, data []byte, pattern []int64) error {
	readers := make([]io.Reader, len(pattern))
	nullBytes := [4096]byte{}
	for i, n := range pattern {
		if i%2 == 0 {
			readers[i] = io.LimitReader(&infiniteReader{data, 0}, n*int64(len(data)))
		} else {
			readers[i] = io.LimitReader(&infiniteReader{nullBytes[:], 0}, n)
		}
	}
	reader := io.MultiReader(readers...)
	for {
		_, err := io.CopyN(writer, reader, chunkSize)
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return err
		}
	}
}

func TestWrite(t *testing.T) {
	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			h := hidrivehash.New()
			err := writeInChunks(h, int64(h.BlockSize()), test.data, test.pattern)
			if assert.NoError(t, err) {
				normalSum := h.Sum(nil)
				assert.Equal(t, test.out, normalSum)
				// Test if different block-sizes produce differing results.
				for _, blockSize := range []int64{397, 512, 4091, 8192, 10000} {
					t.Run(fmt.Sprintf("block-size %v", blockSize), func(t *testing.T) {
						h := hidrivehash.New()
						err := writeInChunks(h, blockSize, test.data, test.pattern)
						if assert.NoError(t, err) {
							assert.Equal(t, normalSum, h.Sum(nil))
						}
					})
				}
			}
		})
	}
}

func TestReset(t *testing.T) {
	h := hidrivehash.New()
	zeroHash := h.Sum(nil)
	_, err := h.Write([]byte{1})
	if assert.NoError(t, err) {
		assert.NotEqual(t, zeroHash, h.Sum(nil))
		h.Reset()
		assert.Equal(t, zeroHash, h.Sum(nil))
	}
}

func TestSize(t *testing.T) {
	h := hidrivehash.New()
	assert.Equal(t, 20, h.Size())
}

func TestBlockSize(t *testing.T) {
	h := hidrivehash.New()
	assert.Equal(t, 4096, h.BlockSize())
}

func TestBinaryMarshaler(t *testing.T) {
	for _, test := range testTable {
		h := hidrivehash.New()
		d := hidrivehash.New()
		half := len(test.pattern) / 2
		t.Run(test.name, func(t *testing.T) {
			err := writeInChunks(h, int64(h.BlockSize()), test.data, test.pattern[:half])
			assert.NoError(t, err)
			encoded, err := h.(encoding.BinaryMarshaler).MarshalBinary()
			if assert.NoError(t, err) {
				err = d.(encoding.BinaryUnmarshaler).UnmarshalBinary(encoded)
				if assert.NoError(t, err) {
					assert.Equal(t, h.Sum(nil), d.Sum(nil))
					err = writeInChunks(h, int64(h.BlockSize()), test.data, test.pattern[half:])
					assert.NoError(t, err)
					err = writeInChunks(d, int64(d.BlockSize()), test.data, test.pattern[half:])
					assert.NoError(t, err)
					assert.Equal(t, h.Sum(nil), d.Sum(nil))
				}
			}
		})
	}
}

func TestInvalidEncoding(t *testing.T) {
	h := hidrivehash.New()
	err := h.(encoding.BinaryUnmarshaler).UnmarshalBinary([]byte{})
	assert.ErrorIs(t, err, hidrivehash.ErrorInvalidEncoding)
}

func TestSum(t *testing.T) {
	assert.Equal(t, [hidrivehash.Size]byte{}, hidrivehash.Sum([]byte{}))
	content := []byte{1}
	h := hidrivehash.New()
	h.Write(content)
	sum := hidrivehash.Sum(content)
	assert.Equal(t, h.Sum(nil), sum[:])
}
