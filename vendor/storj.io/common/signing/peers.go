// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package signing

import (
	"context"
	"crypto"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/common/identity"
	"storj.io/common/pkcrypto"
	"storj.io/common/storj"
)

var mon = monkit.Package()

// PrivateKey implements a signer and signee using a crypto.PrivateKey.
type PrivateKey struct {
	Self storj.NodeID
	Key  crypto.PrivateKey
}

// SignerFromFullIdentity returns signer based on full identity.
func SignerFromFullIdentity(identity *identity.FullIdentity) Signer {
	return &PrivateKey{
		Self: identity.ID,
		Key:  identity.Key,
	}
}

// ID returns node id associated with PrivateKey.
func (private *PrivateKey) ID() storj.NodeID { return private.Self }

// HashAndSign hashes the data and signs with the used key.
func (private *PrivateKey) HashAndSign(ctx context.Context, data []byte) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)
	return pkcrypto.HashAndSign(private.Key, data)
}

// HashAndVerifySignature hashes the data and verifies that the signature belongs to the PrivateKey.
func (private *PrivateKey) HashAndVerifySignature(ctx context.Context, data, signature []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	pub, err := pkcrypto.PublicKeyFromPrivate(private.Key)
	if err != nil {
		return err
	}

	return pkcrypto.HashAndVerifySignature(pub, data, signature)
}

// PublicKey implements a signee using crypto.PublicKey.
type PublicKey struct {
	Self storj.NodeID
	Key  crypto.PublicKey
}

// SigneeFromPeerIdentity returns signee based on peer identity.
func SigneeFromPeerIdentity(identity *identity.PeerIdentity) Signee {
	return &PublicKey{
		Self: identity.ID,
		Key:  identity.Leaf.PublicKey,
	}
}

// ID returns node id associated with this PublicKey.
func (public *PublicKey) ID() storj.NodeID { return public.Self }

// HashAndVerifySignature hashes the data and verifies that the signature belongs to the PublicKey.
func (public *PublicKey) HashAndVerifySignature(ctx context.Context, data, signature []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	return pkcrypto.HashAndVerifySignature(public.Key, data, signature)
}
