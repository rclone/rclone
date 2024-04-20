//go:build !plan9

package sftp

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStringLock(t *testing.T) {
	var wg sync.WaitGroup
	counter := [3]int{}
	lock := newStringLock()
	const (
		outer = 10
		inner = 100
		total = outer * inner
	)
	for k := 0; k < outer; k++ {
		for j := range counter {
			wg.Add(1)
			go func(j int) {
				defer wg.Done()
				ID := fmt.Sprintf("%d", j)
				for i := 0; i < inner; i++ {
					lock.Lock(ID)
					n := counter[j]
					time.Sleep(1 * time.Millisecond)
					counter[j] = n + 1
					lock.Unlock(ID)
				}

			}(j)
		}
	}
	wg.Wait()
	assert.Equal(t, [3]int{total, total, total}, counter)
}
