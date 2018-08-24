// +build !linux

package sdnotify

// SdNotify sends a specified string to the systemd notification socket.
func SdNotify(state string) error {
	// do nothing
	return nil
}
