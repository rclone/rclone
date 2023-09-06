// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package socket

// TCPFastOpenConnectSupported is true if TCPFastOpenConnect is supported on this platform.
const TCPFastOpenConnectSupported = false

func setLowPrioCongestionController(fd uintptr) error {
	// TODO: https://stackoverflow.com/questions/8532372/how-to-load-a-different-congestion-control-algorithm-in-mac-os-x
	return nil
}

func setLowEffortQoS(fd uintptr) error {
	// TODO
	return nil
}

func setTCPFastOpenConnect(fd uintptr) error {
	// TODO
	return nil
}
