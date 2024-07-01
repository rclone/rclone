// Package errcount provides an easy to use error counter which
// returns error count and last error so as to not overwhelm the user
// with errors.
package errcount

import (
	"fmt"
	"sync"
)

// ErrCount stores the state of the error counter.
type ErrCount struct {
	mu      sync.Mutex
	lastErr error
	count   int
}

// New makes a new error counter
func New() *ErrCount {
	return new(ErrCount)
}

// Add an error to the error count.
//
// err may be nil.
//
// Thread safe.
func (ec *ErrCount) Add(err error) {
	if err == nil {
		return
	}
	ec.mu.Lock()
	ec.count++
	ec.lastErr = err
	ec.mu.Unlock()
}

// Err returns the error summary so far - may be nil
//
// txt is put in front of the error summary
//
//	txt: %d errors: last error: %w
//
// or this if only one error
//
//	txt: %w
//
// Thread safe.
func (ec *ErrCount) Err(txt string) error {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	if ec.count == 0 {
		return nil
	} else if ec.count == 1 {
		return fmt.Errorf("%s: %w", txt, ec.lastErr)
	}
	return fmt.Errorf("%s: %d errors: last error: %w", txt, ec.count, ec.lastErr)
}
