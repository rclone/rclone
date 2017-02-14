package fs

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsyncReader(t *testing.T) {
	buf := ioutil.NopCloser(bytes.NewBufferString("Testbuffer"))
	ar, err := newAsyncReader(buf, 4)
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
	ar, err = newAsyncReader(buf, 4)
	require.NoError(t, err)
	err = ar.Close()
	require.NoError(t, err)

}

func TestAsyncWriteTo(t *testing.T) {
	buf := ioutil.NopCloser(bytes.NewBufferString("Testbuffer"))
	ar, err := newAsyncReader(buf, 4)
	require.NoError(t, err)

	var dst = &bytes.Buffer{}
	n, err := io.Copy(dst, ar)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, int64(10), n)

	// Should still return EOF
	n, err = io.Copy(dst, ar)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, int64(0), n)

	err = ar.Close()
	require.NoError(t, err)
}

func TestAsyncReaderErrors(t *testing.T) {
	// test nil reader
	_, err := newAsyncReader(nil, 4)
	require.Error(t, err)

	// invalid buffer number
	buf := ioutil.NopCloser(bytes.NewBufferString("Testbuffer"))
	_, err = newAsyncReader(buf, 0)
	require.Error(t, err)
	_, err = newAsyncReader(buf, -1)
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
		str += string(i%26 + 'a')
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
						ar, _ := newAsyncReader(ioutil.NopCloser(buf), l)
						s := bufreader.fn(ar)
						// "timeout" expects the Reader to recover, asyncReader does not.
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
		str += string(i%26 + 'a')
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
						ar, _ := newAsyncReader(ioutil.NopCloser(buf), l)
						dst := &bytes.Buffer{}
						wt := ar.(io.WriterTo)
						_, err := wt.WriteTo(dst)
						if err != nil && err != io.EOF && err != iotest.ErrTimeout {
							t.Fatal("Copy:", err)
						}
						s := dst.String()
						// "timeout" expects the Reader to recover, asyncReader does not.
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
