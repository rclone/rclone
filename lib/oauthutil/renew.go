package oauthutil

import (
	"sync"
	"sync/atomic"

	"github.com/rclone/rclone/fs"
)

// Renew allows tokens to be renewed on expiry if uploads are in progress.
type Renew struct {
	name     string       // name to use in logs
	ts       *TokenSource // token source that needs renewing
	uploads  atomic.Int32 // number of uploads in progress
	run      func() error // a transaction to run to renew the token on
	done     chan any     // channel to end the go routine
	shutdown sync.Once
}

// NewRenewOnExpiry creates a new Renew struct and starts a background process
// which renews the token whenever it expires.  It uses the run() call
// to run a transaction to do this.
//
// It will only renew the token if the number of uploads > 0
func NewRenewOnExpiry(name string, ts *TokenSource, run func() error) *Renew {
	r := &Renew{
		name: name,
		ts:   ts,
		run:  run,
		done: make(chan any),
	}
	go r.renewOnExpiry()
	return r
}

// NewRenewBeforeExpiry creates a new Renew struct and starts a background process
// which renews the token shortly before it expires.  It uses the run() call
// to run a transaction to do this.
//
// It will only renew the token if the number of uploads > 0
func NewRenewBeforeExpiry(name string, ts *TokenSource, run func() error) *Renew {
	r := &Renew{
		name: name,
		ts:   ts,
		run:  run,
		done: make(chan any),
	}
	go r.renewOnBeforeExpiry()
	return r
}

// renewOnExpiry renews the token whenever it expires.  Useful when there
// are lots of uploads in progress and the token doesn't get renewed.
// Amazon seem to cancel your uploads if you don't renew your token
// for 2hrs.
func (r *Renew) renewOnExpiry() {
	expiry := r.ts.OnExpiry()
	for {
		select {
		case <-expiry:
		case <-r.done:
			return
		}
		r.renewToken()
	}
}

// renewOnBeforeExpiry renews the token shortly before it expires. This
// permits refresh of the token before it expires for packages that
// independently manage token refresh
func (r *Renew) renewOnBeforeExpiry() {
	expiry := r.ts.OnBeforeExpiry()
	for {
		select {
		case <-expiry:
		case <-r.done:
			return
		}
		r.renewToken()
	}
}

// Renew the token by running the provided transaction if any uploads are in progress
func (r *Renew) renewToken() {
	uploads := r.uploads.Load()
	if uploads != 0 {
		fs.Debugf(r.name, "Token expired - %d uploads in progress - refreshing", uploads)
		// Do a transaction
		err := r.run()
		if err == nil {
			fs.Debugf(r.name, "Token refresh successful")
		} else {
			fs.Errorf(r.name, "Token refresh failed: %v", err)
		}
	} else {
		fs.Debugf(r.name, "Token expired but no uploads in progress - doing nothing")
	}
}

// Start should be called before starting an upload
func (r *Renew) Start() {
	r.uploads.Add(1)
}

// Stop should be called after finishing an upload
func (r *Renew) Stop() {
	r.uploads.Add(-1)
}

// Invalidate invalidates the token source
func (r *Renew) Invalidate() {
	r.ts.Invalidate()
}

// Expire expires the token source
func (r *Renew) Expire() error {
	return r.ts.Expire()
}

// Shutdown stops the timer and no more renewal will take place.
func (r *Renew) Shutdown() {
	if r == nil {
		return
	}
	// closing a channel can only be done once
	r.shutdown.Do(func() {
		if r.ts.expiryTimer != nil {
			r.ts.expiryTimer.Stop()
		}
		if r.ts.beforeExpiryTimer != nil {
			r.ts.beforeExpiryTimer.Stop()
		}
		close(r.done)
	})
}
