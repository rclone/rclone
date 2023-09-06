package noiseconn

import (
	"sync"
)

type barrier struct {
	mtx      sync.Mutex
	cv       *sync.Cond
	released bool
}

func (b *barrier) Release() {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	if b.released {
		return
	}
	b.released = true
	if b.cv != nil {
		b.cv.Broadcast()
	}
}

func (b *barrier) Wait() {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	if b.cv == nil {
		b.cv = sync.NewCond(&b.mtx)
	}
	for !b.released {
		b.cv.Wait()
	}
}
