package funambol

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sapiTLSContext(t *testing.T, server *httptest.Server) context.Context {
	t.Helper()
	ctx, ci := fs.AddConfig(context.Background())
	ci.UserAgent = browserUA
	// Trust the test server without disabling verification.
	certPath := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600))
	ci.CaCert = []string{certPath}
	return ctx
}

func TestSAPITransportDesktopClient(t *testing.T) {
	hellos := make(chan *tls.ClientHelloInfo, 4)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, 1, r.ProtoMajor)
		assert.Equal(t, windowsUA, r.UserAgent())
		assert.Equal(t, "present", r.Header.Get("X-Test"))
		access := oauthHeaderData(t, r.Header.Get("Authorization"))["accesstoken"].(string)
		w.Header().Set("Authorization", renewedHeader(access))
		_, _ = io.WriteString(w, `{"data":{"folders":[{"id":1,"name":"root"}]}}`)
	}))
	server.EnableHTTP2 = true
	server.TLS = &tls.Config{GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
		hellos <- hello
		return nil, nil
	}}
	server.StartTLS()
	defer server.Close()
	ctx := sapiTLSContext(t, server)
	fs.GetConfig(ctx).Headers = []*fs.HTTPOption{{Key: "X-Test", Value: "present"}}
	m := configmap.Simple{
		"user": testUser, "endpoint": server.URL,
		"client_id": testClient, "token": `{"access_token":"at1","refresh_token":"rt1"}`,
		"cookies": base64.StdEncoding.EncodeToString([]byte(`[{"n":"validationKey","v":"test"}]`)),
	}
	f, err := NewFs(ctx, "test", "", m)
	require.NoError(t, err)
	t.Cleanup(f.(*Fs).client.CloseIdleConnections)
	// Both construction paths must install the same transport.
	session, err := newSessionFs(ctx, "test", m)
	require.NoError(t, err)
	t.Cleanup(session.client.CloseIdleConnections)
	for range 2 {
		resp, err := session.client.Get(server.URL + "/sapi/test")
		require.NoError(t, err)
		_, err = io.Copy(io.Discard, resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
	}
	access, _ := savedToken(t, m)
	assert.Equal(t, "at1rrr", access, "OAuth tokens are renewed over the desktop transport")
	require.Len(t, hellos, 2, "HTTP/1.1 connections are reused")
	for range 2 {
		hello := <-hellos
		// The official client offers no ALPN and starts with TLS 1.3 AES-256.
		assert.Empty(t, hello.SupportedProtos)
		require.NotEmpty(t, hello.CipherSuites)
		assert.Equal(t, uint16(0x1302), hello.CipherSuites[0])
	}
	session.client.CloseIdleConnections()
	resp, err := session.client.Get(server.URL + "/sapi/test")
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Len(t, hellos, 1, "cleanup reaches the desktop transport through the OAuth transport")
}

func TestSAPITransportHTTP1Fallback(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, 1, r.ProtoMajor)
		_, _ = io.Copy(w, r.Body)
	}))
	server.StartTLS()
	defer server.Close()
	client, err := newSAPIClient(sapiTLSContext(t, server), server.URL, "")
	require.NoError(t, err)
	t.Cleanup(client.CloseIdleConnections)
	for _, path := range []string{"/sapi/test", "/oauth/token", "/sapi"} {
		// A non-replayable body must survive HTTP/1.1 fallback.
		req, err := http.NewRequest(http.MethodPost, server.URL+path, io.NopCloser(strings.NewReader("payload")))
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		assert.Equal(t, "payload", string(body))
	}
}

func TestSAPITransportRouting(t *testing.T) {
	hellos := make(chan *tls.ClientHelloInfo, 4)
	var requests atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			assert.Equal(t, 2, r.ProtoMajor)
		}
	}))
	server.EnableHTTP2 = true
	server.TLS = &tls.Config{GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
		hellos <- hello
		return nil, nil
	}}
	server.StartTLS()
	defer server.Close()
	ctx := sapiTLSContext(t, server)
	client, err := newSAPIClient(ctx, server.URL, "")
	require.NoError(t, err)
	t.Cleanup(client.CloseIdleConnections)
	for _, path := range []string{"/sapi/test", "/oauth/token", "/sapi"} {
		resp, err := client.Get(server.URL + path)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
	}
	require.Len(t, hellos, 2)
	assert.Equal(t, uint16(0x0a0a), (<-hellos).CipherSuites[0]&0x0f0f)
	assert.NotEqual(t, uint16(0x0a0a), (<-hellos).CipherSuites[0]&0x0f0f, "OAuth uses the ordinary TLS handshake")
	// A redirect to another host's SAPI path also uses the ordinary transport.
	other, err := newSAPIClient(ctx, "https://other.example.com", "")
	require.NoError(t, err)
	t.Cleanup(other.CloseIdleConnections)
	resp, err := other.Get(server.URL + "/sapi/test")
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Len(t, hellos, 1)
	assert.NotEqual(t, uint16(0x0a0a), (<-hellos).CipherSuites[0]&0x0f0f)
}

// The upload host the server publishes is a subdomain of the endpoint and gets
// the same session and client traits; other hosts do not.
func TestIsSAPIHost(t *testing.T) {
	endpoint, err := url.Parse("https://cloud.o2online.es")
	require.NoError(t, err)
	for raw, want := range map[string]bool{
		"https://cloud.o2online.es/sapi/x":        true,
		"https://CLOUD.o2online.es/sapi/x":        true,
		"https://upload.cloud.o2online.es/sapi/x": true,
		"https://upload.cloud.o2online.es:8443/":  true,
		"http://upload.cloud.o2online.es/sapi/x":  false,
		"https://o2online.es/sapi/x":              false,
		"https://evilcloud.o2online.es/sapi/x":    false,
		"https://cloud.o2online.es.evil.com/":     false,
		"https://cloud.o2online.es:8443/sapi/x":   false,
	} {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		assert.Equal(t, want, isSAPIHost(u, endpoint), raw)
	}
}

func TestSAPITransportDisableHTTP2(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, 1, r.ProtoMajor)
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	ctx := sapiTLSContext(t, server)
	fs.GetConfig(ctx).DisableHTTP2 = true
	client, err := newSAPIClient(ctx, server.URL, "")
	require.NoError(t, err)
	t.Cleanup(client.CloseIdleConnections)
	resp, err := client.Get(server.URL + "/sapi/test")
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
}

func TestSAPITransportProxy(t *testing.T) {
	for _, tc := range []struct {
		scheme   string
		platform string
		proto    int
	}{
		{"http", "", 2}, {"https", "", 2}, {"http", "windows", 1}, {"https", "windows", 1},
		{"http", "macos", 2}, {"https", "macos", 2},
	} {
		scheme := tc.scheme
		t.Run(fmt.Sprintf("%s/platform=%q", scheme, tc.platform), func(t *testing.T) {
			const body = "response through proxy"
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tc.proto, r.ProtoMajor)
				assert.Empty(t, r.Header.Get("Proxy-Authorization"))
				_, _ = io.WriteString(w, body)
			}))
			server.EnableHTTP2 = true
			server.StartTLS()
			defer server.Close()
			var connects atomic.Int32
			proxyServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				connects.Add(1)
				assert.Equal(t, http.MethodConnect, r.Method)
				assert.Equal(t, "Basic dXNlcjpwQHNzOndvcmQ=", r.Header.Get("Proxy-Authorization"))
				upstream, err := net.Dial("tcp", server.Listener.Addr().String())
				if !assert.NoError(t, err) {
					return
				}
				defer func() { _ = upstream.Close() }()
				conn, rw, err := w.(http.Hijacker).Hijack()
				if !assert.NoError(t, err) {
					return
				}
				defer func() { _ = conn.Close() }()
				_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
				go func() {
					_, _ = io.Copy(upstream, rw)
					_ = upstream.Close()
				}()
				_, _ = io.Copy(conn, upstream)
			}))
			if scheme == "https" {
				proxyServer.StartTLS()
			} else {
				proxyServer.Start()
			}
			defer proxyServer.Close()
			ctx := sapiTLSContext(t, server)
			// httptest uses the same CA for the origin and HTTPS proxy.
			fs.GetConfig(ctx).HTTPProxy = strings.Replace(proxyServer.URL, "://", "://user:p%40ss%3Aword@", 1)
			client, err := newSAPIClient(ctx, server.URL, tc.platform)
			require.NoError(t, err)
			defer client.CloseIdleConnections()
			resp, err := client.Get(server.URL + "/sapi/test")
			require.NoError(t, err)
			got, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			assert.Equal(t, body, string(got))
			assert.Equal(t, int32(1), connects.Load())
		})
	}
}

func TestSAPITransportHandshakeTimeout(t *testing.T) {
	conn, peer := net.Pipe()
	defer func() { _ = peer.Close() }()
	base := &http.Transport{
		TLSHandshakeTimeout: 25 * time.Millisecond,
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return conn, nil
		},
	}
	_, err := dialSAPITLS(context.Background(), "tcp", "example.com:443", &tls.Config{ServerName: "example.com"}, base, nil, true)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	_, err = io.Copy(io.Discard, peer)
	require.NoError(t, err, "a failed handshake closes the connection")
}

func TestSAPITransportCertificateVerification(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	client, err := newSAPIClient(context.Background(), server.URL, "")
	require.NoError(t, err)
	t.Cleanup(client.CloseIdleConnections)
	_, err = client.Get(server.URL + "/sapi/test")
	var certErr x509.UnknownAuthorityError
	require.ErrorAs(t, err, &certErr)
}

// helloListener records the first bytes each accepted connection sends.
type helloListener struct {
	net.Listener
	hellos chan []byte
}

func (l *helloListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &helloConn{Conn: conn, hellos: l.hellos}, nil
}

type helloConn struct {
	net.Conn
	hellos chan []byte
	done   bool
}

func (c *helloConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if !c.done && n > 0 {
		c.done = true
		c.hellos <- append([]byte(nil), p[:n]...)
	}
	return n, err
}

func TestSAPITransportMacClient(t *testing.T) {
	for _, h2 := range []bool{true, false} {
		t.Run(fmt.Sprintf("h2=%v", h2), func(t *testing.T) {
			infos := make(chan *tls.ClientHelloInfo, 4)
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if h2 {
					assert.Equal(t, 2, r.ProtoMajor)
				} else {
					assert.Equal(t, 1, r.ProtoMajor)
				}
				assert.Equal(t, macosUA, r.UserAgent())
				_, _ = io.WriteString(w, `{"data":{"folders":[{"id":1,"name":"root"}]}}`)
			}))
			server.EnableHTTP2 = h2
			server.TLS = &tls.Config{GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
				infos <- hello
				return nil, nil
			}}
			raw := &helloListener{Listener: server.Listener, hellos: make(chan []byte, 4)}
			server.Listener = raw
			server.StartTLS()
			defer server.Close()
			ctx := sapiTLSContext(t, server)
			m := configmap.Simple{
				"user": testUser, "endpoint": server.URL, "oauth_platform": "macos",
				"client_id": testClient, "token": `{"access_token":"at1","refresh_token":"rt1"}`,
				"cookies": base64.StdEncoding.EncodeToString([]byte(`[{"n":"validationKey","v":"test"}]`)),
			}
			f, err := NewFs(ctx, "test", "", m)
			require.NoError(t, err)
			client := f.(*Fs).client
			t.Cleanup(client.CloseIdleConnections)
			resp, err := client.Get(server.URL + "/sapi/test")
			require.NoError(t, err)
			_, err = io.Copy(io.Discard, resp.Body)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())

			wantProtos := [][]string{{"h2", "http/1.1"}}
			if !h2 {
				// The first connection offers HTTP/2, the retry only HTTP/1.1.
				wantProtos = append(wantProtos, []string{"http/1.1"})
			}
			require.Len(t, infos, len(wantProtos))
			for _, protos := range wantProtos {
				info := <-infos
				hello := <-raw.hellos
				// Byte 43 is the session ID length after the record and handshake
				// headers, the version and the random.
				require.Greater(t, len(hello), 43)
				assert.Equal(t, byte(0), hello[43], "Secure Transport offers no session ID")
				assert.Equal(t, []byte{0x03, 0x03}, hello[9:11], "client version TLS 1.2")
				assert.Equal(t, o2MacHello().CipherSuites, info.CipherSuites)
				assert.Equal(t, protos, info.SupportedProtos)
				// There is no SNI for an IP address.
				assert.Equal(t, []uint16{10, 11, 13, 16, 5, 18, 23}, info.Extensions)
				assert.Equal(t, []tls.CurveID{tls.CurveP256, tls.CurveP384, tls.CurveP521}, info.SupportedCurves)
			}
		})
	}
}

func TestSAPITransportUnusedDefaultTransport(t *testing.T) {
	// fshttp copies http.DefaultTransport, which gets an "h2" entry on first
	// use. Use an unused one so the SAPI transport is built as in a fresh process.
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	for _, platform := range []string{"", "macos"} {
		http.DefaultTransport = &http.Transport{ForceAttemptHTTP2: true}
		_, err := newSAPIClient(context.Background(), "https://cloud.example.com/sapi", platform)
		require.NoError(t, err, "platform %q", platform)
	}
}
