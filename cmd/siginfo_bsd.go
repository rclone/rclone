//go:build darwin || freebsd || netbsd || dragonfly || openbsd

package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
)

// SigInfoHandler creates SigInfo handler
func SigInfoHandler() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINFO)
	go func() {
		for range signals {
			fs.Printf(nil, "%v\n", accounting.GlobalStats())
		}
	}()
}
