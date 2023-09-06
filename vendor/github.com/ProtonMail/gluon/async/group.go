package async

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

// Group is forked and improved version of "github.com/bradenaw/juniper/xsync.Group".
//
// It manages a group of goroutines. The main change to original is posibility
// to wait passed function to finish without canceling it's context and adding
// PanicHandler.
type Group struct {
	baseCtx context.Context
	ctx     context.Context
	jobCtx  context.Context
	cancel  context.CancelFunc
	finish  context.CancelFunc
	wg      sync.WaitGroup

	panicHandler PanicHandler
}

// NewGroup returns a Group ready for use. The context passed to any of the f functions will be a
// descendant of ctx.
func NewGroup(ctx context.Context, panicHandler PanicHandler) *Group {
	bgCtx, cancel := context.WithCancel(ctx)
	jobCtx, finish := context.WithCancel(ctx)

	return &Group{
		baseCtx:      ctx,
		ctx:          bgCtx,
		jobCtx:       jobCtx,
		cancel:       cancel,
		finish:       finish,
		panicHandler: panicHandler,
	}
}

// Once calls f once from another goroutine.
func (g *Group) Once(f func(ctx context.Context)) {
	g.wg.Add(1)

	go func() {
		defer HandlePanic(g.panicHandler)

		defer g.wg.Done()

		f(g.ctx)
	}()
}

// jitterDuration returns a random duration in [d - jitter, d + jitter].
func jitterDuration(d time.Duration, jitter time.Duration) time.Duration {
	return d + time.Duration(float64(jitter)*((rand.Float64()*2)-1)) //nolint:gosec
}

// Periodic spawns a goroutine that calls f once per interval +/- jitter.
func (g *Group) Periodic(
	interval time.Duration,
	jitter time.Duration,
	f func(ctx context.Context),
) {
	g.wg.Add(1)

	go func() {
		defer HandlePanic(g.panicHandler)

		defer g.wg.Done()

		t := time.NewTimer(jitterDuration(interval, jitter))
		defer t.Stop()

		for {
			if g.ctx.Err() != nil {
				return
			}

			select {
			case <-g.jobCtx.Done():
				return
			case <-t.C:
			}

			t.Reset(jitterDuration(interval, jitter))
			f(g.ctx)
		}
	}()
}

// Trigger spawns a goroutine which calls f whenever the returned function is called. If f is
// already running when triggered, f will run again immediately when it finishes.
func (g *Group) Trigger(f func(ctx context.Context)) func() {
	c := make(chan struct{}, 1)

	g.wg.Add(1)

	go func() {
		defer HandlePanic(g.panicHandler)

		defer g.wg.Done()

		for {
			if g.ctx.Err() != nil {
				return
			}
			select {
			case <-g.jobCtx.Done():
				return
			case <-c:
			}
			f(g.ctx)
		}
	}()

	return func() {
		select {
		case c <- struct{}{}:
		default:
		}
	}
}

// PeriodicOrTrigger spawns a goroutine which calls f whenever the returned function is called.  If
// f is already running when triggered, f will run again immediately when it finishes. Also calls f
// when it has been interval+/-jitter since the last trigger.
func (g *Group) PeriodicOrTrigger(
	interval time.Duration,
	jitter time.Duration,
	f func(ctx context.Context),
) func() {
	c := make(chan struct{}, 1)

	g.wg.Add(1)

	go func() {
		defer HandlePanic(g.panicHandler)

		defer g.wg.Done()

		t := time.NewTimer(jitterDuration(interval, jitter))
		defer t.Stop()

		for {
			if g.ctx.Err() != nil {
				return
			}
			select {
			case <-g.jobCtx.Done():
				return
			case <-t.C:
				t.Reset(jitterDuration(interval, jitter))
			case <-c:
				if !t.Stop() {
					<-t.C
				}

				t.Reset(jitterDuration(interval, jitter))
			}
			f(g.ctx)
		}
	}()

	return func() {
		select {
		case c <- struct{}{}:
		default:
		}
	}
}

func (g *Group) resetCtx() {
	g.jobCtx, g.finish = context.WithCancel(g.baseCtx)
	g.ctx, g.cancel = context.WithCancel(g.baseCtx)
}

// Cancel is send to all of the spawn goroutines and ends periodic
// or trigger routines.
func (g *Group) Cancel() {
	g.cancel()
	g.finish()
	g.resetCtx()
}

// Finish will ends all periodic or polls routines. It will let
// currently running functions to finish (cancel is not sent).
//
// It is not safe to call Wait concurrently with any other method on g.
func (g *Group) Finish() {
	g.finish()
	g.jobCtx, g.finish = context.WithCancel(g.baseCtx)
}

// CancelAndWait cancels the context passed to any of the spawned goroutines and waits for all spawned
// goroutines to exit.
//
// It is not safe to call Wait concurrently with any other method on g.
func (g *Group) CancelAndWait() {
	g.finish()
	g.cancel()
	g.wg.Wait()
	g.resetCtx()
}

// WaitToFinish will ends all periodic or polls routines. It will wait for
// currently running functions to finish (cancel is not sent).
//
// It is not safe to call Wait concurrently with any other method on g.
func (g *Group) WaitToFinish() {
	g.finish()
	g.wg.Wait()
	g.jobCtx, g.finish = context.WithCancel(g.baseCtx)
}
