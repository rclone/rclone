// HTTP parts pre go1.7

//+build !go1.7

package fs

import (
	"net"
	"net/http"
)

// dial with timeouts
func (ci *ConfigInfo) dialTimeout(network, address string) (net.Conn, error) {
	dialer := ci.NewDialer()
	c, err := dialer.Dial(network, address)
	if err != nil {
		return c, err
	}
	return newTimeoutConn(c, ci.Timeout), nil
}

// Initialise the http.Transport for pre go1.7
func (ci *ConfigInfo) initTransport(t *http.Transport) {
	t.Dial = ci.dialTimeout
}
