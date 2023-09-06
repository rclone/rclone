// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !linux && !darwin && !windows
// +build !linux,!darwin,!windows

package socket

// TCPFastOpenConnectSupported is true if TCPFastOpenConnect is supported on this platform.
const TCPFastOpenConnectSupported = false

func setLowPrioCongestionController(fd uintptr) error { return nil }

func setLowEffortQoS(fd uintptr) error { return nil }

func setTCPFastOpenConnect(fd uintptr) error { return nil }
