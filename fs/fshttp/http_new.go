// HTTP parts go1.7+

//+build go1.7

package fshttp

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/ncw/rclone/fs"
)

// dial with context and timeouts
func dialContextTimeout(ctx context.Context, network, address string, ci *fs.ConfigInfo) (net.Conn, error) {
	dialer := NewDialer(ci)
	c, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return c, err
	}
	return newTimeoutConn(c, ci.Timeout), nil
}

// Initialise the http.Transport for go1.7+
func initTransport(ci *fs.ConfigInfo, t *http.Transport) {
	t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialContextTimeout(ctx, network, addr, ci)
	}
	t.IdleConnTimeout = 60 * time.Second
	t.ExpectContinueTimeout = ci.ConnectTimeout
}
