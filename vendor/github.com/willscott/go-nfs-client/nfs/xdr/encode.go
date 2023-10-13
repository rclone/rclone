// Copyright © 2017 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: BSD-2-Clause
//
package xdr

import (
	"io"

	xdr "github.com/rasky/go-xdr/xdr2"
)

func Write(w io.Writer, val interface{}) error {
	_, err := xdr.Marshal(w, val)
	return err
}
