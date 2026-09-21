package accounting

import (
	"context"
	"math"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestTokenBucketBurstScalesLargeBandwidthWithoutOverflow(t *testing.T) {
	bandwidth := 4 * fs.Tebi
	want := bandwidth / tokenBucketBurstScale
	if want > fs.SizeSuffix(math.MaxInt) {
		want = fs.SizeSuffix(math.MaxInt)
	}

	tb := newEmptyTokenBucket(bandwidth)
	require.NotNil(t, tb)
	assert.Equal(t, rate.Limit(bandwidth), tb.Limit())
	assert.Equal(t, int(want), tb.Burst())
}

func TestTokenBucketBurstCapsAtMaxInt(t *testing.T) {
	want := fs.SizeSuffix(fs.SizeSuffixMaxValue / tokenBucketBurstScale)
	if want > fs.SizeSuffix(math.MaxInt) {
		want = fs.SizeSuffix(math.MaxInt)
	}

	assert.Equal(t, int(want), tokenBucketBurst(fs.SizeSuffixMaxValue))
}

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

// Check setting the limit records it and respects the SIGUSR2 toggle
func TestSetBwLimitToggledOff(t *testing.T) {
	oldCurr, oldPrev, oldToggledOff, oldCurrLimit := TokenBucket.curr, TokenBucket.prev, TokenBucket.toggledOff, TokenBucket.currLimit
	defer func() {
		TokenBucket.curr, TokenBucket.prev = oldCurr, oldPrev
		TokenBucket.toggledOff, TokenBucket.currLimit = oldToggledOff, oldCurrLimit
	}()
	TokenBucket.curr, TokenBucket.prev = buckets{}, buckets{}
	TokenBucket.toggledOff, TokenBucket.currLimit = false, fs.BwTimeSlot{}

	// Normally the limit is set straight away
	TokenBucket.SetBwLimit(fs.BwPair{Tx: 1024, Rx: 1024})
	require.NotNil(t, TokenBucket.curr[TokenBucketSlotTransportTx])
	assert.Equal(t, rate.Limit(1024), TokenBucket.curr[TokenBucketSlotTransportTx].Limit())

	// The limit is recorded, otherwise SIGUSR2 ignores it
	assert.True(t, TokenBucket.currLimit.Bandwidth.IsSet())

	// Toggled off with SIGUSR2 - curr is off and prev holds the limits
	TokenBucket.toggledOff = true
	TokenBucket.curr, TokenBucket.prev = TokenBucket.prev, TokenBucket.curr

	// Setting a limit now must not turn the limits back on
	TokenBucket.SetBwLimit(fs.BwPair{Tx: 2048, Rx: 2048})
	assert.Nil(t, TokenBucket.curr[TokenBucketSlotTransportTx], "limits toggled off must stay off")

	// It becomes active on the next toggle
	TokenBucket.toggledOff = false
	TokenBucket.curr, TokenBucket.prev = TokenBucket.prev, TokenBucket.curr
	require.NotNil(t, TokenBucket.curr[TokenBucketSlotTransportTx])
	assert.Equal(t, rate.Limit(2048), TokenBucket.curr[TokenBucketSlotTransportTx].Limit())
}
