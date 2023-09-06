package async

import (
	"context"
	"github.com/ProtonMail/gluon/logging"
	"sync"
)

// QueuedChannel represents a channel on which queued items can be published without having to worry if the reader
// has actually consumed existing items first or if there's no way of knowing ahead of time what the ideal channel
// buffer size should be.
type QueuedChannel[T any] struct {
	ch     chan T
	stopCh chan struct{}
	items  []T
	cond   *sync.Cond
	closed atomicBool // Should use atomic.Bool once we use Go 1.19!
	name   string     // for debugging
}

func NewQueuedChannel[T any](chanBufferSize, queueCapacity int, panicHandler PanicHandler, name string) *QueuedChannel[T] {
	queue := &QueuedChannel[T]{
		ch:     make(chan T, chanBufferSize),
		stopCh: make(chan struct{}),
		items:  make([]T, 0, queueCapacity),
		cond:   sync.NewCond(&sync.Mutex{}),
		name:   name,
	}

	// The queue is initially not closed.
	queue.closed.store(false)

	// Start the queue consumer.
	GoAnnotated(context.Background(), panicHandler, func(ctx context.Context) {
		defer close(queue.ch)

		for {
			item, ok := queue.pop()
			if !ok {
				return
			}

			select {
			case queue.ch <- item:
				continue

			case <-queue.stopCh:
				return
			}
		}
	}, logging.Labels{"name": name})

	return queue
}

func (q *QueuedChannel[T]) Enqueue(items ...T) bool {
	if q.closed.load() {
		return false
	}

	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	q.items = append(q.items, items...)

	q.cond.Broadcast()

	return true
}

func (q *QueuedChannel[T]) GetChannel() <-chan T {
	return q.ch
}

func (q *QueuedChannel[T]) Close() {
	q.closed.store(true)

	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	q.cond.Broadcast()
}

// CloseAndDiscardQueued force closes the channel and does not guarantee that the remaining queued items will be read.
func (q *QueuedChannel[T]) CloseAndDiscardQueued() {
	close(q.stopCh)
	q.Close()
}

func (q *QueuedChannel[T]) pop() (T, bool) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	var item T

	// Wait until there are items to pop, returning false immediately if the queue is closed.
	// This allows the queue to continue popping elements if it's closed,
	// but will prevent it from hanging indefinitely once it runs out of items.
	for len(q.items) == 0 {
		if q.closed.load() {
			return item, false
		}

		q.cond.Wait()
	}

	item, q.items = q.items[0], q.items[1:]

	return item, true
}
