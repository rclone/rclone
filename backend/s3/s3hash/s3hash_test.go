package s3hash

import (
	"crypto/md5"
	"github.com/stretchr/testify/require"
	"testing"
)

func generateData(size int) (d []byte) {
	d = make([]byte, size)
	chars := []byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c'} // count of chars - primary number
	for i := 0; i < size; i++ {
		d[i] = chars[i%len(chars)]
	}
	return
}

func split(data []byte, partSize int) [][]byte {
	var chunks [][]byte
	for {
		if len(data) == 0 {
			break
		}
		// necessary check to avoid slicing beyond
		// slice capacity
		if len(data) < partSize {
			partSize = len(data)
		}

		chunks = append(chunks, data[0:partSize])
		data = data[partSize:]
	}

	return chunks
}

// generateHash generates universal hash of chunks:
// md5bin(md5bin(chunk1) + md5bin(chunk2) + md5bin(chunk3) ...)
func generateHash(data []byte, chunkSize int) []byte {
	chunks := split(data, chunkSize)
	var h []byte
	var m [md5.Size]byte

	if len(chunks) == 1 {
		m = md5.Sum(chunks[0])
	} else {
		for _, chunk := range chunks {
			md5d := md5.New()
			_, _ = md5d.Write(chunk)
			h = md5d.Sum(h)
		}

		m = md5.Sum(h)
	}
	return m[:]
}

// TestBasicMode tests mode when s3hash configured as plain md5
func TestPlainMd5Mode(t *testing.T) {
	md5h := md5.New()
	md5h.Write([]byte("test"))
	testHash := md5h.Sum(nil)

	h := New(0)
	h.Write([]byte("test"))
	require.Equal(t, testHash, h.Sum(nil))

	h = New(10)
	h.Write([]byte("test"))
	require.Equal(t, testHash, h.Sum(nil))

	h = New(10)
	h.Write([]byte("test"))
	require.Equal(t, testHash, h.Sum(nil))
}

var parts = [][]int{
	// {dataSize, partSize}
	{50, 100},
	{200, 100},
	{150, 100},
	{350, 105},
	{10002, 100},
}

func TestPartedHash(t *testing.T) {
	for _, p := range parts {
		data := generateData(p[0])
		expectedHash := generateHash(data, p[1])
		chunks := split(data, p[1])

		h := New(p[1])
		for _, chunk := range chunks {
			h.Write(chunk)
		}
		require.Equalf(t, expectedHash, h.Sum(nil), "data size %d, part size %d", p[0], p[1])
		// checks what Sum() don't reset state of the S3Hash.
		require.Equalf(t, expectedHash, h.Sum(nil), "data size %d, part size %d (second)", p[0], p[1])
	}
}
