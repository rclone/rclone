package readers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextReader(t *testing.T) {
	r := NewPatternReader(100)
	ctx, cancel := context.WithCancel(context.Background())
	cr := NewContextReader(ctx, r)

	var buf = make([]byte, 3)

	n, err := cr.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, []byte{0, 1, 2}, buf)

	cancel()

	n, err = cr.Read(buf)
	assert.Equal(t, context.Canceled, err)
	assert.Equal(t, 0, n)
}
