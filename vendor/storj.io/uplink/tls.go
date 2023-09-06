// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"sync"

	"storj.io/common/identity"
	"storj.io/common/peertls/tlsopts"
)

var processTLSOptions struct {
	mu         sync.Mutex
	tlsOptions *tlsopts.Options
}

func getProcessTLSOptions(ctx context.Context) (*tlsopts.Options, error) {
	processTLSOptions.mu.Lock()
	defer processTLSOptions.mu.Unlock()

	if processTLSOptions.tlsOptions != nil {
		return processTLSOptions.tlsOptions, nil
	}

	ident, err := identity.NewFullIdentity(ctx, identity.NewCAOptions{
		Difficulty:  0,
		Concurrency: 1,
	})
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	tlsConfig := tlsopts.Config{
		UsePeerCAWhitelist: false,
		PeerIDVersions:     "0",
	}

	tlsOptions, err := tlsopts.NewOptions(ident, tlsConfig, nil)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	processTLSOptions.tlsOptions = tlsOptions
	return tlsOptions, nil
}
