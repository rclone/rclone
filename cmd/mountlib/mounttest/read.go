package mounttest

import (
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Read by byte including don't read any bytes
func TestReadByByte(t *testing.T) {
	run.skipIfNoFUSE(t)

	var data = []byte("hellohello")
	run.createFile(t, "testfile", string(data))
	run.checkDir(t, "testfile 10")

	for i := 0; i < len(data); i++ {
		fd, err := os.Open(run.path("testfile"))
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

// Test seeking
func TestReadSeek(t *testing.T) {
	run.skipIfNoFUSE(t)

	var data = []byte("helloHELLO")
	run.createFile(t, "testfile", string(data))
	run.checkDir(t, "testfile 10")

	fd, err := os.Open(run.path("testfile"))
	assert.NoError(t, err)

	// Seek to half way
	_, err = fd.Seek(5, 0)
	assert.NoError(t, err)

	buf, err := ioutil.ReadAll(fd)
	assert.NoError(t, err)
	assert.Equal(t, buf, []byte("HELLO"))

	// Test seeking to the end
	_, err = fd.Seek(10, 0)
	assert.NoError(t, err)

	buf, err = ioutil.ReadAll(fd)
	assert.NoError(t, err)
	assert.Equal(t, buf, []byte(""))

	// Test seeking beyond the end
	_, err = fd.Seek(1000000, 0)
	assert.NoError(t, err)

	buf, err = ioutil.ReadAll(fd)
	assert.NoError(t, err)
	assert.Equal(t, buf, []byte(""))

	// Now back to the start
	_, err = fd.Seek(0, 0)
	assert.NoError(t, err)

	buf, err = ioutil.ReadAll(fd)
	assert.NoError(t, err)
	assert.Equal(t, buf, []byte("helloHELLO"))

	err = fd.Close()
	assert.NoError(t, err)

	run.rm(t, "testfile")
}
