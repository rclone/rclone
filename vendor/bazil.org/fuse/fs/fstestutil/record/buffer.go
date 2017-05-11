package record

import (
	"bytes"
	"io"
	"sync"
)

// Buffer is like bytes.Buffer but safe to access from multiple
// goroutines.
type Buffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

var _ = io.Writer(&Buffer{})

func (b *Buffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *Buffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Bytes()
}
