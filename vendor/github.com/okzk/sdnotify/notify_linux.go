package sdnotify

import (
	"net"
	"os"
)

// SdNotify sends a specified string to the systemd notification socket.
func SdNotify(state string) error {
	name := os.Getenv("NOTIFY_SOCKET")
	if name == "" {
		return ErrSdNotifyNoSocket
	}

	conn, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: name, Net: "unixgram"})
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write([]byte(state))
	return err
}
