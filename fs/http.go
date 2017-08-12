// The HTTP based parts of the config, Transport and Client

package fs

import (
	"bytes"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httputil"
	"reflect"
	"sync"
	"time"

	"golang.org/x/net/context" // switch to "context" when we stop supporting go1.6
	"golang.org/x/time/rate"
)

const (
	separatorReq  = ">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>"
	separatorResp = "<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<"
)

var (
	transport   http.RoundTripper
	noTransport sync.Once
	tpsBucket   *rate.Limiter // for limiting number of http transactions per second
)

// Start the token bucket if necessary
func startHTTPTokenBucket() {
	if Config.TPSLimit > 0 {
		tpsBurst := Config.TPSLimitBurst
		if tpsBurst < 1 {
			tpsBurst = 1
		}
		tpsBucket = rate.NewLimiter(rate.Limit(Config.TPSLimit), tpsBurst)
		Infof(nil, "Starting HTTP transaction limiter: max %g transactions/s with burst %d", Config.TPSLimit, tpsBurst)
	}
}

// A net.Conn that sets a deadline for every Read or Write operation
type timeoutConn struct {
	net.Conn
	readTimer  *time.Timer
	writeTimer *time.Timer
	timeout    time.Duration
	off        time.Time
}

// create a timeoutConn using the timeout
func newTimeoutConn(conn net.Conn, timeout time.Duration) *timeoutConn {
	return &timeoutConn{
		Conn:    conn,
		timeout: timeout,
	}
}

// Nudge the deadline for an idle timeout on by c.timeout if non-zero
func (c *timeoutConn) nudgeDeadline() (err error) {
	if c.timeout == 0 {
		return nil
	}
	when := time.Now().Add(c.timeout)
	return c.Conn.SetDeadline(when)
}

// readOrWrite bytes doing idle timeouts
func (c *timeoutConn) readOrWrite(f func([]byte) (int, error), b []byte) (n int, err error) {
	err = c.nudgeDeadline()
	if err != nil {
		return n, err
	}
	n, err = f(b)
	cerr := c.nudgeDeadline()
	if err == nil && cerr != nil {
		err = cerr
	}
	return n, err
}

// Read bytes doing idle timeouts
func (c *timeoutConn) Read(b []byte) (n int, err error) {
	return c.readOrWrite(c.Conn.Read, b)
}

// Write bytes doing idle timeouts
func (c *timeoutConn) Write(b []byte) (n int, err error) {
	return c.readOrWrite(c.Conn.Write, b)
}

// setDefaults for a from b
//
// Copy the public members from b to a.  We can't just use a struct
// copy as Transport contains a private mutex.
func setDefaults(a, b interface{}) {
	pt := reflect.TypeOf(a)
	t := pt.Elem()
	va := reflect.ValueOf(a).Elem()
	vb := reflect.ValueOf(b).Elem()
	for i := 0; i < t.NumField(); i++ {
		aField := va.Field(i)
		// Set a from b if it is public
		if aField.CanSet() {
			bField := vb.Field(i)
			aField.Set(bField)
		}
	}
}

// Transport returns an http.RoundTripper with the correct timeouts
func (ci *ConfigInfo) Transport() http.RoundTripper {
	noTransport.Do(func() {
		// Start with a sensible set of defaults then override.
		// This also means we get new stuff when it gets added to go
		t := new(http.Transport)
		setDefaults(t, http.DefaultTransport.(*http.Transport))
		t.Proxy = http.ProxyFromEnvironment
		t.MaxIdleConnsPerHost = 4 * (ci.Checkers + ci.Transfers + 1)
		t.TLSHandshakeTimeout = ci.ConnectTimeout
		t.ResponseHeaderTimeout = ci.Timeout
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: ci.InsecureSkipVerify}
		t.DisableCompression = *noGzip
		// Set in http_old.go initTransport
		//   t.Dial
		// Set in http_new.go initTransport
		//   t.DialContext
		//   t.IdelConnTimeout
		//   t.ExpectContinueTimeout
		ci.initTransport(t)
		// Wrap that http.Transport in our own transport
		transport = NewTransport(t, ci.DumpHeaders, ci.DumpBodies, ci.DumpAuth)
	})
	return transport
}

// Client returns an http.Client with the correct timeouts
func (ci *ConfigInfo) Client() *http.Client {
	return &http.Client{
		Transport: ci.Transport(),
	}
}

// Transport is a our http Transport which wraps an http.Transport
// * Sets the User Agent
// * Does logging
type Transport struct {
	*http.Transport
	logHeader bool
	logBody   bool
	logAuth   bool
}

// NewTransport wraps the http.Transport passed in and logs all
// roundtrips including the body if logBody is set.
func NewTransport(transport *http.Transport, logHeader, logBody, logAuth bool) *Transport {
	return &Transport{
		Transport: transport,
		logHeader: logHeader,
		logBody:   logBody,
		logAuth:   logAuth,
	}
}

// A mutex to protect this map
var checkedHostMu sync.RWMutex

// A map of servers we have checked for time
var checkedHost = make(map[string]struct{}, 1)

// Check the server time is the same as ours, once for each server
func checkServerTime(req *http.Request, resp *http.Response) {
	host := req.URL.Host
	if req.Host != "" {
		host = req.Host
	}
	checkedHostMu.RLock()
	_, ok := checkedHost[host]
	checkedHostMu.RUnlock()
	if ok {
		return
	}
	dateString := resp.Header.Get("Date")
	if dateString == "" {
		return
	}
	date, err := http.ParseTime(dateString)
	if err != nil {
		Debugf(nil, "Couldn't parse Date: from server %s: %q: %v", host, dateString, err)
		return
	}
	dt := time.Since(date)
	const window = 5 * 60 * time.Second
	if dt > window || dt < -window {
		Logf(nil, "Time may be set wrong - time from %q is %v different from this computer", host, dt)
	}
	checkedHostMu.Lock()
	checkedHost[host] = struct{}{}
	checkedHostMu.Unlock()
}

var authBuf = []byte("Authorization: ")

// cleanAuth gets rid of one Authorization: header within the first 4k
func cleanAuth(buf []byte) []byte {
	// Find how much buffer to check
	n := 4096
	if len(buf) < n {
		n = len(buf)
	}
	// See if there is an Authorization: header
	i := bytes.Index(buf[:n], authBuf)
	if i < 0 {
		return buf
	}
	i += len(authBuf)
	// Overwrite the next 4 chars with 'X'
	for j := 0; i < len(buf) && j < 4; j++ {
		if buf[i] == '\n' {
			break
		}
		buf[i] = 'X'
		i++
	}
	// Snip out to the next '\n'
	j := bytes.IndexByte(buf[i:], '\n')
	if j < 0 {
		return buf[:i]
	}
	n = copy(buf[i:], buf[i+j:])
	return buf[:i+n]
}

// RoundTrip implements the RoundTripper interface.
func (t *Transport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	// Get transactions per second token first if limiting
	if tpsBucket != nil {
		tbErr := tpsBucket.Wait(context.Background()) // FIXME switch to req.Context() when we drop go1.6 support
		if tbErr != nil {
			Errorf(nil, "HTTP token bucket error: %v", err)
		}
	}
	// Force user agent
	req.Header.Set("User-Agent", *userAgent)
	// Logf request
	if t.logHeader || t.logBody || t.logAuth {
		buf, _ := httputil.DumpRequestOut(req, t.logBody)
		if !t.logAuth {
			buf = cleanAuth(buf)
		}
		Debugf(nil, "%s", separatorReq)
		Debugf(nil, "%s (req %p)", "HTTP REQUEST", req)
		Debugf(nil, "%s", string(buf))
		Debugf(nil, "%s", separatorReq)
	}
	// Do round trip
	resp, err = t.Transport.RoundTrip(req)
	// Logf response
	if t.logHeader || t.logBody || t.logAuth {
		Debugf(nil, "%s", separatorResp)
		Debugf(nil, "%s (req %p)", "HTTP RESPONSE", req)
		if err != nil {
			Debugf(nil, "Error: %v", err)
		} else {
			buf, _ := httputil.DumpResponse(resp, t.logBody)
			Debugf(nil, "%s", string(buf))
		}
		Debugf(nil, "%s", separatorResp)
	}
	if err == nil {
		checkServerTime(req, resp)
	}
	return resp, err
}

// NewDialer creates a net.Dialer structure with Timeout, Keepalive
// and LocalAddr set from rclone flags.
func (ci *ConfigInfo) NewDialer() *net.Dialer {
	dialer := &net.Dialer{
		Timeout:   ci.ConnectTimeout,
		KeepAlive: 30 * time.Second,
	}
	if ci.BindAddr != nil {
		dialer.LocalAddr = &net.TCPAddr{IP: ci.BindAddr}
	}
	return dialer
}
