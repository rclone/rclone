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

// User-Agents of the official O2 Cloud desktop clients ("omh <platform> client
// <version>"). SAPI calls of OAuth remotes send the one of the OAuth platform
// together with that client's ClientHello, as the service's CDN rejects a mix
// of browser and desktop client traits.
const (
	windowsUA = "omh windows client 4.0.1"
	macosUA   = "omh macos client 4.0.1"
)

// newSAPIClient keeps fshttp's logging, headers and accounting around SAPI's
// fingerprinted transport. It copies the desktop client of platform: on
// "macos" the Apple ClientHello over HTTP/2, on any other platform the Windows
// ClientHello over HTTP/1.1. Without a platform (no OAuth) it uses Chrome's
// ClientHello over HTTP/2. Other requests use fshttp unchanged.
func newSAPIClient(ctx context.Context, endpoint, platform string) (*http.Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid SAPI endpoint: %w", err)
	}
	client := fshttp.NewClient(ctx)
	if u.Scheme != "https" {
		return client, nil
	}
	noHTTP2 := fs.GetConfig(ctx).DisableHTTP2
	userAgent := func(ua string) {
		var ci *fs.ConfigInfo
		ctx, ci = fs.AddConfig(ctx)
		ci.UserAgent = ua
	}
	switch {
	case platform == "" && noHTTP2:
		return client, nil
	case platform == "":
		return newSAPIHTTP2Client(ctx, client, u, nil)
	case platform == "macos" && noHTTP2:
		// Qt offers only HTTP/1.1 when HTTP/2 is not allowed.
		userAgent(macosUA)
		return newSAPIHTTP1Client(ctx, client, u, func() utls.ClientHelloSpec { return o2MacHello("http/1.1") }), nil
	case platform == "macos":
		userAgent(macosUA)
		return newSAPIHTTP2Client(ctx, client, u, o2MacHello)
	default:
		userAgent(windowsUA)
		return newSAPIHTTP1Client(ctx, client, u, o2WindowsHello), nil
	}
}

// newSAPIHTTP1Client sends SAPI calls over HTTP/1.1 with the ClientHello of hello.
func newSAPIHTTP1Client(ctx context.Context, client *http.Client, u *url.URL, hello func() utls.ClientHelloSpec) *http.Client {
	sapi := fshttp.NewTransportCustom(ctx, func(t *http.Transport) {
		// The dialer does its own proxying and TLS, so keep a copy of the
		// settings it needs and stop net/http proxying by itself. A custom
		// TLS dialer also leaves net/http speaking HTTP/1.1.
		base := t.Clone()
		t.Proxy = nil
		t.DialTLSContext = sapiDialTLS(base, hello, false)
	})
	client.Transport = &sapiClientTransport{base: client.Transport, sapi: sapi, endpoint: u}
	return client
}

// sapiDialTLS returns a TLS dialer for http.Transport that sends the
// ClientHello of hello, or Chrome's if hello is nil.
func sapiDialTLS(base *http.Transport, hello func() utls.ClientHelloSpec, needH2 bool) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		cfg := base.TLSClientConfig.Clone()
		cfg.ServerName, _, _ = net.SplitHostPort(addr)
		var spec *utls.ClientHelloSpec
		if hello != nil {
			s := hello()
			spec = &s
		}
		return dialSAPITLS(ctx, network, addr, cfg, base, spec, needH2)
	}
}

// newSAPIHTTP2Client sends SAPI calls over HTTP/2 with the ClientHello of
// hello, offering "h2" and "http/1.1", or with Chrome's if hello is nil. If the
// server does not negotiate HTTP/2, the calls fall back to HTTP/1.1: with
// hello offering only "http/1.1", or with Go's own ClientHello for Chrome.
func newSAPIHTTP2Client(ctx context.Context, client *http.Client, u *url.URL, hello func(alpn ...string) utls.ClientHelloSpec) (*http.Client, error) {
	var h2Hello, http1Hello func() utls.ClientHelloSpec
	if hello != nil {
		h2Hello = func() utls.ClientHelloSpec { return hello(http2.NextProtoTLS, "http/1.1") }
		http1Hello = func() utls.ClientHelloSpec { return hello("http/1.1") }
	}
	var err error
	// http.Transport only speaks HTTP/2 over a *tls.Conn, so the deprecated
	// http2.Transport is still needed for uTLS connections.
	var h2 *http2.Transport //nolint:staticcheck // see above
	sapi := fshttp.NewTransportCustom(ctx, func(t *http.Transport) {
		// Set before Clone, which would otherwise install net/http's own
		// HTTP/2 "https" protocol and make RegisterProtocol below panic.
		t.TLSNextProto = make(map[string]func(string, *tls.Conn) http.RoundTripper)
		base := t.Clone()
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
			var spec *utls.ClientHelloSpec
			if h2Hello != nil {
				s := h2Hello()
				spec = &s
			}
			return dialSAPITLS(ctx, network, addr, cfg, base, spec, true)
		}
		// This transport handles SAPI only, so net/http's automatic HTTP/2
		// registration is disabled above and must not replace our uTLS transport.
		t.RegisterProtocol("https", &sapiHTTP2Transport{h2: h2})
		if http1Hello != nil {
			// The HTTP/1.1 fallback keeps the client's ClientHello too.
			t.Proxy = nil
			t.DialTLSContext = sapiDialTLS(base, http1Hello, false)
		}
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
	h2       *http2.Transport //nolint:staticcheck // see newSAPIHTTP2Client
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
	if t.h2 != nil {
		t.h2.CloseIdleConnections() //nolint:staticcheck // see newSAPIHTTP2Client
	}
}

// o2WindowsHello returns the ClientHello of the official O2 Cloud Windows
// client (Qt 5 with OpenSSL 1.1.1). It offers no ALPN, so SAPI is spoken over
// HTTP/1.1.
func o2WindowsHello() utls.ClientHelloSpec {
	return utls.ClientHelloSpec{
		TLSVersMin: utls.VersionTLS10,
		TLSVersMax: utls.VersionTLS13,
		CipherSuites: []uint16{
			0x1302, 0x1303, 0x1301, 0xc02c, 0xc030, 0x009f, 0xcca9, 0xcca8, 0xccaa, 0xc02b,
			0xc02f, 0x009e, 0xc024, 0xc028, 0x006b, 0xc023, 0xc027, 0x0067, 0xc00a, 0xc014,
			0x0039, 0xc009, 0xc013, 0x0033, 0x00ad, 0x00ab, 0xccae, 0xccad, 0xccac, 0x009d,
			0x00a9, 0xccab, 0x00ac, 0x00aa, 0x009c, 0x00a8, 0x003d, 0x003c, 0xc038, 0xc036,
			0x00b7, 0x00b3, 0x0095, 0x0091, 0x0035, 0x00af, 0x008d, 0xc037, 0xc035, 0x00b6,
			0x00b2, 0x0094, 0x0090, 0x002f, 0x00ae, 0x008c, 0x00ff,
		},
		CompressionMethods: []byte{0},
		Extensions: []utls.TLSExtension{
			&utls.SNIExtension{},
			&utls.SupportedPointsExtension{SupportedPoints: []byte{0, 1, 2}},
			&utls.SupportedCurvesExtension{Curves: []utls.CurveID{
				utls.X25519, utls.CurveP256, utls.CurveID(0x001e), utls.CurveP521, utls.CurveP384,
			}},
			&utls.SessionTicketExtension{},
			&utls.GenericExtension{Id: 22}, // encrypt_then_mac
			&utls.ExtendedMasterSecretExtension{},
			&utls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []utls.SignatureScheme{
				0x0403, 0x0503, 0x0603, 0x0807, 0x0808, 0x0809, 0x080a, 0x080b, 0x0804, 0x0805,
				0x0806, 0x0401, 0x0501, 0x0601, 0x0303, 0x0203, 0x0301, 0x0201, 0x0302, 0x0202,
				0x0402, 0x0502, 0x0602,
			}},
			&utls.SupportedVersionsExtension{Versions: []uint16{
				utls.VersionTLS13, utls.VersionTLS12, utls.VersionTLS11, utls.VersionTLS10,
			}},
			&utls.PSKKeyExchangeModesExtension{Modes: []uint8{utls.PskModeDHE}},
			&utls.KeyShareExtension{KeyShares: []utls.KeyShare{{Group: utls.X25519}}},
			&utls.UtlsPaddingExtension{GetPaddingLen: utls.BoringPaddingStyle},
		},
	}
}

// o2MacHello returns the ClientHello of the official O2 Cloud macOS client
// (Qt 6 with Apple Secure Transport), which offers TLS 1.2 only, offering the
// alpn protocols.
func o2MacHello(alpn ...string) utls.ClientHelloSpec {
	return utls.ClientHelloSpec{
		TLSVersMin: utls.VersionTLS10,
		TLSVersMax: utls.VersionTLS12,
		CipherSuites: []uint16{
			0x00ff, 0xc02c, 0xc02b, 0xc024, 0xc023, 0xc030, 0xc02f, 0xc028, 0xc027, 0x006b,
			0x0067, 0x0039, 0x0033, 0x0016, 0x009d, 0x003d, 0x003c, 0x0035, 0x002f, 0x000a,
			0x009d, // sent twice by the client
		},
		CompressionMethods: []byte{0},
		Extensions: []utls.TLSExtension{
			&utls.SNIExtension{},
			&utls.SupportedCurvesExtension{Curves: []utls.CurveID{utls.CurveP256, utls.CurveP384, utls.CurveP521}},
			&utls.SupportedPointsExtension{SupportedPoints: []byte{0}},
			&utls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []utls.SignatureScheme{
				0x0401, 0x0201, 0x0501, 0x0601, 0x0403, 0x0203, 0x0503, 0x0603,
			}},
			&utls.ALPNExtension{AlpnProtocols: alpn},
			&utls.StatusRequestExtension{},
			&utls.SCTExtension{},
			&utls.ExtendedMasterSecretExtension{},
		},
	}
}

type sapiHTTP2Transport struct {
	h2        *http2.Transport //nolint:staticcheck // see newSAPIHTTP2Client
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
	resp, err := t.h2.RoundTrip(req) //nolint:staticcheck // see newSAPIHTTP2Client
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

// dialSAPITLS dials addr with a uTLS ClientHello: spec if set, else Chrome's.
// If needH2 is set, the server must negotiate HTTP/2 or errSAPIHTTP1 is returned.
func dialSAPITLS(ctx context.Context, network, addr string, cfg *tls.Config, base *http.Transport, spec *utls.ClientHelloSpec, needH2 bool) (net.Conn, error) {
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
	hello := utls.HelloChrome_Auto
	if spec != nil {
		hello = utls.HelloCustom
	}
	uConn := utls.UClient(conn, uCfg, hello)
	if spec != nil {
		if err := uConn.ApplyPreset(spec); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("SAPI TLS hello: %w", err)
		}
		// uTLS always offers a session ID, but clients without TLS 1.3 and
		// session tickets (such as Secure Transport) offer none.
		if spec.TLSVersMax < utls.VersionTLS13 {
			uConn.HandshakeState.Hello.SessionId = nil
		}
	}
	if err := uConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("SAPI TLS handshake: %w", err)
	}
	if needH2 && uConn.ConnectionState().NegotiatedProtocol != http2.NextProtoTLS {
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
