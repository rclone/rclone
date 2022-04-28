package accounting

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs/rc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestRcBwLimit(t *testing.T) {
	call := rc.Calls.Get("core/bwlimit")
	assert.NotNil(t, call)

	// Set
	in := rc.Params{
		"rate": "1M",
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params{
		"bytesPerSecond":   int64(1048576),
		"bytesPerSecondTx": int64(1048576),
		"bytesPerSecondRx": int64(1048576),
		"rate":             "1Mi",
	}, out)
	assert.Equal(t, rate.Limit(1048576), TokenBucket.curr[0].Limit())

	// Query
	in = rc.Params{}
	out, err = call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params{
		"bytesPerSecond":   int64(1048576),
		"bytesPerSecondTx": int64(1048576),
		"bytesPerSecondRx": int64(1048576),
		"rate":             "1Mi",
	}, out)

	// Set
	in = rc.Params{
		"rate": "10M:1M",
	}
	out, err = call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params{
		"bytesPerSecond":   int64(10485760),
		"bytesPerSecondTx": int64(10485760),
		"bytesPerSecondRx": int64(1048576),
		"rate":             "10Mi:1Mi",
	}, out)
	assert.Equal(t, rate.Limit(10485760), TokenBucket.curr[0].Limit())

	// Query
	in = rc.Params{}
	out, err = call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params{
		"bytesPerSecond":   int64(10485760),
		"bytesPerSecondTx": int64(10485760),
		"bytesPerSecondRx": int64(1048576),
		"rate":             "10Mi:1Mi",
	}, out)

	// Reset
	in = rc.Params{
		"rate": "off",
	}
	out, err = call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params{
		"bytesPerSecond":   int64(-1),
		"bytesPerSecondTx": int64(-1),
		"bytesPerSecondRx": int64(-1),
		"rate":             "off",
	}, out)
	assert.Nil(t, TokenBucket.curr[0])

	// Query
	in = rc.Params{}
	out, err = call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params{
		"bytesPerSecond":   int64(-1),
		"bytesPerSecondTx": int64(-1),
		"bytesPerSecondRx": int64(-1),
		"rate":             "off",
	}, out)

}
