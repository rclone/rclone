// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package macaroon

import (
	"crypto/rand"
)

// NewCaveat returns a Caveat with a random generated nonce.
func NewCaveat() (Caveat, error) {
	var buf [8]byte
	_, err := rand.Read(buf[:])
	return Caveat{Nonce: buf[:]}, err
}
