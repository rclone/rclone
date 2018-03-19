package buffer

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuffers(t *testing.T) {
	const dummyData = "dummy data"
	p := NewBytesPool()

	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			for i := 0; i < 100; i++ {
				buf := p.Get()
				assert.Zero(t, buf.Len(), "Expected truncated buffer")
				assert.NotZero(t, buf.Cap(), "Expected non-zero capacity")

				buf.AppendString(dummyData)
				assert.Equal(t, buf.Len(), len(dummyData), "Expected buffer to contain dummy data")

				buf.Free()
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestGlobalBytesPool(t *testing.T) {
	assert.NotNil(t, GlobalBytesPool())
}
