package server

import (
	"sync"
	"time"
)

// rateLimiter is a rate limiter for the server.
// If more than limit requests are made in the time window, the server will return 429.
type rateLimiter struct {
	// limit is the rate limit to apply to the server.
	limit int

	// window is the window in which to apply the rate limit.
	window time.Duration

	// nextReset is the time at which the rate limit will reset.
	nextReset time.Time

	// count is the number of calls made to the server.
	count int

	// countLock is a mutex for the callCount.
	countLock sync.Mutex

	// statusCode to reply with
	statusCode int
}

func newRateLimiter(limit int, window time.Duration, statusCode int) *rateLimiter {
	return &rateLimiter{
		limit:      limit,
		window:     window,
		statusCode: statusCode,
	}
}

// exceeded checks the rate limit and returns how long to wait before the next request.
func (r *rateLimiter) exceeded() time.Duration {
	r.countLock.Lock()
	defer r.countLock.Unlock()

	if time.Now().After(r.nextReset) {
		r.count = 0
		r.nextReset = time.Now().Add(r.window)
	}

	r.count++

	if r.count > r.limit {
		return time.Until(r.nextReset)
	}

	return 0
}
