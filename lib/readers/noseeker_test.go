package readers

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoSeeker(t *testing.T) {
	r := bytes.NewBufferString("hello")
	rs := NoSeeker{Reader: r}

	// Check read
	b := make([]byte, 4)
	n, err := rs.Read(b)
	assert.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, []byte("hell"), b)

	// Check seek
	_, err = rs.Seek(0, io.SeekCurrent)
	assert.Equal(t, errCantSeek, err)
}

// check interfaces
var (
	_ io.Reader = NoSeeker{}
	_ io.Seeker = NoSeeker{}
)
