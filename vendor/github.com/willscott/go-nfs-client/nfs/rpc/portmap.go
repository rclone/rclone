// Copyright © 2017 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: BSD-2-Clause
//
package rpc

import (
	"fmt"

	"github.com/willscott/go-nfs-client/nfs/xdr"
)

// PORTMAP
// RFC 1057 Section A.1

const (
	PmapPort = 111
	PmapProg = 100000
	PmapVers = 2

	PmapProcGetPort = 3

	IPProtoTCP = 6
	IPProtoUDP = 17
)

type Header struct {
	Rpcvers uint32
	Prog    uint32
	Vers    uint32
	Proc    uint32
	Cred    Auth
	Verf    Auth
}

type Mapping struct {
	Prog uint32
	Vers uint32
	Prot uint32
	Port uint32
}

type Portmapper struct {
	*Client
	host string
}

func (p *Portmapper) Getport(mapping Mapping) (int, error) {
	type getport struct {
		Header
		Mapping
	}
	msg := &getport{
		Header{
			Rpcvers: 2,
			Prog:    PmapProg,
			Vers:    PmapVers,
			Proc:    PmapProcGetPort,
			Cred:    AuthNull,
			Verf:    AuthNull,
		},
		mapping,
	}
	res, err := p.Call(msg)
	if err != nil {
		return 0, err
	}
	port, err := xdr.ReadUint32(res)
	if err != nil {
		return int(port), err
	}
	return int(port), nil
}

func DialPortmapper(net, host string) (*Portmapper, error) {
	client, err := DialTCP(net, nil, fmt.Sprintf("%s:%d", host, PmapPort))
	if err != nil {
		return nil, err
	}
	return &Portmapper{client, host}, nil
}
