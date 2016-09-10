// HTTP parts pre go1.7

//+build !go1.7

package fs

import (
	"net"
	"net/http"
	"time"
)

// dial with timeouts
func dialTimeout(network, address string, connectTimeout, timeout time.Duration) (net.Conn, error) {
	dialer := net.Dialer{
		Timeout:   connectTimeout,
		KeepAlive: 30 * time.Second,
	}
	c, err := dialer.Dial(network, address)
	if err != nil {
		return c, err
	}
	return newTimeoutConn(c, timeout), nil
}

// Initialise the http.Transport for pre go1.7
func (ci *ConfigInfo) initTransport(t *http.Transport) {
	t.Dial = func(network, address string) (net.Conn, error) {
		return dialTimeout(network, address, ci.ConnectTimeout, ci.Timeout)
	}
}
