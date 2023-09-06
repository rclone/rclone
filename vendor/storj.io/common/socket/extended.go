// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package socket

import (
	"context"
	"net"
	"syscall"

	"github.com/zeebo/errs"
)

// ExtendedDialer provides a DialContext that knows how to
// set different operating system options.
type ExtendedDialer struct {
	// LowPrioCongestionControl, if set, will try to set the lowest priority socket
	// settings, changing the congestion controller to a background congestion
	// controller if possible or available. On Linux, will use the kernel module
	// specified by STORJ_SOCKET_LOWPRIO_CTL. On Linux, 'cdg' is
	// recommended, with module parameters use_shadow=0 and use_ineff=0.
	LowPrioCongestionControl bool

	// LowEffortQoS, if set, will tell outgoing connections to try to set the low effort DSCP flag.
	LowEffortQoS bool

	// TCPFastOpenConnect is a Linux-only option that, if set, will tell outgoing
	// connections to try and dial with TCP_FASTOPEN.
	TCPFastOpenConnect bool
}

// DialContext will dial with the provided ExtendedDialer settings.
func (b ExtendedDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	var d net.Dialer
	if b.LowPrioCongestionControl || b.LowEffortQoS || b.TCPFastOpenConnect {
		d.Control = func(network, address string, c syscall.RawConn) error {
			var eg errs.Group
			eg.Add(c.Control(func(fd uintptr) {
				if b.LowPrioCongestionControl {
					eg.Add(setLowPrioCongestionController(fd))
				}
				if b.LowEffortQoS {
					eg.Add(setLowEffortQoS(fd))
				}
				if b.TCPFastOpenConnect {
					eg.Add(setTCPFastOpenConnect(fd))
				}
			}))
			err := eg.Err()
			if err != nil {
				// should we log this?
				_ = err
			}
			return nil
		}
	}
	return d.DialContext(ctx, network, address)
}
