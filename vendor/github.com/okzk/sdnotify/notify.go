// +build !linux

package sdnotify

func SdNotify(state string) error {
	// do nothing
	return nil
}
