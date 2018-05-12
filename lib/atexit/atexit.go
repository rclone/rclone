// Package atexit provides handling for functions you want called when
// the program exits unexpectedly due to a signal.
//
// You should also make sure you call Run in the normal exit path.
package atexit

import (
	"os"
	"os/signal"
	"sync"

	"github.com/ncw/rclone/fs"
)

var (
	fns          []func()
	exitChan     chan os.Signal
	exitOnce     sync.Once
	registerOnce sync.Once
)

// Register a function to be called on exit
func Register(fn func()) {
	fns = append(fns, fn)
	// Run AtExit handlers on SIGINT or SIGTERM so everything gets
	// tidied up properly
	registerOnce.Do(func() {
		exitChan = make(chan os.Signal, 1)
		signal.Notify(exitChan, os.Interrupt) // syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT
		go func() {
			sig, closed := <-exitChan
			if closed || sig == nil {
				return
			}
			fs.Infof(nil, "Signal received: %s", sig)
			Run()
			fs.Infof(nil, "Exiting...")
			os.Exit(0)
		}()
	})
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
		for _, fn := range fns {
			fn()
		}
	})
}
