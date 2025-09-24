package proxy

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/proxy"
)

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
	_, err = fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", addr, addr)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("HTTP CONNECT proxy failed to send CONNECT: %q", err)
	}
	br := bufio.NewReader(conn)
	req := &http.Request{URL: &url.URL{Scheme: "http", Host: addr}}
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("HTTP CONNECT proxy failed to read response: %q", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = conn.Close()
		return nil, fmt.Errorf("HTTP CONNECT proxy failed: %s", resp.Status)
	}
	return conn, nil
}
