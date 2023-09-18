package systemd

import (
	"log"
	"sync"

	sysdnotify "github.com/iguanesolutions/go-systemd/v5/notify"
	"github.com/rclone/rclone/lib/atexit"
)

// Notify systemd that the service is starting. This returns a
// function which should be called to notify that the service is
// stopping. This function will be called on exit if the service exits
// on a signal.
func Notify() func() {
	if err := sysdnotify.Ready(); err != nil {
		log.Printf("failed to notify ready to systemd: %v", err)
	}
	var finaliseOnce sync.Once
	finalise := func() {
		finaliseOnce.Do(func() {
			if err := sysdnotify.Stopping(); err != nil {
				log.Printf("failed to notify stopping to systemd: %v", err)
			}
		})
	}
	finaliseHandle := atexit.Register(finalise)
	return func() {
		atexit.Unregister(finaliseHandle)
		finalise()
	}
}
