// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package socket

// TCPFastOpenConnectSupported is true if TCPFastOpenConnect is supported on this platform.
const TCPFastOpenConnectSupported = false

func setLowPrioCongestionController(fd uintptr) error {
	// TODO: Evidently some Windowses come with LEDBAT? A hint:
	// https://deploymentresearch.com/setup-low-extra-delay-background-transport-ledbat-for-configmgr/
	return nil
}

func setLowEffortQoS(fd uintptr) error {
	// TODO
	return nil
}

func setTCPFastOpenConnect(fd uintptr) error {
	// TODO: can we use standard tcp fast open without a send call, or
	// does windows have a version of TCP_FASTOPEN_CONNECT?
	return nil
}
