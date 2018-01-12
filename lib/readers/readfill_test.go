package readers

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

type byteReader struct {
	c byte
}

func (br *byteReader) Read(p []byte) (n int, err error) {
	if br.c == 0 {
		err = io.EOF
	} else if len(p) >= 1 {
		p[0] = br.c
		n = 1
		br.c--
	}
	return
}

func TestReadFill(t *testing.T) {
	buf := []byte{9, 9, 9, 9, 9}

	n, err := ReadFill(&byteReader{0}, buf)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, n)
	assert.Equal(t, []byte{9, 9, 9, 9, 9}, buf)

	n, err = ReadFill(&byteReader{3}, buf)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, []byte{3, 2, 1, 9, 9}, buf)

	n, err = ReadFill(&byteReader{8}, buf)
	assert.Equal(t, nil, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, []byte{8, 7, 6, 5, 4}, buf)
}
