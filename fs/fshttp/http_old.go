// HTTP parts pre go1.7

//+build !go1.7

package fshttp

import (
	"net"
	"net/http"

	"github.com/ncw/rclone/fs"
)

// dial with timeouts
func dialTimeout(network, address string, ci *fs.ConfigInfo) (net.Conn, error) {
	dialer := NewDialer(ci)
	c, err := dialer.Dial(network, address)
	if err != nil {
		return c, err
	}
	return newTimeoutConn(c, ci.Timeout), nil
}

// Initialise the http.Transport for pre go1.7
func initTransport(ci *fs.ConfigInfo, t *http.Transport) {
	t.Dial = func(network, addr string) (net.Conn, error) {
		return dialTimeout(network, addr, ci)
	}
}
