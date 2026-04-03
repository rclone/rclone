package seafile

import (
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
)

// Renew allows tokens to be renewed on expiry.
type Renew struct {
	ts       *time.Ticker // timer indicating when it's time to renew the token
	run      func() error // the callback to do the renewal
	done     chan any     // channel to end the go routine
	shutdown *sync.Once
}

// NewRenew creates a new Renew struct and starts a background process
// which renews the token whenever it expires.  It uses the run() call
// to do the renewal.
func NewRenew(every time.Duration, run func() error) *Renew {
	r := &Renew{
		ts:       time.NewTicker(every),
		run:      run,
		done:     make(chan any),
		shutdown: &sync.Once{},
	}
	go r.renewOnExpiry()
	return r
}

func (r *Renew) renewOnExpiry() {
	for {
		select {
		case <-r.ts.C:
			err := r.run()
			if err != nil {
				fs.Errorf(nil, "error while refreshing decryption token: %s", err)
			}

		case <-r.done:
			return
		}
	}
}

// Shutdown stops the ticker and no more renewal will take place.
func (r *Renew) Shutdown() {
	// closing a channel can only be done once
	r.shutdown.Do(func() {
		r.ts.Stop()
		close(r.done)
	})
}
