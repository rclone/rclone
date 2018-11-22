package rc

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	err := WriteJSON(&buf, Params{
		"String": "hello",
		"Int":    42,
	})
	require.NoError(t, err)
	assert.Equal(t, `{
	"Int": 42,
	"String": "hello"
}
`, buf.String())
}
