// Copyright (c) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity

import (
	"context"
	"crypto"

	"storj.io/common/pkcrypto"
	"storj.io/common/storj"
)

// GenerateKey generates a private key with a node id with difficulty at least
// minDifficulty. No parallelism is used.
func GenerateKey(ctx context.Context, minDifficulty uint16, version storj.IDVersion) (
	k crypto.PrivateKey, id storj.NodeID, err error) {
	defer mon.Task()(&ctx)(&err)

	var d uint16
	for {
		err = ctx.Err()
		if err != nil {
			break
		}
		k, err = version.NewPrivateKey()
		if err != nil {
			break
		}

		var pubKey crypto.PublicKey
		pubKey, err = pkcrypto.PublicKeyFromPrivate(k)
		if err != nil {
			break
		}

		id, err = NodeIDFromKey(pubKey, version)
		if err != nil {
			break
		}
		d, err = id.Difficulty()
		if err != nil {
			break
		}
		if d >= minDifficulty {
			return k, id, nil
		}
	}
	return k, id, storj.ErrNodeID.Wrap(err)
}

// GenerateCallback indicates that key generation is done when done is true.
// if err != nil key generation will stop with that error.
type GenerateCallback func(crypto.PrivateKey, storj.NodeID) (done bool, err error)

// GenerateKeys continues to generate keys until found returns done == false,
// or the ctx is canceled.
func GenerateKeys(ctx context.Context, minDifficulty uint16, concurrency int, version storj.IDVersion, found GenerateCallback) (err error) {
	defer mon.Task()(&ctx)(&err)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errchan := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			for {
				k, id, err := GenerateKey(ctx, minDifficulty, version)
				if err != nil {
					errchan <- err
					return
				}

				done, err := found(k, id)
				if err != nil {
					errchan <- err
					return
				}
				if done {
					errchan <- nil
					return
				}
			}
		}()
	}

	// we only care about the first error. the rest of the errors will be
	// context cancellation errors
	return <-errchan
}
