// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcwire

import (
	"encoding/binary"

	"github.com/zeebo/errs"

	"storj.io/drpc/drpcerr"
)

// MarshalError returns a byte form of the error with any error code incorporated.
func MarshalError(err error) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], drpcerr.Code(err))
	return append(buf[:], err.Error()...)
}

// UnmarshalError unmarshals the marshaled error to one with a code.
func UnmarshalError(data []byte) error {
	if len(data) < 8 {
		return errs.New("%s (drpcwire note: invalid error data)", data)
	}
	return drpcerr.WithCode(errs.New("%s", data[8:]), binary.BigEndian.Uint64(data[:8]))
}
