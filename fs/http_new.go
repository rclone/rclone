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
func dialContextTimeout(ctx context.Context, network, address string, connectTimeout, timeout time.Duration) (net.Conn, error) {
	dialer := net.Dialer{
		Timeout:   connectTimeout,
		KeepAlive: 30 * time.Second,
	}
	c, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return c, err
	}
	return newTimeoutConn(c, timeout), nil
}

// Initialise the http.Transport for go1.7+
func (ci *ConfigInfo) initTransport(t *http.Transport) {
	t.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return dialContextTimeout(ctx, network, address, ci.ConnectTimeout, ci.Timeout)
	}
	t.IdleConnTimeout = 60 * time.Second
	t.ExpectContinueTimeout = ci.ConnectTimeout
}
