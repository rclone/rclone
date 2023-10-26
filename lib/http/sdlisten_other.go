//go:build windows || plan9
// +build windows plan9

package http

import (
	"net"
)

func getInheritedListeners() []net.Listener {
	return make([]net.Listener, 0)
}
