// Package pacer makes pacing and retrying API calls easy
package pacer

import (
	"sync"
	"time"

	"github.com/rclone/rclone/lib/errors"
)

// State represents the public Pacer state that will be passed to the
// configured Calculator
type State struct {
	SleepTime          time.Duration // current time to sleep before adding the pacer token back
	ConsecutiveRetries int           // number of consecutive retries, will be 0 when the last invoker call returned false
	LastError          error         // the error returned by the last invoker call or nil
}

// Calculator is a generic calculation function for a Pacer.
type Calculator interface {
	// Calculate takes the current Pacer state and returns the sleep time after which
	// the next Pacer call will be done.
	Calculate(state State) time.Duration
}

// Pacer is the primary type of the pacer package. It allows to retry calls
// with a configurable delay in between.
type Pacer struct {
	pacerOptions
	mu         sync.Mutex    // Protecting read/writes
	pacer      chan struct{} // To pace the operations
	connTokens chan struct{} // Connection tokens
	state      State
}
type pacerOptions struct {
	maxConnections int         // Maximum number of concurrent connections
	retries        int         // Max number of retries
	calculator     Calculator  // switchable pacing algorithm - call with mu held
	invoker        InvokerFunc // wrapper function used to invoke the target function
}

// InvokerFunc is the signature of the wrapper function used to invoke the
// target function in Pacer.
type InvokerFunc func(try, tries int, f Paced) (bool, error)

// Option can be used in New to configure the Pacer.
type Option func(*pacerOptions)

// CalculatorOption sets a Calculator for the new Pacer.
func CalculatorOption(c Calculator) Option {
	return func(p *pacerOptions) { p.calculator = c }
}

// RetriesOption sets the retries number for the new Pacer.
func RetriesOption(retries int) Option {
	return func(p *pacerOptions) { p.retries = retries }
}

// MaxConnectionsOption sets the maximum connections number for the new Pacer.
func MaxConnectionsOption(maxConnections int) Option {
	return func(p *pacerOptions) { p.maxConnections = maxConnections }
}

// InvokerOption sets a InvokerFunc for the new Pacer.
func InvokerOption(invoker InvokerFunc) Option {
	return func(p *pacerOptions) { p.invoker = invoker }
}

// Paced is a function which is called by the Call and CallNoRetry
// methods.  It should return a boolean, true if it would like to be
// retried, and an error.  This error may be returned or returned
// wrapped in a RetryError.
type Paced func() (bool, error)

// New returns a Pacer with sensible defaults.
func New(options ...Option) *Pacer {
	opts := pacerOptions{
		maxConnections: 10,
		retries:        3,
	}
	for _, o := range options {
		o(&opts)
	}
	p := &Pacer{
		pacerOptions: opts,
		pacer:        make(chan struct{}, 1),
	}
	if p.calculator == nil {
		p.SetCalculator(nil)
	}
	p.state.SleepTime = p.calculator.Calculate(p.state)
	if p.invoker == nil {
		p.invoker = invoke
	}
	p.SetMaxConnections(p.maxConnections)

	// Put the first pacing token in
	p.pacer <- struct{}{}

	return p
}

// SetMaxConnections sets the maximum number of concurrent connections.
// Setting the value to 0 will allow unlimited number of connections.
// Should not be changed once you have started calling the pacer.
// By default this will be set to fs.Config.Checkers.
func (p *Pacer) SetMaxConnections(n int) {
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
}

// SetRetries sets the max number of retries for Call
func (p *Pacer) SetRetries(retries int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.retries = retries
}

// SetCalculator sets the pacing algorithm. Don't modify the Calculator object
// afterwards, use the ModifyCalculator method when needed.
//
// It will choose the default algorithm if nil is passed in.
func (p *Pacer) SetCalculator(c Calculator) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if c == nil {
		c = NewDefault()
	}
	p.calculator = c
}

// ModifyCalculator calls the given function with the currently configured
// Calculator and the Pacer lock held.
func (p *Pacer) ModifyCalculator(f func(Calculator)) {
	p.mu.Lock()
	f(p.calculator)
	p.mu.Unlock()
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
		time.Sleep(t)
		p.pacer <- struct{}{}
	}(p.state.SleepTime)
	p.mu.Unlock()
}

// endCall implements the pacing algorithm
//
// This should calculate a new sleepTime.  It takes a boolean as to
// whether the operation should be retried or not.
func (p *Pacer) endCall(retry bool, err error) {
	if p.maxConnections > 0 {
		p.connTokens <- struct{}{}
	}
	p.mu.Lock()
	if retry {
		p.state.ConsecutiveRetries++
	} else {
		p.state.ConsecutiveRetries = 0
	}
	p.state.LastError = err
	p.state.SleepTime = p.calculator.Calculate(p.state)
	p.mu.Unlock()
}

// call implements Call but with settable retries
func (p *Pacer) call(fn Paced, retries int) (err error) {
	var retry bool
	for i := 1; i <= retries; i++ {
		p.beginCall()
		retry, err = p.invoker(i, retries, fn)
		p.endCall(retry, err)
		if !retry {
			break
		}
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

func invoke(try, tries int, f Paced) (bool, error) {
	return f()
}

type retryAfterError struct {
	error
	retryAfter time.Duration
}

func (r *retryAfterError) Error() string {
	return r.error.Error()
}

func (r *retryAfterError) Cause() error {
	return r.error
}

// RetryAfterError returns a wrapped error that can be used by Calculator implementations
func RetryAfterError(err error, retryAfter time.Duration) error {
	return &retryAfterError{
		error:      err,
		retryAfter: retryAfter,
	}
}

// IsRetryAfter returns true if the the error or any of it's Cause's is an error
// returned by RetryAfterError. It also returns the associated Duration if possible.
func IsRetryAfter(err error) (retryAfter time.Duration, isRetryAfter bool) {
	errors.Walk(err, func(err error) bool {
		if r, ok := err.(*retryAfterError); ok {
			retryAfter, isRetryAfter = r.retryAfter, true
			return true
		}
		return false
	})
	return
}
