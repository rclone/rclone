package accounting

import (
	"context"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
)

func TestLimitTPS(t *testing.T) {
	timeTransactions := func(n int, minTime, maxTime time.Duration) {
		start := time.Now()
		for i := 0; i < n; i++ {
			LimitTPS(context.Background())
		}
		dt := time.Since(start)
		assert.True(t, dt >= minTime && dt <= maxTime, "Expecting time between %v and %v, got %v", minTime, maxTime, dt)
	}

	t.Run("Off", func(t *testing.T) {
		assert.Nil(t, tpsBucket)
		timeTransactions(100, 0*time.Millisecond, 100*time.Millisecond)
	})

	t.Run("On", func(t *testing.T) {
		ctx, ci := fs.AddConfig(context.Background())
		ci.TPSLimit = 100.0
		ci.TPSLimitBurst = 0
		StartLimitTPS(ctx)
		assert.NotNil(t, tpsBucket)
		defer func() {
			tpsBucket = nil
		}()

		timeTransactions(100, 900*time.Millisecond, 5000*time.Millisecond)
	})
}
