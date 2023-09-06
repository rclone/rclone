// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package picobuf

import "google.golang.org/protobuf/encoding/protowire"

func encodeZigZag32(v int32) uint32 {
	return (uint32(v) << 1) ^ (uint32(v) >> 31)
}

func decodeZigZag32(v uint32) int32 {
	return int32(v>>1) ^ int32(v)<<31>>31
}

func appendTag(buf []byte, num FieldNumber, typ protowire.Type) []byte {
	x := uint64(num)<<3 | uint64(typ&7)
	for x >= 0x80 {
		buf = append(buf, byte(x)|0x80)
		x >>= 7
	}
	buf = append(buf, byte(x))
	return buf
}

func encodeBool64(v bool) uint64 {
	if v {
		return 1
	}
	return 0
}

func encodeBool8(v bool) byte {
	if v {
		return 1
	}
	return 0
}
