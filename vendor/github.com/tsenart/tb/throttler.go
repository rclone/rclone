package tb

import (
	"math"
	"sync"
	"time"
)

// Throttler is a thread-safe wrapper around a map of buckets and an easy to
// use API for generic throttling.
type Throttler struct {
	mu      sync.RWMutex
	freq    time.Duration
	buckets map[string]*Bucket
	closing chan struct{}
}

// NewThrottler returns a Throttler with a single filler go-routine for all
// its Buckets which ticks every freq.
// The number of tokens added on each tick for each bucket is computed
// dynamically to be even accross the duration of a second.
//
// If freq <= 0, the filling go-routine won't be started.
func NewThrottler(freq time.Duration) *Throttler {
	th := &Throttler{
		freq:    freq,
		buckets: map[string]*Bucket{},
		closing: make(chan struct{}),
	}

	if freq > 0 {
		go th.fill(freq)
	}

	return th
}

// Bucket returns a Bucket with rate capacity, keyed by key.
//
// If a Bucket (key, rate) doesn't exist yet, it is created.
//
// You must call Close when you're done with the Throttler in order to not leak
// a go-routine and a system-timer.
func (t *Throttler) Bucket(key string, rate int64) *Bucket {
	t.mu.Lock()
	defer t.mu.Unlock()

	b, ok := t.buckets[key]

	if !ok {
		b = NewBucket(rate, -1)
		b.inc = int64(math.Floor(.5 + (float64(b.capacity) * t.freq.Seconds())))
		b.freq = t.freq
		t.buckets[key] = b
	}

	return b
}

// Wait waits for n amount of tokens to be available.
// If n tokens are immediatelly available it doesn't sleep. Otherwise, it sleeps
// the minimum amount of time required for the remaining tokens to be available.
// It returns the wait duration.
//
// If a Bucket (key, rate) doesn't exist yet, it is created.
// If freq < 1/rate seconds, the effective wait rate won't be correct.
//
// You must call Close when you're done with the Throttler in order to not leak
// a go-routine and a system-timer.
func (t *Throttler) Wait(key string, n, rate int64) time.Duration {
	return t.Bucket(key, rate).Wait(n)
}

// Halt returns a bool indicating if the Bucket identified by key and rate has
// n amount of tokens. If it doesn't, the taken tokens are added back to the
// bucket.
//
// If a Bucket (key, rate) doesn't exist yet, it is created.
// If freq < 1/rate seconds, the results won't be correct.
//
// You must call Close when you're done with the Throttler in order to not leak
// a go-routine and a system-timer.
func (t *Throttler) Halt(key string, n, rate int64) bool {
	b := t.Bucket(key, rate)

	if got := b.Take(n); got != n {
		b.Put(got)
		return true
	}

	return false
}

// Close stops filling the Buckets, closing the filling go-routine.
func (t *Throttler) Close() error {
	close(t.closing)

	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, b := range t.buckets {
		b.Close()
	}

	return nil
}

func (t *Throttler) fill(freq time.Duration) {
	ticker := time.NewTicker(freq)
	defer ticker.Stop()

	for _ = range ticker.C {
		select {
		case <-t.closing:
			return
		default:
		}
		t.mu.RLock()
		for _, b := range t.buckets {
			b.Put(b.inc)
		}
		t.mu.RUnlock()
	}
}
