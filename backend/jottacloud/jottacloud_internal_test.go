package jottacloud

import (
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A test reader to return a test pattern of size
type testReader struct {
	size int64
	c    byte
}

// Reader is the interface that wraps the basic Read method.
func (r *testReader) Read(p []byte) (n int, err error) {
	for i := range p {
		if r.size <= 0 {
			return n, io.EOF
		}
		p[i] = r.c
		r.c = (r.c + 1) % 253
		r.size--
		n++
	}
	return
}

func TestReadMD5(t *testing.T) {
	// smoke test the reader
	b, err := ioutil.ReadAll(&testReader{size: 10})
	require.NoError(t, err)
	assert.Equal(t, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, b)

	// Check readMD5 for different size and threshold
	for _, size := range []int64{0, 1024, 10 * 1024, 100 * 1024} {
		t.Run(fmt.Sprintf("%d", size), func(t *testing.T) {
			hasher := md5.New()
			n, err := io.Copy(hasher, &testReader{size: size})
			require.NoError(t, err)
			assert.Equal(t, n, size)
			wantMD5 := fmt.Sprintf("%x", hasher.Sum(nil))
			for _, threshold := range []int64{512, 1024, 10 * 1024, 20 * 1024} {
				t.Run(fmt.Sprintf("%d", threshold), func(t *testing.T) {
					in := &testReader{size: size}
					gotMD5, out, cleanup, err := readMD5(in, size, threshold)
					defer cleanup()
					require.NoError(t, err)
					assert.Equal(t, wantMD5, gotMD5)

					// check md5hash of out
					hasher := md5.New()
					n, err := io.Copy(hasher, out)
					require.NoError(t, err)
					assert.Equal(t, n, size)
					outMD5 := fmt.Sprintf("%x", hasher.Sum(nil))
					assert.Equal(t, wantMD5, outMD5)
				})
			}
		})
	}
}
