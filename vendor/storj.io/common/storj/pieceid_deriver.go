// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"encoding/binary"

	"storj.io/common/internal/hmacsha512"
)

// PieceIDDeriver can be used to for multiple derivation from the same PieceID
// without need to initialize mac for each Derive call.
type PieceIDDeriver struct {
	mac hmacsha512.Partial
}

// Deriver creates piece ID dervier for multiple derive operations.
func (id PieceID) Deriver() PieceIDDeriver {
	return PieceIDDeriver{
		mac: hmacsha512.New(id[:]),
	}
}

// Derive a new PieceID from the piece ID, the given storage node ID and piece number.
// Initial mac is created from piece ID once while creating PieceDeriver and just
// reset to initial state at the beginning of each call.
func (pd PieceIDDeriver) Derive(storagenodeID NodeID, pieceNum int32) PieceID {
	pd.mac.Write(storagenodeID[:]) // on hash.Hash write never returns an error
	var num [4]byte
	binary.BigEndian.PutUint32(num[:], uint32(pieceNum))
	pd.mac.Write(num[:]) // on hash.Hash write never returns an error
	var derived PieceID
	sum := pd.mac.SumAndReset()
	copy(derived[:], sum[:])
	return derived
}
