package sync

import (
	"context"
	"math/bits"
	"strconv"
	"strings"
	"sync"

	"github.com/aalpar/deheap"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
)

// compare two items for order by
type lessFn func(a, b fs.ObjectPair) bool

// pipe provides an unbounded channel like experience
//
// Note unlike channels these aren't strictly ordered.
type pipe struct {
	mu        sync.Mutex
	c         chan struct{}
	queue     []fs.ObjectPair
	closed    bool
	totalSize int64
	stats     func(items int, totalSize int64)
	less      lessFn
	fraction  int
}

func newPipe(orderBy string, stats func(items int, totalSize int64), maxBacklog int) (*pipe, error) {
	if maxBacklog < 0 {
		maxBacklog = (1 << (bits.UintSize - 1)) - 1 // largest posititive int
	}
	less, fraction, err := newLess(orderBy)
	if err != nil {
		return nil, fserrors.FatalError(err)
	}
	p := &pipe{
		c:        make(chan struct{}, maxBacklog),
		stats:    stats,
		less:     less,
		fraction: fraction,
	}
	if p.less != nil {
		deheap.Init(p)
	}
	return p, nil
}

// Len satisfy heap.Interface - must be called with lock held
func (p *pipe) Len() int {
	return len(p.queue)
}

// Len satisfy heap.Interface - must be called with lock held
func (p *pipe) Less(i, j int) bool {
	return p.less(p.queue[i], p.queue[j])
}

// Swap satisfy heap.Interface - must be called with lock held
func (p *pipe) Swap(i, j int) {
	p.queue[i], p.queue[j] = p.queue[j], p.queue[i]
}

// Push satisfy heap.Interface - must be called with lock held
func (p *pipe) Push(item interface{}) {
	p.queue = append(p.queue, item.(fs.ObjectPair))
}

// Pop satisfy heap.Interface - must be called with lock held
func (p *pipe) Pop() interface{} {
	old := p.queue
	n := len(old)
	item := old[n-1]
	old[n-1] = fs.ObjectPair{} // avoid memory leak
	p.queue = old[0 : n-1]
	return item
}

// Put a pair into the pipe
//
// It returns ok = false if the context was cancelled
//
// It will panic if you call it after Close()
func (p *pipe) Put(ctx context.Context, pair fs.ObjectPair) (ok bool) {
	if ctx.Err() != nil {
		return false
	}
	p.mu.Lock()
	if p.less == nil {
		// no order-by
		p.queue = append(p.queue, pair)
	} else {
		deheap.Push(p, pair)
	}
	size := pair.Src.Size()
	if size > 0 {
		p.totalSize += size
	}
	p.stats(len(p.queue), p.totalSize)
	p.mu.Unlock()
	select {
	case <-ctx.Done():
		return false
	case p.c <- struct{}{}:
	}
	return true
}

// Get a pair from the pipe
//
// If fraction is > the mixed fraction set in the pipe then it gets it
// from the other end of the heap if order-by is in effect
//
// It returns ok = false if the context was cancelled or Close() has
// been called.
func (p *pipe) GetMax(ctx context.Context, fraction int) (pair fs.ObjectPair, ok bool) {
	if ctx.Err() != nil {
		return
	}
	select {
	case <-ctx.Done():
		return
	case _, ok = <-p.c:
		if !ok {
			return
		}
	}
	p.mu.Lock()
	if p.less == nil {
		// no order-by
		pair = p.queue[0]
		p.queue[0] = fs.ObjectPair{} // avoid memory leak
		p.queue = p.queue[1:]
	} else if p.fraction < 0 || fraction < p.fraction {
		pair = deheap.Pop(p).(fs.ObjectPair)
	} else {
		pair = deheap.PopMax(p).(fs.ObjectPair)
	}
	size := pair.Src.Size()
	if size > 0 {
		p.totalSize -= size
	}
	if p.totalSize < 0 {
		p.totalSize = 0
	}
	p.stats(len(p.queue), p.totalSize)
	p.mu.Unlock()
	return pair, true
}

// Get a pair from the pipe
//
// It returns ok = false if the context was cancelled or Close() has
// been called.
func (p *pipe) Get(ctx context.Context) (pair fs.ObjectPair, ok bool) {
	return p.GetMax(ctx, -1)
}

// Stats reads the number of items in the queue and the totalSize
func (p *pipe) Stats() (items int, totalSize int64) {
	p.mu.Lock()
	items, totalSize = len(p.queue), p.totalSize
	p.mu.Unlock()
	return items, totalSize
}

// Close the pipe
//
// Writes to a closed pipe will panic as will double closing a pipe
func (p *pipe) Close() {
	p.mu.Lock()
	close(p.c)
	p.closed = true
	p.mu.Unlock()
}

// newLess returns a less function for the heap comparison or nil if
// one is not required
func newLess(orderBy string) (less lessFn, fraction int, err error) {
	fraction = -1
	if orderBy == "" {
		return nil, fraction, nil
	}
	parts := strings.Split(strings.ToLower(orderBy), ",")
	switch parts[0] {
	case "name":
		less = func(a, b fs.ObjectPair) bool {
			return a.Src.Remote() < b.Src.Remote()
		}
	case "size":
		less = func(a, b fs.ObjectPair) bool {
			return a.Src.Size() < b.Src.Size()
		}
	case "modtime":
		less = func(a, b fs.ObjectPair) bool {
			ctx := context.Background()
			return a.Src.ModTime(ctx).Before(b.Src.ModTime(ctx))
		}
	default:
		return nil, fraction, errors.Errorf("unknown --order-by comparison %q", parts[0])
	}
	descending := false
	if len(parts) > 1 {
		switch parts[1] {
		case "ascending", "asc":
		case "descending", "desc":
			descending = true
		case "mixed":
			fraction = 50
			if len(parts) > 2 {
				fraction, err = strconv.Atoi(parts[2])
				if err != nil {
					return nil, fraction, errors.Errorf("bad mixed fraction --order-by %q", parts[2])
				}
			}

		default:
			return nil, fraction, errors.Errorf("unknown --order-by sort direction %q", parts[1])
		}
	}
	if (fraction >= 0 && len(parts) > 3) || (fraction < 0 && len(parts) > 2) {
		return nil, fraction, errors.Errorf("bad --order-by string %q", orderBy)
	}
	if descending {
		oldLess := less
		less = func(a, b fs.ObjectPair) bool {
			return !oldLess(a, b)
		}
	}
	return less, fraction, nil
}
