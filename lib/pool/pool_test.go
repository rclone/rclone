package pool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func testGetPut(t *testing.T, useMmap bool) {
	bp := New(60*time.Second, 4096, 2, useMmap)

	assert.Equal(t, 0, bp.InUse())

	b1 := bp.Get()
	assert.Equal(t, 1, bp.InUse())

	b2 := bp.Get()
	assert.Equal(t, 2, bp.InUse())

	b3 := bp.Get()
	assert.Equal(t, 3, bp.InUse())

	bp.Put(b1)
	assert.Equal(t, 3, bp.InUse())

	bp.Put(b2)
	assert.Equal(t, 3, bp.InUse())

	bp.Put(b3)
	assert.Equal(t, 2, bp.InUse())

	b1a := bp.Get()
	assert.Equal(t, b1, b1a)
	assert.Equal(t, 2, bp.InUse())

	b2a := bp.Get()
	assert.Equal(t, b1, b2a)
	assert.Equal(t, 2, bp.InUse())

	bp.Put(b1a)
	bp.Put(b2a)
	assert.Equal(t, 2, bp.InUse())

	bp.Flush()
	assert.Equal(t, 0, bp.InUse())
}

func testFlusher(t *testing.T, useMmap bool) {
	bp := New(50*time.Millisecond, 4096, 2, useMmap)

	b1 := bp.Get()
	b2 := bp.Get()
	b3 := bp.Get()
	bp.Put(b1)
	bp.Put(b2)
	bp.Put(b3)
	assert.Equal(t, 2, bp.InUse())

	checkFlushHasHappened := func() {
		var n int
		for i := 0; i < 10; i++ {
			time.Sleep(100 * time.Millisecond)
			n = bp.InUse()
			if n == 0 {
				break
			}
		}
		assert.Equal(t, 0, n)
	}

	checkFlushHasHappened()

	b1 = bp.Get()
	bp.Put(b1)
	assert.Equal(t, 1, bp.InUse())

	checkFlushHasHappened()
}

func TestPool(t *testing.T) {
	for _, useMmap := range []bool{false, true} {
		name := "make"
		if useMmap {
			name = "mmap"
		}
		t.Run(name, func(t *testing.T) {
			t.Run("GetPut", func(t *testing.T) { testGetPut(t, useMmap) })
			t.Run("Flusher", func(t *testing.T) { testFlusher(t, useMmap) })
		})
	}
}
