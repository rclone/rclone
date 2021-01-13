package oauthutil

import (
	"sync/atomic"

	"github.com/rclone/rclone/fs"
)

// Renew allows tokens to be renewed on expiry if uploads are in progress.
type Renew struct {
	name    string       // name to use in logs
	ts      *TokenSource // token source that needs renewing
	uploads int32        // number of uploads in progress - atomic access required
	run     func() error // a transaction to run to renew the token on
}

// NewRenew creates a new Renew struct and starts a background process
// which renews the token whenever it expires.  It uses the run() call
// to run a transaction to do this.
//
// It will only renew the token if the number of uploads > 0
func NewRenew(name string, ts *TokenSource, run func() error) *Renew {
	r := &Renew{
		name: name,
		ts:   ts,
		run:  run,
	}
	go r.renewOnExpiry()
	return r
}

// renewOnExpiry renews the token whenever it expires.  Useful when there
// are lots of uploads in progress and the token doesn't get renewed.
// Amazon seem to cancel your uploads if you don't renew your token
// for 2hrs.
func (r *Renew) renewOnExpiry() {
	expiry := r.ts.OnExpiry()
	for {
		<-expiry
		uploads := atomic.LoadInt32(&r.uploads)
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
}

// Start should be called before starting an upload
func (r *Renew) Start() {
	atomic.AddInt32(&r.uploads, 1)
}

// Stop should be called after finishing an upload
func (r *Renew) Stop() {
	atomic.AddInt32(&r.uploads, -1)
}

// Invalidate invalidates the token source
func (r *Renew) Invalidate() {
	r.ts.Invalidate()
}
