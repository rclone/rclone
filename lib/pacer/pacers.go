package pacer

import (
	"math/rand"
	"time"

	"golang.org/x/time/rate"
)

type (
	// MinSleep configures the minimum sleep time of a Calculator
	MinSleep time.Duration
	// MaxSleep configures the maximum sleep time of a Calculator
	MaxSleep time.Duration
	// DecayConstant configures the decay constant time of a Calculator
	DecayConstant uint
	// AttackConstant configures the attack constant of a Calculator
	AttackConstant uint
	// Burst configures the number of API calls to allow without sleeping
	Burst int
)

// Default is a truncated exponential attack and decay.
//
// On retries the sleep time is doubled, on non errors then sleeptime decays
// according to the decay constant as set with SetDecayConstant.
//
// The sleep never goes below that set with SetMinSleep or above that set
// with SetMaxSleep.
type Default struct {
	minSleep       time.Duration // minimum sleep time
	maxSleep       time.Duration // maximum sleep time
	decayConstant  uint          // decay constant
	attackConstant uint          // attack constant
}

// DefaultOption is the interface implemented by all options for the Default Calculator
type DefaultOption interface {
	ApplyDefault(*Default)
}

// NewDefault creates a Calculator used by Pacer as the default.
func NewDefault(opts ...DefaultOption) *Default {
	c := &Default{
		minSleep:       10 * time.Millisecond,
		maxSleep:       2 * time.Second,
		decayConstant:  2,
		attackConstant: 1,
	}
	c.Update(opts...)
	return c
}

// Update applies the Calculator options.
func (c *Default) Update(opts ...DefaultOption) {
	for _, opt := range opts {
		opt.ApplyDefault(c)
	}
}

// ApplyDefault updates the value on the Calculator
func (o MinSleep) ApplyDefault(c *Default) {
	c.minSleep = time.Duration(o)
}

// ApplyDefault updates the value on the Calculator
func (o MaxSleep) ApplyDefault(c *Default) {
	c.maxSleep = time.Duration(o)
}

// ApplyDefault updates the value on the Calculator
func (o DecayConstant) ApplyDefault(c *Default) {
	c.decayConstant = uint(o)
}

// ApplyDefault updates the value on the Calculator
func (o AttackConstant) ApplyDefault(c *Default) {
	c.attackConstant = uint(o)
}

// Calculate takes the current Pacer state and return the wait time until the next try.
func (c *Default) Calculate(state State) time.Duration {
	if t, ok := IsRetryAfter(state.LastError); ok {
		if t < c.minSleep {
			return c.minSleep
		}
		return t
	}

	if state.ConsecutiveRetries > 0 {
		sleepTime := c.maxSleep
		if c.attackConstant != 0 {
			sleepTime = (state.SleepTime << c.attackConstant) / ((1 << c.attackConstant) - 1)
		}
		if sleepTime > c.maxSleep {
			sleepTime = c.maxSleep
		}
		return sleepTime
	}
	sleepTime := (state.SleepTime<<c.decayConstant - state.SleepTime) >> c.decayConstant
	if sleepTime < c.minSleep {
		sleepTime = c.minSleep
	}
	return sleepTime
}

// ZeroDelayCalculator is a Calculator that never delays.
type ZeroDelayCalculator struct {
}

// Calculate takes the current Pacer state and return the wait time until the next try.
func (c *ZeroDelayCalculator) Calculate(state State) time.Duration {
	return 0
}

// AzureIMDS is a pacer for the Azure instance metadata service.
type AzureIMDS struct {
}

// NewAzureIMDS returns a new Azure IMDS calculator.
func NewAzureIMDS() *AzureIMDS {
	c := &AzureIMDS{}
	return c
}

// Calculate takes the current Pacer state and return the wait time until the next try.
func (c *AzureIMDS) Calculate(state State) time.Duration {
	var addBackoff time.Duration

	if state.ConsecutiveRetries == 0 {
		// Initial condition: no backoff.
		return 0
	}

	if state.ConsecutiveRetries > 4 {
		// The number of consecutive retries shouldn't exceed five.
		// In case it does for some reason, cap delay.
		addBackoff = 0
	} else {
		addBackoff = time.Duration(2<<uint(state.ConsecutiveRetries-1)) * time.Second
	}
	return addBackoff + state.SleepTime
}

// GoogleDrive is a specialized pacer for Google Drive
//
// It implements a truncated exponential backoff strategy with randomization.
// Normally operations are paced at the interval set with SetMinSleep. On errors
// the sleep timer is set to (2 ^ n) + random_number_milliseconds seconds.
//
// See https://developers.google.com/drive/v2/web/handle-errors#exponential-backoff
type GoogleDrive struct {
	minSleep time.Duration // minimum sleep time
	burst    int           // number of requests without sleeping
	limiter  *rate.Limiter // rate limiter for the minSleep
}

// GoogleDriveOption is the interface implemented by all options for the GoogleDrive Calculator
type GoogleDriveOption interface {
	ApplyGoogleDrive(*GoogleDrive)
}

// NewGoogleDrive returns a new GoogleDrive Calculator with default values
func NewGoogleDrive(opts ...GoogleDriveOption) *GoogleDrive {
	c := &GoogleDrive{
		minSleep: 10 * time.Millisecond,
		burst:    100,
	}
	c.Update(opts...)
	return c
}

// Update applies the Calculator options.
func (c *GoogleDrive) Update(opts ...GoogleDriveOption) {
	for _, opt := range opts {
		opt.ApplyGoogleDrive(c)
	}
	if c.burst <= 0 {
		c.burst = 1
	}
	c.limiter = rate.NewLimiter(rate.Every(c.minSleep), c.burst)
}

// ApplyGoogleDrive updates the value on the Calculator
func (o MinSleep) ApplyGoogleDrive(c *GoogleDrive) {
	c.minSleep = time.Duration(o)
}

// ApplyGoogleDrive updates the value on the Calculator
func (o Burst) ApplyGoogleDrive(c *GoogleDrive) {
	c.burst = int(o)
}

// Calculate takes the current Pacer state and return the wait time until the next try.
func (c *GoogleDrive) Calculate(state State) time.Duration {
	if t, ok := IsRetryAfter(state.LastError); ok {
		if t < c.minSleep {
			return c.minSleep
		}
		return t
	}

	consecutiveRetries := state.ConsecutiveRetries
	if consecutiveRetries == 0 {
		return c.limiter.Reserve().Delay()
	}
	if consecutiveRetries > 5 {
		consecutiveRetries = 5
	}
	// consecutiveRetries starts at 1 so go from 1,2,3,4,5,5 => 1,2,4,8,16,16
	// maxSleep is 2**(consecutiveRetries-1) seconds + random milliseconds
	return time.Second<<uint(consecutiveRetries-1) + time.Duration(rand.Int63n(int64(time.Second)))
}

// S3 implements a pacer compatible with our expectations of S3, where it tries to not
// delay at all between successful calls, but backs off in the default fashion in response
// to any errors.
// The assumption is that errors should be exceedingly rare (S3 seems to have largely solved
// the sort of stability questions rclone is likely to run into), and in the happy case
// it can handle calls with no delays between them.
//
// Basically defaultPacer, but with some handling of sleepTime going to/from 0ms
type S3 struct {
	minSleep       time.Duration // minimum sleep time
	maxSleep       time.Duration // maximum sleep time
	decayConstant  uint          // decay constant
	attackConstant uint          // attack constant
}

// S3Option is the interface implemented by all options for the S3 Calculator
type S3Option interface {
	ApplyS3(*S3)
}

// NewS3 returns a new S3 Calculator with default values
func NewS3(opts ...S3Option) *S3 {
	c := &S3{
		maxSleep:       2 * time.Second,
		decayConstant:  2,
		attackConstant: 1,
	}
	c.Update(opts...)
	return c
}

// Update applies the Calculator options.
func (c *S3) Update(opts ...S3Option) {
	for _, opt := range opts {
		opt.ApplyS3(c)
	}
}

// ApplyS3 updates the value on the Calculator
func (o MaxSleep) ApplyS3(c *S3) {
	c.maxSleep = time.Duration(o)
}

// ApplyS3 updates the value on the Calculator
func (o MinSleep) ApplyS3(c *S3) {
	c.minSleep = time.Duration(o)
}

// ApplyS3 updates the value on the Calculator
func (o DecayConstant) ApplyS3(c *S3) {
	c.decayConstant = uint(o)
}

// ApplyS3 updates the value on the Calculator
func (o AttackConstant) ApplyS3(c *S3) {
	c.attackConstant = uint(o)
}

// Calculate takes the current Pacer state and return the wait time until the next try.
func (c *S3) Calculate(state State) time.Duration {
	if t, ok := IsRetryAfter(state.LastError); ok {
		if t < c.minSleep {
			return c.minSleep
		}
		return t
	}

	if state.ConsecutiveRetries > 0 {
		if c.attackConstant == 0 {
			return c.maxSleep
		}
		if state.SleepTime == 0 {
			return c.minSleep
		}
		sleepTime := (state.SleepTime << c.attackConstant) / ((1 << c.attackConstant) - 1)
		if sleepTime > c.maxSleep {
			sleepTime = c.maxSleep
		}
		return sleepTime
	}
	sleepTime := (state.SleepTime<<c.decayConstant - state.SleepTime) >> c.decayConstant
	if sleepTime < c.minSleep {
		sleepTime = 0
	}
	return sleepTime
}
