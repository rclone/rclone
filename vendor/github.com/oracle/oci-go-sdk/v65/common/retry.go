// Copyright (c) 2016, 2018, 2023, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.

package common

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"runtime"
	"strings"
	"time"
)

const (
	// UnlimitedNumAttemptsValue is the value for indicating unlimited attempts for reaching success
	UnlimitedNumAttemptsValue = uint(0)

	// number of characters contained in the generated retry token
	generatedRetryTokenLength = 32
)

// OCIRetryableRequest represents a request that can be reissued according to the specified policy.
type OCIRetryableRequest interface {
	// Any retryable request must implement the OCIRequest interface
	OCIRequest

	// Each operation should implement this method, if has binary body, return OCIReadSeekCloser and true, otherwise return nil, false
	BinaryRequestBody() (*OCIReadSeekCloser, bool)

	// Each operation specifies default retry behavior. By passing no arguments to this method, the default retry
	// behavior, as determined on a per-operation-basis, will be honored. Variadic retry policy option arguments
	// passed to this method will override the default behavior.
	RetryPolicy() *RetryPolicy
}

// OCIOperationResponse represents the output of an OCIOperation, with additional context of error message
// and operation attempt number.
type OCIOperationResponse struct {
	// Response from OCI Operation
	Response OCIResponse

	// Error from OCI Operation
	Error error

	// Operation Attempt Number (one-based)
	AttemptNumber uint

	// End of eventually consistent effects, or nil if no such effects
	EndOfWindowTime *time.Time

	// Backoff scaling factor (only used for dealing with eventual consistency)
	BackoffScalingFactor float64

	// Time of the initial attempt
	InitialAttemptTime time.Time
}

const (
	defaultMaximumNumberAttempts  = uint(8)
	defaultExponentialBackoffBase = 2.0
	defaultMinSleepBetween        = 0.0
	defaultMaxSleepBetween        = 30.0

	ecMaximumNumberAttempts  = uint(9)
	ecExponentialBackoffBase = 3.52
	ecMinSleepBetween        = 0.0
	ecMaxSleepBetween        = 45.0
)

var (
	defaultRetryStatusCodeMap = map[StatErrCode]bool{
		{409, "IncorrectState"}:  true,
		{429, "TooManyRequests"}: true,

		{501, "MethodNotImplemented"}: false,
	}
)

// IsErrorRetryableByDefault returns true if the error is retryable by OCI default retry policy
func IsErrorRetryableByDefault(err error) bool {
	if err == nil {
		return false
	}

	if IsNetworkError(err) {
		return true
	}

	if err == io.EOF {
		return true
	}

	if err, ok := IsServiceError(err); ok {
		if shouldRetry, ok := defaultRetryStatusCodeMap[StatErrCode{err.GetHTTPStatusCode(), err.GetCode()}]; ok {
			return shouldRetry
		}

		return 500 <= err.GetHTTPStatusCode() && err.GetHTTPStatusCode() < 505
	}

	return false
}

// NewOCIOperationResponse assembles an OCI Operation Response object.
// Note that InitialAttemptTime is not set, nor is EndOfWindowTime, and BackoffScalingFactor is set to 1.0.
// EndOfWindowTime and BackoffScalingFactor are only important for eventual consistency.
// InitialAttemptTime can be useful for time-based (as opposed to count-based) retry policies.
func NewOCIOperationResponse(response OCIResponse, err error, attempt uint) OCIOperationResponse {
	return OCIOperationResponse{
		Response:             response,
		Error:                err,
		AttemptNumber:        attempt,
		BackoffScalingFactor: 1.0,
	}
}

// NewOCIOperationResponseExtended assembles an OCI Operation Response object, with the value for the EndOfWindowTime, BackoffScalingFactor, and InitialAttemptTime set.
// EndOfWindowTime and BackoffScalingFactor are only important for eventual consistency.
// InitialAttemptTime can be useful for time-based (as opposed to count-based) retry policies.
func NewOCIOperationResponseExtended(response OCIResponse, err error, attempt uint, endOfWindowTime *time.Time, backoffScalingFactor float64,
	initialAttemptTime time.Time) OCIOperationResponse {
	return OCIOperationResponse{
		Response:             response,
		Error:                err,
		AttemptNumber:        attempt,
		EndOfWindowTime:      endOfWindowTime,
		BackoffScalingFactor: backoffScalingFactor,
		InitialAttemptTime:   initialAttemptTime,
	}
}

//
// RetryPolicy
//

// RetryPolicy is the class that holds all relevant information for retrying operations.
type RetryPolicy struct {
	// MaximumNumberAttempts is the maximum number of times to retry a request. Zero indicates an unlimited
	// number of attempts.
	MaximumNumberAttempts uint

	// ShouldRetryOperation inspects the http response, error, and operation attempt number, and
	// - returns true if we should retry the operation
	// - returns false otherwise
	ShouldRetryOperation func(OCIOperationResponse) bool

	// GetNextDuration computes the duration to pause between operation retries.
	NextDuration func(OCIOperationResponse) time.Duration

	// minimum sleep between attempts in seconds
	MinSleepBetween float64

	// maximum sleep between attempts in seconds
	MaxSleepBetween float64

	// the base for the exponential backoff
	ExponentialBackoffBase float64

	// DeterminePolicyToUse may modify the policy to handle eventual consistency; the return values are
	// the retry policy to use, the end of the eventually consistent time window, and the backoff scaling factor
	// If eventual consistency is not considered, this function should return the unmodified policy that was
	// provided as input, along with (*time.Time)(nil) (no time window), and 1.0 (unscaled backoff).
	DeterminePolicyToUse func(policy RetryPolicy) (RetryPolicy, *time.Time, float64)

	// if the retry policy considers eventual consistency, but there is no eventual consistency present
	// the retries will fall back to the policy specified here; recommendation is to set this to DefaultRetryPolicyWithoutEventualConsistency()
	NonEventuallyConsistentPolicy *RetryPolicy

	// Stores the maximum cumulative backoff in seconds. This can usually be calculated using
	// MaximumNumberAttempts, MinSleepBetween, MaxSleepBetween, and ExponentialBackoffBase,
	// but if MaximumNumberAttempts is 0 (unlimited attempts), then this needs to be set explicitly
	// for Eventual Consistency retries to work.
	MaximumCumulativeBackoffWithoutJitter float64
}

// GlobalRetry is user defined global level retry policy, it would impact all services, the precedence is lower
// than user defined client/request level retry policy
var GlobalRetry *RetryPolicy = nil

// RetryPolicyOption is the type of the options for NewRetryPolicy.
type RetryPolicyOption func(rp *RetryPolicy)

// Convert retry policy to human-readable string representation
func (rp RetryPolicy) String() string {
	return fmt.Sprintf("{MaximumNumberAttempts=%v, MinSleepBetween=%v, MaxSleepBetween=%v, ExponentialBackoffBase=%v, NonEventuallyConsistentPolicy=%v}",
		rp.MaximumNumberAttempts, rp.MinSleepBetween, rp.MaxSleepBetween, rp.ExponentialBackoffBase, rp.NonEventuallyConsistentPolicy)
}

// Validate returns true if the RetryPolicy is valid; if not, it also returns an error.
func (rp *RetryPolicy) validate() (success bool, err error) {
	var errorStrings []string
	if rp.ShouldRetryOperation == nil {
		errorStrings = append(errorStrings, "ShouldRetryOperation may not be nil")
	}
	if rp.NextDuration == nil {
		errorStrings = append(errorStrings, "NextDuration may not be nil")
	}
	if rp.NonEventuallyConsistentPolicy != nil {
		if rp.MaximumNumberAttempts == 0 && rp.MaximumCumulativeBackoffWithoutJitter <= 0 {
			errorStrings = append(errorStrings, "If eventual consistency is handled, and the MaximumNumberAttempts of the EC retry policy is 0 (unlimited attempts), then the MaximumCumulativeBackoffWithoutJitter of the EC retry policy must be positive; used WithUnlimitedAttempts instead")
		}
		nonEcRp := rp.NonEventuallyConsistentPolicy
		if nonEcRp.MaximumNumberAttempts == 0 && nonEcRp.MaximumCumulativeBackoffWithoutJitter <= 0 {
			errorStrings = append(errorStrings, "If eventual consistency is handled, and the MaximumNumberAttempts of the non-EC retry policy is 0 (unlimited attempts), then the MaximumCumulativeBackoffWithoutJitter of the non-EC retry policy must be positive; used WithUnlimitedAttempts instead")
		}
	}
	if len(errorStrings) > 0 {
		return false, errors.New(strings.Join(errorStrings, ", "))
	}

	// some legacy code constructing RetryPolicy instances directly may not have set DeterminePolicyToUse.
	// In that case, just assume that it doesn't handle eventual consistency.
	if rp.DeterminePolicyToUse == nil {
		rp.DeterminePolicyToUse = returnSamePolicy
	}

	return true, nil
}

// GetMaximumCumulativeBackoffWithoutJitter returns the maximum cumulative backoff the retry policy would do,
// taking into account whether eventually consistency is considered or not.
// This function uses either GetMaximumCumulativeBackoffWithoutJitter or GetMaximumCumulativeEventuallyConsistentBackoffWithoutJitter,
// whichever is appropriate
func (rp RetryPolicy) GetMaximumCumulativeBackoffWithoutJitter() time.Duration {
	if rp.NonEventuallyConsistentPolicy == nil {
		return GetMaximumCumulativeBackoffWithoutJitter(rp)
	}
	return GetMaximumCumulativeEventuallyConsistentBackoffWithoutJitter(rp)
}

//
// Functions to calculate backoff and maximum cumulative backoff
//

// GetBackoffWithoutJitter calculates the backoff without jitter for the attempt, given the retry policy.
func GetBackoffWithoutJitter(policy RetryPolicy, attempt uint) time.Duration {
	return time.Duration(getBackoffWithoutJitterHelper(policy.MinSleepBetween, policy.MaxSleepBetween, policy.ExponentialBackoffBase, attempt)) * time.Second
}

// getBackoffWithoutJitterHelper calculates the backoff without jitter for the attempt, given the loose retry policy values.
func getBackoffWithoutJitterHelper(minSleepBetween float64, maxSleepBetween float64, exponentialBackoffBase float64, attempt uint) float64 {
	sleepTime := math.Pow(exponentialBackoffBase, float64(attempt-1))
	if sleepTime < minSleepBetween {
		sleepTime = minSleepBetween
	}
	if sleepTime > maxSleepBetween {
		sleepTime = maxSleepBetween
	}
	return sleepTime
}

// GetMaximumCumulativeBackoffWithoutJitter calculates the maximum backoff without jitter, according to the retry
// policy, if every retry attempt is made.
func GetMaximumCumulativeBackoffWithoutJitter(policy RetryPolicy) time.Duration {
	return getMaximumCumulativeBackoffWithoutJitterHelper(policy.MinSleepBetween, policy.MaxSleepBetween, policy.ExponentialBackoffBase, policy.MaximumNumberAttempts, policy.MaximumCumulativeBackoffWithoutJitter)
}

func getMaximumCumulativeBackoffWithoutJitterHelper(minSleepBetween float64, maxSleepBetween float64, exponentialBackoffBase float64, MaximumNumberAttempts uint, MaximumCumulativeBackoffWithoutJitter float64) time.Duration {
	var cumulative time.Duration = 0

	if MaximumNumberAttempts == 0 {
		// unlimited
		return time.Duration(MaximumCumulativeBackoffWithoutJitter) * time.Second
	}

	// use a one-based counter because it's easier to think about operation retry in terms of attempt numbering
	for currentOperationAttempt := uint(1); currentOperationAttempt < MaximumNumberAttempts; currentOperationAttempt++ {
		cumulative += time.Duration(getBackoffWithoutJitterHelper(minSleepBetween, maxSleepBetween, exponentialBackoffBase, currentOperationAttempt)) * time.Second
	}
	return cumulative
}

//
// Functions to calculate backoff and maximum cumulative backoff for eventual consistency
//

// GetEventuallyConsistentBackoffWithoutJitter calculates the backoff without jitter for the attempt, given the retry policy
// and dealing with eventually consistent effects. The result is then multiplied by backoffScalingFactor.
func GetEventuallyConsistentBackoffWithoutJitter(policy RetryPolicy, attempt uint, backoffScalingFactor float64) time.Duration {
	return time.Duration(getEventuallyConsistentBackoffWithoutJitterHelper(policy.MinSleepBetween, policy.MaxSleepBetween, policy.ExponentialBackoffBase, attempt, backoffScalingFactor,
		func(minSleepBetween float64, maxSleepBetween float64, exponentialBackoffBase float64, attempt uint) float64 {
			rp := policy.NonEventuallyConsistentPolicy
			return getBackoffWithoutJitterHelper(rp.MinSleepBetween, rp.MaxSleepBetween, rp.ExponentialBackoffBase, attempt)
		})*1000) * time.Millisecond
}

// getEventuallyConsistentBackoffWithoutJitterHelper calculates the backoff without jitter for the attempt, given the loose retry policy values,
// and dealing with eventually consistent effects. The result is then multiplied by backoffScalingFactor.
func getEventuallyConsistentBackoffWithoutJitterHelper(minSleepBetween float64, maxSleepBetween float64, exponentialBackoffBase float64, attempt uint, backoffScalingFactor float64,
	defaultBackoffWithoutJitterHelper func(minSleepBetween float64, maxSleepBetween float64, exponentialBackoffBase float64, attempt uint) float64) float64 {
	var sleepTime = math.Pow(exponentialBackoffBase, float64(attempt-1))
	if sleepTime < minSleepBetween {
		sleepTime = minSleepBetween
	}
	if sleepTime > maxSleepBetween {
		sleepTime = maxSleepBetween
	}
	sleepTime = sleepTime * backoffScalingFactor
	defaultSleepTime := defaultBackoffWithoutJitterHelper(minSleepBetween, maxSleepBetween, exponentialBackoffBase, attempt)
	if defaultSleepTime > sleepTime {
		sleepTime = defaultSleepTime
	}
	return sleepTime
}

// GetMaximumCumulativeEventuallyConsistentBackoffWithoutJitter calculates the maximum backoff without jitter, according to the retry
// policy and taking eventually consistent effects into account, if every retry attempt is made.
func GetMaximumCumulativeEventuallyConsistentBackoffWithoutJitter(policy RetryPolicy) time.Duration {
	return getMaximumCumulativeEventuallyConsistentBackoffWithoutJitterHelper(policy.MinSleepBetween, policy.MaxSleepBetween, policy.ExponentialBackoffBase,
		policy.MaximumNumberAttempts, policy.MaximumCumulativeBackoffWithoutJitter,
		func(minSleepBetween float64, maxSleepBetween float64, exponentialBackoffBase float64, attempt uint) float64 {
			rp := policy.NonEventuallyConsistentPolicy
			return getBackoffWithoutJitterHelper(rp.MinSleepBetween, rp.MaxSleepBetween, rp.ExponentialBackoffBase, attempt)
		})
}

func getMaximumCumulativeEventuallyConsistentBackoffWithoutJitterHelper(minSleepBetween float64, maxSleepBetween float64, exponentialBackoffBase float64, MaximumNumberAttempts uint,
	MaximumCumulativeBackoffWithoutJitter float64,
	defaultBackoffWithoutJitterHelper func(minSleepBetween float64, maxSleepBetween float64, exponentialBackoffBase float64, attempt uint) float64) time.Duration {
	if MaximumNumberAttempts == 0 {
		// unlimited
		return time.Duration(MaximumCumulativeBackoffWithoutJitter) * time.Second
	}

	var cumulative time.Duration = 0
	// use a one-based counter because it's easier to think about operation retry in terms of attempt numbering
	for currentOperationAttempt := uint(1); currentOperationAttempt < MaximumNumberAttempts; currentOperationAttempt++ {
		cumulative += time.Duration(getEventuallyConsistentBackoffWithoutJitterHelper(minSleepBetween, maxSleepBetween, exponentialBackoffBase, currentOperationAttempt, 1.0, defaultBackoffWithoutJitterHelper)*1000) * time.Millisecond
	}
	return cumulative
}

func returnSamePolicy(policy RetryPolicy) (RetryPolicy, *time.Time, float64) {
	// we're returning the end of window time nonetheless, even though the default non-eventual consistency (EC)
	// retry policy doesn't use it; this is useful in case developers wants to write an EC-aware retry policy
	// on their own
	eowt := EcContext.GetEndOfWindow()
	return policy, eowt, 1.0
}

// NoRetryPolicy is a helper method that assembles and returns a return policy that indicates an operation should
// never be retried (the operation is performed exactly once).
func NoRetryPolicy() RetryPolicy {
	dontRetryOperation := func(OCIOperationResponse) bool { return false }
	zeroNextDuration := func(OCIOperationResponse) time.Duration { return 0 * time.Second }
	return newRetryPolicyWithOptionsNoDefault(
		WithMaximumNumberAttempts(1),
		WithShouldRetryOperation(dontRetryOperation),
		WithNextDuration(zeroNextDuration),
		withMinSleepBetween(0.0*time.Second),
		withMaxSleepBetween(0.0*time.Second),
		withExponentialBackoffBase(0.0),
		withDeterminePolicyToUse(returnSamePolicy),
		withNonEventuallyConsistentPolicy(nil))
}

// DefaultShouldRetryOperation is the function that should be used for RetryPolicy.ShouldRetryOperation when
// not taking eventual consistency into account.
func DefaultShouldRetryOperation(r OCIOperationResponse) bool {
	if r.Error == nil && 199 < r.Response.HTTPResponse().StatusCode && r.Response.HTTPResponse().StatusCode < 300 {
		// success
		return false
	}
	return IsErrorRetryableByDefault(r.Error)
}

// DefaultRetryPolicy is a helper method that assembles and returns a return policy that is defined to be a default one
// The default retry policy will retry on (409, IncorrectState), (429, TooManyRequests) and any 5XX errors except (501, MethodNotImplemented)
// The default retry behavior is using exponential backoff with jitter, the maximum wait time is 30s plus 1s jitter
// The maximum cumulative backoff after all 8 attempts have been made is about 1.5 minutes.
// It will also retry on errors affected by eventual consistency.
// The eventual consistency retry behavior is using exponential backoff with jitter, the maximum wait time is 45s plus 1s jitter
// Under eventual consistency, the maximum cumulative backoff after all 9 attempts have been made is about 4 minutes.
func DefaultRetryPolicy() RetryPolicy {
	return NewRetryPolicyWithOptions(
		ReplaceWithValuesFromRetryPolicy(DefaultRetryPolicyWithoutEventualConsistency()),
		WithEventualConsistency())
}

// DefaultRetryPolicyWithoutEventualConsistency is a helper method that assembles and returns a return policy that is defined to be a default one
// The default retry policy will retry on (409, IncorrectState), (429, TooManyRequests) and any 5XX errors except (501, MethodNotImplemented)
// It will not retry on errors affected by eventual consistency.
// The default retry behavior is using exponential backoff with jitter, the maximum wait time is 30s plus 1s jitter
func DefaultRetryPolicyWithoutEventualConsistency() RetryPolicy {
	exponentialBackoffWithJitter := func(r OCIOperationResponse) time.Duration {
		sleepTime := getBackoffWithoutJitterHelper(defaultMinSleepBetween, defaultMaxSleepBetween, defaultExponentialBackoffBase, r.AttemptNumber)
		nextDuration := time.Duration(1000.0*(sleepTime+rand.Float64())) * time.Millisecond
		return nextDuration
	}
	return newRetryPolicyWithOptionsNoDefault(
		WithMaximumNumberAttempts(defaultMaximumNumberAttempts),
		WithShouldRetryOperation(DefaultShouldRetryOperation),
		WithNextDuration(exponentialBackoffWithJitter),
		withMinSleepBetween(defaultMinSleepBetween*time.Second),
		withMaxSleepBetween(defaultMaxSleepBetween*time.Second),
		withExponentialBackoffBase(defaultExponentialBackoffBase),
		withDeterminePolicyToUse(returnSamePolicy),
		withNonEventuallyConsistentPolicy(nil))
}

// EventuallyConsistentShouldRetryOperation is the function that should be used for RetryPolicy.ShouldRetryOperation when
// taking eventual consistency into account
func EventuallyConsistentShouldRetryOperation(r OCIOperationResponse) bool {
	if r.Error == nil && 199 < r.Response.HTTPResponse().StatusCode && r.Response.HTTPResponse().StatusCode < 300 {
		// success
		Debugln(fmt.Sprintf("EC.ShouldRetryOperation, status = %v, 2xx, returning false", r.Response.HTTPResponse().StatusCode))
		return false
	}
	if IsErrorRetryableByDefault(r.Error) {
		return true
	}
	// not retryable by default
	if _, ok := IsServiceError(r.Error); ok {
		now := EcContext.timeNowProvider()
		if r.EndOfWindowTime == nil || r.EndOfWindowTime.Before(now) {
			// either no eventually consistent effects, or they have disappeared by now
			Debugln(fmt.Sprintf("EC.ShouldRetryOperation, no EC or in the past, returning false: endOfWindowTime = %v, now = %v", r.EndOfWindowTime, now))
			return false
		}
		// there were eventually consistent effects present at the time of the first request
		// and they could still affect the retries
		if IsErrorAffectedByEventualConsistency(r.Error) {
			// and it's one of the three affected error codes
			Debugln(fmt.Sprintf("EC.ShouldRetryOperation, affected by EC, EC is present: endOfWindowTime = %v, now = %v", r.EndOfWindowTime, now))
			return true
		}
		return false
	}

	return false
}

// EventuallyConsistentRetryPolicy is a helper method that assembles and returns a return policy that is defined to be a default one
// plus dealing with errors affected by eventual consistency.
// The default retry behavior is using exponential backoff with jitter, the maximum wait time is 45s plus 1s jitter
func EventuallyConsistentRetryPolicy(nonEventuallyConsistentPolicy RetryPolicy) RetryPolicy {
	if nonEventuallyConsistentPolicy.NonEventuallyConsistentPolicy != nil {
		// already deals with eventual consistency
		return nonEventuallyConsistentPolicy
	}
	exponentialBackoffWithJitter := func(r OCIOperationResponse) time.Duration {
		sleepTime := getEventuallyConsistentBackoffWithoutJitterHelper(ecMinSleepBetween, ecMaxSleepBetween, ecExponentialBackoffBase, r.AttemptNumber, r.BackoffScalingFactor,
			func(minSleepBetween float64, maxSleepBetween float64, exponentialBackoffBase float64, attempt uint) float64 {
				rp := nonEventuallyConsistentPolicy
				return getBackoffWithoutJitterHelper(rp.MinSleepBetween, rp.MaxSleepBetween, rp.ExponentialBackoffBase, attempt)
			})
		nextDuration := time.Duration(1000.0*(sleepTime+rand.Float64())) * time.Millisecond
		Debugln(fmt.Sprintf("EventuallyConsistentRetryPolicy.NextDuration for attempt %v: sleepTime = %.1fs, nextDuration = %v", r.AttemptNumber, sleepTime, nextDuration))
		return nextDuration
	}
	returnModifiedPolicy := func(policy RetryPolicy) (RetryPolicy, *time.Time, float64) { return determinePolicyToUse(policy) }
	nonEventuallyConsistentPolicyCopy := newRetryPolicyWithOptionsNoDefault(
		ReplaceWithValuesFromRetryPolicy(nonEventuallyConsistentPolicy))
	return newRetryPolicyWithOptionsNoDefault(
		WithMaximumNumberAttempts(ecMaximumNumberAttempts),
		WithShouldRetryOperation(EventuallyConsistentShouldRetryOperation),
		WithNextDuration(exponentialBackoffWithJitter),
		withMinSleepBetween(ecMinSleepBetween*time.Second),
		withMaxSleepBetween(ecMaxSleepBetween*time.Second),
		withExponentialBackoffBase(ecExponentialBackoffBase),
		withDeterminePolicyToUse(returnModifiedPolicy),
		withNonEventuallyConsistentPolicy(&nonEventuallyConsistentPolicyCopy))
}

// NewRetryPolicy is a helper method for assembling a Retry Policy object. It does not handle eventual consistency, so as to not break existing code.
// If you want to handle eventual consistency, the simplest way to do that is to replace the code
//
//	NewRetryPolicy(a, r, n)
//
// with the code
//
//	  NewRetryPolicyWithOptions(
//			WithMaximumNumberAttempts(a),
//			WithFixedBackoff(fb) // fb is the fixed backoff duration
//			WithShouldRetryOperation(r))
//
// or
//
//	  NewRetryPolicyWithOptions(
//			WithMaximumNumberAttempts(a),
//			WithExponentialBackoff(mb, e) // mb is the maximum backoff duration, and e is the base for exponential backoff, e.g. 2.0
//			WithShouldRetryOperation(r))
//
// or, if a == 0 (the maximum number of attempts is unlimited)
//
//	NewRetryPolicyWithEventualConsistencyUnlimitedAttempts(a, r, n, mcb) // mcb is the maximum cumulative backoff duration without jitter
func NewRetryPolicy(attempts uint, retryOperation func(OCIOperationResponse) bool, nextDuration func(OCIOperationResponse) time.Duration) RetryPolicy {
	return NewRetryPolicyWithOptions(
		ReplaceWithValuesFromRetryPolicy(DefaultRetryPolicyWithoutEventualConsistency()),
		WithMaximumNumberAttempts(attempts),
		WithShouldRetryOperation(retryOperation),
		WithNextDuration(nextDuration),
	)
}

// NewRetryPolicyWithEventualConsistencyUnlimitedAttempts is a helper method for assembling a Retry Policy object.
// It does handle eventual consistency, but other than that, it is very similar to NewRetryPolicy.
// NewRetryPolicyWithEventualConsistency does not support limited attempts, use NewRetryPolicyWithEventualConsistency instead.
func NewRetryPolicyWithEventualConsistencyUnlimitedAttempts(attempts uint, retryOperation func(OCIOperationResponse) bool, nextDuration func(OCIOperationResponse) time.Duration,
	maximumCumulativeBackoffWithoutJitter time.Duration) (*RetryPolicy, error) {

	if attempts != 0 {
		return nil, fmt.Errorf("NewRetryPolicyWithEventualConsistencyUnlimitedAttempts cannot be used with attempts != 0 (limited attempts), use NewRetryPolicyWithEventualConsistency instead")
	}

	result := NewRetryPolicyWithOptions(
		ReplaceWithValuesFromRetryPolicy(DefaultRetryPolicyWithoutEventualConsistency()),
		WithUnlimitedAttempts(maximumCumulativeBackoffWithoutJitter),
		WithShouldRetryOperation(retryOperation),
		WithNextDuration(nextDuration),
	)
	return &result, nil
}

// NewRetryPolicyWithOptions is a helper method for assembling a Retry Policy object.
// It starts out with the values returned by DefaultRetryPolicy() and does handle eventual consistency,
// unless you replace all options set using ReplaceWithValuesFromRetryPolicy(DefaultRetryPolicyWithoutEventualConsistency()).
func NewRetryPolicyWithOptions(opts ...RetryPolicyOption) RetryPolicy {
	rp := &RetryPolicy{}

	// start with the default retry policy
	ReplaceWithValuesFromRetryPolicy(DefaultRetryPolicyWithoutEventualConsistency())(rp)
	WithEventualConsistency()(rp)

	// then allow changing values
	for _, opt := range opts {
		opt(rp)
	}

	if rp.DeterminePolicyToUse == nil {
		rp.DeterminePolicyToUse = returnSamePolicy
	}

	return *rp
}

// newRetryPolicyWithOptionsNoDefault is a helper method for assembling a Retry Policy object.
// Contrary to newRetryPolicyWithOptions, it does not start out with the values returned by
// DefaultRetryPolicy().
func newRetryPolicyWithOptionsNoDefault(opts ...RetryPolicyOption) RetryPolicy {
	rp := &RetryPolicy{}

	// then allow changing values
	for _, opt := range opts {
		opt(rp)
	}

	if rp.DeterminePolicyToUse == nil {
		rp.DeterminePolicyToUse = returnSamePolicy
	}

	return *rp
}

// WithMaximumNumberAttempts is the option for NewRetryPolicyWithOptions that sets the maximum number of attempts.
func WithMaximumNumberAttempts(attempts uint) RetryPolicyOption {
	// this is the RetryPolicyOption function type
	return func(rp *RetryPolicy) {
		rp.MaximumNumberAttempts = attempts
	}
}

// WithUnlimitedAttempts is the option for NewRetryPolicyWithOptions that sets unlimited number of attempts,
// but it needs to set a MaximumCumulativeBackoffWithoutJitter duration.
// If you use WithUnlimitedAttempts, you should set your own NextDuration function using WithNextDuration.
func WithUnlimitedAttempts(maximumCumulativeBackoffWithoutJitter time.Duration) RetryPolicyOption {
	// this is the RetryPolicyOption function type
	return func(rp *RetryPolicy) {
		rp.MaximumNumberAttempts = 0
		rp.MaximumCumulativeBackoffWithoutJitter = float64(maximumCumulativeBackoffWithoutJitter / time.Second)
	}
}

// WithShouldRetryOperation is the option for NewRetryPolicyWithOptions that sets the function that checks
// whether retries should be performed.
func WithShouldRetryOperation(retryOperation func(OCIOperationResponse) bool) RetryPolicyOption {
	// this is the RetryPolicyOption function type
	return func(rp *RetryPolicy) {
		rp.ShouldRetryOperation = retryOperation
	}
}

// WithNextDuration is the option for NewRetryPolicyWithOptions that sets the function for computing the next
// backoff duration.
// It is preferred to use WithFixedBackoff or WithExponentialBackoff instead.
func WithNextDuration(nextDuration func(OCIOperationResponse) time.Duration) RetryPolicyOption {
	// this is the RetryPolicyOption function type
	return func(rp *RetryPolicy) {
		rp.NextDuration = nextDuration
	}
}

// withMinSleepBetween is the option for NewRetryPolicyWithOptions that sets the minimum backoff duration.
func withMinSleepBetween(minSleepBetween time.Duration) RetryPolicyOption {
	// this is the RetryPolicyOption function type
	return func(rp *RetryPolicy) {
		rp.MinSleepBetween = float64(minSleepBetween / time.Second)
	}
}

// withMaxsSleepBetween is the option for NewRetryPolicyWithOptions that sets the maximum backoff duration.
func withMaxSleepBetween(maxSleepBetween time.Duration) RetryPolicyOption {
	// this is the RetryPolicyOption function type
	return func(rp *RetryPolicy) {
		rp.MaxSleepBetween = float64(maxSleepBetween / time.Second)
	}
}

// withExponentialBackoffBase is the option for NewRetryPolicyWithOptions that sets the base for the
// exponential backoff
func withExponentialBackoffBase(base float64) RetryPolicyOption {
	// this is the RetryPolicyOption function type
	return func(rp *RetryPolicy) {
		rp.ExponentialBackoffBase = base
	}
}

// withDeterminePolicyToUse is the option for NewRetryPolicyWithOptions that sets the function that
// determines which polich should be used and if eventual consistency should be considered
func withDeterminePolicyToUse(determinePolicyToUse func(policy RetryPolicy) (RetryPolicy, *time.Time, float64)) RetryPolicyOption {
	// this is the RetryPolicyOption function type
	return func(rp *RetryPolicy) {
		rp.DeterminePolicyToUse = determinePolicyToUse
	}
}

// withNonEventuallyConsistentPolicy is the option for NewRetryPolicyWithOptions that sets the fallback
// strategy if eventual consistency should not be considered
func withNonEventuallyConsistentPolicy(nonEventuallyConsistentPolicy *RetryPolicy) RetryPolicyOption {
	// this is the RetryPolicyOption function type
	return func(rp *RetryPolicy) {
		// we want a non-EC policy for NonEventuallyConsistentPolicy; make sure that NonEventuallyConsistentPolicy is nil
		for nonEventuallyConsistentPolicy != nil && nonEventuallyConsistentPolicy.NonEventuallyConsistentPolicy != nil {
			nonEventuallyConsistentPolicy = nonEventuallyConsistentPolicy.NonEventuallyConsistentPolicy
		}
		rp.NonEventuallyConsistentPolicy = nonEventuallyConsistentPolicy
	}
}

// WithExponentialBackoff is an option for NewRetryPolicyWithOptions that sets the exponential backoff base,
// minimum and maximum sleep between attempts, and next duration function.
// Therefore, WithExponentialBackoff is a combination of WithNextDuration, withMinSleepBetween, withMaxSleepBetween,
// and withExponentialBackoffBase.
func WithExponentialBackoff(newMaxSleepBetween time.Duration, newExponentialBackoffBase float64) RetryPolicyOption {
	exponentialBackoffWithJitter := func(r OCIOperationResponse) time.Duration {
		sleepTime := getBackoffWithoutJitterHelper(defaultMinSleepBetween, newMaxSleepBetween.Seconds(), newExponentialBackoffBase, r.AttemptNumber)
		nextDuration := time.Duration(1000.0*(sleepTime+rand.Float64())) * time.Millisecond
		Debugln(fmt.Sprintf("NextDuration for attempt %v: sleepTime = %.1fs, nextDuration = %v", r.AttemptNumber, sleepTime, nextDuration))
		return nextDuration
	}

	// this is the RetryPolicyOption function type
	return func(rp *RetryPolicy) {
		withMinSleepBetween(0)(rp)
		withMaxSleepBetween(newMaxSleepBetween)(rp)
		withExponentialBackoffBase(newExponentialBackoffBase)(rp)
		WithNextDuration(exponentialBackoffWithJitter)(rp)
	}
}

// WithFixedBackoff is an option for NewRetryPolicyWithOptions that sets the backoff to always be exactly the same value. There is no jitter either.
// Therefore, WithFixedBackoff is a combination of WithNextDuration, withMinSleepBetween, withMaxSleepBetween, and withExponentialBackoffBase.
func WithFixedBackoff(newSleepBetween time.Duration) RetryPolicyOption {
	fixedBackoffWithoutJitter := func(r OCIOperationResponse) time.Duration {
		nextDuration := newSleepBetween
		Debugln(fmt.Sprintf("NextDuration for attempt %v: nextDuration = %v", r.AttemptNumber, nextDuration))
		return nextDuration
	}

	// this is the RetryPolicyOption function type
	return func(rp *RetryPolicy) {
		withMinSleepBetween(newSleepBetween)(rp)
		withMaxSleepBetween(newSleepBetween)(rp)
		withExponentialBackoffBase(1.0)(rp)
		WithNextDuration(fixedBackoffWithoutJitter)(rp)
	}
}

// WithEventualConsistency is the option for NewRetryPolicyWithOptions that enables considering eventual backoff for the policy.
func WithEventualConsistency() RetryPolicyOption {
	// this is the RetryPolicyOption function type
	return func(rp *RetryPolicy) {
		copy := RetryPolicy{
			MaximumNumberAttempts:         rp.MaximumNumberAttempts,
			ShouldRetryOperation:          rp.ShouldRetryOperation,
			NextDuration:                  rp.NextDuration,
			MinSleepBetween:               rp.MinSleepBetween,
			MaxSleepBetween:               rp.MaxSleepBetween,
			ExponentialBackoffBase:        rp.ExponentialBackoffBase,
			DeterminePolicyToUse:          rp.DeterminePolicyToUse,
			NonEventuallyConsistentPolicy: rp.NonEventuallyConsistentPolicy,
		}
		ecrp := EventuallyConsistentRetryPolicy(copy)
		rp.MaximumNumberAttempts = ecrp.MaximumNumberAttempts
		rp.ShouldRetryOperation = ecrp.ShouldRetryOperation
		rp.NextDuration = ecrp.NextDuration
		rp.MinSleepBetween = ecrp.MinSleepBetween
		rp.MaxSleepBetween = ecrp.MaxSleepBetween
		rp.ExponentialBackoffBase = ecrp.ExponentialBackoffBase
		rp.DeterminePolicyToUse = ecrp.DeterminePolicyToUse
		rp.NonEventuallyConsistentPolicy = ecrp.NonEventuallyConsistentPolicy
	}
}

// WithConditionalOption is an option for NewRetryPolicyWithOptions that enables or disables another option.
func WithConditionalOption(enabled bool, otherOption RetryPolicyOption) RetryPolicyOption {
	// this is the RetryPolicyOption function type
	return func(rp *RetryPolicy) {
		if enabled {
			otherOption(rp)
		}
	}
}

// ReplaceWithValuesFromRetryPolicy is an option for NewRetryPolicyWithOptions that copies over all settings from another RetryPolicy
func ReplaceWithValuesFromRetryPolicy(other RetryPolicy) RetryPolicyOption {
	// this is the RetryPolicyOption function type
	return func(rp *RetryPolicy) {
		rp.MaximumNumberAttempts = other.MaximumNumberAttempts
		rp.ShouldRetryOperation = other.ShouldRetryOperation
		rp.NextDuration = other.NextDuration
		rp.MinSleepBetween = other.MinSleepBetween
		rp.MaxSleepBetween = other.MaxSleepBetween
		rp.ExponentialBackoffBase = other.ExponentialBackoffBase
		rp.DeterminePolicyToUse = other.DeterminePolicyToUse
		rp.NonEventuallyConsistentPolicy = other.NonEventuallyConsistentPolicy
		rp.MaximumCumulativeBackoffWithoutJitter = other.MaximumCumulativeBackoffWithoutJitter
	}
}

// shouldContinueIssuingRequests returns true if we should continue retrying a request, based on the current attempt
// number and the maximum number of attempts specified, or false otherwise.
func shouldContinueIssuingRequests(current, maximum uint) bool {
	return maximum == UnlimitedNumAttemptsValue || current <= maximum
}

// RetryToken generates a retry token that must be included on any request passed to the Retry method.
func RetryToken() string {
	alphanumericChars := []rune("abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	retryToken := make([]rune, generatedRetryTokenLength)
	for i := range retryToken {
		retryToken[i] = alphanumericChars[rand.Intn(len(alphanumericChars))]
	}
	return string(retryToken)
}

func determinePolicyToUse(policy RetryPolicy) (RetryPolicy, *time.Time, float64) {
	initialAttemptTime := EcContext.timeNowProvider()
	var useDefaultTimingInstead = true
	var endOfWindowTime = (*time.Time)(nil)
	var backoffScalingFactor = 1.0
	var policyToUse RetryPolicy = policy

	eowt := EcContext.GetEndOfWindow()
	if eowt != nil {
		// there was an eventually consistent request
		if eowt.After(initialAttemptTime) {
			// and the eventually consistent effects may still be present
			endOfWindowTime = eowt
			// if the time between now and the end of the window is less than the time we normally would retry, use the default timing
			durationToEndOfWindow := endOfWindowTime.Sub(initialAttemptTime)
			maxCumulativeBackoffWithoutJitter := GetMaximumCumulativeBackoffWithoutJitter(*policy.NonEventuallyConsistentPolicy)
			Debugln(fmt.Sprintf("durationToEndOfWindow = %v, maxCumulativeBackoffWithoutJitter = %v", durationToEndOfWindow, maxCumulativeBackoffWithoutJitter))
			if durationToEndOfWindow > maxCumulativeBackoffWithoutJitter {
				// the end of the eventually consistent window is later than when default retries would end
				// do not use default timing
				maximumCumulativeBackoffWithoutJitter := GetMaximumCumulativeEventuallyConsistentBackoffWithoutJitter(policy)
				backoffScalingFactor = float64(durationToEndOfWindow) / float64(maximumCumulativeBackoffWithoutJitter)
				useDefaultTimingInstead = false
				Debugln(fmt.Sprintf("Use eventually consistent timing, durationToEndOfWindow = %v, maximumCumulativeBackoffWithoutJitter = %v, backoffScalingFactor = %.2f",
					durationToEndOfWindow, maximumCumulativeBackoffWithoutJitter, backoffScalingFactor))
			} else {
				Debugln(fmt.Sprintf("Use default timing, end of EC window is sooner than default retries"))
			}
		} else {
			useDefaultTimingInstead = false
			policyToUse = *policy.NonEventuallyConsistentPolicy
			Debugln(fmt.Sprintf("Use default timing and strategy, end of EC window is in the past"))
		}
	} else {
		useDefaultTimingInstead = false
		policyToUse = *policy.NonEventuallyConsistentPolicy
		Debugln(fmt.Sprintf("Use default timing and strategy, no EC window set"))
	}

	if useDefaultTimingInstead {
		// use timing from defaultRetryPolicy, but whether to retry from the policy that was passed into this request
		policyToUse = NewRetryPolicyWithOptions(
			ReplaceWithValuesFromRetryPolicy(*policy.NonEventuallyConsistentPolicy),
			WithShouldRetryOperation(policy.ShouldRetryOperation))
	}

	return policyToUse, endOfWindowTime, backoffScalingFactor
}

// Retry is a package-level operation that executes the retryable request using the specified operation and retry policy.
func Retry(ctx context.Context, request OCIRetryableRequest, operation OCIOperation, policy RetryPolicy) (OCIResponse, error) {
	type retrierResult struct {
		response OCIResponse
		err      error
	}

	var response OCIResponse
	var err error
	retrierChannel := make(chan retrierResult, 1)

	validated, validateError := policy.validate()
	if !validated {
		return nil, validateError
	}

	initialAttemptTime := time.Now()

	go func() {
		// Deal with panics more graciously
		defer func() {
			if r := recover(); r != nil {
				stackBuffer := make([]byte, 1024)
				bytesWritten := runtime.Stack(stackBuffer, false)
				stack := string(stackBuffer[:bytesWritten])
				error := fmt.Errorf("panicked while retrying operation. Panic was: %s\nStack: %s", r, stack)
				Debugln(error)
				retrierChannel <- retrierResult{nil, error}
			}
		}()
		// if request body is binary request body and seekable, save the current position
		var curPos int64 = 0
		isSeekable := false
		rsc, isBinaryRequest := request.BinaryRequestBody()
		if rsc != nil && rsc.rc != nil {
			defer rsc.rc.Close()
		}
		if policy.MaximumNumberAttempts != uint(1) {
			if rsc.Seekable() {
				isSeekable = true
				curPos, _ = rsc.Seek(0, io.SeekCurrent)
			}
		}

		// some legacy code constructing RetryPolicy instances directly may not have set DeterminePolicyToUse.
		// In that case, just assume that it doesn't handle eventual consistency.
		if policy.DeterminePolicyToUse == nil {
			policy.DeterminePolicyToUse = returnSamePolicy
		}

		// this determines which policy to use, when the eventual consistency window ends, and what the backoff
		// scaling factor should be
		policyToUse, endOfWindowTime, backoffScalingFactor := policy.DeterminePolicyToUse(policy)
		Debugln(fmt.Sprintf("Retry policy to use: %v", policyToUse))
		retryStartTime := time.Now()
		extraHeaders := make(map[string]string)

		if policy.MaximumNumberAttempts == 1 {
			extraHeaders[requestHeaderOpcClientRetries] = "false"
		} else {
			extraHeaders[requestHeaderOpcClientRetries] = "true"
		}

		// use a one-based counter because it's easier to think about operation retry in terms of attempt numbering
		for currentOperationAttempt := uint(1); shouldContinueIssuingRequests(currentOperationAttempt, policyToUse.MaximumNumberAttempts); currentOperationAttempt++ {
			Debugln(fmt.Sprintf("operation attempt #%v", currentOperationAttempt))
			// rewind body once needed
			if isSeekable {
				rsc = NewOCIReadSeekCloser(rsc.rc)
				rsc.Seek(curPos, io.SeekStart)
			}
			response, err = operation(ctx, request, rsc, extraHeaders)

			operationResponse := NewOCIOperationResponseExtended(response, err, currentOperationAttempt, endOfWindowTime, backoffScalingFactor, initialAttemptTime)

			if !policyToUse.ShouldRetryOperation(operationResponse) {
				// we should NOT retry operation based on response and/or error => return
				retrierChannel <- retrierResult{response, err}
				return
			}

			// if the request body type is stream, requested retry but doesn't resettable, throw error and stop retrying
			if isBinaryRequest && !isSeekable {
				retrierChannel <- retrierResult{response, NonSeekableRequestRetryFailure{err}}
				return
			}

			duration := policyToUse.NextDuration(operationResponse)
			//The following condition is kept for backwards compatibility reasons
			if deadline, ok := ctx.Deadline(); ok && EcContext.timeNowProvider().Add(duration).After(deadline) {
				// we want to retry the operation, but the policy is telling us to wait for a duration that exceeds
				// the specified overall deadline for the operation => instead of waiting for however long that
				// time period is and then aborting, abort now and save the cycles
				retrierChannel <- retrierResult{response, DeadlineExceededByBackoff}
				return
			}
			Debugln(fmt.Sprintf("waiting %v before retrying operation", duration))
			// sleep before retrying the operation
			<-time.After(duration)
		}
		retryEndTime := time.Now()
		Debugln(fmt.Sprintf("Total Latency for this API call is: %v ms", retryEndTime.Sub(retryStartTime).Milliseconds()))
		retrierChannel <- retrierResult{response, err}
	}()

	select {
	case <-ctx.Done():
		return response, ctx.Err()
	case result := <-retrierChannel:
		return result.response, result.err
	}
}
