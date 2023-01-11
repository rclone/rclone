package seafile

import (
	"sync"
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

func TestRenewal(t *testing.T) {
	var count int64

	wg := sync.WaitGroup{}
	wg.Add(2) // run the renewal twice
	renew := NewRenew(time.Millisecond, func() error {
		atomic.AddInt64(&count, 1)
		wg.Done()
		return nil
	})
	wg.Wait()
	renew.Shutdown()

	// it is technically possible that a third renewal gets triggered between Wait() and Shutdown()
	assert.GreaterOrEqual(t, atomic.LoadInt64(&count), int64(2))
}
