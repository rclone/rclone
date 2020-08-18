package pool

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/rclone/rclone/fstest/testy"
	"github.com/stretchr/testify/assert"
)

// makes the allocations be unreliable
func makeUnreliable(bp *Pool) {
	bp.alloc = func(size int) ([]byte, error) {
		if rand.Intn(3) != 0 {
			return nil, errors.New("failed to allocate memory")
		}
		return make([]byte, size), nil
	}
	bp.free = func(b []byte) error {
		if rand.Intn(3) != 0 {
			return errors.New("failed to free memory")
		}
		return nil
	}
}

func testGetPut(t *testing.T, useMmap bool, unreliable bool) {
	bp := New(60*time.Second, 4096, 2, useMmap)
	if unreliable {
		makeUnreliable(bp)
	}

	assert.Equal(t, 0, bp.InUse())

	b1 := bp.Get()
	assert.Equal(t, 1, bp.InUse())
	assert.Equal(t, 0, bp.InPool())
	assert.Equal(t, 1, bp.Alloced())

	b2 := bp.Get()
	assert.Equal(t, 2, bp.InUse())
	assert.Equal(t, 0, bp.InPool())
	assert.Equal(t, 2, bp.Alloced())

	b3 := bp.Get()
	assert.Equal(t, 3, bp.InUse())
	assert.Equal(t, 0, bp.InPool())
	assert.Equal(t, 3, bp.Alloced())

	bp.Put(b1)
	assert.Equal(t, 2, bp.InUse())
	assert.Equal(t, 1, bp.InPool())
	assert.Equal(t, 3, bp.Alloced())

	bp.Put(b2)
	assert.Equal(t, 1, bp.InUse())
	assert.Equal(t, 2, bp.InPool())
	assert.Equal(t, 3, bp.Alloced())

	bp.Put(b3)
	assert.Equal(t, 0, bp.InUse())
	assert.Equal(t, 2, bp.InPool())
	assert.Equal(t, 2, bp.Alloced())

	addr := func(b []byte) string {
		return fmt.Sprintf("%p", &b[0])
	}
	b1a := bp.Get()
	assert.Equal(t, addr(b2), addr(b1a))
	assert.Equal(t, 1, bp.InUse())
	assert.Equal(t, 1, bp.InPool())
	assert.Equal(t, 2, bp.Alloced())

	b2a := bp.Get()
	assert.Equal(t, addr(b1), addr(b2a))
	assert.Equal(t, 2, bp.InUse())
	assert.Equal(t, 0, bp.InPool())
	assert.Equal(t, 2, bp.Alloced())

	bp.Put(b1a)
	bp.Put(b2a)
	assert.Equal(t, 0, bp.InUse())
	assert.Equal(t, 2, bp.InPool())
	assert.Equal(t, 2, bp.Alloced())

	assert.Panics(t, func() {
		bp.Put(make([]byte, 1))
	})

	bp.Flush()
	assert.Equal(t, 0, bp.InUse())
	assert.Equal(t, 0, bp.InPool())
	assert.Equal(t, 0, bp.Alloced())
}

func testFlusher(t *testing.T, useMmap bool, unreliable bool) {
	bp := New(50*time.Millisecond, 4096, 2, useMmap)
	if unreliable {
		makeUnreliable(bp)
	}

	b1 := bp.Get()
	b2 := bp.Get()
	b3 := bp.Get()
	bp.Put(b1)
	bp.Put(b2)
	bp.Put(b3)
	assert.Equal(t, 0, bp.InUse())
	assert.Equal(t, 2, bp.InPool())
	assert.Equal(t, 2, bp.Alloced())
	bp.mu.Lock()
	assert.Equal(t, 0, bp.minFill)
	assert.Equal(t, true, bp.flushPending)
	bp.mu.Unlock()

	checkFlushHasHappened := func(desired int) {
		var n int
		for i := 0; i < 10; i++ {
			time.Sleep(100 * time.Millisecond)
			n = bp.InPool()
			if n <= desired {
				break
			}
		}
		assert.Equal(t, desired, n)
	}

	checkFlushHasHappened(0)
	assert.Equal(t, 0, bp.InUse())
	assert.Equal(t, 0, bp.InPool())
	assert.Equal(t, 0, bp.Alloced())
	bp.mu.Lock()
	assert.Equal(t, 0, bp.minFill)
	assert.Equal(t, false, bp.flushPending)
	bp.mu.Unlock()

	// Now do manual aging to check it is working properly
	bp = New(100*time.Second, 4096, 2, useMmap)

	// Check the new one doesn't get flushed
	b1 = bp.Get()
	b2 = bp.Get()
	bp.Put(b1)
	bp.Put(b2)

	bp.mu.Lock()
	assert.Equal(t, 0, bp.minFill)
	assert.Equal(t, true, bp.flushPending)
	bp.mu.Unlock()

	bp.flushAged()

	assert.Equal(t, 0, bp.InUse())
	assert.Equal(t, 2, bp.InPool())
	assert.Equal(t, 2, bp.Alloced())
	bp.mu.Lock()
	assert.Equal(t, 2, bp.minFill)
	assert.Equal(t, true, bp.flushPending)
	bp.mu.Unlock()

	bp.Put(bp.Get())

	assert.Equal(t, 0, bp.InUse())
	assert.Equal(t, 2, bp.InPool())
	assert.Equal(t, 2, bp.Alloced())
	bp.mu.Lock()
	assert.Equal(t, 1, bp.minFill)
	assert.Equal(t, true, bp.flushPending)
	bp.mu.Unlock()

	bp.flushAged()

	assert.Equal(t, 0, bp.InUse())
	assert.Equal(t, 1, bp.InPool())
	assert.Equal(t, 1, bp.Alloced())
	bp.mu.Lock()
	assert.Equal(t, 1, bp.minFill)
	assert.Equal(t, true, bp.flushPending)
	bp.mu.Unlock()

	bp.flushAged()

	assert.Equal(t, 0, bp.InUse())
	assert.Equal(t, 0, bp.InPool())
	assert.Equal(t, 0, bp.Alloced())
	bp.mu.Lock()
	assert.Equal(t, 0, bp.minFill)
	assert.Equal(t, false, bp.flushPending)
	bp.mu.Unlock()
}

func TestPool(t *testing.T) {
	for _, test := range []struct {
		name       string
		useMmap    bool
		unreliable bool
	}{
		{
			name:       "make",
			useMmap:    false,
			unreliable: false,
		},
		{
			name:       "mmap",
			useMmap:    true,
			unreliable: false,
		},
		{
			name:       "canFail",
			useMmap:    false,
			unreliable: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Run("GetPut", func(t *testing.T) { testGetPut(t, test.useMmap, test.unreliable) })
			t.Run("Flusher", func(t *testing.T) {
				if test.name == "canFail" {
					testy.SkipUnreliable(t) // fails regularly on macOS
				}
				testFlusher(t, test.useMmap, test.unreliable)
			})
		})
	}
}
