// The HTTP based parts of the config, Transport and Client

package fs

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httputil"
	"reflect"
	"sync"
	"time"
)

const (
	separatorReq  = ">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>"
	separatorResp = "<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<"
)

var (
	transport   http.RoundTripper
	noTransport sync.Once
)

// A net.Conn that sets a deadline for every Read or Write operation
type timeoutConn struct {
	net.Conn
	readTimer  *time.Timer
	writeTimer *time.Timer
	timeout    time.Duration
	_cancel    func()
	off        time.Time
}

// create a timeoutConn using the timeout
func newTimeoutConn(conn net.Conn, timeout time.Duration) *timeoutConn {
	return &timeoutConn{
		Conn:    conn,
		timeout: timeout,
	}
}

// Read bytes doing timeouts
func (c *timeoutConn) Read(b []byte) (n int, err error) {
	err = c.Conn.SetReadDeadline(time.Now().Add(c.timeout))
	if err != nil {
		return n, err
	}
	n, err = c.Conn.Read(b)
	cerr := c.Conn.SetReadDeadline(c.off)
	if cerr != nil {
		err = cerr
	}
	return n, err
}

// Write bytes doing timeouts
func (c *timeoutConn) Write(b []byte) (n int, err error) {
	err = c.Conn.SetWriteDeadline(time.Now().Add(c.timeout))
	if err != nil {
		return n, err
	}
	n, err = c.Conn.Write(b)
	cerr := c.Conn.SetWriteDeadline(c.off)
	if cerr != nil {
		err = cerr
	}
	return n, err
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
		t.ResponseHeaderTimeout = ci.ConnectTimeout
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
		transport = NewTransport(t, ci.DumpHeaders, ci.DumpBodies)
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
}

// NewTransport wraps the http.Transport passed in and logs all
// roundtrips including the body if logBody is set.
func NewTransport(transport *http.Transport, logHeader, logBody bool) *Transport {
	return &Transport{
		Transport: transport,
		logHeader: logHeader,
		logBody:   logBody,
	}
}

// RoundTrip implements the RoundTripper interface.
func (t *Transport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	// Force user agent
	req.Header.Set("User-Agent", UserAgent)
	// Log request
	if t.logHeader || t.logBody {
		buf, _ := httputil.DumpRequestOut(req, t.logBody)
		Debug(nil, "%s", separatorReq)
		Debug(nil, "%s", "HTTP REQUEST")
		Debug(nil, "%s", string(buf))
		Debug(nil, "%s", separatorReq)
	}
	// Do round trip
	resp, err = t.Transport.RoundTrip(req)
	// Log response
	if t.logHeader || t.logBody {
		Debug(nil, "%s", separatorResp)
		Debug(nil, "%s", "HTTP RESPONSE")
		if err != nil {
			Debug(nil, "Error: %v", err)
		} else {
			buf, _ := httputil.DumpResponse(resp, t.logBody)
			Debug(nil, "%s", string(buf))
		}
		Debug(nil, "%s", separatorResp)
	}
	return resp, err
}
