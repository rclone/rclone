//go:build darwin
// +build darwin

package cmd

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/rclone/rclone/fs/accounting"
)

// SigInfoHandler creates SigInfo handler
func SigInfoHandler() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINFO)
	go func() {
		for range signals {
			log.Printf("%v\n", accounting.GlobalStats())
		}
	}()
}
