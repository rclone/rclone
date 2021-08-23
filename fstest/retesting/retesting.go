// Package retesting retries flakey tests
package retesting

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"testing"
)

const defaultFlakeyRetries = 0 // off by default

// Globals
var (
	Debug         = false
	FlakeyRetries = flag.Int("flakey-retries", getDefaultRetries(), "Number of flakey test retriesi (0 = no retries)")
)

func getDefaultRetries() int {
	val, err := strconv.Atoi(os.Getenv("RCLONE_FLAKEY_RETRIES"))
	if err != nil {
		val = defaultFlakeyRetries
	}
	return val
}

// T is the interface representing the standard *testing.T
// It allows to implement and use custom test drivers.
//
// In unit tests you can just pass a *testing.T struct.
// testify/assert and testify/require accept it seamlessly.
type T interface {
	Cleanup(func())
	// Deadline() (time.Time, bool) // go 1.15+
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fail()
	FailNow()
	Failed() bool
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Helper()
	Log(args ...interface{})
	Logf(format string, args ...interface{})
	Name() string
	Parallel()
	Run(name string, f func(t *testing.T)) bool
	Skip(args ...interface{})
	// Setenv(key, val string) // go 1.17+
	SkipNow()
	Skipf(format string, args ...interface{})
	Skipped() bool
	// TempDir() string // go 1.15+
}

// retrier mimics testing.T and retries failing tests
type retrier struct {
	mu       sync.RWMutex
	name     string
	t        T        // connected testing.T
	parent   *retrier // parent flakey test
	retryNum int
	failNum  int
	failMax  int
	failed   bool
	finished bool
}

// Run mimics testing.T.Run but runs subtest with retries.
// retesting.Run(t, name, f) is similar to normal t.Run(name, f).
// The core difference is in prototype of the callback func:
// func(t retesting.T) vs func(t *testing.T).
//
// Subtest of *testing.T is called the "core" retriable test.
// Only the core test will be retried after failures.
//
// Subtests of the core retriable test are not retried on their own.
// They just propagate encountered failures to the core test
// and get called again when the core test is retried.
//
// All failed test attempts are marked as skipped in go test.
func Run(t T, name string, f func(t T)) bool {
	t.Helper()
	if r, ok := t.(*retrier); ok {
		return r.runFlakeySubtest(name, f)
	}
	return runCoreFlakeyTest(t, name, f)
}

func runCoreFlakeyTest(t T, name string, f func(t T)) bool {
	t.Helper()
	r := &retrier{
		name:    name,
		failMax: *FlakeyRetries,
	}
	if r.failMax <= 0 {
		r.failMax = 1
	}

	r.failed = true // so the loop starts
	for r.failed && r.failNum < r.failMax {
		r.failed = false
		r.retryNum++
		retryName := fmt.Sprintf("%s_%d", r.name, r.retryNum)
		t.Run(retryName, func(tt *testing.T) {
			r.t = tt
			f(r)
			if r.failed && r.failNum < r.failMax {
				r.SkipNow() // mark ignored failures as skipped
			}
		})
	}

	return !r.failed
}

func (r *retrier) runFlakeySubtest(name string, f func(t T)) bool {
	r.t.Helper()
	if Debug {
		log.Printf("retesting %s: spawn subtest %s", r.name, name)
	}
	wrapper := func(t *testing.T) {
		rr := &retrier{
			parent:  r,
			t:       t,
			name:    name,
			failMax: r.failMax,
		}
		f(rr)
		if rr.failed && rr.failNum < rr.failMax {
			rr.SkipNow() // mark ignored failures as skipped
		}
	}
	return r.t.Run(name, wrapper)
}

//
// Overridden methods of retrier vs *testing.T
//

// Fail marks the function as having failed but continues execution.
func (r *retrier) Fail() {
	r.t.Helper()

	if r.parent != nil {
		r.parent.Fail() // propagate to parent retrier
	}

	r.mu.Lock()
	r.failed = true
	r.failNum++
	failNum := r.failNum
	r.mu.Unlock()

	if failNum < r.failMax {
		if Debug {
			log.Printf("retesting %s: fail #%d ignored", r.name, failNum)
		}
		return // don't raise it to connected testing.T, just mark in r.failed
	}

	if Debug {
		log.Printf("retesting %s: fail #%d raised", r.name, failNum)
	}
	r.t.Fail() // raise the failure
}

// FailNow marks the function as having failed and stops its execution.
func (r *retrier) FailNow() {
	r.Fail()
	r.finished = true
	r.t.SkipNow()
}

// Failed reports whether the function has failed.
func (r *retrier) Failed() bool {
	r.mu.RLock()
	failed := r.failed
	r.mu.RUnlock()
	return failed
}

// Error is equivalent to Log followed by Fail.
func (r *retrier) Error(args ...interface{}) {
	r.t.Helper()
	r.Log(args...)
	r.Fail()
}

// Errorf is equivalent to Logf followed by Fail.
func (r *retrier) Errorf(format string, args ...interface{}) {
	r.t.Helper()
	r.Logf(format, args...)
	r.Fail()
}

// Fatal is equivalent to Log followed by FailNow.
func (r *retrier) Fatal(args ...interface{}) {
	r.t.Helper()
	r.Log(args...)
	r.FailNow()
}

// Fatalf is equivalent to Logf followed by FailNow.
func (r *retrier) Fatalf(format string, args ...interface{}) {
	r.t.Helper()
	r.Logf(format, args...)
	r.FailNow()
}

//
// Logging
//

// Log prints a message
func (r *retrier) Log(args ...interface{}) {
	r.t.Helper()
	r.t.Log(args...)
}

// Logf prints a message
func (r *retrier) Logf(format string, args ...interface{}) {
	r.t.Helper()
	r.t.Logf(format, args...)
}

//
// Inherited methods of retrier vs *testing.T
//

// Cleanup (inherited)
func (r *retrier) Cleanup(f func()) {
	r.t.Helper()
	r.t.Cleanup(f)
}

// Helper (inherited)
func (r *retrier) Helper() {
	r.t.Helper()
}

// Name (inherited)
func (r *retrier) Name() string {
	return r.t.Name()
}

// Parallel (inherited)
func (r *retrier) Parallel() {
	r.t.Parallel()
}

// Run (inherited)
func (r *retrier) Run(name string, f func(t *testing.T)) bool {
	r.t.Helper()
	return r.t.Run(name, f)
}

// Skip (inherited)
func (r *retrier) Skip(args ...interface{}) {
	r.t.Helper()
	r.t.Skip(args...)
}

// SkipNow (inherited)
func (r *retrier) SkipNow() {
	r.t.SkipNow()
}

// Skipf (inherited)
func (r *retrier) Skipf(format string, args ...interface{}) {
	r.t.Helper()
	r.t.Skipf(format, args...)
}

// Skipped (inherited)
func (r *retrier) Skipped() bool {
	return r.t.Skipped()
}
