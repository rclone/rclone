// +build linux

package sysdnotify

import (
	"net"
	"os"
)

func init() {
	if notifySocketName := os.Getenv("NOTIFY_SOCKET"); notifySocketName != "" {
		socket = &net.UnixAddr{
			Name: notifySocketName,
			Net:  "unixgram",
		}
	}
}
