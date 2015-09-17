// Package pacer makes pacing and retrying API calls easy
package pacer

import (
	"math/rand"
	"sync"
	"time"

	"github.com/ncw/rclone/fs"
)

// Pacer state
type Pacer struct {
	mu                 sync.Mutex    // Protecting read/writes
	minSleep           time.Duration // minimum sleep time
	maxSleep           time.Duration // maximum sleep time
	decayConstant      uint          // decay constant
	pacer              chan struct{} // To pace the operations
	sleepTime          time.Duration // Time to sleep for each transaction
	retries            int           // Max number of retries
	maxConnections     int           // Maximum number of concurrent connections
	connTokens         chan struct{} // Connection tokens
	_calculatePace     func(bool)    // switchable pacing algorithm
	consecutiveRetries int           // number of consecutive retries
}

// Type is for selecting different pacing algorithms
type Type int

const (
	// DefaultPacer is a truncated exponential attack and decay
	DefaultPacer = Type(iota)
	// AmazonCloudDrivePacer is a specialised pacer for Amazon Cloud Drive
	AmazonCloudDrivePacer
)

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
	p.SetPacer(DefaultPacer)
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

// SetPacer sets the pacing algorithm
//
// It will choose the default algorithm if an incorrect value is
// passed in.
func (p *Pacer) SetPacer(t Type) *Pacer {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch t {
	case AmazonCloudDrivePacer:
		p._calculatePace = p._acdPacer
	default:
		p._calculatePace = p._defaultPacer
	}
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

// exponentialImplementation implements a exponentialImplementation up and down pacing algorithm
//
// This should calculate a new sleepTime.  It takes a boolean as to
// whether the operation should be retried or not.
//
// Called with the lock held
func (p *Pacer) _defaultPacer(again bool) {
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
}

// _acdPacer implements a truncated exponential backoff
// strategy with randomization for Amazon Cloud Drive
//
// See https://developer.amazon.com/public/apis/experience/cloud-drive/content/restful-api-best-practices
//
// This should calculate a new sleepTime.  It takes a boolean as to
// whether the operation should be retried or not.
//
// Called with the lock held
func (p *Pacer) _acdPacer(again bool) {
	consecutiveRetries := p.consecutiveRetries
	if consecutiveRetries == 0 {
		if p.sleepTime != p.minSleep {
			p.sleepTime = p.minSleep
			fs.Debug("pacer", "Resetting sleep to minimum %v on success", p.sleepTime)
		}
	} else {
		if consecutiveRetries > 8 {
			consecutiveRetries = 8
		}
		// consecutiveRetries starts at 1 so
		// maxSleep is 2**(consecutiveRetries-1) seconds
		maxSleep := time.Second << uint(consecutiveRetries-1)
		// actual sleep is random from 0..maxSleep
		p.sleepTime = time.Duration(rand.Int63n(int64(maxSleep)))
		fs.Debug("pacer", "Rate limited, sleeping for %v (%d retries)", p.sleepTime, consecutiveRetries)
	}
}

// endCall implements the pacing algorithm
//
// This should calculate a new sleepTime.  It takes a boolean as to
// whether the operation should be retried or not.
func (p *Pacer) endCall(again bool) {
	if p.maxConnections > 0 {
		p.connTokens <- struct{}{}
	}
	p.mu.Lock()
	if again {
		p.consecutiveRetries++
	} else {
		p.consecutiveRetries = 0
	}
	p._calculatePace(again)
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

// CallNoRetry paces the remote operations to not exceed the limits
// and return a retry error on rate limit exceeded
//
// This calls fn and wraps the output in a RetryError if it would like
// it to be retried
func (p *Pacer) CallNoRetry(fn Paced) error {
	return p.call(fn, 1)
}
