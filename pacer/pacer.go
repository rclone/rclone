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
	attackConstant     uint          // attack constant
	pacer              chan struct{} // To pace the operations
	sleepTime          time.Duration // Time to sleep for each transaction
	retries            int           // Max number of retries
	maxConnections     int           // Maximum number of concurrent connections
	connTokens         chan struct{} // Connection tokens
	calculatePace      func(bool)    // switchable pacing algorithm - call with mu held
	consecutiveRetries int           // number of consecutive retries
}

// Type is for selecting different pacing algorithms
type Type int

const (
	// DefaultPacer is a truncated exponential attack and decay.
	//
	// On retries the sleep time is doubled, on non errors then
	// sleeptime decays according to the decay constant as set
	// with SetDecayConstant.
	//
	// The sleep never goes below that set with SetMinSleep or
	// above that set with SetMaxSleep.
	DefaultPacer = Type(iota)

	// AmazonCloudDrivePacer is a specialised pacer for Amazon Drive
	//
	// It implements a truncated exponential backoff strategy with
	// randomization.  Normally operations are paced at the
	// interval set with SetMinSleep.  On errors the sleep timer
	// is set to 0..2**retries seconds.
	//
	// See https://developer.amazon.com/public/apis/experience/cloud-drive/content/restful-api-best-practices
	AmazonCloudDrivePacer

	// GoogleDrivePacer is a specialised pacer for Google Drive
	//
	// It implements a truncated exponential backoff strategy with
	// randomization.  Normally operations are paced at the
	// interval set with SetMinSleep.  On errors the sleep timer
	// is set to (2 ^ n) + random_number_milliseconds seconds
	//
	// See https://developers.google.com/drive/v2/web/handle-errors#exponential-backoff
	GoogleDrivePacer
)

// Paced is a function which is called by the Call and CallNoRetry
// methods.  It should return a boolean, true if it would like to be
// retried, and an error.  This error may be returned or returned
// wrapped in a RetryError.
type Paced func() (bool, error)

// New returns a Pacer with sensible defaults
func New() *Pacer {
	p := &Pacer{
		minSleep:       10 * time.Millisecond,
		maxSleep:       2 * time.Second,
		decayConstant:  2,
		attackConstant: 1,
		retries:        fs.Config.LowLevelRetries,
		pacer:          make(chan struct{}, 1),
	}
	p.sleepTime = p.minSleep
	p.SetPacer(DefaultPacer)
	p.SetMaxConnections(fs.Config.Checkers + fs.Config.Transfers)

	// Put the first pacing token in
	p.pacer <- struct{}{}

	return p
}

// SetSleep sets the current sleep time
func (p *Pacer) SetSleep(t time.Duration) *Pacer {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sleepTime = t
	return p
}

// GetSleep gets the current sleep time
func (p *Pacer) GetSleep() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sleepTime
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
// bigger for slower decay, exponential. 1 is halve, 0 is go straight to minimum
func (p *Pacer) SetDecayConstant(decay uint) *Pacer {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.decayConstant = decay
	return p
}

// SetAttackConstant sets the attack constant for the pacer
//
// This is the speed the time grows from the minimum after errors have
// occurred.
//
// bigger for slower attack, 1 is double, 0 is go straight to maximum
func (p *Pacer) SetAttackConstant(attack uint) *Pacer {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.attackConstant = attack
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
		p.calculatePace = p.acdPacer
	case GoogleDrivePacer:
		p.calculatePace = p.drivePacer
	default:
		p.calculatePace = p.defaultPacer
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

// exponentialImplementation implements a exponentialImplementation up
// and down pacing algorithm
//
// See the description for DefaultPacer
//
// This should calculate a new sleepTime.  It takes a boolean as to
// whether the operation should be retried or not.
//
// Call with p.mu held
func (p *Pacer) defaultPacer(retry bool) {
	oldSleepTime := p.sleepTime
	if retry {
		if p.attackConstant == 0 {
			p.sleepTime = p.maxSleep
		} else {
			p.sleepTime = (p.sleepTime << p.attackConstant) / ((1 << p.attackConstant) - 1)
		}
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

// acdPacer implements a truncated exponential backoff
// strategy with randomization for Amazon Drive
//
// See the description for AmazonCloudDrivePacer
//
// This should calculate a new sleepTime.  It takes a boolean as to
// whether the operation should be retried or not.
//
// Call with p.mu held
func (p *Pacer) acdPacer(retry bool) {
	consecutiveRetries := p.consecutiveRetries
	if consecutiveRetries == 0 {
		if p.sleepTime != p.minSleep {
			p.sleepTime = p.minSleep
			fs.Debug("pacer", "Resetting sleep to minimum %v on success", p.sleepTime)
		}
	} else {
		if consecutiveRetries > 9 {
			consecutiveRetries = 9
		}
		// consecutiveRetries starts at 1 so
		// maxSleep is 2**(consecutiveRetries-1) seconds
		maxSleep := time.Second << uint(consecutiveRetries-1)
		// actual sleep is random from 0..maxSleep
		p.sleepTime = time.Duration(rand.Int63n(int64(maxSleep)))
		if p.sleepTime < p.minSleep {
			p.sleepTime = p.minSleep
		}
		fs.Debug("pacer", "Rate limited, sleeping for %v (%d consecutive low level retries)", p.sleepTime, p.consecutiveRetries)
	}
}

// drivePacer implements a truncated exponential backoff strategy with
// randomization for Google Drive
//
// See the description for GoogleDrivePacer
//
// This should calculate a new sleepTime.  It takes a boolean as to
// whether the operation should be retried or not.
//
// Call with p.mu held
func (p *Pacer) drivePacer(retry bool) {
	consecutiveRetries := p.consecutiveRetries
	if consecutiveRetries == 0 {
		if p.sleepTime != p.minSleep {
			p.sleepTime = p.minSleep
			fs.Debug("pacer", "Resetting sleep to minimum %v on success", p.sleepTime)
		}
	} else {
		if consecutiveRetries > 5 {
			consecutiveRetries = 5
		}
		// consecutiveRetries starts at 1 so go from 1,2,3,4,5,5 => 1,2,4,8,16,16
		// maxSleep is 2**(consecutiveRetries-1) seconds + random milliseconds
		p.sleepTime = time.Second<<uint(consecutiveRetries-1) + time.Duration(rand.Int63n(int64(time.Second)))
		fs.Debug("pacer", "Rate limited, sleeping for %v (%d consecutive low level retries)", p.sleepTime, p.consecutiveRetries)
	}
}

// endCall implements the pacing algorithm
//
// This should calculate a new sleepTime.  It takes a boolean as to
// whether the operation should be retried or not.
func (p *Pacer) endCall(retry bool) {
	if p.maxConnections > 0 {
		p.connTokens <- struct{}{}
	}
	p.mu.Lock()
	if retry {
		p.consecutiveRetries++
	} else {
		p.consecutiveRetries = 0
	}
	p.calculatePace(retry)
	p.mu.Unlock()
}

// call implements Call but with settable retries
func (p *Pacer) call(fn Paced, retries int) (err error) {
	var retry bool
	for i := 1; i <= retries; i++ {
		p.beginCall()
		retry, err = fn()
		p.endCall(retry)
		if !retry {
			break
		}
		fs.Debug("pacer", "low level retry %d/%d (error %v)", i, retries, err)
	}
	if retry {
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
