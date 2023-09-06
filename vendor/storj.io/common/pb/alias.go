// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package pb

import proto "github.com/gogo/protobuf/proto"

// Unmarshal is an alias for proto.Unmarshal.
func Unmarshal(buf []byte, pb proto.Message) error { return proto.Unmarshal(buf, pb) }

// Marshal is an alias for proto.Marshal.
func Marshal(pb proto.Message) ([]byte, error) { return proto.Marshal(pb) }
