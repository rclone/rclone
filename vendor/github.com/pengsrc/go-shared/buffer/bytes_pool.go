package buffer

import "sync"

// A BytesBufferPool is a type-safe wrapper around a sync.BytesBufferPool.
type BytesBufferPool struct {
	p *sync.Pool
}

// NewBytesPool constructs a new BytesBufferPool.
func NewBytesPool() BytesBufferPool {
	return BytesBufferPool{
		p: &sync.Pool{
			New: func() interface{} {
				return &BytesBuffer{bs: make([]byte, 0, defaultSize)}
			},
		},
	}
}

// Get retrieves a BytesBuffer from the pool, creating one if necessary.
func (p BytesBufferPool) Get() *BytesBuffer {
	buf := p.p.Get().(*BytesBuffer)
	buf.Reset()
	buf.pool = p
	return buf
}

func (p BytesBufferPool) put(buf *BytesBuffer) {
	p.p.Put(buf)
}

// GlobalBytesPool returns the global buffer pool.
func GlobalBytesPool() *BytesBufferPool {
	return &bytesPool
}

// bytesPool is a pool of buffer bytes.
var bytesPool = NewBytesPool()
