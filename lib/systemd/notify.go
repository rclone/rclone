package systemd

import (
	"fmt"
	"log"
	"sync"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/rclone/rclone/lib/atexit"
)

// Notify systemd that the service is starting. This returns a
// function which should be called to notify that the service is
// stopping. This function will be called on exit if the service exits
// on a signal.
func Notify() func() {
	if _, err := daemon.SdNotify(false, daemon.SdNotifyReady); err != nil {
		log.Printf("failed to notify ready to systemd: %v", err)
	}
	var finaliseOnce sync.Once
	finalise := func() {
		finaliseOnce.Do(func() {
			if _, err := daemon.SdNotify(false, daemon.SdNotifyStopping); err != nil {
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

// UpdateStatus updates the systemd status
func UpdateStatus(status string) error {
	systemdStatus := fmt.Sprintf("STATUS=%s", status)
	_, err := daemon.SdNotify(false, systemdStatus)
	return err
}
