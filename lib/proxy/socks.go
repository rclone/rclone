// Package proxy enables SOCKS5 proxy dialling
package proxy

import (
	"fmt"
	"net"
	"strings"

	"golang.org/x/net/proxy"
)

// SOCKS5Dial dials a net.Conn using a SOCKS5 proxy server.
// The socks5Proxy address can be in the form of [user:password@]host:port, [user@]host:port or just host:port if no auth is required.
// It will optionally take a proxyDialer to dial the SOCKS5 proxy server. If nil is passed, it will use the default net.Dialer.
func SOCKS5Dial(network, addr, socks5Proxy string, proxyDialer proxy.Dialer) (net.Conn, error) {

	if proxyDialer == nil {
		proxyDialer = &net.Dialer{}
	}
	var (
		proxyAddress string
		proxyAuth    *proxy.Auth
	)
	if credsAndHost := strings.SplitN(socks5Proxy, "@", 2); len(credsAndHost) == 2 {
		proxyCreds := strings.SplitN(credsAndHost[0], ":", 2)
		proxyAuth = &proxy.Auth{
			User: proxyCreds[0],
		}
		if len(proxyCreds) == 2 {
			proxyAuth.Password = proxyCreds[1]
		}
		proxyAddress = credsAndHost[1]
	} else {
		proxyAddress = credsAndHost[0]
	}
	proxyDialer, err := proxy.SOCKS5("tcp", proxyAddress, proxyAuth, proxyDialer)
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy dialer: %w", err)
	}
	return proxyDialer.Dial(network, addr)

}
