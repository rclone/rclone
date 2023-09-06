// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package socket

import (
	"os"
	"strconv"
	"syscall"
)

// TCPFastOpenConnectSupported is true if TCPFastOpenConnect is supported on this platform.
var TCPFastOpenConnectSupported = func() bool {
	if envVar := os.Getenv("STORJ_SOCKET_TCP_FASTOPEN_CONNECT"); envVar != "" {
		if supported, err := strconv.ParseBool(envVar); err == nil {
			return supported
		}
	}
	// default
	return true
}()

var linuxLowPrioCongController = os.Getenv("STORJ_SOCKET_LOWPRIO_CTL")

const tcpFastOpenConnect = 30

func setLowPrioCongestionController(fd uintptr) error {
	if linuxLowPrioCongController != "" {
		return syscall.SetsockoptString(int(fd), syscall.IPPROTO_TCP, syscall.TCP_CONGESTION, linuxLowPrioCongController)
	}
	return nil
}

func setLowEffortQoS(fd uintptr) error {
	return syscall.SetsockoptByte(int(fd), syscall.SOL_IP, syscall.IP_TOS, byte(dscpLE)<<2)
}

func setTCPFastOpenConnect(fd uintptr) error {
	return syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, tcpFastOpenConnect, 1)
}
