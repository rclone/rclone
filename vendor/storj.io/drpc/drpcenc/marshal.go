// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcenc

import "storj.io/drpc"

// MarshalAppend calls enc.Marshal(msg) and returns the data appended to buf. If
// enc implements MarshalAppend, that is called instead.
func MarshalAppend(msg drpc.Message, enc drpc.Encoding, buf []byte) (data []byte, err error) {
	if ma, ok := enc.(interface {
		MarshalAppend(buf []byte, msg drpc.Message) ([]byte, error)
	}); ok {
		return ma.MarshalAppend(buf, msg)
	}
	data, err = enc.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return append(buf, data...), nil
}
