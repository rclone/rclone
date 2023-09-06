// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcmetadata

import (
	"math/bits"

	"storj.io/drpc/drpcwire"
)

func varintSize(n uint64) uint64 {
	return (9*uint64(bits.Len64(n)) + 64) / 64
}

func encodedStringSize(x string) uint64 {
	return 1 + varintSize(uint64(len(x))) + uint64(len(x))
}

func appendEntry(buf []byte, key, value string) []byte {
	buf = append(buf, 10) // 1<<3 | 2
	buf = drpcwire.AppendVarint(buf, encodedStringSize(key)+encodedStringSize(value))

	buf = append(buf, 10) // 1<<3 | 2
	buf = drpcwire.AppendVarint(buf, uint64(len(key)))
	buf = append(buf, key...)

	buf = append(buf, 18) // 2<<3 | 2
	buf = drpcwire.AppendVarint(buf, uint64(len(value)))
	buf = append(buf, value...)

	return buf
}

func readEntry(buf []byte) (rem, key, value []byte, ok bool, err error) {
	var length uint64

	if len(buf) < 1 || buf[0] != 10 {
		goto bad
	}
	buf, length, ok, err = drpcwire.ReadVarint(buf[1:])
	if !ok || err != nil || length > uint64(len(buf)) {
		goto bad
	}

	key, value, ok, err = readKeyValue(buf[:length])
	if !ok || err != nil {
		goto bad
	}

	return buf[length:], key, value, true, nil
bad:
	return nil, nil, nil, false, err
}

func readKeyValue(buf []byte) (key, value []byte, ok bool, err error) {
	var length uint64

	if len(buf) < 1 || buf[0] != 10 {
		goto bad
	}
	buf, length, ok, err = drpcwire.ReadVarint(buf[1:])
	if !ok || err != nil || length > uint64(len(buf)) {
		goto bad
	}
	buf, key = buf[length:], buf[:length]

	if len(buf) < 1 || buf[0] != 18 {
		goto bad
	}
	buf, length, ok, err = drpcwire.ReadVarint(buf[1:])
	if !ok || err != nil || length > uint64(len(buf)) {
		goto bad
	}
	buf, value = buf[length:], buf[:length]

	if len(buf) != 0 {
		goto bad
	}

	return key, value, true, nil
bad:
	return nil, nil, false, err
}
