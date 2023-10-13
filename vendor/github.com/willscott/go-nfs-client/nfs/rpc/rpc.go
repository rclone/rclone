// Copyright © 2017 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: BSD-2-Clause
//
package rpc

import (
	"bytes"
	"math/rand"
	"time"

	"github.com/willscott/go-nfs-client/nfs/xdr"
)

type Auth struct {
	Flavor uint32
	Body   []byte
}

var AuthNull Auth

type AuthUnix struct {
	Stamp       uint32
	Machinename string
	Uid         uint32
	Gid         uint32
	GidLen      uint32
	Gids        uint32
}

func NewAuthUnix(machinename string, uid, gid uint32) *AuthUnix {
	return &AuthUnix{
		Stamp:       rand.New(rand.NewSource(time.Now().UnixNano())).Uint32(),
		Machinename: machinename,
		Uid:         uid,
		Gid:         gid,
		GidLen:      1,
	}
}

// Auth converts a into an Auth opaque struct
func (a AuthUnix) Auth() Auth {
	w := new(bytes.Buffer)
	xdr.Write(w, a)
	return Auth{
		1,
		w.Bytes(),
	}
}
