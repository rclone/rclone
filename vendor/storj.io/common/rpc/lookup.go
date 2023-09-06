// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package rpc

import (
	"context"
	"net"
)

// LookupNodeAddress resolves a storage node address to the first IP address resolved.
// If an IP address is accidentally provided it is returned back. This function
// is used to resolve storage node IP addresses so that uplinks can use
// IP addresses directly without resolving many hosts.
func LookupNodeAddress(ctx context.Context, nodeAddress string) string {
	host, port, err := net.SplitHostPort(nodeAddress)
	if err != nil {
		// If there was an error parsing out the port we just use a plain host.
		host = nodeAddress
		port = ""
	}

	// We check if the address is an IP address to decide if we need to resolve it or not.
	ip := net.ParseIP(host)
	// nodeAddress is already an IP, so we can use that.
	if ip != nil {
		return nodeAddress
	}

	// We have a hostname not an IP address so we should resolve the IP address
	// to give back to the uplink client.
	addresses, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil || len(addresses) == 0 {
		// We ignore the error because if this fails for some reason we can just
		// re-use the hostname, it just won't be as fast for the uplink to dial.
		return nodeAddress
	}

	// We return the first address found because some DNS servers already do
	// round robin load balancing and we would be messing with their behaviour
	// if we tried to get smart here.
	first := addresses[0]

	if port == "" {
		return first
	}
	return net.JoinHostPort(first, port)
}
