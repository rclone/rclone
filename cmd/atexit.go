package cmd

// Atexit handling

import (
	"os"
	"os/signal"
	"sync"

	"github.com/ncw/rclone/fs"
)

var (
	atExitFns          []func()
	atExitOnce         sync.Once
	atExitRegisterOnce sync.Once
)

// AtExit registers a function to be added on exit
func AtExit(fn func()) {
	atExitFns = append(atExitFns, fn)
	// Run AtExit handlers on SIGINT or SIGTERM so everything gets
	// tidied up properly
	atExitRegisterOnce.Do(func() {
		go func() {
			ch := make(chan os.Signal, 1)
			signal.Notify(ch, os.Interrupt) // syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT
			sig := <-ch
			fs.Infof(nil, "Signal received: %s", sig)
			runAtExitFunctions()
			fs.Infof(nil, "Exiting...")
			os.Exit(0)
		}()
	})

}

// Runs all the AtExit functions if they haven't been run already
func runAtExitFunctions() {
	atExitOnce.Do(func() {
		for _, fn := range atExitFns {
			fn()
		}
	})
}
