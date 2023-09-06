// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package drpcmanager reads packets from a transport to make streams.
package drpcmanager

// closedCh is an already closed channel.
var closedCh = func() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}()
