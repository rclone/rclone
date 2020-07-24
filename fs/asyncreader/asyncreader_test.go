package asyncreader

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"time"

	"github.com/rclone/rclone/lib/israce"
	"github.com/rclone/rclone/lib/readers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsyncReader(t *testing.T) {
	buf := ioutil.NopCloser(bytes.NewBufferString("Testbuffer"))
	ar, err := New(buf, 4)
	require.NoError(t, err)

	var dst = make([]byte, 100)
	n, err := ar.Read(dst)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 10, n)

	n, err = ar.Read(dst)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, n)

	// Test read after error
	n, err = ar.Read(dst)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, n)

	err = ar.Close()
	require.NoError(t, err)
	// Test double close
	err = ar.Close()
	require.NoError(t, err)

	// Test Close without reading everything
	buf = ioutil.NopCloser(bytes.NewBuffer(make([]byte, 50000)))
	ar, err = New(buf, 4)
	require.NoError(t, err)
	err = ar.Close()
	require.NoError(t, err)

}

func TestAsyncWriteTo(t *testing.T) {
	buf := ioutil.NopCloser(bytes.NewBufferString("Testbuffer"))
	ar, err := New(buf, 4)
	require.NoError(t, err)

	var dst = &bytes.Buffer{}
	n, err := io.Copy(dst, ar)
	require.NoError(t, err)
	assert.Equal(t, int64(10), n)

	// Should still not return any errors
	n, err = io.Copy(dst, ar)
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)

	err = ar.Close()
	require.NoError(t, err)
}

func TestAsyncReaderErrors(t *testing.T) {
	// test nil reader
	_, err := New(nil, 4)
	require.Error(t, err)

	// invalid buffer number
	buf := ioutil.NopCloser(bytes.NewBufferString("Testbuffer"))
	_, err = New(buf, 0)
	require.Error(t, err)
	_, err = New(buf, -1)
	require.Error(t, err)
}

// Complex read tests, leveraged from "bufio".

type readMaker struct {
	name string
	fn   func(io.Reader) io.Reader
}

var readMakers = []readMaker{
	{"full", func(r io.Reader) io.Reader { return r }},
	{"byte", iotest.OneByteReader},
	{"half", iotest.HalfReader},
	{"data+err", iotest.DataErrReader},
	{"timeout", iotest.TimeoutReader},
}

// Call Read to accumulate the text of a file
func reads(buf io.Reader, m int) string {
	var b [1000]byte
	nb := 0
	for {
		n, err := buf.Read(b[nb : nb+m])
		nb += n
		if err == io.EOF {
			break
		} else if err != nil && err != iotest.ErrTimeout {
			panic("Data: " + err.Error())
		} else if err != nil {
			break
		}
	}
	return string(b[0:nb])
}

type bufReader struct {
	name string
	fn   func(io.Reader) string
}

var bufreaders = []bufReader{
	{"1", func(b io.Reader) string { return reads(b, 1) }},
	{"2", func(b io.Reader) string { return reads(b, 2) }},
	{"3", func(b io.Reader) string { return reads(b, 3) }},
	{"4", func(b io.Reader) string { return reads(b, 4) }},
	{"5", func(b io.Reader) string { return reads(b, 5) }},
	{"7", func(b io.Reader) string { return reads(b, 7) }},
}

const minReadBufferSize = 16

var bufsizes = []int{
	0, minReadBufferSize, 23, 32, 46, 64, 93, 128, 1024, 4096,
}

// Test various  input buffer sizes, number of buffers and read sizes.
func TestAsyncReaderSizes(t *testing.T) {
	var texts [31]string
	str := ""
	all := ""
	for i := 0; i < len(texts)-1; i++ {
		texts[i] = str + "\n"
		all += texts[i]
		str += string(rune(i)%26 + 'a')
	}
	texts[len(texts)-1] = all

	for h := 0; h < len(texts); h++ {
		text := texts[h]
		for i := 0; i < len(readMakers); i++ {
			for j := 0; j < len(bufreaders); j++ {
				for k := 0; k < len(bufsizes); k++ {
					for l := 1; l < 10; l++ {
						readmaker := readMakers[i]
						bufreader := bufreaders[j]
						bufsize := bufsizes[k]
						read := readmaker.fn(strings.NewReader(text))
						buf := bufio.NewReaderSize(read, bufsize)
						ar, _ := New(ioutil.NopCloser(buf), l)
						s := bufreader.fn(ar)
						// "timeout" expects the Reader to recover, AsyncReader does not.
						if s != text && readmaker.name != "timeout" {
							t.Errorf("reader=%s fn=%s bufsize=%d want=%q got=%q",
								readmaker.name, bufreader.name, bufsize, text, s)
						}
						err := ar.Close()
						require.NoError(t, err)
					}
				}
			}
		}
	}
}

// Test various input buffer sizes, number of buffers and read sizes.
func TestAsyncReaderWriteTo(t *testing.T) {
	var texts [31]string
	str := ""
	all := ""
	for i := 0; i < len(texts)-1; i++ {
		texts[i] = str + "\n"
		all += texts[i]
		str += string(rune(i)%26 + 'a')
	}
	texts[len(texts)-1] = all

	for h := 0; h < len(texts); h++ {
		text := texts[h]
		for i := 0; i < len(readMakers); i++ {
			for j := 0; j < len(bufreaders); j++ {
				for k := 0; k < len(bufsizes); k++ {
					for l := 1; l < 10; l++ {
						readmaker := readMakers[i]
						bufreader := bufreaders[j]
						bufsize := bufsizes[k]
						read := readmaker.fn(strings.NewReader(text))
						buf := bufio.NewReaderSize(read, bufsize)
						ar, _ := New(ioutil.NopCloser(buf), l)
						dst := &bytes.Buffer{}
						_, err := ar.WriteTo(dst)
						if err != nil && err != io.EOF && err != iotest.ErrTimeout {
							t.Fatal("Copy:", err)
						}
						s := dst.String()
						// "timeout" expects the Reader to recover, AsyncReader does not.
						if s != text && readmaker.name != "timeout" {
							t.Errorf("reader=%s fn=%s bufsize=%d want=%q got=%q",
								readmaker.name, bufreader.name, bufsize, text, s)
						}
						err = ar.Close()
						require.NoError(t, err)
					}
				}
			}
		}
	}
}

// Read an infinite number of zeros
type zeroReader struct {
	closed bool
}

func (z *zeroReader) Read(p []byte) (n int, err error) {
	if z.closed {
		return 0, io.EOF
	}
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func (z *zeroReader) Close() error {
	if z.closed {
		panic("double close on zeroReader")
	}
	z.closed = true
	return nil
}

// Test closing and abandoning
func testAsyncReaderClose(t *testing.T, writeto bool) {
	zr := &zeroReader{}
	a, err := New(zr, 16)
	require.NoError(t, err)
	var copyN int64
	var copyErr error
	var wg sync.WaitGroup
	started := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		close(started)
		if writeto {
			// exercise the WriteTo path
			copyN, copyErr = a.WriteTo(ioutil.Discard)
		} else {
			// exercise the Read path
			buf := make([]byte, 64*1024)
			for {
				var n int
				n, copyErr = a.Read(buf)
				copyN += int64(n)
				if copyErr != nil {
					break
				}
			}
		}
	}()
	// Do some copying
	<-started
	time.Sleep(100 * time.Millisecond)
	// Abandon the copy
	a.Abandon()
	wg.Wait()
	assert.Equal(t, ErrorStreamAbandoned, copyErr)
	// t.Logf("Copied %d bytes, err %v", copyN, copyErr)
	assert.True(t, copyN > 0)
}
func TestAsyncReaderCloseRead(t *testing.T)    { testAsyncReaderClose(t, false) }
func TestAsyncReaderCloseWriteTo(t *testing.T) { testAsyncReaderClose(t, true) }

func TestAsyncReaderSkipBytes(t *testing.T) {
	t.Parallel()
	data := make([]byte, 15000)
	buf := make([]byte, len(data))
	r := rand.New(rand.NewSource(42))

	n, err := r.Read(data)
	require.NoError(t, err)
	require.Equal(t, len(data), n)

	initialReads := []int{0, 1, 100, 2048,
		softStartInitial - 1, softStartInitial, softStartInitial + 1,
		8000, len(data)}
	skips := []int{-1000, -101, -100, -99, 0, 1, 2048,
		softStartInitial - 1, softStartInitial, softStartInitial + 1,
		8000, len(data), BufferSize, 2 * BufferSize}

	for buffers := 1; buffers <= 5; buffers++ {
		if israce.Enabled && buffers > 1 {
			t.Skip("FIXME Skipping further tests with race detector until https://github.com/golang/go/issues/27070 is fixed.")
		}
		t.Run(fmt.Sprintf("%d", buffers), func(t *testing.T) {
			for _, initialRead := range initialReads {
				t.Run(fmt.Sprintf("%d", initialRead), func(t *testing.T) {
					for _, skip := range skips {
						t.Run(fmt.Sprintf("%d", skip), func(t *testing.T) {
							ar, err := New(ioutil.NopCloser(bytes.NewReader(data)), buffers)
							require.NoError(t, err)

							wantSkipFalse := false
							buf = buf[:initialRead]
							n, err := readers.ReadFill(ar, buf)
							if initialRead >= len(data) {
								wantSkipFalse = true
								if initialRead > len(data) {
									assert.Equal(t, err, io.EOF)
								} else {
									assert.True(t, err == nil || err == io.EOF)
								}
								assert.Equal(t, len(data), n)
								assert.Equal(t, data, buf[:len(data)])
							} else {
								assert.NoError(t, err)
								assert.Equal(t, initialRead, n)
								assert.Equal(t, data[:initialRead], buf)
							}

							skipped := ar.SkipBytes(skip)
							buf = buf[:1024]
							n, err = readers.ReadFill(ar, buf)
							offset := initialRead + skip
							if skipped {
								assert.False(t, wantSkipFalse)
								l := len(buf)
								if offset >= len(data) {
									assert.Equal(t, err, io.EOF)
								} else {
									if offset+1024 >= len(data) {
										l = len(data) - offset
									}
									assert.Equal(t, l, n)
									assert.Equal(t, data[offset:offset+l], buf[:l])
								}
							} else {
								if initialRead >= len(data) {
									assert.Equal(t, err, io.EOF)
								} else {
									assert.True(t, err == ErrorStreamAbandoned || err == io.EOF)
								}
							}
						})
					}
				})
			}
		})
	}
}
