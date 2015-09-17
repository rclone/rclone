// pacer is a utility package to make pacing and retrying API calls easy
package pacer

import (
	"sync"
	"time"

	"github.com/ncw/rclone/fs"
)

type Pacer struct {
	minSleep       time.Duration // minimum sleep time
	maxSleep       time.Duration // maximum sleep time
	decayConstant  uint          // decay constant
	pacer          chan struct{} // To pace the operations
	sleepTime      time.Duration // Time to sleep for each transaction
	retries        int           // Max number of retries
	mu             sync.Mutex    // Protecting read/writes
	maxConnections int           // Maximum number of concurrent connections
	connTokens     chan struct{} // Connection tokens
}

// Paced is a function which is called by the Call and CallNoRetry
// methods.  It should return a boolean, true if it would like to be
// retried, and an error.  This error may be returned or returned
// wrapped in a RetryError.
type Paced func() (bool, error)

// New returns a Pacer with sensible defaults
func New() *Pacer {
	p := &Pacer{
		minSleep:      10 * time.Millisecond,
		maxSleep:      2 * time.Second,
		decayConstant: 2,
		retries:       10,
		pacer:         make(chan struct{}, 1),
	}
	p.sleepTime = p.minSleep
	p.SetMaxConnections(fs.Config.Checkers)
	// Put the first pacing token in
	p.pacer <- struct{}{}

	return p
}

// SetMinSleep sets the minimum sleep time for the pacer
func (p *Pacer) SetMinSleep(t time.Duration) *Pacer {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.minSleep = t
	p.sleepTime = p.minSleep
	return p
}

// SetMaxSleep sets the maximum sleep time for the pacer
func (p *Pacer) SetMaxSleep(t time.Duration) *Pacer {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.maxSleep = t
	p.sleepTime = p.minSleep
	return p
}

// SetMaxConnections sets the maximum number of concurrent connections.
// Setting the value to 0 will allow unlimited number of connections.
// Should not be changed once you have started calling the pacer.
// By default this will be set to fs.Config.Checkers.
func (p *Pacer) SetMaxConnections(n int) *Pacer {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.maxConnections = n
	if n <= 0 {
		p.connTokens = nil
	} else {
		p.connTokens = make(chan struct{}, n)
		for i := 0; i < n; i++ {
			p.connTokens <- struct{}{}
		}
	}
	return p
}

// SetDecayConstant sets the decay constant for the pacer
//
// This is the speed the time falls back to the minimum after errors
// have occurred.
//
// bigger for slower decay, exponential
func (p *Pacer) SetDecayConstant(decay uint) *Pacer {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.decayConstant = decay
	return p
}

// SetRetries sets the max number of tries for Call
func (p *Pacer) SetRetries(retries int) *Pacer {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.retries = retries
	return p
}

// Start a call to the API
//
// This must be called as a pair with endCall
//
// This waits for the pacer token
func (p *Pacer) beginCall() {
	// pacer starts with a token in and whenever we take one out
	// XXX ms later we put another in.  We could do this with a
	// Ticker more accurately, but then we'd have to work out how
	// not to run it when it wasn't needed
	<-p.pacer
	if p.maxConnections > 0 {
		<-p.connTokens
	}

	p.mu.Lock()
	// Restart the timer
	go func(t time.Duration) {
		// fs.Debug(f, "New sleep for %v at %v", t, time.Now())
		time.Sleep(t)
		p.pacer <- struct{}{}
	}(p.sleepTime)
	p.mu.Unlock()
}

// End a call to the API
//
// Refresh the pace given an error that was returned.  It returns a
// boolean as to whether the operation should be retried.
func (p *Pacer) endCall(again bool) {
	if p.maxConnections > 0 {
		p.connTokens <- struct{}{}
	}
	p.mu.Lock()
	oldSleepTime := p.sleepTime
	if again {
		p.sleepTime *= 2
		if p.sleepTime > p.maxSleep {
			p.sleepTime = p.maxSleep
		}
		if p.sleepTime != oldSleepTime {
			fs.Debug("pacer", "Rate limited, increasing sleep to %v", p.sleepTime)
		}
	} else {
		p.sleepTime = (p.sleepTime<<p.decayConstant - p.sleepTime) >> p.decayConstant
		if p.sleepTime < p.minSleep {
			p.sleepTime = p.minSleep
		}
		if p.sleepTime != oldSleepTime {
			fs.Debug("pacer", "Reducing sleep to %v", p.sleepTime)
		}
	}
	p.mu.Unlock()
}

// call implements Call but with settable retries
func (p *Pacer) call(fn Paced, retries int) (err error) {
	var again bool
	for i := 0; i < retries; i++ {
		p.beginCall()
		again, err = fn()
		p.endCall(again)
		if !again {
			break
		}
	}
	if again {
		err = fs.RetryError(err)
	}
	return err
}

// Call paces the remote operations to not exceed the limits and retry
// on rate limit exceeded
//
// This calls fn, expecting it to return a retry flag and an
// error. This error may be returned wrapped in a RetryError if the
// number of retries is exceeded.
func (p *Pacer) Call(fn Paced) (err error) {
	p.mu.Lock()
	retries := p.retries
	p.mu.Unlock()
	return p.call(fn, retries)
}

// Pace the remote operations to not exceed Amazon's limits and return
// a retry error on rate limit exceeded
//
// This calls fn and wraps the output in a RetryError if it would like
// it to be retried
func (p *Pacer) CallNoRetry(fn Paced) error {
	return p.call(fn, 1)
}
