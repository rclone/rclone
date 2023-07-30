// Package fshttp contains the common http parts of the config, Transport and Client
package fshttp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/fshttp/fshttpdump"
	"github.com/rclone/rclone/lib/structs"
	"golang.org/x/net/publicsuffix"
)

var (
	transport    http.RoundTripper
	noTransport  = new(sync.Once)
	cookieJar, _ = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
)

// ResetTransport resets the existing transport, allowing it to take new settings.
// Should only be used for testing.
func ResetTransport() {
	noTransport = new(sync.Once)
}

// NewTransportCustom returns an http.RoundTripper with the correct timeouts.
// The customize function is called if set to give the caller an opportunity to
// customize any defaults in the Transport.
func NewTransportCustom(ctx context.Context, customize func(*http.Transport)) http.RoundTripper {
	ci := fs.GetConfig(ctx)
	// Start with a sensible set of defaults then override.
	// This also means we get new stuff when it gets added to go
	t := new(http.Transport)
	structs.SetDefaults(t, http.DefaultTransport.(*http.Transport))
	t.Proxy = http.ProxyFromEnvironment
	t.MaxIdleConnsPerHost = 2 * (ci.Checkers + ci.Transfers + 1)
	t.MaxIdleConns = 2 * t.MaxIdleConnsPerHost
	t.TLSHandshakeTimeout = ci.ConnectTimeout
	t.ResponseHeaderTimeout = ci.Timeout
	t.DisableKeepAlives = ci.DisableHTTPKeepAlives

	// TLS Config
	t.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: ci.InsecureSkipVerify,
	}

	// Load client certs
	if ci.ClientCert != "" || ci.ClientKey != "" {
		if ci.ClientCert == "" || ci.ClientKey == "" {
			log.Fatalf("Both --client-cert and --client-key must be set")
		}
		cert, err := tls.LoadX509KeyPair(ci.ClientCert, ci.ClientKey)
		if err != nil {
			log.Fatalf("Failed to load --client-cert/--client-key pair: %v", err)
		}
		t.TLSClientConfig.Certificates = []tls.Certificate{cert}
	}

	// Load CA certs
	if len(ci.CaCert) != 0 {

		caCertPool := x509.NewCertPool()

		for _, cert := range ci.CaCert {
			caCert, err := os.ReadFile(cert)
			if err != nil {
				log.Fatalf("Failed to read --ca-cert file %q : %v", cert, err)
			}
			ok := caCertPool.AppendCertsFromPEM(caCert)
			if !ok {
				log.Fatalf("Failed to add certificates from --ca-cert file %q", cert)
			}
		}
		t.TLSClientConfig.RootCAs = caCertPool
	}

	t.DisableCompression = ci.NoGzip
	t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialContext(ctx, network, addr, ci)
	}
	t.IdleConnTimeout = 60 * time.Second
	t.ExpectContinueTimeout = ci.ExpectContinueTimeout

	if ci.Dump&(fs.DumpHeaders|fs.DumpBodies|fs.DumpAuth|fs.DumpRequests|fs.DumpResponses) != 0 {
		fs.Debugf(nil, "You have specified to dump information. Please be noted that the "+
			"Accept-Encoding as shown may not be correct in the request and the response may not show "+
			"Content-Encoding if the go standard libraries auto gzip encoding was in effect. In this case"+
			" the body of the request will be gunzipped before showing it.")
	}

	if ci.DisableHTTP2 {
		t.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	}

	// customize the transport if required
	if customize != nil {
		customize(t)
	}

	// Wrap that http.Transport in our own transport
	return newTransport(ci, t)
}

// NewTransport returns an http.RoundTripper with the correct timeouts
func NewTransport(ctx context.Context) http.RoundTripper {
	(*noTransport).Do(func() {
		transport = NewTransportCustom(ctx, nil)
	})
	return transport
}

// NewClient returns an http.Client with the correct timeouts
func NewClient(ctx context.Context) *http.Client {
	ci := fs.GetConfig(ctx)
	client := &http.Client{
		Transport: NewTransport(ctx),
	}
	if ci.Cookie {
		client.Jar = cookieJar
	}
	return client
}

// Transport is our http Transport which wraps an http.Transport
// * Sets the User Agent
// * Does logging
// * Updates metrics
type Transport struct {
	*http.Transport
	dump          fs.DumpFlags
	filterRequest func(req *http.Request)
	userAgent     string
	headers       []*fs.HTTPOption
	metrics       *Metrics
}

// newTransport wraps the http.Transport passed in and logs all
// roundtrips including the body if logBody is set.
func newTransport(ci *fs.ConfigInfo, transport *http.Transport) *Transport {
	return &Transport{
		Transport: transport,
		dump:      ci.Dump,
		userAgent: ci.UserAgent,
		headers:   ci.Headers,
		metrics:   DefaultMetrics,
	}
}

// SetRequestFilter sets a filter to be used on each request
func (t *Transport) SetRequestFilter(f func(req *http.Request)) {
	t.filterRequest = f
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
		fs.Debugf(nil, "Couldn't parse Date: from server %s: %q: %v", host, dateString, err)
		return
	}
	dt := time.Since(date)
	const window = 5 * 60 * time.Second
	if dt > window || dt < -window {
		fs.Logf(nil, "Time may be set wrong - time from %q is %v different from this computer", host, dt)
	}
	checkedHostMu.Lock()
	checkedHost[host] = struct{}{}
	checkedHostMu.Unlock()
}

// RoundTrip implements the RoundTripper interface.
func (t *Transport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	// Limit transactions per second if required
	accounting.LimitTPS(req.Context())
	// Force user agent
	req.Header.Set("User-Agent", t.userAgent)
	// Set user defined headers
	for _, option := range t.headers {
		req.Header.Set(option.Key, option.Value)
	}
	// Filter the request if required
	if t.filterRequest != nil {
		t.filterRequest(req)
	}
	// Logf request
	fshttpdump.DumpRequest(req, t.dump, true)
	// Do round trip
	resp, err = t.Transport.RoundTrip(req)
	// Logf response
	fshttpdump.DumpResponse(resp, req, err, t.dump)
	// Update metrics
	t.metrics.onResponse(req, resp)

	if err == nil {
		checkServerTime(req, resp)
	}
	return resp, err
}
