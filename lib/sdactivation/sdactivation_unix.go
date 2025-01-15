//go:build !windows && !plan9
// +build !windows,!plan9

// Package sdactivation provides support for systemd socket activation, wrapping
// the coreos/go-systemd package.
// This wraps the underlying go-systemd library, as it fails to build on plan9
// https://github.com/coreos/go-systemd/pull/440
package sdactivation

import (
	"net"

	sdActivation "github.com/coreos/go-systemd/v22/activation"
)

// ListenersWithNames maps a listener name to a set of net.Listener instances.
func ListenersWithNames() (map[string][]net.Listener, error) {
	return sdActivation.ListenersWithNames()
}

// Listeners returns a slice containing a net.Listener for each matching socket type passed to this process.
func Listeners() ([]net.Listener, error) {
	return sdActivation.Listeners()
}
