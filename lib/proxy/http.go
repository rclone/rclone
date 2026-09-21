package proxy

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/proxy"
)

// maxResponseBytes is the maximum size of CONNECT response we will
// read from the proxy so a malicious proxy can't use all our memory.
const maxResponseBytes = 1024 * 1024

// bufferedConn is a net.Conn which reads from buffered first then Conn
type bufferedConn struct {
	net.Conn
	buffered []byte // unread bytes received after the CONNECT response
}

// Read from buffered first then the underlying Conn
func (c *bufferedConn) Read(p []byte) (n int, err error) {
	if len(c.buffered) > 0 {
		n = copy(p, c.buffered)
		c.buffered = c.buffered[n:]
		return n, nil
	}
	return c.Conn.Read(p)
}

// HTTPConnectDial connects using HTTP CONNECT via proxyDialer
//
// It will read the HTTP proxy address from the environment in the
// standard way.
//
// It optionally takes a proxyDialer to dial the HTTP proxy server.
// If nil is passed, it will use the default net.Dialer.
func HTTPConnectDial(network, addr string, proxyURL *url.URL, proxyDialer proxy.Dialer) (net.Conn, error) {
	if proxyDialer == nil {
		proxyDialer = &net.Dialer{}
	}
	if proxyURL == nil {
		return proxyDialer.Dial(network, addr)
	}

	// prepare proxy host with default ports
	host := proxyURL.Host
	if !strings.Contains(host, ":") {
		if strings.EqualFold(proxyURL.Scheme, "https") {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	// connect to proxy
	conn, err := proxyDialer.Dial(network, host)
	if err != nil {
		return nil, fmt.Errorf("HTTP CONNECT proxy failed to Dial: %q", err)
	}

	// wrap TLS if HTTPS proxy
	if strings.EqualFold(proxyURL.Scheme, "https") {
		tlsConfig := &tls.Config{ServerName: proxyURL.Hostname()}
		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.Handshake(); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("HTTP CONNECT proxy failed to make TLS connection: %q", err)
		}
		conn = tlsConn
	}

	// send CONNECT
	user := proxyURL.User
	if user != nil {
		credential := base64.StdEncoding.EncodeToString([]byte(user.String()))
		_, err = fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Authorization: Basic %s\r\n\r\n", addr, addr, credential)
	} else {
		_, err = fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", addr, addr)
	}
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("HTTP CONNECT proxy failed to send CONNECT: %q", err)
	}
	limitedConn := &io.LimitedReader{R: conn, N: maxResponseBytes}
	br := bufio.NewReader(limitedConn)
	req := &http.Request{URL: &url.URL{Scheme: "http", Host: addr}}
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		_ = conn.Close()
		if limitedConn.N <= 0 {
			return nil, fmt.Errorf("HTTP CONNECT proxy response too large (more than %d bytes)", maxResponseBytes)
		}
		return nil, fmt.Errorf("HTTP CONNECT proxy failed to read response: %q", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = conn.Close()
		return nil, fmt.Errorf("HTTP CONNECT proxy failed: %s", resp.Status)
	}
	// The server may have sent bytes for the tunnelled protocol (eg an
	// SSH banner or FTP greeting) which br has buffered along with the
	// CONNECT response - make sure they aren't lost.
	if n := br.Buffered(); n > 0 {
		buffered := make([]byte, n)
		if _, err := io.ReadFull(br, buffered); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("HTTP CONNECT proxy failed to read buffered bytes: %q", err)
		}
		conn = &bufferedConn{Conn: conn, buffered: buffered}
	}
	return conn, nil
}
