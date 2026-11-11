package funambol

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
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

func TestSAPITransportHTTP2(t *testing.T) {
	hellos := make(chan *tls.ClientHelloInfo, 4)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, 2, r.ProtoMajor)
		assert.Equal(t, browserUA, r.UserAgent())
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
		require.NotNil(t, resp.TLS)
		assert.Equal(t, "h2", resp.TLS.NegotiatedProtocol)
		assert.NotEmpty(t, resp.TLS.VerifiedChains)
		_, err = io.Copy(io.Discard, resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
	}
	access, _ := savedToken(t, m)
	assert.Equal(t, "at1rrr", access, "OAuth tokens are renewed over HTTP/2")
	require.Len(t, hellos, 2, "HTTP/2 connections are reused")
	for range 2 {
		hello := <-hellos
		assert.Equal(t, []string{"h2", "http/1.1"}, hello.SupportedProtos)
		// Chrome's ClientHello starts with a GREASE cipher; crypto/tls doesn't.
		require.NotEmpty(t, hello.CipherSuites)
		assert.Equal(t, uint16(0x0a0a), hello.CipherSuites[0]&0x0f0f)
	}
	session.client.CloseIdleConnections()
	resp, err := session.client.Get(server.URL + "/sapi/test")
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Len(t, hellos, 1, "cleanup reaches HTTP/2 through the OAuth transport")
}

func TestSAPITransportHTTP1Fallback(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, 1, r.ProtoMajor)
		_, _ = io.Copy(w, r.Body)
	}))
	server.StartTLS()
	defer server.Close()
	client, err := newSAPIClient(sapiTLSContext(t, server), server.URL)
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
	client, err := newSAPIClient(ctx, server.URL)
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
	other, err := newSAPIClient(ctx, "https://other.example.com")
	require.NoError(t, err)
	t.Cleanup(other.CloseIdleConnections)
	resp, err := other.Get(server.URL + "/sapi/test")
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Len(t, hellos, 1)
	assert.NotEqual(t, uint16(0x0a0a), (<-hellos).CipherSuites[0]&0x0f0f)
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
	client, err := newSAPIClient(ctx, server.URL)
	require.NoError(t, err)
	t.Cleanup(client.CloseIdleConnections)
	resp, err := client.Get(server.URL + "/sapi/test")
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
}

func TestSAPITransportProxy(t *testing.T) {
	for _, scheme := range []string{"http", "https"} {
		t.Run(scheme, func(t *testing.T) {
			const body = "response through proxy"
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, 2, r.ProtoMajor)
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
			client, err := newSAPIClient(ctx, server.URL)
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
	_, err := dialSAPITLS(context.Background(), "tcp", "example.com:443", &tls.Config{ServerName: "example.com"}, base)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	_, err = io.Copy(io.Discard, peer)
	require.NoError(t, err, "a failed handshake closes the connection")
}

func TestSAPITransportCertificateVerification(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	client, err := newSAPIClient(context.Background(), server.URL)
	require.NoError(t, err)
	t.Cleanup(client.CloseIdleConnections)
	_, err = client.Get(server.URL + "/sapi/test")
	var certErr x509.UnknownAuthorityError
	require.ErrorAs(t, err, &certErr)
}
