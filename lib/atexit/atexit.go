// Package atexit provides handling for functions you want called when
// the program exits unexpectedly due to a signal.
//
// You should also make sure you call Run in the normal exit path.
package atexit

import (
	"os"
	"os/signal"
	"sync"

	"github.com/rclone/rclone/fs"
)

var (
	fns          = make(map[FnHandle]bool)
	fnsMutex     sync.Mutex
	exitChan     chan os.Signal
	exitOnce     sync.Once
	registerOnce sync.Once
)

// FnHandle is the type of the handle returned by function `Register`
// that can be used to unregister an at-exit function
type FnHandle *func()

// Register a function to be called on exit.
// Returns a handle which can be used to unregister the function with `Unregister`.
func Register(fn func()) FnHandle {
	fnsMutex.Lock()
	fns[&fn] = true
	fnsMutex.Unlock()

	// Run AtExit handlers on exitSignals so everything gets tidied up properly
	registerOnce.Do(func() {
		exitChan = make(chan os.Signal, 1)
		signal.Notify(exitChan, exitSignals...)
		go func() {
			sig := <-exitChan
			if sig == nil {
				return
			}
			fs.Infof(nil, "Signal received: %s", sig)
			Run()
			fs.Infof(nil, "Exiting...")
			os.Exit(0)
		}()
	})

	return &fn
}

// Unregister a function using the handle returned by `Register`
func Unregister(handle FnHandle) {
	fnsMutex.Lock()
	defer fnsMutex.Unlock()
	delete(fns, handle)
}

// IgnoreSignals disables the signal handler and prevents Run from beeing executed automatically
func IgnoreSignals() {
	registerOnce.Do(func() {})
	if exitChan != nil {
		signal.Stop(exitChan)
		close(exitChan)
		exitChan = nil
	}
}

// Run all the at exit functions if they haven't been run already
func Run() {
	exitOnce.Do(func() {
		fnsMutex.Lock()
		defer fnsMutex.Unlock()
		for fnHandle := range fns {
			(*fnHandle)()
		}
	})
}
