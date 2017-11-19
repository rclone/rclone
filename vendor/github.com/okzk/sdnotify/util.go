package sdnotify

import "errors"

var SdNotifyNoSocket = errors.New("No socket")

func SdNotifyReady() error {
	return SdNotify("READY=1")
}

func SdNotifyStopping() error {
	return SdNotify("STOPPING=1")
}

func SdNotifyReloading() error {
	return SdNotify("RELOADING=1")
}

func SdNotifyStatus(status string) error {
	return SdNotify("STATUS=" + status)
}
