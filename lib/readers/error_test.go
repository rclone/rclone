package readers

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorReader(t *testing.T) {
	errRead := errors.New("boom")
	r := ErrorReader{errRead}

	buf := make([]byte, 16)
	n, err := r.Read(buf)
	assert.Equal(t, errRead, err)
	assert.Equal(t, 0, n)
}
