//go:build !windows && !plan9
// +build !windows,!plan9

package http

import (
	"log"
	"net"

	"github.com/coreos/go-systemd/v22/activation"
)

func getInheritedListeners() []net.Listener {
	sdListeners, err := activation.Listeners()
	if err != nil {
		log.Println("go-systemd/activation error:", err)
		return make([]net.Listener, 0)
	}
	return sdListeners
}
