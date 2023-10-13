// Copyright © 2017 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: BSD-2-Clause
//
package xdr

import (
	"io"

	xdr "github.com/rasky/go-xdr/xdr2"
)

func Read(r io.Reader, val interface{}) error {
	_, err := xdr.Unmarshal(r, val)
	return err
}

func ReadUint32(r io.Reader) (uint32, error) {
	var n uint32
	if err := Read(r, &n); err != nil {
		return n, err
	}

	return n, nil
}

func ReadOpaque(r io.Reader) ([]byte, error) {
	length, err := ReadUint32(r)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, length)
	if _, err = r.Read(buf); err != nil {
		return nil, err
	}

	return buf, nil
}

func ReadUint32List(r io.Reader) ([]uint32, error) {
	length, err := ReadUint32(r)
	if err != nil {
		return nil, err
	}

	buf := make([]uint32, length)

	for i := 0; i < int(length); i++ {
		buf[i], err = ReadUint32(r)
		if err != nil {
			return nil, err
		}
	}

	return buf, nil
}
