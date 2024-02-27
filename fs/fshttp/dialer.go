package fshttp

import (
	"context"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// Dialer structure contains default dialer and timeout, tclass support
type Dialer struct {
	net.Dialer
	timeout time.Duration
	tclass  int
}

// NewDialer creates a Dialer structure with Timeout, Keepalive,
// LocalAddr and DSCP set from rclone flags.
func NewDialer(ctx context.Context) *Dialer {
	ci := fs.GetConfig(ctx)
	dialer := &Dialer{
		Dialer: net.Dialer{
			Timeout:   ci.ConnectTimeout,
			KeepAlive: 30 * time.Second,
		},
		timeout: ci.Timeout,
		tclass:  int(ci.TrafficClass),
	}
	if ci.BindAddr != nil {
		dialer.Dialer.LocalAddr = &net.TCPAddr{IP: ci.BindAddr}
	}
	return dialer
}

// Dial connects to the network address.
func (d *Dialer) Dial(network, address string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, address)
}

var warnDSCPFail, warnDSCPWindows sync.Once

// DialContext connects to the network address using the provided context.
func (d *Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	// If local address is 0.0.0.0 or ::0 force IPv4 or IPv6
	// This works around https://github.com/golang/go/issues/48723
	// Which means 0.0.0.0 and ::0 both bind to both IPv4 and IPv6
	if ip, ok := d.Dialer.LocalAddr.(*net.TCPAddr); ok && ip.IP.IsUnspecified() && (network == "tcp" || network == "udp") {
		if ip.IP.To4() != nil {
			network += "4" // IPv4 address
		} else {
			network += "6" // IPv6 address
		}
	}

	c, err := d.Dialer.DialContext(ctx, network, address)
	if err != nil {
		return c, err
	}

	if d.tclass != 0 {
		// IPv6 addresses must have two or more ":"
		if strings.Count(c.RemoteAddr().String(), ":") > 1 {
			err = ipv6.NewConn(c).SetTrafficClass(d.tclass)
		} else {
			err = ipv4.NewConn(c).SetTOS(d.tclass)
			// Warn of silent failure on Windows (IPv4 only, IPv6 caught by error handler)
			if runtime.GOOS == "windows" {
				warnDSCPWindows.Do(func() {
					fs.LogLevelPrintf(fs.LogLevelWarning, nil, "dialer: setting DSCP on Windows/IPv4 fails silently; see https://github.com/golang/go/issues/42728")
				})
			}
		}
		if err != nil {
			warnDSCPFail.Do(func() {
				fs.LogLevelPrintf(fs.LogLevelWarning, nil, "dialer: failed to set DSCP socket options: %v", err)
			})
		}
	}

	t := &timeoutConn{
		Conn:    c,
		timeout: d.timeout,
	}
	return t, t.nudgeDeadline()
}

// A net.Conn that sets deadline for every Read/Write operation
type timeoutConn struct {
	net.Conn
	timeout time.Duration
}

// Nudge the deadline for an idle timeout on by c.timeout if non-zero
func (c *timeoutConn) nudgeDeadline() error {
	if c.timeout > 0 {
		return c.SetDeadline(time.Now().Add(c.timeout))
	}
	return nil
}

// Read bytes with rate limiting and idle timeouts
func (c *timeoutConn) Read(b []byte) (n int, err error) {
	// Ideally we would LimitBandwidth(len(b)) here and replace tokens we didn't use
	n, err = c.Conn.Read(b)
	accounting.TokenBucket.LimitBandwidth(accounting.TokenBucketSlotTransportRx, n)
	if err == nil && n > 0 && c.timeout > 0 {
		err = c.nudgeDeadline()
	}
	return n, err
}

// Write bytes with rate limiting and idle timeouts
func (c *timeoutConn) Write(b []byte) (n int, err error) {
	accounting.TokenBucket.LimitBandwidth(accounting.TokenBucketSlotTransportTx, len(b))
	n, err = c.Conn.Write(b)
	if err == nil && n > 0 && c.timeout > 0 {
		err = c.nudgeDeadline()
	}
	return n, err
}
