package pool

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/readers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const blockSize = 4096

var rwPool = New(60*time.Second, blockSize, 2, false)

// A writer that always returns an error
type testWriterError struct{}

var errWriteError = errors.New("write error")

func (testWriterError) Write(p []byte) (n int, err error) {
	return 0, errWriteError
}

func TestRW(t *testing.T) {
	var dst []byte
	var pos int64
	var err error
	var n int

	testData := []byte("Goodness!!") // 10 bytes long

	newRW := func() *RW {
		rw := NewRW(rwPool)
		buf := bytes.NewBuffer(testData)
		nn, err := rw.ReadFrom(buf) // fill up with goodness
		assert.NoError(t, err)
		assert.Equal(t, int64(10), nn)
		assert.Equal(t, int64(10), rw.Size())
		return rw
	}

	close := func(rw *RW) {
		assert.NoError(t, rw.Close())
	}

	t.Run("Empty", func(t *testing.T) {
		// Test empty read
		rw := NewRW(rwPool)
		defer close(rw)
		assert.Equal(t, int64(0), rw.Size())

		dst = make([]byte, 10)
		n, err = rw.Read(dst)
		assert.Equal(t, io.EOF, err)
		assert.Equal(t, 0, n)
		assert.Equal(t, int64(0), rw.Size())
	})

	t.Run("Full", func(t *testing.T) {
		rw := newRW()
		defer close(rw)

		// Test full read
		dst = make([]byte, 100)
		n, err = rw.Read(dst)
		assert.Equal(t, io.EOF, err)
		assert.Equal(t, 10, n)
		assert.Equal(t, testData, dst[0:10])

		// Test read EOF
		n, err = rw.Read(dst)
		assert.Equal(t, io.EOF, err)
		assert.Equal(t, 0, n)

		// Test Seek Back to start
		dst = make([]byte, 10)
		pos, err = rw.Seek(0, io.SeekStart)
		assert.Nil(t, err)
		assert.Equal(t, 0, int(pos))

		// Now full read
		n, err = rw.Read(dst)
		assert.Nil(t, err)
		assert.Equal(t, 10, n)
		assert.Equal(t, testData, dst)
	})

	t.Run("WriteTo", func(t *testing.T) {
		rw := newRW()
		defer close(rw)
		var b bytes.Buffer

		n, err := rw.WriteTo(&b)
		assert.NoError(t, err)
		assert.Equal(t, int64(10), n)
		assert.Equal(t, testData, b.Bytes())
	})

	t.Run("WriteToError", func(t *testing.T) {
		rw := newRW()
		defer close(rw)
		w := testWriterError{}

		n, err := rw.WriteTo(w)
		assert.Equal(t, errWriteError, err)
		assert.Equal(t, int64(0), n)
	})

	t.Run("Partial", func(t *testing.T) {
		// Test partial read
		rw := newRW()
		defer close(rw)

		dst = make([]byte, 5)
		n, err = rw.Read(dst)
		assert.Nil(t, err)
		assert.Equal(t, 5, n)
		assert.Equal(t, testData[0:5], dst)
		n, err = rw.Read(dst)
		assert.Nil(t, err)
		assert.Equal(t, 5, n)
		assert.Equal(t, testData[5:], dst)
	})

	t.Run("Seek", func(t *testing.T) {
		// Test Seek
		rw := newRW()
		defer close(rw)

		// Seek to end
		pos, err = rw.Seek(10, io.SeekStart)
		assert.NoError(t, err)
		assert.Equal(t, int64(10), pos)

		// Seek to start
		pos, err = rw.Seek(0, io.SeekStart)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), pos)

		// Should not allow seek past cache index
		pos, err = rw.Seek(11, io.SeekCurrent)
		assert.Equal(t, errSeekPastEnd, err)
		assert.Equal(t, 10, int(pos))

		// Should not allow seek to negative position start
		pos, err = rw.Seek(-1, io.SeekCurrent)
		assert.Equal(t, errNegativeSeek, err)
		assert.Equal(t, 0, int(pos))

		// Should not allow seek with invalid whence
		pos, err = rw.Seek(0, 3)
		assert.Equal(t, errInvalidWhence, err)
		assert.Equal(t, 0, int(pos))

		// Should seek from index with io.SeekCurrent(1) whence
		dst = make([]byte, 5)
		_, _ = rw.Read(dst)
		pos, err = rw.Seek(-3, io.SeekCurrent)
		assert.Nil(t, err)
		assert.Equal(t, 2, int(pos))
		pos, err = rw.Seek(1, io.SeekCurrent)
		assert.Nil(t, err)
		assert.Equal(t, 3, int(pos))

		// Should seek from cache end with io.SeekEnd(2) whence
		pos, err = rw.Seek(-3, io.SeekEnd)
		assert.Nil(t, err)
		assert.Equal(t, 7, int(pos))

		// Should read from seek position and past it
		dst = make([]byte, 3)
		n, err = io.ReadFull(rw, dst)
		assert.Nil(t, err)
		assert.Equal(t, 3, n)
		assert.Equal(t, testData[7:10], dst)
	})

	t.Run("Account", func(t *testing.T) {
		errBoom := errors.New("accounting error")

		t.Run("Read", func(t *testing.T) {
			rw := newRW()
			defer close(rw)

			var total int
			rw.SetAccounting(func(n int) error {
				total += n
				return nil
			})

			dst = make([]byte, 3)
			n, err = rw.Read(dst)
			assert.Equal(t, 3, n)
			assert.NoError(t, err)
			assert.Equal(t, 3, total)
		})

		t.Run("WriteTo", func(t *testing.T) {
			rw := newRW()
			defer close(rw)
			var b bytes.Buffer

			var total int
			rw.SetAccounting(func(n int) error {
				total += n
				return nil
			})

			n, err := rw.WriteTo(&b)
			assert.NoError(t, err)
			assert.Equal(t, 10, total)
			assert.Equal(t, int64(10), n)
			assert.Equal(t, testData, b.Bytes())
		})

		t.Run("ReadDelay", func(t *testing.T) {
			rw := newRW()
			defer close(rw)

			var total int
			rw.SetAccounting(func(n int) error {
				total += n
				return nil
			})

			rewind := func() {
				_, err := rw.Seek(0, io.SeekStart)
				require.NoError(t, err)
			}

			rw.DelayAccounting(3)

			dst = make([]byte, 16)

			n, err = rw.Read(dst)
			assert.Equal(t, 10, n)
			assert.Equal(t, io.EOF, err)
			assert.Equal(t, 0, total)
			rewind()

			n, err = rw.Read(dst)
			assert.Equal(t, 10, n)
			assert.Equal(t, io.EOF, err)
			assert.Equal(t, 0, total)
			rewind()

			n, err = rw.Read(dst)
			assert.Equal(t, 10, n)
			assert.Equal(t, io.EOF, err)
			assert.Equal(t, 10, total)
			rewind()

			n, err = rw.Read(dst)
			assert.Equal(t, 10, n)
			assert.Equal(t, io.EOF, err)
			assert.Equal(t, 20, total)
			rewind()
		})

		t.Run("WriteToDelay", func(t *testing.T) {
			rw := newRW()
			defer close(rw)
			var b bytes.Buffer

			var total int
			rw.SetAccounting(func(n int) error {
				total += n
				return nil
			})

			rw.DelayAccounting(3)

			rewind := func() {
				_, err := rw.Seek(0, io.SeekStart)
				require.NoError(t, err)
				b.Reset()
			}

			n, err := rw.WriteTo(&b)
			assert.NoError(t, err)
			assert.Equal(t, 0, total)
			assert.Equal(t, int64(10), n)
			assert.Equal(t, testData, b.Bytes())
			rewind()

			n, err = rw.WriteTo(&b)
			assert.NoError(t, err)
			assert.Equal(t, 0, total)
			assert.Equal(t, int64(10), n)
			assert.Equal(t, testData, b.Bytes())
			rewind()

			n, err = rw.WriteTo(&b)
			assert.NoError(t, err)
			assert.Equal(t, 10, total)
			assert.Equal(t, int64(10), n)
			assert.Equal(t, testData, b.Bytes())
			rewind()

			n, err = rw.WriteTo(&b)
			assert.NoError(t, err)
			assert.Equal(t, 20, total)
			assert.Equal(t, int64(10), n)
			assert.Equal(t, testData, b.Bytes())
			rewind()
		})

		t.Run("ReadError", func(t *testing.T) {
			// Test accounting errors
			rw := newRW()
			defer close(rw)

			rw.SetAccounting(func(n int) error {
				return errBoom
			})

			dst = make([]byte, 3)
			n, err = rw.Read(dst)
			assert.Equal(t, 3, n)
			assert.Equal(t, errBoom, err)
		})

		t.Run("WriteToError", func(t *testing.T) {
			rw := newRW()
			defer close(rw)
			rw.SetAccounting(func(n int) error {
				return errBoom
			})
			var b bytes.Buffer

			n, err := rw.WriteTo(&b)
			assert.Equal(t, errBoom, err)
			assert.Equal(t, int64(10), n)
			assert.Equal(t, testData, b.Bytes())
		})
	})

}

// A reader to read in chunkSize chunks
type testReader struct {
	data      []byte
	chunkSize int
}

// Read in chunkSize chunks
func (r *testReader) Read(p []byte) (n int, err error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	chunkSize := r.chunkSize
	if chunkSize > len(r.data) {
		chunkSize = len(r.data)
	}
	n = copy(p, r.data[:chunkSize])
	r.data = r.data[n:]
	return n, nil
}

// A writer to write in chunkSize chunks
type testWriter struct {
	t         *testing.T
	data      []byte
	chunkSize int
	buf       []byte
	offset    int
}

// Write in chunkSize chunks
func (w *testWriter) Write(p []byte) (n int, err error) {
	if w.buf == nil {
		w.buf = make([]byte, w.chunkSize)
	}
	n = copy(w.buf, p)
	assert.Equal(w.t, w.data[w.offset:w.offset+n], w.buf[:n])
	w.offset += n
	return n, nil
}

func TestRWBoundaryConditions(t *testing.T) {
	var accounted int
	account := func(n int) error {
		accounted += n
		return nil
	}

	maxSize := 3 * blockSize
	buf := []byte(random.String(maxSize))

	sizes := []int{
		1, 2, 3,
		blockSize - 2, blockSize - 1, blockSize, blockSize + 1, blockSize + 2,
		2*blockSize - 2, 2*blockSize - 1, 2 * blockSize, 2*blockSize + 1, 2*blockSize + 2,
		3*blockSize - 2, 3*blockSize - 1, 3 * blockSize,
	}

	// Write the data in chunkSize chunks
	write := func(rw *RW, data []byte, chunkSize int) {
		writeData := data
		for len(writeData) > 0 {
			i := chunkSize
			if i > len(writeData) {
				i = len(writeData)
			}
			nn, err := rw.Write(writeData[:i])
			assert.NoError(t, err)
			assert.Equal(t, len(writeData[:i]), nn)
			writeData = writeData[nn:]
		}
	}

	// Write the data in chunkSize chunks using ReadFrom
	readFrom := func(rw *RW, data []byte, chunkSize int) {
		nn, err := rw.ReadFrom(&testReader{
			data:      data,
			chunkSize: chunkSize,
		})
		assert.NoError(t, err)
		assert.Equal(t, int64(len(data)), nn)
	}

	// Read the data back and check it is OK in chunkSize chunks
	read := func(rw *RW, data []byte, chunkSize int) {
		size := len(data)
		buf := make([]byte, chunkSize)
		offset := 0
		for {
			nn, err := rw.Read(buf)
			expectedRead := len(buf)
			if offset+chunkSize > size {
				expectedRead = size - offset
				assert.Equal(t, err, io.EOF)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, expectedRead, nn)
			assert.Equal(t, data[offset:offset+nn], buf[:nn])
			offset += nn
			if err == io.EOF {
				break
			}
		}
	}

	// Read the data back and check it is OK in chunkSize chunks using WriteTo
	writeTo := func(rw *RW, data []byte, chunkSize int) {
		nn, err := rw.WriteTo(&testWriter{
			t:         t,
			data:      data,
			chunkSize: chunkSize,
		})
		assert.NoError(t, err)
		assert.Equal(t, int64(len(data)), nn)
	}

	type test struct {
		name string
		fn   func(*RW, []byte, int)
	}

	// Read and Write the data with a range of block sizes and functions
	for _, write := range []test{{"Write", write}, {"ReadFrom", readFrom}} {
		t.Run(write.name, func(t *testing.T) {
			for _, read := range []test{{"Read", read}, {"WriteTo", writeTo}} {
				t.Run(read.name, func(t *testing.T) {
					for _, size := range sizes {
						data := buf[:size]
						for _, chunkSize := range sizes {
							//t.Logf("Testing size=%d chunkSize=%d", useWrite, size, chunkSize)
							rw := NewRW(rwPool)
							assert.Equal(t, int64(0), rw.Size())
							accounted = 0
							rw.SetAccounting(account)
							assert.Equal(t, 0, accounted)
							write.fn(rw, data, chunkSize)
							assert.Equal(t, int64(size), rw.Size())
							assert.Equal(t, 0, accounted)
							read.fn(rw, data, chunkSize)
							assert.NoError(t, rw.Close())
							assert.Equal(t, size, accounted)
						}
					}
				})
			}
		})
	}
}

// The RW should be thread safe for reading and writing concurrently
func TestRWConcurrency(t *testing.T) {
	const bufSize = 1024

	// Write data of size using Write
	write := func(rw *RW, size int64) {
		in := readers.NewPatternReader(size)
		buf := make([]byte, bufSize)
		nn := int64(0)
		for {
			nr, inErr := in.Read(buf)
			if inErr != nil && inErr != io.EOF {
				require.NoError(t, inErr)
			}
			nw, rwErr := rw.Write(buf[:nr])
			require.NoError(t, rwErr)
			assert.Equal(t, nr, nw)
			nn += int64(nw)
			if inErr == io.EOF {
				break
			}
		}
		assert.Equal(t, size, nn)
	}

	// Write the data using ReadFrom
	readFrom := func(rw *RW, size int64) {
		in := readers.NewPatternReader(size)
		nn, err := rw.ReadFrom(in)
		assert.NoError(t, err)
		assert.Equal(t, size, nn)
	}

	// Read the data back from inP and check it is OK
	check := func(in io.Reader, size int64, rw *RW) {
		ck := readers.NewPatternReader(size)
		ckBuf := make([]byte, bufSize)
		rwBuf := make([]byte, bufSize)
		nn := int64(0)
		for {
			nck, ckErr := ck.Read(ckBuf)
			if ckErr != io.EOF {
				require.NoError(t, ckErr)
			}
			var nin int
			var inErr error
			for {
				var nnin int
				nnin, inErr = in.Read(rwBuf[nin:])
				if inErr != io.EOF {
					require.NoError(t, inErr)
				}
				nin += nnin
				nn += int64(nnin)
				if nin >= len(rwBuf) || nn >= size || inErr != io.EOF {
					break
				}
				rw.WaitWrite(context.Background())
			}
			require.Equal(t, ckBuf[:nck], rwBuf[:nin])
			if ckErr == io.EOF && inErr == io.EOF {
				break
			}
		}
		assert.Equal(t, size, nn)
	}

	// Read the data back and check it is OK
	read := func(rw *RW, size int64) {
		check(rw, size, rw)
	}

	// Read the data back and check it is OK in using WriteTo
	writeTo := func(rw *RW, size int64) {
		in, out := io.Pipe()
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			check(in, size, rw)
		}()
		var n int64
		for n < size {
			nn, err := rw.WriteTo(out)
			assert.NoError(t, err)
			n += nn
		}
		assert.Equal(t, size, n)
		require.NoError(t, out.Close())
		wg.Wait()
	}

	type test struct {
		name string
		fn   func(*RW, int64)
	}

	const size = blockSize*255 + 255

	// Read and Write the data with a range of block sizes and functions
	for _, write := range []test{{"Write", write}, {"ReadFrom", readFrom}} {
		t.Run(write.name, func(t *testing.T) {
			for _, read := range []test{{"Read", read}, {"WriteTo", writeTo}} {
				t.Run(read.name, func(t *testing.T) {
					var wg sync.WaitGroup
					wg.Add(2)
					rw := NewRW(rwPool)
					go func() {
						defer wg.Done()
						read.fn(rw, size)
					}()
					go func() {
						defer wg.Done()
						write.fn(rw, size)
					}()
					wg.Wait()
				})
			}
		})
	}

}
