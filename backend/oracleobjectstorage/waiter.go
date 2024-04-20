//go:build !plan9 && !solaris && !js

package oracleobjectstorage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
)

var refreshGracePeriod = 30 * time.Second

// StateRefreshFunc is a function type used for StateChangeConf that is
// responsible for refreshing the item being watched for a state change.
//
// It returns three results. `result` is any object that will be returned
// as the final object after waiting for state change. This allows you to
// return the final updated object, for example an EC2 instance after refreshing
// it. A nil result represents not found.
//
// `state` is the latest state of that object. And `err` is any error that
// may have happened while refreshing the state.
type StateRefreshFunc func() (result interface{}, state string, err error)

// StateChangeConf is the configuration struct used for `WaitForState`.
type StateChangeConf struct {
	Delay          time.Duration    // Wait this time before starting checks
	Pending        []string         // States that are "allowed" and will continue trying
	Refresh        StateRefreshFunc // Refreshes the current state
	Target         []string         // Target state
	Timeout        time.Duration    // The amount of time to wait before timeout
	MinTimeout     time.Duration    // Smallest time to wait before refreshes
	PollInterval   time.Duration    // Override MinTimeout/backoff and only poll this often
	NotFoundChecks int              // Number of times to allow not found (nil result from Refresh)

	// This is to work around inconsistent APIs
	ContinuousTargetOccurrence int // Number of times the Target state has to occur continuously
}

// WaitForStateContext watches an object and waits for it to achieve the state
// specified in the configuration using the specified Refresh() func,
// waiting the number of seconds specified in the timeout configuration.
//
// If the Refresh function returns an error, exit immediately with that error.
//
// If the Refresh function returns a state other than the Target state or one
// listed in Pending, return immediately with an error.
//
// If the Timeout is exceeded before reaching the Target state, return an
// error.
//
// Otherwise, the result is the result of the first call to the Refresh function to
// reach the target state.
//
// Cancellation from the passed in context will cancel the refresh loop
func (conf *StateChangeConf) WaitForStateContext(ctx context.Context, entityType string) (interface{}, error) {
	// fs.Debugf(entityType, "Waiting for state to become: %s", conf.Target)

	notfoundTick := 0
	targetOccurrence := 0

	// Set a default for times to check for not found
	if conf.NotFoundChecks == 0 {
		conf.NotFoundChecks = 20
	}

	if conf.ContinuousTargetOccurrence == 0 {
		conf.ContinuousTargetOccurrence = 1
	}

	type Result struct {
		Result interface{}
		State  string
		Error  error
		Done   bool
	}

	// Read every result from the refresh loop, waiting for a positive result.Done.
	resCh := make(chan Result, 1)
	// cancellation channel for the refresh loop
	cancelCh := make(chan struct{})

	result := Result{}

	go func() {
		defer close(resCh)

		select {
		case <-time.After(conf.Delay):
		case <-cancelCh:
			return
		}

		// start with 0 delay for the first loop
		var wait time.Duration

		for {
			// store the last result
			resCh <- result

			// wait and watch for cancellation
			select {
			case <-cancelCh:
				return
			case <-time.After(wait):
				// first round had no wait
				if wait == 0 {
					wait = 100 * time.Millisecond
				}
			}

			res, currentState, err := conf.Refresh()
			result = Result{
				Result: res,
				State:  currentState,
				Error:  err,
			}

			if err != nil {
				resCh <- result
				return
			}

			// If we're waiting for the absence of a thing, then return
			if res == nil && len(conf.Target) == 0 {
				targetOccurrence++
				if conf.ContinuousTargetOccurrence == targetOccurrence {
					result.Done = true
					resCh <- result
					return
				}
				continue
			}

			if res == nil {
				// If we didn't find the resource, check if we have been
				// not finding it for a while, and if so, report an error.
				notfoundTick++
				if notfoundTick > conf.NotFoundChecks {
					result.Error = &NotFoundError{
						LastError: err,
						Retries:   notfoundTick,
					}
					resCh <- result
					return
				}
			} else {
				// Reset the counter for when a resource isn't found
				notfoundTick = 0
				found := false

				for _, allowed := range conf.Target {
					if currentState == allowed {
						found = true
						targetOccurrence++
						if conf.ContinuousTargetOccurrence == targetOccurrence {
							result.Done = true
							resCh <- result
							return
						}
						continue
					}
				}

				for _, allowed := range conf.Pending {
					if currentState == allowed {
						found = true
						targetOccurrence = 0
						break
					}
				}

				if !found && len(conf.Pending) > 0 {
					result.Error = &UnexpectedStateError{
						LastError:     err,
						State:         result.State,
						ExpectedState: conf.Target,
					}
					resCh <- result
					return
				}
			}

			// Wait between refreshes using exponential backoff, except when
			// waiting for the target state to reoccur.
			if targetOccurrence == 0 {
				wait *= 2
			}

			// If a poll interval has been specified, choose that interval.
			// Otherwise, bound the default value.
			if conf.PollInterval > 0 && conf.PollInterval < 180*time.Second {
				wait = conf.PollInterval
			} else {
				if wait < conf.MinTimeout {
					wait = conf.MinTimeout
				} else if wait > 10*time.Second {
					wait = 10 * time.Second
				}
			}

			// fs.Debugf(entityType, "[TRACE] Waiting %s before next try", wait)
		}
	}()

	// store the last value result from the refresh loop
	lastResult := Result{}

	timeout := time.After(conf.Timeout)
	for {
		select {
		case r, ok := <-resCh:
			// channel closed, so return the last result
			if !ok {
				return lastResult.Result, lastResult.Error
			}

			// we reached the intended state
			if r.Done {
				return r.Result, r.Error
			}

			// still waiting, store the last result
			lastResult = r
		case <-ctx.Done():
			close(cancelCh)
			return nil, ctx.Err()
		case <-timeout:
			// fs.Debugf(entityType, "[WARN] WaitForState timeout after %s", conf.Timeout)
			// fs.Debugf(entityType, "[WARN] WaitForState starting %s refresh grace period", refreshGracePeriod)

			// cancel the goroutine and start our grace period timer
			close(cancelCh)
			timeout := time.After(refreshGracePeriod)

			// we need a for loop and a label to break on, because we may have
			// an extra response value to read, but still want to wait for the
			// channel to close.
		forSelect:
			for {
				select {
				case r, ok := <-resCh:
					if r.Done {
						// the last refresh loop reached the desired state
						return r.Result, r.Error
					}

					if !ok {
						// the goroutine returned
						break forSelect
					}

					// target state not reached, save the result for the
					// TimeoutError and wait for the channel to close
					lastResult = r
				case <-ctx.Done():
					fs.Errorf(entityType, "Context cancellation detected, abandoning grace period")
					break forSelect
				case <-timeout:
					fs.Errorf(entityType, "WaitForState exceeded refresh grace period")
					break forSelect
				}
			}

			return nil, &TimeoutError{
				LastError:     lastResult.Error,
				LastState:     lastResult.State,
				Timeout:       conf.Timeout,
				ExpectedState: conf.Target,
			}
		}
	}
}

// NotFoundError resource not found error
type NotFoundError struct {
	LastError    error
	LastRequest  interface{}
	LastResponse interface{}
	Message      string
	Retries      int
}

func (e *NotFoundError) Error() string {
	if e.Message != "" {
		return e.Message
	}

	if e.Retries > 0 {
		return fmt.Sprintf("couldn't find resource (%d retries)", e.Retries)
	}

	return "couldn't find resource"
}

func (e *NotFoundError) Unwrap() error {
	return e.LastError
}

// UnexpectedStateError is returned when Refresh returns a state that's neither in Target nor Pending
type UnexpectedStateError struct {
	LastError     error
	State         string
	ExpectedState []string
}

func (e *UnexpectedStateError) Error() string {
	return fmt.Sprintf(
		"unexpected state '%s', wanted target '%s'. last error: %s",
		e.State,
		strings.Join(e.ExpectedState, ", "),
		e.LastError,
	)
}

func (e *UnexpectedStateError) Unwrap() error {
	return e.LastError
}

// TimeoutError is returned when WaitForState times out
type TimeoutError struct {
	LastError     error
	LastState     string
	Timeout       time.Duration
	ExpectedState []string
}

func (e *TimeoutError) Error() string {
	expectedState := "resource to be gone"
	if len(e.ExpectedState) > 0 {
		expectedState = fmt.Sprintf("state to become '%s'", strings.Join(e.ExpectedState, ", "))
	}

	extraInfo := make([]string, 0)
	if e.LastState != "" {
		extraInfo = append(extraInfo, fmt.Sprintf("last state: '%s'", e.LastState))
	}
	if e.Timeout > 0 {
		extraInfo = append(extraInfo, fmt.Sprintf("timeout: %s", e.Timeout.String()))
	}

	suffix := ""
	if len(extraInfo) > 0 {
		suffix = fmt.Sprintf(" (%s)", strings.Join(extraInfo, ", "))
	}

	if e.LastError != nil {
		return fmt.Sprintf("timeout while waiting for %s%s: %s",
			expectedState, suffix, e.LastError)
	}

	return fmt.Sprintf("timeout while waiting for %s%s",
		expectedState, suffix)
}

func (e *TimeoutError) Unwrap() error {
	return e.LastError
}
