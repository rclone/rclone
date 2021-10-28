//go:build deadlock
// +build deadlock

package sync

import (
	"sync"

	"github.com/sasha-s/go-deadlock"
)

type (
	// Cond implements a condition variable, a rendezvous point
	// for goroutines waiting for or announcing the occurrence
	// of an event.
	//
	// For full docs see the standard library sync docs
	Cond = sync.Cond

	// A Locker represents an object that can be locked and unlocked.
	//
	// For full docs see the standard library sync docs
	Locker = sync.Locker

	// Map is like a Go map[interface{}]interface{} but is safe for concurrent use
	// by multiple goroutines without additional locking or coordination.
	// Loads, stores, and deletes run in amortized constant time.
	//
	// For full docs see the standard library sync docs
	Map = sync.Map

	// A Mutex is a mutual exclusion lock.
	//
	// For full docs see the standard library sync docs
	Mutex = deadlock.Mutex

	// Once is an object that will perform exactly one action.
	//
	// For full docs see the standard library sync docs
	Once = sync.Once

	// A Pool is a set of temporary objects that may be individually saved and
	// retrieved.
	//
	// For full docs see the standard library sync docs
	Pool = sync.Pool

	// A RWMutex is a reader/writer mutual exclusion lock.
	// The lock can be held by an arbitrary number of readers or a single writer.
	// The zero value for a RWMutex is an unlocked mutex.
	//
	// For full docs see the standard library sync docs
	RWMutex = deadlock.RWMutex

	// A WaitGroup waits for a collection of goroutines to finish.
	// The main goroutine calls Add to set the number of
	// goroutines to wait for. Then each of the goroutines
	// runs and calls Done when finished. At the same time,
	// Wait can be used to block until all goroutines have finished.
	//
	// For full docs see the standard library sync docs
	WaitGroup = sync.WaitGroup
)

var (
	// NewCond returns a new Cond with Locker l.
	NewCond = sync.NewCond
)
