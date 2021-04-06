package api_test

import (
	"testing"
	"time"

	"github.com/rclone/rclone/backend/b2/api"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	emptyT api.Timestamp
	t0     = api.Timestamp(fstest.Time("1970-01-01T01:01:01.123456789Z"))
	t1     = api.Timestamp(fstest.Time("2001-02-03T04:05:06.123000000Z"))
)

func TestTimestampMarshalJSON(t *testing.T) {
	resB, err := t0.MarshalJSON()
	res := string(resB)
	require.NoError(t, err)
	assert.Equal(t, "3661123", res)

	resB, err = t1.MarshalJSON()
	res = string(resB)
	require.NoError(t, err)
	assert.Equal(t, "981173106123", res)
}

func TestTimestampUnmarshalJSON(t *testing.T) {
	var tActual api.Timestamp
	err := tActual.UnmarshalJSON([]byte("981173106123"))
	require.NoError(t, err)
	assert.Equal(t, (time.Time)(t1), (time.Time)(tActual))
}

func TestTimestampIsZero(t *testing.T) {
	assert.True(t, emptyT.IsZero())
	assert.False(t, t0.IsZero())
	assert.False(t, t1.IsZero())
}

func TestTimestampEqual(t *testing.T) {
	assert.False(t, emptyT.Equal(emptyT))
	assert.False(t, t0.Equal(emptyT))
	assert.False(t, emptyT.Equal(t0))
	assert.False(t, t0.Equal(t1))
	assert.False(t, t1.Equal(t0))
	assert.True(t, t0.Equal(t0))
	assert.True(t, t1.Equal(t1))
}
