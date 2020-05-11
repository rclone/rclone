// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tlsopts

import (
	"storj.io/common/peertls/extensions"
)

// Config holds tls configuration parameters
type Config struct {
	RevocationDBURL     string `default:"bolt://$CONFDIR/revocations.db" help:"url for revocation database (e.g. bolt://some.db OR redis://127.0.0.1:6378?db=2&password=abc123)"`
	PeerCAWhitelistPath string `help:"path to the CA cert whitelist (peer identities must be signed by one these to be verified). this will override the default peer whitelist"`
	UsePeerCAWhitelist  bool   `devDefault:"false" releaseDefault:"true" help:"if true, uses peer ca whitelist checking"`
	PeerIDVersions      string `default:"latest" help:"identity version(s) the server will be allowed to talk to"`
	Extensions          extensions.Config
}
