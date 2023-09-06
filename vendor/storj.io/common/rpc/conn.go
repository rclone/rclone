// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rpc

import (
	"github.com/zeebo/errs"

	"storj.io/common/identity"
	"storj.io/common/rpc/rpcpool"
)

// Conn is a wrapper around a drpc client connection.
type Conn struct {
	rpcpool.Conn
}

// PeerIdentity returns the peer identity on the other end of the connection.
func (c *Conn) PeerIdentity() (*identity.PeerIdentity, error) {
	state := c.Conn.State()
	if state == nil {
		return nil, errs.New("unknown identity: need to communicate first")
	}
	return identity.PeerIdentityFromChain(state.PeerCertificates)
}
