package seafile

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestShouldAllowShutdownTwice(t *testing.T) {
	renew := NewRenew(time.Hour, func() error {
		return nil
	})
	renew.Shutdown()
	renew.Shutdown()
}

func TestRenewalInTimeLimit(t *testing.T) {
	var count atomic.Int64

	renew := NewRenew(100*time.Millisecond, func() error {
		count.Add(1)
		return nil
	})
	time.Sleep(time.Second)
	renew.Shutdown()

	// there's no guarantee the CI agent can handle a simple goroutine
	renewCount := count.Load()
	t.Logf("renew count = %d", renewCount)
	assert.Greater(t, renewCount, int64(0))
	assert.Less(t, renewCount, int64(11))
}
