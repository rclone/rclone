package readers

import (
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatternReader(t *testing.T) {
	b2 := make([]byte, 1)

	r := NewPatternReader(0)
	b, err := ioutil.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, []byte{}, b)
	n, err := r.Read(b2)
	require.Equal(t, io.EOF, err)
	require.Equal(t, 0, n)

	r = NewPatternReader(10)
	b, err = ioutil.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, b)
	n, err = r.Read(b2)
	require.Equal(t, io.EOF, err)
	require.Equal(t, 0, n)
}
