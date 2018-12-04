// Daemonization interface for Unix variants only

// +build !windows

package mountlib

import (
	"log"

	daemon "github.com/sevlyar/go-daemon"
)

func startBackgroundMode() bool {
	cntxt := &daemon.Context{}
	d, err := cntxt.Reborn()
	if err != nil {
		log.Fatalln(err)
	}

	if d != nil {
		return true
	}

	defer func() {
		if err := cntxt.Release(); err != nil {
			log.Printf("error encountered while killing daemon: %v", err)
		}
	}()

	return false
}
