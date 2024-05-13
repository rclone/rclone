//go:build windows || plan9
// +build windows plan9

// Package sdactivation provides support for systemd socket activation,
// wrapping the coreos/go-systemd package.
// This wraps the underlying go-systemd binary, as it fails to build on plan9
// https://github.com/coreos/go-systemd/pull/440
package sdactivation

import (
	"net"
)

// ListenersWithNames maps a listener name to a set of net.Listener instances.
// This wraps the underlying go-systemd binary, as it fails to build on plan9
// https://github.com/coreos/go-systemd/pull/440
func ListenersWithNames() (map[string][]net.Listener, error) {
	return make(map[string][]net.Listener), nil
}

// Listeners returns a slice containing a net.Listener for each matching socket type passed to this process.
func Listeners() ([]net.Listener, error) {
	return nil, nil
}
