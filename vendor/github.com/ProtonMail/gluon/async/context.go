package async

import (
	"context"
	"sync"
)

// Abortable collects groups of functions that can be aborted by calling Abort.
type Abortable struct {
	abortFunc []context.CancelFunc
	abortLock sync.RWMutex
}

func (a *Abortable) Do(ctx context.Context, fn func(context.Context)) {
	fn(a.newCancelCtx(ctx))
}

func (a *Abortable) Abort() {
	a.abortLock.RLock()
	defer a.abortLock.RUnlock()

	for _, fn := range a.abortFunc {
		fn()
	}
}

func (a *Abortable) newCancelCtx(ctx context.Context) context.Context {
	a.abortLock.Lock()
	defer a.abortLock.Unlock()

	ctx, cancel := context.WithCancel(ctx)

	a.abortFunc = append(a.abortFunc, cancel)

	return ctx
}

// RangeContext iterates over the given channel until the context is canceled or the
// channel is closed.
func RangeContext[T any](ctx context.Context, ch <-chan T, fn func(T)) {
	for {
		select {
		case v, ok := <-ch:
			if !ok {
				return
			}

			fn(v)

		case <-ctx.Done():
			return
		}
	}
}

// ForwardContext forwards all values from the src channel to the dst channel until the
// context is canceled or the src channel is closed.
func ForwardContext[T any](ctx context.Context, dst chan<- T, src <-chan T) {
	RangeContext(ctx, src, func(v T) {
		select {
		case dst <- v:
		case <-ctx.Done():
		}
	})
}
