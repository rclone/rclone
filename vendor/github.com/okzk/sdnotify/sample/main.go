package main

import (
	"github.com/okzk/sdnotify"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func reload() {
	// Tells the service manager that the service is reloading its configuration.
	sdnotify.SdNotifyReloading()

	log.Println("reloading...")
	time.Sleep(time.Second)
	log.Println("reloaded.")

	// The service must also send a "READY" notification when it completed reloading its configuration.
	sdnotify.SdNotifyReady()
}

func main() {
	log.Println("starting...")
	time.Sleep(time.Second)
	log.Println("started.")

	// Tells the service manager that service startup is finished.
	sdnotify.SdNotifyReady()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	for sig := range sigCh {
		if sig == syscall.SIGHUP {
			reload()
		} else {
			break
		}
	}

	// Tells the service manager that the service is beginning its shutdown.
	sdnotify.SdNotifyStopping()

	log.Println("existing...")
	time.Sleep(time.Second)
}
