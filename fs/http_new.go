// HTTP parts go1.7+

//+build go1.7

package fs

import (
	"context"
	"net"
	"net/http"
	"time"
)

// dial with context and timeouts
func (ci *ConfigInfo) dialContextTimeout(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := ci.NewDialer()
	c, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return c, err
	}
	return newTimeoutConn(c, ci.Timeout), nil
}

// Initialise the http.Transport for go1.7+
func (ci *ConfigInfo) initTransport(t *http.Transport) {
	t.DialContext = ci.dialContextTimeout
	t.IdleConnTimeout = 60 * time.Second
	t.ExpectContinueTimeout = ci.ConnectTimeout
}
