package vfstest

import (
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestReadByByte reads by byte including don't read any bytes
func TestReadByByte(t *testing.T) {
	run.skipIfNoFUSE(t)

	var data = []byte("hellohello")
	run.createFile(t, "testfile", string(data))
	run.checkDir(t, "testfile 10")

	for i := 0; i < len(data); i++ {
		fd, err := run.os.Open(run.path("testfile"))
		assert.NoError(t, err)
		for j := 0; j < i; j++ {
			buf := make([]byte, 1)
			n, err := io.ReadFull(fd, buf)
			assert.NoError(t, err)
			assert.Equal(t, 1, n)
			assert.Equal(t, buf[0], data[j])
		}
		err = fd.Close()
		assert.NoError(t, err)
	}

	run.rm(t, "testfile")
}

// TestReadChecksum checks the checksum reading is working
func TestReadChecksum(t *testing.T) {
	run.skipIfNoFUSE(t)

	// create file big enough so we exceed any single FUSE read
	// request
	b := make([]rune, 3*128*1024)
	for i := range b {
		b[i] = 'r'
	}
	run.createFile(t, "bigfile", string(b))

	// The hash comparison would fail in Flush, if we did not
	// ensure we read the whole file
	fd, err := run.os.Open(run.path("bigfile"))
	assert.NoError(t, err)
	buf := make([]byte, 10)
	_, err = io.ReadFull(fd, buf)
	assert.NoError(t, err)
	err = fd.Close()
	assert.NoError(t, err)

	// The hash comparison would fail, because we only read parts
	// of the file
	fd, err = run.os.Open(run.path("bigfile"))
	assert.NoError(t, err)
	// read at start
	_, err = io.ReadFull(fd, buf)
	assert.NoError(t, err)
	// read at end
	_, err = fd.Seek(int64(len(b)-len(buf)), io.SeekStart)
	assert.NoError(t, err)
	_, err = io.ReadFull(fd, buf)
	assert.NoError(t, err)
	// ensure we don't compare hashes
	err = fd.Close()
	assert.NoError(t, err)

	run.rm(t, "bigfile")
}

// TestReadSeek test seeking
func TestReadSeek(t *testing.T) {
	run.skipIfNoFUSE(t)

	var data = []byte("helloHELLO")
	run.createFile(t, "testfile", string(data))
	run.checkDir(t, "testfile 10")

	fd, err := run.os.Open(run.path("testfile"))
	assert.NoError(t, err)

	// Seek to half way
	_, err = fd.Seek(5, io.SeekStart)
	assert.NoError(t, err)

	buf, err := ioutil.ReadAll(fd)
	assert.NoError(t, err)
	assert.Equal(t, buf, []byte("HELLO"))

	// Test seeking to the end
	_, err = fd.Seek(10, io.SeekStart)
	assert.NoError(t, err)

	buf, err = ioutil.ReadAll(fd)
	assert.NoError(t, err)
	assert.Equal(t, buf, []byte(""))

	// Test seeking beyond the end
	_, err = fd.Seek(1000000, io.SeekStart)
	assert.NoError(t, err)

	buf, err = ioutil.ReadAll(fd)
	assert.NoError(t, err)
	assert.Equal(t, buf, []byte(""))

	// Now back to the start
	_, err = fd.Seek(0, io.SeekStart)
	assert.NoError(t, err)

	buf, err = ioutil.ReadAll(fd)
	assert.NoError(t, err)
	assert.Equal(t, buf, []byte("helloHELLO"))

	err = fd.Close()
	assert.NoError(t, err)

	run.rm(t, "testfile")
}
