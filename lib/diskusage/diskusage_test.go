package diskusage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	info, err := New(".")
	if err == ErrUnsupported {
		t.Skip(err)
	}
	require.NoError(t, err)
	t.Logf("Free      %16d", info.Free)
	t.Logf("Available %16d", info.Available)
	t.Logf("Total     %16d", info.Total)
	assert.True(t, info.Total != 0)
	assert.True(t, info.Total > info.Free)
	assert.True(t, info.Total > info.Available)
	assert.True(t, info.Free >= info.Available)
}
