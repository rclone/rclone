// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"net"
	"time"

	"storj.io/common/identity"
	"storj.io/common/macaroon"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/uplink/private/metainfo"
)

// Config defines configuration for using uplink library.
type Config struct {
	UserAgent string

	// DialTimeout defines how long client should wait for establishing
	// a connection to peers.
	DialTimeout time.Duration

	// DialContext is how sockets are opened and is called to establish
	// a connection. If unset, net.Dialer is used.
	DialContext func(ctx context.Context, network, address string) (net.Conn, error)
}

type dialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)

func (f dialContextFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return f(ctx, network, address)
}

func (config Config) dial(ctx context.Context, satelliteAddress string, apiKey *macaroon.APIKey) (_ *metainfo.Client, _ rpc.Dialer, fullNodeURL string, err error) {
	ident, err := identity.NewFullIdentity(ctx, identity.NewCAOptions{
		Difficulty:  0,
		Concurrency: 1,
	})
	if err != nil {
		return nil, rpc.Dialer{}, "", packageError.Wrap(err)
	}

	tlsConfig := tlsopts.Config{
		UsePeerCAWhitelist: false,
		PeerIDVersions:     "0",
	}

	tlsOptions, err := tlsopts.NewOptions(ident, tlsConfig, nil)
	if err != nil {
		return nil, rpc.Dialer{}, "", packageError.Wrap(err)
	}

	dialer := rpc.NewDefaultDialer(tlsOptions)
	dialer.DialTimeout = config.DialTimeout
	if config.DialContext != nil {
		dialer.Transport = dialContextFunc(config.DialContext)
	}

	nodeURL, err := storj.ParseNodeURL(satelliteAddress)
	if err != nil {
		return nil, rpc.Dialer{}, "", packageError.Wrap(err)
	}

	// Node id is required in satelliteNodeID for all unknown (non-storj) satellites.
	// For known satellite it will be automatically prepended.
	if nodeURL.ID.IsZero() {
		nodeID, found := rpc.KnownNodeID(nodeURL.Address)
		if !found {
			return nil, rpc.Dialer{}, "", packageError.New("node id is required in satelliteNodeURL")
		}
		satelliteAddress = storj.NodeURL{
			ID:      nodeID,
			Address: nodeURL.Address,
		}.String()
	}

	metainfo, err := metainfo.DialNodeURL(ctx, dialer, satelliteAddress, apiKey, config.UserAgent)

	return metainfo, dialer, satelliteAddress, packageError.Wrap(err)
}
