package sftp

// bufPool provides a pool of byte-slices to be reused in various parts of the package.
// It is safe to use concurrently through a pointer.
type bufPool struct {
	ch   chan []byte
	blen int
}

func newBufPool(depth, bufLen int) *bufPool {
	return &bufPool{
		ch:   make(chan []byte, depth),
		blen: bufLen,
	}
}

func (p *bufPool) Get() []byte {
	if p.blen <= 0 {
		panic("bufPool: new buffer creation length must be greater than zero")
	}

	for {
		select {
		case b := <-p.ch:
			if cap(b) < p.blen {
				// just in case: throw away any buffer with insufficient capacity.
				continue
			}

			return b[:p.blen]

		default:
			return make([]byte, p.blen)
		}
	}
}

func (p *bufPool) Put(b []byte) {
	if p == nil {
		// functional default: no reuse.
		return
	}

	if cap(b) < p.blen || cap(b) > p.blen*2 {
		// DO NOT reuse buffers with insufficient capacity.
		// This could cause panics when resizing to p.blen.

		// DO NOT reuse buffers with excessive capacity.
		// This could cause memory leaks.
		return
	}

	select {
	case p.ch <- b:
	default:
	}
}

type resChanPool chan chan result

func newResChanPool(depth int) resChanPool {
	return make(chan chan result, depth)
}

func (p resChanPool) Get() chan result {
	select {
	case ch := <-p:
		return ch
	default:
		return make(chan result, 1)
	}
}

func (p resChanPool) Put(ch chan result) {
	select {
	case p <- ch:
	default:
	}
}
