package proxy

import (
	"bufio"
	"errors"
	"net"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startProxy starts a fake HTTP CONNECT proxy which reads the CONNECT
// request then calls serve with the connection to send the response.
func startProxy(t *testing.T, serve func(conn net.Conn)) *url.URL {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = listener.Close()
	})
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() {
			_ = conn.Close()
		}()
		// Read the CONNECT request up to the blank line
		br := bufio.NewReader(conn)
		for {
			line, err := br.ReadString('\n')
			if err != nil || line == "\r\n" || line == "\n" {
				break
			}
		}
		serve(conn)
	}()
	proxyURL, err := url.Parse("http://" + listener.Addr().String())
	require.NoError(t, err)
	return proxyURL
}

func TestHTTPConnectDial(t *testing.T) {
	proxyURL := startProxy(t, func(conn net.Conn) {
		if _, err := conn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n")); err != nil {
			return
		}
		// echo the tunnelled data back
		buf := make([]byte, 4)
		if _, err := conn.Read(buf); err != nil {
			return
		}
		_, _ = conn.Write(buf)
	})
	conn, err := HTTPConnectDial("tcp", "example.com:1234", proxyURL, nil)
	require.NoError(t, err)
	defer func() {
		_ = conn.Close()
	}()
	_, err = conn.Write([]byte("ping"))
	require.NoError(t, err)
	buf := make([]byte, 4)
	_, err = conn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "ping", string(buf))
}

// Check that tunnel bytes the server sends immediately after the
// CONNECT response (eg an SSH banner) are not lost.
func TestHTTPConnectDialBuffered(t *testing.T) {
	proxyURL := startProxy(t, func(conn net.Conn) {
		// Send the response and the start of the tunnelled protocol in
		// a single write so they arrive in one read.
		_, _ = conn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\nSSH-2.0-banner\r\n"))
	})
	conn, err := HTTPConnectDial("tcp", "example.com:1234", proxyURL, nil)
	require.NoError(t, err)
	defer func() {
		_ = conn.Close()
	}()
	buf := make([]byte, 16)
	n, err := conn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "SSH-2.0-banner\r\n", string(buf[:n]))
}

// Check that a proxy sending an arbitrarily large response can't use
// unbounded memory.
func TestHTTPConnectDialTooLarge(t *testing.T) {
	proxyURL := startProxy(t, func(conn net.Conn) {
		_, err := conn.Write([]byte("HTTP/1.1 200 Connection established\r\nX-Fill: "))
		if err != nil {
			return
		}
		// Stream more header than maxResponseBytes - writes will error
		// once the client gives up and closes the connection.
		chunk := []byte(strings.Repeat("x", 64*1024))
		for written := 0; written <= 2<<20; written += len(chunk) {
			if _, err := conn.Write(chunk); err != nil {
				return
			}
		}
	})
	conn, err := HTTPConnectDial("tcp", "example.com:1234", proxyURL, nil)
	require.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "too large")
}

func TestHTTPConnectDialNon200(t *testing.T) {
	proxyURL := startProxy(t, func(conn net.Conn) {
		_, _ = conn.Write([]byte("HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\n\r\n"))
	})
	conn, err := HTTPConnectDial("tcp", "example.com:1234", proxyURL, nil)
	require.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "403")
}

// recordDialer records the address it is asked to dial and fails.
type recordDialer struct {
	addr string
}

func (d *recordDialer) Dial(network, addr string) (net.Conn, error) {
	d.addr = addr
	return nil, errors.New("not dialing")
}

// Check the proxy is dialled on the scheme's default port when the
// proxy URL has none, including for IPv6 literals.
func TestHTTPConnectDialDefaultPort(t *testing.T) {
	for _, test := range []struct {
		proxy string
		want  string
	}{
		{"http://proxy.example", "proxy.example:80"},
		{"https://proxy.example", "proxy.example:443"},
		{"http://proxy.example:3128", "proxy.example:3128"},
		{"http://[::1]", "[::1]:80"},
		{"https://[2001:db8::1]", "[2001:db8::1]:443"},
		{"http://[::1]:3128", "[::1]:3128"},
	} {
		t.Run(test.proxy, func(t *testing.T) {
			proxyURL, err := url.Parse(test.proxy)
			require.NoError(t, err)
			dialer := &recordDialer{}
			_, err = HTTPConnectDial("tcp", "example.com:1234", proxyURL, dialer)
			require.Error(t, err)
			assert.Equal(t, test.want, dialer.addr)
		})
	}
}
