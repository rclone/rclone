package funambol

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fshttp"
	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
	"golang.org/x/net/proxy"
)

var errSAPIHTTP1 = errors.New("SAPI endpoint does not support HTTP/2")

// newSAPIClient keeps fshttp's logging, headers and accounting around SAPI's
// Chrome ClientHello and HTTP/2 transport. Other requests use fshttp unchanged.
func newSAPIClient(ctx context.Context, endpoint string) (*http.Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid SAPI endpoint: %w", err)
	}
	client := fshttp.NewClient(ctx)
	if fs.GetConfig(ctx).DisableHTTP2 || u.Scheme != "https" {
		return client, nil
	}
	// http.Transport only speaks HTTP/2 over a *tls.Conn, so the deprecated
	// http2.Transport is still needed for uTLS connections.
	var h2 *http2.Transport //nolint:staticcheck // see above
	sapi := fshttp.NewTransportCustom(ctx, func(t *http.Transport) {
		// ConfigureTransports carries HTTP/1 transport settings into HTTP/2,
		// including response-header timeouts and disabled keep-alives.
		settings := &http.Transport{
			TLSClientConfig:        t.TLSClientConfig.Clone(),
			ResponseHeaderTimeout:  t.ResponseHeaderTimeout,
			IdleConnTimeout:        t.IdleConnTimeout,
			DisableKeepAlives:      t.DisableKeepAlives,
			DisableCompression:     t.DisableCompression,
			MaxResponseHeaderBytes: t.MaxResponseHeaderBytes,
		}
		h2, err = http2.ConfigureTransports(settings) //nolint:staticcheck // see above
		if err != nil {
			return
		}
		// uTLS dials new connections, so use HTTP/2's dialing pool instead of
		// the no-dial pool installed for transparent net/http upgrades.
		h2.ConnPool = nil                             //nolint:staticcheck // see above
		h2.TLSClientConfig = settings.TLSClientConfig //nolint:staticcheck // see above
		h2.DialTLSContext = func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
			return dialSAPITLS(ctx, network, addr, cfg, t)
		}
		// This transport handles SAPI only. Disable net/http's automatic
		// HTTP/2 registration so it does not replace our uTLS transport.
		t.TLSNextProto = make(map[string]func(string, *tls.Conn) http.RoundTripper)
		t.RegisterProtocol("https", &sapiHTTP2Transport{h2: h2})
	})
	if err != nil {
		return nil, fmt.Errorf("configure SAPI HTTP/2: %w", err)
	}
	client.Transport = &sapiClientTransport{base: client.Transport, sapi: sapi, endpoint: u, h2: h2}
	return client, nil
}

type sapiClientTransport struct {
	base     http.RoundTripper
	sapi     *fshttp.Transport
	endpoint *url.URL
	h2       *http2.Transport //nolint:staticcheck // see newSAPIClient
}

// RoundTrip implements http.RoundTripper.
func (t *sapiClientTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme == t.endpoint.Scheme && strings.EqualFold(req.URL.Host, t.endpoint.Host) &&
		strings.HasPrefix(req.URL.Path, "/sapi/") {
		return t.sapi.RoundTrip(req)
	}
	return t.base.RoundTrip(req)
}

// CloseIdleConnections closes both the ordinary and fingerprinted connections.
func (t *sapiClientTransport) CloseIdleConnections() {
	t.base.(interface{ CloseIdleConnections() }).CloseIdleConnections()
	t.sapi.CloseIdleConnections()
	t.h2.CloseIdleConnections() //nolint:staticcheck // see newSAPIClient
}

type sapiHTTP2Transport struct {
	h2        *http2.Transport //nolint:staticcheck // see newSAPIClient
	http1Only atomic.Bool
}

// RoundTrip implements http.RoundTripper.
func (t *sapiHTTP2Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.http1Only.Load() {
		return nil, http.ErrSkipAltProtocol
	}
	var body *heldBody
	if req.Body != nil && req.Body != http.NoBody {
		body = &heldBody{ReadCloser: req.Body, holding: true}
		req = req.WithContext(req.Context())
		req.Body = body
	}
	resp, err := t.h2.RoundTrip(req) //nolint:staticcheck // see newSAPIClient
	if errors.Is(err, errSAPIHTTP1) {
		// ALPN failed before any HTTP bytes or request body were sent.
		// The body stays open, so net/http can send it over HTTP/1.1.
		t.http1Only.Store(true)
		return nil, http.ErrSkipAltProtocol
	}
	if body != nil {
		body.release()
	}
	if err != nil && req.Body != nil {
		_ = req.Body.Close()
	}
	return resp, err
}

// heldBody holds back a Close of a request body that was not read while
// HTTP/2 tries a connection.  net/http can resend a body over HTTP/1.1 after
// ErrSkipAltProtocol only if it was neither read nor closed.
type heldBody struct {
	io.ReadCloser
	mu      sync.Mutex
	holding bool // a Close before a Read is held back
	read    bool
	held    bool // a Close was held back
}

// Read implements io.Reader.
func (b *heldBody) Read(p []byte) (int, error) {
	b.mu.Lock()
	b.read = true
	b.mu.Unlock()
	return b.ReadCloser.Read(p)
}

// Close implements io.Closer.
func (b *heldBody) Close() error {
	b.mu.Lock()
	if b.holding && !b.read {
		b.held = true
		b.mu.Unlock()
		return nil
	}
	b.mu.Unlock()
	return b.ReadCloser.Close()
}

// release stops holding back Close and closes the body if a Close was held.
func (b *heldBody) release() {
	b.mu.Lock()
	b.holding = false
	held := b.held
	b.held = false
	b.mu.Unlock()
	if held {
		_ = b.ReadCloser.Close()
	}
}

func dialSAPITLS(ctx context.Context, network, addr string, cfg *tls.Config, base *http.Transport) (net.Conn, error) {
	if base.TLSHandshakeTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, base.TLSHandshakeTimeout)
		defer cancel()
	}
	conn, err := dialSAPIProxy(ctx, network, addr, base)
	if err != nil {
		return nil, err
	}
	uCfg := &utls.Config{
		ServerName:            cfg.ServerName,
		RootCAs:               cfg.RootCAs,
		InsecureSkipVerify:    cfg.InsecureSkipVerify,
		VerifyPeerCertificate: cfg.VerifyPeerCertificate,
		MinVersion:            cfg.MinVersion,
		MaxVersion:            cfg.MaxVersion,
	}
	for _, cert := range cfg.Certificates {
		uCert := utls.Certificate{
			Certificate: cert.Certificate, PrivateKey: cert.PrivateKey, Leaf: cert.Leaf,
			OCSPStaple: cert.OCSPStaple, SignedCertificateTimestamps: cert.SignedCertificateTimestamps,
		}
		for _, scheme := range cert.SupportedSignatureAlgorithms {
			uCert.SupportedSignatureAlgorithms = append(uCert.SupportedSignatureAlgorithms, utls.SignatureScheme(scheme))
		}
		uCfg.Certificates = append(uCfg.Certificates, uCert)
	}
	uConn := utls.UClient(conn, uCfg, utls.HelloChrome_Auto)
	if err := uConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("SAPI TLS handshake: %w", err)
	}
	if uConn.ConnectionState().NegotiatedProtocol != http2.NextProtoTLS {
		_ = uConn.Close()
		return nil, errSAPIHTTP1
	}
	return &sapiTLSConn{UConn: uConn}, nil
}

// sapiTLSConn exposes the standard connection state expected by HTTP/2.
type sapiTLSConn struct {
	*utls.UConn
}

// ConnectionState returns the TLS state in the form expected by HTTP/2.
func (c *sapiTLSConn) ConnectionState() tls.ConnectionState {
	s := c.UConn.ConnectionState()
	return tls.ConnectionState{
		Version: s.Version, HandshakeComplete: s.HandshakeComplete, DidResume: s.DidResume,
		CipherSuite: s.CipherSuite, NegotiatedProtocol: s.NegotiatedProtocol,
		ServerName:       s.ServerName,
		PeerCertificates: s.PeerCertificates, VerifiedChains: s.VerifiedChains,
		SignedCertificateTimestamps: s.SignedCertificateTimestamps, OCSPResponse: s.OCSPResponse,
		TLSUnique: s.TLSUnique, ECHAccepted: s.ECHAccepted,
	}
}

// dialSAPIProxy applies the same proxy selection and dialer as fshttp.
func dialSAPIProxy(ctx context.Context, network, addr string, base *http.Transport) (net.Conn, error) {
	var proxyURL *url.URL
	var err error
	if base.Proxy != nil {
		proxyURL, err = base.Proxy(&http.Request{URL: &url.URL{Scheme: "https", Host: addr}})
		if err != nil {
			return nil, err
		}
	}
	if proxyURL == nil {
		return base.DialContext(ctx, network, addr)
	}
	if proxyURL.Scheme == "socks5" || proxyURL.Scheme == "socks5h" {
		dialer, err := proxy.SOCKS5(network, proxyAddress(proxyURL, "1080"), proxyAuth(proxyURL), sapiDialer{base.DialContext})
		if err != nil {
			return nil, err
		}
		return dialer.(proxy.ContextDialer).DialContext(ctx, network, addr)
	}
	if proxyURL.Scheme != "http" && proxyURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported SAPI proxy scheme %q", proxyURL.Scheme)
	}
	port := "80"
	if proxyURL.Scheme == "https" {
		port = "443"
	}
	conn, err := base.DialContext(ctx, network, proxyAddress(proxyURL, port))
	if err != nil {
		return nil, err
	}
	plainConn := conn
	stop := context.AfterFunc(ctx, func() { _ = plainConn.Close() })
	defer stop()
	if proxyURL.Scheme == "https" {
		cfg := base.TLSClientConfig.Clone()
		cfg.ServerName = proxyURL.Hostname()
		cfg.NextProtos = []string{"http/1.1"}
		tlsConn := tls.Client(conn, cfg)
		if err = tlsConn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return nil, err
		}
		conn = tlsConn
	}
	req := &http.Request{Method: http.MethodConnect, URL: &url.URL{Opaque: addr}, Host: addr, Header: make(http.Header)}
	if proxyURL.User != nil {
		password, _ := proxyURL.User.Password()
		req.SetBasicAuth(proxyURL.User.Username(), password)
		req.Header.Set("Proxy-Authorization", req.Header.Get("Authorization"))
		req.Header.Del("Authorization")
	}
	if err = req.Write(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	reader := bufio.NewReader(io.LimitReader(conn, 1<<20))
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("SAPI proxy CONNECT: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = conn.Close()
		return nil, fmt.Errorf("SAPI proxy CONNECT: %s", resp.Status)
	}
	buffered := make([]byte, reader.Buffered())
	_, _ = io.ReadFull(reader, buffered)
	return &sapiProxyConn{Conn: conn, reader: io.MultiReader(bytes.NewReader(buffered), conn)}, nil
}

func proxyAddress(u *url.URL, defaultPort string) string {
	port := u.Port()
	if port == "" {
		port = defaultPort
	}
	return net.JoinHostPort(u.Hostname(), port)
}

func proxyAuth(u *url.URL) *proxy.Auth {
	if u.User == nil {
		return nil
	}
	password, _ := u.User.Password()
	return &proxy.Auth{User: u.User.Username(), Password: password}
}

type sapiDialer struct {
	dial func(context.Context, string, string) (net.Conn, error)
}

// Dial implements proxy.Dialer.
func (d sapiDialer) Dial(network, addr string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, addr)
}

// DialContext implements proxy.ContextDialer.
func (d sapiDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return d.dial(ctx, network, addr)
}

type sapiProxyConn struct {
	net.Conn
	reader io.Reader
}

// Read reads buffered CONNECT response bytes before reading the connection.
func (c *sapiProxyConn) Read(p []byte) (int, error) { return c.reader.Read(p) }
