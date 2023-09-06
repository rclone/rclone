// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package scheduler

import (
	"context"
	"sync"
)

// Scheduler is a type to regulate a number of resources held by handles
// with the property that earlier acquired handles get preference for
// new resources over later acquired handles.
type Scheduler struct {
	opts  Options
	rsema chan struct{}
	hsema chan struct{}

	mu      sync.Mutex
	prio    int
	waiters []*handle
}

// Options controls the parameters of the Scheduler.
type Options struct {
	MaximumConcurrent        int // number of maximum concurrent resources
	MaximumConcurrentHandles int // number of maximum concurrent handles
}

// New constructs a new Scheduler.
func New(opts Options) *Scheduler {
	var hsema chan struct{}
	if opts.MaximumConcurrentHandles > 0 {
		hsema = make(chan struct{}, opts.MaximumConcurrentHandles)
	}

	return &Scheduler{
		opts:  opts,
		rsema: make(chan struct{}, opts.MaximumConcurrent),
		hsema: hsema,
	}
}

func (s *Scheduler) resourceGet(ctx context.Context, h *handle) bool {
	// ensure that we don't return new resources if the context is
	// already canceled.
	if ctx.Err() != nil {
		return false
	}

	// fast path: if we have a semaphore slot, then immediately return it.
	select {
	default:
	case s.rsema <- struct{}{}:
		return true
	}

	// slow path: add ourselves to a list of waiters so the best priority
	// waiter gets the resource when it becomes available.
	s.mu.Lock()
	s.waiters = append(s.waiters, h)
	s.mu.Unlock()

	for {
		select {
		// someone has acquired a resource token and signaled to us.
		case <-h.sig:
			return true

		// if we acquired a resource, then we're responsible for informing
		// the appropriate handler.
		case s.rsema <- struct{}{}:
			// find the most appropriate handler and forward them the token.
			var w *handle
			s.mu.Lock()

			// this condition is pretty subtle, but imagine two people join
			// to get a resource at the same time, and they both remove each
			// other. then the list of waiters is empty. the next time through
			// the loop, they might nondeterministically get this case again
			// and there are no waiters, so who gets the token? so, if there
			// are no waiters, then we must have been removed from the list
			// and so we must be ready to return the token. wait and do that.
			if len(s.waiters) == 0 {
				<-s.rsema
				s.mu.Unlock()

				<-h.sig
				return true
			}

			s.waiters, w = removeBestHandle(s.waiters)
			s.mu.Unlock()

			w.sig <- struct{}{}

		// if the context is done, we're done waiting for a resource.
		case <-ctx.Done():
			// try to remove ourselves from the set of waiters. if we
			// couldn't be found, then someone else is going to try to
			// send us the token, so we need to read it and succeed.
			var removed bool
			s.mu.Lock()
			s.waiters, removed = removeHandle(s.waiters, h)
			s.mu.Unlock()

			if removed {
				return false
			}

			<-h.sig
			return true
		}
	}
}

func (s *Scheduler) numWaiters() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.waiters)
}

// Join acquires a new Handle that can be used to acquire Resources.
func (s *Scheduler) Join(ctx context.Context) (Handle, bool) {
	if ctx.Err() != nil {
		return nil, false
	} else if s.hsema != nil {
		select {
		case <-ctx.Done():
			return nil, false
		case s.hsema <- struct{}{}:
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.prio++

	return &handle{
		prio:  s.prio,
		sched: s,
		sig:   make(chan struct{}, 1),
	}, true
}

// Handle is the interface describing acquired handles from a scheduler.
type Handle interface {
	// Get attempts to acquire a Resource. It will return nil and false if
	// the handle is already Done or if the context is canceled before the
	// resource is acquired. It should not be called concurrently with Done.
	Get(context.Context) (Resource, bool)

	// Done signals that no more resources should be acquired with Get, and
	// it waits for existing resources to be done. It should not be called
	// concurrently with Get.
	Done()
}

type handle struct {
	prio  int
	wg    sync.WaitGroup
	sched *Scheduler
	sig   chan struct{}

	mu   sync.Mutex
	done bool
}

func (h *handle) Done() {
	h.mu.Lock()
	done := h.done
	h.done = true
	h.mu.Unlock()

	h.wg.Wait()

	if !done && h.sched.hsema != nil {
		<-h.sched.hsema
	}
}

func (h *handle) Get(ctx context.Context) (Resource, bool) {
	if h.done {
		return nil, false
	}
	ok := h.sched.resourceGet(ctx, h)
	if !ok {
		return nil, false
	}
	h.wg.Add(1)
	return (*resource)(h), true
}

// Resource is the interface describing acquired resources from a scheduler.
type Resource interface {
	// Done signals that the resource is no longer in use and must be called
	// exactly once.
	Done()
}

type resource handle

func (r *resource) Done() {
	<-(*handle)(r).sched.rsema
	(*handle)(r).wg.Done()
}

func removeBestHandle(hs []*handle) ([]*handle, *handle) {
	if len(hs) == 0 {
		return hs, nil
	}
	bh, bi := hs[0], 0
	for i, h := range hs {
		if h.prio < bh.prio {
			bh, bi = h, i
		}
	}
	return append(hs[:bi], hs[bi+1:]...), bh
}

func removeHandle(hs []*handle, x *handle) ([]*handle, bool) {
	for i, h := range hs {
		if h == x {
			return append(hs[:i], hs[i+1:]...), true
		}
	}
	return hs, false
}
